package plan

import (
	"testing"
	"time"

	"orq/internal/stops"
)

// Spec stoppers-autonomy, acceptance 1: a blocking stopper holds only its tasks.
func TestStopperBlockingHoldsOnlyItsTasks(t *testing.T) {
	orq := t.TempDir()
	path := Path(orq, "04")
	p := &Plan{Slice: "04", Tareas: []Tarea{
		{ID: "T1", Zona: "a", Rutas: []string{"a/"}},
		{ID: "T2", Zona: "b", Rutas: []string{"b/"}},
	}}
	if err := Guardar(path, p); err != nil {
		t.Fatal(err)
	}
	if _, _, err := stops.Add(stops.Path(orq), stops.Raise{Type: "api_key", Severity: "blocking", Slice: "S-04", Tasks: []string{"T2"}, By: "w", Text: "need key"}, time.Now()); err != nil {
		t.Fatal(err)
	}
	p, _ = Leer(path)
	listas, bloq := Listas(p)
	if len(listas) != 1 || listas[0].ID != "T1" || bloq["T2"] != "stop ST-0001" {
		t.Fatalf("listas %v bloq %v", listas, bloq)
	}
	if _, err := Tomar(p, "T2", "w"); err == nil {
		t.Fatal("held task taken")
	}
}
