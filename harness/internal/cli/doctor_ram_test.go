package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// doctor --ram only suggests: top processes with agent / orq2-worker marks, listening ports with
// their pid, and a suggestion at the end of each line. It never stops anything.
func TestDoctorRAMSugiere(t *testing.T) {
	proc := t.TempDir()
	for _, x := range []struct {
		pid       int
		comm, cwd string
		rss       int
		sock      string
	}{{20, "claude", "/w/propio", 3000000, ""}, {21, "claude", "/w/ajeno", 2000000, ""}, {22, "node", "/w/app", 1000000, "42"}} {
		d := filepath.Join(proc, strconv.Itoa(x.pid))
		os.MkdirAll(filepath.Join(d, "fd"), 0o755)
		os.WriteFile(filepath.Join(d, "comm"), []byte(x.comm+"\n"), 0o644)
		os.WriteFile(filepath.Join(d, "status"), []byte("VmRSS:\t"+strconv.Itoa(x.rss)+" kB\n"), 0o644)
		os.Symlink(x.cwd, filepath.Join(d, "cwd"))
		if x.sock != "" {
			os.Symlink("socket:["+x.sock+"]", filepath.Join(d, "fd", "3"))
		}
	}
	os.MkdirAll(filepath.Join(proc, "net"), 0o755)
	os.WriteFile(filepath.Join(proc, "net", "tcp"), []byte("hdr\n   0: 00000000:0BB8 00000000:0000 0A 0:0 0:0 0 1000 0 42 1 0\n"), 0o644)
	os.WriteFile(filepath.Join(proc, "meminfo"), []byte("MemTotal: 32000000 kB\nMemAvailable: 4000000 kB\n"), 0o644)
	os.WriteFile(filepath.Join(proc, "loadavg"), []byte("1.0 1.0 1.0 1/1 1\n"), 0o644)

	var out bytes.Buffer
	propio := func(cwd string) bool { return cwd == "/w/propio" }
	if code := doctorRAM(&out, proc, []string{"claude", "codex"}, propio); code != 0 {
		t.Fatalf("code %d:\n%s", code, out.String())
	}
	o := out.String()
	contiene(t, o, "RAM libre 4.0 GB de 32.0", "solo sugerencias",
		"20", "claude", "/w/propio", "worker orq2", "3.0 GB",
		"/w/ajeno", "agente", "→ cerrar panel",
		"/w/app", "→ detener servidor en puerto 3000",
		"puerto 3000", "pid 22 node")
	var ls []string
	for _, l := range strings.Split(strings.TrimSpace(o), "\n") {
		if strings.HasPrefix(l, "  ") {
			ls = append(ls, l)
		}
	}
	for _, l := range ls {
		if !strings.Contains(l, "→ ") {
			t.Errorf("line without a suggestion: %q", l)
		}
	}
}

// doctor --ram is orq2's; it runs on the real /proc without a project.
func TestDoctorRAMSoloOrq2(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ORQ_STATE", t.TempDir())
	t.Setenv("ORQ_NOMBRE", "orq2")
	t.Chdir(t.TempDir())
	out, errb, code := run(t, "doctor", "--ram")
	if code != 0 || !strings.Contains(out, "solo sugerencias") {
		t.Fatalf("orq2 doctor --ram (code %d):\n%s\n%s", code, out, errb)
	}
}
