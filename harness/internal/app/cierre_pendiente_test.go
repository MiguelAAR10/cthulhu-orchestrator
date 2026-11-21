package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"orq/internal/slice"
)

// G3 M-1: a rejection the Owner decided but whose close stopped half way stops the slice: nothing is
// dispatched to it and pendientes names the close as owed.
func TestRechazoAMedioCerrarDetieneElSlice(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq2")
	p, term, e := proyectoDePrueba(t, "")
	planS01(t, p, e.NN)
	e.Cierre = &slice.IntencionCierre{Resultado: "rechazado", Motivo: "no era lo pedido", TS: time.Now()}
	raw, _ := json.Marshal(e)
	if err := os.WriteFile(filepath.Join(p.Dir, "slices", e.NN+".json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	ps, err := p.Pendientes()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(ps, func(x Pendiente) bool {
		return x.Tipo == PendCierre && x.Slice == e.NN && strings.Contains(x.Texto, "rechazado")
	}) || slices.ContainsFunc(ps, func(x Pendiente) bool { return x.Tipo == PendFaltaLanzar }) {
		t.Fatalf("the half-done rejection must be what is owed: %+v", ps)
	}
	ds, _ := p.DespacharUnaVez(context.Background(), "claude")
	if slices.ContainsFunc(ds, func(d Despacho) bool { return d.Resultado == "lanzado" }) || len(term.Llamadas) > 0 {
		t.Fatalf("dispatched work to a rejected slice: %+v", ds)
	}
}
