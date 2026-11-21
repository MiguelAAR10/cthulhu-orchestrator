package slice

// G3: approve and close survive a crash at every boundary. A re-run converges to exactly one
// result: one merge on main, one log row, one decision, no lost approval. "aceptado" is logged only
// once the commit is on main. All of it is orq2; orq keeps 1.0.0 (the *Como100 tests).

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"orq/internal/diario"
)

type caida struct{ etapa string }

// conCaida runs f with ORQ_FALLA_EN=etapa and reports whether it stopped there. The simulated death
// unwinds the call (locks are released, as when a process dies) instead of ending the test binary.
func conCaida(t *testing.T, etapa string, f func()) (cayo bool) {
	t.Helper()
	t.Setenv("ORQ_FALLA_EN", etapa)
	prev := morir
	morir = func(e string) { panic(caida{e}) }
	defer func() {
		morir = prev
		os.Setenv("ORQ_FALLA_EN", "")
		if x := recover(); x != nil {
			c, ok := x.(caida)
			if !ok {
				panic(x)
			}
			cayo = c.etapa == etapa
		}
	}()
	f()
	return false
}

// conTras runs f with the hook at punto replaced by h.
func conTras(punto string, h func(), f func()) {
	prev := tras
	tras = func(p string) {
		if p == punto {
			h()
		}
	}
	defer func() { tras = prev }()
	f()
}

func verde(t *testing.T) (Repo, *Estado) {
	t.Helper()
	r := repoDeJuguete(t)
	e := abrir(t, r)
	commitEn(t, e.Worktree, "total.txt", "170\n")
	if _, _, err := r.Verificar("", t0); err != nil {
		t.Fatal(err)
	}
	return r, e
}

func filasLog(r Repo, nn string) []Registro {
	var out []Registro
	for _, reg := range leerJSONL[Registro](r.logPath()) {
		if reg.NN == nn {
			out = append(out, reg)
		}
	}
	return out
}

func refsOrq(t *testing.T, r Repo) string {
	t.Helper()
	out, err := Git(r.Root, "for-each-ref", "--format=%(refname)", "refs/orq/")
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func cerrados(t *testing.T, r Repo) int {
	n := 0
	for _, ev := range eventos(t, r, "fase") {
		if ev.A == "CERRADO" {
			n++
		}
	}
	return n
}

// convergeCerrado re-runs the close (as the Owner would) and checks it ends in exactly one result.
func convergeCerrado(t *testing.T, r Repo, aceptado bool, motivo string) *Registro {
	t.Helper()
	reg, _, _, err := r.Cerrar("01", aceptado, motivo, t0.Add(time.Hour))
	if err != nil && !errors.Is(err, ErrYaCerrado) {
		t.Fatalf("the re-run must finish the close: %v", err)
	}
	if reg == nil {
		t.Fatal("the re-run returned no record")
	}
	if _, _, _, err := r.Cerrar("01", aceptado, motivo, t0.Add(2*time.Hour)); !errors.Is(err, ErrYaCerrado) {
		t.Fatalf("a third run must say already closed: %v", err)
	}
	if n := len(filasLog(r, "01")); n != 1 {
		t.Fatalf("one log row expected, got %d: %+v", n, filasLog(r, "01"))
	}
	if n := cerrados(t, r); n != 1 {
		t.Fatalf("one CERRADO event expected, got %d", n)
	}
	if _, err := os.Stat(r.estadoPath("01")); !os.IsNotExist(err) {
		t.Fatalf("the state file stays: %v", err)
	}
	if refs := refsOrq(t, r); refs != "" {
		t.Fatalf("orq refs stay after the close:\n%s", refs)
	}
	return reg
}

var etapasAceptado = []string{"intencion", "integrar", "integrado", "publicado", "decidido", "limpiando", "registrado", "anotado", "sin-refs"}

func TestCierreAceptadoConvergeTrasCaidaEnCadaEtapa(t *testing.T) {
	for _, etapa := range etapasAceptado {
		t.Run(etapa, func(t *testing.T) {
			t.Setenv("ORQ_NOMBRE", "orq2")
			r, _ := verde(t)
			if !conCaida(t, etapa, func() { r.Cerrar("01", true, "", t0) }) {
				t.Fatalf("the close did not stop at %q", etapa)
			}
			// "aceptado" is never in the diario while main lacks the commit.
			main, _ := Git(r.Root, "log", "--oneline", "main")
			if d := eventos(t, r, "decision"); len(d) > 0 && !strings.Contains(main, "slice S-01") {
				t.Fatalf("aceptado logged before main has the commit: %+v", d)
			}
			reg := convergeCerrado(t, r, true, "")
			if reg.Result != "aceptado" || reg.SHA == "" {
				t.Fatalf("%+v", reg)
			}
			main, _ = Git(r.Root, "log", "--oneline", "main")
			if n := strings.Count(main, "slice S-01"); n != 1 {
				t.Fatalf("integrated %d times:\n%s", n, main)
			}
			if got, _ := os.ReadFile(filepath.Join(r.Root, "total.txt")); string(got) != "170\n" {
				t.Fatal("main lacks the change")
			}
			if d := eventos(t, r, "decision"); len(d) != 1 || d[0].Resultado != "aceptado" || !decididoEn(d[0]).Equal(t0) {
				t.Fatalf("one aceptado decision at the Owner's time expected: %+v", d)
			}
			ok := 0
			for _, ev := range eventos(t, r, "integracion") {
				if ev.Resultado == "ok" {
					ok++
				}
			}
			if ok != 1 {
				t.Fatalf("one integration row expected, got %d", ok)
			}
		})
	}
}

var etapasRechazo = []string{"intencion", "decidido", "limpiando", "registrado", "anotado", "sin-refs"}

func TestCierreRechazadoConvergeTrasCaidaEnCadaEtapa(t *testing.T) {
	for _, etapa := range etapasRechazo {
		t.Run(etapa, func(t *testing.T) {
			t.Setenv("ORQ_NOMBRE", "orq2")
			r := repoDeJuguete(t)
			e := abrir(t, r)
			if !conCaida(t, etapa, func() { r.Cerrar("01", false, "no era lo pedido", t0) }) {
				t.Fatalf("the rejection did not stop at %q", etapa)
			}
			// The Owner's first reason is the one recorded, whatever the re-run says.
			reg := convergeCerrado(t, r, false, "otro motivo")
			if reg.Result != "rechazado" || reg.Motivo != "no era lo pedido" {
				t.Fatalf("%+v", reg)
			}
			if d := eventos(t, r, "decision"); len(d) != 1 || d[0].Resultado != "rechazado" || d[0].Detalle != "no era lo pedido" {
				t.Fatalf("one rejection decision expected: %+v", d)
			}
			if _, err := os.Stat(e.Worktree); !os.IsNotExist(err) {
				t.Fatalf("the worker copy stays: %v", err)
			}
		})
	}
}

// H29: a close that cannot publish (the Owner's checkout has tracked changes) leaves no "aceptado"
// anywhere; once it publishes, there is one.
func TestAceptadoSoloConElCommitEnMain(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq2")
	r, _ := verde(t)
	os.WriteFile(filepath.Join(r.Root, "README"), []byte("borrador del dueño\n"), 0o644)
	Git(r.Root, "add", "README")
	if _, _, _, err := r.Cerrar("01", true, "", t0); err == nil {
		t.Fatal("published over the owner's staged file")
	}
	if d := eventos(t, r, "decision"); len(d) != 0 {
		t.Fatalf("aceptado logged while main is unchanged: %+v", d)
	}
	if n := len(filasLog(r, "01")); n != 0 {
		t.Fatalf("log row written while main is unchanged")
	}
	Git(r.Root, "commit", "-qm", "dueño")
	r.Cerrar("01", true, "", t0.Add(time.Minute)) // the base moved: integrates again
	reg := convergeCerrado(t, r, true, "")
	if reg.Result != "aceptado" {
		t.Fatalf("%+v", reg)
	}
	if d := eventos(t, r, "decision"); len(d) != 1 || !decididoEn(d[0]).Equal(t0) {
		t.Fatalf("one decision at the Owner's first time: %+v", d)
	}
}

// orq keeps 1.0.0: the decision is logged when the slice leaves VERIFICADO, before publishing.
func TestCierreAnotaDecisionAntesDePublicarComo100(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq")
	r, _ := verde(t)
	os.WriteFile(filepath.Join(r.Root, "README"), []byte("borrador\n"), 0o644)
	Git(r.Root, "add", "README")
	if _, _, _, err := r.Cerrar("01", true, "", t0); err == nil {
		t.Fatal("published over the owner's staged file")
	}
	if d := eventos(t, r, "decision"); len(d) != 1 || d[0].Resultado != "aceptado" {
		t.Fatalf("orq 1.0.0 logs the decision before publishing: %+v", d)
	}
	e, _ := r.Load("01")
	if e.Cierre != nil {
		t.Fatalf("orq writes no close intent: %+v", e.Cierre)
	}
}

// orq keeps 1.0.0: none of the new boundaries exists.
func TestEtapasNuevasNoExistenComo100(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq")
	for _, etapa := range []string{"intencion", "decidido", "registrado", "anotado", "sin-refs"} {
		r, _ := verde(t)
		if conCaida(t, etapa, func() { r.Cerrar("01", true, "", t0) }) {
			t.Fatalf("orq stopped at the new boundary %q", etapa)
		}
	}
	r := repoDeJuguete(t)
	abrir(t, r)
	ponerTarjeta(t, r, "01", t0)
	for _, etapa := range []string{"sellado", "aprobado"} {
		if conCaida(t, etapa, func() { r.AprobarTarjeta("01", t0) }) {
			t.Fatalf("orq stopped at the new boundary %q", etapa)
		}
	}
}

// H10: an approval that dies between the seal in git, the state and the diario is not lost and is
// not sealed twice: the re-run keeps the first time and logs one event.
func TestAprobarConvergeTrasCaida(t *testing.T) {
	for _, etapa := range []string{"sellado", "aprobado"} {
		t.Run(etapa, func(t *testing.T) {
			t.Setenv("ORQ_NOMBRE", "orq2")
			r := repoDeJuguete(t)
			abrir(t, r)
			ponerTarjeta(t, r, "01", t0)
			t1 := t0.Add(3 * time.Minute)
			if !conCaida(t, etapa, func() { r.AprobarTarjeta("01", t1) }) {
				t.Fatalf("the approval did not stop at %q", etapa)
			}
			if etapa == "sellado" {
				e, _ := r.Load("01")
				if err := r.confiable(e); err == nil || !strings.Contains(err.Error(), "slice aprobar 01") {
					t.Fatalf("an interrupted approval must say how to finish it: %v", err)
				}
			}
			e, _, err := r.AprobarTarjeta("01", t0.Add(10*time.Minute))
			if err != nil {
				t.Fatalf("the re-run must finish the approval: %v", err)
			}
			if !e.Tarjeta.Aprobada.Equal(t1) {
				t.Fatalf("the first approval time is kept: %v", e.Tarjeta.Aprobada)
			}
			if e, _ = r.Load("01"); r.confiable(e) != nil || e.PuedeLanzar() != nil {
				t.Fatalf("state and contract must agree after the re-run: %v", r.confiable(e))
			}
			var ap int
			for _, ev := range eventos(t, r, "tarjeta") {
				if ev.Resultado == "aprobada" {
					ap++
					if !ev.TS.Equal(t1) {
						t.Fatalf("the event carries the approval time: %+v", ev)
					}
				}
			}
			if ap != 1 {
				t.Fatalf("one approval event expected, got %d", ap)
			}
			if _, ya, err := r.AprobarTarjeta("01", t0.Add(time.Hour)); err != nil || !ya {
				t.Fatalf("a further run says already approved: %v %v", ya, err)
			}
			if n := len(eventos(t, r, "tarjeta")); n != 2 {
				t.Fatalf("lista + aprobada, nothing more: %d", n)
			}
		})
	}
}

// H11: a scope change made while the exam runs is kept, and the receipt computed with the old
// contract is not issued.
func TestVerificarNoPisaUnCambioHechoDuranteLaCorrida(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq2")
	r := repoDeJuguete(t)
	e := abrir(t, r)
	commitEn(t, e.Worktree, "total.txt", "170\n")
	conTras("verificar-cargado", func() {
		if _, err := r.Ampliar("01", []string{".envrc"}); err != nil {
			t.Fatal(err)
		}
	}, func() {
		if _, _, err := r.Verificar("01", t0); err == nil || !strings.Contains(err.Error(), "cambió mientras se verificaba") {
			t.Fatalf("the receipt of the old contract must be discarded: %v", err)
		}
	})
	e, _ = r.Load("01")
	if err := r.confiable(e); err != nil {
		t.Fatalf("the state must still match the contract: %v", err)
	}
	if len(e.Alcance) != 1 || e.Alcance[0] != ".envrc" || e.Fase != Implementando || e.Recibo != nil {
		t.Fatalf("the scope change is kept and nothing is verified: %+v", e)
	}
	if _, _, err := r.Verificar("01", t0); err != nil {
		t.Fatalf("verifying again passes: %v", err)
	}
}

// H11: the early trap exits block the fresh state under the lock, keeping a concurrent change.
func TestTrampaBloqueaSinPisarElAlcance(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq2")
	r := repoDeJuguete(t)
	e := abrir(t, r)
	commitEn(t, e.Worktree, "tests/datos.txt", "0\n")
	conTras("verificar-cargado", func() {
		if _, err := r.Ampliar("01", []string{".envrc"}); err != nil {
			t.Fatal(err)
		}
	}, func() {
		if _, _, err := r.Verificar("01", t0); err == nil || !strings.Contains(err.Error(), "trampa") {
			t.Fatalf("got %v", err)
		}
	})
	e, _ = r.Load("01")
	if e.Fase != Bloqueado {
		t.Fatalf("fase %s", e.Fase)
	}
	if err := r.confiable(e); err != nil {
		t.Fatalf("blocking must not lose the scope change: %v", err)
	}
}

// orq keeps 1.0.0 in the verifier: the old copy is saved (pinned so the orq path is not changed by accident).
func TestVerificarGuardaLaCopiaViejaComo100(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq")
	r := repoDeJuguete(t)
	e := abrir(t, r)
	commitEn(t, e.Worktree, "tests/datos.txt", "0\n")
	conTras("verificar-cargado", func() { r.Ampliar("01", []string{".envrc"}) }, func() { r.Verificar("01", t0) })
	e, _ = r.Load("01")
	if e.Fase != Bloqueado || len(e.Alcance) != 0 {
		t.Fatalf("orq 1.0.0 saves the copy it loaded: %+v", e)
	}
}

// A close that died after recording the Owner's decision cannot be turned into the other one.
func TestIntencionDeCierreNoSeCambia(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq2")
	r, _ := verde(t)
	if !conCaida(t, "intencion", func() { r.Cerrar("01", false, "no", t0) }) {
		t.Fatal("did not stop")
	}
	if _, _, _, err := r.Cerrar("01", true, "", t0); err == nil || !strings.Contains(err.Error(), "rechazado") {
		t.Fatalf("an accepted close over a recorded rejection must be refused: %v", err)
	}
	main, _ := Git(r.Root, "log", "--oneline", "main")
	if strings.Contains(main, "slice S-01") {
		t.Fatal("integrated over a recorded rejection")
	}
	if reg := convergeCerrado(t, r, false, "no"); reg.Result != "rechazado" {
		t.Fatalf("%+v", reg)
	}
}

// Review H-1: an accepted close that stopped (conflict, or a crash at intencion) and was reopened
// can still be rejected: reopening drops the old decision.
func TestReabrirLuegoRechazado(t *testing.T) {
	for _, como := range []string{"conflicto", "caida"} {
		t.Run(como, func(t *testing.T) {
			t.Setenv("ORQ_NOMBRE", "orq2")
			r, _ := verde(t)
			if como == "conflicto" {
				commitEn(t, r.Root, "total.txt", "99\n")
				if _, _, _, err := r.Cerrar("01", true, "", t0); err == nil {
					t.Fatal("integrated over a conflict")
				}
			} else if !conCaida(t, "intencion", func() { r.Cerrar("01", true, "", t0) }) {
				t.Fatal("did not stop")
			}
			if e, _ := r.Load("01"); e.Fase != Integrando || e.Cierre == nil {
				t.Fatalf("setup: %+v", e)
			}
			e, err := r.Reabrir("01", "otra vuelta")
			if err != nil || e.Cierre != nil {
				t.Fatalf("reopening must drop the old close decision: %v %+v", err, e.Cierre)
			}
			if reg := convergeCerrado(t, r, false, "al final no"); reg.Result != "rechazado" || reg.Motivo != "al final no" {
				t.Fatalf("%+v", reg)
			}
		})
	}
}

// Review H-1/M-1: a new attempt does not inherit a half-done rejection or its reason.
func TestReintentarNoArrastraElMotivoDeRechazo(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq2")
	r := repoDeJuguete(t)
	e := abrir(t, r)
	commitEn(t, e.Worktree, "tests/datos.txt", "0\n")
	r.Verificar("01", t0) // trap: BLOQUEADO
	if !conCaida(t, "intencion", func() { r.Cerrar("01", false, "motivo viejo", t0) }) {
		t.Fatal("did not stop")
	}
	if _, err := r.Reintentar("01", "sin tocar el fixture", CausaWorker, false); err == nil || !strings.Contains(err.Error(), "termina el rechazo") {
		t.Fatalf("a pending rejection is not dropped by a retry: %v", err)
	}
	// Only the Owner's explicit override revokes it, and the diario says so.
	e, err := r.Reintentar("01", "sin tocar el fixture", CausaWorker, false, RevocaRechazo{Por: "el Owner cambió de opinión"})
	if err != nil || e.Cierre != nil {
		t.Fatalf("a new attempt must start without the old close decision: %v %+v", err, e)
	}
	if rv := eventos(t, r, "revocacion"); len(rv) != 1 || rv[0].Detalle != "el Owner cambió de opinión" {
		t.Fatalf("the override must be logged: %+v", rv)
	}
	if reg := convergeCerrado(t, r, false, "motivo nuevo"); reg.Motivo != "motivo nuevo" {
		t.Fatalf("%+v", reg)
	}
}

// Review M-1: while a rejection is half done, the slice is stopped: no verify, no launch.
func TestRechazoAMedioCerrarNoSeVerificaNiLanza(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq2")
	r := repoDeJuguete(t)
	e := abrir(t, r)
	commitEn(t, e.Worktree, "total.txt", "170\n")
	if !conCaida(t, "intencion", func() { r.Cerrar("01", false, "no", t0) }) {
		t.Fatal("did not stop")
	}
	if _, _, err := r.Verificar("01", t0); err == nil || !strings.Contains(err.Error(), "rechazado") {
		t.Fatalf("verify must refuse a slice being rejected: %v", err)
	}
	e, _ = r.Load("01")
	if err := e.PuedeLanzar(); err == nil || !strings.Contains(err.Error(), "cerrar --rechazado") {
		t.Fatalf("launch must refuse and say how to finish: %v", err)
	}
}

// Review M-2: a card write that died between the seal and the state converges on the re-run.
func TestPonerTarjetaConvergeTrasCaida(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq2")
	r := repoDeJuguete(t)
	abrir(t, r)
	tj, _ := LeerTarjeta(tarjetaBuena)
	if !conCaida(t, "lista-sellada", func() { r.PonerTarjeta("01", tj, t0) }) {
		t.Fatal("did not stop at lista-sellada")
	}
	e, _ := r.Load("01")
	if err := r.confiable(e); err == nil || !strings.Contains(err.Error(), "slice tarjeta 01") {
		t.Fatalf("the error must say how to finish: %v", err)
	}
	if _, err := r.PonerTarjeta("01", tj, t0.Add(time.Minute)); err != nil {
		t.Fatalf("the re-run must converge: %v", err)
	}
	e, _ = r.Load("01")
	if err := r.confiable(e); err != nil || !e.Tarjeta.Lista.Equal(t0.Add(time.Minute)) {
		t.Fatalf("%v %+v", err, e.Tarjeta)
	}
	if _, _, err := r.AprobarTarjeta("01", t0.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
}

// Review L6: the approval event is matched by its own time, not by any earlier approval.
func TestAprobacionSeAnotaPorSuHora(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq2")
	r := repoDeJuguete(t)
	abrir(t, r)
	ponerTarjeta(t, r, "01", t0)
	// An earlier approval event of the same attempt (another card version).
	if err := r.anotarUnaVez(eventoTarjeta(&Estado{NN: "01", Intento: 1}, "aprobada", t0.Add(time.Minute)), func(diario.Evento) bool { return false }); err != nil {
		t.Fatal(err)
	}
	t1 := t0.Add(5 * time.Minute)
	if !conCaida(t, "aprobado", func() { r.AprobarTarjeta("01", t1) }) {
		t.Fatal("did not stop")
	}
	r.AprobarTarjeta("01", t0.Add(time.Hour))
	var horas []time.Time
	for _, ev := range eventos(t, r, "tarjeta") {
		if ev.Resultado == "aprobada" {
			horas = append(horas, ev.TS)
		}
	}
	if len(horas) != 2 || !horas[1].Equal(t1) {
		t.Fatalf("the second approval must be logged: %v", horas)
	}
}

// Review L1: the decision event is stamped when written (after main has the commit); the Owner's
// decision time travels as a field, and the waiting metric uses it.
func TestDecisionSeFechaAlEscribirla(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq2")
	r, _ := verde(t)
	os.WriteFile(filepath.Join(r.Root, "README"), []byte("borrador\n"), 0o644)
	Git(r.Root, "add", "README")
	r.Cerrar("01", true, "", t0.Add(time.Hour))
	Git(r.Root, "commit", "-qm", "dueño")
	r.Cerrar("01", true, "", t0.Add(3*time.Hour))
	r.Cerrar("01", true, "", t0.Add(3*time.Hour))
	d := eventos(t, r, "decision")
	if len(d) != 1 || !d[0].TS.Equal(t0.Add(3*time.Hour)) {
		t.Fatalf("stamped when written: %+v", d)
	}
	if s, _ := d[0].Datos["decidido"].(string); s == "" {
		t.Fatalf("the Owner's time is a field: %+v", d[0])
	} else if ts, err := time.Parse(time.RFC3339Nano, s); err != nil || !ts.Equal(t0.Add(time.Hour)) {
		t.Fatalf("decidido %q: %v", s, err)
	}
	m := r.Metricas(t0.Add(-time.Hour), t0.Add(4*time.Hour))
	if m.Decididos != 1 || m.DecisionMinMediana > 61 {
		t.Fatalf("the wait ends at the Owner's decision: %+v", m)
	}
}

// decididoEn is the Owner's decision time carried by a decision event.
func decididoEn(ev diario.Evento) time.Time {
	s, _ := ev.Datos["decidido"].(string)
	ts, _ := time.Parse(time.RFC3339Nano, s)
	return ts
}

// Review 2 M-1: a rejection already in the diario is not undone by a reopen; only the Owner's
// override revokes it, logged, and then the slice can be verified and accepted.
func TestRev2RechazoMedioLuegoReabrirAceptar(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq2")
	r, _ := verde(t)
	if !conCaida(t, "decidido", func() { r.Cerrar("01", false, "no era", t0) }) {
		t.Fatal("did not stop")
	}
	if _, err := r.Reabrir("01", "otra vuelta"); err == nil || !strings.Contains(err.Error(), "termina el rechazo") {
		t.Fatalf("a reopen must not drop a pending rejection: %v", err)
	}
	if _, _, _, err := r.Cerrar("01", true, "", t0); err == nil {
		t.Fatal("accepted over a pending rejection")
	}
	e, err := r.Reabrir("01", "otra vuelta", RevocaRechazo{Por: "el Owner lo pidió"})
	if err != nil || e.Cierre != nil || e.Fase != Implementando {
		t.Fatalf("the Owner's override reopens: %v %+v", err, e)
	}
	if rv := eventos(t, r, "revocacion"); len(rv) != 1 || rv[0].Resultado != "rechazado" {
		t.Fatalf("the revocation must be logged: %+v", rv)
	}
	if _, _, err := r.Verificar("01", t0); err != nil {
		t.Fatal(err)
	}
	if reg := convergeCerrado(t, r, true, ""); reg.Result != "aceptado" {
		t.Fatalf("%+v", reg)
	}
}

// Review 2 L-1: a rejection that starts while the exam runs stops the receipt under the lock.
func TestRev2RechazoDuranteVerificar(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq2")
	r := repoDeJuguete(t)
	e := abrir(t, r)
	commitEn(t, e.Worktree, "total.txt", "170\n")
	conTras("verificar-cargado", func() {
		if !conCaida(t, "intencion", func() { r.Cerrar("01", false, "no", t0) }) {
			t.Fatal("did not stop")
		}
	}, func() {
		if _, _, err := r.Verificar("01", t0); err == nil || !strings.Contains(err.Error(), "rechazado") {
			t.Fatalf("the receipt must not be issued over a pending rejection: %v", err)
		}
	})
	if e, _ = r.Load("01"); e.Fase != Implementando || e.Recibo != nil {
		t.Fatalf("%+v", e)
	}
}

// Review 2 L-2: the hint names only the ways out that exist in that phase.
func TestRev2PistaDelRechazoPendiente(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq2")
	e := &Estado{NN: "01", Fase: Implementando, Cierre: &IntencionCierre{Resultado: "rechazado", Motivo: "no", TS: t0}}
	msg := e.CierrePendiente().Error()
	if !strings.Contains(msg, "termina el rechazo") || !strings.Contains(msg, "cerrar --rechazado") || strings.Contains(msg, "reabr") || strings.Contains(msg, "reintent") {
		t.Fatalf("IMPLEMENTANDO: only finishing the rejection: %s", msg)
	}
	e.Fase = Verificado
	if msg := e.CierrePendiente().Error(); !strings.Contains(msg, "reabrir") || !strings.Contains(msg, "--revoca-rechazo") {
		t.Fatalf("VERIFICADO: the Owner may revoke by reopening: %s", msg)
	}
	e.Fase = Bloqueado
	if msg := e.CierrePendiente().Error(); !strings.Contains(msg, "reintentar") || !strings.Contains(msg, "--revoca-rechazo") {
		t.Fatalf("BLOQUEADO: the Owner may revoke with a new attempt: %s", msg)
	}
}
