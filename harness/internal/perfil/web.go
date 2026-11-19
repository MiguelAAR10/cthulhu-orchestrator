package perfil

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"orq/internal/config"
)

// Ruta es una ruta de la app y el archivo que la declara.
type Ruta struct{ Path, Archivo string }

var (
	paginaApp   = regexp.MustCompile(`^(?:src/)?app/(?:(.*)/)?page\.(?:tsx|jsx|ts|js|mdx)$`)
	paginaPages = regexp.MustCompile(`^(?:src/)?pages/(.*)\.(?:tsx|jsx|ts|js)$`)
	rutaRouter  = regexp.MustCompile(`path\s*[=:]\s*\{?\s*["'](/[^"']*)["']`)
)

// Rutas lee los archivos versionados del router: el app router y el pages router de Next, y las
// rutas que declaran los archivos que importan react-router. No ejecuta nada del proyecto.
func Rutas(root string) []Ruta {
	cmd := exec.Command("git", "ls-files")
	cmd.Dir = root
	out, _ := cmd.Output()
	visto := map[string]bool{}
	var rs []Ruta
	add := func(p, f string) {
		if !visto[p] {
			visto[p] = true
			rs = append(rs, Ruta{p, f})
		}
	}
	for _, f := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if f == "" || strings.Contains(f, "node_modules/") {
			continue
		}
		if m := paginaApp.FindStringSubmatch(f); m != nil {
			var segs []string
			for _, s := range strings.Split(m[1], "/") {
				if s != "" && !strings.HasPrefix(s, "(") && !strings.HasPrefix(s, "@") {
					segs = append(segs, s)
				}
			}
			add("/"+strings.Join(segs, "/"), f)
			continue
		}
		if m := paginaPages.FindStringSubmatch(f); m != nil {
			base := filepath.Base(m[1])
			if strings.HasPrefix(base, "_") || m[1] == "api" || strings.HasPrefix(m[1], "api/") {
				continue
			}
			p := strings.TrimSuffix(m[1], "index")
			add("/"+strings.TrimSuffix(p, "/"), f)
			continue
		}
		switch filepath.Ext(f) {
		case ".tsx", ".jsx", ".ts", ".js":
			raw, err := os.ReadFile(filepath.Join(root, f))
			if err != nil || !strings.Contains(string(raw), "react-router") {
				continue
			}
			for _, m := range rutaRouter.FindAllStringSubmatch(string(raw), -1) {
				add(m[1], f)
			}
		}
	}
	sort.Slice(rs, func(i, j int) bool { return rs[i].Path < rs[j].Path })
	return rs
}

// Mapa es el esqueleto de docs/mapa.md: una línea por ruta, que completan el Owner o un worker.
func Mapa(rs []Ruta) string {
	var b strings.Builder
	b.WriteString("# Feature map\n\nOne line per route: path → page file · how to reach it · what it should show.\n")
	b.WriteString("Written by `orq init` from the router files. The Owner or a worker fills in the `?`; a worker that adds a route reports its line.\n\n")
	if len(rs) == 0 {
		b.WriteString("(no routes found in router files: add one line per screen by hand)\n")
	}
	for _, r := range rs {
		fmt.Fprintf(&b, "- `%s` → `%s` · reach: ? · shows: ?\n", r.Path, r.Archivo)
	}
	return b.String()
}

// configsPlaywright son los nombres que Playwright busca en la raíz del proyecto.
var configsPlaywright = []string{"playwright.config.ts", "playwright.config.js", "playwright.config.mjs", "playwright.config.cjs", "playwright.config.mts", "playwright.config.cts"}

// E2E devuelve el check de Playwright del proyecto, o por qué no hay ninguno. Si la config arranca
// su propio servidor (webServer), orq no arranca otro en el mismo puerto.
func E2E(root string) (config.Check, string) {
	var cfg []byte
	for _, n := range configsPlaywright {
		if raw, err := os.ReadFile(filepath.Join(root, n)); err == nil {
			cfg = raw
			break
		}
	}
	if cfg == nil {
		return config.Check{}, "sin playwright.config.* en la raíz: no hay pruebas de navegador que correr"
	}
	var pj struct {
		Scripts map[string]string `json:"scripts"`
		Deps    map[string]string `json:"dependencies"`
		DevDeps map[string]string `json:"devDependencies"`
	}
	if raw, err := os.ReadFile(filepath.Join(root, "package.json")); err == nil {
		json.Unmarshal(raw, &pj)
	}
	ch := config.Check{Cmd: "npx playwright test"}
	var nombres []string
	for n, s := range pj.Scripts {
		if strings.Contains(s, "playwright test") {
			nombres = append(nombres, n)
		}
	}
	if len(nombres) > 0 {
		sort.Strings(nombres)
		ch.Cmd = "npm run " + nombres[0]
	}
	if strings.Contains(string(cfg), "webServer") || pj.Scripts["dev"] == "" {
		return ch, ""
	}
	dep := func(n string) bool { _, a := pj.Deps[n]; _, b := pj.DevDeps[n]; return a || b }
	switch {
	case dep("next"):
		ch.Servidor, ch.Puerto = "npm run dev", 3000
	case dep("vite"):
		ch.Servidor, ch.Puerto = "npm run dev", 5173
	}
	return ch, ""
}
