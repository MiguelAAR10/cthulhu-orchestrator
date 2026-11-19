package slice

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"orq/internal/config"
)

// enRojo commits files in the main checkout before the slice opens: they are part of the red commit.
func enRojo(t *testing.T, r Repo, fs map[string]string) {
	t.Helper()
	for f, body := range fs {
		os.MkdirAll(filepath.Dir(filepath.Join(r.Root, f)), 0o755)
		if err := os.WriteFile(filepath.Join(r.Root, f), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	Git(r.Root, "add", "-A")
	if _, err := Git(r.Root, "commit", "-qm", "proyecto"); err != nil {
		t.Fatal(err)
	}
}

func gitEn(t *testing.T, dir string, args ...string) {
	t.Helper()
	if out, err := Git(dir, args...); err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
}

var checksVerdes = []config.Check{{Nombre: "lint", Cmd: "true"}, {Nombre: "types", Cmd: "true"}, {Nombre: "tests", Cmd: "npm test"}}

// Review repro 2: the worker rewrites the test script; the receipt records it and the close asks the Owner.
func TestDefinicionDeCheckCambiadaPideAlOwner(t *testing.T) {
	r := repoDeJuguete(t)
	r.Checks = []config.Check{{Nombre: "lint", Cmd: "true"}, {Nombre: "types", Cmd: "true"}, {Nombre: "tests", Cmd: "true"}}
	enRojo(t, r, map[string]string{"package.json": `{"scripts":{"test":"exit 1"},"dependencies":{"a":"1"}}`, "tsconfig.json": `{"strict":true}`})
	e := abrir(t, r)
	commitEn(t, e.Worktree, "package.json", `{"scripts":{"test":"exit 0"},"dependencies":{"a":"1"}}`)
	commitEn(t, e.Worktree, "tsconfig.json", `{"strict":false}`)
	commitEn(t, e.Worktree, "total.txt", "170\n")
	e, _, err := r.Verificar("", t0)
	if err != nil || e.Fase != Verificado {
		t.Fatalf("%v %+v", err, e)
	}
	if !slices.Equal(e.Recibo.DefinicionesChecks, []string{"package.json", "tsconfig.json"}) || !strings.Contains(e.Recibo.Nota, "check definitions changed: package.json, tsconfig.json") {
		t.Fatalf("recibo = %+v", e.Recibo)
	}
	pg, err := r.Pagina("01")
	if err != nil || !strings.Contains(pg.DiffDefiniciones, `+{"scripts":{"test":"exit 0"}`) || !strings.Contains(pg.DiffDefiniciones, `-{"strict":true}`) {
		t.Fatalf("%v diff = %q", err, pg.DiffDefiniciones)
	}
	if _, _, _, err := r.Cerrar("", true, "", t0); err == nil || !strings.Contains(err.Error(), "--acepto-checks") {
		t.Fatalf("closed without --acepto-checks: %v", err)
	}
	if _, _, _, err := r.Cerrar("", true, "", t0, CerrarOpc{AceptoChecks: true}); err != nil {
		t.Fatalf("with --acepto-checks: %v", err)
	}
}

func TestDefinicionesSoloDependenciasNoCuenta(t *testing.T) {
	r := repoDeJuguete(t)
	r.Definiciones = []string{"Makefile"}
	enRojo(t, r, map[string]string{"package.json": `{"scripts":{"test":"exit 1"},"dependencies":{"a":"1"}}`, "Makefile": "test:\n\tgo test\n",
		"pyproject.toml": "[project]\nname = \"x\"\n[tool.ruff]\nline-length = 80\n"})
	e := abrir(t, r)
	commitEn(t, e.Worktree, "package.json", `{"scripts":{"test":"exit 1"},"dependencies":{"a":"2","b":"1"}}`)
	commitEn(t, e.Worktree, "pyproject.toml", "[project]\nname = \"y\"\n[tool.ruff]\nline-length = 80\n")
	commitEn(t, e.Worktree, "total.txt", "170\n")
	got, err := r.definicionesCambiadas(e.SHARojo, mustPunta(t, r, e))
	if err != nil || len(got) != 0 {
		t.Fatalf("deps-only change flagged: %v %v", got, err)
	}
	commitEn(t, e.Worktree, "pyproject.toml", "[project]\nname = \"y\"\n[tool.ruff]\nline-length = 200\n")
	commitEn(t, e.Worktree, "Makefile", "test:\n\ttrue\n")
	got, err = r.definicionesCambiadas(e.SHARojo, mustPunta(t, r, e))
	if err != nil || !slices.Equal(got, []string{"Makefile", "pyproject.toml"}) {
		t.Fatalf("got %v %v", got, err)
	}
}

func mustPunta(t *testing.T, r Repo, e *Estado) string {
	t.Helper()
	in, err := r.puntaDelSlice(e)
	if err != nil {
		t.Fatal(err)
	}
	return in.Punta
}

// Review repro 3: a project test file removed or renamed by the slice is in the receipt.
func TestTestsBorradosEnElRecibo(t *testing.T) {
	r := repoDeJuguete(t)
	enRojo(t, r, map[string]string{"x_test.go": "package x\n", "web/a.spec.ts": "it()\n", "__tests__/b.js": "b\n", "util.go": "package x\n"})
	e := abrir(t, r)
	gitEn(t, e.Worktree, "rm", "-q", "x_test.go", "util.go")
	gitEn(t, e.Worktree, "mv", "web/a.spec.ts", "web/a.ts")
	gitEn(t, e.Worktree, "rm", "-q", "__tests__/b.js")
	gitEn(t, e.Worktree, "commit", "-qm", "fuera")
	commitEn(t, e.Worktree, "total.txt", "170\n")
	e, _, err := r.Verificar("", t0)
	if err != nil || e.Fase != Verificado {
		t.Fatalf("%v %+v", err, e)
	}
	want := []string{"__tests__/b.js", "web/a.spec.ts → web/a.ts", "x_test.go"}
	if !slices.Equal(e.Recibo.TestsBorrados, want) || !strings.Contains(e.Recibo.Nota, "tests deleted: ") {
		t.Fatalf("recibo = %+v", e.Recibo)
	}
}

// Review repro 4: a suppression added by the slice fails the built-in guardrail with file:line.
func TestSupresionesGuardiaPorDefecto(t *testing.T) {
	r := repoDeJuguete(t)
	cfg := config.Config{Checks: map[string]config.Check{"lint": {Cmd: "true"}, "types": {Cmd: "true"}, "tests": {Cmd: "true"}}}
	r.Checks = cfg.ChecksEnOrden()
	enRojo(t, r, map[string]string{"src/viejo.ts": "// @ts-ignore\nlet a = 1\n"})
	e := abrir(t, r)
	commitEn(t, e.Worktree, "src/viejo.ts", "// @ts-ignore\nlet a = 2\n")
	commitEn(t, e.Worktree, "src/a.go", "package a\n\nvar x = 1 //nolint:unused\n")
	commitEn(t, e.Worktree, "total.txt", "170\n")
	_, _, err := r.Verificar("", t0)
	if err == nil || !strings.Contains(err.Error(), "check supresiones falla") || !strings.Contains(err.Error(), "src/a.go:3: var x = 1 //nolint:unused") {
		t.Fatalf("err = %v", err)
	}
	if strings.Contains(err.Error(), "viejo.ts") {
		t.Fatalf("a suppression already in the red commit was flagged: %v", err)
	}
}

func TestSupresionesApagadaOExtendida(t *testing.T) {
	r := repoDeJuguete(t)
	r.Checks = append(slices.Clone(checksVerdes[:2]), config.Check{Nombre: "tests", Cmd: "true"},
		config.Check{Nombre: config.Supresiones, Patrones: []string{"NOSONAR"}})
	e := abrir(t, r)
	commitEn(t, e.Worktree, "a.java", "int x; // NOSONAR\n")
	commitEn(t, e.Worktree, "total.txt", "170\n")
	if _, _, err := r.Verificar("", t0); err == nil || !strings.Contains(err.Error(), "a.java:1: int x; // NOSONAR") {
		t.Fatalf("extended list: err = %v", err)
	}
	r.Checks[3] = config.Check{Nombre: config.Supresiones, Omitir: "legacy code"}
	commitEn(t, e.Worktree, "b.ts", "// eslint-disable-next-line\n")
	e, _, err := r.Verificar("", t0)
	if err != nil || e.Fase != Verificado || e.Recibo.Checks[3].Resultado != CheckOmitido {
		t.Fatalf("turned off: %v %+v", err, e.Recibo)
	}
}
