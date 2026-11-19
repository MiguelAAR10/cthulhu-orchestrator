package config

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestDefectoSinCodex(t *testing.T) {
	raw, err := json.Marshal(Defecto())
	if err != nil {
		t.Fatal(err)
	}
	if s := string(raw); strings.Contains(s, "codex") || strings.Contains(s, "gpt-") {
		t.Fatalf("defaults still name codex: %s", s)
	}
	if k := Defecto().Roles["astra"].Kind; k != "claude" {
		t.Fatalf("astra kind = %q", k)
	}
}

func TestCodexEnArchivoSeRechaza(t *testing.T) {
	t.Setenv("ORQ_ESCRIBA_MODELO", "")
	for nombre, cuerpo := range map[string]string{
		"rol":    `{"roles":{"astra":{"agente":"codex","modelo":"x"}}}`,
		"worker": `{"workers":{"alta":{"codex":{"modelo":"x","esfuerzo":"high"}}}}`,
	} {
		ruta := escribir(t, filepath.Join(t.TempDir(), "u.json"), cuerpo)
		_, err := CargarDe(ruta)
		if err == nil || !strings.Contains(err.Error(), "regla del Owner") {
			t.Errorf("%s: err = %v", nombre, err)
		}
	}
	if err := ValidarKind("codex"); err == nil || !strings.Contains(err.Error(), "regla del Owner") {
		t.Errorf("ValidarKind(codex) = %v", err)
	}
	if err := ValidarKind("claude"); err != nil {
		t.Errorf("ValidarKind(claude) = %v", err)
	}
}

func TestChecksDelProyecto(t *testing.T) {
	t.Setenv("ORQ_ESCRIBA_MODELO", "")
	ruta := escribir(t, filepath.Join(t.TempDir(), ".orq", "config.json"), `{"checks":{
		"zz":{"cmd":"true"},"tests":{"omitir":"sin script test"},"aa":{"cmd":"! grep -r TODO ."},
		"lint":{"cmd":"npm run lint"},"types":{"cmd":"npm run typecheck"}}}`)
	c, err := CargarDe(ruta)
	if err != nil {
		t.Fatal(err)
	}
	cs := c.ChecksEnOrden()
	var nombres []string
	for _, ch := range cs {
		nombres = append(nombres, ch.Nombre)
	}
	if !slices.Equal(nombres, []string{"lint", "types", "tests", "aa", "supresiones", "zz"}) {
		t.Fatalf("order = %v", nombres)
	}
	if cs[0].Cmd != "npm run lint" || cs[2].Omitir != "sin script test" || cs[2].Cmd != "" || cs[3].Cmd != "! grep -r TODO ." {
		t.Fatalf("checks = %+v", cs)
	}
	if len(Defecto().ChecksEnOrden()) != 0 {
		t.Fatal("defaults carry checks")
	}
}

func TestChecksInvalidos(t *testing.T) {
	t.Setenv("ORQ_ESCRIBA_MODELO", "")
	base := `"lint":{"cmd":"a"},"types":{"cmd":"b"}`
	for cuerpo, want := range map[string]string{
		`{"checks":{` + base + `}}`:                                                   "checks.tests",
		`{"checks":{` + base + `,"tests":{"cmd":"c","omitir":"x"}}}`:                  "checks.tests",
		`{"checks":{` + base + `,"tests":{}}}`:                                        "checks.tests",
		`{"checks":{` + base + `,"tests":{"cmd":"c"},"Bad Name":{"cmd":"d"}}}`:        "Bad Name",
		`{"checks":{` + base + `,"tests":{"cmd":"c"},"supresiones":{"cmd":"d"}}}`:     "checks.supresiones",
		`{"checks":{` + base + `,"tests":{"cmd":"c"},"definiciones":{"omitir":"x"}}}`: "checks.definiciones",
		`{"checks":{` + base + `,"tests":{"cmd":"c","patrones":["x"]}}}`:              "checks.tests",
	} {
		ruta := escribir(t, filepath.Join(t.TempDir(), "p.json"), cuerpo)
		if _, err := CargarDe(ruta); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: err = %v, want %q", cuerpo, err, want)
		}
	}
}

// The suppressions guardrail is on whenever there is a checks block; the project extends or turns it off.
func TestSupresionesYDefiniciones(t *testing.T) {
	t.Setenv("ORQ_ESCRIBA_MODELO", "")
	base := `"lint":{"cmd":"a"},"types":{"cmd":"b"},"tests":{"cmd":"c"}`
	cargar := func(extra string) Config {
		t.Helper()
		c, err := CargarDe(escribir(t, filepath.Join(t.TempDir(), "p.json"), `{"checks":{`+base+extra+`}}`))
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	cs := cargar("").ChecksEnOrden()
	if len(cs) != 4 || cs[3].Nombre != Supresiones || !cs[3].Interno() || cs[3].Omitir != "" {
		t.Fatalf("default: %+v", cs)
	}
	cs = cargar(`,"supresiones":{"patrones":["NOSONAR"]},"definiciones":{"archivos":["Makefile"]}`).ChecksEnOrden()
	if len(cs) != 4 || !slices.Contains(cs[3].Marcas(), "NOSONAR") || !slices.Contains(cs[3].Marcas(), "//nolint") {
		t.Fatalf("extended: %+v", cs)
	}
	if got := cargar(`,"definiciones":{"archivos":["Makefile"]}`).ArchivosDefinicion(); !slices.Equal(got, []string{"Makefile"}) {
		t.Fatalf("definiciones = %v", got)
	}
	if cs := cargar(`,"supresiones":{"omitir":"legacy"}`).ChecksEnOrden(); cs[3].Omitir != "legacy" {
		t.Fatalf("off: %+v", cs)
	}
}

func TestE2EServidorValido(t *testing.T) {
	t.Setenv("ORQ_ESCRIBA_MODELO", "")
	base := `"lint":{"cmd":"a"},"types":{"cmd":"b"},"tests":{"cmd":"c"}`
	for extra, want := range map[string]string{
		`,"e2e":{"cmd":"npx playwright test","servidor":"npm run dev"}`:              "checks.e2e",
		`,"e2e":{"cmd":"npx playwright test","servidor":"npm run dev","puerto":0}`:   "checks.e2e",
		`,"e2e":{"cmd":"npx playwright test","puerto":3000}`:                         "checks.e2e",
		`,"e2e":{"cmd":"npx playwright test","servidor":"x","puerto":70000}`:         "checks.e2e",
		`,"lint2":{"cmd":"npx playwright test","servidor":"npm run dev","puerto":1}`: "checks.lint2",
	} {
		if _, err := CargarDe(escribir(t, filepath.Join(t.TempDir(), "p.json"), `{"checks":{`+base+extra+`}}`)); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: err = %v, want %q", extra, err, want)
		}
	}
	c, err := CargarDe(escribir(t, filepath.Join(t.TempDir(), "p.json"), `{"checks":{`+base+`,"e2e":{"cmd":"npx playwright test","servidor":"npm run dev","puerto":3000}}}`))
	if err != nil || c.Checks[E2E].Puerto != 3000 || c.Checks[E2E].Servidor != "npm run dev" {
		t.Fatalf("valid e2e: %v %+v", err, c.Checks)
	}
}

func TestEjecutableSinServidor(t *testing.T) {
	ch := Check{Nombre: E2E, Cmd: "npx playwright test"}
	if ch.Ejecutable() != "npx playwright test --forbid-only" {
		t.Fatalf("got %q", ch.Ejecutable())
	}
	ch.Servidor, ch.Puerto = "npm run dev", 3000
	if x := ch.Ejecutable(); !strings.Contains(x, "setsid sh -c 'npm run dev'") || !strings.Contains(x, "/dev/tcp/127.0.0.1/3000") || !strings.HasSuffix(x, "\nnpx playwright test --forbid-only\n") {
		t.Fatalf("got %s", x)
	}
}

// checks.e2e.espera_s sets how long the server may take to open its port (default 60 s).
func TestEsperaServidor(t *testing.T) {
	ch := Check{Nombre: E2E, Cmd: "npx playwright test", Servidor: "npm run dev", Puerto: 3000}
	if x := ch.Ejecutable(); !strings.Contains(x, "-gt 240 ]") {
		t.Fatalf("default wait is not 60 s:\n%s", x)
	}
	ch.EsperaS = 180
	if x := ch.Ejecutable(); !strings.Contains(x, "-gt 720 ]") || !strings.Contains(x, "en 180 s") {
		t.Fatalf("espera_s 180:\n%s", x)
	}
	base := `"lint":{"cmd":"x"},"types":{"cmd":"x"},"tests":{"cmd":"x"}`
	for _, extra := range []string{`,"e2e":{"cmd":"x","espera_s":30}`, `,"e2e":{"cmd":"x","servidor":"s","puerto":1,"espera_s":-1}`,
		`,"e2e":{"cmd":"x","servidor":"s","puerto":1,"espera_s":601}`, `,"lint2":{"cmd":"x","espera_s":5}`} {
		if _, err := CargarDe(escribir(t, filepath.Join(t.TempDir(), "p.json"), `{"checks":{`+base+extra+`}}`)); err == nil || !strings.Contains(err.Error(), "espera_s") {
			t.Errorf("%s: err = %v", extra, err)
		}
	}
	c, err := CargarDe(escribir(t, filepath.Join(t.TempDir(), "p.json"), `{"checks":{`+base+`,"e2e":{"cmd":"x","servidor":"s","puerto":1,"espera_s":120}}}`))
	if err != nil || c.Checks[E2E].EsperaS != 120 {
		t.Fatalf("%v %+v", err, c.Checks)
	}
}
