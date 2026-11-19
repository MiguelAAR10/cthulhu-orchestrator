package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"orq/internal/config"
)

// repoNext es repoCLI con una app tipo Next versionada: dos rutas, un package.json y, si conPW,
// una config de Playwright.
func repoNext(t *testing.T, conPW bool) string {
	t.Helper()
	root := repoCLI(t)
	t.Setenv("ORQ_STATE", t.TempDir())
	escribirEn(t, root, "package.json", `{"scripts":{"dev":"next dev","lint":"next lint"},"dependencies":{"next":"15.0.0","react":"19.0.0"}}`)
	escribirEn(t, root, "app/page.tsx", "export default function P() { return null }\n")
	escribirEn(t, root, "app/(shop)/cart/page.tsx", "export default function C() { return null }\n")
	if conPW {
		escribirEn(t, root, "playwright.config.ts", "export default { use: { baseURL: 'http://localhost:3000' } }\n")
	}
	if out, err := exec.Command("git", "-C", root, "add", "-A").CombinedOutput(); err != nil {
		t.Fatalf("git add: %s", out)
	}
	return root
}

func TestInitWebEscribeMapaYE2E(t *testing.T) {
	root := repoNext(t, true)
	out, errs, code := run(t, "init")
	if code != 0 {
		t.Fatalf("init: %d %s %s", code, out, errs)
	}
	mapa := filepath.Join(root, "docs", "mapa.md")
	raw, err := os.ReadFile(mapa)
	if err != nil {
		t.Fatal(err)
	}
	contiene(t, string(raw), "- `/` → `app/page.tsx` · reach: ? · shows: ?", "- `/cart` → `app/(shop)/cart/page.tsx`")
	c, err := config.CargarDe(config.RutaProyecto(root))
	if err != nil {
		t.Fatal(err)
	}
	if e := c.Checks[config.E2E]; e.Cmd != "npx playwright test" || e.Servidor != "npm run dev" || e.Puerto != 3000 {
		t.Fatalf("e2e = %+v", e)
	}
	contiene(t, out, "mapa de rutas en "+mapa, "check e2e: npx playwright test · servidor npm run dev en 127.0.0.1:3000")

	escribirEn(t, root, "docs/mapa.md", "mine\n")
	if out, errs, code := run(t, "init"); code != 0 {
		t.Fatalf("second init: %d %s %s", code, out, errs)
	}
	if raw, _ := os.ReadFile(mapa); string(raw) != "mine\n" {
		t.Fatalf("init overwrote the map: %s", raw)
	}
	// An old map in .orq/ is still the map: init does not write a second one.
	os.Remove(mapa)
	escribirEn(t, root, ".orq/mapa.md", "old\n")
	if out, _, _ := run(t, "init"); !strings.Contains(out, "ya existe") {
		t.Fatalf("second map over an old one:\n%s", out)
	}
	if _, err := os.Stat(mapa); err == nil {
		t.Fatal("init wrote docs/mapa.md next to an old .orq/mapa.md")
	}
}

func TestInitWebSinPlaywrightNoEscribeE2E(t *testing.T) {
	root := repoNext(t, false)
	out, errs, code := run(t, "init")
	if code != 0 {
		t.Fatalf("init: %d %s %s", code, out, errs)
	}
	c, _ := config.CargarDe(config.RutaProyecto(root))
	if _, ok := c.Checks[config.E2E]; ok {
		t.Fatalf("e2e without a Playwright config: %+v", c.Checks)
	}
	contiene(t, out, "sin check e2e: sin playwright.config")
	if _, err := os.Stat(filepath.Join(root, "docs", "mapa.md")); err != nil {
		t.Fatalf("the map is written for every web project: %v", err)
	}
}

func TestInitSugiereE2EConChecksDelOwner(t *testing.T) {
	root := repoNext(t, true)
	propio := `{"checks": {"lint": {"cmd": "npm run lint"}, "types": {"omitir": "x"}, "tests": {"omitir": "y"}}}`
	escribirEn(t, root, ".orq/config.json", propio)
	out, errs, code := run(t, "init")
	if code != 0 {
		t.Fatalf("init: %d %s %s", code, out, errs)
	}
	if raw, _ := os.ReadFile(config.RutaProyecto(root)); string(raw) != propio {
		t.Fatalf("init rewrote the Owner's file:\n%s", raw)
	}
	contiene(t, out, `añade a checks: "e2e": {"cmd":"npx playwright test","servidor":"npm run dev","puerto":3000}`)
	if strings.Contains(out, "check e2e:") {
		t.Fatalf("claims an e2e check it did not write:\n%s", out)
	}
}

// homeCaja: a temp HOME with a stub npx in ~/.local/bin and, if nav, a Playwright browsers dir.
func homeCaja(t *testing.T, nav bool) (home, cache string) {
	t.Helper()
	home = filepath.Join(t.TempDir(), "home")
	bin := filepath.Join(home, ".local", "bin")
	os.MkdirAll(bin, 0o755)
	os.WriteFile(filepath.Join(bin, "npx"), []byte("#!/bin/sh\necho npx 10.0.0\n"), 0o755)
	cache = filepath.Join(home, ".cache", "ms-playwright")
	if nav {
		os.MkdirAll(cache, 0o755)
	}
	t.Setenv("HOME", home)
	t.Setenv("PLAYWRIGHT_BROWSERS_PATH", "")
	t.Setenv("ORQ_LIMITS", filepath.Join(t.TempDir(), "limits.json"))
	t.Setenv("PATH", bin+":/usr/bin:/bin")
	return home, cache
}

const checksWeb = `{"checks": {"lint": {"cmd": "true"}, "types": {"cmd": "true"}, "tests": {"cmd": "true"}, "e2e": {"cmd": "npx playwright test"}}}`

func TestCheckCajaE2E(t *testing.T) {
	root := repoNext(t, true)
	_, cache := homeCaja(t, false)
	escribirEn(t, root, ".orq/config.json", checksWeb)
	t.Chdir(root)
	out, _, code := run(t, "check", "--caja")
	if code != 1 {
		t.Fatalf("exit %d without browsers:\n%s", code, out)
	}
	contiene(t, out, "e2e  no corre en la caja: no hay navegadores de Playwright en "+cache)

	os.MkdirAll(cache, 0o755)
	out, _, code = run(t, "check", "--caja")
	if code != 0 {
		t.Fatalf("exit %d with browsers:\n%s", code, out)
	}
	contiene(t, out, "e2e  corre en la caja: npx → ", "navegadores de playwright → "+cache, cache)
}

func TestCheckCajaSinE2EDicePorQue(t *testing.T) {
	root := repoNext(t, false)
	homeCaja(t, true)
	escribirEn(t, root, ".orq/config.json", `{"checks": {"lint": {"cmd": "true"}, "types": {"cmd": "true"}, "tests": {"cmd": "true"}}}`)
	t.Chdir(root)
	out, _, code := run(t, "check", "--caja")
	if code != 0 {
		t.Fatalf("exit %d:\n%s", code, out)
	}
	contiene(t, out, "e2e  sin check: sin playwright.config")
}

func TestDecidirMuestraCapturas(t *testing.T) {
	sliceVerdeCon(t, nil, conChecks(`{"checks": {"lint": {"cmd": "true"}, "types": {"cmd": "true"}, "tests": {"cmd": "true"},
		"e2e": {"cmd": "mkdir -p test-results/carrito && printf png > test-results/carrito/final.png"}}}`))
	out, _, code := run(t, "slice", "decidir")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	contiene(t, out, "e2e paso · exit 0 · mkdir -p test-results/carrito", "captura: ", filepath.Join("capturas", "S-01-a1", "test-results", "carrito", "final.png"))
}

func TestGuiaWorkerWeb(t *testing.T) {
	raw, err := guias.ReadFile("guias/worker.md")
	if err != nil {
		t.Fatal(err)
	}
	contiene(t, string(raw), "docs/mapa.md", "real browser", "Playwright MCP", "npx playwright", "any route you add", "same commit")
}

func TestContratoNombraElMapa(t *testing.T) {
	root := repoCLI(t)
	t.Setenv("ORQ_STATE", t.TempDir())
	if _, errs, code := run(t, append(append([]string{}, abrirBase...), "--reporte", "out.tap", "--formato", "tap")...); code != 0 {
		t.Fatalf("abrir: %d %s", code, errs)
	}
	run(t, "slice", "tomar", "w1")
	out, _, _ := run(t, "slice", "contrato")
	if strings.Contains(out, "mapa.md") {
		t.Fatalf("a map line without a map:\n%s", out)
	}
	escribirEn(t, root, ".orq/mapa.md", "# Feature map\n") // an old map still counts
	out, errs, code := run(t, "slice", "contrato")
	if code != 0 {
		t.Fatalf("contrato: %d %s", code, errs)
	}
	contiene(t, out, filepath.Join(root, ".orq", "mapa.md"), "navegador real")
	escribirEn(t, root, "docs/mapa.md", "# Feature map\n")
	out, _, _ = run(t, "slice", "contrato")
	contiene(t, out, "docs/mapa.md", "navegador real", "en el mismo commit")
}

// The worker adds the route's map line in its own commit; the decision page shows the new lines,
// and with a declared scope the map is not "outside the scope".
func TestDecidirMuestraElMapa(t *testing.T) {
	sliceVerdeCon(t, []string{"--alcance", "total.txt"}, func(wt string) {
		os.MkdirAll(filepath.Join(wt, "docs"), 0o755)
		commitClon(t, wt, "docs/mapa.md", "# Feature map\n- `/cart` → `app/cart/page.tsx` · reach: header link · shows: the total\n")
	})
	out, _, code := run(t, "slice", "decidir")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	contiene(t, out, "mapa: - `/cart` → `app/cart/page.tsx` · reach: header link · shows: the total")
	if strings.Contains(out, "docs/mapa.md fuera del alcance") {
		t.Fatalf("the map counted as out of scope:\n%s", out)
	}
}
