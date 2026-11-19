package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"orq/internal/diario"
)

const tarjetaCLI = `## Historia
El total suma 170.
## Fuera
- otros totales
## Examen
- tests/s01.sh compara total.txt con 170
## Zonas
- total.txt
## Preguntas
- ninguna
`

// conTarjeta writes the card file into a temp dir and sets it on slice nn.
func conTarjeta(t *testing.T, nn, texto string) (out, errs string, code int) {
	t.Helper()
	f := filepath.Join(t.TempDir(), "tarjeta.md")
	if err := os.WriteFile(f, []byte(texto), 0o644); err != nil {
		t.Fatal(err)
	}
	return run(t, "slice", "tarjeta", nn, "--archivo", f)
}

func abrirSinConteo(t *testing.T) string {
	t.Helper()
	root := repoCLI(t)
	t.Setenv("ORQ_STATE", t.TempDir())
	if _, errs, code := run(t, append(append([]string{}, abrirBase...), "--sin-conteo", "prueba")...); code != 0 {
		t.Fatalf("abrir: %d %s", code, errs)
	}
	return root
}

func TestTarjetaCLIPoneYMuestra(t *testing.T) {
	abrirSinConteo(t)
	if _, errs, code := conTarjeta(t, "01", strings.Replace(tarjetaCLI, "## Zonas\n- total.txt\n", "", 1)); code != 1 || !strings.Contains(errs, "Zonas") {
		t.Fatalf("una tarjeta sin zonas se rechaza: %d %s", code, errs)
	}
	out, errs, code := conTarjeta(t, "01", tarjetaCLI)
	if code != 0 {
		t.Fatalf("%d %s", code, errs)
	}
	contiene(t, out, "S-01: tarjeta lista", "orq slice aprobar 01")
	out, _, code = run(t, "slice", "tarjeta", "01")
	if code != 0 {
		t.Fatal(out)
	}
	contiene(t, out, "Historia", "El total suma 170.", "Fuera", "otros totales", "Zonas", "total.txt", "Preguntas", "espera al Owner")
}

func TestAprobarCLISella(t *testing.T) {
	root := abrirSinConteo(t)
	if _, errs, code := run(t, "slice", "aprobar", "01"); code != 1 || !strings.Contains(errs, "orq slice tarjeta 01") {
		t.Fatalf("sin tarjeta: %d %s", code, errs)
	}
	conTarjeta(t, "01", tarjetaCLI)
	t.Setenv("ORQ_ROL", "orquestador")
	if _, errs, code := run(t, "slice", "aprobar", "01"); code != 1 || !strings.Contains(errs, "terminal del Owner") {
		t.Fatalf("desde un panel de agente: %d %s", code, errs)
	}
	t.Setenv("ORQ_ROL", "")
	out, errs, code := run(t, "slice", "aprobar", "01")
	if code != 0 {
		t.Fatalf("%d %s", code, errs)
	}
	contiene(t, out, "S-01: tarjeta aprobada", "orq slice lanzar")
	out, _, code = run(t, "slice", "aprobar", "01")
	contiene(t, out, "ya estaba aprobada")
	evs, _ := diario.Leer(diario.Ruta(filepath.Join(root, ".orq")))
	n := 0
	for _, e := range evs {
		if e.Tipo == "tarjeta" && e.Resultado == "aprobada" && e.Slice == "01" {
			n++
		}
	}
	if code != 0 || n != 1 {
		t.Fatalf("un sello de aprobación: %d (code %d)", n, code)
	}
	if _, errs, code := conTarjeta(t, "01", tarjetaCLI); code != 1 || !strings.Contains(errs, "congelada") {
		t.Fatalf("aprobada no se reescribe: %d %s", code, errs)
	}
}

func TestStatusTarjetasPrimero(t *testing.T) {
	abrirSinConteo(t)
	conLimites(t)
	conTarjeta(t, "01", tarjetaCLI)
	out, errs, code := run(t, "status")
	if code != 0 {
		t.Fatalf("%d %s", code, errs)
	}
	contiene(t, out, "tarjetas por aprobar (antes que nada):", "S-01 «total» lista hace 0m · lee: orq slice tarjeta 01 · aprueba: orq slice aprobar 01")
	if strings.Index(out, "tarjetas por aprobar") > strings.Index(out, "esperan al Owner") {
		t.Fatalf("las tarjetas van primero:\n%s", out)
	}
	run(t, "slice", "aprobar", "01")
	out, _, _ = run(t, "status")
	contiene(t, out, "tarjetas por aprobar: ninguna")
}

// seccionDe returns the lines of one section of the text decision page.
func seccionDe(out, titulo string) string {
	_, s, _ := strings.Cut(out, "\n== "+titulo+" (")
	s, _, _ = strings.Cut(s, "\n== ")
	return s
}

func aprobada(t *testing.T) func(wt string) {
	return func(wt string) {
		if _, errs, code := conTarjeta(t, "01", tarjetaCLI); code != 0 {
			t.Fatal(errs)
		}
		if _, errs, code := run(t, "slice", "aprobar", "01"); code != 0 {
			t.Fatal(errs)
		}
	}
}

func TestDecidirMuestraTarjeta(t *testing.T) {
	sliceVerdeCon(t, nil, aprobada(t))
	out, _, code := run(t, "slice", "decidir", "01")
	if code != 0 {
		t.Fatal(out)
	}
	contiene(t, out, "== Historia (tarjeta del slice)", "El total suma 170.", "== Tarjeta (tarjeta del slice · aprobada ")
	tj := seccionDe(out, "Tarjeta")
	contiene(t, tj, "fuera: otros totales", "examen: tests/s01.sh compara total.txt con 170", "zonas: total.txt", "preguntas: ninguna")
	if strings.Contains(tj, "!!") {
		t.Fatalf("sin desvíos no hay alertas en la tarjeta:\n%s", tj)
	}
}

func TestDecidirTarjetaExamenYZonasTrasAprobar(t *testing.T) {
	sliceVerdeCon(t, nil, func(wt string) {
		aprobada(t)(wt)
		commitClon(t, wt, "tests/extra.sh", "true\n")
		if _, errs, code := run(t, "slice", "examen", "tests/extra.sh", "--aprobado", "req ATN-9"); code != 0 {
			t.Fatalf("examen: %d %s", code, errs)
		}
		commitClon(t, wt, "total.txt.bak", "x\n")
	})
	out, _, code := run(t, "slice", "decidir", "01")
	if code != 0 {
		t.Fatal(out)
	}
	tj := seccionDe(out, "Tarjeta")
	contiene(t, tj, "!! examen cambiado tras aprobar la tarjeta", "tests/extra.sh", "req ATN-9", "!! fuera de las zonas de la tarjeta: total.txt.bak")
	if strings.Contains(tj, "fuera de las zonas de la tarjeta: total.txt\n") {
		t.Fatalf("total.txt está en su zona:\n%s", tj)
	}
}

func TestDecidirSinTarjetaAvisa(t *testing.T) {
	sliceVerdeCon(t, nil, nil)
	out, _, _ := run(t, "slice", "decidir", "01")
	contiene(t, seccionDe(out, "Tarjeta"), "!! sin tarjeta aprobada")
}
