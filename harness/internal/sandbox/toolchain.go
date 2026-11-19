package sandbox

// Toolchains: the box sees the system (/usr, /bin, ...) but not $HOME, where go, node or uv usually
// live. For each tool a check names, the directory of every symlink hop from the PATH hit to the real
// binary, and the real binary's directory, are bound read-only,
// with the toolchain root when the tool sits in a bin/ (GOROOT, a node version). Never a directory
// that holds or sits inside credentials, agent config or orq's state.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"orq/internal/limits"
	"orq/internal/state"
)

// Herramienta es un comando que nombra un check y cómo lo ve la caja.
type Herramienta struct {
	Nombre string   // como aparece en el comando
	Ruta   string   // donde lo resuelve el PATH del host; "" si no está
	RO     []string // directorios que se montan de solo lectura para que corra
	Motivo string   // si no es "", la caja no lo va a ver: por qué
}

// palabrasShell no son herramientas: palabras clave y builtins de sh.
var palabrasShell = []string{"if", "then", "else", "elif", "fi", "for", "while", "until", "do", "done", "case", "esac", "in",
	"!", "{", "}", "[", "]", "[[", "]]", "test", "true", "false", "exit", "echo", "printf", "cd", "export", "set", "command", "exec", "eval", "read", "return", "shift", "trap", "wait", "c"}

// palabras devuelve las palabras de cmd que pueden ser un comando, y la primera de ellas.
func palabras(cmd string) []string {
	sep := func(r rune) bool { return strings.ContainsRune(" \t\n;|&()<>`'\"$", r) }
	var out []string
	for _, w := range strings.FieldsFunc(cmd, sep) {
		switch {
		case strings.ContainsAny(w, "=/*?.") || strings.HasPrefix(w, "-") || strings.Trim(w, "0123456789") == "":
		case slices.Contains(palabrasShell, w) || slices.Contains(out, w):
		default:
			out = append(out, w)
		}
	}
	return out
}

// buscar resuelve nombre en path como lo haría sh, sin mirar el directorio actual.
func buscar(nombre, path string) string {
	for _, d := range filepath.SplitList(path) {
		if !filepath.IsAbs(d) {
			continue
		}
		p := filepath.Join(d, nombre)
		if fi, err := os.Stat(p); err == nil && fi.Mode().IsRegular() && fi.Mode()&0o111 != 0 {
			return p
		}
	}
	return ""
}

// prohibidos son los directorios de $HOME que nunca entran en la caja, ni ellos ni un ancestro suyo.
func prohibidos(home string) []string {
	out := []string{filepath.Join(home, ".config"), filepath.Join(home, ".ssh"), filepath.Join(home, ".gnupg"),
		filepath.Join(home, ".local", "state"), filepath.Clean(state.Dir())}
	if es, err := os.ReadDir(home); err == nil {
		for _, e := range es {
			if strings.HasPrefix(e.Name(), ".claude") {
				out = append(out, filepath.Join(home, e.Name()))
			}
		}
	}
	return out
}

// tocaProhibido dice qué directorio prohibido expone d ("" si ninguno).
func tocaProhibido(d, home string, ps []string) string {
	if d == home || !bajo(d, home) {
		return home
	}
	if rel, err := filepath.Rel(home, d); err == nil && strings.HasPrefix(strings.Split(rel, "/")[0], ".claude") {
		return filepath.Join(home, strings.Split(rel, "/")[0])
	}
	for _, p := range ps {
		if bajo(d, p) || bajo(p, d) {
			return p
		}
	}
	return ""
}

// Herramientas dice, para cada herramienta que nombra cmd, qué montar para que la caja la vea.
func Herramientas(cmd string) []Herramienta {
	home, _ := os.UserHomeDir()
	home = filepath.Clean(home)
	path := os.Getenv("PATH")
	sistema := []string{"/usr", "/bin", "/lib", "/lib64", "/etc"}
	if l, err := limits.Load(limits.DefaultPath()); err == nil {
		sistema = append(sistema, l.Sandbox.RO...)
	}
	ps := prohibidos(home)
	var out []Herramienta
	for i, w := range palabras(cmd) {
		h := Herramienta{Nombre: w, Ruta: buscar(w, path)}
		if h.Ruta == "" {
			if i == 0 {
				h.Motivo = w + " no está en el PATH del host"
				out = append(out, h)
			}
			continue
		}
		links, real, err := saltos(h.Ruta)
		if err != nil {
			h.Motivo = fmt.Sprintf("%s: %v", w, err)
			out = append(out, h)
			continue
		}
		// Cada enlace del camino tiene que existir en la caja: se monta su directorio, y el del real.
		var dirs []string
		for _, l := range append(links, real) {
			if d := filepath.Dir(l); d != "/" { // /bin -> usr/bin: la caja ya los recrea
				dirs = append(dirs, d)
			}
		}
		for _, d := range dirs {
			switch {
			case slices.ContainsFunc(sistema, func(s string) bool { return bajo(d, filepath.Clean(s)) }):
			case !bajo(d, home):
				h.Motivo = fmt.Sprintf("%s está en %s, fuera de $HOME y del sistema: súmalo a sandbox.ro en limits.json", w, d)
			case tocaProhibido(d, home, ps) != "":
				h.Motivo = fmt.Sprintf("%s está en %s, que nunca entra en la caja (%s)", w, d, tocaProhibido(d, home, ps))
			default:
				if filepath.Base(d) == "bin" && tocaProhibido(filepath.Dir(d), home, ps) == "" {
					d = filepath.Dir(d) // la raíz del toolchain: GOROOT, una versión de node
				}
				if !slices.Contains(h.RO, d) {
					h.RO = append(h.RO, d)
				}
			}
		}
		if h.Motivo != "" {
			h.RO = nil
		}
		if h.RO != nil || h.Motivo != "" || i == 0 {
			out = append(out, h)
		}
	}
	return out
}

// saltos sigue p hasta el archivo real, componente por componente como el kernel, y devuelve cada
// enlace simbólico del camino (de archivo o de directorio), ya con sus padres resueltos, y el real.
func saltos(p string) (links []string, real string, err error) {
	p = filepath.Clean(p)
	for range 40 {
		partes := strings.Split(strings.TrimPrefix(p, "/"), "/")
		cur, siguio := "/", false
		for i, parte := range partes {
			cur = filepath.Join(cur, parte)
			fi, err := os.Lstat(cur)
			if err != nil {
				return nil, "", err
			}
			if fi.Mode()&os.ModeSymlink == 0 {
				continue
			}
			dst, err := os.Readlink(cur)
			if err != nil {
				return nil, "", err
			}
			if !filepath.IsAbs(dst) {
				dst = filepath.Join(filepath.Dir(cur), dst)
			}
			links = append(links, cur)
			p, siguio = filepath.Join(append([]string{dst}, partes[i+1:]...)...), true
			break
		}
		if !siguio {
			return links, p, nil
		}
	}
	return nil, "", fmt.Errorf("demasiados enlaces simbólicos en %s", p)
}

// RODe junta los directorios a montar, sin repetidos ni uno dentro de otro.
func RODe(hs []Herramienta) []string {
	var todos []string
	for _, h := range hs {
		todos = append(todos, h.RO...)
	}
	slices.Sort(todos)
	var out []string
	for _, d := range todos {
		if len(out) == 0 || !bajo(d, out[len(out)-1]) {
			out = append(out, d)
		}
	}
	return out
}

// NoCorren dice qué herramientas no va a ver la caja; "" si todas.
func NoCorren(hs []Herramienta) string {
	var xs []string
	for _, h := range hs {
		if h.Motivo != "" {
			xs = append(xs, h.Motivo)
		}
	}
	return strings.Join(xs, "; ")
}

// Visto es lo que vio una herramienta corriendo dentro de una caja real.
type Visto struct {
	Ruta   string // dónde la encontró command -v en la caja; "" si no la encontró
	Linea  string // la primera línea de su --version (o version)
	Exit   int    // 126 o 127: la caja no la puede ejecutar; -1: cortada
	Salida string
}

// Corre dice si la sonda llegó a ejecutar la herramienta.
func (s Visto) Corre() bool { return s.Ruta != "" && s.Exit >= 0 && s.Exit != 126 && s.Exit != 127 }

// sondaSh: command -v y luego --version; si falla sin ser 126/127, prueba `version` (go).
const sondaSh = `p=$(command -v "$1") || { echo "$1: no está en el PATH de la caja"; exit 127; }
echo "$p"
out=$("$1" --version </dev/null 2>&1); c=$?
if [ $c -ne 0 ] && [ $c -lt 126 ]; then o=$("$1" version </dev/null 2>&1) && { out=$o; c=0; }; fi
printf '%s\n' "$out"
exit $c`

// Sondear corre h dentro de una caja real, sin red, con su toolchain montada como al verificar.
func Sondear(h Herramienta) (Visto, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	c, err := Command(ctx, Spec{RO: h.RO, Env: map[string]string{"LANG": "C.UTF-8"}}, "sh", "-c", sondaSh, "sonda", h.Nombre)
	if err != nil {
		return Visto{}, err
	}
	var buf bytes.Buffer
	c.Stdout, c.Stderr = &buf, &buf
	errRun := c.Run()
	s := Visto{Salida: strings.TrimSpace(buf.String())}
	var ee *exec.ExitError
	switch {
	case ctx.Err() != nil:
		s.Exit = -1
	case errRun == nil:
	case errors.As(errRun, &ee):
		s.Exit = ee.ExitCode()
	default:
		return s, errRun
	}
	lineas := strings.Split(s.Salida, "\n")
	if s.Exit != 127 || filepath.IsAbs(lineas[0]) {
		s.Ruta, lineas = lineas[0], lineas[1:]
	}
	for _, l := range lineas {
		if l = strings.TrimSpace(l); l != "" {
			s.Linea = l
			break
		}
	}
	return s, nil
}
