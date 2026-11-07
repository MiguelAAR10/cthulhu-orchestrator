package plan

import (
	"path/filepath"
	"strings"
	"testing"
)

func ejemplo() *Plan {
	return &Plan{Slice: "01", Tareas: []Tarea{
		{ID: "T0", Zona: "back", Rutas: []string{"api/router.py"}, Dificultad: "baja", Estado: "pendiente"},
		{ID: "T1", Zona: "seed", Rutas: []string{"scripts/seed.py"}, Dificultad: "media", Estado: "pendiente"},
		{ID: "B4", Zona: "cards", Rutas: []string{"api/cards.py", "api/router.py"}, DependeDe: []string{"T0"}, Dificultad: "alta", Estado: "pendiente"},
		{ID: "B2", Zona: "home", Rutas: []string{"api/views"}, DependeDe: []string{"T0", "T1"}, Dificultad: "media", Estado: "pendiente"},
	}}
}

func TestValidar(t *testing.T) {
	p := ejemplo()
	if err := Validar(p, []string{"api", "scripts"}); err != nil {
		t.Fatal(err)
	}
	if err := Validar(p, []string{"api"}); err == nil || !strings.Contains(err.Error(), "fuera del alcance") {
		t.Fatalf("scripts/seed.py debía quedar fuera: %v", err)
	}
	p.Tareas[0].DependeDe = []string{"B4"} // T0 → B4 → T0
	err := Validar(p, nil)
	if err == nil || !strings.Contains(err.Error(), "ciclo") {
		t.Fatalf("ciclo no detectado: %v", err)
	}
	p = ejemplo()
	p.Tareas[1].DependeDe = []string{"ZZ"}
	p.Tareas[2].Rutas = nil
	err = Validar(p, nil)
	if err == nil || !strings.Contains(err.Error(), "no existe") || !strings.Contains(err.Error(), "sin rutas") {
		t.Fatalf("faltan errores: %v", err)
	}
}

func TestDespacho(t *testing.T) {
	p := ejemplo()
	listas, bloq := Listas(p)
	if len(listas) != 2 || bloq["B4"] == "" || bloq["B2"] == "" {
		t.Fatalf("al inicio solo T0 y T1: %+v %v", listas, bloq)
	}
	if _, err := Tomar(p, "B4", "w"); err == nil {
		t.Fatal("B4 espera a T0")
	}
	if _, err := Tomar(p, "T0", "worker-back-s01"); err != nil {
		t.Fatal(err)
	}
	if _, err := Tomar(p, "T0", "otro"); err == nil {
		t.Fatal("no se toma dos veces")
	}
	Tomar(p, "T1", "worker-seed-s01")
	if _, err := Hecha(p, "B2"); err == nil {
		t.Fatal("no se cierra lo que nadie tomó")
	}
	Hecha(p, "T0")
	// B4 comparte api/router.py con T0, pero T0 ya está hecha: lista. B2 espera a T1.
	listas, bloq = Listas(p)
	if len(listas) != 1 || listas[0].ID != "B4" || !strings.Contains(bloq["B2"], "T1") {
		t.Fatalf("%+v %v", listas, bloq)
	}
	Tomar(p, "B4", "worker-cards-s01")
	Hecha(p, "T1")
	// B2 toca api/views; B4 en curso toca api/cards.py y api/router.py: no se solapan.
	if listas, _ = Listas(p); len(listas) != 1 || listas[0].ID != "B2" {
		t.Fatalf("B2 debía estar lista: %+v", listas)
	}
	// Una tarea que toque api/ entera choca con B4 en curso.
	p.Tareas = append(p.Tareas, Tarea{ID: "X", Zona: "x", Rutas: []string{"api"}, Dificultad: "baja", Estado: "pendiente"})
	_, bloq = Listas(p)
	if !strings.Contains(bloq["X"], "B4") {
		t.Fatalf("X debía chocar con B4: %v", bloq)
	}
	if p.Pendientes() != 3 {
		t.Fatalf("pendientes %d", p.Pendientes())
	}
}

func TestGuardarLeerYModelo(t *testing.T) {
	path := Path(t.TempDir(), "01")
	p := ejemplo()
	p.Tareas[0].Estado, p.Tareas[0].Dificultad = "", ""
	if err := Guardar(path, p); err != nil {
		t.Fatal(err)
	}
	q, err := Leer(path)
	if err != nil || q.Tareas[0].Estado != "pendiente" || q.Tareas[0].Dificultad != "media" || filepath.Base(path) != "01.json" {
		t.Fatalf("%+v %v", q, err)
	}
	for _, c := range []struct{ ag, dif, mod, esf string }{
		{"claude", "baja", "haiku", "medium"}, {"claude", "", "sonnet", "medium"}, {"claude", "alta", "opus", "high"},
		{"codex", "baja", "gpt-6-luna", "medium"}, {"codex", "alta", "gpt-6.1-sol", "high"},
	} {
		if m, e := Modelo(c.ag, c.dif); m != c.mod || e != c.esf {
			t.Errorf("%s/%s: %s %s", c.ag, c.dif, m, e)
		}
	}
}

func TestRevisar(t *testing.T) {
	p := ejemplo()
	p.Tareas[0].Estado = "hecha"                                   // T0 api/router.py
	p.Tareas[1].Estado, p.Tareas[1].Worker = "hecha", "w-seed"     // T1 scripts/seed.py, sin cambios
	p.Tareas[2].Estado, p.Tareas[2].Worker = "en_curso", "w-cards" // B4
	d := Revisar(p, []string{"api/router.py", "infra/k8s.yaml", "api/cards.py"})
	if !d.Hay() || len(d.SinPlan) != 1 || d.SinPlan[0] != "infra/k8s.yaml" || len(d.HechaVacia) != 1 || d.HechaVacia[0] != "T1" || len(d.EnCurso) != 1 {
		t.Fatalf("%+v", d)
	}
	if d := Revisar(p, []string{"api/router.py", "scripts/seed.py"}); d.Hay() {
		t.Fatalf("sin desvío: %+v", d)
	}
}

// Recargar el plan no borra el avance: t1 hecha y t2 en curso siguen así; t3 nueva queda pendiente.
func TestConservarAvance(t *testing.T) {
	viejo := &Plan{Tareas: []Tarea{{ID: "t1", Estado: "hecha", Worker: "w"}, {ID: "t2", Estado: "en_curso", Worker: "w"}}}
	nuevo := &Plan{Tareas: []Tarea{{ID: "t1", Estado: "pendiente"}, {ID: "t2", Estado: "pendiente"}, {ID: "t3", Estado: "pendiente"}}}
	if k := Conservar(nuevo, viejo); len(k) != 2 {
		t.Fatalf("conservó %v", k)
	}
	if nuevo.Tareas[0].Estado != "hecha" || nuevo.Tareas[1].Estado != "en_curso" || nuevo.Tareas[1].Worker != "w" || nuevo.Tareas[2].Estado != "pendiente" {
		t.Fatalf("%+v", nuevo.Tareas)
	}
}

// Un arranque fallido suelta la tarea: vuelve a pendiente y sin worker; solo desde en curso.
func TestSoltar(t *testing.T) {
	p := &Plan{Tareas: []Tarea{{ID: "t1", Estado: "en_curso", Worker: "w"}, {ID: "t2", Estado: "hecha"}}}
	if _, err := Soltar(p, "t1"); err != nil || p.Tareas[0].Estado != "pendiente" || p.Tareas[0].Worker != "" {
		t.Fatalf("%+v %v", p.Tareas[0], err)
	}
	if _, err := Soltar(p, "t2"); err == nil {
		t.Fatal("una tarea hecha no se suelta")
	}
}
