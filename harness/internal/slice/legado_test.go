package slice

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

// aLegado turns a slice opened by this version into what orq 1.0.0 left on disk: a state without
// a schema and no contract in git.
func aLegado(t *testing.T, r Repo, e *Estado) {
	t.Helper()
	if _, err := Git(r.Root, "update-ref", "-d", refContrato(e.NN)); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(r.estadoPath(e.NN))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	delete(m, "esquema")
	raw, _ = json.Marshal(m)
	if err := os.WriteFile(r.estadoPath(e.NN), raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

const msgOrq1 = "S-01 was opened by orq 1.0.0; close it with orq 1.0.0 before using this version"

// Acceptance 6: v2 does not inherit 1.0.0 slices. Verify, merge and close (accepted or rejected)
// refuse a schema-0 slice with one message, and leave its state and refs alone.
func TestSliceOrq1SeNiega(t *testing.T) {
	t.Parallel()
	r := repoDeJuguete(t)
	e := abrir(t, r)
	commitEn(t, e.Worktree, "total.txt", "170\n")
	aLegado(t, r, e)
	antes, _ := os.ReadFile(r.estadoPath("01"))
	casos := map[string]func(nn string) error{
		"verify":       func(nn string) error { _, _, err := r.Verificar(nn, t0); return err },
		"merge":        func(nn string) error { _, _, err := r.Juntar(nn); return err },
		"close accept": func(nn string) error { _, _, _, err := r.Cerrar(nn, true, "", t0.Add(time.Hour)); return err },
		"close reject": func(nn string) error { _, _, _, err := r.Cerrar(nn, false, "x", t0.Add(time.Hour)); return err },
	}
	for nombre, f := range casos {
		for _, nn := range []string{"", "01"} {
			err := f(nn)
			if !errors.Is(err, ErrOrq1) || !strings.Contains(err.Error(), msgOrq1) {
				t.Fatalf("%s (nn=%q): want %q, got %v", nombre, nn, msgOrq1, err)
			}
		}
	}
	if despues, _ := os.ReadFile(r.estadoPath("01")); string(antes) != string(despues) {
		t.Fatal("a refused 1.0.0 slice must keep its state untouched")
	}
	if hay, err := r.leerBlob(refContrato("01"), &Contrato{}); err != nil || hay {
		t.Fatalf("a refused 1.0.0 slice must not get a contract: %v %v", hay, err)
	}
}
