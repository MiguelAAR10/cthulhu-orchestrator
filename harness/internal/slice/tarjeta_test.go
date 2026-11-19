package slice

import (
	"encoding/json"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"
	"time"
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
