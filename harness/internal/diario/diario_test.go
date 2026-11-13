package diario

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestAnotarYLeerEnOrdenYConcurrente(t *testing.T) {
	path := Ruta(filepath.Join(t.TempDir(), ".orq"))
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if err := Anotar(path, Evento{Tipo: "fase", Slice: "01", Intento: i}); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	evs, err := Leer(path)
	if err != nil || len(evs) != 20 {
		t.Fatalf("%d eventos, %v", len(evs), err)
	}
	// Una línea cortada no rompe la lectura de las demás.
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	f.WriteString(`{"tipo":"fase","sli`)
	f.Close()
	if err := Anotar(path, Evento{Tipo: "lanzamiento", Actor: "w", Paso: "intento"}); err != nil {
		t.Fatal(err)
	}
	evs, _ = Leer(path)
	u, ok := Ultimo(evs, func(e Evento) bool { return e.Tipo == "lanzamiento" })
	if !ok || u.Actor != "w" || u.TS.IsZero() {
		t.Fatalf("%+v %v", u, ok)
	}
	if a, b := NuevaOp(), NuevaOp(); a == b || len(a) != 12 {
		t.Fatalf("ops %s %s", a, b)
	}
	if evs, err := Leer(filepath.Join(t.TempDir(), "no.jsonl")); err != nil || evs != nil {
		t.Fatalf("un diario que no existe está vacío: %v %v", evs, err)
	}
}
