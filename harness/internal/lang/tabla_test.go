package lang

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"testing"
)

func TestTraduceLineaConValores(t *testing.T) {
	t.Setenv("ORQ_LANG", "")
	got := Traducir(fmt.Sprintf("orq: "+"%s ya tiene %d workers (tope %d)", "S-01", 3, 4) + "\n")
	if got != "orq: S-01 already has 3 workers (cap 4)\n" {
		t.Fatalf("%q", got)
	}
	if Traducir("texto libre\n") != "texto libre\n" {
		t.Fatal("tocó texto desconocido")
	}
}

func TestEspanolConORQ_LANG(t *testing.T) {
	t.Setenv("ORQ_LANG", "es")
	var b bytes.Buffer
	w := Writer(&b)
	fmt.Fprintf(w, "no hay slice abierto\n")
	if b.String() != "no hay slice abierto\n" {
		t.Fatalf("%q", b.String())
	}
}

func TestWriterTraducePorLineaYAlCerrar(t *testing.T) {
	t.Setenv("ORQ_LANG", "")
	var b bytes.Buffer
	w := Writer(&b)
	fmt.Fprint(w, "no hay slice ")
	fmt.Fprint(w, "abierto\nmemoria vacía")
	w.Flush()
	if b.String() != "no slice is open\nmemory is empty" {
		t.Fatalf("%q", b.String())
	}
}

var verbo = regexp.MustCompile(`%[-+# 0]*[0-9]*(\.[0-9]+)?[a-zA-Z]`)

func TestTablaCoherente(t *testing.T) {
	for _, p := range tabla {
		if strings.Count(p.es, "\n") != strings.Count(p.en, "\n") {
			t.Errorf("líneas distintas: %q", p.es)
		}
		if len(verbo.FindAllString(p.es, -1)) != len(verbo.FindAllString(p.en, -1)) {
			t.Errorf("verbos distintos: %q / %q", p.es, p.en)
		}
	}
}
