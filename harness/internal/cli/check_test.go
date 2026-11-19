package cli

import (
	"os"
	"path/filepath"
	"testing"

	"orq/internal/config"
)

func escribirEn(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestInitEscribeChecks(t *testing.T) {
	root := repoCLI(t)
	t.Setenv("ORQ_STATE", t.TempDir())
	escribirEn(t, root, "go.mod", "module tienda\n")
	out, errs, code := run(t, "init")
	if code != 0 {
		t.Fatalf("init: %d %s %s", code, out, errs)
	}
	c, err := config.CargarDe(config.RutaProyecto(root))
	if err != nil {
		t.Fatal(err)
	}
	cs := c.ChecksEnOrden()
	if len(cs) != 3 || cs[0].Cmd != "! gofmt -l . | grep ." || cs[1].Cmd != "go vet ./..." || cs[2].Cmd != "go test ./..." {
		t.Fatalf("checks = %+v", cs)
	}
	contiene(t, out, "check lint: ! gofmt -l . | grep .", "check types: go vet ./...", "check tests: go test ./...")
}

func TestInitNoPisaChecksDelOwner(t *testing.T) {
	root := repoCLI(t)
	t.Setenv("ORQ_STATE", t.TempDir())
	escribirEn(t, root, "go.mod", "module tienda\n")
	propio := `{"terminal": "herdr", "checks": {"lint": {"cmd": "golangci-lint run"}, "types": {"cmd": "go vet ./..."}, "tests": {"omitir": "lo corre CI"}}}`
	escribirEn(t, root, ".orq/config.json", propio)
	out, errs, code := run(t, "init")
	if code != 0 {
		t.Fatalf("init: %d %s %s", code, out, errs)
	}
	if raw, _ := os.ReadFile(config.RutaProyecto(root)); string(raw) != propio {
		t.Fatalf("init rewrote the Owner's file:\n%s", raw)
	}
	contiene(t, out, "checks ya definidos")
}

func TestInitSumaChecksSinPerderOtrasClaves(t *testing.T) {
	root := repoCLI(t)
	t.Setenv("ORQ_STATE", t.TempDir())
	escribirEn(t, root, "package.json", `{"scripts":{"lint":"sh stubs/lint.sh"}}`)
	escribirEn(t, root, ".orq/config.json", `{"terminal": "orca"}`)
	if out, errs, code := run(t, "init"); code != 0 {
		t.Fatalf("init: %d %s %s", code, out, errs)
	}
	c, err := config.CargarDe(config.RutaProyecto(root))
	if err != nil {
		t.Fatal(err)
	}
	if c.Terminal != "orca" || c.Checks["lint"].Cmd != "npm run lint" || c.Checks["tests"].Omitir == "" {
		t.Fatalf("config = %+v", c)
	}
}
