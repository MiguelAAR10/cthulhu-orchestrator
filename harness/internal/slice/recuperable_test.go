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

var etapasAceptado = []string{"intencion", "integrar", "integrado", "publicado", "decidido", "limpiando", "registrado", "anotado", "sin-estado"}

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
			if d := eventos(t, r, "decision"); len(d) != 1 || d[0].Resultado != "aceptado" || !d[0].TS.Equal(t0) {
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

var etapasRechazo = []string{"intencion", "decidido", "limpiando", "registrado", "anotado", "sin-estado"}

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
	if d := eventos(t, r, "decision"); len(d) != 1 || !d[0].TS.Equal(t0) {
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
	for _, etapa := range []string{"intencion", "decidido", "registrado", "anotado", "sin-estado"} {
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
