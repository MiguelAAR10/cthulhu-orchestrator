package plan

import (
	"strings"
	"testing"
)

func planFrentes() *Plan {
	return &Plan{
		Fronts: []Front{
			{ID: "F1", Goal: "El cliente paga", Paths: []string{"pagos"}, OwnerRole: "worker"},
			{ID: "F2", Goal: "El cliente ve su recibo", Paths: []string{"web"}, OwnerRole: "worker"},
		},
		Tareas: []Tarea{
			{ID: "a", Zona: "p", Rutas: []string{"pagos/a.go"}, Front: "F1", Estado: "hecha", Dificultad: "media"},
			{ID: "b", Zona: "p", Rutas: []string{"pagos/b.go"}, Front: "F1", Estado: "en_curso", Worker: "w1", Dificultad: "media"},
			{ID: "c", Zona: "w", Rutas: []string{"web/c.ts"}, Front: "F2", Estado: "pendiente", Dificultad: "media"},
		},
	}
}

func TestFrontsValidos(t *testing.T) {
	if err := ValidarFronts(planFrentes(), 0); err != nil {
		t.Fatal(err)
	}
	if err := ValidarFronts(&Plan{Tareas: planFrentes().Tareas[:1]}, 0); err == nil {
		// sin fronts, una tarea que nombra un front inexistente es error
		t.Fatal("front inexistente debe fallar")
	}
}

func TestFrontsSolapadosRechazados(t *testing.T) {
	p := planFrentes()
	p.Fronts[1].Paths = []string{"pagos/web"}
	err := ValidarFronts(p, 0)
	if err == nil || !strings.Contains(err.Error(), "solapan") {
		t.Fatalf("rutas solapadas: %v", err)
	}
}

func TestFrontsMaximo(t *testing.T) {
	p := planFrentes()
	p.Fronts = append(p.Fronts, Front{ID: "F3", Goal: "x", Paths: []string{"x"}}, Front{ID: "F4", Goal: "y", Paths: []string{"y"}})
	if err := ValidarFronts(p, 0); err == nil || !strings.Contains(err.Error(), "max_fronts") {
		t.Fatalf(">3 por defecto: %v", err)
	}
	if err := ValidarFronts(p, 4); err != nil {
		t.Fatalf("max 4 lo admite: %v", err)
	}
}

func TestFrontTareaFueraDeSusRutas(t *testing.T) {
	p := planFrentes()
	p.Tareas[2].Rutas = []string{"pagos/z.go"}
	if err := ValidarFronts(p, 0); err == nil {
		t.Fatal("tarea fuera de las rutas de su front")
	}
	p = planFrentes()
	p.Tareas[2].Front = ""
	if err := ValidarFronts(p, 0); err == nil {
		t.Fatal("con fronts, toda tarea nombra su front")
	}
}

func TestFrontResumen(t *testing.T) {
	r, err := planFrentes().Resumen("F1")
	if err != nil || r.Done != 1 || r.Total != 2 || len(r.Tareas) != 2 {
		t.Fatalf("resumen F1: %+v %v", r, err)
	}
	if _, err := planFrentes().Resumen("F9"); err == nil {
		t.Fatal("front inexistente")
	}
}
