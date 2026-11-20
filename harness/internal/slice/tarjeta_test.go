package slice

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"orq/internal/diario"
)

const tarjetaBuena = `# S-01 · checkout

## Historia
El cliente paga con tarjeta desde el carrito.
Ve el recibo al terminar.

## Fuera
- pagos en cuotas

## Examen
- tests/s01.sh: un pago de prueba deja un recibo

## Zonas
- src/pagos/
- web/carrito

## Preguntas
* ninguna
`

func TestLeerTarjetaCincoPartes(t *testing.T) {
	tj, err := LeerTarjeta(tarjetaBuena)
	if err != nil {
		t.Fatal(err)
	}
	if len(tj.Historia) != 2 || tj.Historia[1] != "Ve el recibo al terminar." {
		t.Fatalf("historia: %q", tj.Historia)
	}
	if !slices.Equal(tj.Fuera, []string{"pagos en cuotas"}) || !slices.Equal(tj.Zonas, []string{"src/pagos/", "web/carrito"}) ||
		!slices.Equal(tj.Preguntas, []string{"ninguna"}) || len(tj.Examen) != 1 {
		t.Fatalf("%+v", tj)
	}
	if tj.EstaAprobada() {
		t.Fatal("una tarjeta leída no está aprobada")
	}
}

func TestLeerTarjetaRechaza(t *testing.T) {
	cambiar := func(viejo, nuevo string) string { return strings.Replace(tarjetaBuena, viejo, nuevo, 1) }
	casos := []struct{ nombre, texto, falta string }{
		{"sin zonas", cambiar("## Zonas\n- src/pagos/\n- web/carrito\n", ""), "Zonas"},
		{"preguntas vacía", cambiar("* ninguna\n", ""), "Preguntas"},
		{"historia larga", cambiar("Ve el recibo al terminar.\n", "a\nb\nc\n"), "Historia"},
		{"sección larga", cambiar("- pagos en cuotas\n", "a\nb\nc\nd\ne\nf\n"), "Fuera"},
		{"línea larga", cambiar("pagos en cuotas", strings.Repeat("x", 201)), "200"},
		{"muchas palabras", cambiar("pagos en cuotas", strings.Repeat(strings.Repeat("a ", 90)+"\n", 5)), "400"},
		{"sección desconocida", cambiar("## Preguntas", "## Riesgos\nalgo\n## Preguntas"), "Riesgos"},
		{"repetida", cambiar("## Preguntas", "## Fuera\notra\n## Preguntas"), "Fuera"},
		{"zona fuera del repo", cambiar("web/carrito", "../otro"), "Zonas"},
		{"zona raíz punto", cambiar("web/carrito", "."), "todo el repo"},
		{"zona raíz ./", cambiar("web/carrito", "./"), "todo el repo"},
		{"zona raíz barra", cambiar("web/carrito", "/"), "Zonas"},
		{"zona absoluta", cambiar("web/carrito", "/etc"), "Zonas"},
		{"texto suelto", "hola\n" + tarjetaBuena, "hola"},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			_, err := LeerTarjeta(c.texto)
			if err == nil || !strings.Contains(err.Error(), c.falta) {
				t.Fatalf("quiero un error que nombre %q, salió %v", c.falta, err)
			}
		})
	}
	if _, err := LeerTarjeta(cambiar("Ve el recibo al terminar.\n", "a\nb\nc\n")); !strings.Contains(err.Error(), "3") {
		t.Fatalf("el error de la historia dice el tope: %v", err)
	}
}

func ponerTarjeta(t *testing.T, r Repo, nn string, now time.Time) *Estado {
	t.Helper()
	tj, err := LeerTarjeta(tarjetaBuena)
	if err != nil {
		t.Fatal(err)
	}
	e, err := r.PonerTarjeta(nn, tj, now)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func TestTarjetaAprobadaSellaYCongela(t *testing.T) {
	r := repoDeJuguete(t)
	abrir(t, r)
	if e := ponerTarjeta(t, r, "01", t0); !e.Tarjeta.Lista.Equal(t0) || e.Tarjeta.EstaAprobada() {
		t.Fatalf("%+v", e.Tarjeta)
	}
	if ev := eventos(t, r, "tarjeta"); len(ev) != 1 || ev[0].Resultado != "lista" || ev[0].Slice != "01" {
		t.Fatalf("lista: %+v", ev)
	}
	e, ya, err := r.AprobarTarjeta("01", t0.Add(3*time.Minute))
	if err != nil || ya || !e.Tarjeta.Aprobada.Equal(t0.Add(3*time.Minute)) {
		t.Fatalf("%v %v %+v", err, ya, e.Tarjeta)
	}
	if _, ya, err := r.AprobarTarjeta("01", t0.Add(5*time.Minute)); err != nil || !ya {
		t.Fatalf("repetir aprobar: %v %v", ya, err)
	}
	ev := eventos(t, r, "tarjeta")
	if len(ev) != 2 || ev[1].Resultado != "aprobada" || !ev[1].TS.Equal(t0.Add(3*time.Minute)) {
		t.Fatalf("un solo sello de aprobación: %+v", ev)
	}
	tj, _ := LeerTarjeta(tarjetaBuena)
	if _, err := r.PonerTarjeta("01", tj, t0.Add(time.Hour)); err == nil || !strings.Contains(err.Error(), "congelada") {
		t.Fatalf("una tarjeta aprobada no se reescribe: %v", err)
	}
	var c Contrato
	if _, err := r.leerBlob(refContrato("01"), &c); err != nil || !c.Tarjeta.EstaAprobada() || !slices.Equal(c.Tarjeta.Zonas, tj.Zonas) {
		t.Fatalf("el contrato en git lleva la tarjeta aprobada: %v %+v", err, c.Tarjeta)
	}
	if err := e.PuedeLanzar(); err != nil {
		t.Fatal(err)
	}
}

func TestTarjetaForjadaNoEsConfiable(t *testing.T) {
	r := repoDeJuguete(t)
	abrir(t, r)
	e := ponerTarjeta(t, r, "01", t0)
	e.Tarjeta.Aprobada = t0.Add(time.Minute)
	raw, _ := json.Marshal(e)
	os.WriteFile(r.estadoPath("01"), raw, 0o644)
	e, _ = r.Load("01")
	if err := r.confiable(e); !errors.Is(err, ErrEstadoForjado) || !strings.Contains(err.Error(), "tarjeta") {
		t.Fatalf("una aprobación escrita a mano no vale: %v", err)
	}
}

func TestSinTarjetaAprobadaNoSeLanza(t *testing.T) {
	r := repoDeJuguete(t)
	e := abrir(t, r)
	if _, _, err := r.AprobarTarjeta("01", t0); err == nil || !strings.Contains(err.Error(), "orq slice tarjeta 01") {
		t.Fatalf("aprobar sin tarjeta: %v", err)
	}
	if err := e.PuedeLanzar(); err == nil || !strings.Contains(err.Error(), "orq slice tarjeta 01") {
		t.Fatalf("sin tarjeta: %v", err)
	}
	e = ponerTarjeta(t, r, "01", t0)
	if err := e.PuedeLanzar(); err == nil || !strings.Contains(err.Error(), "orq slice aprobar 01") {
		t.Fatalf("tarjeta sin aprobar: %v", err)
	}
	e.Ruta = RutaFix
	if err := e.PuedeLanzar(); err != nil {
		t.Fatalf("un fix no pide tarjeta: %v", err)
	}
}

// diarioTarjetas writes the fixture of TestMetricasTarjetaDesdeDiario: 4 closed slices, 3 approved
// cards (4, 10 and 2 min; S-01 was rewritten at 10:20), one card never approved, 2 exam corrections.
func diarioTarjetas(t *testing.T, orqDir string) {
	t.Helper()
	h := func(hh, mm int) time.Time { return time.Date(2026, 11, 18, hh, mm, 0, 0, time.UTC) }
	evs := []diario.Evento{
		{TS: h(10, 0), Tipo: "tarjeta", Slice: "01", Resultado: "lista"},
		{TS: h(10, 20), Tipo: "tarjeta", Slice: "01", Resultado: "lista"},
		{TS: h(10, 24), Tipo: "tarjeta", Slice: "01", Resultado: "aprobada"},
		{TS: h(11, 0), Tipo: "tarjeta", Slice: "02", Resultado: "lista"},
		{TS: h(11, 10), Tipo: "tarjeta", Slice: "02", Resultado: "aprobada"},
		{TS: h(12, 0), Tipo: "tarjeta", Slice: "03", Resultado: "lista"},
		{TS: h(12, 2), Tipo: "tarjeta", Slice: "03", Resultado: "aprobada"},
		{TS: h(13, 0), Tipo: "tarjeta", Slice: "04", Resultado: "lista"},
		{TS: h(11, 30), Tipo: "examen", Slice: "02", Detalle: "req ATN-1"},
		{TS: h(12, 30), Tipo: "examen", Slice: "03", Detalle: "req ATN-2"},
	}
	for _, e := range evs {
		if err := diario.Anotar(diario.Ruta(orqDir), e); err != nil {
			t.Fatal(err)
		}
	}
	for i, res := range []string{"aceptado", "aceptado", "rechazado", "aceptado"} {
		appendJSON(filepath.Join(orqDir, "slices.jsonl"), Registro{NN: fmt.Sprintf("%02d", i+1), Intento: 1, Result: res, Abierto: h(9, 0), Cerrado: h(14, 0)})
	}
}

func TestMetricasTarjetaDesdeDiario(t *testing.T) {
	r := Repo{OrqDir: t.TempDir()}
	diarioTarjetas(t, r.OrqDir)
	m := r.Metricas(time.Time{}, time.Date(2026, 11, 19, 0, 0, 0, 0, time.UTC))
	if m.TarjetaMinMediana != 4 || m.TarjetasAprobadas != 3 {
		t.Fatalf("tarjeta: mediana %v de %d", m.TarjetaMinMediana, m.TarjetasAprobadas)
	}
	if m.CorreccionesExamen != 2 || m.CorreccionesPor10 != 5 {
		t.Fatalf("correcciones %d, por 10 %v", m.CorreccionesExamen, m.CorreccionesPor10)
	}
	m = r.Metricas(time.Date(2026, 11, 18, 10, 30, 0, 0, time.UTC), time.Date(2026, 11, 19, 0, 0, 0, 0, time.UTC))
	if m.TarjetaMinMediana != 6 || m.TarjetasAprobadas != 2 {
		t.Fatalf("desde 10:30: mediana %v de %d", m.TarjetaMinMediana, m.TarjetasAprobadas)
	}
}

// Agent-first step 4: under orq2 every cap violation comes back in one pass; orq keeps the first.
func TestLeerTarjetaTodasLasViolaciones(t *testing.T) {
	malo := strings.Replace(tarjetaBuena, "Ve el recibo al terminar.\n", "a\nb\nc\n", 1)
	malo = strings.Replace(malo, "pagos en cuotas", strings.Repeat("x", 201), 1)
	malo = strings.Replace(malo, "web/carrito", "../otro", 1)
	t.Setenv("ORQ_NOMBRE", "orq2")
	_, err := LeerTarjeta(malo)
	if err == nil || strings.Count(err.Error(), "\n") != 2 {
		t.Fatalf("want 3 violations, got %v", err)
	}
	for _, q := range []string{"Historia", "200", "Zonas"} {
		if !strings.Contains(err.Error(), q) {
			t.Errorf("missing %q in %v", q, err)
		}
	}
	t.Setenv("ORQ_NOMBRE", "")
	if _, err := LeerTarjeta(malo); err == nil || strings.Contains(err.Error(), "\n") {
		t.Fatalf("orq keeps one error: %v", err)
	}
}

// The caps sentence carries each constant's value.
func TestTopesTarjetaDiceLasConstantes(t *testing.T) {
	for _, n := range []int{maxLineasHistoria, maxLineasSeccion, maxCaracteres, maxPalabras} {
		if !strings.Contains(TopesTarjeta(), fmt.Sprintf("≤%d ", n)) {
			t.Errorf("caps sentence lacks %d: %s", n, TopesTarjeta())
		}
	}
}
