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
	if !slices.Equal(nombres, []string{"lint", "types", "tests", "aa", "zz"}) {
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
		`{"checks":{` + base + `}}`:                                            "checks.tests",
		`{"checks":{` + base + `,"tests":{"cmd":"c","omitir":"x"}}}`:           "checks.tests",
		`{"checks":{` + base + `,"tests":{}}}`:                                 "checks.tests",
		`{"checks":{` + base + `,"tests":{"cmd":"c"},"Bad Name":{"cmd":"d"}}}`: "Bad Name",
	} {
		ruta := escribir(t, filepath.Join(t.TempDir(), "p.json"), cuerpo)
		if _, err := CargarDe(ruta); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: err = %v, want %q", cuerpo, err, want)
		}
	}
}
