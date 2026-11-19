package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"orq/internal/config"
	"orq/internal/perfil"
	"orq/internal/sandbox"
	"orq/internal/slice"
)

// ErrSinChecks: el proyecto no declaró checks en .orq/config.json.
var ErrSinChecks = errors.New("sin checks en .orq/config.json: corre `orq init` para detectarlos, o escríbelos ahí")

// Check corre los checks del proyecto en el clon o checkout de cwd, sin caja: es el lazo del worker
// antes de pedir verificación. Un clon no tiene .orq: los checks salen del checkout principal.
func Check(cwd string) ([]slice.ResultadoCheck, error) {
	dir, rojo, cs, err := checksDe(cwd)
	if err != nil {
		return nil, err
	}
	rs, err := slice.CorrerChecks(cs, func(cmd string) (string, int, error) {
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
	})
	if err != nil {
		return rs, err
	}
	for _, c := range cs {
		// Fuera de un clon de slice no hay rojo contra el que comparar: la guardia corre al verificar.
		if c.Interno() && rojo != "" {
			rs = append(rs, slice.SupresionesEnCopia(dir, rojo, c))
		}
	}
	return rs, nil
}

// CheckCaja es un check del proyecto y lo que la caja de orq slice verificar ve de sus herramientas
// y, en checks.e2e, de los navegadores de Playwright.
type CheckCaja struct {
	config.Check
	Herramientas []sandbox.Herramienta
	Navegadores  *sandbox.Herramienta
}

// ChecksCaja dice qué herramientas nombra cada check del proyecto y qué montaría la caja para ellas.
// sinE2E dice por qué un proyecto web no tiene checks.e2e ("" si lo tiene o no es web).
func ChecksCaja(cwd string) (out []CheckCaja, sinE2E string, err error) {
	root, _, cs, err := checksDe(cwd)
	if err != nil {
		return nil, "", err
	}
	tiene := false
	for _, c := range cs {
		if c.Interno() || c.Cmd == "" {
			continue
		}
		cc := CheckCaja{Check: c, Herramientas: sandbox.Herramientas(c.Ejecutable())}
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

// checksDe ubica el repo de cwd, el rojo si es el clon de un slice abierto, y los checks del proyecto.
func checksDe(cwd string) (dir, rojo string, cs []config.Check, err error) {
	out, err := exec.Command("git", "-C", cwd, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", "", nil, errors.New("orq check: no estás dentro de un repositorio git")
	}
	dir = filepath.Clean(strings.TrimSpace(string(out)))
	root := dir
	if m, ok := slice.LeerMarca(dir); ok && m.Root != "" {
		root = m.Root
		if e, err := (slice.Repo{Root: root, OrqDir: filepath.Join(root, ".orq")}).Load(m.Slice); err == nil && e != nil {
			rojo = e.SHARojo
		}
	}
	cfg, err := config.Cargar(root)
	if err != nil {
		return "", "", nil, fmt.Errorf("orq check: %w", err)
	}
	cs = cfg.ChecksEnOrden()
	if len(cs) == 0 {
		return "", "", nil, ErrSinChecks
	}
	return dir, rojo, cs, nil
}
