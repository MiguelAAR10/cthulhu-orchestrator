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

// N3: kept ungated copies are capped at the newest 2; the integrator's file and the current
// rollback are never touched.
func TestGuardarActualLimitaCopiasSinPuerta(t *testing.T) {
	d := t.TempDir()
	c := ConfUpdate{Repo: d, Bin: filepath.Join(d, "bin"), Rollback: filepath.Join(d, "rb")}
	os.MkdirAll(c.Bin, 0o755)
	os.MkdirAll(c.Rollback, 0o755)
	os.WriteFile(filepath.Join(c.Rollback, "orq2-24f2438"), []byte("integrador"), 0o755)
	os.WriteFile(c.RutaRollback(), []byte("bueno"), 0o755)
	for i := 0; i < 5; i++ {
		os.WriteFile(c.Instalado(), []byte("#!/bin/sh\necho '2.0.0-pilot revision=desconocida-"+string(rune('a'+i))+" modified=true'\n"), 0o755)
		if _, err := c.GuardarActual(); err != nil {
			t.Fatal(err)
		}
	}
	copias, _ := filepath.Glob(filepath.Join(c.Rollback, "orq2-sin-puerta-*"))
	if len(copias) != 2 {
		t.Fatalf("copies kept: %v", copias)
	}
	for _, p := range copias {
		raw, _ := os.ReadFile(p)
		if !strings.Contains(string(raw), "-d ") && !strings.Contains(string(raw), "-e ") {
			t.Errorf("an old copy survived instead of the newest: %s = %s", p, raw)
		}
	}
	if raw, _ := os.ReadFile(filepath.Join(c.Rollback, "orq2-24f2438")); string(raw) != "integrador" {
		t.Fatal("integrator's file touched")
	}
	if raw, _ := os.ReadFile(c.RutaRollback()); string(raw) != "bueno" {
		t.Fatal("current rollback touched")
	}
}
