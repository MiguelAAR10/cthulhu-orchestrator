package app

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"orq/internal/canal"
	"orq/internal/config"
	"orq/internal/limits"
	"orq/internal/plan"
	"orq/internal/slice"
	"orq/internal/sysinfo"
)

// orquestadorGen1 deja un Orquestador vivo de generación 1, como lo deja `orq up`.
func orquestadorGen1(t *testing.T, p Proyecto) canal.Sesion {
	t.Helper()
	s, _, err := p.LanzarRol(context.Background(), map[string]canal.Sesion{}, "orquestador", config.Agente{})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// G2 review 2 M1: two `relevo orquestador` from generation 1 at once open one new session.
func TestDosRelevosALaVezAbrenUnaOrq2(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq2")
	p, term, _ := proyectoDePrueba(t, "")
	g1 := orquestadorGen1(t, p)
	mapas := make([]map[string]canal.Sesion, 2)
	for k := range mapas {
		mapas[k], _ = canal.LoadSesiones(p.SesPath()) // both commands read generation 1
	}
	errs := make([]error, 2)
	anteriores := make([]canal.Sesion, 2)
	var wg sync.WaitGroup
	for k := range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, anteriores[k], errs[k] = p.RelevarRol(context.Background(), mapas[k], "orquestador", config.Agente{}, g1, false)
		}()
	}
	wg.Wait()
	ok := 0
	for k, err := range errs {
		switch {
		case err == nil:
			ok++
			if anteriores[k].Gen != 1 {
				t.Fatalf("the session to close is generation %d, want 1", anteriores[k].Gen)
			}
		case !errors.Is(err, ErrYaHay):
			t.Fatal(err)
		}
	}
	if ok != 1 || len(term.Ops("crear")) != 2 {
		t.Fatalf("%d relevos opened, %d panes in total (want 1 and 2): %v", ok, len(term.Ops("crear")), errs)
	}
}

// The "previous session can be closed" line names the session the new one really replaces.
func TestRelevoForzadoNombraLaSesionQueReemplazaOrq2(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq2")
	p, _, _ := proyectoDePrueba(t, "")
	g1 := orquestadorGen1(t, p)
	viejo := map[string]canal.Sesion{"orquestador": g1} // read before another relevo opened generation 2
	if _, _, err := p.RelevarRol(context.Background(), map[string]canal.Sesion{"orquestador": g1}, "orquestador", config.Agente{}, g1, false); err != nil {
		t.Fatal(err)
	}
	if _, _, err := p.RelevarRol(context.Background(), viejo, "orquestador", config.Agente{}, g1, false); !errors.Is(err, ErrYaHay) {
		t.Fatalf("a relevo that started before generation 2 opened must be refused: %v", err)
	}
	nueva, anterior, err := p.RelevarRol(context.Background(), map[string]canal.Sesion{"orquestador": g1}, "orquestador", config.Agente{}, g1, true)
	if err != nil || anterior.Gen != 2 || nueva.Gen != 3 {
		t.Fatalf("forced: new gen %d, previous gen %d, %v", nueva.Gen, anterior.Gen, err)
	}
}

// G2 review 2 M2: one free slot and two launches at once in different slices: one launches.
func TestCapacidadSeReservaEntreSlicesOrq2(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq2")
	p, term, _ := proyectoDePrueba(t, "")
	if _, out, err := p.Slices().Abrir("02", "dos", "tests/s01.sh", "sh tests/s01.sh", slice.Opciones{SinConteo: "prueba"}, time.Now()); err != nil {
		t.Fatalf("abrir 02: %v\n%s", err, out)
	}
	aprobarTarjeta(t, p, "02")
	l, err := limits.Load(limits.DefaultPath())
	if err != nil {
		t.Fatal(err)
	}
	ocupados := l.TopeWorkers() - 1 // one slot left; every started agent takes it
	p.Medir = Medir{
		Maquina: func() (sysinfo.Maquina, error) {
			return sysinfo.Maquina{MemTotalGB: 4096, MemDisponibleGB: 4000, CPUs: 1024, Carga1: 1}, nil
		},
		Vivos: func(limits.Limits) ([]sysinfo.Proc, error) {
			return make([]sysinfo.Proc, ocupados+len(term.Ops("arrancar"))), nil
		},
	}
	errs := make([]error, 2)
	var wg sync.WaitGroup
	for k, nn := range []string{"01", "02"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, errs[k] = p.LanzarWorker(context.Background(), PedidoWorker{Slice: nn, Agente: "claude", Alcance: "x", Presupuesto: "10 min"})
		}()
	}
	wg.Wait()
	if n := len(term.Ops("arrancar")); n != 1 || (errs[0] == nil) == (errs[1] == nil) {
		t.Fatalf("%d workers started with one free slot: %v / %v", n, errs[0], errs[1])
	}
}

// G2 review 2 L1: with RechazarOcupado, a zone worker busy with another task is refused under the lock.
func TestRechazarOcupadoNoDaOtraTareaOrq2(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq2")
	p, term, e := proyectoDePrueba(t, "")
	pl := &plan.Plan{Slice: e.NN, Debatido: true, Tareas: []plan.Tarea{
		{ID: "T1", Zona: "a", Rutas: []string{"v01.txt"}, Dificultad: "baja", Estado: "pendiente"},
		{ID: "T3", Zona: "a", Rutas: []string{"v03.txt"}, Dificultad: "baja", Estado: "pendiente"},
	}}
	if err := plan.Guardar(p.PlanPath(e.NN), pl); err != nil {
		t.Fatal(err)
	}
	if err := lanzarTarea(p, "T1"); err != nil {
		t.Fatal(err)
	}
	_, _, err := p.LanzarWorker(context.Background(), PedidoWorker{Agente: "claude", Tarea: "T3", RechazarOcupado: true})
	if err == nil || !strings.Contains(err.Error(), "T1") {
		t.Fatalf("a busy worker must be refused, naming its task: %v", err)
	}
	got, _ := plan.Leer(p.PlanPath(e.NN))
	if got.Tareas[1].Estado != "pendiente" || len(term.Ops("avisar")) != 1 {
		t.Fatalf("T3 %+v, contracts %d", got.Tareas[1], len(term.Ops("avisar")))
	}
	// Without the option it keeps 1.0.0: the live worker gets the next task too.
	if err := lanzarTarea(p, "T3"); err != nil || len(term.Ops("avisar")) != 2 {
		t.Fatalf("%v, contracts %d", err, len(term.Ops("avisar")))
	}
}

// G2 review 2 L3: a second Orchestrator is refused with the way to a fresh one.
func TestAbrirOrquestadorYaAbiertoApuntaAlRelevoOrq2(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq2")
	p, _, _ := proyectoDePrueba(t, "")
	orquestadorGen1(t, p)
	_, _, err := p.AbrirRol(context.Background(), map[string]canal.Sesion{}, "orquestador", config.Agente{})
	if !errors.Is(err, ErrYaHay) || !strings.Contains(err.Error(), "orq relevo orquestador") {
		t.Fatalf("%v", err)
	}
}
