package cli

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

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
		dst := filepath.Join(root, ".claude", "skills", d.Name(), "SKILL.md")
		actual, err := os.ReadFile(dst)
		switch {
		case err != nil:
			os.MkdirAll(filepath.Dir(dst), 0o755)
			if os.WriteFile(dst, raw, 0o644) == nil {
				nuevas = append(nuevas, d.Name())
			}
		case string(actual) != string(raw):
			distintas = append(distintas, d.Name())
		}
	}
	return
}

// runSkill imprime la guía del rol y las skills que ese rol carga en este proyecto.
func runSkill(args []string, stdout, stderr io.Writer) int {
	rol := os.Getenv("ORQ_ROL")
	if len(args) > 0 {
		rol = args[0]
	}
	if !slices.Contains(perfil.Roles, rol) {
		fmt.Fprintf(stderr, "uso: orq skill %s\n", strings.Join(perfil.Roles, "|"))
		return 1
	}
	g, _ := guias.ReadFile("guias/" + rol + ".md")
	fmt.Fprint(stdout, string(g))
	if p, err := abrirProyecto(); err == nil {
		ls := perfil.ParaRol(perfil.Lista(p.perfil()), rol)
		if len(ls) > 0 {
			fmt.Fprintf(stdout, "\n## Skills de tu rol en este proyecto\n")
			for _, s := range ls {
				mark := "✓"
				if !perfil.Instalada(p.root, s.Nombre) {
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

func (p proyecto) lockPath() string { return filepath.Join(p.dir, "skills.lock.json") }

func (p proyecto) fijadas() map[string]perfil.Fijada {
	m := map[string]perfil.Fijada{}
	raw, err := os.ReadFile(p.lockPath())
	if err == nil {
		json.Unmarshal(raw, &m)
	}
	return m
}
