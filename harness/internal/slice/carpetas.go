package slice

import (
	"fmt"
	"path"
	"slices"
	"sort"
	"strings"

	"orq/internal/config"
)

// Manifiestos marcan una carpeta como un proyecto con su propio toolchain: si un slice la toca, el
// examen solo la cubre si el proyecto le declara checks (carpetas.<ruta>.checks) o la exime.
var Manifiestos = []string{"package.json", "pyproject.toml", "setup.py", "go.mod", "Cargo.toml", "Gemfile", "composer.json", "pom.xml", "build.gradle", "deno.json"}

// Raiz es la carpeta dueña de lo que no cae en ninguna subcarpeta declarada ni con manifiesto.
const Raiz = "."

// cubreCarpetas dice si orq exige que cada carpeta tocada tenga checks: con Cobertura (orq2) y un
// proyecto que declaró checks. Sin checks no hay nada que exigir: el comportamiento de antes.
func (r Repo) cubreCarpetas() bool {
	return r.Cobertura && (len(r.Checks) > 0 || len(r.Carpetas) > 0)
}

// proyectosEn lista las subcarpetas con un manifiesto versionado en sha (la raíz no cuenta).
func (r Repo) proyectosEn(sha string) ([]string, error) {
	out, err := r.git("ls-tree", "-r", "--name-only", "-z", sha)
	if err != nil {
		return nil, err
	}
	var ds []string
	for _, f := range strings.Split(out, "\x00") {
		if d := path.Dir(f); f != "" && d != "." && slices.Contains(Manifiestos, path.Base(f)) && !strings.Contains("/"+d+"/", "/node_modules/") && !slices.Contains(ds, d) {
			ds = append(ds, d)
		}
	}
	return ds, nil
}

// limpiarRuta deja de una entrada de alcance (archivo, carpeta o patrón) la parte fija del camino.
func limpiarRuta(p string) string {
	if i := strings.IndexAny(p, "*?["); i >= 0 {
		p = p[:i]
	}
	p = strings.Trim(path.Clean("/"+strings.TrimPrefix(p, "./")), "/")
	if p == "" {
		return Raiz
	}
	return p
}

// duenaDe es la carpeta más profunda, entre las declaradas y las de manifiesto, que contiene p.
func duenaDe(p string, carpetas []string) string {
	mejor := Raiz
	for _, d := range carpetas {
		if (p == d || strings.HasPrefix(p, d+"/")) && (mejor == Raiz || len(d) > len(mejor)) {
			mejor = d
		}
	}
	return mejor
}

// cobertura reparte rutas (alcance y diff del slice) entre sus carpetas dueñas, con los manifiestos
// del commit sha. Devuelve las carpetas declaradas con checks que tocan, en orden, y las dueñas
// sin checks ni exención, cada una con una ruta de ejemplo.
func (r Repo) cobertura(sha string, rutas []string) (aplican []config.Carpeta, sin map[string]string, err error) {
	if len(r.Carpetas) == 0 && !r.cubreCarpetas() {
		return nil, nil, nil
	}
	var todas []string
	declaradas := map[string]config.Carpeta{}
	for _, ca := range r.Carpetas {
		declaradas[ca.Ruta] = ca
		if ca.Ruta != Raiz {
			todas = append(todas, ca.Ruta)
		}
	}
	if r.cubreCarpetas() {
		ps, err := r.proyectosEn(sha)
		if err != nil {
			return nil, nil, err
		}
		todas = append(todas, ps...)
	}
	tocadas := map[string]bool{}
	sin = map[string]string{}
	for _, p := range rutas {
		p = limpiarRuta(p)
		d := duenaDe(p, todas)
		ca, declarada := declaradas[d]
		switch {
		case declarada && len(ca.Lista) > 0:
			tocadas[d] = true
		case declarada, d == Raiz && len(r.Checks) > 0:
		case r.cubreCarpetas():
			if _, ya := sin[d]; !ya {
				sin[d] = p
			}
		}
	}
	for _, ca := range r.Carpetas {
		if tocadas[ca.Ruta] {
			aplican = append(aplican, ca)
		}
	}
	if len(sin) == 0 {
		sin = nil
	}
	return aplican, sin, nil
}

// errSinChecks explica qué carpeta tocada no tiene checks y cómo declararlos o eximirla.
func errSinChecks(id string, sin map[string]string) error {
	var ds []string
	for d := range sin {
		ds = append(ds, d)
	}
	sort.Strings(ds)
	var xs []string
	for _, d := range ds {
		if d == Raiz {
			xs = append(xs, fmt.Sprintf("la raíz (%s) no declara checks: declara el bloque checks o exímela con carpetas.\".\".exenta \"motivo\"", sin[d]))
			continue
		}
		xs = append(xs, fmt.Sprintf("%s/ (%s) no declara checks: decláralos en carpetas.%q.checks (lint, types y tests; corren dentro de %s/ con sus propias dependencias, p. ej. su node_modules) o exímela con carpetas.%q.exenta \"motivo\"", d, sin[d], d, d, d))
	}
	return fmt.Errorf("%s toca carpetas que el examen no cubriría, en .orq/config.json: %s", id, strings.Join(xs, "; "))
}

// rutasDelSlice son el alcance y lo que el slice cambió del rojo a head.
func (r Repo) rutasDelSlice(e *Estado, head string) ([]string, error) {
	fs, err := r.tocadas(e.SHARojo, head)
	if err != nil {
		return nil, err
	}
	return append(slices.Clone(e.Alcance), fs...), nil
}

// checksDeCarpetas corre los checks de cada carpeta en dir/<ruta>, con el nombre <ruta>/<check>.
func (r Repo) checksDeCarpetas(e *Estado, dir string, cs []config.Carpeta) ([]ResultadoCheck, error) {
	var out []ResultadoCheck
	for _, ca := range cs {
		sub := strings.TrimSuffix(dir, "/") + "/" + ca.Ruta
		rs, err := CorrerChecks(ca.Lista, sub, func(cmd string) (string, int, error) {
			o, code, _, err := r.correrEnCarpeta(dir, ca.Ruta, cmd, e.limite())
			return o, code, err
		})
		for i := range rs {
			rs[i].Nombre = ca.Ruta + "/" + rs[i].Nombre
		}
		out = append(out, rs...)
		if err != nil {
			return out, fmt.Errorf("%s: %w", ca.Ruta, err)
		}
	}
	return out, nil
}

// CarpetasTocadas son las carpetas declaradas con checks que contienen alguna de rutas (archivos,
// carpetas o patrones de alcance), en el orden de cs. Solo mira las declaradas: es el lazo local del
// worker, un superconjunto de lo que Verificar correrá (allí una subcarpeta con su propio manifiesto
// dentro de una declarada es otra dueña).
func CarpetasTocadas(cs []config.Carpeta, rutas []string) []config.Carpeta {
	var decl []string
	for _, ca := range cs {
		if ca.Ruta != Raiz && len(ca.Lista) > 0 {
			decl = append(decl, ca.Ruta)
		}
	}
	tocadas := map[string]bool{}
	for _, p := range rutas {
		tocadas[duenaDe(limpiarRuta(p), decl)] = true
	}
	var out []config.Carpeta
	for _, ca := range cs {
		if ca.Ruta != Raiz && len(ca.Lista) > 0 && tocadas[ca.Ruta] {
			out = append(out, ca)
		}
	}
	return out
}
