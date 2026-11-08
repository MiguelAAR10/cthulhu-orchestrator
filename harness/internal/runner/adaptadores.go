package runner

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"orq/internal/herdr"
)

// ---------- herdr: el código de siempre, detrás de la interfaz ----------

type Herdr struct{}

func (Herdr) Name() string    { return "herdr" }
func (Herdr) Inside() bool    { return herdr.Inside() }
func (Herdr) Current() string { return herdr.CurrentPane() }
func (Herdr) Open(label, cwd string, env map[string]string) (string, string, error) {
	return herdr.Tab(label, cwd, env)
}
func (Herdr) Split(pane, cwd string, env map[string]string) (string, error) {
	return herdr.SplitDe(pane, cwd, env)
}
func (Herdr) Start(name string, l Launch, pane string) error {
	kind := l.Kind
	if kind == "" {
		kind = l.CLI
	}
	return herdr.Start(name, kind, pane, l.Args)
}
func (Herdr) Rename(target, name string) error { return herdr.Rename(target, name) }
func (Herdr) Prompt(target, text string) error { return herdr.Prompt(target, text) }
func (Herdr) Read(string) (string, error)      { return "", ErrNoSoportado }
func (Herdr) Alive(target string) bool {
	_, err := herdr.Run("agent", "get", target)
	return err == nil
}
func (Herdr) Close(pane string) error { return herdr.ClosePane(pane) }

// ---------- tmux ----------

// Tmux usa ventanas como pestañas y el título del panel como nombre del agente.
type Tmux struct {
	Exec func(args ...string) ([]byte, error) // se reemplaza en pruebas
}

func (t *Tmux) run(args ...string) ([]byte, error) {
	if t.Exec != nil {
		return t.Exec(args...)
	}
	out, err := exec.Command("tmux", args...).CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("tmux %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return out, nil
}

func envArgs(env map[string]string) []string {
	var ks []string
	for k := range env {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	var a []string
	for _, k := range ks {
		a = append(a, "-e", k+"="+env[k])
	}
	return a
}

func (t *Tmux) Name() string    { return "tmux" }
func (t *Tmux) Inside() bool    { return os.Getenv("TMUX") != "" }
func (t *Tmux) Current() string { return os.Getenv("TMUX_PANE") }

func (t *Tmux) Open(label, cwd string, env map[string]string) (string, string, error) {
	args := append([]string{"new-window", "-d", "-P", "-F", "#{window_id} #{pane_id}", "-n", label, "-c", cwd}, envArgs(env)...)
	out, err := t.run(args...)
	if err != nil {
		return "", "", err
	}
	f := strings.Fields(string(out))
	if len(f) != 2 {
		return "", "", fmt.Errorf("tmux new-window: respuesta inesperada: %q", out)
	}
	return f[0], f[1], nil
}

func (t *Tmux) Split(pane, cwd string, env map[string]string) (string, error) {
	args := append([]string{"split-window", "-h", "-d", "-t", pane, "-P", "-F", "#{pane_id}", "-c", cwd}, envArgs(env)...)
	out, err := t.run(args...)
	if err != nil {
		return "", err
	}
	f := strings.Fields(string(out))
	if len(f) == 0 {
		return "", fmt.Errorf("tmux split-window: respuesta vacía")
	}
	return f[len(f)-1], nil
}

func (t *Tmux) Start(name string, l Launch, pane string) error {
	if err := t.Rename(pane, name); err != nil {
		return err
	}
	_, err := t.run("send-keys", "-t", pane, shellLine(append([]string{l.CLI}, l.Args...)), "Enter")
	return err
}

func (t *Tmux) Rename(target, name string) error {
	_, err := t.run("select-pane", "-t", target, "-T", name)
	return err
}

// resolver: un id de panel (%N) se usa tal cual; un nombre se busca por título.
func (t *Tmux) resolver(target string) (string, error) {
	if strings.HasPrefix(target, "%") {
		return target, nil
	}
	out, err := t.run("list-panes", "-a", "-F", "#{pane_id} #{pane_title}")
	if err != nil {
		return "", err
	}
	for _, l := range strings.Split(string(out), "\n") {
		if id, title, ok := strings.Cut(strings.TrimSpace(l), " "); ok && title == target {
			return id, nil
		}
	}
	return "", fmt.Errorf("tmux: no hay panel %q", target)
}

func (t *Tmux) Prompt(target, text string) error {
	p, err := t.resolver(target)
	if err != nil {
		return err
	}
	if _, err := t.run("send-keys", "-t", p, "-l", text); err != nil {
		return err
	}
	_, err = t.run("send-keys", "-t", p, "Enter")
	return err
}

func (t *Tmux) Read(target string) (string, error) {
	p, err := t.resolver(target)
	if err != nil {
		return "", err
	}
	out, err := t.run("capture-pane", "-p", "-t", p)
	return string(out), err
}

func (t *Tmux) Alive(target string) bool { _, err := t.resolver(target); return err == nil }
func (t *Tmux) Close(pane string) error  { _, err := t.run("kill-pane", "-t", pane); return err }

// ---------- process: sin multiplexor ----------

// Proc es un arranque concreto de un proceso hijo.
type Proc struct {
	Argv  []string
	Dir   string
	Env   []string
	Stdin string
	Log   string
}

// Process corre cada agente como proceso hijo en modo headless; la salida va a un log.
// Cada Prompt es una corrida nueva (el agente no conserva conversación entre prompts).
type Process struct {
	Dir   string                  // registro y logs; por defecto $TMPDIR/orq-process
	Spawn func(Proc) (int, error) // se reemplaza en pruebas
}

type procPanel struct {
	Cwd string            `json:"cwd"`
	Env map[string]string `json:"env,omitempty"`
}

type procAgente struct {
	Pane   string `json:"pane"`
	Launch Launch `json:"launch"`
	PID    int    `json:"pid,omitempty"`
	Log    string `json:"log"`
}

func (p *Process) dir() string {
	if p.Dir != "" {
		return p.Dir
	}
	if d := os.Getenv("ORQ_PROCESS_DIR"); d != "" {
		return d
	}
	return filepath.Join(os.TempDir(), "orq-process")
}

func (p *Process) guardar(nombre string, v any) error {
	os.MkdirAll(p.dir(), 0o755)
	raw, _ := json.MarshalIndent(v, "", "  ")
	return os.WriteFile(filepath.Join(p.dir(), nombre+".json"), raw, 0o644)
}

func (p *Process) leer(nombre string, v any) error {
	raw, err := os.ReadFile(filepath.Join(p.dir(), nombre+".json"))
	if err != nil {
		return fmt.Errorf("process: no hay %q", nombre)
	}
	return json.Unmarshal(raw, v)
}

func (p *Process) Name() string    { return "process" }
func (p *Process) Inside() bool    { return true }
func (p *Process) Current() string { return "" }

func (p *Process) Open(label, cwd string, env map[string]string) (string, string, error) {
	pane := fmt.Sprintf("proc-%d", time.Now().UnixNano())
	return label, pane, p.guardar(pane, procPanel{Cwd: cwd, Env: env})
}

func (p *Process) Split(_ string, cwd string, env map[string]string) (string, error) {
	_, pane, err := p.Open("", cwd, env)
	return pane, err
}

func (p *Process) Start(name string, l Launch, pane string) error {
	var pp procPanel
	if err := p.leer(pane, &pp); err != nil {
		return err
	}
	return p.guardar("agente-"+name, procAgente{Pane: pane, Launch: l, Log: filepath.Join(p.dir(), name+".log")})
}

func (p *Process) Rename(string, string) error { return nil }

func (p *Process) Prompt(target, text string) error {
	var a procAgente
	if err := p.leer("agente-"+target, &a); err != nil {
		return err
	}
	var pp procPanel
	if err := p.leer(a.Pane, &pp); err != nil {
		return err
	}
	argv := []string{a.Launch.CLI}
	stdin := ""
	for _, x := range a.Launch.Headless {
		if a.Launch.PromptMode == "arg" {
			x = strings.ReplaceAll(x, "{prompt}", text)
		}
		argv = append(argv, x)
	}
	if a.Launch.PromptMode != "arg" {
		stdin = text
	}
	env := os.Environ()
	for k, v := range pp.Env {
		env = append(env, k+"="+v)
	}
	spawn := p.Spawn
	if spawn == nil {
		spawn = spawnReal
	}
	pid, err := spawn(Proc{Argv: argv, Dir: pp.Cwd, Env: env, Stdin: stdin, Log: a.Log})
	if err != nil {
		return err
	}
	a.PID = pid
	return p.guardar("agente-"+target, a)
}

func spawnReal(pr Proc) (int, error) {
	log, err := os.OpenFile(pr.Log, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return 0, err
	}
	defer log.Close()
	cmd := exec.Command(pr.Argv[0], pr.Argv[1:]...)
	cmd.Dir, cmd.Env, cmd.Stdout, cmd.Stderr = pr.Dir, pr.Env, log, log
	cmd.Stdin = strings.NewReader(pr.Stdin)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	pid := cmd.Process.Pid
	go cmd.Wait()
	return pid, nil
}

func (p *Process) Read(target string) (string, error) {
	var a procAgente
	if err := p.leer("agente-"+target, &a); err != nil {
		return "", err
	}
	raw, err := os.ReadFile(a.Log)
	return string(raw), err
}

func (p *Process) Alive(target string) bool {
	var a procAgente
	if p.leer("agente-"+target, &a) != nil || a.PID == 0 {
		return false
	}
	return syscall.Kill(a.PID, 0) == nil
}

// Close mata los procesos de los agentes de ese panel y borra su registro.
func (p *Process) Close(pane string) error {
	ms, _ := filepath.Glob(filepath.Join(p.dir(), "agente-*.json"))
	for _, m := range ms {
		var a procAgente
		raw, _ := os.ReadFile(m)
		if json.Unmarshal(raw, &a) == nil && a.Pane == pane {
			if a.PID > 0 {
				syscall.Kill(-a.PID, syscall.SIGTERM)
			}
			os.Remove(m)
		}
	}
	return os.Remove(filepath.Join(p.dir(), pane+".json"))
}

// ---------- Fake para pruebas ----------

type Fake struct {
	Calls  []string
	Dentro bool
	Envs   []map[string]string // env de cada Open/Split, en orden
	n      int
}

func (f *Fake) log(s string)    { f.Calls = append(f.Calls, s) }
func (f *Fake) Name() string    { return "fake" }
func (f *Fake) Inside() bool    { return true }
func (f *Fake) Current() string { return "fake:0" }
func (f *Fake) Open(label, cwd string, env map[string]string) (string, string, error) {
	f.n++
	f.Envs = append(f.Envs, env)
	f.log("open " + label + " " + cwd)
	return "tab", fmt.Sprintf("fake:%d", f.n), nil
}
func (f *Fake) Split(pane, cwd string, env map[string]string) (string, error) {
	f.n++
	f.Envs = append(f.Envs, env)
	f.log("split " + pane + " " + cwd)
	return fmt.Sprintf("fake:%d", f.n), nil
}
func (f *Fake) Start(name string, l Launch, pane string) error {
	f.log("start " + name + " " + l.Kind + " " + strings.Join(l.Args, " "))
	return nil
}
func (f *Fake) Rename(t, n string) error    { f.log("rename " + t + " " + n); return nil }
func (f *Fake) Prompt(t, text string) error { f.log("prompt " + t + " " + text); return nil }
func (f *Fake) Read(string) (string, error) { return "", nil }
func (f *Fake) Alive(string) bool           { return true }
func (f *Fake) Close(p string) error        { f.log("close " + p); return nil }
