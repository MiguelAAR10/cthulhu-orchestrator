package slice

import (
	"os"
	"path/filepath"
	"slices"
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
	t.Parallel()
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
	if len(v) != 1 || v[0].Resultado != "paso" || v[0].TS.Before(t1) || v[0].TS.Sub(t1) > time.Minute {
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	r := repoDeJuguete(t)
	e := abrir(t, r)
	commitEn(t, e.Worktree, "wip.txt", "intento\n")
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
	t.Parallel()
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
	t.Parallel()
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

func TestEsperandoOrdenaPorAntiguedad(t *testing.T) {
	t.Parallel()
	r, _, _ := dosSlices(t)
	if _, _, err := r.Verificar("01", t0.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, _, err := r.Verificar("02", t0); err != nil {
		t.Fatal(err)
	}
	es, err := r.Esperando()
	if err != nil || len(es) != 2 || es[0].NN != "02" || es[1].NN != "01" {
		t.Fatalf("%v %v", es, err)
	}
}

func metricasDe(t *testing.T, evs []diario.Evento, now time.Time) Metricas {
	t.Helper()
	r := Repo{OrqDir: t.TempDir()}
	for _, e := range evs {
		if err := diario.Anotar(diario.Ruta(r.OrqDir), e); err != nil {
			t.Fatal(err)
		}
	}
	return r.Metricas(time.Time{}, now)
}

// Review fix 1: re-verifying a VERIFICADO slice does not restart the Owner's clock.
func TestMetricasReverificarNoReiniciaElReloj(t *testing.T) {
	t.Parallel()
	m := metricasDe(t, []diario.Evento{
		{TS: t0, Tipo: "verificacion", Slice: "01", Resultado: "paso"},
		{TS: t0.Add(30 * time.Minute), Tipo: "verificacion", Slice: "01", Resultado: "paso"},
		{TS: t0.Add(40 * time.Minute), Tipo: "decision", Slice: "01", Resultado: "aceptado"},
	}, t0.Add(time.Hour))
	if m.DecisionMinMediana != 40 {
		t.Fatalf("median %v, want 40", m.DecisionMinMediana)
	}
	m = metricasDe(t, []diario.Evento{
		{TS: t0, Tipo: "verificacion", Slice: "01", Resultado: "paso"},
		{TS: t0.Add(47 * time.Hour), Tipo: "verificacion", Slice: "01", Resultado: "paso"},
	}, t0.Add(48*time.Hour))
	if m.SinDecidir24h != 1 {
		t.Fatalf("a stale slice re-verified is still stale, got %d", m.SinDecidir24h)
	}
}

// Review fix 2: VERIFICADO→BLOQUEADO leaves the Owner's hands, like `orq status` says.
func TestMetricasBloqueoDetieneElReloj(t *testing.T) {
	t.Parallel()
	m := metricasDe(t, []diario.Evento{
		{TS: t0, Tipo: "verificacion", Slice: "01", Resultado: "paso"},
		{TS: t0.Add(time.Minute), Tipo: "fase", Slice: "01", De: Verificado, A: Bloqueado},
	}, t0.Add(48*time.Hour))
	if m.SinDecidir24h != 0 {
		t.Fatalf("blocked slice counted as waiting for the Owner: %d", m.SinDecidir24h)
	}
	m = metricasDe(t, []diario.Evento{
		{TS: t0, Tipo: "verificacion", Slice: "01", Resultado: "paso"},
		{TS: t0.Add(time.Minute), Tipo: "fase", Slice: "01", De: Verificado, A: Integrando},
	}, t0.Add(48*time.Hour))
	if m.SinDecidir24h != 1 {
		t.Fatalf("INTEGRANDO keeps the clock: %d", m.SinDecidir24h)
	}
}

// Review fix 3: the page diffs red→rec.Head and warns when the clone moved on.
func TestPaginaArchivosSonDelCommitVerificado(t *testing.T) {
	t.Parallel()
	r := repoDeJuguete(t)
	e := abrir(t, r)
	commitEn(t, e.Worktree, "total.txt", "170\n")
	if _, _, err := r.Verificar("", t0.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	commitEn(t, e.Worktree, "despues.txt", "x\n")
	pg, err := r.Pagina("01")
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(pg.Alineacion.Cambiados, "despues.txt") {
		t.Fatalf("unverified file listed: %v", pg.Alineacion.Cambiados)
	}
	if len(pg.Avisos) != 1 || !strings.Contains(pg.Avisos[0], Corto(pg.Estado.Recibo.Head)) {
		t.Fatalf("avisos %v", pg.Avisos)
	}
}

// Review fix 5: the stamp is taken after the run, so a slow run does not count against the Owner's wait.
func TestVerificacionSelloDespuesDeLaCorrida(t *testing.T) {
	t.Parallel()
	r := repoDeJuguete(t)
	e, _, err := r.Abrir("01", "total", "tests/s01.sh", "sleep 1.2; sh tests/s01.sh", Opciones{SinConteo: "juguete", Bundle: []string{"tests/datos.txt"}, Sano: "test -f total.txt"}, t0)
	if err != nil {
		t.Fatal(err)
	}
	commitEn(t, e.Worktree, "total.txt", "170\n")
	if _, _, err := r.Verificar("", t0); err != nil {
		t.Fatal(err)
	}
	v := eventos(t, r, "verificacion")
	if len(v) != 1 || v[0].TS.Sub(t0) < time.Second {
		t.Fatalf("stamp %v must include the run time after %v", v, t0)
	}
}

// Review 2: Fuera is computed for rec.Head itself, so a file the verified commit added outside the scope
// is flagged even when a later clone commit deleted it.
func TestPaginaFueraEsDelCommitVerificado(t *testing.T) {
	t.Parallel()
	r := repoDeJuguete(t)
	e, _, err := r.Abrir("01", "total", "tests/s01.sh", examen, Opciones{SinConteo: "juguete", Bundle: []string{"tests/datos.txt"}, Alcance: []string{"total.txt"}, Sano: "test -f total.txt"}, t0)
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(e.Worktree, "fuera.txt"), []byte("x\n"), 0o644)
	commitEn(t, e.Worktree, "total.txt", "170\n")
	if _, _, err := r.Verificar("", t0.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	Git(e.Worktree, "rm", "-q", "fuera.txt")
	if _, err := Git(e.Worktree, "commit", "-qm", "borra"); err != nil {
		t.Fatal(err)
	}
	pg, err := r.Pagina("01")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(pg.Alineacion.Fuera, "fuera.txt") {
		t.Fatalf("fuera.txt not flagged: Fuera=%v Cambiados=%v", pg.Alineacion.Fuera, pg.Alineacion.Cambiados)
	}
}
