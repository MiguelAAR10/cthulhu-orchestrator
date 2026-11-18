package cli

import (
	"strings"
	"testing"
)

// The usage text offers only what exists: --en-sitio, `slice convertir` and `slice soltar` were removed.
func TestUsoSinEnSitioNiAdopcion(t *testing.T) {
	for _, viejo := range []string{"--en-sitio", "convertir", "soltar NN", "ratific", "legado"} {
		if strings.Contains(usage, viejo) {
			t.Fatalf("usage still offers %q", viejo)
		}
	}
}

// `orq slice convertir` and `orq slice soltar` no longer exist: usage error, exit 1.
func TestSliceConvertirYSoltarNoExisten(t *testing.T) {
	repoCLI(t)
	t.Setenv("ORQ_STATE", t.TempDir())
	for _, args := range [][]string{{"slice", "convertir", "01"}, {"slice", "soltar", "01"}} {
		if _, errs, code := run(t, args...); code != 1 || !strings.Contains(errs, "orq slice abrir NN") || strings.Contains(errs, "S-01") {
			t.Fatalf("%v: %d %s", args, code, errs)
		}
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
