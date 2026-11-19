package skilleval

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ResultadoDePrueba imita aggregate-result.json: dos casos (1.0 y 0.5), 3 + 3 corridas.
const resultadoDePrueba = `{"schemaVersion":1,"claudeVersion":"2.1.328","costUsd":0.41,"partial":false,
"aggregates":{"overallScore":0.75,"casesPassed":1,"casesTotal":2},
"cases":[{"name":"c1","aggregates":{"score":1.0},"arms":{"with":[{},{},{}]}},
{"name":"c2","aggregates":{"score":0.5},"arms":{"with":[{},{},{"error":"timeout"}]}}]}`

// stubClaude pone en PATH un `claude` de mentira con el cuerpo dado; nunca corre el real.
func stubClaude(t *testing.T, cuerpo string) {
	t.Helper()
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte("#!/bin/sh\n"+cuerpo), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":/usr/bin:/bin")
}

// stubEvalJSON comprueba los argumentos y la forma del plugin, escribe el JSON y sale con salida.
func stubEvalJSON(json, salida string) string {
	return `[ "$1 $2" = "plugin eval" ] || { echo "args: $*" >&2; exit 9; }
[ -f "$3/.claude-plugin/plugin.json" ] && [ -d "$3/evals" ] || { echo "no es un plugin: $3" >&2; exit 9; }
out="$5"; shift 3
[ "$*" = "--json $out --ablation none --max-cost-usd 2.5 --trust-plugin" ] || { echo "flags: $*" >&2; exit 9; }
cat > "$out" <<'JSON'
` + json + `
JSON
exit ` + salida + "\n"
}

func plugin(t *testing.T) string {
	t.Helper()
	skill := filepath.Join(t.TempDir(), "mi-skill")
	escribir(t, skill, map[string]string{"SKILL.md": "x", "evals/c1/prompt.md": "y"})
	dst := t.TempDir()
	if _, err := Envolver(skill, dst); err != nil {
		t.Fatal(err)
	}
	return dst
}

func TestCorrerPasaLosArgumentosYLee(t *testing.T) {
	stubClaude(t, stubEvalJSON(resultadoDePrueba, "0"))
	r, err := Correr(context.Background(), plugin(t), filepath.Join(t.TempDir(), "r.json"), 2.5)
	if err != nil {
		t.Fatal(err)
	}
	if r.Nota != 0.75 || r.Corridas != 6 || r.CostoUSD != 0.41 || r.Claude != "2.1.328" || r.Parcial {
		t.Fatalf("%+v", r)
	}
	if len(r.Casos) != 2 || r.Casos[0] != (Caso{"c1", 1}) || r.Casos[1] != (Caso{"c2", 0.5}) {
		t.Fatalf("casos: %+v", r.Casos)
	}
}

func TestCorrerSalida1EsResultado(t *testing.T) {
	stubClaude(t, stubEvalJSON(resultadoDePrueba, "1"))
	if r, err := Correr(context.Background(), plugin(t), filepath.Join(t.TempDir(), "r.json"), 2.5); err != nil || r.Nota != 0.75 {
		t.Fatalf("un caso bajo el umbral sigue siendo una nota: %+v %v", r, err)
	}
}

func TestCorrerTopeDeCostoEsParcial(t *testing.T) {
	stubClaude(t, stubEvalJSON(resultadoDePrueba, "2"))
	if r, err := Correr(context.Background(), plugin(t), filepath.Join(t.TempDir(), "r.json"), 2.5); err != nil || !r.Parcial {
		t.Fatalf("tope de costo: %+v %v", r, err)
	}
}

func TestCorrerFallaSinJSON(t *testing.T) {
	stubClaude(t, "echo boom >&2; exit 1\n")
	_, err := Correr(context.Background(), plugin(t), filepath.Join(t.TempDir(), "r.json"), 2.5)
	if err == nil || errors.Is(err, ErrNoDisponible) || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("%v", err)
	}
}

func TestCorrerNoDisponible(t *testing.T) {
	stubClaude(t, "echo 'Error: plugin eval is currently in early access' >&2; exit 1\n")
	_, err := Correr(context.Background(), plugin(t), filepath.Join(t.TempDir(), "r.json"), 2.5)
	if !errors.Is(err, ErrNoDisponible) || !strings.Contains(err.Error(), "early access") {
		t.Fatalf("%v", err)
	}
	t.Setenv("PATH", t.TempDir())
	if _, err := Correr(context.Background(), plugin(t), filepath.Join(t.TempDir(), "r.json"), 2.5); !errors.Is(err, ErrNoDisponible) {
		t.Fatalf("sin claude en PATH: %v", err)
	}
}

func TestLeerExigeEsquema1(t *testing.T) {
	if _, err := Leer([]byte(`{"schemaVersion":2}`)); err == nil {
		t.Fatal("esquema 2 aceptado")
	}
}
