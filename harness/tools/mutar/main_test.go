package main

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

const fuente = `package p

func f(a, b int, c bool) bool { return a == b && !c || true }

func g(a, b int) bool { return a < b }
`

func TestSitios(t *testing.T) {
	ms, err := sitios("p.go", []byte(fuente), "f")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, m := range ms {
		if fuente[m.Off:m.Off+len(m.Old)] != m.Old || m.Line != 3 {
			t.Errorf("site %+v does not point at %q on line 3", m, m.Old)
		}
		got = append(got, m.Old+"→"+m.New)
	}
	want := "==→!=, &&→||, !→, ||→&&, true→false"
	if strings.Join(got, ", ") != want {
		t.Errorf("sites %q, want %q", strings.Join(got, ", "), want)
	}
	if _, err := sitios("p.go", []byte(fuente), "h"); err == nil {
		t.Error("an unknown function is not an error")
	}
}

func TestAplicar(t *testing.T) {
	ms, _ := sitios("p.go", []byte(fuente), "g")
	if len(ms) != 2 {
		t.Fatalf("g has %d sites, want 2 (< → >= and < → <=)", len(ms))
	}
	out := aplicar([]byte(fuente), ms[0])
	if _, err := parser.ParseFile(token.NewFileSet(), "p.go", out, 0); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "return a >= b") {
		t.Errorf("mutated source:\n%s", out)
	}
}
