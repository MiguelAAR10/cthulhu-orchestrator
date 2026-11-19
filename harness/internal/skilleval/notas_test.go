package skilleval

import (
	"testing"
	"time"
)

func TestGuardarYNotas(t *testing.T) {
	dir := t.TempDir()
	if ns, err := Notas(dir); err != nil || len(ns) != 0 {
		t.Fatalf("sin archivo: %v %v", ns, err)
	}
	n := Nota{Skill: "a", Hash: "h1", Tipo: "go", Nota: 0.5, Corridas: 3, Fecha: time.Date(2026, 11, 19, 10, 0, 0, 0, time.UTC), Casos: []Caso{{"c", 0.5}}}
	if err := Guardar(dir, n); err != nil {
		t.Fatal(err)
	}
	n2 := n
	n2.Hash, n2.Nota = "h2", 0.8
	Guardar(dir, n2)
	ns, err := Notas(dir)
	if err != nil || len(ns) != 2 || ns[0].Hash != "h1" || ns[1].Nota != 0.8 || ns[0].Casos[0].Nombre != "c" {
		t.Fatalf("%+v %v", ns, err)
	}
}

func TestListarAgrupaPorSkillYHashMasNuevoPrimero(t *testing.T) {
	d := func(h int) time.Time { return time.Date(2026, 11, 19, h, 0, 0, 0, time.UTC) }
	ns := []Nota{
		{Skill: "b", Hash: "x", Tipo: "go", Fecha: d(1)},
		{Skill: "a", Hash: "viejo", Tipo: "go", Fecha: d(2)},
		{Skill: "a", Hash: "nuevo", Tipo: "go", Fecha: d(3)},
		{Skill: "a", Hash: "viejo", Tipo: "go", Fecha: d(4)},
		{Skill: "a", Hash: "otro", Tipo: "web", Fecha: d(5)},
	}
	var got []string
	for _, n := range Listar(ns, "go") {
		got = append(got, n.Skill+":"+n.Hash+":"+n.Fecha.Format("15"))
	}
	want := "a:viejo:04 a:viejo:02 a:nuevo:03 b:x:01"
	if s := join(got); s != want {
		t.Fatalf("%s\nwant %s", s, want)
	}
	if len(Listar(ns, "")) != 5 {
		t.Fatal("sin tipo se listan todas")
	}
}

func join(s []string) string {
	out := ""
	for i, x := range s {
		if i > 0 {
			out += " "
		}
		out += x
	}
	return out
}
