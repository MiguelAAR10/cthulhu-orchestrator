package skilleval

import (
	"strings"
	"testing"
	"time"
)

func notasComparar() []Nota {
	d := func(h int) time.Time { return time.Date(2026, 11, 19, h, 0, 0, 0, time.UTC) }
	return []Nota{
		{Skill: "a", Hash: "aaaa1111", Tipo: "go", Nota: 0.6, Fecha: d(1)},
		{Skill: "a", Hash: "bbbb2222", Tipo: "go", Nota: 0.8, Fecha: d(2)},
		{Skill: "a", Hash: "bbbb3333", Tipo: "go", Nota: 0.65, Fecha: d(3)},
		{Skill: "a", Hash: "cccc4444", Tipo: "go", Nota: 0.3, Fecha: d(4)},
		{Skill: "a", Hash: "bbbb2222", Tipo: "go", Nota: 0.99, Parcial: true, Fecha: d(5)},
		{Skill: "a", Hash: "dddd5555", Tipo: "web", Nota: 0.9, Fecha: d(6)},
	}
}

func TestCompararConMargen(t *testing.T) {
	ns := notasComparar()
	casos := []struct {
		viejo, nuevo string
		margen       float64
		conservar    bool
	}{
		{"aaaa1111", "bbbb2222", 0.1, true},  // +0.20; la parcial de 0.99 no cuenta
		{"aaaa1111", "bbbb3333", 0.1, false}, // +0.05
		{"aaaa1111", "cccc4444", 0.1, false}, // −0.30
		{"aaaa1111", "bbbb3333", 0.01, true},
		{"aaaa", "cccc", 0.1, false}, // prefijos
	}
	for _, c := range casos {
		v, err := Comparar(ns, c.viejo, c.nuevo, "", c.margen)
		if err != nil || v.Conservar != c.conservar {
			t.Errorf("%s→%s margen %.2f: %+v %v", c.viejo, c.nuevo, c.margen, v, err)
		}
	}
	if v, _ := Comparar(ns, "aaaa1111", "bbbb2222", "", 0.1); v.Nuevo.Parcial || v.Delta < 0.199 || v.Delta > 0.201 {
		t.Fatalf("delta: %+v", v)
	}
	if _, err := Comparar(ns, "aaaa", "bbbb", "", 0.1); err == nil || !strings.Contains(err.Error(), "ambiguo") {
		t.Fatalf("prefijo ambiguo: %v", err)
	}
	if _, err := Comparar(ns, "aaaa", "zzzz", "", 0.1); err == nil || !strings.Contains(err.Error(), "zzzz") {
		t.Fatalf("sin nota: %v", err)
	}
	if _, err := Comparar(ns, "aaaa", "dddd", "", 0.1); err == nil || !strings.Contains(err.Error(), "--tipo") {
		t.Fatalf("tipos distintos: %v", err)
	}
	if _, err := Comparar(ns, "aaaa", "dddd", "go", 0.1); err == nil {
		t.Fatal("dddd no tiene nota de tipo go")
	}
	solo := []Nota{{Skill: "a", Hash: "eeee", Tipo: "go", Nota: 0.9, Parcial: true}}
	if _, err := Comparar(append(solo, ns[0]), "aaaa", "eeee", "", 0.1); err == nil || !strings.Contains(err.Error(), "parcial") {
		t.Fatalf("solo parcial: %v", err)
	}
}
