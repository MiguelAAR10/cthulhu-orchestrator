package cli

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// orq2 status without max_workers says there is no head-count cap and which of RAM/CPU/load limits.
func TestStatusOrq2SinMaxWorkers(t *testing.T) {
	root := repoCLI(t)
	t.Setenv("ORQ_NOMBRE", "orq2")
	t.Setenv("ORQ_STATE", t.TempDir())
	lp := filepath.Join(t.TempDir(), "limits.json")
	os.WriteFile(lp, []byte(`{"min_free_gb": {"/": 1}, "scratch": {"paths": ["/tmp/x"], "quota_gb": 20, "critical_free_gb": 1}, "state_max_age_hours": 24}`), 0o644)
	t.Setenv("ORQ_LIMITS", lp)
	t.Chdir(root)
	out, errs, code := run(t, "status")
	if code != 0 {
		t.Fatalf("exit %d: %s%s", code, out, errs)
	}
	contiene(t, out, "max_workers=sin tope (deciden RAM, CPU y carga)", "max_workers sin tope")
	if !regexp.MustCompile(`capacidad: \d+ agentes a la vez \(limita (RAM|CPU|carga)\)`).MatchString(out) {
		t.Fatalf("status must name RAM, CPU or carga as the limit:\n%s", out)
	}
}
