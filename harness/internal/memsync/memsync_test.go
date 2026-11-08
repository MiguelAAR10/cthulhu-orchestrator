package memsync

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func g(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	c.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %s", args, out)
	}
	return string(out)
}

// setup: bare remote + clone on a feature branch with .orq content, in branch mode.
func setup(t *testing.T) (root, remote string) {
	base := t.TempDir()
	remote = filepath.Join(base, "remote.git")
	g(t, base, "init", "-q", "--bare", "-b", "main", remote)
	root = filepath.Join(base, "repo")
	g(t, base, "init", "-q", "-b", "main", root)
	g(t, root, "remote", "add", "origin", remote)
	os.WriteFile(filepath.Join(root, "app.txt"), []byte("v1"), 0o644)
	if err := SetMode(root, "branch"); err != nil {
		t.Fatal(err)
	}
	g(t, root, "add", "-A")
	g(t, root, "commit", "-qm", "init")
	g(t, root, "push", "-q", "origin", "main")
	g(t, root, "checkout", "-q", "-b", "feature")
	orq := filepath.Join(root, ".orq")
	os.MkdirAll(filepath.Join(orq, "workers", "w1"), 0o755)
	os.WriteFile(filepath.Join(orq, "memoria.jsonl"), []byte(`{"id":"m1"}`+"\n"), 0o644)
	os.WriteFile(filepath.Join(orq, "workers", "w1", "state.json"), []byte("{}"), 0o644)
	os.WriteFile(filepath.Join(orq, "x.lock"), nil, 0o644)
	return root, remote
}

func snapshot(t *testing.T, root string) string {
	idx, _ := os.ReadFile(filepath.Join(root, ".git", "index"))
	return g(t, root, "rev-parse", "HEAD") + g(t, root, "symbolic-ref", "HEAD") + g(t, root, "status", "--porcelain=v1", "-uall") + string(idx) + g(t, root, "branch", "--list", "main", "feature")
}

// Spec worker-state, acceptance 4.
func TestPushNeverTouchesUserCheckout(t *testing.T) {
	root, remote := setup(t)
	os.WriteFile(filepath.Join(root, "app.txt"), []byte("dirty"), 0o644) // uncommitted change
	os.WriteFile(filepath.Join(root, "staged.txt"), []byte("s"), 0o644)
	g(t, root, "add", "staged.txt")
	before := snapshot(t, root)
	out, err := Push(root, time.Now())
	if err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if after := snapshot(t, root); after != before {
		t.Fatalf("user checkout changed:\n%s\n---\n%s", before, after)
	}
	files := g(t, remote, "ls-tree", "-r", "--name-only", "orq/state")
	if !strings.Contains(files, "orq/memoria.jsonl") || strings.Contains(files, "workers/") || strings.Contains(files, ".lock") {
		t.Fatalf("remote tree: %s", files)
	}
	if _, err := os.Stat(filepath.Join(root, ".orq-sync")); !os.IsNotExist(err) {
		t.Fatal(".orq-sync left behind")
	}
	n1 := strings.TrimSpace(g(t, remote, "rev-list", "--count", "orq/state"))
	// second push with a change makes a new commit
	os.WriteFile(filepath.Join(root, ".orq", "memoria.jsonl"), []byte(`{"id":"m2"}`+"\n"), 0o644)
	if out, err := Push(root, time.Now()); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	if n := strings.TrimSpace(g(t, remote, "rev-list", "--count", "orq/state")); n == n1 {
		t.Fatalf("no new commit on orq/state: %s", n)
	}
}

// Spec worker-state, acceptance 5.
func TestPushStopsOnDivergence(t *testing.T) {
	root, remote := setup(t)
	if _, err := Push(root, time.Now()); err != nil {
		t.Fatal(err)
	}
	// someone else pushes on top of the remote
	other := filepath.Join(t.TempDir(), "other")
	g(t, filepath.Dir(other), "clone", "-q", remote, other)
	g(t, other, "fetch", "-q", "origin", "orq/state:orq/state")
	g(t, other, "checkout", "-q", "orq/state")
	os.WriteFile(filepath.Join(other, "orq", "remote.txt"), []byte("r"), 0o644)
	g(t, other, "add", "-A")
	g(t, other, "commit", "-qm", "remote")
	g(t, other, "push", "-q", "origin", "orq/state")
	remoteHead := strings.TrimSpace(g(t, remote, "rev-parse", "orq/state"))
	// and we have an unpushed local commit on orq/state
	tree := strings.TrimSpace(g(t, root, "rev-parse", "orq/state^{tree}"))
	c := strings.TrimSpace(g(t, root, "commit-tree", tree, "-p", "orq/state", "-m", "local"))
	g(t, root, "update-ref", "refs/heads/orq/state", c)

	out, err := Push(root, time.Now())
	if err == nil || !strings.Contains(err.Error()+out, "diverged") {
		t.Fatalf("expected diverged, got %v %s", err, out)
	}
	if strings.TrimSpace(g(t, remote, "rev-parse", "orq/state")) != remoteHead {
		t.Fatal("remote changed: no force push allowed")
	}
}

func TestPullCopiesWithoutOverwritingNewer(t *testing.T) {
	root, remote := setup(t)
	if _, err := Push(root, time.Now()); err != nil {
		t.Fatal(err)
	}
	// second machine
	m2 := filepath.Join(t.TempDir(), "m2")
	g(t, filepath.Dir(m2), "clone", "-q", remote, m2)
	if _, err := Pull(m2, false); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(m2, ".orq", "memoria.jsonl"))
	if err != nil || !strings.Contains(string(raw), "m1") {
		t.Fatalf("%s %v", raw, err)
	}
	// a newer local file is kept unless --theirs
	future := time.Now().Add(time.Hour)
	os.WriteFile(filepath.Join(m2, ".orq", "memoria.jsonl"), []byte("local"), 0o644)
	os.Chtimes(filepath.Join(m2, ".orq", "memoria.jsonl"), future, future)
	out, _ := Pull(m2, false)
	raw, _ = os.ReadFile(filepath.Join(m2, ".orq", "memoria.jsonl"))
	if string(raw) != "local" || !strings.Contains(out, "kept") {
		t.Fatalf("%s / %s", raw, out)
	}
	Pull(m2, true)
	raw, _ = os.ReadFile(filepath.Join(m2, ".orq", "memoria.jsonl"))
	if !strings.Contains(string(raw), "m1") {
		t.Fatal("--theirs did not overwrite")
	}
}

func TestSetModeGitignore(t *testing.T) {
	root := t.TempDir()
	g(t, root, "init", "-q")
	os.WriteFile(filepath.Join(root, ".gitignore"), []byte("node_modules/\n"), 0o644)
	os.MkdirAll(filepath.Join(root, ".git", "info"), 0o755)
	os.WriteFile(filepath.Join(root, ".git", "info", "exclude"), []byte("/.orq/\n"), 0o644)
	SetMode(root, "commit")
	raw, _ := os.ReadFile(filepath.Join(root, ".gitignore"))
	ex, _ := os.ReadFile(filepath.Join(root, ".git", "info", "exclude"))
	s := string(raw)
	if !strings.Contains(s, "node_modules/") || !strings.Contains(s, "/.orq/workers/") || strings.Contains(s, "\n/.orq/\n") || strings.Contains(string(ex), "/.orq/") {
		t.Fatalf("commit: %q exclude %q", s, ex)
	}
	SetMode(root, "local")
	raw, _ = os.ReadFile(filepath.Join(root, ".gitignore"))
	if strings.Contains(string(raw), "/.orq/workers/") || !strings.Contains(string(raw), "/.orq/\n") || strings.Count(string(raw), "node_modules") != 1 {
		t.Fatalf("local: %q", raw)
	}
	if err := SetMode(root, "cloud"); err == nil {
		t.Fatal("bad mode accepted")
	}
}
