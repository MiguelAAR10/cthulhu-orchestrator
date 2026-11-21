package canal

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"
)

// H2: closing a story archives its channel; ids keep growing so "c3" names one message forever.
func TestArchivarGuardaElCanalYLosIdsSiguen(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "canal.jsonl")
	for i := 0; i < 3; i++ {
		if err := Append(p, msg(fmt.Sprint(i))); err != nil {
			t.Fatal(err)
		}
	}
	arch, err := Archivar(p, "S-01")
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
	arch2, err := Archivar(p, "S-01")
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
				a, err := Archivar(p, "S-01")
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
