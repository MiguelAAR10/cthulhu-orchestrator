package agentes

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func cargar(t *testing.T, global, propuesta, proyecto string) (*Config, error) {
	t.Helper()
	d := t.TempDir()
	esc := func(n, s string) string {
		if s == "" {
			return filepath.Join(d, "no-"+n)
		}
		p := filepath.Join(d, n)
		os.WriteFile(p, []byte(s), 0o644)
		return p
	}
	return Load(esc("global.json", global), esc("propuesta.json", propuesta), esc("proyecto.json", proyecto))
}

func TestBuiltinMantieneLosDefaultsDeSiempre(t *testing.T) {
	c, err := cargar(t, "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, x := range []struct{ rol, ag, mod, esf string }{
		{"main", "claude", "sonnet", "medium"}, {"planner", "claude", "opus", "high"},
		{"adversary", "codex", "gpt-6-astra", "high"}, {"atena", "claude", "opus", "medium"},
	} {
		if e := c.Rol(x.rol); e.Agent != x.ag || e.Model != x.mod || e.Effort != x.esf {
			t.Fatalf("%s: %+v", x.rol, e)
		}
	}
	for _, x := range []struct{ ag, risk, mod, esf string }{
		{"", "low", "haiku", "medium"}, {"claude", "medium", "sonnet", "medium"}, {"claude", "high", "opus", "high"},
		{"codex", "low", "gpt-6-luna", "medium"}, {"codex", "high", "gpt-6.1-sol", "high"}, {"codex", "", "gpt-6.1-sol", "medium"},
	} {
		if e, err := c.Worker(x.ag, x.risk); err != nil || e.Model != x.mod || e.Effort != x.esf {
			t.Fatalf("worker %s/%s: %+v %v", x.ag, x.risk, e, err)
		}
	}
	if _, err := c.Worker("gpt-pirata", "low"); err == nil {
		t.Fatal("agente desconocido debe fallar")
	}
}

func TestProyectoGanaYCambiaSoloLoQueDice(t *testing.T) {
	c, err := cargar(t, `{"roles":{"main":{"model":"opus"}}}`, `{"roles":{"planner":{"agent":"codex","model":"gpt-6.1-sol"},"adversary":{"agent":"gemini","model":"gemini-pro"}}}`,
		`{"roles":{"planner":{"agent":"claude","model":"opus"}}}`)
	if err != nil {
		t.Fatal(err)
	}
	if e := c.Rol("main"); e.Agent != "claude" || e.Model != "opus" || e.Effort != "medium" {
		t.Fatalf("global sobre builtin: %+v", e)
	}
	if e := c.Rol("planner"); e.Agent != "claude" {
		t.Fatalf("el Owner gana a la propuesta: %+v", e)
	}
}

func TestAdversarioDeOtraFamilia(t *testing.T) {
	_, err := cargar(t, "", "", `{"roles":{"adversary":{"agent":"claude","model":"opus"}}}`)
	if err == nil || !strings.Contains(err.Error(), "familia") {
		t.Fatalf("planner y adversary claude: %v", err)
	}
	c, _ := cargar(t, "", "", "")
	if err := c.Set("adversary", "claude", "sonnet"); err == nil {
		t.Fatal("set debe aplicar la regla")
	}
	if err := c.Set("adversary", "gemini", "gemini-3-pro"); err != nil {
		t.Fatal(err)
	}
}

func TestVerificadorFrescoYDocsBarato(t *testing.T) {
	c, _ := cargar(t, "", "", `{"roles":{"verifier":{"agent":"codex","model":"gpt-6.1-sol","fresh":false},"docs":{"agent":"claude","model":"opus"}}}`)
	if !c.Rol("verifier").Fresh {
		t.Fatal("el verificador siempre arranca en sesión nueva")
	}
	if m := c.Rol("docs").Model; m != "haiku" {
		t.Fatalf("docs usa el modelo más barato: %s", m)
	}
}

func TestArgsDesdeDatos(t *testing.T) {
	c, _ := cargar(t, "", "", "")
	l, err := c.Launch(Eleccion{Agent: "codex", Model: "gpt-6-luna", Effort: "medium"}, "w", Contexto{GitCommon: "/g/.git"}, "worker")
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range []string{"-m", "gpt-6-luna", "model_reasoning_effort=medium", "workspace-write", "on-request", "sandbox_workspace_write.network_access=true", "/g/.git"} {
		if !slices.Contains(l.Args, w) {
			t.Fatalf("falta %q en %v", w, l.Args)
		}
	}
	if l.Kind != "codex" || l.CLI != "codex" {
		t.Fatalf("%+v", l)
	}
	l, _ = c.Launch(Eleccion{Agent: "codex", Model: "m", Effort: "e"}, "w", Contexto{}, "worker")
	if slices.Contains(l.Args, "--add-dir") {
		t.Fatalf("sin git común no va --add-dir: %v", l.Args)
	}
	l, _ = c.Launch(Eleccion{Agent: "claude", Model: "opus", Effort: "high"}, "w", Contexto{}, "worker")
	if strings.Join(l.Args, " ") != "--model opus --effort high -n w" || slices.Contains(l.Args, "workspace-write") {
		t.Fatalf("claude: %v", l.Args)
	}
	if !c.Agents["claude"].TieneHooks() || c.Agents["codex"].TieneHooks() {
		t.Fatal("claude tiene hooks; codex no")
	}
}

func TestDetectPropone(t *testing.T) {
	c, _ := cargar(t, "", "", "")
	hay := map[string]bool{"claude": true, "gemini": true}
	p := c.Detect(func(cli string) bool { return hay[cli] })
	if !slices.Equal(p.Instalados, []string{"claude", "gemini"}) {
		t.Fatalf("%v", p.Instalados)
	}
	if p.Roles["adversary"].Agent != "gemini" {
		t.Fatalf("adversario de otra familia instalada: %+v", p.Roles)
	}
}

func TestPlantillaDelRepoEsValida(t *testing.T) {
	if _, err := cargar(t, "", "", mustRead(t, "../../../templates/orq.agents.json")); err != nil {
		t.Fatal(err)
	}
}

func mustRead(t *testing.T, p string) string {
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
