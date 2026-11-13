package herdr

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"orq/internal/terminal"
)

type guion struct {
	llamadas [][]string
	resp     func(args []string) ([]byte, error)
}

func (g *guion) driver(env map[string]string) *Driver {
	return &Driver{
		Ejecutar: func(_ context.Context, args ...string) ([]byte, error) {
			g.llamadas = append(g.llamadas, args)
			return g.resp(args)
		},
		Entorno: func(k string) string { return env[k] },
	}
}

var _ terminal.Driver = (*Driver)(nil)

func TestCrearPanelPestanaYDivision(t *testing.T) {
	g := &guion{resp: func(a []string) ([]byte, error) {
		switch a[0] + " " + a[1] {
		case "tab create":
			return []byte(`{"result":{"tab":{"tab_id":"w1:t9"}}}`), nil
		case "pane list":
			return []byte(`{"result":{"panes":[{"pane_id":"w1:p1","tab_id":"w1:t1"},{"pane_id":"w1:p7","tab_id":"w1:t9"}]}}`), nil
		case "pane split":
			return []byte(`{"result":{"pane":{"pane_id":"w1:p8","tab_id":"w1:t9"}}}`), nil
		}
		return nil, errors.New("inesperado")
	}}
	d := g.driver(map[string]string{"HERDR_ENV": "1", "HERDR_PANE_ID": "w1:p1", "HERDR_WORKSPACE_ID": "w1"})
	if !d.Dentro() || d.PanelActual() != "w1:p1" {
		t.Fatal("no detecta el panel")
	}
	p, err := d.CrearPanel(context.Background(), terminal.PanelNuevo{Etiqueta: "orq · x", Cwd: "/r", Env: map[string]string{"ORQ_ROL": "orquestador", "A": "1"}})
	if err != nil || p.ID != "w1:p7" || p.Pestana != "w1:t9" {
		t.Fatalf("%+v %v", p, err)
	}
	crear := strings.Join(g.llamadas[0], " ")
	for _, want := range []string{"--label orq · x", "--cwd /r", "--workspace w1", "--env A=1 --env ORQ_ROL=orquestador"} {
		if !strings.Contains(crear, want) {
			t.Errorf("falta %q en %q", want, crear)
		}
	}
	p, err = d.CrearPanel(context.Background(), terminal.PanelNuevo{Junto: "w1:p7", Cwd: "/r"})
	if err != nil || p.ID != "w1:p8" || p.Pestana != "w1:t9" || !slices.Contains(g.llamadas[2], "--no-focus") {
		t.Fatalf("%+v %v %v", p, err, g.llamadas[2])
	}
}

func TestArrancarAvisarEstadoCerrar(t *testing.T) {
	existe := true
	g := &guion{resp: func(a []string) ([]byte, error) {
		if a[0] == "agent" && a[1] == "get" && !existe {
			return []byte(`{"error":{"code":"agent_not_found"}}`), errors.New("herdr agent get: exit status 1")
		}
		if a[0] == "pane" && a[1] == "close" && !existe {
			return nil, errors.New("pane not found")
		}
		return []byte(`{}`), nil
	}}
	d := g.driver(nil)
	ctx := context.Background()
	if err := d.Arrancar(ctx, terminal.Arranque{Nombre: "w", Kind: "claude", Panel: "p1", Args: []string{"--model", "m"}}); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(g.llamadas[0], " "); got != "agent start w --kind claude --pane p1 --timeout 90000 -- --model m" {
		t.Fatalf("arranque: %q", got)
	}
	if v, err := d.Estado(ctx, "w"); v != terminal.Vivo || err != nil {
		t.Fatalf("%v %v", v, err)
	}
	existe = false
	if v, err := d.Estado(ctx, "w"); v != terminal.Ausente || err != nil {
		t.Fatalf("%v %v", v, err)
	}
	if err := d.Cerrar(ctx, "p1"); err != nil {
		t.Fatalf("cerrar un panel que no existe debe ser un no-op: %v", err)
	}
	if err := d.Avisar(ctx, "w", "hola"); err != nil || strings.Join(g.llamadas[len(g.llamadas)-1], " ") != "agent prompt w hola" {
		t.Fatalf("%v %v", err, g.llamadas)
	}
}

func TestTiempoVencidoEsDesconocido(t *testing.T) {
	d := &Driver{Entorno: func(string) string { return "" }, Ejecutar: func(ctx context.Context, _ ...string) ([]byte, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := d.Arrancar(ctx, terminal.Arranque{Nombre: "w", Kind: "claude", Panel: "p"}); !errors.Is(err, terminal.ErrDesconocido) {
		t.Fatalf("un vencimiento debe ser desconocido: %v", err)
	}
	ctx2, cancel2 := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel2()
	if v, err := d.Estado(ctx2, "w"); v != terminal.Desconocida || !errors.Is(err, terminal.ErrDesconocido) {
		t.Fatalf("%v %v", v, err)
	}
	// Un error que no dice «no existe» tampoco se toma como ausente.
	d.Ejecutar = func(context.Context, ...string) ([]byte, error) { return nil, errors.New("socket roto") }
	if v, err := d.Estado(context.Background(), "w"); v != terminal.Desconocida || err == nil {
		t.Fatalf("%v %v", v, err)
	}
}
