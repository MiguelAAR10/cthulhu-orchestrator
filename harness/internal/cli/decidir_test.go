package cli

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"orq/internal/diario"
)

func TestMetricasCLIImprimeMediana(t *testing.T) {
	root := repoCLI(t)
	now := time.Now().UTC()
	path := diario.Ruta(filepath.Join(root, ".orq"))
	for _, e := range []diario.Evento{
		{TS: now.Add(-3 * time.Hour), Tipo: "verificacion", Slice: "01", Resultado: "paso"},
		{TS: now.Add(-3*time.Hour + 4*time.Minute), Tipo: "decision", Slice: "01", Resultado: "aceptado"},
		{TS: now.Add(-3 * time.Hour), Tipo: "verificacion", Slice: "02", Resultado: "paso"},
		{TS: now.Add(-3*time.Hour + 12*time.Minute), Tipo: "decision", Slice: "02", Resultado: "rechazado"},
		{TS: now.Add(-30 * time.Hour), Tipo: "verificacion", Slice: "03", Resultado: "paso"},
		{TS: now.Add(-time.Hour), Tipo: "intento", Slice: "04", Intento: 2, Datos: map[string]any{"causa": "examen"}},
	} {
		if err := diario.Anotar(path, e); err != nil {
			t.Fatal(err)
		}
	}
	out, errs, code := run(t, "metricas")
	if code != 0 {
		t.Fatalf("%d %s", code, errs)
	}
	for _, want := range []string{"verificado → decisión: mediana 8 min (2 decididos)", "sin decidir tras 24 h: 1", "intentos nuevos por causa: worker 0 · examen 1"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
}
