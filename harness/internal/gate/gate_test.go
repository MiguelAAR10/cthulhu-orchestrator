package gate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"orq/internal/limits"
	"orq/internal/state"
)

func ptrF(v float64) *float64 { return &v }
func ptrI(v int) *int         { return &v }

func baseLimits() limits.Limits {
	return limits.Limits{
		MinFreeGB:        map[string]float64{"/": 50, "/mnt/c": 30},
		MaxWorkers:       ptrI(1),
		Scratch:          limits.Scratch{Paths: []string{"/tmp/x"}, QuotaGB: ptrF(20), CriticalFreeGB: ptrF(10)},
		StateMaxAgeHours: ptrF(24),
		Protected:        []string{"tests/aceptacion/", "SLICES.md"},
	}
}

type fakes struct {
	free    map[string]float64
	workers int
	now     time.Time
	wt      bool
	wtrees  int
}

func (f fakes) env(t *testing.T, stateDir string) Env {
	return Env{
		DiskFreeGB: func(p string) (float64, error) { return f.free[p], nil },
		Workers:    func() (int, error) { return f.workers, nil },
		Now:        func() time.Time { return f.now },
		StateDir:   stateDir,
		IsWorktree: func(string) bool { return f.wt },
		Worktrees:  func(string) (int, error) { return f.wtrees, nil },
	}
}

func freshState(t *testing.T, now time.Time, scratchGB float64) string {
	dir := t.TempDir()
	if err := state.SaveSnapshot(dir, state.Snapshot{TS: now, ScratchGB: scratchGB}); err != nil {
		t.Fatal(err)
	}
	return dir
}

func bash(cmd string) Payload {
	return Payload{ToolName: "Bash", Cwd: "/home/x/repo", ToolInput: map[string]any{"command": cmd}}
}

func TestLaunchDecisions(t *testing.T) {
	now := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	ok := fakes{free: map[string]float64{"/": 400, "/mnt/c": 100}, workers: 0, now: now}

	cases := []struct {
		name    string
		p       Payload
		f       fakes
		scratch float64
		stateAt time.Time
		halt    string
		allow   bool
		reason  string
	}{
		{"bash normal pasa aunque todo esté mal", bash("ls -la"), fakes{free: map[string]float64{"/": 1}, workers: 9, now: now}, 999, now, "", true, ""},
		{"Agent con recursos pasa", Payload{ToolName: "Agent"}, ok, 5, now, "", true, ""},
		{"herdr agent start con recursos pasa", bash("herdr agent start --name w1 codex"), ok, 5, now, "", true, ""},
		{"codex suelto es lanzamiento", bash("cd x && codex --full-auto"), fakes{free: ok.free, workers: 1, now: now}, 5, now, "", false, "worker(s) vivos"},
		{"claude -p es lanzamiento", bash("claude --model sonnet -p 'hola'"), fakes{free: ok.free, workers: 1, now: now}, 5, now, "", false, "worker(s) vivos"},
		// Orca: el usuario trabaja con herdr u Orca, y el guardia debe cubrir ambos.
		{"orca agent launch es lanzamiento", bash("orca agent launch --agent claude --worktree w1 --prompt hola"), fakes{free: ok.free, workers: 1, now: now}, 5, now, "", false, "worker(s) vivos"},
		{"orca-ide worker-start es lanzamiento", bash("orca-ide orchestration worker-start --spec x --agent claude"), fakes{free: ok.free, workers: 1, now: now}, 5, now, "", false, "worker(s) vivos"},
		{"orca dispatch es lanzamiento", bash("orca orchestration dispatch --task t1 --to term1"), fakes{free: ok.free, workers: 1, now: now}, 5, now, "", false, "worker(s) vivos"},
		{"orca terminal create es lanzamiento", bash("cd x && orca terminal create --worktree w1"), fakes{free: ok.free, workers: 1, now: now}, 5, now, "", false, "worker(s) vivos"},
		{"orca worktree create es lanzamiento", bash("orca worktree create --name s02"), fakes{free: map[string]float64{"/": 40, "/mnt/c": 100}, now: now}, 5, now, "", false, "/ tiene 40 GB"},
		{"orca agent launch con recursos pasa", bash("orca agent launch --agent claude --prompt hola"), ok, 5, now, "", true, ""},
		{"orca status no es lanzamiento", bash("orca status"), fakes{free: map[string]float64{"/": 1}, workers: 9, now: now}, 999, now, "", true, ""},
		{"orca terminal read no es lanzamiento", bash("orca-ide terminal read --terminal t1 --json"), fakes{free: map[string]float64{"/": 1}, workers: 9, now: now}, 999, now, "", true, ""},
		{"orca worktree list no es lanzamiento", bash("orca worktree list --json"), fakes{free: map[string]float64{"/": 1}, workers: 9, now: now}, 999, now, "", true, ""},
		{"subagente no lanza", Payload{ToolName: "Agent", AgentID: "a1"}, ok, 5, now, "", false, "subagente"},
		{"HALT bloquea", Payload{ToolName: "Agent"}, ok, 5, now, "disco crítico", false, "HALT activo"},
		{"disco raíz bajo", Payload{ToolName: "Agent"}, fakes{free: map[string]float64{"/": 40, "/mnt/c": 100}, now: now}, 5, now, "", false, "/ tiene 40 GB"},
		{"disco C bajo aunque raíz sobre", Payload{ToolName: "Agent"}, fakes{free: map[string]float64{"/": 400, "/mnt/c": 20}, now: now}, 5, now, "", false, "/mnt/c tiene 20 GB"},
		{"medición vieja", Payload{ToolName: "Agent"}, ok, 5, now.Add(-30 * time.Hour), "", false, "30 h"},
		{"scratch sobre cuota", Payload{ToolName: "Agent"}, ok, 25, now, "", false, "cuota"},
		{"tope de worktrees (max_worktrees por defecto 12)", Payload{ToolName: "Agent"}, fakes{free: ok.free, wtrees: 12, now: now}, 5, now, "", false, "max_worktrees"},
		{"tope de workers", Payload{ToolName: "Agent"}, fakes{free: ok.free, workers: 1, now: now}, 5, now, "", false, "tope es 1"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := freshState(t, c.stateAt, c.scratch)
			if c.halt != "" {
				_ = state.WriteHalt(dir, c.halt, now)
			}
			d := Evaluate(c.p, baseLimits(), c.f.env(t, dir))
			if d.Allow != c.allow {
				t.Fatalf("allow=%v, esperaba %v (%s)", d.Allow, c.allow, d.Reason)
			}
			if !c.allow && !strings.Contains(d.Reason, c.reason) {
				t.Fatalf("motivo %q no contiene %q", d.Reason, c.reason)
			}
		})
	}
}

func TestSinMedicionDeniegaLanzamiento(t *testing.T) {
	f := fakes{free: map[string]float64{"/": 400, "/mnt/c": 100}, now: time.Now()}
	d := Evaluate(Payload{ToolName: "Agent"}, baseLimits(), f.env(t, t.TempDir()))
	if d.Allow || !strings.Contains(d.Reason, "orq watch") {
		t.Fatalf("esperaba denegar por falta de medición: %+v", d)
	}
}

func TestProtegidosSoloEnWorktree(t *testing.T) {
	now := time.Now()
	f := fakes{free: map[string]float64{"/": 400, "/mnt/c": 100}, now: now}
	dir := freshState(t, now, 0)
	edit := Payload{ToolName: "Edit", Cwd: "/wt", ToolInput: map[string]any{"file_path": "/wt/tests/aceptacion/s07.test.ts"}}
	ledger := Payload{ToolName: "Write", Cwd: "/wt", ToolInput: map[string]any{"file_path": "/wt/SLICES.md"}}
	shell := bash("echo x > tests/aceptacion/s07.test.ts")
	readOnly := bash("cat tests/aceptacion/s07.test.ts")
	other := Payload{ToolName: "Edit", Cwd: "/wt", ToolInput: map[string]any{"file_path": "/wt/src/cart.ts"}}

	f.wt = true
	for _, p := range []Payload{edit, ledger, shell} {
		if d := Evaluate(p, baseLimits(), f.env(t, dir)); d.Allow {
			t.Fatalf("en worktree debía bloquear %v", p.ToolInput)
		}
	}
	for _, p := range []Payload{readOnly, other} {
		if d := Evaluate(p, baseLimits(), f.env(t, dir)); !d.Allow {
			t.Fatalf("en worktree debía permitir %v: %s", p.ToolInput, d.Reason)
		}
	}
	f.wt = false
	for _, p := range []Payload{edit, ledger, shell} {
		if d := Evaluate(p, baseLimits(), f.env(t, dir)); !d.Allow {
			t.Fatalf("en checkout principal debía permitir %v: %s", p.ToolInput, d.Reason)
		}
	}
}

// Encontrado por validacion/validar-mvp.sh --real: el worker no podía correr el test
// porque `2>&1` contaba como escritura sobre tests/aceptacion/.
func TestEscriturasDeShellMiranElDestino(t *testing.T) {
	now := time.Now()
	f := fakes{free: map[string]float64{"/": 400, "/mnt/c": 100}, now: now, wt: true}
	dir := freshState(t, now, 0)
	cases := []struct {
		cmd   string
		allow bool
	}{
		{"node --test tests/aceptacion/ 2>&1", true},
		{"node --test tests/aceptacion/ 2>&1 | tail -20", true},
		{"node --test tests/aceptacion/ > /tmp/salida.log", true},
		{"pnpm vitest run tests/aceptacion/s07.test.ts &> out.txt", true},
		{"cp tests/aceptacion/s07.test.ts /tmp/copia.ts", true},
		{"grep -n total tests/aceptacion/s07.test.ts >&2", true},
		{"echo x > tests/aceptacion/s07.test.ts", false},
		{"echo x >>tests/aceptacion/s07.test.ts", false},
		{"cat a 2> tests/aceptacion/s07.test.ts", false},
		{"echo x | tee -a tests/aceptacion/s07.test.ts", false},
		{"sed -i 's/15480/0/' tests/aceptacion/s07.test.ts", false},
		{"cp /tmp/falso.ts tests/aceptacion/s07.test.ts", false},
		{"mv tests/aceptacion/s07.test.ts /tmp/", false},
		{"rm -f tests/aceptacion/s07.test.ts", false},
		{"git checkout HEAD~1 -- tests/aceptacion/s07.test.ts", false},
		{"git restore tests/aceptacion/", false},
		{"node --test tests/aceptacion/ && echo ok > SLICES.md", false},
	}
	for _, c := range cases {
		d := Evaluate(bash(c.cmd), baseLimits(), f.env(t, dir))
		if d.Allow != c.allow {
			t.Errorf("%q: allow=%v, esperaba %v (%s)", c.cmd, d.Allow, c.allow, d.Reason)
		}
	}
}

func TestIsLinkedWorktree(t *testing.T) {
	root := t.TempDir()
	main := filepath.Join(root, "main")
	wt := filepath.Join(root, "wt", "deep")
	_ = os.MkdirAll(filepath.Join(main, ".git"), 0o755)
	_ = os.MkdirAll(wt, 0o755)
	_ = os.WriteFile(filepath.Join(root, "wt", ".git"), []byte("gitdir: x"), 0o644)
	if IsLinkedWorktree(main) {
		t.Fatal("checkout principal detectado como worktree")
	}
	if !IsLinkedWorktree(wt) {
		t.Fatal("worktree enlazado no detectado")
	}
	if IsLinkedWorktree(t.TempDir()) {
		t.Fatal("directorio sin git detectado como worktree")
	}
}

// wtReal arma un worktree de mentira en disco para probar rutas resueltas.
func wtReal(t *testing.T) (string, Env, limits.Limits) {
	t.Helper()
	wt := t.TempDir()
	os.MkdirAll(filepath.Join(wt, "tests", "aceptacion"), 0o755)
	os.WriteFile(filepath.Join(wt, "tests", "aceptacion", "a.py"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(wt, "conftest.py"), []byte("x"), 0o644)
	env := fakes{wt: true}.env(t, t.TempDir())
	env.Bundle = func(string) []string { return []string{"conftest.py"} }
	return wt, env, baseLimits()
}

func TestRutasRealesYTrucosDeShell(t *testing.T) {
	wt, env, l := wtReal(t)
	os.Symlink(filepath.Join(wt, "tests", "aceptacion"), filepath.Join(wt, "atajo"))
	en := func(cmd string) Payload {
		return Payload{ToolName: "Bash", Cwd: wt, ToolInput: map[string]any{"command": cmd}}
	}
	niega := []Payload{
		en("cd tests/aceptacion && echo hola > a.py"),
		en("echo x > tests//aceptacion/./a.py"),
		en("echo x > atajo/a.py"),
		en("ln -s /tmp/falso tests/aceptacion/b.py"),
		en(`python3 -c "open('tests/aceptacion/a.py','w').write('')"`),
		en("dd if=/dev/zero of=tests/aceptacion/a.py count=1"),
		en("echo trampa > conftest.py"), // bundle del slice abierto
		{ToolName: "Write", Cwd: wt, ToolInput: map[string]any{"file_path": filepath.Join(wt, "atajo", "a.py")}},
	}
	for _, p := range niega {
		if d := Evaluate(p, l, env); d.Allow {
			t.Errorf("dejó pasar: %v", p.ToolInput)
		}
	}
	if d := Evaluate(en("echo ok > src/app.py"), l, env); !d.Allow {
		t.Errorf("negó una escritura normal: %s", d.Reason)
	}
}

func TestDestructivosSiempreBloqueados(t *testing.T) {
	env := fakes{}.env(t, t.TempDir())
	for _, c := range []string{"git push --force origin main", "git push origin main", "git branch -D slice/01",
		"git worktree remove --force ../wt", "git reset --hard HEAD~1", "git clean -fdx", "rm -rf ~/worktrees/x", "rm -rf .orq"} {
		if d := Evaluate(bash(c), baseLimits(), env); d.Allow {
			t.Errorf("dejó pasar %q", c)
		}
	}
	for _, c := range []string{"git branch -d slice/01", "git worktree remove ../wt", "rm -rf node_modules", "git status"} {
		if d := Evaluate(bash(c), baseLimits(), env); !d.Allow {
			t.Errorf("negó %q: %s", c, d.Reason)
		}
	}
}

func TestWorkerNoCierraNiAbre(t *testing.T) {
	wt := fakes{wt: true}.env(t, t.TempDir())
	main := fakes{}.env(t, t.TempDir())
	for _, c := range []string{"orq slice cerrar --aceptado", "orq slice abrir 02 --titulo x", "orq slice tomar yo --relevo", "orq msg send --para atena --tipo cierre listo"} {
		if d := Evaluate(bash(c), baseLimits(), wt); d.Allow {
			t.Errorf("un worker pudo: %q", c)
		}
		if d := Evaluate(bash(c), baseLimits(), main); !d.Allow {
			t.Errorf("el orquestador no pudo: %q (%s)", c, d.Reason)
		}
	}
	if d := Evaluate(bash("orq slice verificar"), baseLimits(), wt); !d.Allow {
		t.Error("un worker debe poder pedir verificación")
	}
}

func TestNoSeEsquivaConOpcionesComillasNiRutas(t *testing.T) {
	env := fakes{wt: true, free: map[string]float64{"/": 400, "/mnt/c": 100}, workers: 1, now: time.Now()}.env(t, t.TempDir())
	for _, c := range []string{"git -C . push origin main", "git --no-pager push", "git -C . branch -D foo", "git -c x=y reset --hard HEAD~1",
		"git clean --force", "rm --recursive --force .git", "git checkout -f main",
		`sh -c 'codex exec hola'`, `bash -lc "herdr agent start x"`,
		`bash -c "orq slice cerrar"`, "/usr/local/bin/orq slice cerrar 01", "orq --repo . slice cerrar", "orq slice --slice 01 cerrar"} {
		if d := Evaluate(bash(c), baseLimits(), env); d.Allow {
			t.Errorf("dejó pasar %q", c)
		}
	}
	if d := Evaluate(bash("orq slice --slice 01 verificar"), baseLimits(), env); !d.Allow {
		t.Errorf("negó verificar: %s", d.Reason)
	}
}

// El Owner deja subir ramas en sus checkouts listados; main, fuerza, encadenados y worktrees siguen bloqueados.
func TestPushDeRamasPermitido(t *testing.T) {
	env := fakes{}.env(t, t.TempDir())
	repo, otro := t.TempDir(), t.TempDir()
	l := baseLimits()
	l.PushRamas = []string{repo}
	en := func(cwd, cmd string) Payload { p := bash(cmd); p.Cwd = cwd; return p }
	for _, c := range []Payload{
		en(repo, "git push origin dev"),
		en(repo, "git push -u origin demo/2026-10-26"),
		en(otro, "git -C "+repo+" push origin dev"),
	} {
		if d := Evaluate(c, l, env); !d.Allow {
			t.Errorf("negó %q: %s", c.ToolInput["command"], d.Reason)
		}
	}
	for _, c := range []Payload{
		en(repo, "git push origin main"),
		en(repo, "git push origin master"),
		en(repo, "git push --force origin dev"),
		en(repo, "git push origin +dev"),
		en(repo, "git push origin :dev"),
		en(repo, "git push origin dev && git push origin main"),
		en(repo, "git push origin dev; rm -rf .orq"),
		en(repo, "git push \"origin\" dev"),
		en(repo, "git push"),
		en(otro, "git push origin dev"),
		en(repo, "git -C "+otro+" push origin dev"),
	} {
		if d := Evaluate(c, l, env); d.Allow {
			t.Errorf("dejó pasar %q", c.ToolInput["command"])
		}
	}
	if d := Evaluate(en(repo, "git push origin dev"), baseLimits(), env); d.Allow {
		t.Error("sin push_ramas el push sigue bloqueado")
	}
}

// H-01: el contrato de aceptación (examen y alcance) no lo cambia un worker.
func TestWorkerNoCambiaElExamenNiElAlcance(t *testing.T) {
	wt := fakes{wt: true}.env(t, t.TempDir())
	main := fakes{}.env(t, t.TempDir())
	for _, c := range []string{`orq slice examen tests/x.sh --aprobado "req ATN-1" --cmd "true"`, `orq slice --slice 01 examen tests/x.sh --aprobado m1`, `orq slice alcance src --aprobado "m1"`} {
		if d := Evaluate(bash(c), baseLimits(), wt); d.Allow {
			t.Errorf("un worker pudo: %q", c)
		}
		p := bash(c)
		p.AgentID = "sub-1"
		if d := Evaluate(p, baseLimits(), main); d.Allow {
			t.Errorf("un subagente pudo: %q", c)
		}
		if d := Evaluate(bash(c), baseLimits(), main); !d.Allow {
			t.Errorf("el orquestador no pudo: %q (%s)", c, d.Reason)
		}
	}
}
