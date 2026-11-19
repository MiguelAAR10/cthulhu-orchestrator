package app

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"orq/internal/slice"
)

func TestPausaNiegaLanzarYConservaClones(t *testing.T) {
	p, term, e1 := proyectoDePrueba(t, "")
	r := p.Slices()
	if _, err := r.AbrirHistoria("h", "H", time.Now()); err != nil {
		t.Fatal(err)
	}
	e2, _, err := r.Abrir("02", "dos", "tests/s01.sh", "sh tests/s01.sh", slice.Opciones{SinConteo: "prueba", Historia: "h"}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	aprobarTarjeta(t, p, "02")
	ctx := context.Background()
	ped := PedidoWorker{Slice: "02", Agente: "claude", Alcance: "x", Presupuesto: "10 min"}
	if err := r.PausarHistoria("h", true, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, _, err := p.LanzarWorker(ctx, ped); err == nil || !strings.Contains(err.Error(), "orq historia reanudar h") {
		t.Fatalf("paused story launched: %v", err)
	}
	if n := len(term.Ops("crear")); n != 0 {
		t.Fatalf("a paused launch opened %d panes", n)
	}
	if _, err := os.Stat(e2.Worktree); err != nil {
		t.Fatalf("the clone must stay: %v", err)
	}
	if e, _ := r.Load("02"); e.Fase != slice.Implementando || e.Escritor != "" {
		t.Fatalf("state changed: %s %q", e.Fase, e.Escritor)
	}
	r.PausarHistoria("h", false, time.Now())
	r.PausarTodo(true, "almuerzo", time.Now())
	if _, _, err := p.LanzarWorker(ctx, PedidoWorker{Slice: e1.NN, Agente: "claude", Alcance: "x", Presupuesto: "10 min"}); err == nil || !strings.Contains(err.Error(), "orq reanudar") {
		t.Fatalf("global pause launched: %v", err)
	}
	r.PausarTodo(false, "", time.Now())
	if _, _, err := p.LanzarWorker(ctx, ped); err != nil {
		t.Fatalf("resumed: %v", err)
	}
}
