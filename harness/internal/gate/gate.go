// Package gate decide si una herramienta puede ejecutarse.
// Falla cerrado: ante duda, dato ausente o estado viejo, deniega el lanzamiento.
// Solo gobierna lanzamientos de workers y escrituras a archivos protegidos;
// el resto de herramientas pasan sin tocar.
package gate

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"orq/internal/admision"
	"orq/internal/limits"
)

type Payload struct {
	SessionID string         `json:"session_id"`
	Cwd       string         `json:"cwd"`
	Event     string         `json:"hook_event_name"`
	ToolName  string         `json:"tool_name"`
	ToolInput map[string]any `json:"tool_input"`
	AgentID   string         `json:"agent_id"`
}

// Env inyecta lo que depende del sistema, para probar el gate sin tocar el disco real.
type Env struct {
	DiskFreeGB func(string) (float64, error)
	Workers    func() (int, error)
	Now        func() time.Time
	StateDir   string
	IsWorktree func(string) bool
	// Bundle devuelve el paquete del examen del slice abierto (rutas relativas a la raíz), si lo hay.
	Bundle func(cwd string) []string
	// Worktrees cuenta los worktrees que orq registró en el proyecto de cwd (max_worktrees).
	Worktrees func(cwd string) (int, error)
}

type Decision struct {
	Allow  bool
	Reason string
}

func allow() Decision                  { return Decision{Allow: true} }
func deny(f string, a ...any) Decision { return Decision{Reason: fmt.Sprintf(f, a...)} }

// launchRe reconoce comandos que crean un worker, un panel o una copia de trabajo,
// en herdr, en Orca (`orca` es un alias de `orca-ide`) o directo.
var launchRe = regexp.MustCompile(
	`(^|[\s;&|(])(` +
		`herdr\s+(agent\s+(start|prompt)|pane\s+(run|split)|worktree\s+create)` +
		`|orca(-ide)?\s+(agent\s+launch|orchestration\s+(worker-start|dispatch|coordinator-start)|terminal\s+(create|split)|worktree\s+create)` +
		`|codex(\s|$)` +
		`|claude(\s+\S+)*\s+(-p|--print)(\s|$))`)

func Evaluate(p Payload, l limits.Limits, env Env) Decision {
	if p.ToolName == "Bash" {
		cmd, _ := p.ToolInput["command"].(string)
		if pushPermitido(cmd, p.Cwd, l.PushRamas) {
			return allow()
		}
		p.ToolInput = map[string]any{"command": Normalizar(cmd)}
	}
	if d, ok := destructivo(p); ok {
		return d
	}
	if d, ok := soloOrquestador(p, env); ok {
		return d
	}
	if target, ok := protectedTarget(p, l, env); ok {
		return deny("orq: %q está protegido dentro de un worktree. Solo el orquestador, desde el checkout principal, lo edita.", target)
	}
	if !isLaunch(p) {
		return allow()
	}
	return evaluateLaunch(p, l, env)
}

func isLaunch(p Payload) bool {
	switch p.ToolName {
	case "Agent", "Task": // Task es el nombre anterior de la herramienta Agent
		return true
	case "Bash":
		cmd, _ := p.ToolInput["command"].(string)
		return launchRe.MatchString(cmd)
	}
	return false
}

// evaluateLaunch delega en admision, la misma puerta que usa `orq slice lanzar`.
func evaluateLaunch(p Payload, l limits.Limits, env Env) Decision {
	if p.AgentID != "" {
		return deny("orq: un subagente no lanza workers. Devuelve el resultado al orquestador.")
	}
	ae := admision.Env{DiskFreeGB: env.DiskFreeGB, Workers: env.Workers, Now: env.Now, StateDir: env.StateDir}
	if env.Worktrees != nil {
		ae.Worktrees = func() (int, error) { return env.Worktrees(p.Cwd) }
	}
	if err := admision.Evaluar(l, ae); err != nil {
		return deny("%s.", err.Error())
	}
	return allow()
}

// protectedTarget detecta escrituras a rutas protegidas desde un worktree,
// tanto con Edit/Write como con redirecciones, comandos e intérpretes en Bash.
// Las rutas se resuelven de verdad: contra el cwd, tras `cd`, y siguiendo enlaces simbólicos.
func protectedTarget(p Payload, l limits.Limits, env Env) (string, bool) {
	if !env.IsWorktree(p.Cwd) {
		return "", false
	}
	pats := append([]string{}, l.Protected...)
	if env.Bundle != nil {
		pats = append(pats, env.Bundle(p.Cwd)...)
	}
	if len(pats) == 0 {
		return "", false
	}
	switch p.ToolName {
	case "Edit", "Write", "MultiEdit", "NotebookEdit":
		for _, k := range []string{"file_path", "notebook_path"} {
			if fp, _ := p.ToolInput[k].(string); fp != "" && matchesProtected(resolver(p.Cwd, fp), pats) {
				return fp, true
			}
		}
	case "Bash":
		cmd, _ := p.ToolInput["command"].(string)
		for _, t := range shellWriteTargets(cmd, p.Cwd) {
			if matchesProtected(t, pats) {
				return t, true
			}
		}
		if t, ok := interpreteEscribe(cmd, pats); ok {
			return t, true
		}
	}
	return "", false
}

// resolver devuelve la ruta absoluta y limpia, con los enlaces simbólicos resueltos
// en la parte que ya existe.
func resolver(cwd, p string) string {
	p = strings.Trim(p, `'"`)
	if strings.HasPrefix(p, "~/") {
		if h, err := os.UserHomeDir(); err == nil {
			p = filepath.Join(h, p[2:])
		}
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(cwd, p)
	}
	p = filepath.Clean(p)
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	if r, err := filepath.EvalSymlinks(filepath.Dir(p)); err == nil {
		return filepath.Join(r, filepath.Base(p))
	}
	return p
}

// redirRe captura el destino de >, >>, 2>, &>, >|. Destinos como &1 no se capturan.
var redirRe = regexp.MustCompile(`(?:\d|&)?>>?\|?\s*([^\s;&|<>()]+)`)

// segRe parte un comando en segmentos simples.
var segRe = regexp.MustCompile(`\|\||&&|[;|&\n()]`)

// shellWriteTargets devuelve, ya resueltas, las rutas que un comando puede escribir, mover o borrar.
// Sigue los `cd` dentro del mismo comando. Es una primera barrera; la garantía es la huella al verificar.
func shellWriteTargets(cmd, cwd string) []string {
	var out []string
	dir := cwd
	for _, seg := range segRe.Split(cmd, -1) {
		for _, m := range redirRe.FindAllStringSubmatch(seg, -1) {
			out = append(out, resolver(dir, m[1]))
		}
		f := stripPrefixes(strings.Fields(seg))
		if len(f) == 0 {
			continue
		}
		args := nonFlags(f[1:])
		var ts []string
		switch f[0] {
		case "cd", "pushd":
			if len(args) > 0 {
				dir = resolver(dir, args[0])
			}
		case "tee", "rm", "mv", "truncate", "unlink", "shred", "touch", "chmod":
			ts = args
		case "ln": // un enlace hacia lo protegido es una puerta para escribirlo
			ts = args
		case "cp", "install", "rsync":
			if len(args) > 0 {
				ts = args[len(args)-1:]
			}
		case "dd":
			for _, a := range f[1:] {
				if v, ok := strings.CutPrefix(a, "of="); ok {
					ts = append(ts, v)
				}
			}
		case "sed", "perl":
			if inPlace(f[1:]) {
				ts = args
			}
		case "git":
			if len(args) > 0 {
				switch args[0] {
				case "checkout", "restore", "rm", "mv", "reset", "apply", "clean":
					ts = args[1:]
				}
			}
		}
		for _, t := range ts {
			out = append(out, resolver(dir, t))
		}
	}
	return out
}

// interpreteRe reconoce código en línea: python -c, node -e, perl -e, ruby -e.
var interpreteRe = regexp.MustCompile(`(^|[\s;&|(])(python3?|node|perl|ruby|php)\s+(-[a-zA-Z]*[ce])\s`)

// interpreteEscribe: si hay código en línea que nombra una ruta protegida, se trata como escritura.
func interpreteEscribe(cmd string, pats []string) (string, bool) {
	if !interpreteRe.MatchString(cmd) {
		return "", false
	}
	for _, pat := range pats {
		if strings.Contains(cmd, strings.TrimSuffix(pat, "/")) {
			return pat, true
		}
	}
	return "", false
}

var (
	comillasRe = regexp.MustCompile("[\"'`\\\\]")
	rutaBinRe  = regexp.MustCompile(`(^|[\s;&|(])\S*/(orq|git|rm|herdr|orca|orca-ide|codex|claude)\b`)
	gitOptRe   = regexp.MustCompile(`\bgit((\s+(-C|-c|--git-dir|--work-tree|--namespace)(\s+|=)\S+)|(\s+--(no-pager|bare|no-replace-objects|literal-pathspecs)))+`)
	orqOptRe   = regexp.MustCompile(`\borq((\s+--(slice|repo)\s+\S+))+`)
	sliceOptRe = regexp.MustCompile(`(\bslice)(\s+--slice\s+\S+)`)
	rmLargoRe  = regexp.MustCompile(`--recursive|--force`)
)

// Normalizar quita lo que sirve para esquivar los patrones: comillas (sh -c '...'),
// rutas absolutas a binarios y opciones globales antes del subcomando.
func Normalizar(cmd string) string {
	c := comillasRe.ReplaceAllString(cmd, " ")
	c = rutaBinRe.ReplaceAllString(c, "$1$2")
	c = gitOptRe.ReplaceAllString(c, "git")
	c = orqOptRe.ReplaceAllString(c, "orq")
	c = sliceOptRe.ReplaceAllString(c, "$1")
	return c
}

// destructivoRe: lo que el harness nunca hace, venga de quien venga.
var destructivoRe = []struct {
	re  *regexp.Regexp
	por string
}{
	{regexp.MustCompile(`git\s+push\s+.*(--force|-f\b|--force-with-lease|\+\S+:)`), "push forzado"},
	{regexp.MustCompile(`git\s+push\b`), "push: el harness es local, publicar lo decide el Owner"},
	{regexp.MustCompile(`git\s+branch\s+.*(-D\b|--delete\s+--force|-d\s+-f|-df|-fd)`), "borrar una rama a la fuerza; usa -d o deja que `orq slice cerrar` la mande a cuarentena"},
	{regexp.MustCompile(`git\s+worktree\s+(remove|prune)\s+.*(--force|-f\b)`), "quitar un worktree a la fuerza"},
	{regexp.MustCompile(`git\s+(reset\s+.*--hard|clean\s+.*(-[a-zA-Z]*f|--force)|checkout\s+.*(-f\b|--force))`), "descartar trabajo sin respaldo"},
	{regexp.MustCompile(`\brm\s+.*(worktrees|\.orq|\.git)\b`), "borrar worktrees, .orq o .git con rm"},
}

// pushRamaRe: un solo comando, sin comillas ni encadenados: `git [-C dir] push [-u] remoto rama`.
var pushRamaRe = regexp.MustCompile(`^\s*git(\s+-C\s+([A-Za-z0-9._/~-]+))?\s+push(\s+(-u|--set-upstream))?\s+([A-Za-z0-9._-]+)\s+([A-Za-z0-9._/-]+)\s*$`)

// pushPermitido deja subir una rama solo desde un checkout que el Owner listó en push_ramas.
// Nunca main/master/HEAD, nunca forzado ni borrando, nunca desde un worktree de slice.
func pushPermitido(cmd, cwd string, raices []string) bool {
	m := pushRamaRe.FindStringSubmatch(cmd)
	if m == nil || len(raices) == 0 {
		return false
	}
	switch m[6] {
	case "main", "master", "HEAD":
		return false
	}
	dir := cwd
	if m[2] != "" {
		dir = resolver(cwd, m[2])
	}
	for _, r := range raices {
		if r = resolver("/", r); dir == r || strings.HasPrefix(dir, r+"/") {
			return true
		}
	}
	return false
}

func destructivo(p Payload) (Decision, bool) {
	if p.ToolName != "Bash" {
		return Decision{}, false
	}
	cmd, _ := p.ToolInput["command"].(string)
	for _, d := range destructivoRe {
		if d.re.MatchString(cmd) {
			return deny("orq: bloqueado (%s). Si de verdad hace falta, lo hace el Owner a mano.", d.por), true
		}
	}
	return Decision{}, false
}

// soloOrqRe: transiciones que solo pide el Orquestador desde el checkout principal.
var soloOrqRe = regexp.MustCompile(`(^|[\s;&|(])orq\s+(slice\s+(abrir|cerrar|reintentar|tomar|lanzar|juntar|examen|alcance)|plan\s+(cargar|hecha|debatido)|sesion\s+(abrir|cerrar)|aprender\s+promover|msg\s+send\s+.*--tipo\s+cierre|halt\s+--clear)`)

func soloOrquestador(p Payload, env Env) (Decision, bool) {
	if p.ToolName != "Bash" {
		return Decision{}, false
	}
	cmd, _ := p.ToolInput["command"].(string)
	if !soloOrqRe.MatchString(cmd) {
		return Decision{}, false
	}
	if p.AgentID != "" || env.IsWorktree(p.Cwd) {
		return deny("orq: un worker no abre, toma, reintenta ni cierra slices, ni cambia su examen o alcance. Pide verificación al Orquestador con tu resultado."), true
	}
	return Decision{}, false
}

func stripPrefixes(f []string) []string {
	for len(f) > 0 && (strings.Contains(f[0], "=") || f[0] == "sudo" || f[0] == "env" || f[0] == "command" || f[0] == "nohup" || f[0] == "time") {
		f = f[1:]
	}
	return f
}

func nonFlags(f []string) []string {
	var out []string
	for _, t := range f {
		if !strings.HasPrefix(t, "-") {
			out = append(out, t)
		}
	}
	return out
}

func inPlace(f []string) bool {
	for _, t := range f {
		if t == "--in-place" || (strings.HasPrefix(t, "-") && !strings.HasPrefix(t, "--") && strings.Contains(t, "i")) {
			return true
		}
	}
	return false
}

// matchesProtected compara rutas limpias con barra final, así "tests/aceptacion" también coincide.
func matchesProtected(fp string, patterns []string) bool {
	norm := filepath.ToSlash(filepath.Clean(strings.Trim(fp, `'"`))) + "/"
	for _, pat := range patterns {
		pat = strings.TrimPrefix(filepath.ToSlash(filepath.Clean(pat)), "./")
		if strings.HasPrefix(norm, pat+"/") || strings.Contains(norm, "/"+pat+"/") {
			return true
		}
	}
	return false
}

// IsLinkedWorktree sube desde dir hasta encontrar .git; si es un archivo, es un worktree enlazado.
func IsLinkedWorktree(dir string) bool {
	for d := dir; d != "" && d != "/"; d = filepath.Dir(d) {
		info, err := os.Lstat(filepath.Join(d, ".git"))
		if err == nil {
			return !info.IsDir()
		}
	}
	return false
}
