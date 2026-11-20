package sysinfo

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// procFalso builds a /proc tree: pid → comm, cwd, VmRSS kB and socket inodes.
func procFalso(t *testing.T) string {
	t.Helper()
	proc := t.TempDir()
	type p struct {
		pid     int
		comm    string
		cwd     string
		rssKB   int
		sockets []string
	}
	for _, x := range []p{
		{10, "claude", "/w/a", 900000, nil},
		{11, "node", "/w/app", 2000000, []string{"5001"}},
		{12, "codex", "/w/b", 300000, nil},
		{13, "kworker", "", -1, nil},
		{14, "python3", "/w/api", 100000, []string{"5002", "7777"}},
	} {
		d := filepath.Join(proc, strconv.Itoa(x.pid))
		os.MkdirAll(filepath.Join(d, "fd"), 0o755)
		os.WriteFile(filepath.Join(d, "comm"), []byte(x.comm+"\n"), 0o644)
		st := "Name:\t" + x.comm + "\n"
		if x.rssKB >= 0 {
			st += "VmRSS:\t  " + strconv.Itoa(x.rssKB) + " kB\n"
		}
		os.WriteFile(filepath.Join(d, "status"), []byte(st), 0o644)
		if x.cwd != "" {
			os.Symlink(x.cwd, filepath.Join(d, "cwd"))
		}
		os.Symlink("/dev/null", filepath.Join(d, "fd", "0"))
		for i, s := range x.sockets {
			os.Symlink("socket:["+s+"]", filepath.Join(d, "fd", strconv.Itoa(3+i)))
		}
	}
	os.MkdirAll(filepath.Join(proc, "net"), 0o755)
	head := "  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode\n"
	os.WriteFile(filepath.Join(proc, "net", "tcp"), []byte(head+
		"   0: 0100007F:0BB8 00000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 5001 1 0\n"+
		"   1: 0100007F:A1B2 0100007F:0BB8 01 00000000:00000000 00:00000000 00000000  1000        0 7777 1 0\n"+
		"   2: 00000000:1F90 00000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 9999 1 0\n"), 0o644)
	os.WriteFile(filepath.Join(proc, "net", "tcp6"), []byte(head+
		"   0: 00000000000000000000000000000000:1F40 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 5002 1 0\n"), 0o644)
	return proc
}

func TestProcesosPorRSS(t *testing.T) {
	ps, err := ProcesosPorRSSDe(procFalso(t), 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 3 || ps[0].PID != 11 || ps[1].PID != 10 || ps[2].PID != 12 {
		t.Fatalf("top 3 by RSS: %+v", ps)
	}
	if ps[0].Comm != "node" || ps[0].Cwd != "/w/app" || ps[0].RSSGB != 2 {
		t.Fatalf("fields: %+v", ps[0])
	}
}

// Only LISTEN sockets, v4 and v6, matched to their pid through fd links; an unowned one keeps pid 0.
func TestPuertosEscucha(t *testing.T) {
	ps, err := PuertosEscuchaDe(procFalso(t))
	if err != nil {
		t.Fatal(err)
	}
	want := []Puerto{{3000, 11, "node"}, {8000, 14, "python3"}, {8080, 0, ""}}
	if len(ps) != len(want) {
		t.Fatalf("ports: %+v", ps)
	}
	for i := range want {
		if ps[i] != want[i] {
			t.Fatalf("port %d: %+v, want %+v", i, ps[i], want[i])
		}
	}
}
