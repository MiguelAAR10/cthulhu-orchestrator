package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"orq/internal/worktrees"
)

func TestGCDryRunNoTocaYReporta(t *testing.T) {
	root := t.TempDir()
	for _, a := range [][]string{{"init", "-q", "-b", "main"}, {"-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "--allow-empty", "-m", "x"}} {
		if out, err := exec.Command("git", append([]string{"-C", root}, a...)...).CombinedOutput(); err != nil {
			t.Fatal(string(out))
		}
	}
	t.Setenv("ORQ_ROOT", root)
	orq := filepath.Join(root, ".orq")
	wt := filepath.Join(t.TempDir(), "s01")
	if err := worktrees.Crear(root, orq, worktrees.Entrada{Path: wt, Kind: worktrees.KindSlice}, "-b", "slice/01"); err != nil {
		t.Fatal(err)
	}
	out, errs, code := run(t, "gc", "--dry-run")
	if code != 0 || !strings.Contains(out, wt) || !strings.Contains(out, "gc:") {
		t.Fatalf("code=%d out=%q errs=%q", code, out, errs)
	}
	if _, err := os.Stat(wt); err != nil {
		t.Fatal("dry-run borró")
	}
}
