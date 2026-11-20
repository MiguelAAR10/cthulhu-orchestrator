package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"orq/internal/config"
)

// orq2's orchestrator guide does not assume Astra runs on Codex: the role config decides.
// orq's guide stays byte-identical.
func TestGuiaOrquestadorAstraSinCodex(t *testing.T) {
	root := repoCLI(t)
	t.Setenv("ORQ_STATE", t.TempDir())
	t.Setenv("ORQ_ROOT", root)
	os.MkdirAll(filepath.Join(root, ".orq"), 0o755)
	os.WriteFile(filepath.Join(root, ".orq", "config.json"), []byte(`{"roles":{"astra":{"agente":"claude","modelo":"m-astra"}}}`), 0o644)
	raw, _ := guias.ReadFile("guias/orquestador.md")
	cfg, err := config.Cargar(root)
	if err != nil {
		t.Fatal(err)
	}
	out, _, code := run(t, "skill", "orquestador")
	if code != 0 || !strings.HasPrefix(out, cfg.Expandir(string(raw))) {
		t.Fatalf("orq guide changed (code %d):\n%s", code, out)
	}
	t.Setenv("ORQ_NOMBRE", "orq2")
	out, _, code = run(t, "skill", "orquestador")
	if code != 0 || strings.Contains(out, "Codex quota") {
		t.Fatalf("orq2 guide still assumes Codex for Astra (code %d):\n%s", code, out)
	}
	contiene(t, out, "orq2 sesion abrir astra` (claude m-astra: the agent and model the astra role sets in `orq2 config ver`)")
}
