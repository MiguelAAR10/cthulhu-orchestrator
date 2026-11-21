package config

import (
	"path/filepath"
	"strings"
	"testing"
)

const raiz = `"checks":{"lint":{"cmd":"ruff"},"types":{"cmd":"mypy"},"tests":{"cmd":"pytest"}}`

func cargarCarpetas(t *testing.T, cuerpo string) (Config, error) {
	t.Helper()
	t.Setenv("ORQ_ESCRIBA_MODELO", "")
	t.Setenv("ORQ_NOMBRE", "orq2")
	return CargarDe(escribir(t, filepath.Join(t.TempDir(), "p.json"), "{"+cuerpo+"}"))
}

func TestCarpetasValidas(t *testing.T) {
	c, err := cargarCarpetas(t, raiz+`,"carpetas":{"dashboard":{"checks":{"lint":{"cmd":"pnpm lint"},"types":{"cmd":"pnpm typecheck"},"tests":{"cmd":"pnpm test"},"build":{"cmd":"pnpm build"}}},"docs":{"exenta":"solo texto"}}`)
	if err != nil {
		t.Fatal(err)
	}
	cs := c.CarpetasEnOrden()
	if len(cs) != 2 || cs[0].Ruta != "dashboard" || cs[1].Ruta != "docs" || cs[1].Exenta != "solo texto" || len(cs[1].Lista) != 0 {
		t.Fatalf("carpetas = %+v", cs)
	}
	var nombres []string
	for _, ch := range cs[0].Lista {
		nombres = append(nombres, ch.Nombre)
	}
	if strings.Join(nombres, ",") != "lint,types,tests,build" {
		t.Fatalf("dashboard checks = %v (no supresiones: orq runs that guard once, on the whole diff)", nombres)
	}
	if len(c.ChecksEnOrden()) != 4 {
		t.Fatalf("root checks changed: %+v", c.ChecksEnOrden())
	}
}

func TestCarpetasInvalidas(t *testing.T) {
	tres := `"lint":{"cmd":"a"},"types":{"cmd":"b"},"tests":{"cmd":"c"}`
	for cuerpo, want := range map[string]string{
		`"carpetas":{"dashboard":{}}`:                                                             `carpetas.dashboard`,
		`"carpetas":{"dashboard":{"exenta":"x","checks":{` + tres + `}}}`:                         `carpetas.dashboard`,
		`"carpetas":{"dashboard":{"checks":{"lint":{"cmd":"a"},"types":{"cmd":"b"}}}}`:            `carpetas.dashboard.checks.tests`,
		`"carpetas":{"../fuera":{"exenta":"x"}}`:                                                  `carpetas.../fuera`,
		`"carpetas":{"/abs":{"exenta":"x"}}`:                                                      `carpetas./abs`,
		`"carpetas":{"dashboard/":{"exenta":"x"}}`:                                                `carpetas.dashboard/`,
		`"carpetas":{"dash*":{"exenta":"x"}}`:                                                     `carpetas.dash*`,
		`"carpetas":{".":{"checks":{` + tres + `}}}`:                                              `carpetas..`,
		`"carpetas":{"dashboard":{"checks":{` + tres + `,"supresiones":{"omitir":"x"}}}}`:         `carpetas.dashboard.checks.supresiones`,
		`"carpetas":{"dashboard":{"checks":{` + tres + `,"e2e":{"cmd":"x"}}}}`:                    `carpetas.dashboard.checks.e2e`,
		`"carpetas":{"dashboard":{"checks":{"lint":{"cmd":"a"},"types":{"cmd":"b"},"tests":{}}}}`: `carpetas.dashboard.checks.tests`,
	} {
		if _, err := cargarCarpetas(t, cuerpo); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: err = %v, want %q", cuerpo, err, want)
		}
	}
	if c, err := cargarCarpetas(t, `"carpetas":{".":{"exenta":"repo de solo documentación"}}`); err != nil || c.CarpetasEnOrden()[0].Ruta != "." {
		t.Fatalf("root exemption: %v", err)
	}
}

// The flat config keeps loading exactly as before: no carpetas, same checks.
func TestConfigPlanaSinCarpetas(t *testing.T) {
	c, err := cargarCarpetas(t, raiz)
	if err != nil || len(c.CarpetasEnOrden()) != 0 || len(c.ChecksEnOrden()) != 4 {
		t.Fatalf("%v %+v", err, c)
	}
}

// M2: under identity orq the carpetas key is refused exactly as orq 1.0.0 refuses an unknown field.
func TestCarpetasRechazadaComoOrq(t *testing.T) {
	t.Setenv("ORQ_ESCRIBA_MODELO", "")
	t.Setenv("ORQ_NOMBRE", "orq")
	_, err := CargarDe(escribir(t, filepath.Join(t.TempDir(), "p.json"), `{`+raiz+`,"carpetas":{"docs":{"exenta":"x"}}}`))
	if err == nil || !strings.Contains(err.Error(), `json: unknown field "carpetas"`) {
		t.Fatalf("orq accepted carpetas: %v", err)
	}
	if _, err := CargarDe(escribir(t, filepath.Join(t.TempDir(), "p.json"), `{`+raiz+`}`)); err != nil {
		t.Fatalf("flat config under orq: %v", err)
	}
}

// Under orq the schema check still names the struct Config in JSON type errors, as 1.0.0 did.
func TestErrorDeTipoComoOrq(t *testing.T) {
	t.Setenv("ORQ_ESCRIBA_MODELO", "")
	for _, n := range []string{"orq", "orq2"} {
		t.Setenv("ORQ_NOMBRE", n)
		_, err := CargarDe(escribir(t, filepath.Join(t.TempDir(), "p.json"), `{"terminal": 3}`))
		if err == nil || !strings.Contains(err.Error(), "Go struct field Config.terminal of type string") {
			t.Fatalf("%s: err = %v", n, err)
		}
	}
}
