package slice

import (
	"slices"
	"strings"
	"testing"
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
