package slice

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"orq/internal/config"
)

const tiposTS = `if grep -q "x: number = 'a'" src.ts; then echo "src.ts(1,5): error TS2322: Type 'string' is not assignable to type 'number'."; exit 2; fi`

func TestCheckFallidoBloqueaElCierre(t *testing.T) {
	r := repoDeJuguete(t)
	r.Checks = []config.Check{{Nombre: "lint", Cmd: "true"}, {Nombre: "types", Cmd: tiposTS}, {Nombre: "tests", Omitir: "sin script test"}}
	e := abrir(t, r)
	commitEn(t, e.Worktree, "src.ts", "let x: number = 'a'\n")
	commitEn(t, e.Worktree, "total.txt", "170\n")
	e, _, err := r.Verificar("", t0)
	if err == nil || !strings.Contains(err.Error(), "check types falla (exit 2)") || !strings.Contains(err.Error(), "error TS2322") {
		t.Fatalf("err = %v", err)
	}
	if e.Fase == Verificado {
		t.Fatal("verified with a failing check")
	}
	if _, _, _, err := r.Cerrar("", true, "", t0); err == nil {
		t.Fatal("closed with a failing check")
	}
}

func TestCheckGuardiaExtraBloquea(t *testing.T) {
	r := repoDeJuguete(t)
	r.Checks = []config.Check{{Nombre: "lint", Cmd: "true"}, {Nombre: "types", Cmd: "true"}, {Nombre: "tests", Cmd: "true"},
		{Nombre: "sin-suppress", Cmd: "! grep -rn eslint-disable src"}}
	e := abrir(t, r)
	os.MkdirAll(filepath.Join(e.Worktree, "src"), 0o755)
	commitEn(t, e.Worktree, "src/a.ts", "// eslint-disable-next-line\nlet y = 1\n")
	commitEn(t, e.Worktree, "total.txt", "170\n")
	_, _, err := r.Verificar("", t0)
	if err == nil || !strings.Contains(err.Error(), "check sin-suppress falla (exit 1)") || !strings.Contains(err.Error(), "src/a.ts:1:// eslint-disable-next-line") {
		t.Fatalf("err = %v", err)
	}
}

func TestReciboLlevaCadaCheck(t *testing.T) {
	r := repoDeJuguete(t)
	r.Checks = []config.Check{{Nombre: "lint", Cmd: "echo limpio"}, {Nombre: "types", Cmd: "true"},
		{Nombre: "tests", Omitir: "sin script test"}, {Nombre: "arch", Cmd: "test -f total.txt"}}
	e := abrir(t, r)
	commitEn(t, e.Worktree, "total.txt", "170\n")
	e, _, err := r.Verificar("", t0)
	if err != nil || e.Fase != Verificado {
		t.Fatalf("%v %+v", err, e)
	}
	sellado, err := r.reciboConfiable(e)
	if err != nil {
		t.Fatal(err)
	}
	for _, rec := range []*Recibo{e.Recibo, sellado} {
		cs := rec.Checks
		if len(cs) != 4 {
			t.Fatalf("checks = %+v", cs)
		}
		want := []ResultadoCheck{
			{Nombre: "lint", Cmd: "echo limpio", Exit: 0, Resultado: "paso", Salida: "limpio"},
			{Nombre: "types", Cmd: "true", Exit: 0, Resultado: "paso"},
			{Nombre: "tests", Exit: 0, Resultado: "omitido", Motivo: "sin script test"},
			{Nombre: "arch", Cmd: "test -f total.txt", Exit: 0, Resultado: "paso"},
		}
		for i := range want {
			if cs[i] != want[i] {
				t.Errorf("check %d = %+v, want %+v", i, cs[i], want[i])
			}
		}
	}
}

// A lint fix that only changes whitespace is progress: the failure signature includes the checks' output.
func TestCheckArregloDeFormatoNoEsSinProgreso(t *testing.T) {
	r := repoDeJuguete(t)
	r.Checks = []config.Check{{Nombre: "lint", Cmd: "! grep -l ' $' notas.txt otra.txt"}, {Nombre: "types", Cmd: "true"}, {Nombre: "tests", Cmd: "true"}}
	e := abrir(t, r)
	commitEn(t, e.Worktree, "otra.txt", "chau \n")
	commitEn(t, e.Worktree, "notas.txt", "hola \n")
	commitEn(t, e.Worktree, "total.txt", "170\n")
	if _, _, err := r.Verificar("", t0); err == nil || !strings.Contains(err.Error(), "check lint falla") {
		t.Fatalf("err = %v", err)
	}
	commitEn(t, e.Worktree, "notas.txt", "hola\n")
	e, _, err := r.Verificar("", t0)
	if e.Fase == Bloqueado || err == nil || !strings.Contains(err.Error(), "otra.txt") {
		t.Fatalf("fase %s err = %v", e.Fase, err)
	}
}

func TestVerificarSinChecksComoAntes(t *testing.T) {
	r := repoDeJuguete(t)
	e := abrir(t, r)
	commitEn(t, e.Worktree, "total.txt", "170\n")
	e, _, err := r.Verificar("", t0)
	if err != nil || e.Fase != Verificado || len(e.Recibo.Checks) != 0 {
		t.Fatalf("%v %+v", err, e)
	}
}

// A tool the box cannot see is an infrastructure failure: it still blocks, and says so.
func TestCheckSinHerramientaEsInfraestructura(t *testing.T) {
	got := FalloDeChecks([]ResultadoCheck{{Nombre: "types", Cmd: "tsc", Exit: 127, Resultado: CheckFallo, Salida: "sh: 1: tsc: not found"}})
	if !strings.Contains(got, "check types falla (exit 127)") || !strings.Contains(got, "infraestructura") || !strings.Contains(got, "sandbox.ro") {
		t.Fatalf("got %q", got)
	}
}
