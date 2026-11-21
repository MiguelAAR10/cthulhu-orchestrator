package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"orq/internal/config"
	"orq/internal/identidad"
	"orq/internal/perfil"
	"orq/internal/sandbox"
	"orq/internal/slice"
)

// ErrSinChecks: el proyecto no declaró checks en .orq/config.json.
var ErrSinChecks = errors.New("sin checks en .orq/config.json: corre `orq init` para detectarlos, o escríbelos ahí")

// Check corre los checks del proyecto en el clon o checkout de cwd, sin caja: es el lazo del worker
// antes de pedir verificación. Un clon no tiene .orq: los checks salen del checkout principal.
func Check(cwd string) ([]slice.ResultadoCheck, error) {
	f, err := fuenteDe(cwd)
	if err != nil {
		return nil, err
	}
	dir, rojo, cs := f.dir, f.rojo, f.cs
	rs, err := slice.CorrerChecks(cs, dir, correrLocal(dir))
	if err != nil {
		return rs, err
	}
	// Las carpetas con checks propios que el clon toca (alcance y cambios desde el rojo, commiteados o no),
	// dentro de cada carpeta: lo mismo que Verificar exigirá.
	if len(f.carpetas) > 0 {
		cambiados, err := cambiadosDesde(dir, rojo)
		if err != nil {
			return rs, err
		}
		for _, ca := range slice.CarpetasTocadas(f.carpetas, append(slices.Clone(f.alcance), cambiados...)) {
			sub := filepath.Join(dir, ca.Ruta)
			if fi, err := os.Stat(sub); err != nil || !fi.IsDir() {
				continue
			}
			de, err := slice.CorrerChecks(ca.Lista, sub, correrLocal(sub))
			for i := range de {
				de[i].Nombre = ca.Ruta + "/" + de[i].Nombre
			}
			rs = append(rs, de...)
			if err != nil {
				return rs, fmt.Errorf("%s: %w", ca.Ruta, err)
			}
		}
	}
	for _, c := range cs {
		// Fuera de un clon de slice no hay rojo contra el que comparar: la guardia corre al verificar.
		if c.Interno() && rojo != "" {
			rs = append(rs, slice.SupresionesEnCopia(dir, rojo, c))
		}
	}
	return rs, nil
}

// correrLocal corre un check en dir, sin caja, con el tiempo máximo de un examen.
func correrLocal(dir string) slice.Correr {
	return func(cmd string) (string, int, error) {
		ctx, cancel := context.WithTimeout(context.Background(), slice.TimeoutDefecto)
		defer cancel()
		c := exec.CommandContext(ctx, "sh", "-c", cmd)
		c.Dir = dir
		var buf bytes.Buffer
		c.Stdout, c.Stderr = &buf, &buf
		err := c.Run()
		var ee *exec.ExitError
		switch {
		case err == nil:
			return buf.String(), 0, nil
		case errors.As(err, &ee):
			return buf.String(), ee.ExitCode(), nil
		}
		return buf.String(), -1, err
	}
}

// cambiadosDesde lista lo que la carpeta de trabajo dir cambia desde base (el rojo; HEAD fuera de un
// clon de slice): lo commiteado, lo sin commit y lo nuevo que git no ignora.
func cambiadosDesde(dir, base string) ([]string, error) {
	if base == "" {
		base = "HEAD"
	}
	var fs []string
	for _, args := range [][]string{{"diff", "--name-only", "--no-renames", "-z", base}, {"ls-files", "--others", "--exclude-standard", "-z"}} {
		out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).Output()
		if err != nil {
			return nil, fmt.Errorf("orq check: git %s: %w", args[0], err)
		}
		for _, f := range strings.Split(string(out), "\x00") {
			if f != "" {
				fs = append(fs, f)
			}
		}
	}
	return fs, nil
}

// CheckCaja es un check del proyecto y lo que la caja de orq slice verificar ve de sus herramientas
// y, en checks.e2e, de los navegadores de Playwright.
type CheckCaja struct {
	config.Check
	Herramientas []sandbox.Herramienta
	Navegadores  *sandbox.Herramienta
	// Solo orq2, en un check con `uv run`: NoCorreUv es la negativa estática ("" si no la hay);
	// si no, SondaUv es la sonda que corrió en una copia de HEAD con el .venv, LineaUv su primera
	// línea y OkUv si salió 0. ErrUv: la caja o la copia no arrancaron.
	NoCorreUv, SondaUv, LineaUv string
	VenvUv                      string // el .venv que la sonda montó solo lectura
	OkUv                        bool
	ErrUv                       error
}

// ChecksCaja dice qué herramientas nombra cada check del proyecto y qué montaría la caja para ellas.
// sinE2E dice por qué un proyecto web no tiene checks.e2e ("" si lo tiene o no es web).
func ChecksCaja(cwd string) (out []CheckCaja, sinE2E string, err error) {
	f, err := fuenteDe(cwd)
	if err != nil {
		return nil, "", err
	}
	root, cs := f.dir, f.cs
	tiene := false
	for _, c := range cs {
		if c.Interno() || c.Cmd == "" {
			continue
		}
		cc := CheckCaja{Check: c, Herramientas: sandbox.Herramientas(c.Ejecutable())}
		if identidad.EsOrq2() && strings.Contains(c.Cmd, "uv run") {
			sondarUv(&cc, principal(root))
		}
		if c.Nombre == config.E2E {
			tiene = true
			nav := sandbox.Navegadores()
			cc.Navegadores = &nav
		}
		out = append(out, cc)
	}
	if !tiene && slices.Contains(perfil.Detect(root).Senales, "web") {
		_, sinE2E = perfil.E2E(root)
		if sinE2E == "" {
			sinE2E = "hay playwright.config pero el bloque checks no declara e2e: corre `orq init` y añade la línea que sugiere"
		}
	}
	return out, sinE2E, nil
}

// fuente es de dónde salen los checks de cwd: su repo, el rojo y el alcance si es el clon de un slice
// abierto, y los checks del proyecto (los de la raíz y los de sus carpetas).
type fuente struct {
	dir, rojo string
	alcance   []string
	cs        []config.Check
	carpetas  []config.Carpeta
}

// fuenteDe ubica el repo de cwd, el rojo y el alcance si es el clon de un slice abierto, y los checks del proyecto.
func fuenteDe(cwd string) (fuente, error) {
	var f fuente
	out, err := exec.Command("git", "-C", cwd, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return f, errors.New("orq check: no estás dentro de un repositorio git")
	}
	f.dir = filepath.Clean(strings.TrimSpace(string(out)))
	root := f.dir
	if m, ok := slice.LeerMarca(f.dir); ok && m.Root != "" {
		root = m.Root
		if e, err := (slice.Repo{Root: root, OrqDir: filepath.Join(root, ".orq")}).Load(m.Slice); err == nil && e != nil {
			f.rojo, f.alcance = e.SHARojo, e.Alcance
		}
	}
	cfg, err := config.Cargar(root)
	if err != nil {
		return f, fmt.Errorf("orq check: %w", err)
	}
	f.cs, f.carpetas = cfg.ChecksEnOrden(), cfg.CarpetasEnOrden()
	if len(f.cs) == 0 && len(f.carpetas) == 0 {
		return f, ErrSinChecks
	}
	return f, nil
}

// principal es el repo principal de dir: el de la marca si dir es un clon de worker.
func principal(dir string) string {
	if m, ok := slice.LeerMarca(dir); ok && m.Root != "" {
		return m.Root
	}
	return dir
}

// sondarUv decide un check uv sin red: uv no puede sincronizar en la caja y el .venv del checkout
// principal entra solo lectura (nunca se sincroniza dentro). Si eso está, corre la sonda real.
func sondarUv(cc *CheckCaja, root string) {
	// La herramienta tras `uv run` sale del .venv: la prueba la sonda, no el PATH del host.
	uvs := slice.HerramientasUv(cc.Cmd)
	cc.Herramientas = slices.DeleteFunc(cc.Herramientas, func(h sandbox.Herramienta) bool { return slices.Contains(uvs, h.Nombre) })
	uv, _ := slice.TramosUv(cc.Cmd)
	for _, t := range uv {
		if !strings.Contains(t, "--no-sync") {
			cc.NoCorreUv = "uv sincroniza en la caja y no hay red; usa `uv run --offline --no-sync`"
			return
		}
	}
	if fi, err := os.Stat(filepath.Join(root, ".venv")); err != nil || !fi.IsDir() {
		cc.NoCorreUv = "falta .venv; corre `uv sync` en el checkout principal"
		return
	}
	cc.VenvUv = filepath.Join(root, ".venv")
	repo := slice.Repo{Root: root, OrqDir: filepath.Join(root, ".orq")}
	// Cada tramo `uv run` con herramienta tiene su sonda: la primera que falla decide; si todas
	// corren, se muestran juntas.
	var sondas, lineas []string
	for _, t := range uv {
		if slice.SondaUvDe(t) == "" {
			continue
		}
		cc.SondaUv, cc.LineaUv, cc.OkUv, cc.ErrUv = repo.SondarUv(t)
		if cc.ErrUv != nil || !cc.OkUv {
			return
		}
		sondas, lineas = append(sondas, cc.SondaUv), append(lineas, cc.LineaUv)
	}
	if len(sondas) == 0 {
		cc.SondaUv, cc.LineaUv, cc.OkUv, cc.ErrUv = repo.SondarUv(cc.Cmd)
		return
	}
	cc.SondaUv, cc.LineaUv = strings.Join(sondas, " && "), strings.Join(lineas, "; ")
}
