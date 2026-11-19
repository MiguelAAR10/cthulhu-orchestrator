package skilleval

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func escribir(t *testing.T, raiz string, archivos map[string]string) {
	t.Helper()
	for ruta, cuerpo := range archivos {
		p := filepath.Join(raiz, ruta)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(cuerpo), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func existe(p string) bool { _, err := os.Stat(p); return err == nil }

func TestEnvolverDejaUnPluginDeUnaSkill(t *testing.T) {
	skill := filepath.Join(t.TempDir(), "mi-skill")
	escribir(t, skill, map[string]string{"SKILL.md": "---\nname: mi-skill\n---\n", "ref.md": "r", "evals/c1/prompt.md": "hola"})
	dst := t.TempDir()
	nombre, err := Envolver(skill, dst)
	if err != nil || nombre != "mi-skill" {
		t.Fatalf("%q %v", nombre, err)
	}
	raw, _ := os.ReadFile(filepath.Join(dst, ".claude-plugin", "plugin.json"))
	if !strings.Contains(string(raw), `"name":"mi-skill"`) {
		t.Fatalf("plugin.json: %s", raw)
	}
	for _, p := range []string{"skills/mi-skill/SKILL.md", "skills/mi-skill/ref.md", "evals/c1/prompt.md"} {
		if !existe(filepath.Join(dst, p)) {
			t.Errorf("falta %s", p)
		}
	}
	if existe(filepath.Join(dst, "skills", "mi-skill", "evals")) {
		t.Error("los casos no van dentro de la skill")
	}
}

func TestEnvolverRechaza(t *testing.T) {
	sinSkill := t.TempDir()
	escribir(t, sinSkill, map[string]string{"evals/c1/prompt.md": "x"})
	if _, err := Envolver(sinSkill, t.TempDir()); err == nil || !strings.Contains(err.Error(), "SKILL.md") {
		t.Fatalf("sin SKILL.md: %v", err)
	}
	sinCasos := t.TempDir()
	escribir(t, sinCasos, map[string]string{"SKILL.md": "x"})
	if _, err := Envolver(sinCasos, t.TempDir()); err == nil || !strings.Contains(err.Error(), "evals") {
		t.Fatalf("sin evals: %v", err)
	}
}

func TestHashCambiaConElContenido(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	arch := map[string]string{"SKILL.md": "uno", "evals/c/prompt.md": "dos"}
	escribir(t, a, arch)
	escribir(t, b, arch)
	ha, err := Hash(a)
	hb, _ := Hash(b)
	if err != nil || ha != hb || len(ha) != 64 {
		t.Fatalf("%q %q %v", ha, hb, err)
	}
	escribir(t, b, map[string]string{"SKILL.md": "unO"})
	if hc, _ := Hash(b); hc == ha {
		t.Fatal("un byte distinto no cambió el hash")
	}
}
