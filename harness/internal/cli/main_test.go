package cli

import (
	"os"
	"path/filepath"
	"testing"
)

// Las pruebas nunca leen la configuración real del usuario: ORQ_CONFIG apunta a un archivo que no existe.
// They also run from a temp directory outside any repository.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "orq-cli-test-")
	if err != nil {
		panic(err)
	}
	os.Setenv("ORQ_CONFIG", filepath.Join(dir, "config.json"))
	// The suite may run from a linked worktree (a worker for the gate), where orq refuses the
	// orchestrator's slice commands: run from a directory that belongs to no repository.
	if err := os.Chdir(dir); err != nil {
		panic(err)
	}
	os.Unsetenv("ORQ_ESCRIBA_MODELO")
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
