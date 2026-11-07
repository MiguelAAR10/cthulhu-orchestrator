// Package perfil detecta qué clase de proyecto es (mejora o nuevo, y su stack) y
// recomienda solo las skills que ese proyecto necesita, instaladas en el propio repo.
package perfil

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

//go:embed catalogo.json
var catalogoJSON []byte

type Entrada struct {
	Senal    string              `json:"senal"`
	Repo     string              `json:"repo"`
	Skills   []string            `json:"skills"`
	Para     string              `json:"para"`
	Roles    map[string][]string `json:"roles"`    // skill → roles que la cargan
	Requiere map[string][]string `json:"requiere"` // skill → skills sin las que no funciona
	Adapta   map[string]string   `json:"adapta"`   // skill → regla del harness que manda sobre la skill
}

// Roles del harness.
var Roles = []string{"atena", "orquestador", "planificador", "astra", "scout", "worker", "verificador"}

// Nunca: skills que dan al agente un control que en este harness es de orq o del Orquestador.
var Nunca = map[string]string{
	"using-superpowers":              "convierte a cada sesión en orquestador; aquí coordina uno solo",
	"using-git-worktrees":            "los worktrees los crea y cierra orq",
	"finishing-a-development-branch": "integrar y limpiar es `orq slice cerrar`",
	"brainstorming":                  "la entrevista es del Atena (grilling)",
	"subagent-driven-development":    "decide solo ante ambigüedad; aquí para y pregunta",
	"webapp-testing":                 "usa networkidle; se prueba con playwright-cli y aserciones",
	"grill-me":                       "reemplazada por grilling / grill-with-docs",
	"tdd":                            "duplica test-driven-development",
	"handoff":                        "el relevo es `orq relevo`",
}

// Skill es una skill recomendada con su contexto en el harness.
type Skill struct {
	Nombre, Repo, Adapta string
	Roles, Requiere      []string
}

// Lista devuelve las skills del perfil con sus dependencias incluidas, sin repetir.
func Lista(p Perfil) []Skill {
	activas := append([]string{"base", "modo:" + p.Modo}, p.Senales...)
	var out []Skill
	visto := map[string]bool{}
	for _, e := range Catalogo() {
		if !slices.Contains(activas, e.Senal) {
			continue
		}
		for _, s := range e.Skills {
			pend := append([]string{s}, e.Requiere[s]...)
			for _, n := range pend {
				if visto[n] {
					continue
				}
				visto[n] = true
				out = append(out, Skill{Nombre: n, Repo: e.Repo, Adapta: e.Adapta[n], Roles: e.Roles[n], Requiere: e.Requiere[n]})
			}
		}
	}
	return out
}

// ParaRol filtra las skills que carga un rol.
func ParaRol(ls []Skill, rol string) []Skill {
	var out []Skill
	for _, s := range ls {
		if slices.Contains(s.Roles, rol) {
			out = append(out, s)
		}
	}
	return out
}

func Catalogo() []Entrada {
	var c []Entrada
	if err := json.Unmarshal(catalogoJSON, &c); err != nil {
		panic(err) // catálogo embebido roto: error de compilación del harness
	}
	return c
}

type Perfil struct {
	Modo    string   `json:"modo"`    // mejora | nuevo
	Senales []string `json:"senales"` // react, web, go, python, ...
	Fuentes int      `json:"archivos_fuente"`
}

var fuenteExt = map[string]bool{".go": true, ".py": true, ".ts": true, ".tsx": true, ".js": true, ".jsx": true, ".rs": true, ".java": true, ".kt": true, ".rb": true, ".php": true, ".cs": true, ".swift": true, ".vue": true, ".svelte": true}

// Detect lee archivos versionados; no ejecuta nada del proyecto.
func Detect(root string) Perfil {
	p := Perfil{}
	cmd := exec.Command("git", "ls-files")
	cmd.Dir = root
	out, _ := cmd.Output()
	files := strings.Split(strings.TrimSpace(string(out)), "\n")
	set := map[string]bool{}
	add := func(s string) { set[s] = true }
	for _, f := range files {
		if f == "" {
			continue
		}
		ext := filepath.Ext(f)
		if fuenteExt[ext] && !strings.Contains(f, "node_modules/") {
			p.Fuentes++
		}
		switch {
		case ext == ".go":
			add("go")
		case ext == ".py":
			add("python")
		case ext == ".html", ext == ".css", ext == ".vue", ext == ".svelte":
			add("web")
		case ext == ".tsx", ext == ".jsx":
			add("react")
			add("web")
		}
	}
	for _, pkg := range []string{"package.json"} {
		raw, err := os.ReadFile(filepath.Join(root, pkg))
		if err != nil {
			continue
		}
		var pj struct {
			Deps    map[string]string `json:"dependencies"`
			DevDeps map[string]string `json:"devDependencies"`
		}
		json.Unmarshal(raw, &pj)
		has := func(n string) bool { _, a := pj.Deps[n]; _, b := pj.DevDeps[n]; return a || b }
		if has("react") || has("next") {
			add("react")
			add("web")
		}
		if has("react-native") || has("expo") {
			add("react-native")
		}
		if has("vue") || has("svelte") || has("vite") {
			add("web")
		}
		if has("@anthropic-ai/sdk") || has("@anthropic-ai/claude-agent-sdk") {
			add("claude-sdk")
		}
		if has("@modelcontextprotocol/sdk") {
			add("mcp")
		}
	}
	for _, f := range []string{"pyproject.toml", "requirements.txt"} {
		raw, _ := os.ReadFile(filepath.Join(root, f))
		s := string(raw)
		if strings.Contains(s, "anthropic") {
			add("claude-sdk")
		}
		if strings.Contains(s, "mcp") {
			add("mcp")
		}
	}
	p.Modo = "nuevo"
	if p.Fuentes >= 5 {
		p.Modo = "mejora"
	}
	for s := range set {
		p.Senales = append(p.Senales, s)
	}
	sort.Strings(p.Senales)
	return p
}

// Recomendadas devuelve, por repo, las skills que este perfil necesita.
func Recomendadas(p Perfil) map[string][]string {
	res := map[string][]string{}
	for _, s := range Lista(p) {
		res[s.Repo] = append(res[s.Repo], s.Nombre)
	}
	return res
}

// HuellaSkill resume el contenido de la carpeta de una skill instalada en el repo.
func HuellaSkill(root, skill string) (string, error) {
	dir := filepath.Join(root, ".claude", "skills", skill)
	h := sha256.New()
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		fmt.Fprintf(h, "%s\x00%d\x00", rel, len(raw))
		h.Write(raw)
		return nil
	})
	return hex.EncodeToString(h.Sum(nil)), err
}

// Fijada es una entrada del manifiesto .orq/skills.lock.json.
type Fijada struct {
	Repo   string   `json:"repo"`
	Huella string   `json:"huella"`
	Roles  []string `json:"roles"`
	Adapta string   `json:"adapta,omitempty"`
}

// Problema de verificación: FALTA (no instalada), DEP (le falta una dependencia),
// CAMBIO (difiere de lo fijado), NUNCA (instalada pero prohibida en el harness).
type Problema struct{ Tipo, Skill, Detalle string }

func Verificar(root string, ls []Skill, fijadas map[string]Fijada) []Problema {
	var out []Problema
	for _, s := range ls {
		if !Instalada(root, s.Nombre) {
			out = append(out, Problema{"FALTA", s.Nombre, s.Repo})
			continue
		}
		for _, d := range s.Requiere {
			if !Instalada(root, d) {
				out = append(out, Problema{"DEP", s.Nombre, "necesita " + d})
			}
		}
		if f, ok := fijadas[s.Nombre]; ok {
			if h, _ := HuellaSkill(root, s.Nombre); h != f.Huella {
				out = append(out, Problema{"CAMBIO", s.Nombre, "difiere de la versión fijada: revisa el cambio y `orq skills fijar`"})
			}
		}
	}
	for n, por := range Nunca {
		if Instalada(root, n) {
			out = append(out, Problema{"NUNCA", n, por})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Tipo+out[i].Skill < out[j].Tipo+out[j].Skill })
	return out
}

// Instalada mira la carpeta de skills del proyecto, no la global.
func Instalada(root, skill string) bool {
	_, err := os.Stat(filepath.Join(root, ".claude", "skills", skill, "SKILL.md"))
	return err == nil
}

// ComandoInstalar es el comando que deja las skills solo en este repo.
func ComandoInstalar(repo string, skills []string) []string {
	args := []string{"npx", "-y", "skills@latest", "add", repo}
	for _, s := range skills {
		args = append(args, "-s", s)
	}
	return append(args, "-a", "claude-code", "-y")
}
