package internal_test

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
)

// orq2 installs side by side with the live orq 1.0.0. These tests build the real binary twice,
// once named orq and once named orq2, and run them with a temp HOME and no ORQ_* overrides:
// nothing here reads or writes the installed orq, its config or its state.

var binarios struct {
	once      sync.Once
	orq, orq2 string
	err       error
}

func construir(t *testing.T) (orq, orq2 string) {
	t.Helper()
	binarios.once.Do(func() {
		dir, err := os.MkdirTemp("", "orq2-bin-")
		if err != nil {
			binarios.err = err
			return
		}
		binarios.orq = filepath.Join(dir, "orq", "orq")
		binarios.orq2 = filepath.Join(dir, "orq2", "orq2")
		for _, b := range []string{binarios.orq, binarios.orq2} {
			c := exec.Command("go", "build", "-o", b, "./cmd/orq")
			c.Dir = ".."
			if out, err := c.CombinedOutput(); err != nil {
				binarios.err = &buildErr{string(out), err}
				return
			}
		}
	})
	if binarios.err != nil {
		t.Fatal(binarios.err)
	}
	return binarios.orq, binarios.orq2
}

type buildErr struct {
	out string
	err error
}

func (e *buildErr) Error() string { return e.err.Error() + ": " + e.out }

// fixtureGo is a fresh git repo with one Go file and a temp HOME of its own.
func fixtureGo(t *testing.T) (home, repo string) {
	t.Helper()
	base := t.TempDir()
	home, repo = filepath.Join(base, "home"), filepath.Join(base, "repo")
	for _, d := range []string{home, repo, filepath.Join(base, "wt")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	os.WriteFile(filepath.Join(repo, "main.go"), []byte("package main\n"), 0o644)
	os.WriteFile(filepath.Join(repo, "go.mod"), []byte("module x\n"), 0o644)
	for _, a := range [][]string{{"init", "-q"}, {"add", "."}, {"-c", "user.name=a", "-c", "user.email=a@b", "commit", "-qm", "init"}} {
		c := exec.Command("git", a...)
		c.Dir = repo
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", a, err, out)
		}
	}
	return home, repo
}

// correr runs bin in repo with only HOME, PATH and the doc/worktree switches: no ORQ_STATE,
// ORQ_CONFIG, ORQ_LIMITS or ORQ_NOMBRE, so the defaults are the ones under test.
func correr(t *testing.T, bin, home, repo string, extra []string, args ...string) (string, int) {
	t.Helper()
	c := exec.Command(bin, args...)
	c.Dir = repo
	c.Env = append([]string{"HOME=" + home, "PATH=/usr/bin:/bin", "ORQ_SIN_DOC=1",
		"ORQ_WORKTREES=" + filepath.Join(filepath.Dir(repo), "wt")}, extra...)
	out, err := c.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return string(out), code
}

// foto is init's output plus every file it wrote under .orq and .claude, with the temp paths
// replaced, so two runs compare byte for byte.
func foto(t *testing.T, out, home, repo string) string {
	t.Helper()
	var b strings.Builder
	b.WriteString(out)
	var rutas []string
	for _, d := range []string{".orq", ".claude"} {
		filepath.WalkDir(filepath.Join(repo, d), func(p string, e fs.DirEntry, err error) error {
			if err == nil && !e.IsDir() {
				rutas = append(rutas, p)
			}
			return nil
		})
	}
	slices.Sort(rutas)
	for _, p := range rutas {
		raw, _ := os.ReadFile(p)
		rel, _ := filepath.Rel(repo, p)
		b.WriteString("--- " + rel + "\n" + string(raw))
	}
	s := strings.ReplaceAll(b.String(), repo, "<ROOT>")
	return strings.ReplaceAll(s, home, "<HOME>")
}

// TestOrqSinCambios: a binary named orq writes exactly what orq wrote before orq2 existed.
// testdata/orq-init.golden was taken from the pre-orq2 binary (commit 2d326a5) on this fixture;
// ORQ_GOLDEN_BIN=<that binary> ORQ_GOLDEN_UPDATE=1 regenerates it.
func TestOrqSinCambios(t *testing.T) {
	orq, _ := construir(t)
	golden := filepath.Join("testdata", "orq-init.golden")
	if b := os.Getenv("ORQ_GOLDEN_BIN"); b != "" {
		orq = b
	}
	home, repo := fixtureGo(t)
	out, code := correr(t, orq, home, repo, nil, "init")
	if code != 0 {
		t.Fatalf("orq init: exit %d\n%s", code, out)
	}
	got := foto(t, out, home, repo)
	if os.Getenv("ORQ_GOLDEN_UPDATE") == "1" {
		os.MkdirAll("testdata", 0o755)
		if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Fatalf("orq init changed:\n--- got\n%s\n--- want\n%s", got, want)
	}
	if strings.Contains(got, "orq2") {
		t.Fatal("a binary named orq mentions orq2")
	}
}

// TestOrq2Identidad: a binary named orq2 hooks itself by absolute path, says orq2, and keeps
// config and state under ~/.config/orq2 and ~/.local/state/orq2.
func TestOrq2Identidad(t *testing.T) {
	_, orq2 := construir(t)
	home, repo := fixtureGo(t)
	out, code := correr(t, orq2, home, repo, nil, "init")
	if code != 0 {
		t.Fatalf("orq2 init: exit %d\n%s", code, out)
	}
	set, err := os.ReadFile(filepath.Join(repo, ".claude", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range []string{"gate", "contexto", "relevo --auto"} {
		if !strings.Contains(string(set), `"command": "`+orq2+" "+h+`"`) {
			t.Errorf("settings.json lacks the hook %q with orq2's absolute path:\n%s", h, set)
		}
	}
	if strings.Contains(string(set), "bin/orq ") {
		t.Errorf("settings.json still calls $HOME/bin/orq:\n%s", set)
	}
	for _, f := range []string{out, string(set)} {
		if strings.Contains(f, "`orq ") || strings.Contains(f, " orq check") {
			t.Errorf("orq2 text still says orq:\n%s", f)
		}
	}
	if !strings.Contains(out, "`orq2 check` los corre") {
		t.Errorf("init output does not say orq2:\n%s", out)
	}
	skill, _ := os.ReadFile(filepath.Join(repo, ".claude", "skills", "orq2-orchestrator", "SKILL.md"))
	if strings.Contains(string(skill), "`orq up`") || !strings.Contains(string(skill), "`orq2 slice lanzar`") {
		t.Errorf("the orchestrator skill orq2 installs still runs orq by name")
	}
	guia, _ := correr(t, orq2, home, repo, nil, "skill", "worker")
	if strings.Contains(guia, "`orq ") || !strings.Contains(guia, "orq2 ") {
		t.Errorf("orq2 skill worker still says orq:\n%s", guia)
	}

	// Defaults: state and config under orq2, nothing under orq.
	if out, code := correr(t, orq2, home, repo, nil, "halt", "--set", "prueba"); code != 0 {
		t.Fatalf("orq2 halt --set: %d %s", code, out)
	}
	if _, err := os.Stat(filepath.Join(home, ".local", "state", "orq2", "HALT")); err != nil {
		t.Errorf("orq2 state is not ~/.local/state/orq2: %v", err)
	}
	st, _ := correr(t, orq2, home, repo, nil, "status")
	if !strings.Contains(st, filepath.Join(home, ".config", "orq2", "limits.json")) {
		t.Errorf("orq2 limits are not under ~/.config/orq2:\n%s", st)
	}
	cv, _ := correr(t, orq2, home, repo, nil, "config", "ver")
	if strings.Contains(cv, filepath.Join(home, ".config", "orq")+"/") {
		t.Errorf("orq2 config ver points at ~/.config/orq:\n%s", cv)
	}
	for _, d := range []string{filepath.Join(home, ".config", "orq"), filepath.Join(home, ".local", "state", "orq")} {
		if _, err := os.Stat(d); err == nil {
			t.Errorf("orq2 created %s, which belongs to orq 1.0.0", d)
		}
	}

	// ORQ_NOMBRE=orq2 on a binary named orq: same identity, the hook carries the name.
	home2, repo2 := fixtureGo(t)
	orq, _ := construir(t)
	if out, code := correr(t, orq, home2, repo2, []string{"ORQ_NOMBRE=orq2"}, "init"); code != 0 {
		t.Fatalf("ORQ_NOMBRE=orq2 orq init: %d %s", code, out)
	}
	set2, _ := os.ReadFile(filepath.Join(repo2, ".claude", "settings.json"))
	if !strings.Contains(string(set2), `"command": "ORQ_NOMBRE=orq2 `+orq+` gate"`) {
		t.Errorf("ORQ_NOMBRE=orq2 hook does not carry the name:\n%s", set2)
	}
}

// TestOrq2NoPisaOrq1: orq2 init refuses a project orq 1.0.0 manages; --convivir writes nothing
// to .claude and says what to add.
func TestOrq2NoPisaOrq1(t *testing.T) {
	orq, orq2 := construir(t)
	casos := map[string]func(t *testing.T, home, repo string){
		"hooks": func(t *testing.T, home, repo string) {
			if out, code := correr(t, orq, home, repo, nil, "init"); code != 0 {
				t.Fatalf("orq init: %d %s", code, out)
			}
		},
		"slices": func(t *testing.T, home, repo string) {
			d := filepath.Join(repo, ".orq", "slices")
			os.MkdirAll(d, 0o755)
			os.WriteFile(filepath.Join(d, "07.json"), []byte(`{"nn":"07","test":"a_test.go","cmd":"go test ./..."}`), 0o644)
		},
	}
	for nombre, preparar := range casos {
		t.Run(nombre, func(t *testing.T) {
			home, repo := fixtureGo(t)
			preparar(t, home, repo)
			antes := foto(t, "", home, repo)
			out, code := correr(t, orq2, home, repo, nil, "init")
			if code == 0 || !strings.Contains(out, "lo maneja orq 1.0.0") || !strings.Contains(out, "--convivir") {
				t.Fatalf("orq2 init on a 1.0.0 project: exit %d\n%s", code, out)
			}
			if despues := foto(t, "", home, repo); despues != antes {
				t.Fatalf("a refused orq2 init changed the project:\n%s\n---\n%s", antes, despues)
			}
			claude := foto(t, "", home, repo)
			out, code = correr(t, orq2, home, repo, nil, "init", "--convivir")
			if code != 0 || !strings.Contains(out, "--convivir: no se escribió nada") || !strings.Contains(out, orq2+" gate") {
				t.Fatalf("orq2 init --convivir: exit %d\n%s", code, out)
			}
			if c := soloClaude(foto(t, "", home, repo)); c != soloClaude(claude) {
				t.Fatalf("--convivir wrote to .claude:\n%s\n---\n%s", soloClaude(claude), c)
			}
		})
	}
}

func soloClaude(f string) string {
	var b strings.Builder
	dentro := false
	for _, l := range strings.SplitAfter(f, "\n") {
		if strings.HasPrefix(l, "--- ") {
			dentro = strings.HasPrefix(l, "--- .claude")
		}
		if dentro {
			b.WriteString(l)
		}
	}
	return b.String()
}

// ---------- review fixes (orq2-review.md) ----------

// Item 1: under orq2 the skills have orq2-* names, on disk and in every text that loads them.
func TestOrq2SkillsPropias(t *testing.T) {
	orq, orq2 := construir(t)
	home, repo := fixtureGo(t)
	if out, code := correr(t, orq2, home, repo, nil, "init"); code != 0 {
		t.Fatalf("orq2 init: %d %s", code, out)
	}
	for _, n := range []string{"orq2-orchestrator", "orq2-planner", "orq2-atena", "orq2-ruta"} {
		raw, err := os.ReadFile(filepath.Join(repo, ".claude", "skills", n, "SKILL.md"))
		if err != nil {
			t.Errorf("orq2 init did not write skill %s: %v", n, err)
			continue
		}
		if !strings.Contains(string(raw), "name: "+n+"\n") {
			t.Errorf("skill %s does not declare that name:\n%.200s", n, raw)
		}
	}
	for _, n := range []string{"orq-orchestrator", "orq-planner", "orq-atena", "orq-ruta"} {
		if _, err := os.Stat(filepath.Join(repo, ".claude", "skills", n)); err == nil {
			t.Errorf("orq2 init wrote the 1.0.0 skill name %s", n)
		}
	}
	for _, rol := range []string{"atena", "orquestador", "planificador"} {
		g, _ := correr(t, orq2, home, repo, nil, "skill", rol)
		if strings.Contains(g, "`orq-ruta`") {
			t.Errorf("orq2 skill %s still points at orq-ruta:\n%s", rol, g)
		}
	}
	// orq keeps its names.
	h2, r2 := fixtureGo(t)
	if out, code := correr(t, orq, h2, r2, nil, "init"); code != 0 {
		t.Fatalf("orq init: %d %s", code, out)
	}
	if _, err := os.Stat(filepath.Join(r2, ".claude", "skills", "orq-orchestrator", "SKILL.md")); err != nil {
		t.Errorf("orq lost orq-orchestrator: %v", err)
	}
}

func gate(t *testing.T, bin, home, repo, cmd string, extra ...string) (string, int) {
	t.Helper()
	tpl, err := os.ReadFile(filepath.Join("..", "..", "templates", "limits.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{"orq", "orq2"} {
		os.MkdirAll(filepath.Join(home, ".config", d), 0o755)
		os.WriteFile(filepath.Join(home, ".config", d, "limits.json"), tpl, 0o644)
	}
	c := exec.Command(bin, "gate")
	c.Dir = repo
	c.Env = append([]string{"HOME=" + home, "PATH=/usr/bin:/bin"}, extra...)
	in, _ := json.Marshal(map[string]any{"tool_name": "Bash", "cwd": repo, "tool_input": map[string]any{"command": cmd}})
	c.Stdin = bytes.NewReader(in)
	out, err := c.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return string(out), code
}

// Item 2: orq2's gate refuses the bare orq command; orq2 stays allowed; orq's gate is unchanged.
func TestOrq2GateRechazaOrqPelado(t *testing.T) {
	orq, orq2 := construir(t)
	home, repo := fixtureGo(t)
	for _, cmd := range []string{"orq status", "orq slice lanzar 01", "cd /tmp && orq msg leer", "$HOME/bin/orq status", "echo hi | orq gate"} {
		out, code := gate(t, orq2, home, repo, cmd)
		if code != 2 || !strings.Contains(out, "orq2") {
			t.Errorf("orq2 gate on %q: exit %d, want 2 saying to use orq2\n%s", cmd, code, out)
		}
	}
	for _, cmd := range []string{"orq2 status", "orq2 slice estado", "echo orq status", "ls ~/.orq"} {
		if out, code := gate(t, orq2, home, repo, cmd); code != 0 {
			t.Errorf("orq2 gate on %q: exit %d\n%s", cmd, code, out)
		}
	}
	for _, cmd := range []string{"orq status", "orq msg leer"} {
		if out, code := gate(t, orq, home, repo, cmd); code != 0 {
			t.Errorf("orq gate on %q changed: exit %d\n%s", cmd, code, out)
		}
	}
}

// Items 3 and 4: orq2 recognises its own hooks, quotes the real hook it refuses, and its usage
// line shows orq2's config.
func TestOrq2HooksPropiosYTextos(t *testing.T) {
	orq, orq2 := construir(t)
	// A binary named orq run as orq2 writes the ORQ_NOMBRE=orq2 <path>/orq form.
	home, repo := fixtureGo(t)
	for i := 1; i <= 2; i++ {
		if out, code := correr(t, orq, home, repo, []string{"ORQ_NOMBRE=orq2"}, "init"); code != 0 {
			t.Fatalf("ORQ_NOMBRE=orq2 orq init, run %d: %d\n%s", i, code, out)
		}
	}
	// A binary named orq2 run twice.
	home, repo = fixtureGo(t)
	for i := 1; i <= 2; i++ {
		if out, code := correr(t, orq2, home, repo, nil, "init"); code != 0 {
			t.Fatalf("orq2 init, run %d: %d\n%s", i, code, out)
		}
	}
	// A real 1.0.0 hook is still refused, and the message quotes the hook as written.
	home, repo = fixtureGo(t)
	if out, code := correr(t, orq, home, repo, nil, "init"); code != 0 {
		t.Fatalf("orq init: %d %s", code, out)
	}
	out, code := correr(t, orq2, home, repo, nil, "init")
	if code != 1 || !strings.Contains(out, "$HOME/bin/orq gate") {
		t.Errorf("refusal does not quote the real hook $HOME/bin/orq gate (exit %d):\n%s", code, out)
	}
	u, _ := correr(t, orq2, home, repo, nil)
	if !strings.Contains(u, "~/.config/orq2/config.json") || strings.Contains(u, "~/.config/orq/config.json") {
		t.Errorf("orq2 usage does not show ~/.config/orq2/config.json:\n%s", u)
	}
	u, _ = correr(t, orq, home, repo, nil)
	if !strings.Contains(u, "~/.config/orq/config.json") {
		t.Errorf("orq usage changed:\n%s", u)
	}
}
