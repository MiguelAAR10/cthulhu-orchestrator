package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"orq/internal/perfil"
)

func proyectoSkills(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("ORQ_ROOT", root)
	t.Setenv("ORQ_ROL", "")
	t.Setenv("HERDR_PANE_ID", "")
	os.MkdirAll(filepath.Join(root, ".orq"), 0o755)
	os.WriteFile(filepath.Join(root, "package.json"), []byte(`{}`), 0o644)
	os.WriteFile(filepath.Join(root, "tsconfig.json"), []byte(`{}`), 0o644)
	os.WriteFile(filepath.Join(root, ".orq", "proyecto.json"), []byte(`{"modo":"nuevo"}`), 0o644)
	orig := skillRunner
	skillRunner = func(dir string, argv []string) ([]byte, error) {
		t.Fatalf("no real install in tests: %v", argv)
		return nil, nil
	}
	t.Cleanup(func() { skillRunner = orig })
	return root
}

func TestProfileSetPersistsAndWarns(t *testing.T) {
	root := proyectoSkills(t)
	out, errs, code := run(t, "profile", "set", "type=brownfield", "criticality=high", "stage=production")
	if code != 0 || !strings.Contains(errs+out, "warning") {
		t.Fatalf("%d %s %s", code, out, errs)
	}
	raw, _ := os.ReadFile(filepath.Join(root, ".orq", "profile.json"))
	var f perfil.Forced
	json.Unmarshal(raw, &f)
	if f["type"] != "brownfield" || f["criticality"] != "high" {
		t.Fatalf("%s", raw)
	}
	out, _, _ = run(t, "profile")
	if !strings.Contains(out, "brownfield") || !strings.Contains(out, "typescript") || !strings.Contains(out, "forced") {
		t.Fatal(out)
	}
	if _, _, code := run(t, "profile", "set", "type=moon"); code == 0 {
		t.Fatal("bad value accepted")
	}
}

func TestSkillsPlanByPhaseAndRole(t *testing.T) {
	proyectoSkills(t)
	out, _, code := run(t, "skills", "plan", "--phase", "D", "--role", "worker")
	if code != 0 || !strings.Contains(out, "typescript-best-practices") || strings.Contains(out, "grilling") {
		t.Fatal(out)
	}
	run(t, "profile", "set", "criticality=trivial")
	out, _, _ = run(t, "skills", "plan", "--phase", "D", "--role", "worker")
	if strings.Count(out, "\n  ") > 1 {
		t.Fatalf("trivial ceremony: %s", out)
	}
}

func TestSkillsInstallRefusesNeverListAndVetRecordsGallery(t *testing.T) {
	root := proyectoSkills(t)
	if _, errs, code := run(t, "skills", "install", "swarm"); code == 0 || !strings.Contains(errs, "never") {
		t.Fatalf("%d %s", code, errs)
	}
	out, errs, code := run(t, "skills", "install", "orq-board")
	if code != 0 {
		t.Fatalf("%s %s", out, errs)
	}
	raw, _ := os.ReadFile(filepath.Join(root, ".orq", "skills.lock.json"))
	if !strings.Contains(string(raw), `"orq.orq-board"`) {
		t.Fatalf("lock: %s", raw)
	}
	if _, err := os.Stat(filepath.Join(root, ".agents", "skills", "orq-board", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(root, "bad.md")
	os.WriteFile(bad, []byte("---\nname: x\ndescription: y\n---\ngit push --force\n"), 0o644)
	if _, _, code := run(t, "skills", "vet", "evil", "--file", bad); code == 0 {
		t.Fatal("vet accepted a force push")
	}
	run(t, "skills", "vet", "orq-board")
	g := perfil.LoadGallery(filepath.Join(root, ".orq", "skills.gallery.json"))
	if g["evil"].Verdict != "rejected" || !strings.Contains(g["evil"].Reason, "git push") || g["orq.orq-board"].Verdict != "validated" {
		t.Fatalf("%+v", g)
	}
}

func TestRoleGuideLoadsOnlyPhaseRoleSet(t *testing.T) {
	proyectoSkills(t)
	out, _, _ := run(t, "skill", "atena") // phase T: no story yet
	if !strings.Contains(out, "grilling") || strings.Contains(out, "test-driven-development") {
		t.Fatal(out)
	}
}
