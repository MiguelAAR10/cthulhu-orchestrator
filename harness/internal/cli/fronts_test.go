package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"orq/internal/plan"
	"orq/internal/workers"
)

func proyectoFronts(t *testing.T) string {
	root := proyectoVis(t)
	orq := filepath.Join(root, ".orq")
	plan.Guardar(plan.Path(orq, "04"), &plan.Plan{Slice: "04",
		Fronts: []plan.Front{{ID: "F1", Goal: "The customer pays", Paths: []string{"a"}, OwnerRole: "worker"}, {ID: "F2", Goal: "The card is charged", Paths: []string{"b"}}},
		Tareas: []plan.Tarea{
			{ID: "T1", Zona: "a", Rutas: []string{"a/"}, Que: "checkout page", Front: "F1", Estado: "hecha"},
			{ID: "T2", Zona: "b", Rutas: []string{"b/"}, Que: "charge the card", Front: "F2"},
			{ID: "T3", Zona: "a", Rutas: []string{"a/x"}, Que: "pay button", Front: "F1", Estado: "en_curso", Worker: "w-s04-t3"},
		}})
	workers.Save(orq, workers.State{ID: "w-s04-t3", Slice: "S-04", Task: "T3", Tests: &workers.Tests{Passing: 4, Total: 5}})
	return root
}

func TestFrontBrief(t *testing.T) {
	proyectoFronts(t)
	out, errs, code := run(t, "front", "brief", "F1", "--slice", "04")
	for _, w := range []string{"F1", "The customer pays", "a", "Customers pay by card", "T1", "T3", "pay button"} {
		if code != 0 || !strings.Contains(out, w) {
			t.Fatalf("falta %q: %d %s %s", w, code, out, errs)
		}
	}
	if strings.Contains(out, "T2") {
		t.Fatal("el brief de F1 no lleva tareas de F2", out)
	}
}

func TestFrontReceiptAggregates(t *testing.T) {
	proyectoFronts(t)
	if _, e, c := run(t, "stop", "raise", "--type", "decision", "--options", "blue|green", "--severity", "soon", "--slice", "S-04", "--tasks", "T3", "--text", "Which button color do you want for paying?"); c != 0 {
		t.Fatal(e)
	}
	out, errs, code := run(t, "front", "receipt", "F1", "--slice", "04")
	for _, w := range []string{"1/2 tasks done", "tests 4/5", "stoppers: 1", "findings: 0"} {
		if code != 0 || !strings.Contains(out, w) {
			t.Fatalf("falta %q: %d %s %s", w, code, out, errs)
		}
	}
	if _, _, code := run(t, "front", "receipt", "F9", "--slice", "04"); code == 0 {
		t.Fatal("front inexistente")
	}
}

func TestPlanCargarRechazaFrontsSolapados(t *testing.T) {
	root := proyectoVis(t)
	f := filepath.Join(root, "p.json")
	os.WriteFile(f, []byte(`{"fronts":[{"id":"F1","goal":"x","paths":["a"]},{"id":"F2","goal":"y","paths":["a/b"]}],
	"tareas":[{"id":"T1","zona":"a","rutas":["a/b"],"front":"F2"}]}`), 0o644)
	if _, errs, code := run(t, "plan", "--slice", "04", "cargar", f); code == 0 || !strings.Contains(errs, "solapan") {
		t.Fatalf("%d %s", code, errs)
	}
}
