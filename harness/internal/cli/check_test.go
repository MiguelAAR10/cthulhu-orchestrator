package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"orq/internal/config"
	"orq/internal/slice"
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
	if len(cs) != 4 || cs[3].Nombre != "supresiones" || !strings.HasPrefix(cs[0].Cmd, "out=$(gofmt -l . 2>&1)") || cs[1].Cmd != "go vet ./..." || cs[2].Cmd != "go test ./..." {
		t.Fatalf("checks = %+v", cs)
	}
	contiene(t, out, "check lint: out=$(gofmt -l . 2>&1)", "check types: go vet ./...", "check tests: go test ./...", "check supresiones: "+slice.CmdSupresiones)
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
	contiene(t, out, "lint  paso", "4 pasan · 0 fallan · 0 omitidos")
	// The suppressions guardrail runs in the worker's loop too, on what the clone adds since the red.
	escribirEn(t, wt, "a.ts", "let a = 1\n// @ts-ignore\n")
	out, _, code = run(t, "check")
	if code == 0 {
		t.Fatalf("suppression passed:\n%s", out)
	}
	contiene(t, out, "supresiones  fallo", "a.ts:2: // @ts-ignore")
}

func TestGuiaWorkerPideOrqCheck(t *testing.T) {
	raw, err := guias.ReadFile("guias/worker.md")
	if err != nil {
		t.Fatal(err)
	}
	contiene(t, string(raw), "`orq check`", "until it passes", "Before REQUEST VERIFICATION")
}

// orq check --caja says per check whether the box can run it, with what it binds; nothing runs.
func TestCheckCajaInforma(t *testing.T) {
	root := repoCLI(t)
	t.Setenv("ORQ_STATE", t.TempDir())
	home := filepath.Join(t.TempDir(), "home")
	real := filepath.Join(home, ".local", "lib", "fake", "bin", "faketool")
	os.MkdirAll(filepath.Dir(real), 0o755)
	os.WriteFile(real, []byte("#!/bin/sh\nexit 0\n"), 0o755)
	os.MkdirAll(filepath.Join(home, ".config", "x"), 0o755)
	os.WriteFile(filepath.Join(home, ".config", "x", "cfgtool"), []byte("#!/bin/sh\nexit 0\n"), 0o755)
	t.Setenv("HOME", home)
	t.Setenv("PATH", filepath.Dir(real)+":"+filepath.Join(home, ".config", "x")+":"+os.Getenv("PATH"))
	escribirEn(t, root, ".orq/config.json", `{"checks": {"lint": {"cmd": "faketool -l ."}, "types": {"cmd": "cfgtool"}, "tests": {"cmd": "nosuchtool run"}}}`)
	t.Chdir(root)
	out, _, code := run(t, "check", "--caja")
	if code != 1 {
		t.Fatalf("exit %d:\n%s", code, out)
	}
	contiene(t, out, "lint  corre en la caja · monta "+filepath.Join(home, ".local", "lib", "fake"),
		"types  no corre en la caja: cfgtool está en "+filepath.Join(home, ".config", "x")+", que nunca entra en la caja",
		"tests  no corre en la caja: nosuchtool no está en el PATH del host")
}
