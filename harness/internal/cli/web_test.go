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
	mapa := filepath.Join(root, ".orq", "mapa.md")
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

	escribirEn(t, root, ".orq/mapa.md", "mine\n")
	if out, errs, code := run(t, "init"); code != 0 {
		t.Fatalf("second init: %d %s %s", code, out, errs)
	}
	if raw, _ := os.ReadFile(mapa); string(raw) != "mine\n" {
		t.Fatalf("init overwrote the map: %s", raw)
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
	if _, err := os.Stat(filepath.Join(root, ".orq", "mapa.md")); err != nil {
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
