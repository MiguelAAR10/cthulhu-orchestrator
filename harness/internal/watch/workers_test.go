package watch

import (
	"os"
	"strings"
	"testing"
	"time"

	"orq/internal/workers"
)

var w0 = time.Date(2026, 11, 8, 10, 0, 0, 0, time.UTC)

type sink struct{ msgs []string }

func (s *sink) env(now time.Time, alive bool) WorkerEnv {
	return WorkerEnv{
		Now: now, StaleMin: 10, Host: "h1",
		Alive:  func(int) bool { return alive },
		Notify: func(w, v, r string, seq int) error { s.msgs = append(s.msgs, w+":"+v); return nil },
	}
}

func lines(t *testing.T, path string) []string {
	raw, _ := os.ReadFile(path)
	var out []string
	for _, l := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if l != "" {
			out = append(out, l)
		}
	}
	return out
}

// Spec worker-state, acceptance 2.
func TestStuckLoggedOnce(t *testing.T) {
	orq := t.TempDir()
	workers.Save(orq, workers.State{ID: "w1", Slice: "S-01", Phase: "running", Host: "h1", Head: "abc",
		Started: w0.Add(-11 * time.Minute), Heartbeat: w0.Add(-11 * time.Minute)})
	s := &sink{}
	for i := 0; i < 2; i++ {
		if _, err := Workers(orq, s.env(w0, true)); err != nil {
			t.Fatal(err)
		}
	}
	l := lines(t, WatchLogPath(orq))
	if len(l) != 1 || !strings.Contains(l[0], `"verdict":"stuck"`) || len(s.msgs) != 1 {
		t.Fatalf("log %v msgs %v", l, s.msgs)
	}
	// a third pass changes nothing
	Workers(orq, s.env(w0.Add(time.Minute), true))
	if len(lines(t, WatchLogPath(orq))) != 1 || len(s.msgs) != 1 {
		t.Fatal("repeated verdict logged twice")
	}
}

// Spec worker-state, acceptance 3.
func TestDeadNeverTouches(t *testing.T) {
	orq := t.TempDir()
	wt := t.TempDir()
	os.WriteFile(wt+"/f", []byte("x"), 0o644)
	pid := 999999
	workers.Save(orq, workers.State{ID: "w1", Phase: "running", Host: "h1", PID: &pid, Worktree: wt, Started: w0, Heartbeat: w0})
	s := &sink{}
	vs, err := Workers(orq, WorkerEnv{Now: w0, StaleMin: 10, Host: "h1", Alive: PIDAlive,
		Notify: func(w, v, r string, seq int) error { s.msgs = append(s.msgs, v); return nil }})
	if err != nil || len(vs) != 1 || vs[0].Verdict != "dead" {
		t.Fatalf("%+v %v", vs, err)
	}
	if _, err := os.Stat(wt + "/f"); err != nil {
		t.Fatal("worktree touched")
	}
	st, _ := workers.Load(orq, "w1")
	if st.Phase != "running" {
		t.Fatal("watcher must not change the phase of a dead worker")
	}
}

func TestDeadOnOtherHostIsNotJudged(t *testing.T) {
	pid := 999999
	st := workers.State{Phase: "running", Host: "other", PID: &pid, Started: w0, Heartbeat: w0}
	v, _, _ := Judge(st, "", false, w0, 10, "h1", func(int) bool { return false })
	if v != "ok" {
		t.Fatal(v)
	}
}

func TestTimeoutWarnAndActRaisesOnce(t *testing.T) {
	orq := t.TempDir()
	workers.Save(orq, workers.State{ID: "w1", Slice: "S-01", Task: "T1", Phase: "running", Host: "h1", BudgetMin: 10,
		Started: w0.Add(-11 * time.Minute), Heartbeat: w0})
	s := &sink{}
	raised := 0
	env := s.env(w0, true)
	env.Raise = func(st workers.State, reason string) error { raised++; return nil }
	vs, _ := Workers(orq, env)
	if vs[0].Verdict != "timeout" || raised != 0 {
		t.Fatalf("warn: %+v raised %d", vs, raised)
	}
	env.Now = w0.Add(5 * time.Minute) // 16 min > 15 → act
	Workers(orq, env)
	env.Now = w0.Add(6 * time.Minute)
	Workers(orq, env)
	if raised != 1 {
		t.Fatalf("raised %d", raised)
	}
	if len(s.msgs) != 1 {
		t.Fatalf("one message per verdict change: %v", s.msgs)
	}
}

func TestExitedAndOK(t *testing.T) {
	st := workers.State{Phase: "exited", Started: w0, Heartbeat: w0}
	if v, _, _ := Judge(st, "", false, w0, 10, "h", nil); v != "exited" {
		t.Fatal(v)
	}
	st.Phase = "running"
	if v, _, _ := Judge(st, "x", true, w0.Add(time.Minute), 10, "h", nil); v != "ok" {
		t.Fatal(v)
	}
}
