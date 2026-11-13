// Package gitx corre git con tiempo máximo. Es el único lugar que ejecuta el binario git por orq.
package gitx

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Limite corta un git colgado (credenciales, red, un hook que no vuelve).
const Limite = 60 * time.Second

// Salida corre `git args...` con dir como directorio de trabajo y devuelve la salida sin espacios finales.
func Salida(ctx context.Context, dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, Limite)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.WaitDelay = 2 * time.Second
	out, err := cmd.CombinedOutput()
	s := strings.TrimSpace(string(out))
	if ctx.Err() != nil {
		return s, fmt.Errorf("git %s: sin respuesta en %s (resultado desconocido)", strings.Join(args, " "), Limite)
	}
	if err != nil {
		return s, fmt.Errorf("git %s: %s", strings.Join(args, " "), s)
	}
	return s, nil
}
