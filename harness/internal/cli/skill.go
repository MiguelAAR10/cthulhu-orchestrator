package cli

import (
	"embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"orq/internal/config"
	"orq/internal/identidad"
	"orq/internal/perfil"
)

//go:embed guias/*.md
var guias embed.FS

//go:embed orqskills
var orqSkills embed.FS

// instalarSkillsOrq deja las skills propias de orq en el repo; no pisa una versión editada.
func instalarSkillsOrq(root string) (nuevas, distintas []string) {
	ents, _ := orqSkills.ReadDir("orqskills")
	for _, d := range ents {
		raw, _ := orqSkills.ReadFile("orqskills/" + d.Name() + "/SKILL.md")
		raw = []byte(identidad.Skills(identidad.Texto(skillOrq2(d.Name(), string(raw)))))
		nombre := identidad.Skills(d.Name())
		dst := filepath.Join(root, ".claude", "skills", nombre, "SKILL.md")
		actual, err := os.ReadFile(dst)
		switch {
		case err != nil:
			os.MkdirAll(filepath.Dir(dst), 0o755)
			if os.WriteFile(dst, raw, 0o644) == nil {
				nuevas = append(nuevas, nombre)
			}
		case string(actual) != string(raw):
			distintas = append(distintas, nombre)
		}
	}
	return
}

// nombresSkillsOrq son las skills propias de orq que init deja en el repo.
func nombresSkillsOrq() []string {
	ents, _ := orqSkills.ReadDir("orqskills")
	var ns []string
	for _, d := range ents {
		ns = append(ns, identidad.Skills(d.Name()))
	}
	return ns
}

// runSkill imprime la guía del rol y las skills que ese rol carga en este proyecto.
func runSkill(args []string, stdout, stderr io.Writer) int {
	rol := os.Getenv("ORQ_ROL")
	if len(args) > 0 {
		rol = args[0]
		switch rol {
		case "eval":
			return runSkillEval(args[1:], stdout, stderr)
		case "notas":
			return runSkillNotas(args[1:], stdout, stderr)
		case "comparar":
			return runSkillComparar(args[1:], stdout, stderr)
		}
	}
	if !slices.Contains(perfil.Roles, rol) {
		fmt.Fprintf(stderr, "uso: orq skill %s\n", strings.Join(perfil.Roles, "|"))
		return 1
	}
	g, _ := guias.ReadFile("guias/" + rol + ".md")
	p, err := abrirProyecto()
	cfg := p.Cfg
	if err != nil { // fuera de un repo, o con el proyecto ilegible: la guía sale con la capa del usuario
		if cfg, err = config.Cargar(""); err != nil {
			fmt.Fprintln(stderr, "orq:", err)
			return 1
		}
	}
	if rol == "orquestador" && identidad.EsOrq2() {
		g = []byte(astraPorRol(string(g), cfg.Rol("astra").Kind))
	}
	// Los modelos de la guía salen de la configuración, no del texto.
	fmt.Fprint(stdout, identidad.Skills(cfg.Expandir(string(g))))
	if p.Root != "" {
		ls := perfil.ParaRol(perfil.Lista(p.Perfil()), rol)
		if len(ls) > 0 {
			fmt.Fprintf(stdout, "\n## Skills de tu rol en este proyecto\n")
			for _, s := range ls {
				mark := "✓"
				if !perfil.Instalada(p.Root, s.Nombre) {
					mark = "· falta"
				}
				fmt.Fprintf(stdout, "- %s %s", s.Nombre, mark)
				if s.Adapta != "" {
					fmt.Fprintf(stdout, " — regla del harness: %s", s.Adapta)
				}
				fmt.Fprintln(stdout)
			}
		}
	}
	fmt.Fprintf(stdout, "\nNo cargues: %s.\n", strings.Join(nunca(), ", "))
	return 0
}

func nunca() []string {
	var n []string
	for k := range perfil.Nunca {
		n = append(n, k)
	}
	slices.Sort(n)
	return n
}

// astraCodex es lo que la guía de orq dice de Astra; orq2 no supone Codex: manda el rol.
const astraCodex = "({{rol:astra}}; `--kind claude` if no Codex quota)"

// astraPorRol cambia, para orq2, la frase de Astra por el agente y modelo del rol astra.
func astraPorRol(g, kind string) string {
	return strings.Replace(g, astraCodex, "("+kind+" {{rol:astra}}: the agent and model the astra role sets in `orq config ver`)", 1)
}
