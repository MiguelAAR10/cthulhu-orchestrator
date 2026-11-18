package cli

import (
	"strings"
	"testing"
)

// The usage text offers only what exists: --en-sitio was removed, `slice convertir` exists.
func TestUsoSinEnSitioYConConvertir(t *testing.T) {
	if strings.Contains(usage, "--en-sitio") {
		t.Fatal("usage still offers --en-sitio")
	}
	if !strings.Contains(usage, "orq slice convertir NN --ratifico") {
		t.Fatal("usage does not list `orq slice convertir NN --ratifico`")
	}
	if strings.Contains(usage, "verify does the same on its own") {
		t.Fatal("usage still says verify adopts a legacy slice")
	}
	if !strings.Contains(usage, "step 3") {
		t.Fatal("usage must say the ratification is not authenticated until roadmap step 3")
	}
}

// `orq slice convertir` on a slice opened by this version refuses: it already has a contract.
func TestSliceConvertirSliceActualSeNiega(t *testing.T) {
	repoCLI(t)
	t.Setenv("ORQ_STATE", t.TempDir())
	if _, errs, code := run(t, append(abrirBase, "--reporte", "out.tap", "--formato", "tap")...); code != 0 {
		t.Fatalf("abrir: %d %s", code, errs)
	}
	for _, args := range [][]string{{"slice", "convertir"}, {"slice", "convertir", "01"}, {"slice", "convertir", "01", "--ratifico", " "}} {
		if _, errs, code := run(t, args...); code != 1 || !strings.Contains(errs, "uso: orq slice convertir NN --ratifico") {
			t.Fatalf("%v: %d %s", args, code, errs)
		}
	}
	if out, errs, code := run(t, "slice", "convertir", "01", "--ratifico", "Owner: test"); code != 1 || !strings.Contains(errs, "nothing to convert") {
		t.Fatalf("%d %s %s", code, out, errs)
	}
}

// `orq slice abrir --en-sitio` is refused with its reason and exit 1.
func TestSliceAbrirEnSitioRechazado(t *testing.T) {
	repoCLI(t)
	t.Setenv("ORQ_STATE", t.TempDir())
	_, errs, code := run(t, append(abrirBase, "--sin-conteo", "x", "--en-sitio", "motivo")...)
	if code != 1 || !strings.Contains(errs, "--en-sitio ya no existe") {
		t.Fatalf("%d %s", code, errs)
	}
}
