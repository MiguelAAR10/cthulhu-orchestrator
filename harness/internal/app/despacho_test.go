package app

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"orq/internal/canal"
	"orq/internal/diario"
	"orq/internal/plan"
	"orq/internal/sysinfo"
)

// planS01 is an S-01-like plan: T1 has no dependencies, T2 and T3 wait for T1.
func planS01(t *testing.T, p Proyecto, nn string) {
	t.Helper()
	pl := &plan.Plan{Slice: nn, Debatido: true, Tareas: []plan.Tarea{
		{ID: "T1", Zona: "back", Rutas: []string{"src"}, Dificultad: "baja", Estado: "pendiente", Que: "contract"},
		{ID: "T2", Zona: "canal", Rutas: []string{"canal"}, DependeDe: []string{"T1"}, Dificultad: "media", Estado: "pendiente"},
		{ID: "T3", Zona: "uso", Rutas: []string{"uso"}, DependeDe: []string{"T1"}, Dificultad: "alta", Estado: "pendiente"},
	}}
	if err := plan.Guardar(p.PlanPath(nn), pl); err != nil {
		t.Fatal(err)
	}
}

func tarea(t *testing.T, p Proyecto, nn, id string) plan.Tarea {
	t.Helper()
	pl, err := plan.Leer(p.PlanPath(nn))
	if err != nil {
		t.Fatal(err)
	}
	for _, x := range pl.Tareas {
		if x.ID == id {
			return x
		}
	}
	t.Fatalf("no task %s", id)
	return plan.Tarea{}
}

// Two `despachar --una-vez` runs on an approved slice with T1 ready give exactly one worker.
func TestDespacharUnaVezDosVecesUnWorker(t *testing.T) {
	p, term, e := proyectoDePrueba(t, "")
	planS01(t, p, e.NN)
	ctx := context.Background()
	ds, err := p.DespacharUnaVez(ctx, "claude")
	if err != nil {
		t.Fatalf("first run: %v %+v", err, ds)
	}
	if len(ds) != 1 || ds[0].Tarea != "T1" || ds[0].Worker != "worker-back-s01" || ds[0].Resultado != "lanzado" {
		t.Fatalf("first run: %+v", ds)
	}
	if x := tarea(t, p, e.NN, "T1"); x.Estado != "en_curso" || x.Worker != "worker-back-s01" {
		t.Fatalf("T1 = %+v", x)
	}
	ds, err = p.DespacharUnaVez(ctx, "claude")
	if err != nil || len(ds) != 0 {
		t.Fatalf("second run launches nothing: %v %+v", err, ds)
	}
	if c, a, v := len(term.Ops("crear")), len(term.Ops("arrancar")), len(term.Ops("avisar")); c != 1 || a != 1 || v != 1 {
		t.Fatalf("crear=%d arrancar=%d avisar=%d: %+v", c, a, v, term.Llamadas)
	}
	// The intent is recorded before the launch, and its result after.
	evs, _ := diario.Leer(p.DiarioPath())
	var got []string
	for _, ev := range evs {
		if ev.Tipo == "despacho" {
			got = append(got, ev.Paso+"="+ev.Resultado+":"+fmt.Sprint(ev.Datos["tarea"]))
		}
	}
	if strings.Join(got, " ") != "intento=:T1 resultado=lanzado:T1" {
		t.Fatalf("despacho events: %v", got)
	}
}

// A crash after the task was taken (intent recorded, plan saved) and before any panel: the re-run
// launches it once, and a third run launches nothing.
func TestDespacharTrasCaidaAntesDelPanel(t *testing.T) {
	p, term, e := proyectoDePrueba(t, "")
	planS01(t, p, e.NN)
	pl, _ := plan.Leer(p.PlanPath(e.NN))
	if _, err := plan.Tomar(pl, "T1", "worker-back-s01"); err != nil {
		t.Fatal(err)
	}
	plan.Guardar(p.PlanPath(e.NN), pl)
	p.anotar(diario.Evento{Tipo: "despacho", Paso: "intento", Slice: e.NN, Intento: e.Intento, Actor: "worker-back-s01", Datos: map[string]any{"tarea": "T1"}})
	ctx := context.Background()
	ds, err := p.DespacharUnaVez(ctx, "claude")
	if err != nil || len(ds) != 1 || ds[0].Tarea != "T1" || ds[0].Resultado != "lanzado" {
		t.Fatalf("re-run: %v %+v", err, ds)
	}
	if ds, err := p.DespacharUnaVez(ctx, "claude"); err != nil || len(ds) != 0 {
		t.Fatalf("third run: %v %+v", err, ds)
	}
	if c, a := len(term.Ops("crear")), len(term.Ops("arrancar")); c != 1 || a != 1 {
		t.Fatalf("crear=%d arrancar=%d", c, a)
	}
}

// A crash after the agent started and before its session and contract were recorded: the re-run
// adopts the live agent (no second panel, no second start) and delivers the contract once.
func TestDespacharTrasCaidaConAgenteVivo(t *testing.T) {
	p, term, e := proyectoDePrueba(t, "")
	planS01(t, p, e.NN)
	ctx := context.Background()
	if _, err := p.DespacharUnaVez(ctx, "claude"); err != nil {
		t.Fatal(err)
	}
	// Undo what came after the start: the session entry, the contract line and the despacho result.
	ses, _ := canal.LoadSesiones(p.SesPath())
	delete(ses, "worker-back-s01")
	canal.SaveSesiones(p.SesPath(), ses)
	evs, _ := diario.Leer(p.DiarioPath())
	os.Remove(p.DiarioPath())
	for _, ev := range evs {
		if strings.HasPrefix(ev.Paso, "contrato") || ev.Tipo == "despacho" && ev.Paso == "resultado" {
			continue
		}
		diario.Anotar(p.DiarioPath(), ev)
	}
	ds, err := p.DespacharUnaVez(ctx, "claude")
	if err != nil || len(ds) != 1 || ds[0].Resultado != "adoptado" {
		t.Fatalf("re-run: %v %+v", err, ds)
	}
	if c, a, v := len(term.Ops("crear")), len(term.Ops("arrancar")), len(term.Ops("avisar")); c != 1 || a != 1 || v != 2 {
		t.Fatalf("crear=%d arrancar=%d avisar=%d", c, a, v)
	}
	if ses, _ := canal.LoadSesiones(p.SesPath()); ses["worker-back-s01"].Pane == "" {
		t.Fatalf("session not restored: %+v", ses)
	}
	if ds, err := p.DespacharUnaVez(ctx, "claude"); err != nil || len(ds) != 0 {
		t.Fatalf("third run: %v %+v", err, ds)
	}
}

// A worker that got its contract and later closed is not relaunched: its task waits for `plan hecha`.
func TestDespacharNoRelanzaUnWorkerQueYaTrabajo(t *testing.T) {
	p, term, e := proyectoDePrueba(t, "")
	planS01(t, p, e.NN)
	ctx := context.Background()
	p.DespacharUnaVez(ctx, "claude")
	ses, _ := canal.LoadSesiones(p.SesPath())
	if _, err := p.CerrarSesiones(ctx, ses, func(k string) bool { return k == "worker-back-s01" }); err != nil {
		t.Fatal(err)
	}
	if ds, err := p.DespacharUnaVez(ctx, "claude"); err != nil || len(ds) != 0 {
		t.Fatalf("%v %+v", err, ds)
	}
	if a := len(term.Ops("arrancar")); a != 1 {
		t.Fatalf("arrancar=%d", a)
	}
}

// A paused project, a card still waiting for the Owner, or a plan not agreed: nothing is
// launched and no intent is written.
func TestDespacharRespetaPausaTarjetaYDebate(t *testing.T) {
	p, term, e := proyectoDePrueba(t, "")
	planS01(t, p, e.NN)
	ctx := context.Background()
	if err := p.Slices().PausarTodo(true, "prueba", time.Now()); err != nil {
		t.Fatal(err)
	}
	if ds, err := p.DespacharUnaVez(ctx, "claude"); err != nil || len(ds) != 0 {
		t.Fatalf("paused: %v %+v", err, ds)
	}
	p.Slices().PausarTodo(false, "", time.Now())
	pl, _ := plan.Leer(p.PlanPath(e.NN))
	pl.Debatido = false
	plan.Guardar(p.PlanPath(e.NN), pl)
	if ds, err := p.DespacharUnaVez(ctx, "claude"); err != nil || len(ds) != 0 {
		t.Fatalf("not agreed: %v %+v", err, ds)
	}
	if len(term.Ops("crear")) != 0 {
		t.Fatalf("%+v", term.Llamadas)
	}
	evs, _ := diario.Leer(p.DiarioPath())
	if slices.ContainsFunc(evs, func(ev diario.Evento) bool { return ev.Tipo == "despacho" }) {
		t.Fatal("an intent was written for nothing")
	}
}

// huellaDir hashes every file under dir: a read-only command leaves it identical.
func huellaDir(t *testing.T, dir string) string {
	t.Helper()
	h := sha256.New()
	filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, _ := d.Info()
		fmt.Fprintf(h, "%s %v %v\n", path, info.Mode(), info.ModTime().UnixNano())
		if !d.IsDir() {
			raw, _ := os.ReadFile(path)
			h.Write(raw)
		}
		return nil
	})
	return fmt.Sprintf("%x", h.Sum(nil))
}

// Pendientes names the approved slice with T1 ready and no worker, since its approval, and
// changes nothing.
func TestPendientesSoloLeeYNombraLaTareaSinWorker(t *testing.T) {
	p, term, e := proyectoDePrueba(t, "")
	planS01(t, p, e.NN)
	antes := huellaDir(t, p.Dir)
	ps, err := p.Pendientes()
	if err != nil {
		t.Fatal(err)
	}
	if huellaDir(t, p.Dir) != antes || len(term.Llamadas) != 0 {
		t.Fatal("pendientes wrote state or touched the terminal")
	}
	i := slices.IndexFunc(ps, func(x Pendiente) bool { return x.Tipo == PendFaltaLanzar })
	if i < 0 {
		t.Fatalf("%+v", ps)
	}
	x := ps[i]
	if x.Slice != e.NN || !slices.Equal(x.Tareas, []string{"T1"}) || x.Debe != "orquestador" || !x.Desde.Equal(e.Tarjeta.Aprobada) {
		t.Fatalf("%+v", x)
	}
	// Once dispatched, it is no longer owed.
	p.DespacharUnaVez(context.Background(), "claude")
	ps, _ = p.Pendientes()
	if slices.ContainsFunc(ps, func(x Pendiente) bool { return x.Tipo == PendFaltaLanzar }) {
		t.Fatalf("still owed after dispatch: %+v", ps)
	}
}

// A task taken whose launch never delivered a contract is owed too: the dispatcher relaunches it.
func TestPendientesNombraUnLanzamientoCortado(t *testing.T) {
	p, _, e := proyectoDePrueba(t, "")
	planS01(t, p, e.NN)
	pl, _ := plan.Leer(p.PlanPath(e.NN))
	plan.Tomar(pl, "T1", "worker-back-s01")
	plan.Guardar(p.PlanPath(e.NN), pl)
	ps, err := p.Pendientes()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(ps, func(x Pendiente) bool {
		return x.Tipo == PendCortado && slices.Equal(x.Tareas, []string{"T1"}) && x.Worker == "worker-back-s01"
	}) {
		t.Fatalf("%+v", ps)
	}
}

// reintentar blocks the slice and opens a new attempt, as `orq slice reintentar` would.
func reintentar(t *testing.T, p Proyecto, nn string) {
	t.Helper()
	ruta := filepath.Join(p.Dir, "slices", nn+".json")
	raw, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	json.Unmarshal(raw, &m)
	m["fase"], m["motivo"] = "BLOQUEADO", "prueba"
	raw, _ = json.Marshal(m)
	os.WriteFile(ruta, raw, 0o644)
	if _, err := p.Slices().Reintentar(nn, "otro enfoque", "worker", false); err != nil {
		t.Fatal(err)
	}
}

// M1: after slice reintentar, a task en_curso from the earlier attempt is neither relaunched nor
// reported as a cut-short launch: the orchestrator decides what changes; once the plan makes it
// ready again (plan soltar), despachar launches it.
func TestDespacharTrasReintentarNoRelanzaLaTareaVieja(t *testing.T) {
	p, term, e := proyectoDePrueba(t, "")
	planS01(t, p, e.NN)
	ctx := context.Background()
	if _, err := p.DespacharUnaVez(ctx, "claude"); err != nil {
		t.Fatal(err)
	}
	ses, _ := canal.LoadSesiones(p.SesPath())
	p.CerrarSesiones(ctx, ses, func(k string) bool { return k == "worker-back-s01" })
	reintentar(t, p, e.NN)
	ps, err := p.Pendientes()
	if err != nil {
		t.Fatal(err)
	}
	if slices.ContainsFunc(ps, func(x Pendiente) bool { return x.Tipo == PendCortado }) ||
		!slices.ContainsFunc(ps, func(x Pendiente) bool {
			return x.Tipo == PendReintento && slices.Equal(x.Tareas, []string{"T1"}) && x.Debe == "orquestador"
		}) {
		t.Fatalf("%+v", ps)
	}
	if ds, err := p.DespacharUnaVez(ctx, "claude"); err != nil || len(ds) != 0 {
		t.Fatalf("relaunched an earlier attempt's task: %v %+v", err, ds)
	}
	if a := len(term.Ops("arrancar")); a != 1 {
		t.Fatalf("arrancar=%d", a)
	}
	pl, _ := plan.Leer(p.PlanPath(e.NN))
	plan.Soltar(pl, "T1")
	plan.Guardar(p.PlanPath(e.NN), pl)
	ds, err := p.DespacharUnaVez(ctx, "claude")
	if err != nil || len(ds) != 1 || ds[0].Tarea != "T1" || ds[0].Resultado != "lanzado" || len(term.Ops("arrancar")) != 2 {
		t.Fatalf("ready again: %v %+v", err, ds)
	}
}

// L3: with the machine full, every ready task waits: no intent, no failure, no error.
func TestDespacharSinCapacidadEspera(t *testing.T) {
	p, term, e := proyectoDePrueba(t, "")
	planS01(t, p, e.NN)
	p.Medir = maquinaDe(sysinfo.Maquina{MemTotalGB: 8, MemDisponibleGB: 0.5, CPUs: 2, Carga1: 1}, 0)
	ds, err := p.DespacharUnaVez(context.Background(), "claude")
	if err != nil || len(ds) != 1 || ds[0].Tarea != "T1" || ds[0].Resultado != "espera" || ds[0].Detalle == "" {
		t.Fatalf("%v %+v", err, ds)
	}
	evs, _ := diario.Leer(p.DiarioPath())
	if slices.ContainsFunc(evs, func(ev diario.Evento) bool { return ev.Tipo == "despacho" }) || len(term.Ops("crear")) != 0 {
		t.Fatal("a full machine wrote an intent or opened a panel")
	}
}

// L4: a live worker that takes a new task gets a new contract, labelled tarea_nueva.
func TestDespacharTareaNuevaAWorkerVivo(t *testing.T) {
	p, term, e := proyectoDePrueba(t, "")
	pl := &plan.Plan{Slice: e.NN, Debatido: true, Tareas: []plan.Tarea{
		{ID: "T1", Zona: "back", Rutas: []string{"src"}, Dificultad: "baja", Estado: "pendiente"},
		{ID: "T2", Zona: "back", Rutas: []string{"api"}, DependeDe: []string{"T1"}, Dificultad: "baja", Estado: "pendiente"},
	}}
	plan.Guardar(p.PlanPath(e.NN), pl)
	ctx := context.Background()
	p.DespacharUnaVez(ctx, "claude")
	pl, _ = plan.Leer(p.PlanPath(e.NN))
	plan.Hecha(pl, "T1")
	plan.Guardar(p.PlanPath(e.NN), pl)
	ds, err := p.DespacharUnaVez(ctx, "claude")
	if err != nil || len(ds) != 1 || ds[0].Tarea != "T2" || ds[0].Resultado != "tarea_nueva" {
		t.Fatalf("%v %+v", err, ds)
	}
	if a, v := len(term.Ops("arrancar")), len(term.Ops("avisar")); a != 1 || v != 2 {
		t.Fatalf("arrancar=%d avisar=%d", a, v)
	}
}
