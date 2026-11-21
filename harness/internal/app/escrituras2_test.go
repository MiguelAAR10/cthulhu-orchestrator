package app

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"orq/internal/canal"
	"orq/internal/config"
	"orq/internal/orqdir"
	"orq/internal/plan"
)

// G2 review M1: a launch while a member's close is verifying waits for it, and then cannot join
// a team that is already verified.
func TestLanzarDuranteUnCierreNoSeSumaOrq2(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq2")
	p, term, e := proyectoDePrueba(t, "")
	planDeDos(t, p, e.NN)
	if err := lanzarTarea(p, "T1"); err != nil {
		t.Fatal(err)
	}
	e, _ = p.Slices().Load(e.NN)
	commitEnClon(t, e.Equipo[0].Worktree, "v01.txt", "ok")
	// worker-a-s01 is closing: CerrarMiembro holds cierre-s01 while it verifies.
	soltar, err := orqdir.Candado(filepath.Join(p.Dir, "cierre-s01"))
	if err != nil {
		t.Fatal(err)
	}
	hecho := make(chan error, 1)
	go func() { hecho <- lanzarTarea(p, "T2") }()
	time.Sleep(200 * time.Millisecond)
	if n := len(term.Ops("crear")); n != 1 {
		soltar()
		<-hecho
		t.Fatalf("the launch went ahead during the close: %d panes", n)
	}
	if _, err := p.CerrarWorker(context.Background(), "worker-a-s01"); err != nil {
		soltar()
		t.Fatal(err)
	}
	soltar()
	if err := <-hecho; err == nil {
		t.Fatal("a launch into a verified slice must be refused")
	}
	e, _ = p.Slices().Load(e.NN)
	pl, _ := plan.Leer(p.PlanPath(e.NN))
	if len(e.Equipo) != 1 || pl.Tareas[1].Estado != "pendiente" {
		t.Fatalf("team %+v, T2 %+v", e.Equipo, pl.Tareas[1])
	}
}

// G2 review M2: two `sesion abrir planificador` at once open one pane; the other is refused.
func TestAbrirRolDosALaVezAbreUnoOrq2(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq2")
	p, term, _ := proyectoDePrueba(t, "")
	// Each command read the session map before the other wrote it.
	mapas := []map[string]canal.Sesion{{}, {}}
	errs := make([]error, 2)
	var wg sync.WaitGroup
	for k := range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, errs[k] = p.AbrirRol(context.Background(), mapas[k], "planificador", config.Agente{})
		}()
	}
	wg.Wait()
	ok, ya := 0, 0
	for _, err := range errs {
		switch {
		case err == nil:
			ok++
		case errors.Is(err, ErrYaHay):
			ya++
		default:
			t.Fatal(err)
		}
	}
	if ok != 1 || ya != 1 || len(term.Ops("crear")) != 1 {
		t.Fatalf("ok %d, refused %d, panes %d", ok, ya, len(term.Ops("crear")))
	}
	ses, _ := canal.LoadSesiones(p.SesPath())
	if ses["planificador"].Pane == "" || ses["planificador"].Gen != 1 {
		t.Fatalf("%+v", ses)
	}
}

func TestAbrirRolOrqComo100(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq")
	p, term, _ := proyectoDePrueba(t, "")
	if _, _, err := p.AbrirRol(context.Background(), map[string]canal.Sesion{}, "planificador", config.Agente{}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := p.AbrirRol(context.Background(), map[string]canal.Sesion{}, "planificador", config.Agente{}); err != nil || len(term.Ops("crear")) != 1 {
		// orq 1.0.0 decides on the map it was given: an empty map opens again (the agent exists, so it is adopted).
		t.Fatalf("%v, panes %d", err, len(term.Ops("crear")))
	}
}

// G2 review L2: a reenviar row written by orq 1.0.0 (no paso) is still honoured by orq2.
func TestReenviarDe100SeRespetaOrq2(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq2")
	p, term, _ := proyectoDePrueba(t, "")
	actor := "worker-x-s01"
	if _, err := avisar(p, actor, "contrato"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ORQ_NOMBRE", "orq")
	if err := p.Reenviar(actor); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ORQ_NOMBRE", "orq2")
	if ya, err := avisar(p, actor, "contrato"); err != nil || ya || len(term.Ops("avisar")) != 2 {
		t.Fatalf("the 1.0.0 reenviar was dropped: ya=%v err=%v sends=%d", ya, err, len(term.Ops("avisar")))
	}
	if ya, _ := avisar(p, actor, "contrato"); !ya {
		t.Fatal("after the re-send it is delivered")
	}
}

// G2 review L4: update sees a launch, a close or a delivery in progress.
func TestOcupadoVeLanzarCierreYAvisos(t *testing.T) {
	p, _, _ := proyectoDePrueba(t, "")
	for _, n := range []string{"lanzar-s01", "cierre-s01", "avisos"} {
		soltar, err := orqdir.Candado(filepath.Join(p.Dir, n))
		if err != nil {
			t.Fatal(err)
		}
		got := strings.Join(ocupadoEn(p.Root), "\n")
		soltar()
		if !strings.Contains(got, n+".lock") {
			t.Fatalf("%s held, update sees: %q", n, got)
		}
	}
	if got := ocupadoEn(p.Root); len(got) != 0 {
		t.Fatalf("nothing held: %q", got)
	}
}
