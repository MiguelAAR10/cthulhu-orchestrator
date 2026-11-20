package gate

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

// orq2 without max_workers: no head-count cap, the machine (env.Capacidad) decides; with no
// capacity reading either, nothing is known and the launch is refused.
func TestSinMaxWorkersDecideCapacidad(t *testing.T) {
	now := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)
	l := baseLimits()
	l.MaxWorkers = nil
	f := fakes{free: map[string]float64{"/": 400, "/mnt/c": 100}, workers: 50, now: now}
	env := f.env(t, freshState(t, now, 5))
	env.Capacidad = func(string, int) (string, error) { return "", nil }
	if d := Evaluate(Payload{ToolName: "Agent"}, l, env); !d.Allow {
		t.Fatalf("no cap and room on the machine: %s", d.Reason)
	}
	env.Capacidad = func(string, int) (string, error) { return "no cabe otro agente por RAM", nil }
	if d := Evaluate(Payload{ToolName: "Agent"}, l, env); d.Allow || !strings.Contains(d.Reason, "RAM") {
		t.Fatalf("RAM must refuse: %+v", d)
	}
	env.Capacidad = nil
	if d := Evaluate(Payload{ToolName: "Agent"}, l, env); d.Allow {
		t.Fatal("no max_workers and no capacity reading must refuse")
	}
}
