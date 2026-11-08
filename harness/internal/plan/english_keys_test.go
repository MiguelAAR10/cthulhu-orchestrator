package plan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLeerAceptaClavesEspanolas(t *testing.T) {
	path := filepath.Join(t.TempDir(), "planes", "01.json")
	os.MkdirAll(filepath.Dir(path), 0o755)
	os.WriteFile(path, []byte(`{"slice":"S-01","tareas":[{"id":"a","zona":"back","rutas":["x"],"depende_de":["b"],"dificultad":"alta","que":"hace x","estado":"hecha"},{"id":"b","zona":"front","rutas":["y"]}]}`), 0o644)
	p, err := Leer(path)
	if err != nil {
		t.Fatal(err)
	}
	a := p.Tareas[0]
	if len(p.Tareas) != 2 || a.Zona != "back" || a.Rutas[0] != "x" || a.DependeDe[0] != "b" || a.Dificultad != "alta" || a.Que != "hace x" || a.Estado != "pendiente" || !a.HechaSinFirma {
		t.Fatalf("%+v", p)
	}
}

func TestLeerAceptaClavesInglesasYGuardaEnIngles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "planes", "01.json")
	os.MkdirAll(filepath.Dir(path), 0o755)
	os.WriteFile(path, []byte(`{"slice":"S-01","tasks":[{"id":"a","zone":"back","paths":["x"],"depends_on":["b"],"difficulty":"high","what":"does x","status":"done"},{"id":"b","zone":"front","paths":["y"],"status":"in_progress","difficulty":"low"}]}`), 0o644)
	p, err := Leer(path)
	if err != nil {
		t.Fatal(err)
	}
	a, b := p.Tareas[0], p.Tareas[1]
	if a.Zona != "back" || a.Rutas[0] != "x" || a.DependeDe[0] != "b" || a.Dificultad != "alta" || a.Que != "does x" || a.Estado != "pendiente" || !a.HechaSinFirma || b.Estado != "en_curso" || b.Dificultad != "baja" {
		t.Fatalf("%+v", p)
	}
	if err := Guardar(path, p); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	s := string(raw)
	for _, k := range []string{`"tasks"`, `"zone"`, `"paths"`, `"depends_on"`, `"difficulty"`, `"what"`, `"status"`} {
		if !strings.Contains(s, k) {
			t.Fatalf("falta %s en %s", k, s)
		}
	}
	for _, k := range []string{`"tareas"`, `"zona"`, `"rutas"`, `"estado"`} {
		if strings.Contains(s, k) {
			t.Fatalf("sobra %s en %s", k, s)
		}
	}
	p2, err := Leer(path)
	if err != nil || p2.Revision() != p.Revision() || p2.Tareas[0].Estado != "pendiente" {
		t.Fatalf("ida y vuelta: %v", err)
	}
}
