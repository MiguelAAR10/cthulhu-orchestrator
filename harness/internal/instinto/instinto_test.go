package instinto

import (
	"path/filepath"
	"testing"
	"time"
)

func TestAprenderRepetirConfirmarRefutar(t *testing.T) {
	p := filepath.Join(t.TempDir(), "i.json")
	now := time.Now()
	a, err := Aprender(p, "Correr migraciones antes del examen", "reintento", "S-01/a2", now)
	if err != nil || a.ID != "i1" || a.Conf != Inicial {
		t.Fatalf("nuevo: %+v %v", a, err)
	}
	if len(Top(6, p)) != 0 {
		t.Fatal("una hipótesis no se inyecta")
	}
	b, _ := Aprender(p, "  correr migraciones   antes del examen", "rechazo", "", now)
	if b.ID != "i1" || b.Visto != 2 || b.Conf <= Inicial {
		t.Fatalf("repetida debe subir, no duplicar: %+v", b)
	}
	c, vive, _ := Ajustar(p, "i1", Confirmar)
	if !vive || c.Conf < Umbral || len(Top(6, p)) != 1 {
		t.Fatalf("confirmada debe inyectarse: %+v", c)
	}
	for i := 0; i < 5; i++ {
		Ajustar(p, "i1", Confirmar)
	}
	if l, _ := Leer(p); l[0].Conf > Techo {
		t.Fatalf("techo: %v", l[0].Conf)
	}
	Ajustar(p, "i1", -Refutar)
	Ajustar(p, "i1", -Refutar)
	if _, vive, _ := Ajustar(p, "i1", -Refutar); vive {
		t.Fatal("bajo el piso se poda")
	}
	if l, _ := Leer(p); len(l) != 0 {
		t.Fatal("debe quedar vacío")
	}
	if _, err := Aprender(p, string(make([]rune, MaxTexto+1)), "x", "", now); err == nil {
		t.Fatal("texto largo")
	}
}

func TestTopYPromover(t *testing.T) {
	d := t.TempDir()
	loc, glo := filepath.Join(d, "l.json"), filepath.Join(d, "g.json")
	now := time.Now()
	for i := 0; i < 8; i++ {
		x, _ := Aprender(loc, "lección "+string(rune('a'+i)), "orquestador", "", now.Add(time.Duration(i)*time.Second))
		Ajustar(loc, x.ID, 2*Confirmar+float64(i)/100)
	}
	top := Top(Inyectar, loc, glo)
	if len(top) != Inyectar || top[0].Texto != "lección h" {
		t.Fatalf("top: %+v", top)
	}
	g, err := Promover(loc, glo, "i8", now)
	if err != nil || g.Conf < Umbral {
		t.Fatalf("promover: %+v %v", g, err)
	}
	if len(Top(Inyectar, loc, glo)) != Inyectar {
		t.Fatal("sin duplicados entre archivos")
	}
}
