package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// orq2's planner skill tells the Planner to write the card as the Owner's plain-Spanish decision
// document, and the orchestrator skill points there. orq's skills stay byte-identical.
func TestGuiaTarjetaClaraParaOwner(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ORQ_NOMBRE", "")
	root := t.TempDir()
	instalarSkillsOrq(root)
	for _, s := range []string{"orq-planner", "orq-orchestrator"} {
		raw, _ := orqSkills.ReadFile("orqskills/" + s + "/SKILL.md")
		got, err := os.ReadFile(filepath.Join(root, ".claude", "skills", s, "SKILL.md"))
		if err != nil || string(got) != string(raw) {
			t.Fatalf("orq %s skill changed (%v)", s, err)
		}
	}

	t.Setenv("ORQ_NOMBRE", "orq2")
	root = t.TempDir()
	instalarSkillsOrq(root)
	leer := func(s string) string {
		b, err := os.ReadFile(filepath.Join(root, ".claude", "skills", s, "SKILL.md"))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	plan := leer("orq2-planner")
	for _, w := range []string{
		"## Card",
		"plain Spanish a non-programmer reads in 2 minutes",
		"`.orq/plan-sNN.json`",
		"Max 25 lines",
		"`## Historia`, `## Fuera`, `## Examen`, `## Zonas`, `## Preguntas`",
		"step by step",
		"`Valor: <product metric> hoy <value> → meta <value>`",
		"si hay un precio en el mensaje, no se envía",
		"no status codes or internals",
		"the test file path once",
		"`## Zonas`: one repo path per line",
		"relative to the repo root, never absolute",
		"what changes for the Owner goes in `## Historia` and `## Examen`",
		"Task ids",
		"your recommendation",
		"- [ ] lo haces tú:",
	} {
		if !strings.Contains(plan, w) {
			t.Errorf("orq2 planner skill lacks %q", w)
		}
	}
	// Zonas are repo paths: slice.LeerTarjeta and the decision page compare changed files against them.
	if strings.Contains(plan, "what changes for the user") {
		t.Errorf("orq2 planner skill still asks for Zonas as prose")
	}
	orch := leer("orq2-orchestrator")
	// orq's own card sentence is replaced; the caps line printed from slice.TopesTarjeta (P11) may stay,
	// because the constants are what orq slice tarjeta enforces (integration-verdict.md §3).
	if strings.Contains(orch, tarjetaOrq) {
		t.Errorf("orq2 orchestrator skill still carries orq's card sentence")
	}
	for _, w := range []string{"plain Spanish for the Owner", "≤25 lines", "`orq2-planner` skill, section Card"} {
		if !strings.Contains(orch, w) {
			t.Errorf("orq2 orchestrator skill lacks %q", w)
		}
	}
}
