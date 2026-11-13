package sysinfo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkersCuentaSoloAgentesDePrimerNivelEnWorktrees(t *testing.T) {
	procs := []Proc{
		{PID: 1, PPID: 0, Comm: "zsh", Cwd: "/home/u"},
		{PID: 10, PPID: 1, Comm: "claude", Cwd: "/home/u/repo"},            // sesión del Owner: no cuenta
		{PID: 20, PPID: 1, Comm: "codex", Cwd: "/home/u/worktrees/s01"},    // worker
		{PID: 21, PPID: 20, Comm: "codex", Cwd: "/home/u/worktrees/s01"},   // hijo del worker: no cuenta otra vez
		{PID: 30, PPID: 1, Comm: "claude", Cwd: "/home/u/worktrees/s02/a"}, // worker en subcarpeta
		{PID: 40, PPID: 1, Comm: "node", Cwd: "/home/u/worktrees/s01"},     // no es agente
		{PID: 50, PPID: 1, Comm: "claude", Cwd: ""},                        // cwd ilegible: no cuenta
	}
	inWorktree := func(cwd string) bool { return strings.Contains(cwd, "/worktrees/") }

	got := Workers(procs, []string{"codex", "claude"}, inWorktree)

	if len(got) != 2 || got[0].PID != 20 || got[1].PID != 30 {
		t.Fatalf("esperaba workers 20 y 30, obtuve %+v", got)
	}
}

func TestUsanDetectaProcesoConCwdDentro(t *testing.T) {
	ps := []Proc{{PID: 10, Comm: "claude", Cwd: "/w/s01/src"}, {PID: 11, Comm: "zsh", Cwd: "/w/s01-int"}, {PID: 12, Cwd: "/w/s01"}}
	u := Usan(ps, "/w/s01")
	if len(u) != 2 || u[0].PID != 10 || u[1].PID != 12 {
		t.Fatalf("%+v", u)
	}
}

func TestLeerMaquinaDeProc(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "meminfo"), []byte("MemTotal:       16000000 kB\nMemFree:          100000 kB\nMemAvailable:    4500000 kB\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "loadavg"), []byte("2.25 7.94 8.93 3/1444 1543887\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "cpuinfo"), []byte("processor\t: 0\nmodel name\t: x\n\nprocessor\t: 1\n\nprocessor\t: 2\n"), 0o644)
	m, err := LeerMaquinaDe(dir)
	if err != nil || m.MemTotalGB != 16 || m.MemDisponibleGB != 4.5 || m.CPUs != 3 || m.Carga1 != 2.25 {
		t.Fatalf("%+v %v", m, err)
	}
	// Sin MemAvailable no se inventa un número.
	os.WriteFile(filepath.Join(dir, "meminfo"), []byte("MemTotal: 16000000 kB\n"), 0o644)
	if _, err := LeerMaquinaDe(dir); err == nil {
		t.Fatal("aceptó un meminfo sin MemAvailable")
	}
	if _, err := LeerMaquinaDe(filepath.Join(dir, "no")); err == nil {
		t.Fatal("aceptó un /proc que no existe")
	}
	if m, err := LeerMaquina(); err != nil || m.CPUs < 1 || m.MemDisponibleGB <= 0 {
		t.Fatalf("máquina real: %+v %v", m, err)
	}
}
