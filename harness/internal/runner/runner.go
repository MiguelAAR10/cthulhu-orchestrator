// Package runner separa a orq del multiplexor: herdr, tmux o un proceso hijo con log.
// Todos los sitios que abren paneles, arrancan agentes o les escriben pasan por Runner.
package runner

import (
	"errors"
	"os"
	"os/exec"
	"strings"
)

// Launch es cómo se arranca un agente; cada adaptador usa lo suyo.
// herdr: Kind + Args · tmux: CLI + Args (interactivo) · process: CLI + Headless + prompt.
type Launch struct {
	Kind       string   // tipo de agente para herdr (claude, codex…)
	CLI        string   // binario
	Args       []string // argumentos interactivos
	Headless   []string // argumentos sin TTY; {prompt} se reemplaza en modo "arg"
	PromptMode string   // "stdin" | "arg"
}

type Runner interface {
	Name() string
	Inside() bool    // ¿se puede usar desde aquí?
	Current() string // panel actual ("" si no aplica)
	Open(label, cwd string, env map[string]string) (tab, pane string, err error)
	Split(pane, cwd string, env map[string]string) (string, error)
	Start(name string, l Launch, pane string) error
	Rename(target, name string) error
	Prompt(target, text string) error
	Read(target string) (string, error)
	Alive(target string) bool
	Close(pane string) error
}

var ErrNoSoportado = errors.New("el runner no lo soporta")

// Select: ORQ_RUNNER gana, luego la config; por defecto herdr si estamos dentro o está instalado,
// si no un proceso hijo con log.
func Select(cfg string) Runner {
	name := os.Getenv("ORQ_RUNNER")
	if name == "" {
		name = cfg
	}
	switch name {
	case "herdr":
		return Herdr{}
	case "tmux":
		return &Tmux{}
	case "process":
		return &Process{}
	}
	if (Herdr{}).Inside() {
		return Herdr{}
	}
	if _, err := exec.LookPath("herdr"); err == nil {
		return Herdr{}
	}
	return &Process{}
}

// quote deja un argumento listo para una línea de shell.
func quote(s string) string {
	if s != "" && strings.IndexFunc(s, func(r rune) bool {
		return !(r == '-' || r == '_' || r == '.' || r == '/' || r == '=' || r == ':' || r == ',' || r == '@' || r == '+' ||
			(r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9'))
	}) < 0 {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func shellLine(argv []string) string {
	q := make([]string, len(argv))
	for i, a := range argv {
		q[i] = quote(a)
	}
	return strings.Join(q, " ")
}
