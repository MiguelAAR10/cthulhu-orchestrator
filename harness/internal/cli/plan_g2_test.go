package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// G2 review H1: under orq2 the first plan cargar on a project that never had a plan works.
func TestPlanCargarPrimeraVezOrq2(t *testing.T) {
	repoCLI(t)
	t.Setenv("ORQ_NOMBRE", "orq2")
	if _, errs, code := run(t, append(abrirBase, "--reporte", "out.tap", "--formato", "tap")...); code != 0 {
		t.Fatalf("abrir %d %s", code, errs)
	}
	root := os.Getenv("ORQ_ROOT")
	if _, err := os.Stat(filepath.Join(root, ".orq", "planes")); err == nil {
		t.Fatal("the test needs a project with no planes/ directory yet")
	}
	pj := filepath.Join(root, "plan.json")
	os.WriteFile(pj, []byte(`{"tareas":[{"id":"T1","zona":"a","rutas":["total.txt"],"dificultad":"baja","estado":"pendiente"}]}`), 0o644)
	out, errs, code := run(t, "plan", "cargar", pj)
	if code != 0 || !strings.Contains(out, "plan de 1 tareas guardado") {
		t.Fatalf("first cargar: code %d out %q err %q", code, out, errs)
	}
}
