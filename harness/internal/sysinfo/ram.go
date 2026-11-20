package sysinfo

import (
	"bufio"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// ProcRSS is a process and the resident memory it holds.
type ProcRSS struct {
	PID       int
	Comm, Cwd string
	RSSGB     float64
}

// ProcesosPorRSSDe returns the n processes holding the most resident memory under proc.
// Processes without VmRSS (kernel threads) or unreadable ones are skipped.
func ProcesosPorRSSDe(proc string, n int) ([]ProcRSS, error) {
	ents, err := os.ReadDir(proc)
	if err != nil {
		return nil, err
	}
	var out []ProcRSS
	for _, e := range ents {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		d := filepath.Join(proc, e.Name())
		kb, ok := vmRSS(filepath.Join(d, "status"))
		if !ok {
			continue
		}
		c, _ := os.ReadFile(filepath.Join(d, "comm"))
		cwd, _ := os.Readlink(filepath.Join(d, "cwd"))
		out = append(out, ProcRSS{PID: pid, Comm: strings.TrimSpace(string(c)), Cwd: cwd, RSSGB: kb / 1e6})
	}
	slices.SortStableFunc(out, func(a, b ProcRSS) int {
		switch {
		case a.RSSGB > b.RSSGB:
			return -1
		case a.RSSGB < b.RSSGB:
			return 1
		}
		return a.PID - b.PID
	})
	if len(out) > n {
		out = out[:n]
	}
	return out, nil
}

func vmRSS(status string) (float64, bool) {
	f, err := os.Open(status)
	if err != nil {
		return 0, false
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if v, ok := strings.CutPrefix(sc.Text(), "VmRSS:"); ok {
			if fs := strings.Fields(v); len(fs) > 0 {
				kb, err := strconv.ParseFloat(fs[0], 64)
				return kb, err == nil
			}
		}
	}
	return 0, false
}

// Puerto is a listening TCP port and the process that owns its socket (PID 0 if none was found).
type Puerto struct {
	Port int
	PID  int
	Comm string
}

// PuertosEscuchaDe reads the LISTEN sockets of proc/net/tcp and tcp6 and finds their owner through
// proc/<pid>/fd links (socket:[inode]). Sorted by port; one line per port and owner.
func PuertosEscuchaDe(proc string) ([]Puerto, error) {
	inodos := map[string]int{} // inode → port
	leido := false
	for _, f := range []string{"tcp", "tcp6"} {
		raw, err := os.ReadFile(filepath.Join(proc, "net", f))
		if err != nil {
			continue
		}
		leido = true
		for _, l := range strings.Split(string(raw), "\n")[1:] {
			fs := strings.Fields(l)
			if len(fs) < 10 || fs[3] != "0A" {
				continue
			}
			_, hex, ok := strings.Cut(fs[1], ":")
			if !ok {
				continue
			}
			if p, err := strconv.ParseInt(hex, 16, 32); err == nil {
				inodos[fs[9]] = int(p)
			}
		}
	}
	if !leido {
		return nil, os.ErrNotExist
	}
	dueño := map[string]Puerto{}
	ents, _ := os.ReadDir(proc)
	for _, e := range ents {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		fds, _ := os.ReadDir(filepath.Join(proc, e.Name(), "fd"))
		for _, fd := range fds {
			l, err := os.Readlink(filepath.Join(proc, e.Name(), "fd", fd.Name()))
			if err != nil || !strings.HasPrefix(l, "socket:[") {
				continue
			}
			ino := strings.TrimSuffix(strings.TrimPrefix(l, "socket:["), "]")
			if port, ok := inodos[ino]; ok {
				if _, visto := dueño[ino]; !visto {
					c, _ := os.ReadFile(filepath.Join(proc, e.Name(), "comm"))
					dueño[ino] = Puerto{Port: port, PID: pid, Comm: strings.TrimSpace(string(c))}
				}
			}
		}
	}
	var out []Puerto
	for ino, port := range inodos {
		p, ok := dueño[ino]
		if !ok {
			p = Puerto{Port: port}
		}
		if !slices.Contains(out, p) {
			out = append(out, p)
		}
	}
	slices.SortFunc(out, func(a, b Puerto) int {
		if a.Port != b.Port {
			return a.Port - b.Port
		}
		return a.PID - b.PID
	})
	return out, nil
}
