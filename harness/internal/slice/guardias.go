package slice

// Guardias que orq aplica al diff del slice, del rojo al commit verificado: archivos que definen lo
// que corre un check, pruebas borradas y marcas de supresión en líneas nuevas. Corren en el repo
// principal, que es de confianza; nada de esto ejecuta código del worker.

import (
	"encoding/json"
	"fmt"
	"maps"
	"path"
	"slices"
	"strconv"
	"strings"

	"orq/internal/config"
)

// definicionesBase son los archivos (por nombre, en cualquier carpeta) que deciden qué corre un check.
var definicionesBase = []string{"package.json", "tsconfig*.json", "eslint.config.*", ".eslintrc*", "biome.json*",
	".golangci.*", "pyproject.toml", "setup.cfg", "ruff.toml", ".prettierrc*"}

func esDefinicion(f string, extra []string) bool {
	for _, p := range definicionesBase {
		if ok, _ := path.Match(p, path.Base(f)); ok {
			return true
		}
	}
	for _, p := range extra {
		if ok, _ := path.Match(p, f); ok || p == f {
			return true
		}
	}
	return false
}

// definicionesCambiadas lista los archivos de definición de checks que cambian entre desde y hasta.
// De package.json solo cuenta scripts y de pyproject.toml solo las secciones [tool.*].
func (r Repo) definicionesCambiadas(desde, hasta string) ([]string, error) {
	fs, err := r.tocadas(desde, hasta)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, f := range fs {
		if !esDefinicion(f, r.Definiciones) {
			continue
		}
		var parte func(string) string
		switch path.Base(f) {
		case "package.json":
			parte = scriptsDe
		case "pyproject.toml":
			parte = seccionesTool
		}
		if parte != nil && parte(r.blobTexto(desde, f)) == parte(r.blobTexto(hasta, f)) {
			continue
		}
		out = append(out, f)
	}
	return out, nil
}

// blobTexto es el contenido de f en sha, o "" si ahí no existe.
func (r Repo) blobTexto(sha, f string) string {
	out, err := r.git("show", sha+":"+f)
	if err != nil {
		return ""
	}
	return out
}

// scriptsDe devuelve el campo scripts de un package.json en forma canónica; un archivo ilegible
// se devuelve entero, así cualquier cambio cuenta.
func scriptsDe(raw string) string {
	var pj struct {
		Scripts map[string]string `json:"scripts"`
	}
	if raw == "" {
		return ""
	}
	if json.Unmarshal([]byte(raw), &pj) != nil {
		return "ilegible:" + raw
	}
	var b strings.Builder
	for _, k := range slices.Sorted(maps.Keys(pj.Scripts)) {
		b.WriteString(strconv.Quote(k) + "=" + strconv.Quote(pj.Scripts[k]) + "\n")
	}
	return b.String()
}

// seccionesTool devuelve las secciones [tool.*] y [[tool.*]] de un pyproject.toml.
func seccionesTool(raw string) string {
	var b strings.Builder
	dentro := false
	for _, l := range strings.Split(raw, "\n") {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "[") {
			dentro = strings.HasPrefix(t, "[tool.") || strings.HasPrefix(t, "[[tool.") || t == "[tool]"
		}
		if dentro {
			b.WriteString(l + "\n")
		}
	}
	return b.String()
}

// DiffDefiniciones es el diff de los archivos de definición entre desde y hasta.
func (r Repo) DiffDefiniciones(desde, hasta string, fs []string) (string, error) {
	if len(fs) == 0 {
		return "", nil
	}
	return r.git(append([]string{"diff", "--no-color", "--no-ext-diff", desde, hasta, "--"}, fs...)...)
}

// esTest dice si f es un archivo de pruebas por su nombre o su carpeta.
func esTest(f string) bool {
	for _, p := range []string{"*_test.go", "*.test.*", "*.spec.*", "test_*.py", "*_test.py"} {
		if ok, _ := path.Match(p, path.Base(f)); ok {
			return true
		}
	}
	partes := strings.Split(f, "/")
	for _, d := range partes[:len(partes)-1] {
		if d == "test" || d == "tests" || d == "__tests__" {
			return true
		}
	}
	return false
}

// testsBorrados lista los archivos de prueba que el slice borró o renombró ("viejo → nuevo").
func (r Repo) testsBorrados(desde, hasta string) ([]string, error) {
	out, err := r.git("diff", "--name-status", "-M", "-z", desde, hasta)
	if err != nil {
		return nil, err
	}
	var res []string
	cs := strings.Split(out, "\x00")
	for i := 0; i < len(cs); i++ {
		st := cs[i]
		switch {
		case st == "":
		case (st[0] == 'R' || st[0] == 'C') && i+2 < len(cs):
			if st[0] == 'R' && esTest(cs[i+1]) {
				res = append(res, cs[i+1]+" → "+cs[i+2])
			}
			i += 2
		case i+1 < len(cs):
			if st[0] == 'D' && esTest(cs[i+1]) {
				res = append(res, cs[i+1])
			}
			i++
		}
	}
	slices.Sort(res)
	return res, nil
}

// SupresionesEn lista "archivo:línea: texto" de cada línea que suma diff (git diff -U0) y lleva una marca.
func SupresionesEn(diff string, marcas []string) []string {
	var out []string
	archivo, linea := "", 0
	for _, l := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(l, "+++ "):
			archivo = strings.TrimPrefix(strings.TrimPrefix(l, "+++ "), "b/")
		case strings.HasPrefix(l, "@@ "):
			// @@ -a,b +c,d @@
			campos := strings.Fields(l)
			if len(campos) >= 3 {
				n, _, _ := strings.Cut(strings.TrimPrefix(campos[2], "+"), ",")
				linea, _ = strconv.Atoi(n)
			}
		case strings.HasPrefix(l, "+"):
			if slices.ContainsFunc(marcas, func(m string) bool { return tieneMarca(l, m) }) {
				out = append(out, fmt.Sprintf("%s:%d: %s", archivo, linea, strings.TrimSpace(l[1:])))
			}
			linea++
		}
	}
	return out
}

// tieneMarca dice si l lleva la marca m. Una marca que empieza por letra (fit(, eslint-disable) no
// cuenta dentro de otra palabra: benefit( no es fit(.
func tieneMarca(l, m string) bool {
	if m == "" || !esIdent(m[0]) {
		return m != "" && strings.Contains(l, m)
	}
	for i := 0; ; {
		j := strings.Index(l[i:], m)
		if j < 0 {
			return false
		}
		if k := i + j; k == 0 || !esIdent(l[k-1]) {
			return true
		}
		i += j + 1
	}
}

func esIdent(c byte) bool {
	return c == '_' || c == '$' || '0' <= c && c <= '9' || 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z'
}

// CmdSupresiones es lo que la página y el recibo muestran como comando de la guardia supresiones.
const CmdSupresiones = "orq: marcas de supresión en las líneas que suma el slice"

// supresiones corre la guardia sobre el diff de desde a hasta.
func (r Repo) supresiones(ch config.Check, desde, hasta string) (ResultadoCheck, error) {
	res := ResultadoCheck{Nombre: ch.Nombre, Cmd: CmdSupresiones, Resultado: CheckPaso}
	if ch.Omitir != "" {
		return ResultadoCheck{Nombre: ch.Nombre, Resultado: CheckOmitido, Motivo: ch.Omitir}, nil
	}
	diff, err := r.git("diff", "-U0", "-M", "--no-color", "--no-ext-diff", desde, hasta)
	if err != nil {
		return res, err
	}
	if hs := SupresionesEn(diff, ch.Marcas()); len(hs) > 0 {
		res.Exit, res.Resultado, res.Salida = 1, CheckFallo, primeras(strings.Join(hs, "\n"), LineasCheck)
	}
	return res, nil
}

// SupresionesEnCopia corre la guardia en la copia del worker (orq check): lo commiteado y lo que
// aún no, desde el rojo, más los archivos nuevos sin seguimiento. Es el lazo del worker; la
// verificación vuelve a correrla sobre el commit en el repo principal.
func SupresionesEnCopia(dir, rojo string, ch config.Check) ResultadoCheck {
	if ch.Omitir != "" {
		return ResultadoCheck{Nombre: ch.Nombre, Resultado: CheckOmitido, Motivo: ch.Omitir}
	}
	res := ResultadoCheck{Nombre: ch.Nombre, Cmd: CmdSupresiones, Resultado: CheckPaso}
	diff, err := Git(dir, "diff", "-U0", "-M", "--no-color", "--no-ext-diff", rojo)
	if err != nil {
		return ResultadoCheck{Nombre: ch.Nombre, Resultado: CheckOmitido, Motivo: "git diff en la copia falló; la verificación la corre igual"}
	}
	hs := SupresionesEn(diff, ch.Marcas())
	if nuevos, err := Git(dir, "ls-files", "--others", "--exclude-standard", "-z"); err == nil {
		for _, f := range strings.Split(nuevos, "\x00") {
			raw, err := leerSeguro(dir, f, MaxReporte)
			if f == "" || err != nil {
				continue
			}
			var b strings.Builder
			b.WriteString("+++ b/" + f + "\n@@ -0,0 +1 @@\n")
			for _, l := range strings.Split(string(raw), "\n") {
				b.WriteString("+" + l + "\n")
			}
			hs = append(hs, SupresionesEn(b.String(), ch.Marcas())...)
		}
	}
	if len(hs) > 0 {
		res.Exit, res.Resultado, res.Salida = 1, CheckFallo, primeras(strings.Join(hs, "\n"), LineasCheck)
	}
	return res
}
