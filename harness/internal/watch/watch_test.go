package watch

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"orq/internal/limits"
	"orq/internal/state"
)

func ptrF(v float64) *float64 { return &v }
func ptrI(v int) *int         { return &v }

func TestWatchMideYEscribeHalt(t *testing.T) {
	scratch := t.TempDir()
	if err := os.WriteFile(filepath.Join(scratch, "big"), make([]byte, 3_000_000), 0o644); err != nil {
		t.Fatal(err)
	}
	stateDir := t.TempDir()
	l := limits.Limits{
		MinFreeGB:        map[string]float64{"/": 1},
		MaxWorkers:       ptrI(1),
		Scratch:          limits.Scratch{Paths: []string{scratch, "/no/existe"}, QuotaGB: ptrF(0.001), CriticalFreeGB: ptrF(0)},
		StateMaxAgeHours: ptrF(24),
	}
	now := time.Now()
	r, err := Run(l, stateDir, now)
	if err != nil {
		t.Fatal(err)
	}
	if r.Snapshot.ScratchGB < 0.0029 || r.Snapshot.ScratchGB > 0.0031 {
		t.Fatalf("scratch mal medido: %f GB", r.Snapshot.ScratchGB)
	}
	if r.Halt == "" {
		t.Fatal("debía escribir HALT por cuota")
	}
	if _, ok := state.ReadHalt(stateDir); !ok {
		t.Fatal("HALT no quedó en disco")
	}
	snap, err := state.LoadSnapshot(stateDir)
	if err != nil || !snap.TS.Equal(now) {
		t.Fatalf("medición no guardada: %v %v", err, snap.TS)
	}
}

func TestWatchSinHaltBajoCuota(t *testing.T) {
	l := limits.Limits{
		MinFreeGB:        map[string]float64{"/": 1},
		MaxWorkers:       ptrI(1),
		Scratch:          limits.Scratch{Paths: []string{t.TempDir()}, QuotaGB: ptrF(100), CriticalFreeGB: ptrF(0)},
		StateMaxAgeHours: ptrF(24),
	}
	r, err := Run(l, t.TempDir(), time.Now())
	if err != nil || r.Halt != "" {
		t.Fatalf("no debía haber HALT: %v %q", err, r.Halt)
	}
}
