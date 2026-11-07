package sysinfo

import (
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
