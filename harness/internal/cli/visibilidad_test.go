package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"orq/internal/memoria"
	"orq/internal/plan"
	"orq/internal/slice"
	"orq/internal/stops"
	"orq/internal/workers"
)

// proyectoVis: repo vacío con S-04 abierto y plan T1, T2 independientes.
func proyectoVis(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("ORQ_ROOT", root)
	t.Setenv("ORQ_ROL", "")
	t.Setenv("ORQ_WORKER_ID", "")
	t.Setenv("HERDR_PANE_ID", "")
	orq := filepath.Join(root, ".orq")
	os.MkdirAll(filepath.Join(orq, "slices"), 0o755)
	raw, _ := json.Marshal(slice.Estado{NN: "04", Intento: 1, Fase: slice.Implementando, Titulo: "Customers pay by card", Test: "t"})
	os.WriteFile(filepath.Join(orq, "slices", "04.json"), raw, 0o644)
	plan.Guardar(plan.Path(orq, "04"), &plan.Plan{Slice: "04", Tareas: []plan.Tarea{
		{ID: "T1", Zona: "a", Rutas: []string{"a/"}, Que: "checkout page"},
		{ID: "T2", Zona: "b", Rutas: []string{"b/"}, Que: "charge the card"},
	}})
	return root
}

func stopsFile(root string) string {
	b, _ := os.ReadFile(filepath.Join(root, ".orq", "stops.jsonl"))
	return string(b)
}

// stoppers-autonomy acceptance 1.
func TestAccStopHoldsOnlyItsTask(t *testing.T) {
	proyectoVis(t)
	t.Setenv("ORQ_WORKER_ID", "w-s04-t2")
	out, errs, code := run(t, "stop", "raise", "--type", "api_key", "--severity", "blocking", "--slice", "S-04", "--tasks", "T2",
		"--text", "I need a Stripe test API key to check payments. Put it in env var STRIPE_TEST_KEY.", "--where", "STRIPE_TEST_KEY")
	if code != 0 || !strings.Contains(out, "ST-0001") {
		t.Fatalf("%d %s %s", code, out, errs)
	}
	out, _, _ = run(t, "plan", "--slice", "04", "listas")
	if !strings.Contains(out, "ready: T1\n") || !strings.Contains(out, "T2 waits: stop ST-0001") {
		t.Fatal(out)
	}
	if _, errs, code := run(t, "stop", "raise", "--type", "api_key", "--severity", "soon", "--slice", "S-04", "--tasks", "T9", "--text", "x"); code == 0 {
		t.Fatal("unknown task accepted", errs)
	}
}

// stoppers-autonomy acceptance 2.
func TestAccSecretRejectedExit2(t *testing.T) {
	root := proyectoVis(t)
	before := stopsFile(root)
	_, errs, code := run(t, "stop", "raise", "--type", "api_key", "--severity", "blocking", "--slice", "S-04", "--text", "use sk-live-abc123def456ghi789jkl")
	if code != 2 || !strings.Contains(errs, "looks like a secret") || stopsFile(root) != before {
		t.Fatalf("%d %s", code, errs)
	}
}

// stoppers-autonomy acceptance 3.
func TestAccOnlyAtenaResolves(t *testing.T) {
	root := proyectoVis(t)
	t.Setenv("ORQ_WORKER_ID", "w-s04-t2")
	run(t, "stop", "raise", "--type", "api_key", "--severity", "blocking", "--slice", "S-04", "--tasks", "T2", "--text", "need key")
	if _, _, code := run(t, "stop", "resolve", "ST-0001", "--provided-at", "env:STRIPE_TEST_KEY"); code == 0 {
		t.Fatal("worker resolved a stopper")
	}
	t.Setenv("ORQ_WORKER_ID", "")
	t.Setenv("ORQ_ROL", "atena")
	t.Setenv("STRIPE_TEST_KEY", "value-never-printed")
	out, errs, code := run(t, "stop", "resolve", "ST-0001", "--provided-at", "env:STRIPE_TEST_KEY")
	if code != 0 || !strings.Contains(out, "(set here)") || strings.Contains(out+stopsFile(root), "value-never-printed") {
		t.Fatalf("%d %s %s", code, out, errs)
	}
	all, _ := stops.Load(stops.Path(filepath.Join(root, ".orq")))
	if all[0].Provided != "yes" || all[0].ProvidedAt != "env:STRIPE_TEST_KEY" || all[0].State != "resolved" {
		t.Fatalf("%+v", all[0])
	}
	out, _, _ = run(t, "plan", "--slice", "04", "listas")
	if !strings.Contains(out, "ready: T1, T2") {
		t.Fatal(out)
	}
}

// stoppers-autonomy acceptance 4.
func TestAccFreeModeOrchestratorResolvesSoonOnly(t *testing.T) {
	root := proyectoVis(t)
	t.Setenv("ORQ_ROL", "atena")
	if _, errs, code := run(t, "autonomy", "free", "--scope", "src/**,docs/**"); code != 0 {
		t.Fatal(errs)
	}
	t.Setenv("ORQ_ROL", "orquestador")
	if _, _, code := run(t, "autonomy", "guided"); code == 0 {
		t.Fatal("orchestrator changed autonomy")
	}
	run(t, "stop", "raise", "--type", "decision", "--severity", "soon", "--slice", "S-04", "--text", "Button color?", "--options", "blue|green")
	run(t, "stop", "raise", "--type", "decision", "--severity", "blocking", "--slice", "S-04", "--text", "Which bank?", "--options", "a|b")
	if _, errs, code := run(t, "stop", "resolve", "ST-0001", "--choice", "blue", "--note", "auto: matches the brand"); code != 0 {
		t.Fatal(errs)
	}
	items, _ := memoria.Load(filepath.Join(root, ".orq", "memoria.jsonl"))
	if len(items) != 1 || items[0].Cat != "decision" || items[0].Fuente != "orquestador" || items[0].Evidencia != "ST-0001" {
		t.Fatalf("%+v", items)
	}
	if _, _, code := run(t, "stop", "resolve", "ST-0002", "--choice", "a", "--note", "auto: cheaper"); code == 0 {
		t.Fatal("orchestrator resolved a blocking stopper")
	}
	// Atena resolving a decision writes an owner decision.
	t.Setenv("ORQ_ROL", "atena")
	if _, errs, code := run(t, "stop", "resolve", "ST-0002"); code == 0 {
		t.Fatal("decision without --choice", errs)
	}
	run(t, "stop", "resolve", "ST-0002", "--choice", "a")
	items, _ = memoria.Load(filepath.Join(root, ".orq", "memoria.jsonl"))
	if len(items) != 2 || items[1].Fuente != "owner" || items[1].Tipo != "decision" {
		t.Fatalf("%+v", items)
	}
}

// stoppers-autonomy acceptance 5.
func TestAccFreeModeWorkerCannotChangeExam(t *testing.T) {
	proyectoVis(t)
	t.Setenv("ORQ_ROL", "atena")
	run(t, "autonomy", "free", "--scope", "src/**")
	t.Setenv("ORQ_ROL", "")
	t.Setenv("ORQ_WORKER_ID", "w1")
	_, errs, code := run(t, "slice", "examen", "tests/x.sh", "--aprobado", "m1")
	if code == 0 || !strings.Contains(errs, "H-01") {
		t.Fatalf("%d %s", code, errs)
	}
}

// stoppers-autonomy acceptance 6.
func TestAccSameStopperOneID(t *testing.T) {
	root := proyectoVis(t)
	for i := 0; i < 2; i++ {
		out, _, code := run(t, "stop", "raise", "--type", "access", "--severity", "fyi", "--slice", "S-04", "--text", "need staging login")
		if code != 0 || !strings.Contains(out, "ST-0001") {
			t.Fatal(out)
		}
	}
	if n := strings.Count(stopsFile(root), "\n"); n != 1 {
		t.Fatalf("%d lines", n)
	}
}

// board acceptance 5: one file, no script, no http, nothing else in .orq changes.
func TestAccBoardHTMLOneFile(t *testing.T) {
	root := proyectoVis(t)
	run(t, "stop", "raise", "--type", "access", "--severity", "fyi", "--slice", "S-04", "--text", "see https later")
	snap := func() map[string]string {
		m := map[string]string{}
		filepath.Walk(filepath.Join(root, ".orq"), func(p string, fi os.FileInfo, err error) error {
			if err == nil && !fi.IsDir() {
				b, _ := os.ReadFile(p)
				m[p] = string(b) + fi.ModTime().String()
			}
			return nil
		})
		return m
	}
	before := snap()
	if _, errs, code := run(t, "board", "--html"); code != 0 {
		t.Fatal(errs)
	}
	after := snap()
	out := filepath.Join(root, ".orq", "board.html")
	if len(after) != len(before)+1 {
		t.Fatalf("files %d → %d", len(before), len(after))
	}
	for k, v := range before {
		if after[k] != v {
			t.Fatalf("%s changed", k)
		}
	}
	h := after[out]
	if strings.Contains(h, "<script") || strings.Contains(h, "http") {
		t.Fatal(h)
	}
	j, _, _ := run(t, "board", "--json")
	if !strings.Contains(j, `"schema": "orq.board/v1"`) {
		t.Fatal(j)
	}
	txt, _, _ := run(t, "board")
	if strings.Contains(txt, "\x1b[") || strings.Index(txt, "STOPPERS") > strings.Index(txt, "SLICES") {
		t.Fatal(txt)
	}
}

// worker-state acceptance 1 and 2 through the CLI, and done marks the task.
func TestAccWorkerBeatWatchDone(t *testing.T) {
	root := proyectoVis(t)
	orq := filepath.Join(root, ".orq")
	wt := t.TempDir()
	for _, a := range [][]string{{"init", "-q", "-b", "main"}, {"commit", "-q", "--allow-empty", "-m", "base"}} {
		c := exec.Command("git", a...)
		c.Dir = wt
		c.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatal(string(out))
		}
	}
	base, _ := exec.Command("git", "-C", wt, "rev-parse", "HEAD").Output()
	old := time.Now().Add(-11 * time.Minute)
	workers.Launched(orq, workers.State{ID: "w-s04-t1", Slice: "S-04", Task: "T1", Worktree: wt, Base: strings.TrimSpace(string(base))}, old)
	plan.Actualizar(plan.Path(orq, "04"), func(p *plan.Plan) error { _, err := plan.Tomar(p, "T1", "w-s04-t1"); return err })
	t.Setenv("ORQ_WORKER_ID", "w-s04-t1")
	t.Setenv("HOME", t.TempDir()) // sin límites: el vigía de workers corre igual
	for i := 0; i < 2; i++ {
		run(t, "watch")
	}
	log, _ := os.ReadFile(filepath.Join(orq, "watch.jsonl"))
	canal, _ := os.ReadFile(filepath.Join(orq, "canal.jsonl"))
	if strings.Count(string(log), `"stuck"`) != 1 || strings.Count(string(canal), `"de":"vigia"`) != 1 {
		t.Fatalf("log %s canal %s", log, canal)
	}
	if out, errs, code := run(t, "worker", "beat", "--summary", "page renders", "--context-pct", "40"); code != 0 {
		t.Fatal(out, errs)
	}
	out, _, _ := run(t, "worker", "list")
	if !strings.Contains(out, "w-s04-t1") || !strings.Contains(out, "page renders") {
		t.Fatal(out)
	}
	out, errs, code := run(t, "worker", "done", "--summary", "checkout page works", "--ask-verification")
	if code != 0 || !strings.Contains(out, "task T1 marked done") {
		t.Fatal(out, errs)
	}
	ev, _ := workers.LoadTaskEvents(orq)
	if len(ev) != 1 || ev[0].Event != "done" {
		t.Fatalf("%+v", ev)
	}
	if _, _, code := run(t, "worker", "beat", "--phase", "blocked"); code == 0 {
		t.Fatal("blocked without an open stopper")
	}
}

// worker-state acceptance 6.
func TestAccHandoff(t *testing.T) {
	root := proyectoVis(t)
	run(t, "mem", "add", "--imp", "alta", "--cat", "requisito", "--fuente", "owner", "Small shops take card payments online")
	run(t, "stop", "raise", "--type", "api_key", "--severity", "blocking", "--slice", "S-04", "--tasks", "T2", "--text", "Need the Stripe test key in STRIPE_TEST_KEY")
	if _, errs, code := run(t, "handoff"); code != 0 {
		t.Fatal(errs)
	}
	raw, err := os.ReadFile(filepath.Join(root, "HANDOFF.md"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	for i := 1; i <= 6; i++ {
		if !strings.Contains(s, "## "+string(rune('0'+i))+".") {
			t.Fatalf("section %d missing:\n%s", i, s)
		}
	}
	if stops.LooksSecret(s) || strings.Contains(s, root) || !strings.Contains(s, "Small shops") || !strings.Contains(s, "next ready tasks: T1") {
		t.Fatal(s)
	}
}

func TestAccInitSyncModes(t *testing.T) {
	root := t.TempDir()
	exec.Command("git", "init", "-q", root).Run()
	t.Setenv("ORQ_ROOT", root)
	t.Setenv("HOME", t.TempDir())
	if _, errs, code := run(t, "init", "--memoria=branch"); code != 0 {
		t.Fatal(errs)
	}
	gi, _ := os.ReadFile(filepath.Join(root, ".gitignore"))
	cfg, _ := os.ReadFile(filepath.Join(root, ".orq", "config.json"))
	if !strings.Contains(string(gi), "/.orq-sync/") || !strings.Contains(string(cfg), `"sync": "branch"`) {
		t.Fatalf("%s %s", gi, cfg)
	}
	if _, _, code := run(t, "init", "--sync=nube"); code == 0 {
		t.Fatal("bad mode accepted")
	}
	if _, _, code := run(t, "sync", "push"); code == 0 {
		t.Fatal("push without remote must fail")
	}
}
