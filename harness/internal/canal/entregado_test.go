package canal

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"

	"orq/internal/orqdir"
)

// MarcarEntregado waits for the channel lock: a delivery mark written while the channel is being
// archived lands in the new live channel, not in the file that is being replaced (G1 review N3).
func TestMarcarEntregadoEsperaElCandadoDelCanal(t *testing.T) {
	p := filepath.Join(t.TempDir(), "c.jsonl")
	m := msg("hola")
	if err := Append(p, m); err != nil {
		t.Fatal(err)
	}
	f, err := abrirBloqueado(p)
	if err != nil {
		t.Fatal(err)
	}
	hecho := make(chan error, 1)
	go func() { hecho <- MarcarEntregado(p, m.ID, t0) }()
	select {
	case err := <-hecho:
		t.Fatalf("MarcarEntregado wrote while the channel lock was held (err %v)", err)
	case <-time.After(150 * time.Millisecond):
	}
	// What Archivar does under the lock: the live channel is swapped for a new file.
	line, _ := json.Marshal(m)
	if err := orqdir.WriteAtomic(p, append(line, '\n')); err != nil {
		t.Fatal(err)
	}
	syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	f.Close()
	if err := <-hecho; err != nil {
		t.Fatal(err)
	}
	ms, err := Load(p)
	if err != nil || len(ms) != 1 || ms[0].Estado != "entregado" {
		t.Fatalf("the delivery mark was lost in the swap: %+v %v", ms, err)
	}
}

// Twenty writers at once, each adding its own session: none is lost.
func TestActualizarSesionesConcurrentesNoPierdeNinguna(t *testing.T) {
	p := filepath.Join(t.TempDir(), "sesiones.json")
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := ActualizarSesiones(p, func(m map[string]Sesion) { m[fmt.Sprintf("worker-%d-s01", i)] = Sesion{Gen: 1} }); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	s, err := LoadSesiones(p)
	if err != nil || len(s) != 20 {
		t.Fatalf("%d sessions, want 20 (%v)", len(s), err)
	}
}
