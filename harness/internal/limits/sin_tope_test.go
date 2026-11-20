package limits

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sinMaxWorkers = `{"min_free_gb": {"/": 10}, "scratch": {"paths": ["/tmp/x"], "quota_gb": 20, "critical_free_gb": 5}, "state_max_age_hours": 24}`

// orq2: max_workers is optional; absent means no head-count cap. orq still requires it.
func TestMaxWorkersOpcionalSoloOrq2(t *testing.T) {
	p := filepath.Join(t.TempDir(), "limits.json")
	if err := os.WriteFile(p, []byte(sinMaxWorkers), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ORQ_NOMBRE", "orq2")
	l, err := Load(p)
	if err != nil || l.MaxWorkers != nil || l.TopeWorkers() != SinTope {
		t.Fatalf("orq2 without max_workers: %v %v", err, l.MaxWorkers)
	}
	t.Setenv("ORQ_NOMBRE", "")
	if _, err := Load(p); err == nil || !strings.Contains(err.Error(), "falta max_workers") {
		t.Fatalf("orq must still require max_workers: %v", err)
	}
	cuatro := 4
	if (Limits{MaxWorkers: &cuatro}).TopeWorkers() != 4 {
		t.Fatal("present max_workers still caps")
	}
}

// F7: orq2 rejects a negative max_workers; orq keeps -1 as a real cap, not "sin tope".
func TestMaxWorkersNegativo(t *testing.T) {
	p := filepath.Join(t.TempDir(), "limits.json")
	conMenos := strings.Replace(sinMaxWorkers, `"state_max_age_hours"`, `"max_workers": -1, "state_max_age_hours"`, 1)
	if err := os.WriteFile(p, []byte(conMenos), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ORQ_NOMBRE", "orq2")
	if _, err := Load(p); err == nil || !strings.Contains(err.Error(), "max_workers") {
		t.Fatalf("orq2 must reject max_workers -1: %v", err)
	}
	t.Setenv("ORQ_NOMBRE", "")
	l, err := Load(p)
	if err != nil || l.TopeWorkers() == SinTope {
		t.Fatalf("orq: -1 must stay a cap, not SinTope: %v %d", err, l.TopeWorkers())
	}
}
