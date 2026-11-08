package identidad

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

type tablaFalsa struct {
	ppid map[int]int
	env  map[int]map[string]string
}

func (t tablaFalsa) PPID(pid int) (int, error) {
	p, ok := t.ppid[pid]
	if !ok {
		return 0, os.ErrNotExist
	}
	return p, nil
}
func (t tablaFalsa) Environ(pid int) (map[string]string, error) { return t.env[pid], nil }

func TestWorkerAncestroSobreviveAlUnset(t *testing.T) {
	// 50 (exploit, env reescrito a owner) <- 40 (bash) <- 30 (agente worker) <- 1
	tb := tablaFalsa{
		ppid: map[int]int{50: 40, 40: 30, 30: 1},
		env: map[int]map[string]string{
			40: {"ORQ_ROL": "owner"},
			30: {"ORQ_WORKER_ID": "w1", "ORQ_ROL": "w1"},
		},
	}
	if id, ok := WorkerAncestro(tb, 50); !ok || id != "w1" {
		t.Fatalf("debía heredar w1: %q %v", id, ok)
	}
	tb.env[30] = map[string]string{"ORQ_ROL": "orquestador"}
	if _, ok := WorkerAncestro(tb, 50); ok {
		t.Fatal("el orquestador no es worker")
	}
	tb.env[30] = map[string]string{"ORQ_ROL": "tester"}
	if id, ok := WorkerAncestro(tb, 50); !ok || id != "tester" {
		t.Fatal("tester es worker")
	}
	tb.env[30] = map[string]string{"ORQ_SHIM": "1"}
	if _, ok := WorkerAncestro(tb, 50); !ok {
		t.Fatal("el shim marca worker")
	}
	if _, ok := WorkerAncestro(nil, 50); ok {
		t.Fatal("sin tabla no hay marca")
	}
	// ciclo no cuelga
	if _, ok := WorkerAncestro(tablaFalsa{ppid: map[int]int{5: 6, 6: 5}}, 5); ok {
		t.Fatal("ciclo")
	}
}

func TestProcFSParsea(t *testing.T) {
	r := t.TempDir()
	os.MkdirAll(filepath.Join(r, "7"), 0o755)
	os.WriteFile(filepath.Join(r, "7", "stat"), []byte("7 (a b) c)) S 3 7 7 0"), 0o644)
	os.WriteFile(filepath.Join(r, "7", "environ"), []byte("HOME=/h\x00ORQ_WORKER_ID=w9\x00X=1=2\x00"), 0o644)
	p := ProcFS{Root: r}
	if pp, err := p.PPID(7); err != nil || pp != 3 {
		t.Fatal(pp, err)
	}
	if e, _ := p.Environ(7); e["ORQ_WORKER_ID"] != "w9" {
		t.Fatal(e)
	}
	if _, err := os.Stat("/proc/self/stat"); err == nil {
		pp, err := ProcFS{}.PPID(os.Getpid())
		if err != nil || pp != os.Getppid() {
			t.Fatalf("real: %d %v (want %s)", pp, err, strconv.Itoa(os.Getppid()))
		}
	}
}
