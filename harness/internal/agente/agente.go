// Package agente sabe cómo se arranca cada CLI de agente (claude, codex): sus argumentos para una
// sesión interactiva y cómo hacerle una pregunta suelta sin sesión.
package agente

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"
)

// Args traduce modelo y esfuerzo a los argumentos de cada CLI.
func Args(kind, modelo, esfuerzo, nombre string) []string {
	if kind == "codex" {
		return []string{"-m", modelo, "-c", "model_reasoning_effort=" + esfuerzo}
	}
	return []string{"--model", modelo, "--effort", esfuerzo, "-n", nombre}
}

// ArgsWorker suma a un worker Codex lo que su tarea necesita sin pedir aprobación a cada paso:
// escribir en su copia de trabajo y en los directorios extra (el git común de un worktree enlazado)
// y red, para las pruebas contra la base de prueba local. Lo demás sigue pidiendo aprobación.
func ArgsWorker(kind, modelo, esfuerzo, nombre string, dirsExtra ...string) []string {
	args := Args(kind, modelo, esfuerzo, nombre)
	if kind != "codex" {
		return args
	}
	args = append(args, "-s", "workspace-write", "-a", "on-request",
		"-c", "sandbox_workspace_write.network_access=true")
	for _, d := range dirsExtra {
		if d != "" {
			args = append(args, "--add-dir", d)
		}
	}
	return args
}

// Entorno devuelve base con las variables de extra puestas encima (la última gana).
func Entorno(base []string, extra map[string]string) []string {
	out := make([]string, 0, len(base)+len(extra))
	for _, kv := range base {
		if k, _, ok := strings.Cut(kv, "="); ok {
			if _, pisa := extra[k]; pisa {
				continue
			}
		}
		out = append(out, kv)
	}
	ks := make([]string, 0, len(extra))
	for k := range extra {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	for _, k := range ks {
		out = append(out, k+"="+extra[k])
	}
	return out
}

// LimitePregunta corta una pregunta suelta colgada.
const LimitePregunta = 3 * time.Minute

// Preguntar hace una pregunta sin sesión a `claude -p` y devuelve el texto. entorno son las variables
// de cuenta configuradas (CLAUDE_CONFIG_DIR); se suman a las del proceso.
func Preguntar(ctx context.Context, modelo, prompt string, entorno map[string]string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, LimitePregunta)
	defer cancel()
	cmd := exec.CommandContext(ctx, "claude", "-p", "--model", modelo, "--output-format", "text", "--no-session-persistence")
	cmd.Stdin = strings.NewReader(prompt)
	cmd.Env = Entorno(os.Environ(), entorno)
	cmd.WaitDelay = 2 * time.Second
	out, err := cmd.Output()
	if ctx.Err() != nil {
		return "", fmt.Errorf("%s no respondió en %s", modelo, LimitePregunta)
	}
	if err != nil {
		return "", fmt.Errorf("%s: %v", modelo, err)
	}
	return string(out), nil
}
