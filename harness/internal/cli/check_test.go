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

const checksMixtos = `{"checks": {"lint": {"cmd": "echo todo limpio"}, "types": {"cmd": "echo 'a.go:3: undefined: x'; exit 1"}, "tests": {"omitir": "sin script test"}}}`

func TestCheckImprimeCadaResultado(t *testing.T) {
	root := repoCLI(t)
	escribirEn(t, root, ".orq/config.json", checksMixtos)
	t.Chdir(root)
	out, _, code := run(t, "check")
	if code != 1 {
		t.Fatalf("exit %d:\n%s", code, out)
	}
	contiene(t, out, "lint  paso  · echo todo limpio", "types  fallo (exit 1)", "a.go:3: undefined: x",
		"tests  omitido: sin script test", "1 pasan · 1 fallan · 1 omitidos (omitido no cuenta como pasado)")
}

func TestCheckSinBloque(t *testing.T) {
	root := repoCLI(t)
	t.Chdir(root)
	_, errs, code := run(t, "check")
	if code != 1 {
		t.Fatalf("exit %d", code)
	}
	contiene(t, errs, "sin checks", "orq init")
}

func TestCheckEnClonLeeConfigDelProyecto(t *testing.T) {
	root := repoCLI(t)
	t.Setenv("ORQ_STATE", t.TempDir())
	if _, errs, code := run(t, append(append([]string{}, abrirBase...), "--reporte", "out.tap", "--formato", "tap")...); code != 0 {
		t.Fatalf("abrir: %d %s", code, errs)
	}
	run(t, "slice", "tomar", "w1")
	wt := filepath.Join(filepath.Dir(root), "wt", "tienda-s01")
	escribirEn(t, wt, "solo-en-clon", "x")
	escribirEn(t, root, ".orq/config.json", `{"checks": {"lint": {"cmd": "test -f solo-en-clon"}, "types": {"cmd": "true"}, "tests": {"cmd": "true"}}}`)
	t.Setenv("ORQ_ROOT", "")
	t.Chdir(wt)
	out, errs, code := run(t, "check")
	if code != 0 {
		t.Fatalf("exit %d:\n%s%s", code, out, errs)
	}
	contiene(t, out, "lint  paso", "3 pasan · 0 fallan · 0 omitidos")
}

func TestGuiaWorkerPideOrqCheck(t *testing.T) {
	raw, err := guias.ReadFile("guias/worker.md")
	if err != nil {
		t.Fatal(err)
	}
	contiene(t, string(raw), "`orq check`", "until it passes", "Before REQUEST VERIFICATION")
}
