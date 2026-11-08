package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"orq/internal/canal"
	"orq/internal/runner"
)

// sliceConFake abre un slice real (git) y cambia el runner por uno falso.
func sliceConFake(t *testing.T, conHook bool) (string, *runner.Fake) {
	t.Helper()
	_, orq, _ := lanzarAislado(t, 12)
	root := filepath.Dir(orq)
	t.Setenv("ORQ_WORKTREES", t.TempDir())
	t.Setenv("ORQ_AGENTS_GLOBAL", filepath.Join(t.TempDir(), "no.json"))
	git := func(a ...string) {
		if out, err := exec.Command("git", append([]string{"-C", root}, a...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", a, err, out)
		}
	}
	git("init", "-q", "-b", "main")
	git("config", "user.email", "t@t")
	git("config", "user.name", "t")
	os.WriteFile(filepath.Join(root, "examen.sh"), []byte("exit 1\n"), 0o644)
	os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".orq/\n.claude/\n"), 0o644)
	if conHook {
		os.MkdirAll(filepath.Join(root, ".claude"), 0o755)
		os.WriteFile(filepath.Join(root, ".claude", "settings.json"), []byte(hookSettings), 0o644)
	}
	git("add", "-A")
	git("commit", "-qm", "base")
	if _, errs, code := run(t, "slice", "abrir", "01", "--titulo", "x", "--test", "examen.sh", "--cmd", "sh examen.sh", "--sin-scouts", "prueba"); code != 0 {
		t.Fatalf("abrir: %s", errs)
	}
	f := &runner.Fake{}
	orig := rn
	rn = func() runner.Runner { return f }
	t.Cleanup(func() { rn = orig })
	return root, f
}

func TestLanzarCodexEsPostOnlyYElExamenQuedaDeSoloLectura(t *testing.T) {
	root, f := sliceConFake(t, false)
	_, errs, code := run(t, "slice", "lanzar", "--agente", "codex", "--dificultad", "baja")
	if code != 0 {
		t.Fatalf("lanzar: %s", errs)
	}
	start := ""
	for _, c := range f.Calls {
		if strings.HasPrefix(c, "start ") {
			start = c
		}
	}
	for _, w := range []string{"worker-codex-s01 codex", "gpt-6-luna", "workspace-write", "--add-dir"} {
		if !strings.Contains(start, w) {
			t.Fatalf("falta %q en %q", w, start)
		}
	}
	ses, _ := canal.LoadSesiones(filepath.Join(root, ".orq", "sesiones.json"))
	if ses["worker-codex-s01"].Gate != "post-only" {
		t.Fatalf("registro: %+v", ses["worker-codex-s01"])
	}
	wt := filepath.Join(os.Getenv("ORQ_WORKTREES"))
	ms, _ := filepath.Glob(filepath.Join(wt, "*", "examen.sh"))
	if len(ms) == 0 {
		t.Fatal("no encontré el examen en el worktree")
	}
	if st, _ := os.Stat(ms[0]); st.Mode().Perm()&0o222 != 0 {
		t.Fatalf("el examen sigue escribible: %v", st.Mode())
	}
}

func TestLanzarClaudeSinHookSeNiega(t *testing.T) {
	_, f := sliceConFake(t, false)
	_, errs, code := run(t, "slice", "lanzar", "--agente", "claude")
	if code == 0 || !strings.Contains(errs, "orq gate") {
		t.Fatalf("code=%d errs=%q", code, errs)
	}
	for _, c := range f.Calls {
		if strings.HasPrefix(c, "start ") {
			t.Fatalf("arrancó sin hook: %v", f.Calls)
		}
	}
}

func TestLanzarClaudeCopiaElHookAlWorktree(t *testing.T) {
	root, f := sliceConFake(t, true)
	if _, errs, code := run(t, "slice", "lanzar", "--agente", "claude"); code != 0 {
		t.Fatalf("lanzar: %s", errs)
	}
	ms, _ := filepath.Glob(filepath.Join(os.Getenv("ORQ_WORKTREES"), "*", ".claude", "settings.json"))
	if len(ms) == 0 || !hookEn(filepath.Dir(filepath.Dir(ms[0]))) {
		t.Fatalf("el worktree no tiene el hook: %v (calls %v)", ms, f.Calls)
	}
	ses, _ := canal.LoadSesiones(filepath.Join(root, ".orq", "sesiones.json"))
	if ses["worker-claude-s01"].Gate != "hook" {
		t.Fatalf("%+v", ses)
	}
}

func TestConfigAgentsSetDetectShow(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ORQ_ROOT", root)
	t.Setenv("ORQ_AGENTS_GLOBAL", filepath.Join(t.TempDir(), "no.json"))
	os.MkdirAll(filepath.Join(root, ".orq"), 0o755)
	if _, errs, code := run(t, "config", "agents", "set", "adversary", "claude:opus"); code == 0 || !strings.Contains(errs, "familia") {
		t.Fatalf("adversario de la misma familia: %d %s", code, errs)
	}
	if _, errs, code := run(t, "config", "agents", "set", "worker.low", "codex:gpt-6-luna"); code != 0 {
		t.Fatal(errs)
	}
	orig := buscarCLI
	buscarCLI = func(c string) bool { return c == "claude" || c == "gemini" }
	t.Cleanup(func() { buscarCLI = orig })
	if out, errs, code := run(t, "config", "agents", "detect"); code != 0 || !strings.Contains(out, "adversary → gemini") {
		t.Fatalf("detect: %s %s", out, errs)
	}
	out, _, _ := run(t, "config", "agents", "show")
	// worker.low lo fijó el Owner: la propuesta no lo pisa; el adversario sale de la propuesta.
	if !strings.Contains(out, "worker.low       codex:gpt-6-luna") || !strings.Contains(out, "adversary        gemini:") {
		t.Fatalf("show: %s", out)
	}
}
