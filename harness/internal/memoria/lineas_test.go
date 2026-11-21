package memoria

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const memConUnaLineaRota = `{"id":"m1","imp":"critica","cat":"requisito","fuente":"owner","texto":"el total incluye envío"}
{"id":"m2","imp":"alta","cat":"deci
{"id":"m3","imp":"alta","cat":"decision","fuente":"atena","texto":"pago con tarjeta"}
`

// H13 (memory): one unreadable line is skipped and reported; the rest of the memory still counts.
func TestLoadSaltaLaLineaRotaYLaReporta(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq2")
	p := filepath.Join(t.TempDir(), "memoria.jsonl")
	os.WriteFile(p, []byte(memConUnaLineaRota), 0o644)
	items, err := Load(p)
	if len(items) != 2 || items[0].ID != "m1" || items[1].ID != "m3" {
		t.Fatalf("good lines must survive a bad one: %+v (%v)", items, err)
	}
	var malas *LineasMalas
	if !errors.As(err, &malas) || len(malas.Lineas) != 1 || malas.Lineas[0] != 2 || !strings.Contains(err.Error(), ":2") {
		t.Fatalf("the bad line must be reported by number: %v", err)
	}
	if _, err := Load(filepath.Join(t.TempDir(), "nada.jsonl")); err != nil {
		t.Fatalf("missing memory is empty, not an error: %v", err)
	}
}

// Adding to a memory with a bad line keeps that line on disk (nobody fixed it yet) and the new
// item never glues onto a torn last line.
func TestAddConservaLaLineaRota(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq2")
	p := filepath.Join(t.TempDir(), "memoria.jsonl")
	os.WriteFile(p, []byte(strings.TrimSuffix(memConUnaLineaRota, "\n")+"\n{\"id\":\"m4\",\"imp\":\"al"), 0o644)
	it, _, err := Add(p, Item{Imp: "media", Cat: "dominio", Fuente: "atena", Texto: "talla única"}, t0)
	if err != nil {
		t.Fatal(err)
	}
	if it.ID != "m4" {
		t.Fatalf("id = %s, want m4 (after the highest readable id)", it.ID)
	}
	raw, _ := os.ReadFile(p)
	if !strings.Contains(string(raw), `"cat":"deci`+"\n") || !strings.Contains(string(raw), `{"id":"m4","imp":"al`+"\n") {
		t.Fatalf("bad lines must stay on disk untouched:\n%s", raw)
	}
	items, err := Load(p)
	if len(items) != 3 || items[2].Texto != "talla única" {
		t.Fatalf("new item must be readable: %+v (%v)", items, err)
	}
}

// orq (not orq2) keeps 1.0.0: Add refuses a memory with an unreadable line and does not touch it.
func TestAddOrqRechazaComo100(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq")
	p := filepath.Join(t.TempDir(), "memoria.jsonl")
	os.WriteFile(p, []byte(memConUnaLineaRota), 0o644)
	if items, err := Load(p); items != nil || err == nil || err.Error() != p+":2: unexpected end of JSON input" {
		t.Fatalf("orq Load = %+v, %v; want nil and 1.0.0's first-line error", items, err)
	}
	_, _, err := Add(p, Item{Imp: "media", Cat: "dominio", Fuente: "atena", Texto: "talla única"}, t0)
	if err == nil || err.Error() != p+":2: unexpected end of JSON input" {
		t.Fatalf("orq Add = %v, want 1.0.0's first-line error", err)
	}
	if raw, _ := os.ReadFile(p); string(raw) != memConUnaLineaRota {
		t.Fatalf("file changed:\n%s", raw)
	}
}
