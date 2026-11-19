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
	"orq/internal/slice"
)

// ErrSinChecks: el proyecto no declaró checks en .orq/config.json.
var ErrSinChecks = errors.New("sin checks en .orq/config.json: corre `orq init` para detectarlos, o escríbelos ahí")

// Check corre los checks del proyecto en el clon o checkout de cwd, sin caja: es el lazo del worker
// antes de pedir verificación. Un clon no tiene .orq: los checks salen del checkout principal.
func Check(cwd string) ([]slice.ResultadoCheck, error) {
	out, err := exec.Command("git", "-C", cwd, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return nil, errors.New("orq check: no estás dentro de un repositorio git")
	}
	dir := filepath.Clean(strings.TrimSpace(string(out)))
	root, rojo := dir, ""
	if m, ok := slice.LeerMarca(dir); ok && m.Root != "" {
		root = m.Root
		if e, err := (slice.Repo{Root: root, OrqDir: filepath.Join(root, ".orq")}).Load(m.Slice); err == nil && e != nil {
			rojo = e.SHARojo
		}
	}
	cfg, err := config.Cargar(root)
	if err != nil {
		return nil, fmt.Errorf("orq check: %w", err)
	}
	cs := cfg.ChecksEnOrden()
	if len(cs) == 0 {
		return nil, ErrSinChecks
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
