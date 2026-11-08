package cli

import (
	"os"
	"strings"
	"testing"
)

// Older cli tests assert orq's original Spanish messages; they run with ORQ_LANG=es
// unless they set ORQ_LANG themselves. English (the default) is tested explicitly.
func TestMain(m *testing.M) {
	if os.Getenv("ORQ_LANG") == "" {
		os.Setenv("ORQ_LANG", "es")
	}
	os.Exit(m.Run())
}

func TestErroresEnInglesPorDefecto(t *testing.T) {
	t.Setenv("ORQ_ROOT", t.TempDir())
	t.Setenv("ORQ_LANG", "")
	_, errs, code := run(t, "plan", "show")
	if code == 0 || !strings.Contains(errs, "no slice is open") || strings.Contains(errs, "no hay slice") {
		t.Fatalf("%d %q", code, errs)
	}
	_, errs, _ = run(t, "mem", "add", "--imp", "x", "--cat", "c", "")
	if strings.Contains(errs, "texto vacío") {
		t.Fatalf("%q", errs)
	}
	t.Setenv("ORQ_LANG", "es")
	if _, errs, _ := run(t, "plan", "show"); !strings.Contains(errs, "no hay slice abierto") {
		t.Fatalf("%q", errs)
	}
}
