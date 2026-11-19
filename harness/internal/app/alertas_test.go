package app

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"orq/internal/canal"
	"orq/internal/diario"
	"orq/internal/slice"
	"orq/internal/terminal"
)

// reloj is an injectable clock the test moves by hand.
type reloj struct{ t time.Time }

func (r *reloj) ahora() time.Time { return r.t }

func alertasEn(t *testing.T, p Proyecto) []Alerta {
	t.Helper()
	as, err := p.Alertas(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return as
}

func eventosAlerta(t *testing.T, p Proyecto) []diario.Evento {
	t.Helper()
	evs, err := diario.Leer(p.DiarioPath())
	if err != nil {
		t.Fatal(err)
	}
	var out []diario.Evento
	for _, ev := range evs {
		if ev.Tipo == "alerta" {
			out = append(out, ev)
		}
	}
	return out
}

func commitEnClon(t *testing.T, dir, file, body string) {
	t.Helper()
	c := exec.Command("sh", "-c", "printf '"+body+"' > "+file+" && git add -A && git commit -qm w")
	c.Dir = dir
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("%s", out)
	}
}

func lanzar(t *testing.T, p Proyecto) {
	t.Helper()
	p.LanzarWorker(context.Background(), PedidoWorker{Agente: "claude", Alcance: "x", Presupuesto: "10 min"})
}

func TestWorkerQueNoArrancaSeMarcaEn60s(t *testing.T) {
	p, term, _ := proyectoDePrueba(t, "")
	rl := &reloj{time.Now()}
	p.Ahora = rl.ahora
	term.Fallos["arrancar"] = []error{terminal.ErrDesconocido}
	term.EstadoFijo["worker-claude-s01"] = terminal.Ausente
	lanzar(t, p)
	rl.t = rl.t.Add(59 * time.Second)
	if as := alertasEn(t, p); len(as) != 0 {
		t.Fatalf("before 60 s: %+v", as)
	}
	rl.t = rl.t.Add(time.Second)
	as := alertasEn(t, p)
	if len(as) != 1 || as[0].Tipo != "no_arranco" || as[0].Actor != "worker-claude-s01" || as[0].Slice != "01" {
		t.Fatalf("at 60 s: %+v", as)
	}
	alertasEn(t, p)
	if evs := eventosAlerta(t, p); len(evs) != 1 || evs[0].Paso != "no_arranco" {
		t.Fatalf("journaled once: %+v", evs)
	}
}

func TestSinAlertasEnCorridaLimpia(t *testing.T) {
	p, _, e := proyectoDePrueba(t, "")
	rl := &reloj{time.Now()}
	p.Ahora = rl.ahora
	lanzar(t, p)
	commitEnClon(t, e.Worktree, "v01.txt", "ok")
	rl.t = rl.t.Add(5 * time.Minute)
	if as := alertasEn(t, p); len(as) != 0 {
		t.Fatalf("healthy slice: %+v", as)
	}
	if evs := eventosAlerta(t, p); len(evs) != 0 {
		t.Fatalf("no alert event on a healthy slice: %+v", evs)
	}
}

func TestAlertaSinProgreso(t *testing.T) {
	p, _, _ := proyectoDePrueba(t, `{"alertas":{"sin_progreso_min":10}}`)
	rl := &reloj{time.Now()}
	p.Ahora = rl.ahora
	lanzar(t, p)
	rl.t = rl.t.Add(9 * time.Minute)
	if as := alertasEn(t, p); len(as) != 0 {
		t.Fatalf("before the deadline: %+v", as)
	}
	rl.t = rl.t.Add(2 * time.Minute)
	as := alertasEn(t, p)
	if len(as) != 1 || as[0].Tipo != "sin_progreso" || as[0].Actor != "worker-claude-s01" {
		t.Fatalf("no progress: %+v", as)
	}
}

func TestAlertasFueraDeAlcanceYExamen(t *testing.T) {
	p, _, _ := proyectoDePrueba(t, "")
	e, _, err := p.Slices().Abrir("02", "dos", "tests/s01.sh", "sh tests/s01.sh", slice.Opciones{SinConteo: "prueba", Alcance: []string{"v01.txt"}}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	aprobarTarjeta(t, p, "02")
	p.LanzarWorker(context.Background(), PedidoWorker{Slice: "02", Agente: "claude", Alcance: "x", Presupuesto: "10 min"})
	commitEnClon(t, e.Worktree, "otro.txt", "x")
	diario.Anotar(p.DiarioPath(), diario.Evento{TS: time.Now().Add(time.Second), Tipo: "examen", Slice: "02", Intento: 1, Detalle: "req ATN-1"})
	var tipos []string
	for _, a := range alertasEn(t, p) {
		tipos = append(tipos, a.Slice+" "+a.Tipo+":"+a.Detalle)
	}
	got := strings.Join(tipos, " | ")
	if !strings.Contains(got, "02 fuera_de_alcance:otro.txt") || !strings.Contains(got, "02 examen_tras_aprobar:") || strings.Contains(got, "01 ") {
		t.Fatalf("alerts: %s", got)
	}
}

func TestWorkerCerradoNoAlertaSinProgreso(t *testing.T) {
	p, _, _ := proyectoDePrueba(t, "")
	rl := &reloj{time.Now()}
	p.Ahora = rl.ahora
	lanzar(t, p)
	ses, _ := canal.LoadSesiones(p.SesPath())
	if _, err := p.CerrarSesiones(context.Background(), ses, func(k string) bool { return k == "worker-claude-s01" }); err != nil {
		t.Fatal(err)
	}
	rl.t = rl.t.Add(time.Hour)
	if as := alertasEn(t, p); len(as) != 0 {
		t.Fatalf("a closed worker makes no progress by design: %+v", as)
	}
}

func TestFueraDeAlcanceSeAnotaUnaVezPorSlice(t *testing.T) {
	p, _, _ := proyectoDePrueba(t, "")
	e, _, err := p.Slices().Abrir("02", "dos", "tests/s01.sh", "sh tests/s01.sh", slice.Opciones{SinConteo: "prueba", Alcance: []string{"v01.txt"}}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	aprobarTarjeta(t, p, "02")
	p.LanzarWorker(context.Background(), PedidoWorker{Slice: "02", Agente: "claude", Alcance: "x", Presupuesto: "10 min"})
	commitEnClon(t, e.Worktree, "otro.txt", "x")
	alertasEn(t, p)
	commitEnClon(t, e.Worktree, "otro2.txt", "x")
	as := alertasEn(t, p)
	n := 0
	for _, ev := range eventosAlerta(t, p) {
		if ev.Paso == "fuera_de_alcance" && ev.Slice == "02" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("journaled %d times, want 1", n)
	}
	if len(as) != 1 || !strings.Contains(as[0].Detalle, "otro2.txt") {
		t.Fatalf("status shows the latest list: %+v", as)
	}
}
