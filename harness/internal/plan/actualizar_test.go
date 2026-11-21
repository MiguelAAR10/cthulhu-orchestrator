package plan

import (
	"errors"
	"path/filepath"
	"sync"
	"testing"
)

// H5: writers that each take a different task at once all keep their change.
func TestActualizarConcurrenteNoPierdeCambios(t *testing.T) {
	path := filepath.Join(t.TempDir(), "planes", "01.json")
	p := &Plan{Slice: "01"}
	ids := []string{"A", "B", "C", "D", "E", "F"}
	for _, id := range ids {
		p.Tareas = append(p.Tareas, Tarea{ID: id, Zona: "z" + id, Rutas: []string{id}, Dificultad: "baja", Estado: "pendiente"})
	}
	if err := Guardar(path, p); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for _, id := range ids {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := Actualizar(path, func(f *Plan) error { _, err := Tomar(f, id, "w-"+id); return err }); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	got, err := Leer(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, tr := range got.Tareas {
		if tr.Estado != "en_curso" || tr.Worker != "w-"+tr.ID {
			t.Fatalf("lost update: %+v", got.Tareas)
		}
	}
}

// A refused change saves nothing, and the second taker of one task is refused.
func TestActualizarRechazoNoGuarda(t *testing.T) {
	path := filepath.Join(t.TempDir(), "01.json")
	if err := Guardar(path, ejemplo()); err != nil {
		t.Fatal(err)
	}
	if _, err := Actualizar(path, func(f *Plan) error { _, err := Tomar(f, "T0", "w1"); return err }); err != nil {
		t.Fatal(err)
	}
	if _, err := Actualizar(path, func(f *Plan) error { _, err := Tomar(f, "T0", "w2"); return err }); err == nil {
		t.Fatal("a task taken by w1 must refuse w2")
	}
	boom := errors.New("boom")
	if _, err := Actualizar(path, func(f *Plan) error { f.Debatido = true; return boom }); !errors.Is(err, boom) {
		t.Fatal(err)
	}
	got, _ := Leer(path)
	if got.Debatido || got.tarea("T0").Worker != "w1" {
		t.Fatalf("%+v", got)
	}
}
