package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"orq/internal/gate"
	"orq/internal/limits"
	"orq/internal/orqdir"
	"orq/internal/state"
	"orq/internal/sysinfo"
)

func gitIni(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, a := range [][]string{{"init", "-q", "-b", "main"}, {"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--allow-empty", "-m", "i"}} {
		if out, err := exec.Command("git", append([]string{"-C", dir}, a...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", a, err, out)
		}
	}
}

// conteoDirs builds (1) a linked worktree, (2) a plain repo, (3) a marked clone, (4) a repo nested in it.
func conteoDirs(t *testing.T) []string {
	base := t.TempDir()
	repo, wt, clon := filepath.Join(base, "repo"), filepath.Join(base, "wt"), filepath.Join(base, "clon")
	gitIni(t, repo)
	if out, err := exec.Command("git", "-C", repo, "worktree", "add", "-q", "-b", "w", wt).CombinedOutput(); err != nil {
		t.Fatalf("%s", out)
	}
	gitIni(t, clon)
	if err := os.WriteFile(filepath.Join(clon, ".git", orqdir.MarcaClon), []byte(`{"slice":"01"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	anidado := filepath.Join(clon, "sub")
	gitIni(t, anidado)
	return []string{wt, repo, clon, anidado}
}

func procsEn(dirs []string) []sysinfo.Proc {
	var ps []sysinfo.Proc
	for i, d := range dirs {
		ps = append(ps, sysinfo.Proc{PID: 100 + i, PPID: 1, Comm: "claude", Cwd: d})
	}
	return ps
}

func TestConteoOrq2SoloClonesPropios(t *testing.T) {
	ps := procsEn(conteoDirs(t))
	t.Setenv("ORQ_NOMBRE", "orq2")
	got := sysinfo.Workers(ps, []string{"claude"}, EsWorkerContable())
	if len(got) != 2 || got[0].PID != 102 || got[1].PID != 103 {
		t.Fatalf("orq2 must count only its own clones (3, 4): %+v", got)
	}
	t.Setenv("ORQ_NOMBRE", "")
	got = sysinfo.Workers(ps, []string{"claude"}, EsWorkerContable())
	if len(got) != 3 || got[0].PID != 100 || got[1].PID != 102 || got[2].PID != 103 {
		t.Fatalf("orq must count as today (1, 3, 4): %+v", got)
	}
}

// Five foreign agents in linked worktrees and none of orq2's own: orq2 launches, orq refuses as today.
func TestGateOrq2NoCuentaAgentesAjenos(t *testing.T) {
	base := t.TempDir()
	repo := filepath.Join(base, "repo")
	gitIni(t, repo)
	var dirs []string
	for i := range 5 {
		wt := filepath.Join(base, "wt"+string(rune('a'+i)))
		if out, err := exec.Command("git", "-C", repo, "worktree", "add", "-q", "-b", "w"+string(rune('a'+i)), wt).CombinedOutput(); err != nil {
			t.Fatalf("%s", out)
		}
		dirs = append(dirs, wt)
	}
	ps := procsEn(dirs)
	now := time.Now()
	st := t.TempDir()
	if err := state.SaveSnapshot(st, state.Snapshot{TS: now, ScratchGB: 1}); err != nil {
		t.Fatal(err)
	}
	four, quota, age := 4, 20.0, 24.0
	l := limits.Limits{MaxWorkers: &four, Scratch: limits.Scratch{QuotaGB: &quota}, StateMaxAgeHours: &age}
	decide := func() gate.Decision {
		env := gate.Env{
			DiskFreeGB: func(string) (float64, error) { return 500, nil },
			Workers:    func() (int, error) { return len(sysinfo.Workers(ps, []string{"claude"}, EsWorkerContable())), nil },
			Now:        func() time.Time { return now },
			StateDir:   st,
			IsWorktree: func(string) bool { return false },
		}
		return gate.Evaluate(gate.Payload{ToolName: "Agent", Cwd: repo}, l, env)
	}
	t.Setenv("ORQ_NOMBRE", "orq2")
	if d := decide(); !d.Allow {
		t.Fatalf("orq2 must not count foreign agents: %s", d.Reason)
	}
	t.Setenv("ORQ_NOMBRE", "")
	if d := decide(); d.Allow {
		t.Fatal("orq must still count linked worktrees and refuse at 5 of 4")
	}
}
