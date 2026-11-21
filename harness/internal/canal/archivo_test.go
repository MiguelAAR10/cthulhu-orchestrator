package canal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// H2: closing a story archives its channel; ids keep growing so "c3" names one message forever.
func TestArchivarGuardaElCanalYLosIdsSiguen(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq2")
	dir := t.TempDir()
	p := filepath.Join(dir, "canal.jsonl")
	for i := 0; i < 3; i++ {
		if err := Append(p, msg(fmt.Sprint(i))); err != nil {
			t.Fatal(err)
		}
	}
	arch, err := Archivar(p, "S-01", nil)
	if err != nil {
		t.Fatal(err)
	}
	viejos, err := Load(arch)
	if err != nil || len(viejos) != 3 || viejos[2].ID != "c3" {
		t.Fatalf("archive %s: %+v %v", arch, viejos, err)
	}
	if ms, _ := Load(p); len(ms) != 0 {
		t.Fatalf("the live channel must start empty after archiving: %+v", ms)
	}
	m := msg("nueva historia")
	if err := Append(p, m); err != nil {
		t.Fatal(err)
	}
	if m.ID != "c4" {
		t.Fatalf("id after archiving = %s, want c4 (ids never restart)", m.ID)
	}
	// A second close of the same story never overwrites the first archive.
	arch2, err := Archivar(p, "S-01", nil)
	if err != nil || arch2 == arch {
		t.Fatalf("second archive %s (first %s): %v", arch2, arch, err)
	}
	if viejos, _ := Load(arch); len(viejos) != 3 {
		t.Fatalf("first archive was overwritten: %+v", viejos)
	}
	m = msg("otra")
	Append(p, m)
	if m.ID != "c5" {
		t.Fatalf("id after the second archive = %s, want c5", m.ID)
	}
}

// Messages sent while the channel is archived land in the archive or in the new channel, never lost.
func TestArchivarConcurrenteNoPierdeMensajes(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq2")
	p := filepath.Join(t.TempDir(), "canal.jsonl")
	Append(p, msg("primero"))
	var wg sync.WaitGroup
	var archs []string
	var mu sync.Mutex
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i%10 == 5 {
				a, err := Archivar(p, "S-01", nil)
				if err != nil {
					t.Error(err)
				}
				mu.Lock()
				archs = append(archs, a)
				mu.Unlock()
				return
			}
			if err := Append(p, msg(fmt.Sprint(i))); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	vistos := map[string]bool{}
	for _, f := range append(archs, p) {
		ms, err := Load(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range ms {
			if vistos[m.ID] {
				t.Fatalf("id %s repeated across archives", m.ID)
			}
			vistos[m.ID] = true
		}
	}
	if len(vistos) != 28 {
		t.Fatalf("%d messages survive, want 28", len(vistos))
	}
}

// orq keeps 1.0.0: the next id is the number of messages + 1, even after a gap.
func TestAppendOrqIdComo100(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq")
	p := filepath.Join(t.TempDir(), "canal.jsonl")
	os.WriteFile(p, []byte(`{"id":"c1","de":"atena","para":"orquestador","tipo":"estado","texto":"a"}`+"\n"+`{"id":"c5","de":"atena","para":"orquestador","tipo":"estado","texto":"b"}`+"\n"), 0o644)
	m := msg("c")
	if err := Append(p, m); err != nil || m.ID != "c3" {
		t.Fatalf("orq id = %s (%v), want 1.0.0's c3", m.ID, err)
	}
	t.Setenv("ORQ_NOMBRE", "orq2")
	m = msg("d")
	if Append(p, m); m.ID != "c6" {
		t.Fatalf("orq2 id = %s, want c6 (after the highest)", m.ID)
	}
}

// M2: closing S-01 archives only S-01's delivered messages. Other stories' messages and every
// undelivered message stay in the live channel with their ids; ids keep growing.
func TestArchivarSoloLaHistoriaQueCierra(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq2")
	p := filepath.Join(t.TempDir(), "canal.jsonl")
	for _, a := range []string{"S-01/a1", "S-02/a1", "S-01/a1", ""} {
		m := msg("de " + a)
		m.Attempt = a
		if err := Append(p, m); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []string{"c1", "c2", "c4"} { // c3 (S-01) stays undelivered
		MarcarEntregado(p, id, t0)
	}
	arch, err := Archivar(p, "S-01", func(m Msg) bool { return m.Estado != "pendiente" && strings.HasPrefix(m.Attempt, "S-01/") })
	if err != nil {
		t.Fatal(err)
	}
	viejos, _ := Load(arch)
	if len(viejos) != 1 || viejos[0].ID != "c1" {
		t.Fatalf("archive must hold only S-01's delivered c1: %+v", viejos)
	}
	vivos, _ := Load(p)
	var ids []string
	for _, m := range vivos {
		ids = append(ids, m.ID+":"+m.Estado)
	}
	if strings.Join(ids, " ") != "c2:entregado c3:pendiente c4:entregado" {
		t.Fatalf("live channel after archiving S-01: %v", ids)
	}
	m := msg("nuevo")
	Append(p, m)
	if m.ID != "c5" {
		t.Fatalf("next id = %s, want c5", m.ID)
	}
	// The archive is its own file: messages sent later never show up in it.
	if viejos, _ := Load(arch); len(viejos) != 1 {
		t.Fatalf("the archive changed after archiving: %+v", viejos)
	}
}
