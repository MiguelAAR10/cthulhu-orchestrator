package cli

import (
	"orq/internal/identidad"
	"os"
	"strings"
	"testing"
)

// Older cli tests assert orq's original Spanish messages; they run with ORQ_LANG=es
// unless they set ORQ_LANG themselves. English (the default) is tested explicitly.
func TestMain(m *testing.M) {
	if os.Getenv("ORQ_LANG") == "" {
		os.Setenv("ORQ_LANG", "es")
	}
	// Identidad hermética: el token del Owner vive en una carpeta temporal y las pruebas actúan
	// como el Owner (ORQ_TOKEN). Las que prueban suplantación lo vacían.
	cfg, _ := os.MkdirTemp("", "orq-cfg")
	os.Setenv("XDG_CONFIG_HOME", cfg)
	tok, err := identidad.Asegurar()
	if err != nil {
		panic(err)
	}
	os.Setenv("ORQ_TOKEN", tok)
	// Las pruebas no dependen de tener una terminal: el proceso de prueba es una sesión anclada.
	if identidad.Tabla != nil {
		if _, err := identidad.Anclar(identidad.Tabla, os.Getpid(), "owner"); err != nil {
			panic(err)
		}
	}
	os.Unsetenv("ORQ_SHIM")
	os.Exit(m.Run())
}

func TestErroresEnInglesPorDefecto(t *testing.T) {
	t.Setenv("ORQ_ROOT", t.TempDir())
	t.Setenv("ORQ_LANG", "")
	_, errs, code := run(t, "plan", "show")
	if code == 0 || !strings.Contains(errs, "no slice is open") || strings.Contains(errs, "no hay slice") {
		t.Fatalf("%d %q", code, errs)
	}
	_, errs, _ = run(t, "mem", "add", "--imp", "x", "--cat", "c", "")
	if strings.Contains(errs, "texto vacío") {
		t.Fatalf("%q", errs)
	}
	t.Setenv("ORQ_LANG", "es")
	if _, errs, _ := run(t, "plan", "show"); !strings.Contains(errs, "no hay slice abierto") {
		t.Fatalf("%q", errs)
	}
}
