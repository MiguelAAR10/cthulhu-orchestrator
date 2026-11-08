package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"orq/internal/config"
	"orq/internal/identidad"
	"orq/internal/memoria"
	"orq/internal/runner"
	"orq/internal/worktrees"
)

// construirOrq compila el binario real de orq para pruebas de procesos.
func construirOrq(t *testing.T) string {
	t.Helper()
	if _, err := os.Stat("/proc/self/environ"); err != nil {
		t.Skip("sin /proc: la comprobación de ancestros solo existe en Linux")
	}
	bin := filepath.Join(t.TempDir(), "orq")
	c := exec.Command("go", "build", "-o", bin, "orq/cmd/orq")
	if out, err := c.CombinedOutput(); err != nil {
		t.Skipf("go build: %v %s", err, out)
	}
	return bin
}

// El exploit verificado del evaluador: un worker lee el token, borra ORQ_* de su entorno, se declara
// owner con el token y llama a orq por una ruta que el gate no ve. El runner process lo lanza como
// worker de verdad (ORQ_WORKER_ID en su entorno inicial) y orq lo rechaza por su ancestro.
func TestExploitDeWorkerRechazadoPorAncestro(t *testing.T) {
	bin := construirOrq(t)
	root := proyectoVis(t)
	exec.Command("git", "-C", root, "init", "-q").Run()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "cfg"))
	tok, err := identidad.Asegurar()
	if err != nil {
		t.Fatal(err)
	}
	identidad.Anclar(identidad.Tabla, os.Getpid(), "owner") // control del Owner sin depender de una TTY
	d := filepath.Dir(bin)
	exploit := `T=$(cat ` + identidad.Path() + `); unset "${!ORQ@}"; export $(printf "ORQ_%s=%s ORQ_%s=%s" ROL owner TOKEN $T); export ORQ_ROOT=` + root + `; ` +
		`P=` + d + `; $P/orq mem add --source owner --cat decision --imp alta "PWNED"; echo mem=$?; ` +
		`$P/orq autonomy free --scope src; echo aut=$?; $P/orq token >/dev/null; echo tok=$?; echo FIN`

	pr := &runner.Process{Dir: t.TempDir()}
	_, pane, err := pr.Open("S-04", root, map[string]string{"ORQ_ROL": "w1", "ORQ_WORKER_ID": "w1", "ORQ_TOKEN": ""})
	if err != nil {
		t.Fatal(err)
	}
	if err := pr.Start("w1", runner.Launch{CLI: "bash", Headless: []string{"-c", "{prompt}"}, PromptMode: "arg"}, pane); err != nil {
		t.Fatal(err)
	}
	if err := pr.Prompt("w1", exploit); err != nil {
		t.Fatal(err)
	}
	var log string
	for i := 0; i < 200 && !strings.Contains(log, "FIN"); i++ {
		time.Sleep(50 * time.Millisecond)
		log, _ = pr.Read("w1")
	}
	for _, want := range []string{"mem=2", "aut=2", "tok=2", "worker cannot run"} {
		if !strings.Contains(log, want) {
			t.Fatalf("falta %q:\n%s", want, log)
		}
	}
	if items, _ := memoria.Load(filepath.Join(root, ".orq", "memoria.jsonl")); len(items) != 0 {
		t.Fatalf("memoria forjada: %+v", items)
	}
	if cfg, _ := config.Load(filepath.Join(root, ".orq")); cfg.Mode() == "free" {
		t.Fatal("autonomía cambiada por un worker")
	}

	// Control: el mismo comando sin un worker en la cadena de padres sí vale (Owner con token).
	c := exec.Command(bin, "autonomy", "free", "--scope", "src")
	c.Dir = root
	c.Env = append(os.Environ(), "ORQ_ROL=owner", "ORQ_TOKEN="+tok, "ORQ_WORKER_ID=", "ORQ_ROOT="+root)
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("el Owner no pudo: %v %s", err, out)
	}
	if cfg, _ := config.Load(filepath.Join(root, ".orq")); cfg.Mode() != "free" {
		t.Fatal("control: el Owner no cambió la autonomía")
	}
}

// Un cwd dentro de un worktree que orq creó para un slice es un worker, aunque falte todo ORQ_*.
func TestCwdEnWorktreeDeSliceEsWorker(t *testing.T) {
	repo := t.TempDir()
	for _, a := range [][]string{{"init", "-q"}, {"-c", "user.email=a@b", "-c", "user.name=a", "commit", "-q", "--allow-empty", "-m", "x"}} {
		if out, err := exec.Command("git", append([]string{"-C", repo}, a...)...).CombinedOutput(); err != nil {
			t.Fatal(err, string(out))
		}
	}
	wt := filepath.Join(t.TempDir(), "wt")
	if err := worktrees.Crear(repo, filepath.Join(repo, ".orq"), worktrees.Entrada{Path: wt, Kind: worktrees.KindSlice, Slice: "S-01"}, "-b", "slice/s01"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ORQ_WORKER_ID", "")
	t.Setenv("ORQ_SHIM", "")
	t.Chdir(repo)
	if _, w := workerEfectivo(); w {
		t.Fatal("el checkout principal no es worker")
	}
	os.MkdirAll(filepath.Join(wt, "sub"), 0o755)
	t.Chdir(filepath.Join(wt, "sub"))
	if id, w := workerEfectivo(); !w || id != "worktree" {
		t.Fatalf("worktree de slice: %q %v", id, w)
	}
}

// msg send: un worker (aquí, por estar en un worktree enlazado) no manda un cierre aunque pase
// la lista blanca; sí manda estado, firmado como worker.
func TestWorkerNoMandaCierre(t *testing.T) {
	root := proyectoVis(t)
	repo := t.TempDir()
	exec.Command("git", "-C", repo, "init", "-q").Run()
	exec.Command("git", "-C", repo, "-c", "user.email=a@b", "-c", "user.name=a", "commit", "-q", "--allow-empty", "-m", "x").Run()
	wt := filepath.Join(t.TempDir(), "wt")
	if out, err := exec.Command("git", "-C", repo, "worktree", "add", "-q", wt).CombinedOutput(); err != nil {
		t.Fatal(err, string(out))
	}
	t.Chdir(wt)
	t.Setenv("ORQ_ROL", "orquestador")
	for _, tipo := range []string{"cierre", "closing", "historia", "plan"} {
		if _, errs, code := run(t, "msg", "send", "-para", "orquestador", "-tipo", tipo, "-intento", "a1", "listo"); code == 0 {
			t.Errorf("worker mandó %s: %s", tipo, errs)
		}
	}
	if _, errs, code := run(t, "msg", "send", "--para", "orquestador", "--tipo", "estado", "voy"); code != 0 {
		t.Fatal(errs)
	}
	raw, _ := os.ReadFile(filepath.Join(root, ".orq", "canal.jsonl"))
	if strings.Contains(string(raw), "cierre") || !strings.Contains(string(raw), `"worker-worktree"`) {
		t.Fatalf("canal: %s", raw)
	}
}

// El escape que dejaba abierto la comprobación de ancestros: el worker hace doble fork + setsid en
// Python, el nieto queda reparentado a init (o a un subreaper) sin TTY, limpia su entorno y llama a
// orq como owner con el token robado. Sin ancla registrada ni terminal de persona en su cadena,
// orq lo rechaza: la identidad privilegiada falla cerrada.
func TestHuerfanoSetsidConTokenRobadoRechazado(t *testing.T) {
	bin := construirOrq(t)
	py, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("sin python3")
	}
	root := proyectoVis(t)
	exec.Command("git", "-C", root, "init", "-q").Run()
	cfg := filepath.Join(t.TempDir(), "cfg")
	t.Setenv("XDG_CONFIG_HOME", cfg)
	if _, err := identidad.Asegurar(); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "out")
	code := `import os,time
tok=open(` + "'" + identidad.Path() + "'" + `).read().strip()
if os.fork()==0:
    os.setsid()
    if os.fork()==0:
        time.sleep(0.3)
        env={"PATH":"/usr/bin:/bin","HOME":os.environ.get("HOME","/"),"XDG_CONFIG_HOME":"` + cfg + `","ORQ_ROL":"owner","ORQ_TOKEN":tok,"ORQ_ROOT":"` + root + `","ORQ_LANG":"en"}
        sh="; ".join("%s >/dev/null 2>>%s.err; echo %s=$?" % (c, "` + out + `", c.split()[1]) for c in ("` + bin + ` mem add --source owner --cat decision --imp alta PWNED","` + bin + ` autonomy free --scope src","` + bin + ` token","` + bin + ` session register"))
        os.execve("/bin/sh",["sh","-c","exec >` + out + `; cd ` + root + `; "+sh+"; cat ` + out + `.err; echo FIN"],env)
        os._exit(0)
    os._exit(0)
os.wait()
print("lanzado")`

	pr := &runner.Process{Dir: t.TempDir()}
	_, pane, err := pr.Open("S-04", root, map[string]string{"ORQ_ROL": "w1", "ORQ_WORKER_ID": "w1", "ORQ_TOKEN": ""})
	if err != nil {
		t.Fatal(err)
	}
	if err := pr.Start("w1", runner.Launch{CLI: py, Headless: []string{"-c", "{prompt}"}, PromptMode: "arg"}, pane); err != nil {
		t.Fatal(err)
	}
	if err := pr.Prompt("w1", code); err != nil {
		t.Fatal(err)
	}
	var log string
	for i := 0; i < 200 && !strings.Contains(log, "FIN"); i++ {
		time.Sleep(50 * time.Millisecond)
		raw, _ := os.ReadFile(out)
		log = string(raw)
	}
	if !strings.Contains(log, "FIN") {
		wl, _ := pr.Read("w1")
		t.Fatalf("el huérfano no terminó: %q / %s", log, wl)
	}
	for _, l := range strings.Split(strings.TrimSpace(log), "\n")[:4] {
		if strings.HasSuffix(l, "=0") {
			t.Errorf("el huérfano pudo: %s", l)
		}
	}
	t.Log(log)
	if !strings.Contains(log, "no registered privileged session") {
		t.Errorf("rechazo sin motivo de ancla/terminal:\n%s", log)
	}
	if items, _ := memoria.Load(filepath.Join(root, ".orq", "memoria.jsonl")); len(items) != 0 {
		t.Fatalf("memoria forjada: %+v", items)
	}
	if c, _ := config.Load(filepath.Join(root, ".orq")); c.Mode() == "free" {
		t.Fatal("autonomía cambiada por un huérfano")
	}
}
