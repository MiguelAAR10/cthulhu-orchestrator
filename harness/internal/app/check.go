package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"orq/internal/config"
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

// CheckCaja es un check del proyecto y lo que la caja de orq slice verificar ve de sus herramientas.
type CheckCaja struct {
	config.Check
	Herramientas []sandbox.Herramienta
}

// ChecksCaja dice, sin correr nada, si cada check del proyecto puede correr en la caja.
func ChecksCaja(cwd string) ([]CheckCaja, error) {
	_, _, cs, err := checksDe(cwd)
	if err != nil {
		return nil, err
	}
	var out []CheckCaja
	for _, c := range cs {
		if c.Interno() || c.Cmd == "" {
			continue
		}
		out = append(out, CheckCaja{Check: c, Herramientas: sandbox.Herramientas(c.Cmd)})
	}
	return out, nil
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
