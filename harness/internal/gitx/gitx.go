// Package gitx corre git con tiempo máximo. Es el único lugar que ejecuta el binario git por orq.
package gitx

import (
	"bytes"
	"context"
	"errors"
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

// seguro son los ajustes con que orq corre git en el repo principal: sin monitor de archivos ni
// ganchos (un worker de una versión anterior pudo dejarlos en .git/config) y sin pedir credenciales.
var seguro = []string{"-c", "core.fsmonitor=false", "-c", "core.hooksPath=/dev/null", "-c", "protocol.ext.allow=never"}

// Principal corre git en el repo principal (de confianza) con los ajustes seguros.
func Principal(ctx context.Context, root string, args ...string) (string, error) {
	return Salida(ctx, root, append(append([]string{}, seguro...), args...)...)
}

// Dueno corre git en el checkout del dueño dejando activos sus ganchos: es la única orden que
// cambia sus archivos (el avance rápido al publicar).
func Dueno(ctx context.Context, root string, args ...string) (string, error) {
	return Salida(ctx, root, append([]string{"-c", "core.fsmonitor=false"}, args...)...)
}

// ConEntrada corre git en el repo principal dándole stdin (hash-object --stdin).
func ConEntrada(ctx context.Context, root string, stdin []byte, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, Limite)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", append(append([]string{}, seguro...), args...)...)
	cmd.Dir = root
	cmd.Stdin = bytes.NewReader(stdin)
	cmd.WaitDelay = 2 * time.Second
	var errb bytes.Buffer
	cmd.Stderr = &errb
	out, err := cmd.Output()
	if ctx.Err() != nil {
		return "", fmt.Errorf("git %s: sin respuesta en %s (resultado desconocido)", strings.Join(args, " "), Limite)
	}
	if err != nil {
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(errb.String()))
	}
	return strings.TrimSpace(string(out)), nil
}

// Archivo devuelve el comando `git archive <sha>` del repo principal, para conectar su salida a una caja.
func Archivo(ctx context.Context, root, sha string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "git", append(append([]string{}, seguro...), "archive", "--format=tar", sha)...)
	cmd.Dir = root
	cmd.WaitDelay = 2 * time.Second
	return cmd
}

// Codigo corre git en el repo principal y distingue «salió con 1» (respuesta: no) de un fallo.
func Codigo(ctx context.Context, root string, args ...string) (salida string, codigo int, err error) {
	ctx, cancel := context.WithTimeout(ctx, Limite)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", append(append([]string{}, seguro...), args...)...)
	cmd.Dir = root
	cmd.WaitDelay = 2 * time.Second
	out, errRun := cmd.CombinedOutput()
	s := strings.TrimSpace(string(out))
	if ctx.Err() != nil {
		return s, -1, fmt.Errorf("git %s: sin respuesta en %s (resultado desconocido)", strings.Join(args, " "), Limite)
	}
	var ee *exec.ExitError
	if errors.As(errRun, &ee) {
		return s, ee.ExitCode(), nil
	}
	if errRun != nil {
		return s, -1, fmt.Errorf("git %s: %v", strings.Join(args, " "), errRun)
	}
	return s, 0, nil
}

// Bytes devuelve la salida exacta de git en el repo principal (el contenido de un archivo en un commit).
func Bytes(ctx context.Context, root string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, Limite)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", append(append([]string{}, seguro...), args...)...)
	cmd.Dir = root
	cmd.WaitDelay = 2 * time.Second
	var errb bytes.Buffer
	cmd.Stderr = &errb
	out, err := cmd.Output()
	if ctx.Err() != nil {
		return nil, fmt.Errorf("git %s: sin respuesta en %s (resultado desconocido)", strings.Join(args, " "), Limite)
	}
	if err != nil {
		return nil, fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(errb.String()))
	}
	return out, nil
}
