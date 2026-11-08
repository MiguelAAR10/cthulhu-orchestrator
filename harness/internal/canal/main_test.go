package canal

import (
	"os"
	"testing"

	"orq/internal/identidad/pruebas"
)

func TestMain(m *testing.M) {
	pruebas.Owner()
	os.Exit(m.Run())
}
