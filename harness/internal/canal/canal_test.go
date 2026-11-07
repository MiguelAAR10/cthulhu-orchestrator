package canal

import (
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

var t0 = time.Date(2026, 10, 27, 10, 0, 0, 0, time.UTC)

func msg(texto string) *Msg {
	return &Msg{TS: t0, De: "orquestador", Para: "atena", Tipo: "estado", Texto: texto}
}

func TestAppendIdsUnicosConcurrentes(t *testing.T) {
	p := filepath.Join(t.TempDir(), "c.jsonl")
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) { defer wg.Done(); Append(p, msg(fmt.Sprint(i))) }(i)
	}
	wg.Wait()
	ms, _ := Load(p)
	vistos := map[string]bool{}
	for _, m := range ms {
		if vistos[m.ID] {
			t.Fatalf("id repetido %s", m.ID)
		}
		vistos[m.ID] = true
	}
	if len(ms) != 20 {
		t.Fatalf("%d mensajes", len(ms))
	}
}

func TestAppendDeduplicaPorClave(t *testing.T) {
	p := filepath.Join(t.TempDir(), "c.jsonl")
	a := msg("x")
	a.Clave = "k1"
	Append(p, a)
	b := msg("x otra vez")
	b.Clave = "k1"
	if err := Append(p, b); !errors.Is(err, ErrDuplicado) || b.ID != a.ID {
		t.Fatalf("%v %s", err, b.ID)
	}
	if ms, _ := Load(p); len(ms) != 1 {
		t.Fatal("guardó el duplicado")
	}
}

func TestEntregadoSePliegaYPendientes(t *testing.T) {
	p := filepath.Join(t.TempDir(), "c.jsonl")
	a, b := msg("a"), msg("b")
	Append(p, a)
	Append(p, b)
	MarcarEntregado(p, a.ID, t0)
	ms, _ := Load(p)
	if len(ms) != 2 || ms[0].Estado != "entregado" {
		t.Fatalf("%+v", ms)
	}
	if pe := Pendientes(ms); len(pe) != 1 || pe[0].ID != b.ID {
		t.Fatalf("%+v", pe)
	}
}
