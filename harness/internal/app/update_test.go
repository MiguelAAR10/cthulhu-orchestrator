package app

import (
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"testing"
)

// H1: the real binary's configuration comes from the account, never from the environment an
// agent controls: not HOME, not ORQ_UPDATE_*, not ORQ_WORKTREES, not PATH.
func TestConfUpdateFijaNoLeeElEntorno(t *testing.T) {
	u, err := user.Current()
	if err != nil || u.HomeDir == "" {
		t.Skip("no account home")
	}
	malo := t.TempDir()
	for _, k := range []string{"ORQ_UPDATE_REPO", "ORQ_UPDATE_RAMA", "ORQ_UPDATE_BIN", "ORQ_UPDATE_ROLLBACK", "ORQ_UPDATE_GO", "ORQ_UPDATE_PROC", "ORQ_WORKTREES"} {
		t.Setenv(k, malo)
	}
	t.Setenv("HOME", malo)
	t.Setenv("PATH", malo)
	c, err := ConfUpdateFija()
	if err != nil {
		t.Skipf("no pinned go on this machine: %v", err)
	}
	want := ConfUpdate{Repo: filepath.Join(u.HomeDir, "projects", "MiguelAR10-Builder", "AI-Edge-Runners", "experiments", "cthulhu-orchestrator"),
		Rama: "harness-v2", Bin: filepath.Join(u.HomeDir, "bin"), Rollback: filepath.Join(u.HomeDir, ".local", "share", "orq2-rollback"),
		Proc: "/proc", Worktrees: filepath.Join(u.HomeDir, "worktrees"), Go: c.Go}
	if c.Repo != want.Repo || c.Rama != want.Rama || c.Bin != want.Bin || c.Rollback != want.Rollback || c.Proc != want.Proc || c.Worktrees != want.Worktrees || len(c.Proyectos) != 0 {
		t.Fatalf("got  %+v\nwant %+v", c, want)
	}
	if !filepath.IsAbs(c.Go) || strings.HasPrefix(c.Go, malo) {
		t.Fatalf("go is not pinned: %q", c.Go)
	}
}

// H1: any ORQ_UPDATE_* override is refused outright, so nobody believes it took effect.
func TestRevisarEntornoUpdate(t *testing.T) {
	if err := RevisarEntornoUpdate([]string{"PATH=/bin", "HOME=/h"}); err != nil {
		t.Fatal(err)
	}
	err := RevisarEntornoUpdate([]string{"PATH=/bin", "ORQ_UPDATE_BIN=/tmp/x"})
	if err == nil || !strings.Contains(err.Error(), "ORQ_UPDATE_BIN") {
		t.Fatalf("override accepted: %v", err)
	}
}

// H1: git and go run with a pinned PATH and without the variables that redirect git.
func TestSanearEntornoUpdate(t *testing.T) {
	t.Setenv("PATH", "/malo/bin:/usr/bin")
	for _, k := range []string{"GIT_DIR", "GIT_WORK_TREE", "GIT_CONFIG_GLOBAL", "GIT_EXEC_PATH", "GIT_ALTERNATE_OBJECT_DIRECTORIES", "GIT_CONFIG_COUNT"} {
		t.Setenv(k, "/malo")
	}
	t.Setenv("GIT_AUTHOR_NAME", "a")
	SanearEntornoUpdate()
	if os.Getenv("PATH") != PathUpdate {
		t.Fatalf("PATH %q", os.Getenv("PATH"))
	}
	for _, k := range []string{"GIT_DIR", "GIT_WORK_TREE", "GIT_CONFIG_GLOBAL", "GIT_EXEC_PATH", "GIT_ALTERNATE_OBJECT_DIRECTORIES", "GIT_CONFIG_COUNT"} {
		if _, ok := os.LookupEnv(k); ok {
			t.Errorf("%s still set", k)
		}
	}
	if os.Getenv("GIT_AUTHOR_NAME") != "a" {
		t.Error("commit identity dropped")
	}
}
