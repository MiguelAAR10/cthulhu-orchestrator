package app

import (
	"context"
	"testing"

	"orq/internal/diario"
	"orq/internal/slice"
)

func filasIntegracion(t *testing.T, p Proyecto) []diario.Evento {
	t.Helper()
	evs, err := diario.Leer(p.DiarioPath())
	if err != nil {
		t.Fatal(err)
	}
	var out []diario.Evento
	for _, ev := range evs {
		if ev.Tipo == "integracion" {
			out = append(out, ev)
		}
	}
	return out
}

func TestCerrarWorkerVerificaYAnota(t *testing.T) {
	p, _, e := proyectoDePrueba(t, "")
	lanzar(t, p)
	commitEnClon(t, e.Worktree, "v01.txt", "ok")
	if _, err := p.CerrarWorker(context.Background(), "worker-claude-s01"); err != nil {
		t.Fatal(err)
	}
	e, _ = p.Slices().Load("01")
	fs := filasIntegracion(t, p)
	if e.Fase != slice.Verificado || len(fs) != 1 || fs[0].Paso != "worker" || fs[0].Actor != "worker-claude-s01" || fs[0].Resultado != "ok" || fs[0].Datos["sha"] != e.Recibo.Head {
		t.Fatalf("fase %s, rows %+v", e.Fase, fs)
	}
	if _, err := p.CerrarWorker(context.Background(), "worker-claude-s01"); err != nil || len(filasIntegracion(t, p)) != 1 {
		t.Fatalf("a repeated close integrates nothing more: %v", err)
	}
}

func TestCerrarWorkerFalloNoCierra(t *testing.T) {
	p, _, e := proyectoDePrueba(t, "")
	lanzar(t, p)
	commitEnClon(t, e.Worktree, "v01.txt", "mal")
	if _, err := p.CerrarWorker(context.Background(), "worker-claude-s01"); err == nil {
		t.Fatal("a failing exam must refuse the close")
	}
	e, _ = p.Slices().Load("01")
	fs := filasIntegracion(t, p)
	if e.Fase == slice.Verificado || len(fs) != 1 || fs[0].Resultado != "fallo" || fs[0].Detalle == "" {
		t.Fatalf("fase %s, rows %+v", e.Fase, fs)
	}
}
