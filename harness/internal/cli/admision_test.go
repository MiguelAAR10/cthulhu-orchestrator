package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"orq/internal/herdr"
	"orq/internal/state"
	"orq/internal/worktrees"
)

// lanzarAislado prepara un proyecto, límites y estado de juguete; devuelve el dir de estado,
// el .orq del proyecto y un puntero a las llamadas hechas a herdr.
func lanzarAislado(t *testing.T, maxWT int) (string, string, *[][]string) {
	t.Helper()
	root := t.TempDir()
	t.Setenv("ORQ_ROOT", root)
	t.Setenv("ORQ_ROL", "orquestador")
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "w1:p1")
	st := t.TempDir()
	t.Setenv("ORQ_STATE", st)
	lim := filepath.Join(t.TempDir(), "limits.json")
	os.WriteFile(lim, []byte(`{"min_free_gb":{"/":0},"max_workers":4,"max_worktrees":`+itoa(maxWT)+`,
		"scratch":{"paths":["/nada"],"quota_gb":10,"critical_free_gb":0},"state_max_age_hours":24}`), 0o644)
	t.Setenv("ORQ_LIMITS", lim)
	state.SaveSnapshot(st, state.Snapshot{TS: time.Now(), ScratchGB: 1})
	var llamadas [][]string
	orig := herdr.Run
	herdr.Run = func(a ...string) ([]byte, error) { llamadas = append(llamadas, a); return []byte(`{}`), nil }
	t.Cleanup(func() { herdr.Run = orig })
	origW := contarWorkers
	contarWorkers = func([]string) (int, error) { return 0, nil }
	t.Cleanup(func() { contarWorkers = origW })
	orq := filepath.Join(root, ".orq")
	os.MkdirAll(orq, 0o755)
	return st, orq, &llamadas
}

func itoa(n int) string { b, _ := json.Marshal(n); return string(b) }

func TestLanzarPasaPorLaAdmision(t *testing.T) {
	casos := map[string]func(t *testing.T, st, orq string){
		"HALT": func(t *testing.T, st, _ string) { state.WriteHalt(st, "disco crítico", time.Now()) },
		"measurement": func(t *testing.T, st, _ string) {
			state.SaveSnapshot(st, state.Snapshot{TS: time.Now().Add(-48 * time.Hour)})
		},
		"quota": func(t *testing.T, st, _ string) {
			state.SaveSnapshot(st, state.Snapshot{TS: time.Now(), ScratchGB: 99})
		},
		"max_worktrees": func(t *testing.T, _, orq string) {
			f, _ := os.Create(filepath.Join(orq, "worktrees.jsonl"))
			for i := 0; i < 2; i++ {
				d := t.TempDir()
				line, _ := json.Marshal(worktrees.Entrada{Path: d, Kind: worktrees.KindSlice})
				f.Write(append(line, '\n'))
			}
			f.Close()
		},
	}
	for want, prep := range casos {
		t.Run(want, func(t *testing.T) {
			st, orq, llamadas := lanzarAislado(t, 2)
			prep(t, st, orq)
			_, errs, code := run(t, "slice", "lanzar", "--agente", "claude")
			if code == 0 || !strings.Contains(errs, want) {
				t.Fatalf("code=%d errs=%q", code, errs)
			}
			if len(*llamadas) != 0 {
				t.Fatalf("llamó a herdr pese al rechazo: %v", *llamadas)
			}
			if raw, _ := os.ReadFile(filepath.Join(st, "reservas.json")); strings.Contains(string(raw), `"id"`) {
				t.Fatalf("la reserva no se liberó: %s", raw)
			}
		})
	}
}

func TestLanzarSinSliceLiberaLaReserva(t *testing.T) {
	st, _, llamadas := lanzarAislado(t, 12)
	_, errs, code := run(t, "slice", "lanzar", "--agente", "claude")
	if code == 0 || len(*llamadas) != 0 {
		t.Fatalf("code=%d errs=%q llamadas=%v", code, errs, *llamadas)
	}
	if raw, _ := os.ReadFile(filepath.Join(st, "reservas.json")); strings.Contains(string(raw), `"id"`) {
		t.Fatalf("la reserva quedó tomada tras fallar: %s", raw)
	}
}
