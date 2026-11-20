// Package identidad decides which orq this binary is. One name decides everything: an executable
// named orq2 (or ORQ_NOMBRE=orq2) is the side-by-side v2; any other name is orq, as always.
// orq2 writes hooks with its own absolute path, says orq2 in every text it gives an agent or a
// person, and keeps its config and state under ~/.config/orq2 and ~/.local/state/orq2, so it
// never reads or writes what the live orq 1.0.0 uses.
package identidad

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	Orq  = "orq"
	Orq2 = "orq2"
)

// Nombre is orq2 when ORQ_NOMBRE=orq2 or the running executable is called orq2; orq otherwise.
// os.Args[0] is checked first because os.Executable resolves symlinks: ~/bin/orq2 -> dist/orq is orq2.
func Nombre() string {
	if n := os.Getenv("ORQ_NOMBRE"); n != "" {
		if n == Orq2 {
			return Orq2
		}
		return Orq
	}
	if len(os.Args) > 0 && filepath.Base(os.Args[0]) == Orq2 {
		return Orq2
	}
	if e, err := os.Executable(); err == nil && filepath.Base(e) == Orq2 {
		return Orq2
	}
	return Orq
}

// EsOrq2 says whether this binary is the side-by-side v2.
func EsOrq2() bool { return Nombre() == Orq2 }

// Binario is how a hook calls this binary. orq keeps "$HOME/bin/orq"; orq2 uses the absolute path
// of the running binary, with ORQ_NOMBRE=orq2 in front when that file is not itself named orq2.
func Binario() string {
	if !EsOrq2() {
		return "$HOME/bin/orq"
	}
	ruta := rutaPropia()
	cmd := citar(ruta)
	if filepath.Base(ruta) != Orq2 {
		cmd = "ORQ_NOMBRE=orq2 " + cmd
	}
	return cmd
}

// rutaPropia keeps the name the binary was called by (a symlink ~/bin/orq2 stays ~/bin/orq2).
func rutaPropia() string {
	if len(os.Args) > 0 && os.Args[0] != "" {
		a := os.Args[0]
		if !strings.ContainsRune(a, '/') {
			if p, err := exec.LookPath(a); err == nil {
				a = p
			}
		}
		if strings.ContainsRune(a, '/') {
			if abs, err := filepath.Abs(a); err == nil {
				if _, err := os.Stat(abs); err == nil {
					return abs
				}
			}
		}
	}
	e, _ := os.Executable()
	return e
}

func citar(s string) string {
	if s != "" && strings.Trim(s, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789/._-+") == "" {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// orqComando is orq used as a command: a whole word followed by a space and a subcommand.
// Paths (~/.config/orq/, .orq/, bin/orq), skill names (orq-planner) and tags ([orq · ...]) stay.
var orqComando = regexp.MustCompile(`(^|[^A-Za-z0-9_./~$-])orq( [a-z-])`)

// rutasOrq are the user-level directories orq2 does not use: it has its own under orq2.
var rutasOrq = strings.NewReplacer("~/.config/orq/", "~/.config/orq2/", "~/.local/state/orq/", "~/.local/state/orq2/")

// Texto rewrites "orq <sub>" into "orq2 <sub>" and the user-level dirs to their orq2 names for
// orq2; for orq it returns s untouched. Project paths (.orq/) and tags stay.
func Texto(s string) string {
	if !EsOrq2() {
		return s
	}
	return orqComando.ReplaceAllString(rutasOrq.Replace(s), "${1}orq2${2}")
}

// skillOrq is a skill name orq ships: orq-atena, orq-orchestrator, orq-planner, orq-ruta, orq-worker.
var skillOrq = regexp.MustCompile(`(^|[^A-Za-z0-9_-])orq-(atena|orchestrator|planner|ruta|worker)\b`)

// Skills renames the orq skills in s to orq2-<name> for orq2, so a personal orq 1.0.0 skill with
// the orq-<name> name can never stand in for orq2's; for orq it returns s untouched.
func Skills(s string) string {
	if !EsOrq2() {
		return s
	}
	return skillOrq.ReplaceAllString(s, "${1}orq2-${2}")
}

// Escritor rewrites what passes through it with Texto; for orq it is w itself.
func Escritor(w io.Writer) io.Writer {
	if !EsOrq2() {
		return w
	}
	return escritor{w}
}

type escritor struct{ w io.Writer }

func (e escritor) Write(p []byte) (int, error) {
	if _, err := io.WriteString(e.w, Texto(string(p))); err != nil {
		return 0, err
	}
	return len(p), nil
}

// ConfigDir is ~/.config/<name>; StateDir is ~/.local/state/<name>.
func ConfigDir() string {
	h, _ := os.UserHomeDir()
	return filepath.Join(h, ".config", Nombre())
}

func StateDir() string {
	h, _ := os.UserHomeDir()
	return filepath.Join(h, ".local", "state", Nombre())
}
