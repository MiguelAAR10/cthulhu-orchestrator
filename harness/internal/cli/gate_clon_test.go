package cli

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"orq/internal/orqdir"
)

// Inside a worker clone the gate's real environment reads the exam bundle and the main
// checkout from the clone marker: the clone has no .orq of its own.
func TestRealEnvEnUnClon(t *testing.T) {
	base := t.TempDir()
	principal, clon := filepath.Join(base, "main"), filepath.Join(base, "clon")
	sub := filepath.Join(clon, "src")
	for _, d := range []string{filepath.Join(principal, ".git"), filepath.Join(clon, ".git"), sub} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	marca := `{"root":"` + principal + `","slice":"01","rama":"slice/01","bundle":["examen/s01.sh"]}`
	if err := os.WriteFile(filepath.Join(clon, ".git", orqdir.MarcaClon), []byte(marca), 0o644); err != nil {
		t.Fatal(err)
	}
	env := realEnv()
	if !env.IsWorktree(sub) {
		t.Fatal("a marked clone must be a worker for the real gate")
	}
	if env.IsWorktree(principal) {
		t.Fatal("the main checkout must not be a worker")
	}
	if got := env.Bundle(sub); !slices.Equal(got, []string{"examen/s01.sh"}) {
		t.Fatalf("bundle in a clone = %v, want the marker's bundle", got)
	}
	if got := env.MainRoot(sub); got != principal {
		t.Fatalf("main root in a clone = %q, want %q", got, principal)
	}
	if got := env.MainRoot(principal); got != principal {
		t.Fatalf("main root of the main checkout = %q, want %q", got, principal)
	}
}

// Review 4, finding 4: a copied or renamed orq binary, an alias or a script escapes the gate's
// name match, so the binary itself refuses every orchestrator-only `orq slice` subcommand when
// it runs inside a worker. The read-only ones still answer.
func TestSliceDelOrquestadorSeNiegaEnUnWorker(t *testing.T) {
	base := t.TempDir()
	clon := filepath.Join(base, "clon")
	sub := filepath.Join(clon, "src")
	for _, d := range []string{filepath.Join(clon, ".git"), sub} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	marca := `{"root":"` + filepath.Join(base, "main") + `","slice":"01","rama":"slice/01"}`
	if err := os.WriteFile(filepath.Join(clon, ".git", orqdir.MarcaClon), []byte(marca), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ORQ_ROOT", t.TempDir())
	t.Chdir(sub)
	for _, args := range [][]string{
		{"slice", "verificar"}, {"slice", "podar", "w-front"}, {"slice", "cerrar", "--aceptado"},
		{"slice", "--slice", "01", "tomar", "w1"}, {"slice", "abrir", "02"}, {"slice", "examen", "x", "--aprobado", "y"},
		{"slice", "aprobar", "01"}, {"slice", "tarjeta", "01", "--archivo", "t.md"},
	} {
		_, errs, code := run(t, args...)
		if code == 0 || !strings.Contains(errs, "belongs to the orchestrator") {
			t.Errorf("%v from a worker clone: code %d, stderr %q", args, code, errs)
		}
	}
	if _, errs, _ := run(t, "slice", "estado"); strings.Contains(errs, "belongs to the orchestrator") {
		t.Errorf("slice estado is read-only and must not be refused: %q", errs)
	}
}
