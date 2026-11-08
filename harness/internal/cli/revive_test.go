package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"orq/internal/workers"
)

// Un proceso dentro de un worktree enlazado es un worker aunque no tenga ORQ_WORKER_ID ni rol.
func TestWorktreeSinEnvNoActuaComoOwner(t *testing.T) {
	root := proyectoVis(t)
	t.Setenv("ORQ_ROL", "atena")
	run(t, "stop", "raise", "--type", "decision", "--severity", "soon", "--slice", "S-04", "--text", "pick a or b", "--options", "a|b")
	t.Setenv("ORQ_ROL", "")
	wt := filepath.Join(t.TempDir(), "wt")
	os.MkdirAll(wt, 0o755)
	os.WriteFile(filepath.Join(wt, ".git"), []byte("gitdir: "+root+"/.git/worktrees/wt\n"), 0o644)
	t.Chdir(wt)
	if _, _, code := run(t, "stop", "resolve", "ST-0001", "--choice", "a"); code == 0 {
		t.Fatal("a worktree resolved a stopper")
	}
	if _, _, code := run(t, "autonomy", "free", "--scope", "src/**"); code == 0 {
		t.Fatal("a worktree switched autonomy to free")
	}
	if strings.Contains(stopsFile(root), "resolved") {
		t.Fatal(stopsFile(root))
	}
}

func TestLanzarDejaEstadoDelWorkerYExportaID(t *testing.T) {
	root, f := sliceConFake(t, false)
	if _, errs, code := run(t, "slice", "lanzar", "--agente", "codex", "--dificultad", "baja"); code != 0 {
		t.Fatalf("lanzar: %s", errs)
	}
	st, err := workers.Load(filepath.Join(root, ".orq"), "worker-codex-s01")
	if err != nil {
		t.Fatal(err)
	}
	if st.Phase != "running" || st.Slice != "S-01" || st.Agent != "codex" || st.Model != "gpt-6-luna" || st.Worktree == "" || st.BudgetMin != 45 {
		t.Fatalf("%+v", st)
	}
	if len(f.Envs) == 0 || f.Envs[len(f.Envs)-1]["ORQ_WORKER_ID"] != "worker-codex-s01" || f.Envs[len(f.Envs)-1]["ORQ_ROL"] != "worker-codex-s01" {
		t.Fatalf("env: %v", f.Envs)
	}
	// Evaluador A/C: el worker no recibe el token y su PATH empieza por el shim de orq.
	env := f.Envs[len(f.Envs)-1]
	if tok, ok := env["ORQ_TOKEN"]; !ok || tok != "" {
		t.Fatalf("el worker heredaría ORQ_TOKEN: %v", env)
	}
	shim := filepath.Join(root, ".orq", "workers", "worker-codex-s01", "bin")
	if !strings.HasPrefix(env["PATH"], shim+":") {
		t.Fatalf("PATH sin shim: %q", env["PATH"])
	}
	if _, err := os.Stat(filepath.Join(shim, "orq")); err != nil {
		t.Fatal(err)
	}
	out, _, _ := run(t, "board", "--json")
	var b struct{ Workers []json.RawMessage }
	if json.Unmarshal([]byte(out), &b) != nil || len(b.Workers) == 0 {
		t.Fatalf("board: %s", out)
	}
}

func TestReviveContextoDeterminista(t *testing.T) {
	root := proyectoVis(t)
	t.Setenv("ORQ_ROL", "atena")
	run(t, "mem", "add", "--imp", "alta", "--cat", "requisito", "--fuente", "owner", "Small shops take card payments online")
	run(t, "stop", "raise", "--type", "api_key", "--severity", "blocking", "--slice", "S-04", "--tasks", "T2", "--text", "Need the Stripe test key in STRIPE_TEST_KEY")
	run(t, "msg", "send", "--de", "atena", "--para", "orquestador", "--tipo", "historia", "build checkout")
	out, errs, code := run(t, "revive", "orquestador")
	if code != 0 {
		t.Fatal(errs)
	}
	for _, w := range []string{"role: orquestador", "S-04", "plan rev", "ST-0001", "build checkout", "Small shops", "last receipt: none"} {
		if !strings.Contains(out, w) {
			t.Fatalf("missing %q:\n%s", w, out)
		}
	}
	out2, _, _ := run(t, "revive", "orquestador")
	if out != out2 || strings.Contains(out, root) {
		t.Fatalf("not deterministic or leaks path:\n%s", out)
	}
}
