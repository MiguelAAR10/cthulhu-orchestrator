package slice

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"orq/internal/config"
)

// Fixtures for per-folder checks (H14, K8): a Python root plus a dashboard/ with its own package.json and its own
// node_modules. The checks are small shell scripts; no real pnpm or python is needed.

var checksRaizVerdes = []config.Check{{Nombre: "lint", Cmd: "true"}, {Nombre: "types", Cmd: "true"}, {Nombre: "tests", Cmd: "true"}}

// repoConDashboard commits dashboard/package.json and leaves an untracked dashboard/node_modules/.bin/fake-test
// (git ignores it) whose exit code is codigo unless the folder holds a file called roto.
func repoConDashboard(t *testing.T, codigo string) Repo {
	t.Helper()
	r := repoDeJuguete(t)
	dash := filepath.Join(r.Root, "dashboard")
	os.MkdirAll(filepath.Join(dash, "node_modules", ".bin"), 0o755)
	os.WriteFile(filepath.Join(dash, "package.json"), []byte(`{"name":"dashboard"}`+"\n"), 0o644)
	os.WriteFile(filepath.Join(dash, "node_modules", ".bin", "fake-test"),
		[]byte("#!/bin/sh\nif [ -f roto ]; then echo 'dashboard roto'; exit 1; fi\necho dashboard "+codigo+"\nexit "+codigo+"\n"), 0o755)
	os.WriteFile(filepath.Join(r.Root, ".git", "info", "exclude"), []byte(".orq/\nnode_modules/\n"), 0o644)
	Git(r.Root, "add", "-A")
	Git(r.Root, "commit", "-qm", "dashboard")
	r.Checks = checksRaizVerdes
	r.Cobertura = true
	r.Carpetas = []config.Carpeta{{Ruta: "dashboard", Lista: []config.Check{
		{Nombre: "lint", Cmd: "true"}, {Nombre: "types", Cmd: "true"}, {Nombre: "tests", Cmd: "./node_modules/.bin/fake-test"}}}}
	return r
}

func abrirCon(t *testing.T, r Repo, alcance ...string) (*Estado, error) {
	t.Helper()
	e, _, err := r.Abrir("01", "total", "tests/s01.sh", examen, Opciones{SinConteo: "juguete", Bundle: []string{"tests/datos.txt"}, Sano: "test -f total.txt", Alcance: alcance}, t0)
	return e, err
}

// Python green + dashboard red must not pass the worker verify.
func TestCarpetaRojaNoPasaVerificar(t *testing.T) {
	r := repoConDashboard(t, "1")
	e, err := abrirCon(t, r)
	if err != nil {
		t.Fatal(err)
	}
	commitEn(t, e.Worktree, "dashboard/app.ts", "export const x = 1\n")
	commitEn(t, e.Worktree, "total.txt", "170\n")
	e, _, err = r.Verificar("", t0)
	if err == nil || !strings.Contains(err.Error(), "check dashboard/tests falla (exit 1)") {
		t.Fatalf("err = %v", err)
	}
	if e.Fase == Verificado {
		t.Fatal("verified with a red dashboard")
	}
}

// The dashboard check runs inside dashboard/ with that folder's own node_modules (never installed): green passes,
// and the receipt names the folder's checks.
func TestCarpetaVerdeUsaSuNodeModules(t *testing.T) {
	r := repoConDashboard(t, "0")
	e, err := abrirCon(t, r, "total.txt", "dashboard/app.ts")
	if err != nil {
		t.Fatal(err)
	}
	commitEn(t, e.Worktree, "dashboard/app.ts", "export const x = 1\n")
	commitEn(t, e.Worktree, "total.txt", "170\n")
	e, _, err = r.Verificar("", t0)
	if err != nil || e.Fase != Verificado {
		t.Fatalf("%v %+v", err, e)
	}
	var vio bool
	for _, c := range e.Recibo.Checks {
		vio = vio || c.Nombre == "dashboard/tests" && c.Resultado == CheckPaso && c.Salida == "dashboard 0"
	}
	if !vio {
		t.Fatalf("checks = %+v", e.Recibo.Checks)
	}
}

// A slice that does not touch dashboard/ does not pay for its checks.
func TestCarpetaNoTocadaNoCorre(t *testing.T) {
	r := repoConDashboard(t, "1")
	e, err := abrirCon(t, r)
	if err != nil {
		t.Fatal(err)
	}
	commitEn(t, e.Worktree, "total.txt", "170\n")
	e, _, err = r.Verificar("", t0)
	if err != nil || e.Fase != Verificado {
		t.Fatalf("%v %+v", err, e)
	}
	for _, c := range e.Recibo.Checks {
		if strings.HasPrefix(c.Nombre, "dashboard/") {
			t.Fatalf("ran %s on a slice that does not touch dashboard/", c.Nombre)
		}
	}
}

// Green at the worker commit, red on the merge commit: the base moved and breaks the dashboard. Not integrated.
func TestCarpetaRojaEnElMergeNoIntegra(t *testing.T) {
	r := repoConDashboard(t, "0")
	e, err := abrirCon(t, r)
	if err != nil {
		t.Fatal(err)
	}
	commitEn(t, e.Worktree, "dashboard/app.ts", "export const x = 1\n")
	commitEn(t, e.Worktree, "total.txt", "170\n")
	if _, _, err := r.Verificar("", t0); err != nil {
		t.Fatal(err)
	}
	commitEn(t, r.Root, "dashboard/roto", "x\n") // the base moves and breaks the dashboard on the merge
	mainAntes, _ := Git(r.Root, "rev-parse", "main")
	_, _, _, err = r.Cerrar("", true, "", t0)
	if err == nil || !strings.Contains(err.Error(), "check dashboard/tests falla") {
		t.Fatalf("integrated a merge with a red dashboard: %v", err)
	}
	if st, _ := r.Load(""); st == nil || st.Fase != Integrando || st.Integrado != "" {
		t.Fatalf("the close should stay in INTEGRANDO with nothing integrated: %+v", st)
	}
	if m, _ := Git(r.Root, "rev-parse", "main"); m != mainAntes {
		t.Fatalf("main moved: %s -> %s", mainAntes, m)
	}
	if _, err := Git(r.Root, "rev-parse", "--verify", "-q", refIntegrado("01")); err == nil {
		t.Fatal("the integrado ref was written for a red merge")
	}
	if _, err := os.Stat(filepath.Join(r.Root, "dashboard", "app.ts")); err == nil {
		t.Fatal("published the red merge")
	}
	// Recoverable: once the base is fixed, the same close integrates on the new base.
	Git(r.Root, "rm", "-q", "dashboard/roto")
	Git(r.Root, "commit", "-qm", "fix base")
	if _, _, _, err := r.Cerrar("", true, "", t0); err != nil {
		t.Fatalf("retry after fixing the base: %v", err)
	}
	if _, err := os.Stat(filepath.Join(r.Root, "dashboard", "app.ts")); err != nil {
		t.Fatal("the retry did not publish")
	}
}

// Flat (root) checks are re-run on the merge commit too, not only on the worker commit.
func TestChecksRaizSeRecorrenEnElMerge(t *testing.T) {
	r := repoDeJuguete(t)
	r.Cobertura = true
	r.Checks = []config.Check{{Nombre: "lint", Cmd: "true"}, {Nombre: "types", Cmd: "true"}, {Nombre: "tests", Cmd: "test ! -f roto"}}
	e := abrir(t, r)
	commitEn(t, e.Worktree, "total.txt", "170\n")
	if _, _, err := r.Verificar("", t0); err != nil {
		t.Fatal(err)
	}
	commitEn(t, r.Root, "roto", "x\n")
	if _, _, _, err := r.Cerrar("", true, "", t0); err == nil || !strings.Contains(err.Error(), "check tests falla") {
		t.Fatalf("integrated with a red root check on the merge: %v", err)
	}
}

// A story whose scope touches a folder with its own project and no declared check does not open.
func TestAbrirRechazaCarpetaSinChecks(t *testing.T) {
	r := repoConDashboard(t, "0")
	r.Carpetas = nil // only the flat Python checks, like Real State today
	_, err := abrirCon(t, r, "src/app.py", "dashboard/components/LaunchBoard.tsx")
	if err == nil || !strings.Contains(err.Error(), "dashboard/") || !strings.Contains(err.Error(), "carpetas") {
		t.Fatalf("opened a story touching an undeclared folder: %v", err)
	}
	if e, _ := r.Load(""); e != nil {
		t.Fatalf("left a slice behind: %+v", e)
	}
	// Explicitly exempted: it opens.
	r.Carpetas = []config.Carpeta{{Ruta: "dashboard", Exenta: "prototype, no tests yet"}}
	if _, err := abrirCon(t, r, "src/app.py", "dashboard/components/LaunchBoard.tsx"); err != nil {
		t.Fatal(err)
	}
}

// The diff counts too: a worker that adds a new project folder with no checks is not verified.
func TestVerificarRechazaDiffEnCarpetaSinChecks(t *testing.T) {
	r := repoDeJuguete(t)
	r.Cobertura = true
	r.Checks = checksRaizVerdes
	e := abrir(t, r)
	os.MkdirAll(filepath.Join(e.Worktree, "web"), 0o755)
	commitEn(t, e.Worktree, "web/package.json", "{}\n")
	commitEn(t, e.Worktree, "total.txt", "170\n")
	e, _, err := r.Verificar("", t0)
	if err == nil || !strings.Contains(err.Error(), "web/") {
		t.Fatalf("verified a diff in an undeclared folder: %v", err)
	}
	if e.Fase == Verificado {
		t.Fatal("verified")
	}
}

// Old flat config, no sub-projects: the full cycle works as before, with coverage on.
func TestConfigPlanaSigueFuncionando(t *testing.T) {
	r := repoDeJuguete(t)
	r.Cobertura = true
	r.Checks = checksRaizVerdes
	e, err := abrirCon(t, r, "total.txt", "README.md")
	if err != nil {
		t.Fatal(err)
	}
	commitEn(t, e.Worktree, "total.txt", "170\n")
	if _, _, err := r.Verificar("", t0); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := r.Cerrar("", true, "", t0); err != nil {
		t.Fatal(err)
	}
}

// Every check in the same copy sees the owner's node_modules, not only the first one: the first run
// leaves the mount point behind and that must not hide the dependencies from the next checks.
func TestDependenciasEnCadaCheck(t *testing.T) {
	r := repoDeJuguete(t)
	os.MkdirAll(filepath.Join(r.Root, "node_modules", ".bin"), 0o755)
	os.WriteFile(filepath.Join(r.Root, "node_modules", ".bin", "ok"), []byte("#!/bin/sh\nexit 0\n"), 0o755)
	os.WriteFile(filepath.Join(r.Root, ".git", "info", "exclude"), []byte(".orq/\nnode_modules/\n"), 0o644)
	bin := "./node_modules/.bin/ok"
	r.Checks = []config.Check{{Nombre: "lint", Cmd: bin}, {Nombre: "types", Cmd: bin}, {Nombre: "tests", Cmd: bin}}
	e := abrir(t, r)
	commitEn(t, e.Worktree, "total.txt", "170\n")
	if e, _, err := r.Verificar("", t0); err != nil || e.Fase != Verificado {
		t.Fatalf("%v", err)
	}
}

// Identity orq (Cobertura off) keeps 1.0.0 behaviour: an undeclared folder opens and verifies, and
// the checks are not re-run on the merge commit.
func TestSinCoberturaComoAntes(t *testing.T) {
	r := repoConDashboard(t, "0")
	r.Cobertura, r.Carpetas = false, nil
	r.Checks = []config.Check{{Nombre: "lint", Cmd: "true"}, {Nombre: "types", Cmd: "true"}, {Nombre: "tests", Cmd: "test ! -f roto"}}
	e, err := abrirCon(t, r, "total.txt", "dashboard/app.ts")
	if err != nil {
		t.Fatal(err)
	}
	commitEn(t, e.Worktree, "dashboard/app.ts", "export const x = 1\n")
	commitEn(t, e.Worktree, "total.txt", "170\n")
	if _, _, err := r.Verificar("", t0); err != nil {
		t.Fatal(err)
	}
	commitEn(t, r.Root, "roto", "x\n")
	if _, _, _, err := r.Cerrar("", true, "", t0); err != nil {
		t.Fatalf("orq 1.0.0 does not re-run checks on the merge: %v", err)
	}
}
