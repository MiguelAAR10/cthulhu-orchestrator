// Package sysinfo lee disco y procesos. Solo Linux (WSL).
package sysinfo

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Proc es lo mínimo que el gate necesita de un proceso.
type Proc struct {
	PID, PPID int
	Comm, Cwd string
}

// ListProcs lee /proc. Los procesos ilegibles se omiten.
func ListProcs() ([]Proc, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}
	var out []Proc
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		c, err := os.ReadFile(filepath.Join("/proc", e.Name(), "comm"))
		if err != nil {
			continue
		}
		p := Proc{PID: pid, Comm: strings.TrimSpace(string(c))}
		p.PPID, _ = parentPID(pid)
		p.Cwd, _ = os.Readlink(filepath.Join("/proc", e.Name(), "cwd"))
		out = append(out, p)
	}
	return out, nil
}

// Workers devuelve los agentes de primer nivel (nombre en names, padre no) cuyo
// directorio de trabajo cumple isWorker. Un agente con sus hijos cuenta una vez.
// Las sesiones interactivas del Owner, abiertas fuera de un worktree, no cuentan.
func Workers(procs []Proc, names []string, isWorker func(cwd string) bool) []Proc {
	want := map[string]bool{}
	for _, n := range names {
		want[n] = true
	}
	comm := map[int]string{}
	for _, p := range procs {
		comm[p.PID] = p.Comm
	}
	var out []Proc
	for _, p := range procs {
		if want[p.Comm] && !want[comm[p.PPID]] && p.Cwd != "" && isWorker(p.Cwd) {
			out = append(out, p)
		}
	}
	return out
}

func parentPID(pid int) (int, bool) {
	raw, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return 0, false
	}
	// El comm va entre paréntesis y puede tener espacios: cortar tras el último ')'.
	i := bytes.LastIndexByte(raw, ')')
	if i < 0 {
		return 0, false
	}
	fields := strings.Fields(string(raw[i+1:]))
	if len(fields) < 2 {
		return 0, false
	}
	p, err := strconv.Atoi(fields[1])
	return p, err == nil
}

// DirSizeBytes suma archivos regulares sin seguir symlinks y cuenta hardlinks una vez.
func DirSizeBytes(root string) (int64, error) {
	seen := map[uint64]bool{}
	var total int64
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // directorio ilegible: se omite, no se aborta
		}
		if !d.Type().IsRegular() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if ino, ok := enlaceDuro(info); ok {
			if seen[ino] {
				return nil
			}
			seen[ino] = true
		}
		total += info.Size()
		return nil
	})
	return total, err
}

// Usan devuelve los procesos cuyo directorio de trabajo está dentro de dir (sin contar el propio).
func Usan(procs []Proc, dir string) []Proc {
	dir = filepath.Clean(dir)
	var out []Proc
	for _, p := range procs {
		if p.PID == os.Getpid() || p.Cwd == "" {
			continue
		}
		if c := filepath.Clean(p.Cwd); c == dir || strings.HasPrefix(c, dir+string(filepath.Separator)) {
			out = append(out, p)
		}
	}
	return out
}
