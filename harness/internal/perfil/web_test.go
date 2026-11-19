package perfil

import (
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

// repoGit es archivos con cada archivo versionado: Rutas lee git ls-files, como Detect.
func repoGit(t *testing.T, fs map[string]string) string {
	t.Helper()
	root := archivos(t, fs)
	for _, args := range [][]string{{"init", "-q"}, {"add", "-A"}} {
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	return root
}

func TestRutasNext(t *testing.T) {
	root := repoGit(t, map[string]string{
		"app/page.tsx":              "x",
		"app/(shop)/cart/page.tsx":  "x",
		"app/users/[id]/page.tsx":   "x",
		"app/users/layout.tsx":      "x",
		"pages/about.tsx":           "x",
		"pages/_app.tsx":            "x",
		"pages/api/x.ts":            "x",
		"node_modules/a/page.tsx":   "x",
		"src/app/@modal/a/page.jsx": "x",
	})
	want := []Ruta{{"/", "app/page.tsx"}, {"/a", "src/app/@modal/a/page.jsx"}, {"/about", "pages/about.tsx"}, {"/cart", "app/(shop)/cart/page.tsx"}, {"/users/[id]", "app/users/[id]/page.tsx"}}
	if got := Rutas(root); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v\nwant %v", got, want)
	}
}

func TestRutasReactRouter(t *testing.T) {
	root := repoGit(t, map[string]string{
		"src/main.tsx":  "import { Route } from 'react-router-dom'\n<Route path=\"/login\" element={<L/>} />\nconst r = [{ path: \"/settings\", element: <S/> }]\n",
		"src/other.tsx": "const x = { path: \"/not-a-route\" }\n",
	})
	want := []Ruta{{"/login", "src/main.tsx"}, {"/settings", "src/main.tsx"}}
	if got := Rutas(root); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v\nwant %v", got, want)
	}
}

func TestMapaUnaLineaPorRuta(t *testing.T) {
	m := Mapa([]Ruta{{"/", "app/page.tsx"}, {"/cart", "app/(shop)/cart/page.tsx"}})
	for _, w := range []string{"- `/` → `app/page.tsx` · reach: ? · shows: ?", "- `/cart` → `app/(shop)/cart/page.tsx` · reach: ? · shows: ?", "worker"} {
		if !strings.Contains(m, w) {
			t.Errorf("map lacks %q:\n%s", w, m)
		}
	}
	if strings.Count(m, "\n- ") != 2 {
		t.Errorf("want 2 route lines:\n%s", m)
	}
	if vacio := Mapa(nil); !strings.Contains(vacio, "no routes found") {
		t.Errorf("empty map: %s", vacio)
	}
}

func TestE2EConConfig(t *testing.T) {
	ch, por := E2E(archivos(t, map[string]string{
		"playwright.config.ts": "export default { use: { baseURL: 'http://localhost:3000' } }",
		"package.json":         `{"scripts":{"dev":"next dev"},"dependencies":{"next":"15.0.0"}}`,
	}))
	if ch.Cmd != "npx playwright test" || ch.Servidor != "npm run dev" || ch.Puerto != 3000 || por != "" {
		t.Fatalf("got %+v %q", ch, por)
	}
	ch, _ = E2E(archivos(t, map[string]string{
		"playwright.config.js": "module.exports = {}",
		"package.json":         `{"scripts":{"dev":"vite"},"devDependencies":{"vite":"6.0.0"}}`,
	}))
	if ch.Puerto != 5173 {
		t.Fatalf("vite: %+v", ch)
	}
}

func TestE2EUsaScript(t *testing.T) {
	ch, _ := E2E(archivos(t, map[string]string{
		"playwright.config.ts": "export default {}",
		"package.json":         `{"scripts":{"test:e2e":"playwright test","test":"vitest"}}`,
	}))
	if ch.Cmd != "npm run test:e2e" || ch.Servidor != "" {
		t.Fatalf("got %+v", ch)
	}
}

// Playwright's webServer starts the app itself; a second server on the same port would make it fail.
func TestE2ENoArrancaServidorConWebServer(t *testing.T) {
	ch, _ := E2E(archivos(t, map[string]string{
		"playwright.config.ts": "export default { webServer: { command: 'npm run dev', url: 'http://localhost:3000' } }",
		"package.json":         `{"scripts":{"dev":"next dev"},"dependencies":{"next":"15.0.0"}}`,
	}))
	if ch.Cmd != "npx playwright test" || ch.Servidor != "" || ch.Puerto != 0 {
		t.Fatalf("got %+v", ch)
	}
}

func TestE2ESinConfig(t *testing.T) {
	ch, por := E2E(archivos(t, map[string]string{"package.json": `{"dependencies":{"next":"15.0.0"}}`}))
	if ch.Cmd != "" || !strings.Contains(por, "playwright.config") {
		t.Fatalf("got %+v %q", ch, por)
	}
}
