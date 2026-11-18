package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"orq/internal/limits"
	"orq/internal/orqdir"
)

// A worker whose cwd is an orq clone (marker in .git, not a linked worktree) counts against max_workers.
func TestCapacidadCuentaWorkersEnClones(t *testing.T) {
	clon := t.TempDir()
	if out, err := exec.Command("git", "init", "-q", clon).CombinedOutput(); err != nil {
		t.Fatalf("%s", out)
	}
	if err := os.WriteFile(filepath.Join(clon, ".git", orqdir.MarcaClon), []byte(`{"slice":"01"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	// A stand-in worker process: a copy of sleep under a name nothing else on the machine uses.
	sleep, err := exec.LookPath("sleep")
	if err != nil {
		t.Skip("no sleep binary")
	}
	raw, err := os.ReadFile(sleep)
	if err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(t.TempDir(), "orqfakewrk")
	if err := os.WriteFile(bin, raw, 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bin, "30")
	cmd.Dir = clon
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { cmd.Process.Kill(); cmd.Wait() }()
	vivos, err := MedirReal().Vivos(limits.Limits{WorkerProcessName: []string{"orqfakewrk"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range vivos {
		if p.PID == cmd.Process.Pid {
			return
		}
	}
	t.Fatalf("a worker running in a clone was not counted: %+v", vivos)
}
