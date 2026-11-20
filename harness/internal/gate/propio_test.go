package gate

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// EsWorkerPropio is the narrow counting question of orq2: only a clone orq made counts, not any
// linked worktree on the machine and not a directory that cannot be placed.
func TestEsWorkerPropio(t *testing.T) {
	base := t.TempDir()
	repo, wt := filepath.Join(base, "repo"), filepath.Join(base, "wt")
	gitInit(t, repo)
	if out, err := exec.Command("git", "-C", repo, "worktree", "add", "-q", "-b", "w", wt).CombinedOutput(); err != nil {
		t.Fatalf("worktree add: %v\n%s", err, out)
	}
	clon := clonMarcado(t, `{"slice":"01"}`)
	anidado := filepath.Join(clon, "sub")
	gitInit(t, anidado)
	corrupto := clonMarcado(t, "")
	for _, c := range []struct {
		dir  string
		want bool
	}{{wt, false}, {repo, false}, {clon, true}, {anidado, true}, {corrupto, true}, {"", false}, {"rel/dir", false}, {"/", false}} {
		if got := EsWorkerPropio(c.dir); got != c.want {
			t.Errorf("EsWorkerPropio(%q) = %v, want %v", c.dir, got, c.want)
		}
	}
}
