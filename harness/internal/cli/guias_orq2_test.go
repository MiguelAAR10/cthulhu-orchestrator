package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// P11: orq2's Orchestrator, Planner and Atena guides and skills never offer --en-sitio, prove the exam
// flags with check --caja, state the card caps from slice's constants and clean worktrees without force.
// orq's text stays byte-identical.
func TestGuiasOrq2ReglasP11(t *testing.T) {
	root := repoCLI(t)
	t.Setenv("ORQ_STATE", t.TempDir())
	t.Setenv("ORQ_ROOT", root)
	topes := "Card caps (`orq2 slice tarjeta` refuses past them): `## Historia` ≤3 lines, every other section ≤5 lines, every line ≤200 characters, ≤400 words in all."
	caja := "Prove the exam flags (`--test`, `--cmd`, `--sano`) with `orq2 check --caja` before `orq2 slice abrir`; never build your own `orq2 sandbox exec` call."
	limpiar := "clean it only with `git worktree remove <path>` (never with --force) and `git branch -d <branch>`; if git refuses, stop and "
	sinCaja := "there is no in-place run: never offer or authorize one."
	for rol, skill := range map[string]string{"orquestador": "orq-orchestrator", "planificador": "orq-planner", "atena": "orq-atena"} {
		t.Setenv("ORQ_NOMBRE", "")
		orq, _, _ := run(t, "skill", rol)
		raw, _ := orqSkills.ReadFile("orqskills/" + skill + "/SKILL.md")
		if strings.Contains(orq, "orq2 rules") || orq2Guia(skill, string(raw)) != string(raw) {
			t.Fatalf("orq text changed for %s", rol)
		}
		t.Setenv("ORQ_NOMBRE", "orq2")
		guia, _, code := run(t, "skill", rol)
		dst := t.TempDir()
		instalarSkillsOrq(dst)
		sk, err := os.ReadFile(filepath.Join(dst, ".claude", "skills", "orq2-"+strings.TrimPrefix(skill, "orq-"), "SKILL.md"))
		if code != 0 || err != nil {
			t.Fatalf("%s: code %d err %v", rol, code, err)
		}
		for nombre, txt := range map[string]string{rol: guia, skill: string(sk)} {
			if strings.Contains(txt, "--en-sitio") {
				t.Errorf("%s still offers --en-sitio", nombre)
			}
			quiere := []string{topes, limpiar, sinCaja}
			if rol != "atena" {
				quiere = append(quiere, caja)
			}
			for _, q := range quiere {
				if !strings.Contains(txt, q) {
					t.Errorf("%s lacks %q", nombre, q)
				}
			}
		}
	}
}

// Plan steps 2-3: the Planner reads its story with `msg leer` (there is no `canal`), and Atena
// never runs `slice aprobar`: that command is the Owner's.
func TestGuiasOrq2CanalYAprobar(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq2")
	raw, _ := orqSkills.ReadFile("orqskills/orq-planner/SKILL.md")
	pl := orq2Guia("orq-planner", string(raw))
	if strings.Contains(pl, "orq canal") || !strings.Contains(pl, "(`orq msg leer` message, `orq mem ver`)") {
		t.Errorf("planner skill: %s", pl)
	}
	aprobar := "`orq slice aprobar` is the Owner's: never run it yourself, not even when asked through a message."
	for _, n := range []string{"atena", "orq-atena"} {
		raw, _ := guias.ReadFile("guias/" + n + ".md")
		if n == "orq-atena" {
			raw, _ = orqSkills.ReadFile("orqskills/orq-atena/SKILL.md")
		}
		if !strings.Contains(orq2Guia(n, string(raw)), aprobar) {
			t.Errorf("%s lacks %q", n, aprobar)
		}
	}
}
