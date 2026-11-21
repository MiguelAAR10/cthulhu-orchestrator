package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"orq/internal/plan"
)

// H7: plan hecha, soltar and debatido must fail when the plan cannot be saved, never claim success.
func TestPlanSinGuardarFalla(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq2")
	root := repoCLI(t)
	t.Setenv("ORQ_STATE", t.TempDir())
	if _, errs, code := run(t, append(abrirBase, "--reporte", "out.tap", "--formato", "tap")...); code != 0 {
		t.Fatalf("abrir: %d %s", code, errs)
	}
	dir := filepath.Join(root, ".orq", "planes")
	pl := &plan.Plan{Slice: "01", Tareas: []plan.Tarea{{ID: "T1", Zona: "back", Rutas: []string{"total.txt"}, Estado: "en_curso", Worker: "w1"}}}
	if err := plan.Guardar(filepath.Join(dir, "01.json"), pl); err != nil {
		t.Fatal(err)
	}
	// The plan stays readable, but nothing new can be written next to it.
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })
	for _, args := range [][]string{{"plan", "hecha", "T1"}, {"plan", "soltar", "T1"}, {"plan", "debatido"}} {
		out, errs, code := run(t, args...)
		if code == 0 || !strings.Contains(errs, "plan: el plan no se guardó") {
			t.Errorf("%v with an unsaveable plan: code %d out %q err %q; want a failure", args, code, out, errs)
		}
	}
	if got, _ := plan.Leer(filepath.Join(dir, "01.json")); got.Tareas[0].Estado != "en_curso" || got.Debatido {
		t.Fatalf("plan changed on disk: %+v", got)
	}
}

// H2/H27: when the close cannot write memory, the slice is closed but the command says so and fails.
func TestCerrarSinMemoriaLoDice(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq2")
	sliceVerde(t)
	root := os.Getenv("ORQ_ROOT")
	t.Setenv("ORQ_SIN_DOC", "1")
	mem := filepath.Join(root, ".orq", "memoria.jsonl")
	os.Remove(mem)
	if err := os.Mkdir(mem, 0o755); err != nil { // a directory where the memory file should be: unreadable
		t.Fatal(err)
	}
	out, errs, code := run(t, "slice", "cerrar", "--aceptado")
	if code == 0 || !strings.Contains(errs, "memoria") || !strings.Contains(out, "cerrado como") {
		t.Fatalf("cerrar with unwritable memory: code %d out %q err %q", code, out, errs)
	}
}
