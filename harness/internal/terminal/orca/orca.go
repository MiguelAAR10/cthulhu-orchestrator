// Package orca es el adaptador de Orca del puerto terminal.Driver, sobre la CLI `orca`
// (esquema de comandos: `orca agent-context --json`).
//
// Correspondencia:
//
//	CrearPanel  → orca terminal create --worktree path:<cwd> --title <etiqueta>   (o terminal split)
//	Arrancar    → orca terminal send --text "cd <cwd> && env K=V <agente> args" --enter,
//	              terminal rename --title <nombre>, terminal wait --for tui-idle
//	Avisar      → orca terminal send --text <línea> --enter
//	Estado      → orca terminal list: existe una terminal con ese título
//	Cerrar      → orca terminal close --terminal <handle>
//
// Orca distingue un fallo de un resultado ambiguo y da un id de reintento; aquí lo ambiguo es
// terminal.ErrDesconocido y nunca se reintenta solo.
package orca

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"

	"orq/internal/terminal"
)

const (
	LimiteLlamada  = 20 * time.Second
	LimiteArranque = 120 * time.Second
	esperaListoMS  = "90000"
)

// Driver habla con Orca por su CLI.
type Driver struct {
	// Ejecutar corre `orca args...` dentro del plazo de ctx y devuelve su salida. Se reemplaza en pruebas.
	Ejecutar func(ctx context.Context, args ...string) ([]byte, error)

	una    sync.Once
	actual string
}

// Binario es la CLI de Orca: ORQ_ORCA_BIN, o `orca`, o `orca-ide` en el PATH.
func Binario() (string, error) {
	if p := os.Getenv("ORQ_ORCA_BIN"); p != "" {
		return p, nil
	}
	for _, n := range []string{"orca", "orca-ide"} {
		if p, err := exec.LookPath(n); err == nil {
			return p, nil
		}
	}
	return "", errors.New("no está la CLI de Orca (orca u orca-ide) en el PATH; fija ORQ_ORCA_BIN o usa terminal=herdr en `orq config ver`")
}

// Nuevo devuelve el driver contra la CLI real.
func Nuevo() *Driver { return &Driver{Ejecutar: ejecutar} }

func ejecutar(ctx context.Context, args ...string) ([]byte, error) {
	bin, err := Binario()
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.WaitDelay = 2 * time.Second
	out, err := cmd.Output() // el sobre JSON va por stdout también cuando falla
	if ctx.Err() != nil {
		return out, fmt.Errorf("%w: orca %s", terminal.ErrDesconocido, strings.Join(args, " "))
	}
	var ee *exec.ExitError
	if err != nil && !errors.As(err, &ee) {
		return out, fmt.Errorf("orca %s: %v", strings.Join(args, " "), err)
	}
	return out, nil
}

// sobre es la envoltura de toda respuesta --json de Orca.
type sobre struct {
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// llamar corre el comando con --json y devuelve result. Un error ambiguo es ErrDesconocido.
func (d *Driver) llamar(ctx context.Context, limite time.Duration, args ...string) (json.RawMessage, error) {
	ctx, cancel := context.WithTimeout(ctx, limite)
	defer cancel()
	out, err := d.Ejecutar(ctx, append(args, "--json")...)
	if err != nil {
		if ctx.Err() != nil && !errors.Is(err, terminal.ErrDesconocido) {
			err = fmt.Errorf("%w: orca %s", terminal.ErrDesconocido, strings.Join(args, " "))
		}
		return nil, err
	}
	var s sobre
	if err := json.Unmarshal(out, &s); err != nil {
		return nil, fmt.Errorf("orca %s: respuesta inesperada: %s", strings.Join(args, " "), recortar(string(out)))
	}
	if s.OK {
		return s.Result, nil
	}
	code, msg := "", recortar(string(out))
	if s.Error != nil {
		code, msg = s.Error.Code, s.Error.Message
	}
	e := fmt.Errorf("orca %s: %s: %s", strings.Join(args[:min(2, len(args))], " "), code, msg)
	switch {
	case ambiguo(code, msg):
		return nil, fmt.Errorf("%w: %v", terminal.ErrDesconocido, e)
	case noExiste(code, msg):
		return nil, fmt.Errorf("%w: %v", terminal.ErrNoExiste, e)
	}
	return nil, e
}

func recortar(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 300 {
		return s[:300] + "…"
	}
	return s
}

// ambiguo: Orca ofrece un id de reintento cuando no sabe si lo pedido ocurrió.
func ambiguo(code, msg string) bool {
	t := strings.ToLower(code + " " + msg)
	return strings.Contains(t, "timeout") || strings.Contains(t, "timed out") || strings.Contains(t, "--retry-") || strings.Contains(t, "ambiguous")
}

func noExiste(code, msg string) bool {
	t := strings.ToLower(code + " " + msg)
	return strings.Contains(t, "not_found") || strings.Contains(t, "not found") || strings.Contains(t, "no such") || strings.Contains(t, "unknown terminal")
}

// handleEn busca el identificador de terminal (term_…) en un resultado cuyo esquema no se fija aquí.
func handleEn(v any) string {
	switch x := v.(type) {
	case string:
		if strings.HasPrefix(x, "term_") {
			return x
		}
	case map[string]any:
		for _, k := range []string{"handle", "terminal", "terminalHandle", "id"} { // primero los nombres probables
			if s, ok := x[k].(string); ok && strings.HasPrefix(s, "term_") {
				return s
			}
		}
		ks := make([]string, 0, len(x))
		for k := range x {
			ks = append(ks, k)
		}
		sort.Strings(ks)
		for _, k := range ks {
			if h := handleEn(x[k]); h != "" {
				return h
			}
		}
	case []any:
		for _, e := range x {
			if h := handleEn(e); h != "" {
				return h
			}
		}
	}
	return ""
}

// porTitulo busca la terminal cuyo título es nombre y devuelve su handle.
func porTitulo(v any, nombre string) string {
	switch x := v.(type) {
	case map[string]any:
		for _, k := range []string{"title", "paneTitle", "name"} {
			if s, ok := x[k].(string); ok && s == nombre {
				if h := handleEn(x); h != "" {
					return h
				}
			}
		}
		for _, e := range x {
			if h := porTitulo(e, nombre); h != "" {
				return h
			}
		}
	case []any:
		for _, e := range x {
			if h := porTitulo(e, nombre); h != "" {
				return h
			}
		}
	}
	return ""
}

func (d *Driver) Nombre() string { return "orca" }

// PanelActual pregunta a Orca por la terminal de este proceso (una vez por proceso).
func (d *Driver) PanelActual() string {
	d.una.Do(func() {
		res, err := d.llamar(context.Background(), 5*time.Second, "terminal", "show")
		if err != nil {
			return
		}
		var v any
		if json.Unmarshal(res, &v) == nil {
			d.actual = handleEn(v)
		}
	})
	return d.actual
}

func (d *Driver) Dentro() bool { return d.PanelActual() != "" }

func (d *Driver) CrearPanel(ctx context.Context, p terminal.PanelNuevo) (terminal.Panel, error) {
	var args []string
	if p.Junto != "" {
		args = []string{"terminal", "split", "--terminal", p.Junto, "--direction", "horizontal"}
	} else {
		args = []string{"terminal", "create", "--worktree", "path:" + p.Cwd, "--title", p.Etiqueta}
	}
	res, err := d.llamar(ctx, LimiteLlamada, args...)
	if err != nil {
		return terminal.Panel{}, err
	}
	var v any
	if err := json.Unmarshal(res, &v); err != nil {
		return terminal.Panel{}, fmt.Errorf("orca %s: respuesta inesperada: %s", strings.Join(args[:2], " "), recortar(string(res)))
	}
	h := handleEn(v)
	if h == "" {
		return terminal.Panel{}, fmt.Errorf("orca %s: la respuesta no trae el handle de la terminal: %s", strings.Join(args[:2], " "), recortar(string(res)))
	}
	tab := h
	if p.Junto != "" {
		tab = p.Junto
	}
	return terminal.Panel{ID: h, Pestana: tab}, nil
}

// comillas deja un argumento como una sola palabra de sh.
func comillas(s string) string {
	if s != "" && strings.IndexFunc(s, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("-_./=:,@+", r))
	}) < 0 {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// Orden es la línea que se escribe en la terminal para arrancar el agente con su entorno.
func Orden(a terminal.Arranque) string {
	var b strings.Builder
	if a.Cwd != "" {
		b.WriteString("cd " + comillas(a.Cwd) + " && ")
	}
	ks := make([]string, 0, len(a.Env))
	for k := range a.Env {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	if len(ks) > 0 {
		b.WriteString("env")
		for _, k := range ks {
			b.WriteString(" " + comillas(k+"="+a.Env[k]))
		}
		b.WriteString(" ")
	}
	b.WriteString(comillas(a.Kind))
	for _, x := range a.Args {
		b.WriteString(" " + comillas(x))
	}
	return b.String()
}

func (d *Driver) Arrancar(ctx context.Context, a terminal.Arranque) error {
	ctx, cancel := context.WithTimeout(ctx, LimiteArranque)
	defer cancel()
	// El título va primero: si el envío queda ambiguo, Estado ya puede encontrar la terminal por nombre.
	if _, err := d.llamar(ctx, LimiteLlamada, "terminal", "rename", "--terminal", a.Panel, "--title", a.Nombre); err != nil {
		return err
	}
	if _, err := d.llamar(ctx, LimiteLlamada, "terminal", "send", "--terminal", a.Panel, "--text", Orden(a), "--enter"); err != nil {
		return err
	}
	_, err := d.llamar(ctx, LimiteArranque, "terminal", "wait", "--terminal", a.Panel, "--for", "tui-idle", "--timeout-ms", esperaListoMS)
	return err
}

// destino acepta un handle (term_…) o el nombre de un agente, que se busca por título.
func (d *Driver) destino(ctx context.Context, x string) (string, error) {
	if strings.HasPrefix(x, "term_") {
		return x, nil
	}
	res, err := d.llamar(ctx, LimiteLlamada, "terminal", "list")
	if err != nil {
		return "", err
	}
	var v any
	if err := json.Unmarshal(res, &v); err != nil {
		return "", fmt.Errorf("orca terminal list: respuesta inesperada: %s", recortar(string(res)))
	}
	if h := porTitulo(v, x); h != "" {
		return h, nil
	}
	return "", fmt.Errorf("%w: ninguna terminal de Orca se llama %s", terminal.ErrNoExiste, x)
}

func (d *Driver) Avisar(ctx context.Context, dest, texto string) error {
	h, err := d.destino(ctx, dest)
	if err != nil {
		return err
	}
	_, err = d.llamar(ctx, LimiteLlamada, "terminal", "send", "--terminal", h, "--text", texto, "--enter")
	return err
}

func (d *Driver) Estado(ctx context.Context, nombre string) (terminal.Vida, error) {
	_, err := d.destino(ctx, nombre)
	switch {
	case err == nil:
		return terminal.Vivo, nil
	case errors.Is(err, terminal.ErrNoExiste):
		return terminal.Ausente, nil
	}
	return terminal.Desconocida, err
}

func (d *Driver) Renombrar(ctx context.Context, dest, nombre string) error {
	h, err := d.destino(ctx, dest)
	if err != nil {
		return err
	}
	_, err = d.llamar(ctx, LimiteLlamada, "terminal", "rename", "--terminal", h, "--title", nombre)
	return err
}

func (d *Driver) Cerrar(ctx context.Context, panel string) error {
	_, err := d.llamar(ctx, LimiteLlamada, "terminal", "close", "--terminal", panel)
	if errors.Is(err, terminal.ErrNoExiste) {
		return nil
	}
	return err
}
