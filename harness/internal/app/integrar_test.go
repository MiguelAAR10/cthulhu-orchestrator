package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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

func TestCerrarWorkerNombreSinSesionNoIntegra(t *testing.T) {
	p, _, e := proyectoDePrueba(t, "")
	lanzar(t, p)
	commitEnClon(t, e.Worktree, "v01.txt", "ok")
	if _, err := p.CerrarWorker(context.Background(), "worker-nadie-s01"); err == nil {
		t.Fatal("a name that is no session must be refused")
	}
	e, _ = p.Slices().Load("01")
	if e.Fase == slice.Verificado || len(filasIntegracion(t, p)) != 0 {
		t.Fatalf("nothing may change: fase %s, rows %+v", e.Fase, filasIntegracion(t, p))
	}
}

// H13: an unreadable slice state is not a finished slice: the close fails and the pane stays.
func TestCerrarWorkerEstadoIlegibleNoCierra(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq2")
	p, _, e := proyectoDePrueba(t, "")
	lanzar(t, p)
	commitEnClon(t, e.Worktree, "v01.txt", "ok")
	estado := filepath.Join(p.Dir, "slices", "01.json")
	if err := os.WriteFile(estado, []byte("{roto"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := p.CerrarWorker(context.Background(), "worker-claude-s01"); err == nil {
		t.Fatal("an unreadable slice state must refuse the close")
	}
	// A slice that is really closed (no state file) still has nothing to integrate.
	os.Remove(estado)
	if _, err := p.CerrarWorker(context.Background(), "worker-claude-s01"); err != nil {
		t.Fatalf("closed slice: %v", err)
	}
}

// H27: the relevo and the worker contract say when memory could not be read, instead of
// silently handing a session less memory than exists.
func TestRelevoYContratoAvisanMemoriaIncompleta(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq2")
	p, _, e := proyectoDePrueba(t, "")
	os.WriteFile(p.MemPath(), []byte(`{"id":"m1","imp":"alta","cat":"decision","fuente":"atena","texto":"pago con tarjeta"}`+"\n{roto\n"), 0o644)
	rv := p.DatosRelevo("orquestador")
	if !strings.Contains(rv.Memoria, "memoria.jsonl:2") || len(rv.Decisiones) != 1 {
		t.Fatalf("relevo: memoria %q decisiones %v", rv.Memoria, rv.Decisiones)
	}
	if md := rv.Markdown("hoy", ""); !strings.Contains(md, "## Memoria incompleta") {
		t.Fatalf("relevo markdown:\n%s", md)
	}
	c, err := p.Contrato(e.NN, "worker-claude-s01", "x", "", "10 min")
	if err != nil || !strings.Contains(c, "memoria.jsonl:2") || !strings.Contains(c, "pago con tarjeta") {
		t.Fatalf("contrato (%v):\n%s", err, c)
	}
}

// orq keeps 1.0.0: with a bad memory line the relevo carries no memory and no warning.
func TestRelevoOrqMemoriaRotaComo100(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq")
	p, _, _ := proyectoDePrueba(t, "")
	os.WriteFile(p.MemPath(), []byte(`{"id":"m1","imp":"alta","cat":"decision","fuente":"atena","texto":"pago con tarjeta"}`+"\n{roto\n"), 0o644)
	rv := p.DatosRelevo("orquestador")
	if rv.Memoria != "" || len(rv.Decisiones) != 0 || strings.Contains(rv.Markdown("hoy", ""), "Memoria incompleta") {
		t.Fatalf("orq relevo: %+v", rv)
	}
}
