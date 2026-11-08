// Package perfil detecta qué clase de proyecto es (mejora o nuevo, y su stack) y
// recomienda solo las skills que ese proyecto necesita, instaladas en el propio repo.
package perfil

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// Roles del harness.
var Roles = []string{"atena", "orquestador", "planificador", "astra", "scout", "worker", "verificador"}

// Nunca: skills que dan al agente un control que en este harness es de orq o del Orquestador.
// Sale de la never-list del catálogo embebido.
var Nunca = DefaultCatalog().neverMap()

// Skill es una skill recomendada con su contexto en el harness.
type Skill struct {
	Nombre, Repo, Adapta string
	Roles, Requiere      []string
}

// Lista devuelve todas las skills que el proyecto podría cargar (cualquier fase y rol,
// ceremonia completa), con dependencias y sin repetir. Para un rol concreto usa Plan.
func Lista(p Perfil) []Skill {
	pr := Profile{ProjectType: "greenfield", Stacks: p.Senales, Criticality: "high", Stage: "production"}
	if p.Modo == "mejora" {
		pr.ProjectType = "brownfield"
	}
	return AsSkills(PlanAll(DefaultCatalog(), pr))
}

// AsSkills adapta entradas del catálogo al tipo que usa Verificar.
func AsSkills(es []Entry) []Skill {
	var out []Skill
	for _, e := range es {
		out = append(out, Skill{Nombre: e.Name, Repo: e.Source, Adapta: e.Adapt, Roles: e.Roles, Requiere: e.Requires})
	}
	return out
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
	ID     string   `json:"id,omitempty"`
	Ref    string   `json:"ref,omitempty"`
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
