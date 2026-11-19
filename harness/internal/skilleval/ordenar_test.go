package skilleval

import (
	"testing"
	"time"
)

func TestOrdenar(t *testing.T) {
	d := func(h int) time.Time { return time.Date(2026, 11, 19, h, 0, 0, 0, time.UTC) }
	ns := []Nota{
		{Skill: "a", Hash: "1", Tipo: "go", Nota: 0.9, Fecha: d(1)},
		{Skill: "a", Hash: "2", Tipo: "go", Nota: 0.4, Fecha: d(2)}, // la última de a
		{Skill: "b", Hash: "1", Tipo: "go", Nota: 0.7, Fecha: d(1)},
		{Skill: "b", Hash: "2", Tipo: "go", Nota: 0.1, Parcial: true, Fecha: d(3)},
		{Skill: "c", Hash: "1", Tipo: "web", Nota: 0.99, Fecha: d(1)},
		{Skill: "e", Hash: "1", Tipo: "go", Nota: 0.7, Fecha: d(1)},
	}
	nombres := []string{"d", "c", "a", "b", "e"}
	hs := map[string]string{"a": "2", "b": "1", "c": "1", "d": "1", "e": "1"}
	el, fuera := Ordenar(nombres, ns, hs, "go", nil)
	if got := texto(el); got != "b:0.70 e:0.70 a:0.40 d:- c:-" || len(fuera) != 0 {
		t.Fatalf("sin mínimo: %s fuera=%s", got, texto(fuera))
	}
	min := 0.5
	el, fuera = Ordenar(nombres, ns, hs, "go", &min)
	if got := texto(el); got != "b:0.70 e:0.70 d:- c:-" || texto(fuera) != "a:0.40" {
		t.Fatalf("con mínimo: %s fuera=%s", got, texto(fuera))
	}
}

func texto(es []Eleccion) string {
	var s []string
	for _, e := range es {
		s = append(s, e.String())
	}
	return join(s)
}

func TestOrdenarSoloCuentaElHashInstalado(t *testing.T) {
	ns := []Nota{{Skill: "a", Hash: "v1", Tipo: "go", Nota: 0.9}, {Skill: "a", Hash: "v2", Tipo: "go", Nota: 0.1}}
	el, _ := Ordenar([]string{"a"}, ns, map[string]string{"a": "v1"}, "go", nil)
	if texto(el) != "a:0.90" {
		t.Fatalf("v1 instalada: %s", texto(el))
	}
	el, _ = Ordenar([]string{"a"}, ns, map[string]string{"a": "v3"}, "go", nil)
	if texto(el) != "a:-" {
		t.Fatalf("v3 sin evaluar: %s", texto(el))
	}
}
