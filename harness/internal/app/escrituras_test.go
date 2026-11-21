package app

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"orq/internal/canal"
	"orq/internal/plan"
	"orq/internal/slice"
	"orq/internal/terminal"
)

// planDeDos deja un plan acordado con dos tareas listas en zonas distintas.
func planDeDos(t *testing.T, p Proyecto, nn string) {
	t.Helper()
	pl := &plan.Plan{Slice: nn, Debatido: true, Tareas: []plan.Tarea{
		{ID: "T1", Zona: "a", Rutas: []string{"v01.txt"}, Dificultad: "baja", Estado: "pendiente"},
		{ID: "T2", Zona: "b", Rutas: []string{"v02.txt"}, Dificultad: "baja", Estado: "pendiente"},
	}}
	if err := plan.Guardar(p.PlanPath(nn), pl); err != nil {
		t.Fatal(err)
	}
}

func lanzarTarea(p Proyecto, id string) error {
	_, _, err := p.LanzarWorker(context.Background(), PedidoWorker{Agente: "claude", Tarea: id, Presupuesto: "10 min"})
	return err
}

// H3 + H5: two `lanzar --tarea` at once. Both tasks stay taken in the plan and both sessions stay
// registered: the second writer waits for the first, it never overwrites it.
func TestLanzarDosTareasALaVezNoPierdeNadaOrq2(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq2")
	for i := 0; i < 3; i++ {
		p, term, e := proyectoDePrueba(t, "")
		planDeDos(t, p, e.NN)
		var wg sync.WaitGroup
		errs := make([]error, 2)
		for k, id := range []string{"T1", "T2"} {
			wg.Add(1)
			go func() { defer wg.Done(); errs[k] = lanzarTarea(p, id) }()
		}
		wg.Wait()
		if errs[0] != nil || errs[1] != nil {
			t.Fatalf("run %d: %v / %v", i, errs[0], errs[1])
		}
		pl, err := plan.Leer(p.PlanPath(e.NN))
		if err != nil {
			t.Fatal(err)
		}
		for _, tr := range pl.Tareas {
			if tr.Estado != "en_curso" || tr.Worker != "worker-"+tr.Zona+"-s01" {
				t.Fatalf("run %d: plan lost an update: %+v", i, pl.Tareas)
			}
		}
		ses, _ := canal.LoadSesiones(p.SesPath())
		if ses["worker-a-s01"].Pane == "" || ses["worker-b-s01"].Pane == "" {
			t.Fatalf("run %d: sessions lost an update: %+v", i, ses)
		}
		if n := len(term.Ops("avisar")); n != 2 {
			t.Fatalf("run %d: %d contracts, want 2", i, n)
		}
	}
}

// H3: a close that started with an old session map does not drop a session registered meanwhile.
func TestCerrarConMapaViejoNoBorraSesionNuevaOrq2(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq2")
	p, _, _ := proyectoDePrueba(t, "")
	lanzar(t, p)
	viejo, err := canal.LoadSesiones(p.SesPath())
	if err != nil {
		t.Fatal(err)
	}
	// Another process registers the Planner while this close still holds the old map.
	otro, _ := canal.LoadSesiones(p.SesPath())
	otro["planificador"] = canal.Sesion{Agente: "planificador-tienda", Pane: "p9", Gen: 1}
	if err := canal.SaveSesiones(p.SesPath(), otro); err != nil {
		t.Fatal(err)
	}
	if _, err := p.CerrarSesiones(context.Background(), viejo, func(k string) bool { return strings.HasPrefix(k, "worker-") }); err != nil {
		t.Fatal(err)
	}
	ses, _ := canal.LoadSesiones(p.SesPath())
	if _, ok := ses["planificador"]; !ok {
		t.Fatalf("the stale save dropped the Planner: %+v", ses)
	}
	if _, ok := ses["worker-claude-s01"]; ok {
		t.Fatalf("the worker was not closed: %+v", ses)
	}
}

// H4: two team members closing at once still get exactly one verification.
func TestDosMiembrosCerrandoALaVezVerificanUnaVezOrq2(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq2")
	p, _, e := proyectoDePrueba(t, "")
	planDeDos(t, p, e.NN)
	for _, id := range []string{"T1", "T2"} {
		if err := lanzarTarea(p, id); err != nil {
			t.Fatal(err)
		}
	}
	e, _ = p.Slices().Load(e.NN)
	for _, w := range e.Equipo {
		f, body := "v01.txt", "ok"
		if w.Zona == "b" {
			f, body = "v02.txt", "b"
		}
		commitEnClon(t, w.Worktree, f, body)
	}
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for k, n := range []string{"worker-a-s01", "worker-b-s01"} {
		wg.Add(1)
		go func() { defer wg.Done(); _, _, errs[k] = p.CerrarMiembro(context.Background(), n) }()
	}
	wg.Wait()
	if errs[0] != nil || errs[1] != nil {
		t.Fatalf("%v / %v", errs[0], errs[1])
	}
	verificadas := 0
	for _, f := range filasIntegracion(t, p) {
		if f.Datos["verificado"] == true {
			verificadas++
		}
	}
	e, _ = p.Slices().Load(e.NN)
	ses, _ := canal.LoadSesiones(p.SesPath())
	if verificadas != 1 || e.Fase != slice.Verificado {
		t.Fatalf("%d verifications, phase %s; rows %+v", verificadas, e.Fase, filasIntegracion(t, p))
	}
	if _, ok := ses["worker-a-s01"]; ok {
		t.Fatalf("closed session came back: %+v", ses)
	}
	if _, ok := ses["worker-b-s01"]; ok {
		t.Fatalf("closed session came back: %+v", ses)
	}
}

// H8(b): a dead worker relaunched with the same name, task and attempt gets its contract again.
func TestRelanzarWorkerMuertoRecibeElContratoOrq2(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq2")
	p, term, _ := proyectoDePrueba(t, "")
	lanzar(t, p)
	ses, _ := canal.LoadSesiones(p.SesPath())
	term.Cerrar(context.Background(), ses["worker-claude-s01"].Pane) // the agent died
	lanzar(t, p)
	if n := len(term.Ops("avisar")); n != 2 {
		t.Fatalf("%d contracts; the relaunched worker must get its own", n)
	}
	lanzar(t, p) // a pure repeat still sends nothing
	if n := len(term.Ops("avisar")); n != 2 {
		t.Fatalf("%d contracts after a repeat", n)
	}
}

func TestRelanzarWorkerMuertoOrqComo100(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq")
	p, term, _ := proyectoDePrueba(t, "")
	lanzar(t, p)
	ses, _ := canal.LoadSesiones(p.SesPath())
	term.Cerrar(context.Background(), ses["worker-claude-s01"].Pane)
	lanzar(t, p)
	if n := len(term.Ops("avisar")); n != 1 {
		t.Fatalf("orq 1.0.0 keys the contract by name, step, slice and attempt: %d contracts", n)
	}
}

func avisar(p Proyecto, actor, paso string) (bool, error) {
	return p.avisarUnaVez(context.Background(), actor, paso, actor, "texto de "+paso, "01", 1)
}

// H8(a): two deliveries of the same contract at once send it once.
func TestContratoALaVezSeEntregaUnaVezOrq2(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq2")
	p, term, _ := proyectoDePrueba(t, "")
	for i := 0; i < 15; i++ {
		actor := fmt.Sprintf("worker-x%d-s01", i)
		var wg sync.WaitGroup
		for range 2 {
			wg.Add(1)
			go func() { defer wg.Done(); avisar(p, actor, "contrato") }()
		}
		wg.Wait()
		n := 0
		for _, l := range term.Ops("avisar") {
			if l.Destino == actor {
				n++
			}
		}
		if n != 1 {
			t.Fatalf("%s got %d contracts", actor, n)
		}
	}
}

// H8(c): `reenviar` repeats only the actor's last delivery, not every step it ever had.
func TestReenviarSoloRepiteElUltimoEnvioOrq2(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq2")
	p, term, _ := proyectoDePrueba(t, "")
	actor := "worker-x-s01"
	if _, err := avisar(p, actor, "contrato:T1"); err != nil {
		t.Fatal(err)
	}
	term.Fallos["avisar"] = []error{terminal.ErrDesconocido}
	avisar(p, actor, "contrato:T2")
	if err := p.Reenviar(actor); err != nil {
		t.Fatal(err)
	}
	if ya, err := avisar(p, actor, "contrato:T1"); err != nil || !ya {
		t.Fatalf("T1 was delivered; reenviar was about T2: ya=%v err=%v", ya, err)
	}
	if ya, err := avisar(p, actor, "contrato:T2"); err != nil || ya {
		t.Fatalf("T2 must be sent again: ya=%v err=%v", ya, err)
	}
	if n := len(term.Ops("avisar")); n != 3 {
		t.Fatalf("%d sends, want 3 (T1, T2 unknown, T2 again)", n)
	}
}

func TestReenviarOrqComo100(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq")
	p, term, _ := proyectoDePrueba(t, "")
	actor := "worker-x-s01"
	avisar(p, actor, "contrato:T1")
	p.Reenviar(actor)
	if ya, _ := avisar(p, actor, "contrato:T1"); ya || len(term.Ops("avisar")) != 2 {
		t.Fatal("orq 1.0.0: reenviar is keyed by actor alone")
	}
}
