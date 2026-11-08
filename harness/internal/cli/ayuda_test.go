package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// La ayuda de flags, las pistas de valores y la línea de cuarentena de doctor salen en inglés.
func TestAyudaDeFlagsEnIngles(t *testing.T) {
	root := proyectoVis(t)
	t.Setenv("ORQ_LANG", "")
	_, errs, _ := run(t, "msg", "send", "-h")
	for _, want := range []string{"--to string", "atena|orchestrator", "--type string", "story|question|answer|status|closing|plan", "role that writes", "idempotency key"} {
		if !strings.Contains(errs, want) {
			t.Errorf("falta %q en:\n%s", want, errs)
		}
	}
	_, errs, _ = run(t, "mem", "add", "-h")
	for _, want := range []string{"critical|high|medium|low", "requirement|decision|domain|technical|risk|learning"} {
		if !strings.Contains(errs, want) {
			t.Errorf("falta %q en:\n%s", want, errs)
		}
	}
	if m := marcasES.FindAllString(errs, -1); len(m) > 0 {
		t.Errorf("español: %v", m)
	}
	os.WriteFile(filepath.Join(root, ".orq", "cuarentena.jsonl"), []byte(`{"path":"/x"}`+"\n"), 0o644)
	out, _, _ := run(t, "doctor")
	if !strings.Contains(out, "quarantined: {") || strings.Contains(out, "en cuarentena") {
		t.Errorf("doctor en español:\n%s", out)
	}
	t.Setenv("ORQ_LANG", "es")
	if _, errs, _ = run(t, "msg", "send", "-h"); !strings.Contains(errs, "rol que escribe") {
		t.Errorf("ORQ_LANG=es: %s", errs)
	}
}
