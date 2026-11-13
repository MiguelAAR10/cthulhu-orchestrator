package cli

import (
	"os"
	"path/filepath"
	"testing"
)

// Las pruebas nunca leen la configuración real del usuario: ORQ_CONFIG apunta a un archivo que no existe.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "orq-cli-test-")
	if err != nil {
		panic(err)
	}
	os.Setenv("ORQ_CONFIG", filepath.Join(dir, "config.json"))
	os.Unsetenv("ORQ_ESCRIBA_MODELO")
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
