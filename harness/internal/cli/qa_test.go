package cli

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestQAFindingYRunDesdeCLI(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ORQ_ROOT", root)
	t.Setenv("ORQ_ROL", "")
	t.Setenv("ORQ_WORKER_ID", "")
	os.MkdirAll(filepath.Join(root, ".orq"), 0o755)
	orig := qaRunner
	defer func() { qaRunner = orig }()
	var corridas []string
	qaRunner = func(dir, cmd string, _ time.Duration) (string, int) {
		corridas = append(corridas, cmd)
		if cmd == "falla" {
			return "", 1
		}
		return "2 passed", 0
	}
	if _, _, c := run(t, "qa", "finding", "add", "S-07", "--requirement", "R1", "--repro", "falla"); c == 0 {
		t.Fatal("hallazgo incompleto rechazado")
	}
	out, _, c := run(t, "qa", "finding", "add", "S-07", "--requirement", "R1", "--commit", "abc", "--repro", "pasa",
		"--observed", "o", "--expected", "e", "--impact", "i")
	if c != 0 || !strings.Contains(out, "NO confirmado") {
		t.Fatalf("repro que pasa no confirma: %s", out)
	}
	if _, errs, c := run(t, "qa", "run", "S-07"); c == 0 || !strings.Contains(errs, "sin capas") {
		t.Fatalf("sin capas no hay QA: %s", errs)
	}
	os.WriteFile(filepath.Join(root, ".orq", "config.json"), []byte(`{"qa":{"layers":[{"name":"unit","cmd":"u"},{"name":"mutation","cmd":"m","min_criticality":"high"}]}}`), 0o644)
	t.Setenv("ORQ_ROL", "tester")
	if _, _, c := run(t, "qa", "run", "S-07"); c == 0 {
		t.Fatal("el tester no corre qa run")
	}
	t.Setenv("ORQ_ROL", "")
	out, _, c = run(t, "qa", "run", "S-07")
	if c != 0 || !strings.Contains(out, "omitida") || strings.Contains(strings.Join(corridas, ","), "m") {
		t.Fatalf("QA en media: %s %v", out, corridas)
	}
	if _, err := os.Stat(filepath.Join(root, ".orq/qa/S-07/receipt.json")); err != nil {
		t.Fatal("recibo QA guardado")
	}
}

func TestEvidenceDesdeCLI(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ORQ_ROOT", root)
	os.MkdirAll(filepath.Join(root, ".orq"), 0o755)
	png1 := filepath.Join(t.TempDir(), "a.png")
	f, _ := os.Create(png1)
	png.Encode(f, image.NewRGBA(image.Rect(0, 0, 4, 4)))
	f.Close()
	for _, k := range []string{"target", "after"} {
		if _, errs, c := run(t, "evidence", "add", "S-09", "--kind", k, "--file", png1, "--note", "Clients see their orders"); c != 0 {
			t.Fatal(errs)
		}
	}
	md, err := os.ReadFile(filepath.Join(root, "docs/slices/S-09/README.md"))
	if err != nil || !strings.Contains(string(md), "0.00%") || len(strings.Split(string(md), "\n")) > 41 {
		t.Fatalf("README: %s %v", md, err)
	}
}
