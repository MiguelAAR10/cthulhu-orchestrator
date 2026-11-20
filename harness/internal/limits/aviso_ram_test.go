package limits

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// aviso_ram_gb is optional; absent means 5 GB.
func TestAvisoRAMPorDefecto(t *testing.T) {
	if (Limits{}).AvisoRAM() != 5 {
		t.Fatal("default RAM warning must be 5 GB")
	}
	tres := 3.0
	if (Limits{AvisoRAMGB: &tres}).AvisoRAM() != 3 {
		t.Fatal("aviso_ram_gb must set the warning")
	}
	p := filepath.Join(t.TempDir(), "limits.json")
	os.WriteFile(p, []byte(strings.Replace(sinMaxWorkers, "{", `{"aviso_ram_gb": 7,`, 1)), 0o644)
	t.Setenv("ORQ_NOMBRE", "orq2")
	if l, err := Load(p); err != nil || l.AvisoRAM() != 7 {
		t.Fatalf("aviso_ram_gb from file: %v %v", err, l.AvisoRAM())
	}
}

// orq2 ships its own limits template: no head-count cap, its own state dir, the RAM warning.
// orq's template keeps max_workers.
func TestPlantillaOrq2SinMaxWorkers(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ORQ_NOMBRE", "orq2")
	raw, err := os.ReadFile("../../../templates/limits-orq2.json")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "max_workers") || !strings.Contains(string(raw), "~/.local/state/orq2/") {
		t.Fatalf("orq2 template:\n%s", raw)
	}
	l, err := Load("../../../templates/limits-orq2.json")
	if err != nil || l.TopeWorkers() != SinTope || l.AvisoRAM() != 5 {
		t.Fatalf("orq2 template must load with no cap and the 5 GB warning: %v", err)
	}
	t.Setenv("ORQ_NOMBRE", "")
	if l, err := Load("../../../templates/limits.json"); err != nil || l.MaxWorkers == nil {
		t.Fatalf("orq template must keep max_workers: %v", err)
	}
}
