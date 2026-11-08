package qa

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var t0 = time.Date(2026, 11, 8, 10, 0, 0, 0, time.UTC)

// fake runner: exit code by command.
func fake(codes map[string]int, outs map[string]string) Runner {
	return func(dir, cmd string, _ time.Duration) (string, int) { return outs[cmd], codes[cmd] }
}

func completo() Finding {
	return Finding{Requirement: "R1 login", Commit: "abc123", Repro: "repro.sh", Observed: "500", Expected: "200", Impact: "users cannot log in"}
}

func TestFindingIncompletoSeRechaza(t *testing.T) {
	f := completo()
	f.Expected, f.Impact = "", "  "
	err := f.Validar()
	if err == nil || !strings.Contains(err.Error(), "expected") || !strings.Contains(err.Error(), "impact") {
		t.Fatalf("debe rechazar y nombrar los campos que faltan: %v", err)
	}
	dir := t.TempDir()
	if _, err := AddFinding(dir, "07", f, fake(nil, nil), t0); err == nil {
		t.Fatal("AddFinding no debe guardar un hallazgo incompleto")
	}
	if fs := Findings(dir, "07"); len(fs) != 0 {
		t.Fatalf("no debía guardarse nada: %v", fs)
	}
}

func TestFindingSoloConfirmadoSiReproFalla(t *testing.T) {
	dir := t.TempDir()
	pasa, falla := completo(), completo()
	pasa.Repro, falla.Repro = "true", "false"
	run := fake(map[string]int{"true": 0, "false": 1}, nil)
	f1, err := AddFinding(dir, "07", pasa, run, t0)
	if err != nil || f1.Confirmed {
		t.Fatalf("repro que pasa no confirma: %+v %v", f1, err)
	}
	f2, err := AddFinding(dir, "07", falla, run, t0)
	if err != nil || !f2.Confirmed {
		t.Fatalf("repro que falla confirma: %+v %v", f2, err)
	}
	if n := Confirmados(Findings(dir, "07")); n != 1 {
		t.Fatalf("solo cuenta el confirmado: %d", n)
	}
}

func capas() []Layer {
	return []Layer{
		{Name: "unit", Cmd: "go test ./..."},
		{Name: "integration", Cmd: "it.sh"},
		{Name: "e2e", Cmd: "npx playwright test"},
		{Name: "mutation", Cmd: "mut.sh", MinCriticality: "high"},
		{Name: "adversarial", Cmd: "adv.sh", MinCriticality: "high"},
	}
}

func TestMutacionSeOmiteEnMediaYCorreEnAlta(t *testing.T) {
	ran := map[string]int{}
	run := func(dir, cmd string, _ time.Duration) (string, int) { ran[cmd]++; return "3 passed", 0 }
	rec := Run(".", "07", "abc", capas(), "medium", run, t0)
	if ran["mut.sh"] != 0 || ran["adv.sh"] != 0 {
		t.Fatalf("en media no corre mutación ni adversarial: %v", ran)
	}
	var omitida bool
	for _, l := range rec.Layers {
		if l.Name == "mutation" && l.Skipped {
			omitida = true
		}
	}
	if !omitida || rec.Result != "pass" {
		t.Fatalf("la capa queda registrada como omitida: %+v", rec)
	}
	rec = Run(".", "07", "abc", capas(), "high", run, t0)
	if ran["mut.sh"] != 1 || ran["adv.sh"] != 1 {
		t.Fatalf("en alta corren: %v", ran)
	}
	if rec.Layers[0].Passed != 3 {
		t.Fatalf("cuenta por capa: %+v", rec.Layers[0])
	}
}

func TestPiramideEnOrdenYFallo(t *testing.T) {
	var orden []string
	run := func(dir, cmd string, _ time.Duration) (string, int) {
		orden = append(orden, cmd)
		if cmd == "it.sh" {
			return "--- PASS: A\n--- FAIL: B\n", 1
		}
		return "ok", 0
	}
	rec := Run(".", "07", "abc", capas(), "medium", run, t0)
	if strings.Join(orden, ",") != "go test ./...,it.sh,npx playwright test" {
		t.Fatalf("orden de la pirámide: %v", orden)
	}
	if rec.Result != "fail" || rec.Layers[1].Failed != 1 || rec.Layers[1].Passed != 1 {
		t.Fatalf("recibo: %+v", rec)
	}
}

func TestCierreBloqueadoSinReciboEnMedia(t *testing.T) {
	dir := t.TempDir()
	if err := PuedeCerrar(dir, "07", Config{}); err == nil {
		t.Fatal("criticidad media por defecto: sin recibo QA no se cierra")
	}
	if err := PuedeCerrar(dir, "07", Config{Criticality: "low"}); err != nil {
		t.Fatalf("baja no exige recibo: %v", err)
	}
	rec := Run(dir, "07", "abc", []Layer{{Name: "unit", Cmd: "x"}}, "medium", fake(map[string]int{"x": 0}, nil), t0)
	if err := Guardar(dir, rec); err != nil {
		t.Fatal(err)
	}
	if err := PuedeCerrar(dir, "07", Config{}); err != nil {
		t.Fatalf("con recibo verde se cierra: %v", err)
	}
	// un hallazgo confirmado abierto también bloquea
	f := completo()
	f.Repro = "false"
	AddFinding(dir, "07", f, fake(map[string]int{"false": 1}, nil), t0)
	if err := PuedeCerrar(dir, "07", Config{}); err == nil {
		t.Fatal("un hallazgo confirmado bloquea el cierre")
	}
}

func TestConfigTolerante(t *testing.T) {
	dir := t.TempDir()
	if c := LoadConfig(dir); c.Crit() != "medium" {
		t.Fatalf("por defecto medium: %q", c.Crit())
	}
	os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"project":{"criticality":"high"},"qa":{"layers":[{"name":"unit","cmd":"go test"}],"require_receipt_from":"high"}}`), 0o644)
	c := LoadConfig(dir)
	if c.Crit() != "high" || len(c.Layers) != 1 || c.RequireFrom != "high" {
		t.Fatalf("lee project.criticality y qa: %+v", c)
	}
	os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"criticality":"production-critical"}`), 0o644)
	if c := LoadConfig(dir); c.Crit() != "high" {
		t.Fatalf("criticality en raíz y alias: %q", c.Crit())
	}
	os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"criticality":5}`), 0o644)
	if c := LoadConfig(dir); c.Crit() != "medium" {
		t.Fatalf("valor raro → medium: %q", c.Crit())
	}
}

func TestProfileCriticalityWins(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"criticality":"high"}`), 0o644)
	os.WriteFile(filepath.Join(dir, "profile.json"), []byte(`{"criticality":"low"}`), 0o644)
	c := LoadConfig(dir)
	if c.Crit() != "low" {
		t.Fatalf("profile.json gana: %q", c.Crit())
	}
	if err := PuedeCerrar(dir, "01", c); err != nil {
		t.Fatalf("criticality=low no exige recibo: %v", err)
	}
	os.WriteFile(filepath.Join(dir, "profile.json"), []byte(`{"criticality":"trivial"}`), 0o644)
	if c := LoadConfig(dir); c.Crit() != "low" {
		t.Fatalf("trivial cuenta como low: %q", c.Crit())
	}
	os.Remove(filepath.Join(dir, "profile.json"))
	if c := LoadConfig(dir); c.Crit() != "high" || c.Note == "" {
		t.Fatalf("config.json solo como legado con nota: %+v", c)
	}
}

// Un tester que escribe en el worktree con un script de python (el gate no lo ve) invalida el
// recibo; escribir el informe permitido (out.tap) no.
func TestTesterQueEscribeInvalidaElRecibo(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("sin python3")
	}
	dir := t.TempDir()
	git := func(a ...string) {
		if out, err := exec.Command("git", append([]string{"-C", dir, "-c", "user.email=a@b", "-c", "user.name=a"}, a...)...).CombinedOutput(); err != nil {
			t.Fatal(err, string(out))
		}
	}
	git("init", "-q")
	os.WriteFile(filepath.Join(dir, "app.py"), []byte("print(1)\n"), 0o644)
	os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("cache/\n"), 0o644)
	git("add", ".")
	git("commit", "-q", "-m", "x")
	limpio := []Layer{{Name: "unit", Cmd: `python3 -c "open('out.tap','w').write('ok 1\n'); import os; os.makedirs('cache',exist_ok=True); open('cache/x','w').write('1')"`}}
	rec := RunVigilado(dir, "07", limpio, "medium", ShellRunner, t0, []string{"out.tap"})
	if rec.Result != "pass" || rec.Tree == nil || rec.Tree.Before.Head == "" {
		t.Fatalf("informe permitido: %+v", rec)
	}
	for _, cmd := range []string{
		`python3 -c "open('app.py','w').write('print(2)\n')"`,
		`python3 -c "open('nuevo_test.py','w').write('')"`,
		`git -c user.email=a@b -c user.name=a commit -q --allow-empty -m sneaky`,
	} {
		rec := RunVigilado(dir, "07", []Layer{{Name: "unit", Cmd: cmd}}, "medium", ShellRunner, t0, []string{"out.tap"})
		if rec.Result != "invalid" || len(rec.Tree.Changed) == 0 {
			t.Errorf("%s: %+v", cmd, rec)
		}
		git("reset", "-q", "--hard", "HEAD@{0}")
		exec.Command("git", "-C", dir, "clean", "-qfd").Run()
	}
	// un archivo ya sucio que el tester reescribe también cuenta
	os.WriteFile(filepath.Join(dir, "app.py"), []byte("print(3)\n"), 0o644)
	rec = RunVigilado(dir, "07", []Layer{{Name: "unit", Cmd: `python3 -c "open('app.py','w').write('print(4)\n')"`}}, "medium", ShellRunner, t0, nil)
	if rec.Result != "invalid" {
		t.Fatalf("reescritura de un archivo sucio: %+v", rec.Tree)
	}
}

// QA corre en una copia desechable (git worktree --detach) en el HEAD del worker y vigila el
// checkout principal y .orq/config.json.
func TestQAAisladoEnCopia(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("sin python3")
	}
	main := t.TempDir()
	git := func(dir string, a ...string) {
		if out, err := exec.Command("git", append([]string{"-C", dir, "-c", "user.email=a@b", "-c", "user.name=a"}, a...)...).CombinedOutput(); err != nil {
			t.Fatal(err, string(out))
		}
	}
	git(main, "init", "-q")
	os.MkdirAll(filepath.Join(main, "tests"), 0o755)
	os.WriteFile(filepath.Join(main, "tests", "exam.sh"), []byte("exit 0\n"), 0o644)
	os.WriteFile(filepath.Join(main, "app.py"), []byte("print(1)\n"), 0o644)
	os.WriteFile(filepath.Join(main, ".gitignore"), []byte("out.tap\n.orq/\n"), 0o644)
	git(main, "add", ".")
	git(main, "commit", "-q", "-m", "x")
	orqDir := filepath.Join(main, ".orq")
	os.MkdirAll(orqDir, 0o755)
	os.WriteFile(filepath.Join(orqDir, "config.json"), []byte(`{"qa":{}}`), 0o644)
	wt := filepath.Join(t.TempDir(), "wt")
	git(main, "worktree", "add", "-q", "-b", "slice/s07", wt)
	os.WriteFile(filepath.Join(wt, "app.py"), []byte("print(2)\n"), 0o644)
	git(wt, "commit", "-qam", "feat")
	headWorker, _ := exec.Command("git", "-C", wt, "rev-parse", "HEAD").Output()
	a := Aislado{WorkerDir: wt, MainRoot: main, OrqDir: orqDir}
	corre := func(cmd string) Receipt {
		return RunAislado(a, "07", []Layer{{Name: "unit", Cmd: cmd}}, "medium", ShellRunner, t0, []string{"out.tap"})
	}

	// limpio: corre en una copia en el HEAD del worker (ve print(2)), y la copia desaparece.
	rec := corre(`grep -q 'print(2)' app.py && pwd > ` + filepath.Join(t.TempDir(), "x") + ` && echo 'ok 1' > out.tap`)
	if rec.Result != "pass" || rec.Isolation == nil || rec.Head != strings.TrimSpace(string(headWorker)) {
		t.Fatalf("limpio: %+v %+v", rec, rec.Tree)
	}
	if rec.Isolation.Copy == wt || rec.Isolation.Copy == main {
		t.Fatalf("no corrió en una copia: %s", rec.Isolation.Copy)
	}
	if _, err := os.Stat(rec.Isolation.Copy); !os.IsNotExist(err) {
		t.Fatalf("la copia sigue ahí: %v", err)
	}
	if out, _ := exec.Command("git", "-C", main, "worktree", "list").Output(); strings.Contains(string(out), "orq-qa-") {
		t.Fatalf("worktree de QA registrado: %s", out)
	}

	// cambiar y restaurar el worktree del worker: el examen corre en la copia, el worker no se toca.
	rec = corre(`python3 -c "p='app.py'; s=open(p).read(); open(p,'w').write('print(9)'); open(p,'w').write(s)" && echo 'ok 1' > out.tap`)
	if rec.Result != "pass" {
		t.Fatalf("cambiar y restaurar dentro de la copia: %+v", rec.Tree)
	}
	if raw, _ := os.ReadFile(filepath.Join(wt, "app.py")); string(raw) != "print(2)\n" {
		t.Fatalf("el worktree del worker cambió: %q", raw)
	}

	// escribir el examen del checkout principal (ruta absoluta), o .orq/config.json: invalid.
	for nombre, cmd := range map[string]string{
		"examen principal": `echo 'exit 0 # trampa' >> ` + filepath.Join(main, "tests", "exam.sh"),
		"config":           `python3 -c "open('` + filepath.Join(orqDir, "config.json") + `','w').write('{}')"`,
		"untracked main":   `touch ` + filepath.Join(main, "nuevo.txt"),
		"worker worktree":  `echo x > ` + filepath.Join(wt, "app.py"),
		"copia":            `echo 'print(5)' > app.py`,
	} {
		rec := corre(cmd)
		if rec.Result != "invalid" || len(rec.Tree.Changed) == 0 {
			t.Errorf("%s: %+v", nombre, rec.Tree)
		}
		git(main, "checkout", "-q", "--", ".")
		os.Remove(filepath.Join(main, "nuevo.txt"))
		os.WriteFile(filepath.Join(orqDir, "config.json"), []byte(`{"qa":{}}`), 0o644)
		git(wt, "checkout", "-q", "--", ".")
	}
}
