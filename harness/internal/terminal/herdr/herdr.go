// Package herdr es el adaptador de herdr del puerto terminal.Driver.
package herdr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"orq/internal/terminal"
)

// Tiempos máximos por llamada. Arrancar un agente tarda más que el resto.
const (
	LimiteLlamada  = 15 * time.Second
	LimiteArranque = 120 * time.Second
	esperaListoMS  = "90000" // lo que herdr espera a que el agente esté listo
)

// Driver habla con herdr por su CLI.
type Driver struct {
	// Ejecutar corre `herdr args...` dentro del plazo de ctx. Se reemplaza en pruebas.
	Ejecutar func(ctx context.Context, args ...string) ([]byte, error)
	// Entorno lee una variable; se reemplaza en pruebas.
	Entorno func(string) string
}

// Nuevo devuelve el driver contra el herdr real.
func Nuevo() *Driver { return &Driver{Ejecutar: ejecutar, Entorno: os.Getenv} }

func ejecutar(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "herdr", args...)
	cmd.WaitDelay = 2 * time.Second
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return out, fmt.Errorf("%w: herdr %s", terminal.ErrDesconocido, strings.Join(args, " "))
	}
	if err != nil {
		return out, fmt.Errorf("herdr %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return out, nil
}

func (d *Driver) correr(ctx context.Context, limite time.Duration, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, limite)
	defer cancel()
	out, err := d.Ejecutar(ctx, args...)
	if err != nil && ctx.Err() != nil && !errors.Is(err, terminal.ErrDesconocido) {
		err = fmt.Errorf("%w: herdr %s", terminal.ErrDesconocido, strings.Join(args, " "))
	}
	return out, err
}

func (d *Driver) Nombre() string { return "herdr" }

// Dentro usa la única detección confiable: las variables que herdr pone en sus paneles.
// `herdr pane current` no sirve: devuelve el panel enfocado de otra sesión.
func (d *Driver) Dentro() bool {
	return d.Entorno("HERDR_ENV") == "1" && d.Entorno("HERDR_PANE_ID") != ""
}

func (d *Driver) PanelActual() string { return d.Entorno("HERDR_PANE_ID") }

func envArgs(env map[string]string) []string {
	ks := make([]string, 0, len(env))
	for k := range env {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	var a []string
	for _, k := range ks {
		a = append(a, "--env", k+"="+env[k])
	}
	return a
}

// CrearPanel abre una pestaña con nombre (cada rol vive en su pestaña tipada) o divide al lado de otro panel.
func (d *Driver) CrearPanel(ctx context.Context, p terminal.PanelNuevo) (terminal.Panel, error) {
	if p.Junto != "" {
		args := append([]string{"pane", "split", p.Junto, "--direction", "right", "--cwd", p.Cwd, "--no-focus"}, envArgs(p.Env)...)
		out, err := d.correr(ctx, LimiteLlamada, args...)
		if err != nil {
			return terminal.Panel{}, err
		}
		var r struct {
			Result struct {
				Pane struct {
					PaneID string `json:"pane_id"`
					TabID  string `json:"tab_id"`
				} `json:"pane"`
			} `json:"result"`
		}
		if json.Unmarshal(out, &r) != nil || r.Result.Pane.PaneID == "" {
			return terminal.Panel{}, fmt.Errorf("herdr pane split: respuesta inesperada: %s", strings.TrimSpace(string(out)))
		}
		return terminal.Panel{ID: r.Result.Pane.PaneID, Pestana: r.Result.Pane.TabID}, nil
	}
	args := []string{"tab", "create", "--label", p.Etiqueta, "--cwd", p.Cwd}
	// Sin --workspace herdr usa el espacio con foco: la pestaña caería en el proyecto que el Owner mira.
	if ws := d.Entorno("HERDR_WORKSPACE_ID"); ws != "" {
		args = append(args, "--workspace", ws)
	}
	out, err := d.correr(ctx, LimiteLlamada, append(args, envArgs(p.Env)...)...)
	if err != nil {
		return terminal.Panel{}, err
	}
	var r struct {
		Result struct {
			Tab struct {
				TabID string `json:"tab_id"`
			} `json:"tab"`
		} `json:"result"`
	}
	if json.Unmarshal(out, &r) != nil || r.Result.Tab.TabID == "" {
		return terminal.Panel{}, fmt.Errorf("herdr tab create: respuesta inesperada: %s", strings.TrimSpace(string(out)))
	}
	tab := r.Result.Tab.TabID
	out, err = d.correr(ctx, LimiteLlamada, "pane", "list")
	if err != nil {
		return terminal.Panel{Pestana: tab}, err
	}
	var l struct {
		Result struct {
			Panes []struct {
				PaneID string `json:"pane_id"`
				TabID  string `json:"tab_id"`
			} `json:"panes"`
		} `json:"result"`
	}
	if err := json.Unmarshal(out, &l); err != nil {
		return terminal.Panel{Pestana: tab}, fmt.Errorf("herdr pane list: respuesta inesperada: %v", err)
	}
	for _, x := range l.Result.Panes {
		if x.TabID == tab {
			return terminal.Panel{ID: x.PaneID, Pestana: tab}, nil
		}
	}
	return terminal.Panel{Pestana: tab}, fmt.Errorf("herdr: la pestaña %s no tiene panel", tab)
}

// Arrancar usa `herdr agent start`: el panel debe estar en su prompt. Las variables ya van en el panel.
func (d *Driver) Arrancar(ctx context.Context, a terminal.Arranque) error {
	args := []string{"agent", "start", a.Nombre, "--kind", a.Kind, "--pane", a.Panel, "--timeout", esperaListoMS}
	if len(a.Args) > 0 {
		args = append(append(args, "--"), a.Args...)
	}
	_, err := d.correr(ctx, LimiteArranque, args...)
	return err
}

// Avisar entrega el texto sin esperar: quien envía sigue con su trabajo.
func (d *Driver) Avisar(ctx context.Context, destino, texto string) error {
	_, err := d.correr(ctx, LimiteLlamada, "agent", "prompt", destino, texto)
	return err
}

// Estado pregunta por el agente. Solo «no existe» dicho por herdr cuenta como ausente.
func (d *Driver) Estado(ctx context.Context, nombre string) (terminal.Vida, error) {
	out, err := d.correr(ctx, LimiteLlamada, "agent", "get", nombre)
	switch {
	case err == nil:
		return terminal.Vivo, nil
	case errors.Is(err, terminal.ErrDesconocido):
		return terminal.Desconocida, err
	case noExiste(err.Error() + string(out)):
		return terminal.Ausente, nil
	}
	return terminal.Desconocida, err
}

func noExiste(s string) bool {
	s = strings.ToLower(s)
	return strings.Contains(s, "not found") || strings.Contains(s, "not_found") || strings.Contains(s, "no such") || strings.Contains(s, "unknown agent")
}

func (d *Driver) Renombrar(ctx context.Context, destino, nombre string) error {
	_, err := d.correr(ctx, LimiteLlamada, "agent", "rename", destino, nombre)
	return err
}

// Cerrar cierra el panel de una sesión terminada; uno que ya no existe no es un error.
func (d *Driver) Cerrar(ctx context.Context, panel string) error {
	out, err := d.correr(ctx, LimiteLlamada, "pane", "close", panel)
	if err != nil && !errors.Is(err, terminal.ErrDesconocido) && noExiste(err.Error()+string(out)) {
		return nil
	}
	return err
}
