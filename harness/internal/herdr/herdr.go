// Package herdr es el driver de herdr: crea paneles, arranca agentes y les entrega mensajes.
// Orca tendrá su propio driver con las mismas funciones cuando herdr esté probado.
package herdr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Inside es la única detección confiable: la variable que herdr pone en sus paneles.
// `herdr pane current` no sirve: devuelve el panel enfocado de otra sesión.
func Inside() bool { return os.Getenv("HERDR_ENV") == "1" && os.Getenv("HERDR_PANE_ID") != "" }

func CurrentPane() string { return os.Getenv("HERDR_PANE_ID") }

// ErrTimeout: herdr no respondió a tiempo; el resultado es desconocido, no "falló".
var ErrTimeout = errors.New("herdr no respondió a tiempo")

// Limite por llamada; arrancar un agente tarda más.
func limite(args []string) time.Duration {
	if len(args) > 1 && args[0] == "agent" && args[1] == "start" {
		return 120 * time.Second
	}
	return 15 * time.Second
}

// Run ejecuta herdr con tiempo máximo; se reemplaza en pruebas.
var Run = func(args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), limite(args))
	defer cancel()
	out, err := exec.CommandContext(ctx, "herdr", args...).CombinedOutput()
	if ctx.Err() != nil {
		return out, fmt.Errorf("%w: herdr %s", ErrTimeout, strings.Join(args, " "))
	}
	if err != nil {
		return out, fmt.Errorf("herdr %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return out, nil
}

// Split abre un panel hermano sin quitar el foco al Owner y devuelve su id.
func Split(cwd string, env map[string]string) (string, error) {
	args := []string{"pane", "split", "--current", "--direction", "right", "--cwd", cwd, "--no-focus"}
	for k, v := range env {
		args = append(args, "--env", k+"="+v)
	}
	out, err := Run(args...)
	if err != nil {
		return "", err
	}
	var r struct {
		Result struct {
			Pane struct {
				PaneID string `json:"pane_id"`
			} `json:"pane"`
		} `json:"result"`
	}
	if err := json.Unmarshal(out, &r); err != nil || r.Result.Pane.PaneID == "" {
		return "", fmt.Errorf("herdr pane split: respuesta inesperada: %s", strings.TrimSpace(string(out)))
	}
	return r.Result.Pane.PaneID, nil
}

func Start(name, kind, pane string, agentArgs []string) error {
	args := []string{"agent", "start", name, "--kind", kind, "--pane", pane, "--timeout", "90000"}
	if len(agentArgs) > 0 {
		args = append(append(args, "--"), agentArgs...)
	}
	_, err := Run(args...)
	return err
}

func Rename(target, name string) error { _, err := Run("agent", "rename", target, name); return err }

// Prompt entrega el texto sin esperar: quien envía sigue con su trabajo.
func Prompt(target, text string) error { _, err := Run("agent", "prompt", target, text); return err }

// Tab abre una pestaña con nombre y devuelve el panel raíz: cada rol vive en su pestaña tipada.
func Tab(label, cwd string, env map[string]string) (tabID, paneID string, err error) {
	args := []string{"tab", "create", "--label", label, "--cwd", cwd}
	// Sin --workspace herdr usa el espacio con foco: la pestaña caería en el proyecto que el Owner mira.
	if ws := os.Getenv("HERDR_WORKSPACE_ID"); ws != "" {
		args = append(args, "--workspace", ws)
	}
	for k, v := range env {
		args = append(args, "--env", k+"="+v)
	}
	out, err := Run(args...)
	if err != nil {
		return "", "", err
	}
	var r struct {
		Result struct {
			Tab struct {
				TabID string `json:"tab_id"`
			} `json:"tab"`
		} `json:"result"`
	}
	if json.Unmarshal(out, &r) != nil || r.Result.Tab.TabID == "" {
		return "", "", fmt.Errorf("herdr tab create: respuesta inesperada: %s", strings.TrimSpace(string(out)))
	}
	tabID = r.Result.Tab.TabID
	out, err = Run("pane", "list")
	if err != nil {
		return tabID, "", err
	}
	var l struct {
		Result struct {
			Panes []struct {
				PaneID string `json:"pane_id"`
				TabID  string `json:"tab_id"`
			} `json:"panes"`
		} `json:"result"`
	}
	json.Unmarshal(out, &l)
	for _, p := range l.Result.Panes {
		if p.TabID == tabID {
			return tabID, p.PaneID, nil
		}
	}
	return tabID, "", fmt.Errorf("herdr: la pestaña %s no tiene panel", tabID)
}

// SplitDe abre un panel al lado de otro (no del actual): los workers de un slice comparten pestaña.
func SplitDe(pane, cwd string, env map[string]string) (string, error) {
	args := []string{"pane", "split", pane, "--direction", "right", "--cwd", cwd, "--no-focus"}
	for k, v := range env {
		args = append(args, "--env", k+"="+v)
	}
	out, err := Run(args...)
	if err != nil {
		return "", err
	}
	var r struct {
		Result struct {
			Pane struct {
				PaneID string `json:"pane_id"`
			} `json:"pane"`
		} `json:"result"`
	}
	if json.Unmarshal(out, &r) != nil || r.Result.Pane.PaneID == "" {
		return "", fmt.Errorf("herdr pane split: respuesta inesperada: %s", strings.TrimSpace(string(out)))
	}
	return r.Result.Pane.PaneID, nil
}

// ClosePane cierra el panel de una sesión terminada: higiene, no se dejan paneles vivos.
func ClosePane(pane string) error { _, err := Run("pane", "close", pane); return err }
