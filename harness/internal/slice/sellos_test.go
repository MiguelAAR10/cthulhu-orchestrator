package slice

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"orq/internal/diario"
)

func eventos(t *testing.T, r Repo, tipo string) []diario.Evento {
	t.Helper()
	evs, err := diario.Leer(diario.Ruta(r.OrqDir))
	if err != nil {
		t.Fatal(err)
	}
	var out []diario.Evento
	for _, e := range evs {
		if e.Tipo == tipo {
			out = append(out, e)
		}
	}
	return out
}

func TestSellosDelCiclo(t *testing.T) {
	r := repoDeJuguete(t)
	e := abrir(t, r)
	commitEn(t, e.Worktree, "total.txt", "170\n")
	t1, t2 := t0.Add(time.Hour), t0.Add(2*time.Hour)
	if _, _, err := r.Verificar("", t1); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := r.Cerrar("", true, "", t2); err != nil {
		t.Fatal(err)
	}
	v := eventos(t, r, "verificacion")
	if len(v) != 1 || v[0].Resultado != "paso" || !v[0].TS.Equal(t1) {
		t.Fatalf("verificacion: %+v", v)
	}
	terminado, _ := v[0].Datos["terminado"].(string)
	if _, err := time.Parse(time.RFC3339, terminado); err != nil {
		t.Fatalf("terminado %q: %v", terminado, err)
	}
	d := eventos(t, r, "decision")
	if len(d) != 1 || d[0].Resultado != "aceptado" || !d[0].TS.Equal(t2) || d[0].Slice != "01" {
		t.Fatalf("decision: %+v", d)
	}
}

func TestRechazoSellaMotivo(t *testing.T) {
	r := repoDeJuguete(t)
	abrir(t, r)
	if _, _, _, err := r.Cerrar("", false, "no es lo pedido", t0); err != nil {
		t.Fatal(err)
	}
	d := eventos(t, r, "decision")
	if len(d) != 1 || d[0].Resultado != "rechazado" || d[0].Detalle != "no es lo pedido" {
		t.Fatalf("decision: %+v", d)
	}
}

func TestCierreRetomadoNoSellaDosVeces(t *testing.T) {
	r := repoDeJuguete(t)
	e := abrir(t, r)
	commitEn(t, e.Worktree, "total.txt", "170\n")
	if _, _, err := r.Verificar("", t0); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(r.Root, "README"), []byte("borrador\n"), 0o644)
	Git(r.Root, "add", "README")
	if _, _, _, err := r.Cerrar("", true, "", t0); err == nil {
		t.Fatal("the first close must stop on the owner's staged file")
	}
	Git(r.Root, "commit", "-qm", "dueño")
	r.Cerrar("", true, "", t0)
	if reg, _, _, err := r.Cerrar("", true, "", t0); err != nil || reg.Result != "aceptado" {
		t.Fatalf("%+v %v", reg, err)
	}
	if d := eventos(t, r, "decision"); len(d) != 1 {
		t.Fatalf("one decision expected, got %+v", d)
	}
}

func TestReintentoAnotaCausa(t *testing.T) {
	r := repoDeJuguete(t)
	abrir(t, r)
	r.Verificar("", t0)
	r.Verificar("", t0) // sin progreso → BLOQUEADO
	for _, causa := range []string{"", "otra"} {
		if _, err := r.Reintentar("", "x", causa, false); err == nil || !strings.Contains(err.Error(), "worker|examen") {
			t.Fatalf("causa %q: %v", causa, err)
		}
	}
	if _, err := r.Reintentar("", "el examen pedía otro total", CausaExamen, false); err != nil {
		t.Fatal(err)
	}
	ev := eventos(t, r, "intento")
	if len(ev) != 1 || ev[0].Intento != 2 || ev[0].Datos["causa"] != "examen" || ev[0].Detalle != "el examen pedía otro total" {
		t.Fatalf("intento: %+v", ev)
	}
}

func TestSumarExamenAnota(t *testing.T) {
	r := repoDeJuguete(t)
	e := abrir(t, r)
	commitEn(t, e.Worktree, "tests/extra.sh", "true\n")
	if _, err := r.SumarExamen("", []string{"tests/extra.sh"}, "", "req ATN-1"); err != nil {
		t.Fatal(err)
	}
	ev := eventos(t, r, "examen")
	if len(ev) != 1 || ev[0].Detalle != "req ATN-1" || ev[0].Slice != "01" {
		t.Fatalf("examen: %+v", ev)
	}
}

// diarioFijo writes the fixture journal of TestMetricasDecisionDesdeDiario and returns its `now`.
func diarioFijo(t *testing.T, orqDir string) time.Time {
	t.Helper()
	now := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)
	dia := time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC)
	h := func(hh, mm int) time.Time {
		return dia.Add(time.Duration(hh)*time.Hour + time.Duration(mm)*time.Minute)
	}
	evs := []diario.Evento{
		{TS: h(10, 0), Tipo: "verificacion", Slice: "01", Resultado: "paso"},
		{TS: h(10, 6), Tipo: "decision", Slice: "01", Resultado: "aceptado"},
		{TS: h(10, 0), Tipo: "verificacion", Slice: "02", Resultado: "paso"},
		{TS: h(10, 20), Tipo: "decision", Slice: "02", Resultado: "rechazado", Detalle: "no es lo pedido"},
		{TS: h(9, 0), Tipo: "verificacion", Slice: "03", Resultado: "fallo"},
		{TS: h(10, 0), Tipo: "verificacion", Slice: "03", Resultado: "paso"},
		{TS: h(10, 10), Tipo: "decision", Slice: "03", Resultado: "aceptado"},
		{TS: h(10, 0), Tipo: "verificacion", Slice: "04", Resultado: "paso"},
		{TS: h(10, 30), Tipo: "fase", Slice: "04", De: Verificado, A: Implementando},
		{TS: h(11, 0), Tipo: "verificacion", Slice: "04", Resultado: "paso"},
		{TS: h(11, 4), Tipo: "decision", Slice: "04", Resultado: "aceptado"},
		{TS: now.Add(-48 * time.Hour), Tipo: "verificacion", Slice: "05", Resultado: "paso"},
		{TS: now.Add(-time.Hour), Tipo: "verificacion", Slice: "06", Resultado: "paso"},
		{TS: h(8, 0), Tipo: "intento", Slice: "07", Intento: 2, Datos: map[string]any{"causa": CausaWorker}},
		{TS: h(8, 0), Tipo: "intento", Slice: "08", Intento: 2, Datos: map[string]any{"causa": CausaExamen}},
		{TS: h(8, 30), Tipo: "intento", Slice: "08", Intento: 3, Datos: map[string]any{"causa": CausaExamen}},
	}
	for _, e := range evs {
		if err := diario.Anotar(diario.Ruta(orqDir), e); err != nil {
			t.Fatal(err)
		}
	}
	return now
}

func TestMetricasDecisionDesdeDiario(t *testing.T) {
	r := Repo{OrqDir: t.TempDir()}
	now := diarioFijo(t, r.OrqDir)
	m := r.Metricas(time.Time{}, now)
	if m.Decididos != 4 || m.DecisionMinMediana != 8 || m.SinDecidir24h != 1 {
		t.Fatalf("decididos %d, mediana %v, sin decidir %d", m.Decididos, m.DecisionMinMediana, m.SinDecidir24h)
	}
	if m.IntentosPorCausa[CausaWorker] != 1 || m.IntentosPorCausa[CausaExamen] != 2 {
		t.Fatalf("causas %v", m.IntentosPorCausa)
	}
	desde := time.Date(2026, 11, 18, 10, 7, 0, 0, time.UTC)
	if m := r.Metricas(desde, now); m.Decididos != 3 {
		t.Fatalf("since 10:07 only 3 decisions count, got %d", m.Decididos)
	}
}
