package cli

import (
	"os"
	"path/filepath"
	"slices"
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
