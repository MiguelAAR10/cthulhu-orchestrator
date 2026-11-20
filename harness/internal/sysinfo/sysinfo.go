// Package sysinfo lee disco y procesos. Solo Linux (WSL).
package sysinfo

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

// FreeGB devuelve los GB disponibles para el usuario en el sistema de archivos de path.
func FreeGB(path string) (float64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, err
	}
	return float64(st.Bavail) * float64(st.Bsize) / 1e9, nil
}

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
// Los hijos de primer nivel de root se recorren en paralelo (hasta tamanoParalelo a la vez), con un
// solo mapa de inodos: el total es el mismo que el de un recorrido en serie.
func DirSizeBytes(root string) (int64, error) {
	var (
		mu    sync.Mutex
		seen  = map[uint64]bool{}
		total int64
	)
	sumar := func(p string, d fs.DirEntry, err error) error {
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
		mu.Lock()
		defer mu.Unlock()
		if st, ok := info.Sys().(*syscall.Stat_t); ok && st.Nlink > 1 {
			if seen[st.Ino] {
				return nil
			}
			seen[st.Ino] = true
		}
		total += info.Size()
		return nil
	}
	hijos, err := os.ReadDir(root)
	if err != nil {
		// No es un directorio legible (un archivo, o no existe): el recorrido en serie decide.
		err := filepath.WalkDir(root, sumar)
		return total, err
	}
	trabajo := make(chan string)
	var wg sync.WaitGroup
	for range tamanoParalelo {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for p := range trabajo {
				filepath.WalkDir(p, sumar)
			}
		}()
	}
	for _, h := range hijos {
		p := filepath.Join(root, h.Name())
		if h.IsDir() {
			trabajo <- p
		} else {
			sumar(p, h, nil)
		}
	}
	close(trabajo)
	wg.Wait()
	return total, nil
}

// tamanoParalelo acota los recorridos simultáneos de DirSizeBytes.
const tamanoParalelo = 8

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
