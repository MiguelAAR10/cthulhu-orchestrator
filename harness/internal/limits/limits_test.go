package limits

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const valid = `{
  "min_free_gb": {"/": 60, "/mnt/c": 40},
  "max_workers": 1,
  "scratch": {"paths": ["~/.local/state/orq/scratch"], "quota_gb": 20, "critical_free_gb": 30},
  "state_max_age_hours": 24,
  "protected": ["tests/aceptacion/", "SLICES.md"]
}`

func write(t *testing.T, body string) string {
	p := filepath.Join(t.TempDir(), "limits.json")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadValido(t *testing.T) {
	l, err := Load(write(t, valid))
	if err != nil {
		t.Fatal(err)
	}
	if *l.MaxWorkers != 1 || *l.Scratch.QuotaGB != 20 || len(l.WorkerProcessName) != 2 {
		t.Fatalf("carga incorrecta: %+v", l)
	}
	if strings.HasPrefix(l.Scratch.Paths[0], "~") {
		t.Fatalf("~ sin expandir: %s", l.Scratch.Paths[0])
	}
}

func TestCampoAusenteFalla(t *testing.T) {
	for _, drop := range []string{`"max_workers": 1,`, `"state_max_age_hours": 24,`, `"quota_gb": 20,`} {
		body := strings.Replace(valid, drop, "", 1)
		body = strings.Replace(body, `20, "critical_free_gb"`, `"critical_free_gb"`, 1) // limpia la coma sobrante del caso quota
		if _, err := Load(write(t, body)); err == nil {
			t.Fatalf("debía fallar sin %s", drop)
		}
	}
}

func TestArchivoAusenteFalla(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "no.json")); err == nil {
		t.Fatal("debía fallar sin archivo")
	}
}

func TestCampoDesconocidoFalla(t *testing.T) {
	body := strings.Replace(valid, `"max_workers": 1,`, `"max_workers": 1, "max_wokers": 3,`, 1)
	if _, err := Load(write(t, body)); err == nil {
		t.Fatal("debía rechazar un campo mal escrito")
	}
}

func TestTopeFronts(t *testing.T) {
	if (Limits{}).TopeFronts() != 3 {
		t.Fatal("defecto 3")
	}
	if (Limits{MaxFronts: 5}).TopeFronts() != 5 {
		t.Fatal("max_fronts manda")
	}
}
