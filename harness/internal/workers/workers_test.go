package workers

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var t0 = time.Date(2026, 11, 8, 10, 0, 0, 0, time.UTC)

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	c.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %s", args, out)
	}
	return strings.TrimSpace(string(out))
}

func repo(t *testing.T) (string, string) {
	d := t.TempDir()
	git(t, d, "init", "-q", "-b", "main")
	os.WriteFile(filepath.Join(d, "a"), []byte("a"), 0o644)
	git(t, d, "add", "-A")
	git(t, d, "commit", "-qm", "base")
	return d, git(t, d, "rev-parse", "HEAD")
}

func TestLaunchedWritesAtomicState(t *testing.T) {
	orq := t.TempDir()
	pid := 4242
	if err := Launched(orq, State{ID: "w-s04-t2", Slice: "S-04", Task: "T2", Agent: "claude", PID: &pid, BudgetMin: 45}, t0); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(Path(orq, "w-s04-t2"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	json.Unmarshal(raw, &m)
	if m["schema"] != "orq.worker/v1" || m["phase"] != "running" || m["context_pct"] != nil || m["exit_code"] != nil {
		t.Fatalf("%s", raw)
	}
	if _, ok := m["tests"]; !ok {
		t.Fatal("tests key must exist (null)")
	}
	if _, err := os.Stat(Path(orq, "w-s04-t2") + ".tmp"); !os.IsNotExist(err) {
		t.Fatal("tmp file left behind")
	}
}

// Spec worker-state, acceptance 1: head and commits come from git, not from the worker.
func TestBeatComputesHeadAndCommits(t *testing.T) {
	orq := t.TempDir()
	wt, base := repo(t)
	Launched(orq, State{ID: "w1", Slice: "S-01", Task: "T1", Worktree: wt, Base: base}, t0)
	for _, f := range []string{"x", "y"} {
		os.WriteFile(filepath.Join(wt, f), []byte(f), 0o644)
		git(t, wt, "add", "-A")
		git(t, wt, "commit", "-qm", f)
	}
	st, _ := Load(orq, "w1")
	st.Head, st.Commits = "self-reported", 99
	Save(orq, st)
	pct := 30
	st, err := Beat(orq, "w1", BeatOpts{Summary: "two commits", ContextPct: &pct}, t0.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if st.Commits != 2 || st.Head != git(t, wt, "rev-parse", "HEAD") || *st.ContextPct != 30 || !st.Heartbeat.Equal(t0.Add(time.Minute)) {
		t.Fatalf("%+v", st)
	}
}

func TestBeatBlockedNeedsStop(t *testing.T) {
	orq := t.TempDir()
	Launched(orq, State{ID: "w1", Slice: "S-01"}, t0)
	if _, err := Beat(orq, "w1", BeatOpts{Phase: "blocked"}, t0); err == nil {
		t.Fatal("blocked without stop accepted")
	}
	if _, err := Beat(orq, "w1", BeatOpts{Phase: "flying"}, t0); err == nil {
		t.Fatal("bad phase accepted")
	}
	st, err := Beat(orq, "w1", BeatOpts{Phase: "blocked", Stop: "ST-0001"}, t0)
	if err != nil || st.Stop != "ST-0001" {
		t.Fatal(err)
	}
}

func TestDone(t *testing.T) {
	orq := t.TempDir()
	Launched(orq, State{ID: "w1", Slice: "S-01", Task: "T1"}, t0)
	st, err := Done(orq, "w1", "ok", true, t0)
	if err != nil || st.Phase != "asks_verification" || st.ExitCode == nil || *st.ExitCode != 0 {
		t.Fatalf("%+v %v", st, err)
	}
	st, _ = Done(orq, "w1", "ok", false, t0)
	if st.Phase != "exited" {
		t.Fatal(st.Phase)
	}
	if err := AppendTaskEvent(orq, "S-01", "T1", "done", "w1", t0); err != nil {
		t.Fatal(err)
	}
	ev, _ := LoadTaskEvents(orq)
	if len(ev) != 1 || ev[0].Event != "done" {
		t.Fatalf("%+v", ev)
	}
	ls, _ := List(orq)
	if len(ls) != 1 {
		t.Fatal(ls)
	}
}
