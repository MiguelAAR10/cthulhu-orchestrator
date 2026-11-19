package slice

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"orq/internal/config"
)

// e2eStub es el check e2e de un proyecto tipo Next: Playwright con el servidor de la app delante.
var e2eStub = config.Check{Nombre: config.E2E, Cmd: "npx playwright test", Servidor: "npm run dev", Puerto: 4173}

// homeWeb: a temp HOME with stub npm and npx in ~/.local/bin and a Playwright browsers dir in
// ~/.cache/ms-playwright. `npm run dev` runs dev; `npx playwright test` runs test. No browser, no network.
func homeWeb(t *testing.T, dev, test string) string {
	t.Helper()
	home := filepath.Join(t.TempDir(), "home")
	t.Setenv("HOME", home)
	t.Setenv("PLAYWRIGHT_BROWSERS_PATH", "")
	t.Setenv("ORQ_STATE", filepath.Join(t.TempDir(), "state"))
	t.Setenv("ORQ_LIMITS", filepath.Join(t.TempDir(), "limits.json"))
	bin := filepath.Join(home, ".local", "bin")
	os.MkdirAll(bin, 0o755)
	os.MkdirAll(filepath.Join(home, ".cache", "ms-playwright", "chromium-1"), 0o755)
	os.WriteFile(filepath.Join(home, ".cache", "ms-playwright", "chromium-1", "ok"), nil, 0o644)
	for n, body := range map[string]string{"npm": dev, "npx": test} {
		if err := os.WriteFile(filepath.Join(bin, n), []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	return home
}

// devStub serves the copy on 127.0.0.1:4173 and leaves a mark that it started.
const devStub = `[ "$1 $2" = "run dev" ] || exit 64
touch dev-arrancado
exec python3 -m http.server 4173 --bind 127.0.0.1`

// pwStub is `npx playwright test`: it needs the server up and the browsers visible, then saves a screenshot.
const pwStub = `[ "$1 $2" = "playwright test" ] || exit 64
test -f dev-arrancado || { echo "sin servidor"; exit 3; }
test -f "$PLAYWRIGHT_BROWSERS_PATH/chromium-1/ok" || { echo "sin navegadores"; exit 4; }
bash -c ': </dev/tcp/127.0.0.1/4173' || { echo "puerto cerrado"; exit 5; }
echo "servidor y navegadores ok"
mkdir -p test-results/carrito
printf 'PNG-carrito' > test-results/carrito/test-finished-1.png`

func reciboCheck(t *testing.T, rec *Recibo, nombre string) ResultadoCheck {
	t.Helper()
	for _, c := range rec.Checks {
		if c.Nombre == nombre {
			return c
		}
	}
	t.Fatalf("no %s in the receipt: %+v", nombre, rec.Checks)
	return ResultadoCheck{}
}

func TestE2EEnLaCaja(t *testing.T) {
	homeWeb(t, devStub, pwStub)
	r := repoDeJuguete(t)
	r.Checks = []config.Check{{Nombre: "lint", Cmd: "true"}, {Nombre: "types", Cmd: "true"}, {Nombre: "tests", Cmd: "true"}, e2eStub}
	e := abrir(t, r)
	commitEn(t, e.Worktree, "total.txt", "170\n")
	e, out, err := r.Verificar("", t0)
	if err != nil || e.Fase != Verificado {
		t.Fatalf("%v %s\n%s", err, e.Fase, out)
	}
	c := reciboCheck(t, e.Recibo, config.E2E)
	if c.Resultado != CheckPaso || c.Cmd != "npx playwright test" || c.Servidor != "npm run dev" || !strings.Contains(c.Salida, "servidor y navegadores ok") {
		t.Fatalf("e2e = %+v", c)
	}
	if len(c.Capturas) != 1 || !strings.HasPrefix(c.Capturas[0], filepath.Join(r.OrqDir, "capturas")+"/") {
		t.Fatalf("capturas = %v", c.Capturas)
	}
	if raw, err := os.ReadFile(c.Capturas[0]); err != nil || string(raw) != "PNG-carrito" {
		t.Fatalf("screenshot %s: %q %v", c.Capturas[0], raw, err)
	}
}

func TestE2EServidorQueNoAbre(t *testing.T) {
	home := homeWeb(t, devStub, pwStub)
	r := repoDeJuguete(t)
	r.Checks = []config.Check{{Nombre: "lint", Cmd: "true"}, {Nombre: "types", Cmd: "true"}, {Nombre: "tests", Cmd: "true"}, e2eStub}
	e := abrir(t, r)
	// The server dies after abrir (abrir's --sano runs behind it too).
	os.WriteFile(filepath.Join(home, ".local", "bin", "npm"), []byte("#!/bin/sh\necho \"boom: falta .env\"; exit 2\n"), 0o755)
	commitEn(t, e.Worktree, "total.txt", "170\n")
	t1 := time.Now()
	e, _, err := r.Verificar("", t0)
	if err == nil || e.Fase == Verificado {
		t.Fatal("verified with a server that never opened its port")
	}
	rs := leerJSONL[Recibo](r.recibosPath())
	c := reciboCheck(t, &rs[len(rs)-1], config.E2E)
	if c.Resultado != CheckFallo || !strings.Contains(c.Salida, "127.0.0.1:4173") || !strings.Contains(c.Salida, "boom: falta .env") {
		t.Fatalf("e2e = %+v", c)
	}
	if d := time.Since(t1); d > 30*time.Second {
		t.Fatalf("took %s: a dead server should be noticed at once", d)
	}
}

func TestCapturasNoSiguenEnlaces(t *testing.T) {
	homeWeb(t, devStub, pwStub+"\nln -s /etc/passwd test-results/fuga.png")
	r := repoDeJuguete(t)
	r.Checks = []config.Check{{Nombre: "lint", Cmd: "true"}, {Nombre: "types", Cmd: "true"}, {Nombre: "tests", Cmd: "true"}, e2eStub}
	e := abrir(t, r)
	commitEn(t, e.Worktree, "total.txt", "170\n")
	e, out, err := r.Verificar("", t0)
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	c := reciboCheck(t, e.Recibo, config.E2E)
	if len(c.Capturas) != 1 || strings.Contains(c.Capturas[0], "fuga") {
		t.Fatalf("capturas = %v", c.Capturas)
	}
}

// A Playwright exam runs with the app server of checks.e2e started in the box.
func TestExamenPlaywrightArrancaServidor(t *testing.T) {
	homeWeb(t, devStub, pwStub+"\ncmp -s total.txt tests/datos.txt")
	r := repoDeJuguete(t)
	r.Checks = []config.Check{{Nombre: "lint", Cmd: "true"}, {Nombre: "types", Cmd: "true"}, {Nombre: "tests", Cmd: "true"}, e2eStub}
	e, out, err := r.Abrir("01", "total", "tests/s01.sh", "npx playwright test", Opciones{SinConteo: "juguete", Bundle: []string{"tests/datos.txt"}, Sano: "test -f total.txt"}, t0)
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if !strings.Contains(out, "servidor y navegadores ok") {
		t.Fatalf("the red did not come from the assertion:\n%s", out)
	}
	commitEn(t, e.Worktree, "total.txt", "170\n")
	e, out, err = r.Verificar("", t0)
	if err != nil || e.Fase != Verificado {
		t.Fatalf("%v %s\n%s", err, e.Fase, out)
	}
}

// With a counted report the exam still runs behind the server (the real run's bug: the report
// prefix rebuilt the command from e.Cmd and dropped the server).
func TestExamenPlaywrightConReporteArrancaServidor(t *testing.T) {
	homeWeb(t, devStub, pwStub+"\nif cmp -s total.txt tests/datos.txt; then echo 'ok 1 total' > out.tap; else echo 'not ok 1 total' > out.tap; exit 1; fi")
	r := repoDeJuguete(t)
	r.Checks = []config.Check{{Nombre: "lint", Cmd: "true"}, {Nombre: "types", Cmd: "true"}, {Nombre: "tests", Cmd: "true"}, e2eStub}
	e, out, err := r.Abrir("01", "total", "tests/s01.sh", "npx playwright test", Opciones{Reporte: "out.tap", Formato: "tap", Bundle: []string{"tests/datos.txt"}, Sano: "test -f total.txt"}, t0)
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if !strings.Contains(out, "servidor y navegadores ok") {
		t.Fatalf("the red did not come from the assertion:\n%s", out)
	}
	commitEn(t, e.Worktree, "total.txt", "170\n")
	if e, out, err = r.Verificar("", t0); err != nil || e.Fase != Verificado {
		t.Fatalf("%v %s\n%s", err, e.Fase, out)
	}
}

// pwOnlyStub is Playwright honoring .only: a spec with test.only( runs alone (and passes) unless
// --forbid-only is given; otherwise the exam's own assertion runs.
const pwOnlyStub = `[ "$1 $2" = "playwright test" ] || exit 64
if grep -qs 'test.only(' e2e/*.spec.ts; then
	case " $* " in *" --forbid-only "*) echo "Error: focused item found in the --forbid-only mode"; exit 1;; esac
	echo "1 passed (only)"; exit 0
fi
cmp -s total.txt tests/datos.txt`

// Review repro 1: a new spec with test.only, feature not built, must not verify. Each layer alone
// stops it: --forbid-only on the exam and e2e, and the .only( mark in supresiones.
func TestTestOnlyNoVerifica(t *testing.T) {
	for _, capa := range []string{"forbid-only", "supresiones"} {
		t.Run(capa, func(t *testing.T) {
			homeWeb(t, devStub, pwOnlyStub)
			r := repoDeJuguete(t)
			sup := config.Check{Nombre: config.Supresiones}
			cmd := "npx playwright test"
			if capa == "forbid-only" {
				sup.Omitir = "only the flag layer"
			} else {
				cmd = "npx playwright test --reporter=line" // still gets the flag; the mark must catch it too
			}
			r.Checks = []config.Check{{Nombre: "lint", Cmd: "true"}, {Nombre: "types", Cmd: "true"}, {Nombre: "tests", Cmd: "true"},
				{Nombre: config.E2E, Cmd: cmd, Servidor: e2eStub.Servidor, Puerto: e2eStub.Puerto}, sup}
			e, out, err := r.Abrir("01", "total", "tests/s01.sh", cmd, Opciones{SinConteo: "juguete", Bundle: []string{"tests/datos.txt"}, Sano: "test -f total.txt"}, t0)
			if err != nil {
				t.Fatalf("%v\n%s", err, out)
			}
			os.MkdirAll(filepath.Join(e.Worktree, "e2e"), 0o755)
			commitEn(t, e.Worktree, "e2e/smoke.spec.ts", "test.only('page loads', async ({ page }) => {})\n")
			e, out, err = r.Verificar("", t0)
			if err == nil || e.Fase == Verificado {
				t.Fatalf("verified with test.only and no feature:\n%s", out)
			}
			if capa == "supresiones" && !strings.Contains(err.Error(), "e2e/smoke.spec.ts:1: test.only(") {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

// npmStub runs `npm run dev` as devStub and `npm run test:e2e` as `npx playwright test`.
const npmStub = `if [ "$1 $2" = "run dev" ]; then touch dev-arrancado; exec python3 -m http.server 4173 --bind 127.0.0.1; fi
[ "$1 $2" = "run test:e2e" ] || exit 64
shift 2
exec npx playwright test "$@"`

// Review repro 2: an exam (and --sano) through an npm script gets the server of checks.e2e too.
func TestExamenNpmScriptArrancaServidor(t *testing.T) {
	homeWeb(t, npmStub, pwStub+"\ncmp -s total.txt tests/datos.txt")
	r := repoDeJuguete(t)
	r.Checks = []config.Check{{Nombre: "lint", Cmd: "true"}, {Nombre: "types", Cmd: "true"}, {Nombre: "tests", Cmd: "true"}, e2eStub}
	e, out, err := r.Abrir("01", "total", "tests/s01.sh", "npm run test:e2e", Opciones{SinConteo: "juguete", Bundle: []string{"tests/datos.txt"}, Sano: "test -f dev-arrancado"}, t0)
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if !strings.Contains(out, "servidor y navegadores ok") {
		t.Fatalf("the red did not come from the assertion:\n%s", out)
	}
	commitEn(t, e.Worktree, "total.txt", "170\n")
	if e, out, err = r.Verificar("", t0); err != nil || e.Fase != Verificado {
		t.Fatalf("%v %s\n%s", err, e.Fase, out)
	}
}

// Review, lower risk: the files behind checks.e2e.servidor are runner files too. A worker that points
// the server at a stub has to say so with --acepto-runner.
func TestServidorVigilado(t *testing.T) {
	r := repoDeJuguete(t)
	enRojo(t, r, map[string]string{"server.js": "listen()\n", "vite.config.ts": "export default {}\n", "next.config.mjs": "export default {}\n", "playwright.config.ts": "export default {}\n"})
	r.Checks = []config.Check{{Nombre: "lint", Cmd: "true"}, {Nombre: "types", Cmd: "true"}, {Nombre: "tests", Cmd: "true"},
		{Nombre: config.E2E, Cmd: "true", Servidor: "node server.js --port 4173", Puerto: 4173}}
	home := homeWeb(t, devStub, pwStub)
	os.WriteFile(filepath.Join(home, ".local", "bin", "node"), []byte("#!/bin/sh\nexec python3 -m http.server 4173 --bind 127.0.0.1\n"), 0o755)
	e := abrir(t, r)
	var got []string
	for _, v := range e.Vigilados {
		got = append(got, v.Ruta)
	}
	for _, f := range []string{"server.js", "vite.config.ts", "next.config.mjs", "playwright.config.ts"} {
		if !slices.Contains(got, f) {
			t.Fatalf("%s not watched: %v", f, got)
		}
	}
	commitEn(t, e.Worktree, "server.js", "stub()\n")
	commitEn(t, e.Worktree, "total.txt", "170\n")
	var rc *ErrRunnerCambio
	if _, _, err := r.Verificar("", t0); !errors.As(err, &rc) || !slices.Equal(rc.Archivos, []string{"server.js"}) {
		t.Fatalf("got %v", err)
	}
	if e, out, err := r.Verificar("", t0, VerificarOpc{AceptoRunner: "Owner approved the new server"}); err != nil || e.Fase != Verificado {
		t.Fatalf("%v\n%s", err, out)
	}
}

// pwFocoStub is Playwright with a JUnit report: any focused spec (any spelling: `test.only (`,
// `test['only']`, an alias) runs alone and passes; otherwise the exam's assertion runs.
const pwFocoStub = `[ "$1 $2" = "playwright test" ] || exit 64
if grep -Eqs "only|grep" e2e/*.spec.ts; then
	printf '<testsuites><testsuite><testcase name="page loads" classname="smoke.spec.ts"></testcase></testsuite></testsuites>' > results.xml
	echo "1 passed (focused)"; exit 0
fi
f=''
cmp -s total.txt tests/datos.txt || f='<failure message="expected 170"/>'
printf '<testsuites><testsuite><testcase name="cart shows the total" classname="total.spec.ts">%s</testcase></testsuite></testsuites>' "$f" > results.xml
[ -z "$f" ]`

// Review 2 repro: `test.only (` (with a space) through `npm run test:e2e` slips past the .only( mark
// and past --forbid-only. The exam's own failing tests are in the contract: they must pass in the report.
func TestFocoNoSaltaElExamen(t *testing.T) {
	for _, spec := range []string{
		"test.only ('page loads', async ({ page }) => {})\n",
		"test['only']('page loads', async ({ page }) => {})\n",
		"const t = test.only; t('page loads', async () => {})\n",
	} {
		t.Run(spec[:12], func(t *testing.T) {
			homeWeb(t, npmStub, pwFocoStub)
			r := repoDeJuguete(t)
			r.Checks = []config.Check{{Nombre: "lint", Cmd: "true"}, {Nombre: "types", Cmd: "true"}, {Nombre: "tests", Cmd: "true"},
				{Nombre: config.E2E, Cmd: "npm run test:e2e", Servidor: e2eStub.Servidor, Puerto: e2eStub.Puerto}, {Nombre: config.Supresiones}}
			e, out, err := r.Abrir("01", "total", "tests/s01.sh", "npm run test:e2e", Opciones{Reporte: "results.xml", Formato: "junit", Bundle: []string{"tests/datos.txt"}}, t0)
			if err != nil {
				t.Fatalf("%v\n%s", err, out)
			}
			if !slices.Equal(e.PruebasExamen, []string{"total.spec.ts › cart shows the total"}) {
				t.Fatalf("contract exam tests = %q", e.PruebasExamen)
			}
			os.MkdirAll(filepath.Join(e.Worktree, "e2e"), 0o755)
			commitEn(t, e.Worktree, "e2e/smoke.spec.ts", spec)
			e, out, err = r.Verificar("", t0)
			if err == nil || e.Fase == Verificado {
				t.Fatalf("verified with a focused test and no feature:\n%s", out)
			}
			if !strings.Contains(err.Error(), "total.spec.ts › cart shows the total") {
				t.Fatalf("err does not name the missing exam test: %v", err)
			}
			commitEn(t, e.Worktree, "e2e/smoke.spec.ts", "test('page loads', async () => {})\n")
			commitEn(t, e.Worktree, "total.txt", "170\n")
			if e, out, err = r.Verificar("", t0); err != nil || e.Fase != Verificado {
				t.Fatalf("%v %s\n%s", err, e.Fase, out)
			}
		})
	}
}

// pwForbidStub honors --forbid-only like Playwright: a focused spec (any spelling) fails with it, runs alone without it.
const pwForbidStub = `[ "$1 $2" = "playwright test" ] || exit 64
if grep -Eqs "only" e2e/*.spec.ts; then
	case " $* " in *" --forbid-only "*) echo "Error: focused item found in the --forbid-only mode"; exit 1;; esac
	echo "1 passed (focused)"; exit 0
fi
cmp -s total.txt tests/datos.txt`

// Review 2, second layer: through `npm run test:e2e` (the init default) whose script is `playwright test`,
// the exam and checks.e2e get `-- --forbid-only`. No report here, so only this layer can stop it.
func TestForbidOnlyPorScriptNpm(t *testing.T) {
	homeWeb(t, npmStub, pwForbidStub)
	r := repoDeJuguete(t)
	os.WriteFile(filepath.Join(r.Root, "package.json"), []byte(`{"scripts":{"test:e2e":"playwright test"}}`), 0o644)
	Git(r.Root, "add", "-A")
	Git(r.Root, "commit", "-qm", "package.json")
	sup := config.Check{Nombre: config.Supresiones, Omitir: "only the flag layer"}
	r.Checks = []config.Check{{Nombre: "lint", Cmd: "true"}, {Nombre: "types", Cmd: "true"}, {Nombre: "tests", Cmd: "true"},
		{Nombre: config.E2E, Cmd: "npm run test:e2e", Servidor: e2eStub.Servidor, Puerto: e2eStub.Puerto}, sup}
	e, out, err := r.Abrir("01", "total", "tests/s01.sh", "npm run test:e2e", Opciones{SinConteo: "juguete", Bundle: []string{"tests/datos.txt"}}, t0)
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	os.MkdirAll(filepath.Join(e.Worktree, "e2e"), 0o755)
	commitEn(t, e.Worktree, "e2e/smoke.spec.ts", "test.only ('page loads', async ({ page }) => {})\n")
	e, out, err = r.Verificar("", t0)
	if err == nil || e.Fase == Verificado {
		t.Fatalf("verified with test.only through an npm script:\n%s", out)
	}
	if !strings.Contains(out, "focused item") {
		t.Fatalf("the exam did not get --forbid-only:\n%s", out)
	}
}
