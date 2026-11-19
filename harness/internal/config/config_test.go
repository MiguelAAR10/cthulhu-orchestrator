package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func escribir(t *testing.T, ruta, contenido string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(ruta), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ruta, []byte(contenido), 0o644); err != nil {
		t.Fatal(err)
	}
	return ruta
}

// Las tablas que vivían en cli/proyecto.go (porRol) y plan/plan.go (Modelo) antes de C1.
func TestDefaultsIgualesAHoy(t *testing.T) {
	t.Setenv("ORQ_ESCRIBA_MODELO", "")
	c, err := CargarDe()
	if err != nil {
		t.Fatal(err)
	}
	if c.Terminal != "herdr" {
		t.Fatalf("terminal = %q", c.Terminal)
	}
	roles := map[string][3]string{
		"orquestador":  {"claude", "sonnet", "medium"},
		"planificador": {"claude", "opus", "high"},
		"astra":        {"claude", "opus", "high"},
		"atena":        {"claude", "opus", "medium"},
	}
	for rol, want := range roles {
		a := c.ParaRol(rol, Agente{})
		if got := [3]string{a.Kind, a.Modelo, a.Esfuerzo}; got != want || len(a.Entorno) != 0 {
			t.Errorf("%s = %+v, antes %+v", rol, got, want)
		}
	}
	workers := []struct{ kind, dif, modelo, esfuerzo string }{
		{"claude", "", "sonnet", "medium"}, {"claude", "baja", "haiku", "medium"}, {"claude", "media", "sonnet", "medium"}, {"claude", "alta", "opus", "high"},
	}
	for _, w := range workers {
		if m, e := c.Worker(w.kind, w.dif); m != w.modelo || e != w.esfuerzo {
			t.Errorf("worker %s/%s = %s %s, antes %s %s", w.kind, w.dif, m, e, w.modelo, w.esfuerzo)
		}
	}
	if c.Rol("escriba").Modelo != "haiku" {
		t.Errorf("escriba = %+v", c.Rol("escriba"))
	}
}

func TestProyectoGanaAlUsuarioCampoPorCampoYGuardaElOrigen(t *testing.T) {
	t.Setenv("ORQ_ESCRIBA_MODELO", "")
	dir := t.TempDir()
	usuario := escribir(t, filepath.Join(dir, "u.json"), `{"terminal":"orca","roles":{"astra":{"modelo":"m-usuario","esfuerzo":"low"}},"recursos":{"reserva_ram_gb":4}}`)
	proyecto := escribir(t, filepath.Join(dir, "p", ".orq", "config.json"), `{"roles":{"astra":{"modelo":"m-proyecto"}},"workers":{"baja":{"claude":{"modelo":"mini"}}}}`)
	c, err := CargarDe(usuario, proyecto)
	if err != nil {
		t.Fatal(err)
	}
	astra := c.Rol("astra")
	if astra.Kind != "claude" || astra.Modelo != "m-proyecto" || astra.Esfuerzo != "low" {
		t.Fatalf("astra = %+v", astra)
	}
	for clave, want := range map[string]string{
		"terminal":                     usuario,
		"roles.astra.agente":           OrigenDefecto,
		"roles.astra.modelo":           proyecto,
		"roles.astra.esfuerzo":         usuario,
		"recursos.reserva_ram_gb":      usuario,
		"recursos.ram_por_agente_gb":   OrigenDefecto,
		"workers.baja.claude.modelo":   proyecto,
		"workers.baja.claude.esfuerzo": OrigenDefecto,
	} {
		if got := c.Origen(clave); got != want {
			t.Errorf("origen de %s = %s, quiero %s", clave, got, want)
		}
	}
	if m, e := c.Worker("claude", "baja"); m != "mini" || e != "medium" {
		t.Errorf("worker baja = %s %s", m, e)
	}
	if c.Terminal != "orca" || c.Recursos.ReservaRAMGB != 4 || c.Recursos.RAMPorAgenteGB != 1.5 {
		t.Errorf("c = %+v", c)
	}
	var visto bool
	for _, l := range c.Lineas() {
		if l.Clave == "roles.astra.modelo" {
			visto = l.Valor == "m-proyecto" && l.Origen == proyecto
		}
	}
	if !visto {
		t.Errorf("Lineas no trae roles.astra.modelo con su origen: %+v", c.Lineas())
	}
}

func TestCampoDesconocidoEsError(t *testing.T) {
	dir := t.TempDir()
	for nombre, cuerpo := range map[string]string{
		"raiz":       `{"terminales":"herdr"}`,
		"en rol":     `{"roles":{"astra":{"model":"x"}}}`,
		"rol":        `{"roles":{"jefe":{"agente":"claude","modelo":"x"}}}`,
		"dificultad": `{"workers":{"extrema":{"claude":{"modelo":"x","esfuerzo":"high"}}}}`,
		"agente":     `{"workers":{"baja":{"gemini":{"modelo":"x","esfuerzo":"high"}}}}`,
		"terminal":   `{"terminal":"tmux"}`,
		"recursos":   `{"recursos":{"ram_por_agente_gb":0}}`,
		"basura":     `{"terminal":"herdr"} {"otro":1}`,
		"no json":    `terminal: herdr`,
	} {
		ruta := escribir(t, filepath.Join(dir, strings.ReplaceAll(nombre, " ", "_")+".json"), cuerpo)
		if _, err := CargarDe(ruta); err == nil {
			t.Errorf("%s: se aceptó %s", nombre, cuerpo)
		} else if !strings.Contains(err.Error(), "config:") {
			t.Errorf("%s: el error no dice que es de config: %v", nombre, err)
		}
	}
}

func TestSinArchivosValeElDefectoYListaLasCapas(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ORQ_CONFIG", filepath.Join(dir, "no-existe.json"))
	c, err := Cargar(filepath.Join(dir, "repo"))
	if err != nil {
		t.Fatal(err)
	}
	as := c.Archivos()
	if len(as) != 2 || as[0].Existe || as[1].Existe || as[1].Ruta != filepath.Join(dir, "repo", ".orq", "config.json") {
		t.Fatalf("archivos = %+v", as)
	}
	for _, l := range c.Lineas() {
		if l.Origen != OrigenDefecto {
			t.Errorf("%s viene de %s sin archivos", l.Clave, l.Origen)
		}
	}
}

func TestVariableDelEscribaMandaYSeVe(t *testing.T) {
	t.Setenv("ORQ_ESCRIBA_MODELO", "otro")
	c, err := CargarDe()
	if err != nil {
		t.Fatal(err)
	}
	if c.Rol("escriba").Modelo != "otro" || c.Origen("roles.escriba.modelo") != "ORQ_ESCRIBA_MODELO" {
		t.Fatalf("escriba = %+v de %s", c.Rol("escriba"), c.Origen("roles.escriba.modelo"))
	}
}

func TestExpandirGuias(t *testing.T) {
	t.Setenv("ORQ_ESCRIBA_MODELO", "")
	c, _ := CargarDe()
	got := c.Expandir("plan con {{rol:planificador}}; astra {{rol:astra}}; {{workers}}")
	for _, want := range []string{"plan con opus", "astra opus", "claude baja=haiku/medium"} {
		if !strings.Contains(got, want) {
			t.Errorf("falta %q en %q", want, got)
		}
	}
	if strings.Contains(got, "{{") {
		t.Errorf("quedaron marcas: %q", got)
	}
}

// Dos cuentas de Claude: cada rol y cada dificultad de worker puede nombrar la suya.
func TestCuentaPorRolYPorDificultad(t *testing.T) {
	t.Setenv("ORQ_ESCRIBA_MODELO", "")
	home := t.TempDir()
	t.Setenv("HOME", home)
	ruta := escribir(t, filepath.Join(t.TempDir(), "c.json"), `{
	  "roles": {"atena": {"entorno": {"CLAUDE_CONFIG_DIR": "~/.claude-alt"}},
	            "planificador": {"entorno": {"CLAUDE_CONFIG_DIR": "/cuentas/principal"}},
	            "astra": {"entorno": {"CLAUDE_CONFIG_DIR": "/cuentas/astra"}}},
	  "workers": {"alta": {"claude": {"entorno": {"CLAUDE_CONFIG_DIR": "~/.claude-alt"}}}}}`)
	c, err := CargarDe(ruta)
	if err != nil {
		t.Fatal(err)
	}
	atena, plan := c.ParaRol("atena", Agente{}), c.ParaRol("planificador", Agente{})
	if atena.Entorno["CLAUDE_CONFIG_DIR"] != filepath.Join(home, ".claude-alt") || plan.Entorno["CLAUDE_CONFIG_DIR"] != "/cuentas/principal" {
		t.Fatalf("atena %v · planificador %v", atena.Entorno, plan.Entorno)
	}
	if atena.Entorno["CLAUDE_CONFIG_DIR"] == plan.Entorno["CLAUDE_CONFIG_DIR"] {
		t.Fatal("dos roles con cuentas distintas recibieron la misma")
	}
	if len(c.ParaRol("orquestador", Agente{}).Entorno) != 0 {
		t.Fatal("un rol sin entorno configurado no recibe variables")
	}
	if got := c.WorkerEntorno("claude", "alta")["CLAUDE_CONFIG_DIR"]; got != filepath.Join(home, ".claude-alt") {
		t.Fatalf("worker alta = %q", got)
	}
	if len(c.WorkerEntorno("claude", "")) != 0 {
		t.Fatal("la cuenta de una dificultad no se filtra a otra")
	}
	if got := c.ParaRol("astra", Agente{}).Entorno["CLAUDE_CONFIG_DIR"]; got != "/cuentas/astra" {
		t.Fatalf("astra = %q", got)
	}
	if c.Origen("roles.atena.entorno.CLAUDE_CONFIG_DIR") != ruta {
		t.Fatalf("origen = %s", c.Origen("roles.atena.entorno.CLAUDE_CONFIG_DIR"))
	}
	if Cuenta(nil) != "la del entorno de quien lanza" || Cuenta(plan.Entorno) != "CLAUDE_CONFIG_DIR=/cuentas/principal" {
		t.Fatalf("%q %q", Cuenta(nil), Cuenta(plan.Entorno))
	}
}

func TestEntornoSoloRutasNuncaSecretos(t *testing.T) {
	dir := t.TempDir()
	for nombre, cuerpo := range map[string]string{
		"token":    `{"roles":{"atena":{"entorno":{"CLAUDE_CODE_OAUTH_TOKEN":"/x"}}}}`,
		"api key":  `{"roles":{"astra":{"entorno":{"OPENAI_API_KEY":"/x"}}}}`,
		"no ruta":  `{"roles":{"atena":{"entorno":{"CLAUDE_CONFIG_DIR":"sk-ant-abc"}}}}`,
		"path":     `{"roles":{"atena":{"entorno":{"PATH":"/x"}}}}`,
		"orq":      `{"workers":{"baja":{"claude":{"entorno":{"ORQ_ROL":"/x"}}}}}`,
		"minusc":   `{"roles":{"atena":{"entorno":{"claude_dir":"/x"}}}}`,
		"relativa": `{"roles":{"atena":{"entorno":{"CODEX_HOME":"./codex"}}}}`,
	} {
		ruta := escribir(t, filepath.Join(dir, strings.ReplaceAll(nombre, " ", "_")+".json"), cuerpo)
		if _, err := CargarDe(ruta); err == nil {
			t.Errorf("%s: se aceptó %s", nombre, cuerpo)
		}
	}
}

// Review 2: the default init command is an npm script. When the script is a bare `playwright test`,
// --forbid-only reaches it through the package manager.
func TestSinOnlyEnScripts(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"scripts":{"test:e2e":"playwright test","test":"npx playwright test --reporter=junit","e2e:ci":"playwright test && echo done","lint":"eslint ."}}`), 0o644)
	casos := map[string]string{
		"npm run test:e2e":                   "npm run test:e2e -- --forbid-only",
		"npm run test:e2e -- --grep x":       "npm run test:e2e -- --grep x --forbid-only",
		"npm test":                           "npm test -- --forbid-only",
		"npm run-script test:e2e && echo ok": "npm run-script test:e2e -- --forbid-only && echo ok",
		"pnpm test:e2e":                      "pnpm test:e2e --forbid-only",
		"pnpm run test:e2e":                  "pnpm run test:e2e --forbid-only",
		"yarn test:e2e":                      "yarn test:e2e --forbid-only",
		"npm run e2e:ci":                     "npm run e2e:ci", // compound: the flag would reach echo
		"npm run lint":                       "npm run lint",
		"npm install":                        "npm install",
		"npm run test:e2e -- --forbid-only":  "npm run test:e2e -- --forbid-only",
		"npx playwright test":                "npx playwright test --forbid-only",
	}
	for in, want := range casos {
		if got := SinOnlyEn(dir, in); got != want {
			t.Errorf("SinOnlyEn(%q) = %q, want %q", in, got, want)
		}
	}
	if got := SinOnlyEn(t.TempDir(), "npm run test:e2e"); got != "npm run test:e2e" {
		t.Errorf("without package.json: %q", got)
	}
}

func TestConfigSkillsMinimo(t *testing.T) {
	dir := t.TempDir()
	c, err := CargarDe(escribir(t, filepath.Join(dir, "a.json"), `{"skills":{"minimo":0.5}}`))
	if err != nil || c.Skills.Minimo == nil || *c.Skills.Minimo != 0.5 {
		t.Fatalf("%+v %v", c.Skills, err)
	}
	if c, _ := CargarDe(); c.Skills.Minimo != nil {
		t.Fatal("por defecto no hay mínimo")
	}
	if _, err := CargarDe(escribir(t, filepath.Join(dir, "b.json"), `{"skills":{"minimo":1.5}}`)); err == nil || !strings.Contains(err.Error(), "skills.minimo debe estar entre 0 y 1") {
		t.Fatalf("%v", err)
	}
}
