package identidad

import (
	"os"
	"testing"
)

func TestTokenPrivadoYValidacion(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("ORQ_TOKEN", "")
	if Valido() {
		t.Fatal("sin token no hay validez")
	}
	tok, err := Asegurar()
	if err != nil || len(tok) != 64 {
		t.Fatal(tok, err)
	}
	info, _ := os.Stat(Path())
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("permisos %v", info.Mode())
	}
	if again, _ := Asegurar(); again != tok {
		t.Fatal("el token cambió")
	}
	t.Setenv("ORQ_TOKEN", "otro")
	if Valido() {
		t.Fatal("token falso aceptado")
	}
	t.Setenv("ORQ_TOKEN", tok)
	if !Valido() {
		t.Fatal("token verdadero rechazado")
	}
	os.Chmod(Path(), 0o644)
	if Valido() {
		t.Fatal("un token legible por otros no vale")
	}
}
