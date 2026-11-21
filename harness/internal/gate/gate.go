// Package gate decide si una herramienta puede ejecutarse.
// Falla cerrado: ante duda, dato ausente o estado viejo, deniega el lanzamiento.
// Gobierna lanzamientos de workers, comandos destructivos y escrituras a archivos
// protegidos; el resto de herramientas pasan sin tocar. Los comandos de shell se
// analizan como los vería bash (shell.go) y las reglas viven en reglas.go.
package gate

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"orq/internal/identidad"
	"orq/internal/limits"
	"orq/internal/orqdir"
	"orq/internal/state"
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
	// LimitsPath es el limits.json en uso: un worker no lo edita.
	LimitsPath string
	// MainRoot devuelve el checkout principal del repo de cwd, cuyo .orq/ es del orquestador.
	MainRoot func(cwd string) string
	// RepoPropio returns the root of the worker's own repository (its clone) and its branch.
	// "" root: the repository cannot be placed and a worker runs no git.
	RepoPropio func(cwd string) (raiz, rama string)
	// Capacidad dice si la máquina (RAM, CPU, carga) admite un agente más con vivos corriendo.
	// Devuelve "" si cabe, o el motivo. nil = no se mira la máquina (pruebas del resto de reglas).
	Capacidad func(cwd string, vivos int) (motivo string, err error)
	// AvisoRAM returns a warning line for an allowed launch, or "". nil = no warning (orq).
	AvisoRAM func() string
}

type Decision struct {
	Allow  bool
	Reason string
	// Aviso is a warning that rides on an allowed launch; it never turns it into a refusal.
	Aviso string
}

func allow() Decision                  { return Decision{Allow: true} }
func deny(f string, a ...any) Decision { return Decision{Reason: fmt.Sprintf(f, a...)} }

func Evaluate(p Payload, l limits.Limits, env Env) Decision {
	wt := env.IsWorktree(p.Cwd)
	var cmds []simple
	if p.ToolName == "Bash" {
		cmd, _ := p.ToolInput["command"].(string)
		// `orq2 update` replaces the binary that is everyone's gate hook: it is the Owner's alone.
		if (p.AgentID != "" || wt || os.Getenv("ORQ_ROL") != "") && nombraUpdate(cmd) {
			return deny("orq2: `orq2 update` (también --ver, --volver y --registrar) es del Owner, en su terminal; un agente, un worker o un panel de rol no lo corre. Pídeselo al Owner.")
		}
		if pushPermitido(cmd, p.Cwd, l.PushRamas) {
			return allow()
		}
		cmds = analizar(cmd, p.Cwd)
		for _, c := range cmds {
			if d, ok := destructivo(c, resolver("/", p.Cwd)); ok {
				return d
			}
		}
		if wt {
			raiz, rama := "", ""
			if env.RepoPropio != nil {
				raiz, rama = env.RepoPropio(p.Cwd)
			}
			for _, c := range cmds {
				if d, ok := gitAjeno(c, raiz, rama); ok {
					return d
				}
			}
		}
		if identidad.EsOrq2() {
			for _, c := range cmds {
				if orqPelado(c) {
					return deny("orq2: el comando \"orq\" es orq 1.0.0, con otra configuración y otro estado. Usa `orq2 <subcomando>`.")
				}
			}
		}
		for _, c := range cmds {
			if soloOrquestador(c) && (p.AgentID != "" || wt) {
				return deny("orq: un worker no abre, toma, reintenta ni cierra slices. Pide verificación al Orquestador con tu resultado.")
			}
		}
	}
	if wt || p.AgentID != "" {
		if d, ok := escrituraProhibida(p, cmds, proteccionDe(p, l, env, wt), wt); ok {
			return d
		}
	}
	if !isLaunch(p, cmds) {
		return allow()
	}
	if wt {
		return deny("orq: un worker no lanza workers ni subagentes. Devuelve el resultado al orquestador.")
	}
	return evaluateLaunch(p, l, env)
}

func isLaunch(p Payload, cmds []simple) bool {
	switch p.ToolName {
	case "Agent", "Task": // Task es el nombre anterior de la herramienta Agent
		return true
	}
	for _, c := range cmds {
		if esLanzamiento(c) {
			return true
		}
	}
	return false
}

func evaluateLaunch(p Payload, l limits.Limits, env Env) Decision {
	if p.AgentID != "" {
		return deny("orq: un subagente no lanza workers. Devuelve el resultado al orquestador.")
	}
	if reason, halted := state.ReadHalt(env.StateDir); halted {
		return deny("orq: HALT activo (%s). Libera espacio y luego `orq halt --clear`.", reason)
	}
	for path, min := range l.MinFreeGB {
		free, err := env.DiskFreeGB(path)
		if err != nil {
			return deny("orq: no pude medir el disco en %s (%v). Sin dato no hay lanzamiento.", path, err)
		}
		if free < min {
			return deny("orq: %s tiene %.0f GB libres y el mínimo es %.0f GB. Libera espacio antes de lanzar.", path, free, min)
		}
	}
	snap, err := state.LoadSnapshot(env.StateDir)
	if err != nil {
		return deny("orq: falta la medición de scratch. Ejecuta `orq watch` y vuelve a intentar.")
	}
	age := env.Now().Sub(snap.TS)
	if age > time.Duration(*l.StateMaxAgeHours*float64(time.Hour)) {
		return deny("orq: la medición de scratch tiene %.0f h. Ejecuta `orq watch` y vuelve a intentar.", age.Hours())
	}
	if snap.ScratchGB > *l.Scratch.QuotaGB {
		return deny("orq: scratch ocupa %.0f GB y la cuota es %.0f GB. Limpia snapshots antes de lanzar.", snap.ScratchGB, *l.Scratch.QuotaGB)
	}
	n, err := env.Workers()
	if err != nil {
		return deny("orq: no pude contar workers (%v). Sin dato no hay lanzamiento.", err)
	}
	if l.MaxWorkers == nil && env.Capacidad == nil {
		return deny("orq: limits.json no trae max_workers y no hay medición de la máquina. Sin dato no hay lanzamiento.")
	}
	if l.MaxWorkers != nil && n >= *l.MaxWorkers {
		return deny("orq: hay %d worker(s) vivos y el tope es %d. Cierra paneles antes de lanzar otro.", n, *l.MaxWorkers)
	}
	if env.Capacidad != nil {
		motivo, err := env.Capacidad(p.Cwd, n)
		if err != nil {
			return deny("orq: no pude medir la máquina (%v). Sin dato no hay lanzamiento.", err)
		}
		if motivo != "" {
			return deny("orq: %s.", motivo)
		}
	}
	d := allow()
	if env.AvisoRAM != nil {
		d.Aviso = env.AvisoRAM()
	}
	return d
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

// IsWorker reports whether dir belongs to a worker: a linked worktree (orq 1.0.0) or a clone
// that orq marked. It looks at every .git upward, not only the nearest: a repo nested inside a
// worker clone (`git init sub`, `git clone . sub`) is still a worker. The marker counts by
// existing: an empty, corrupt or unreadable marker is still a worker (fail closed), and so is a
// directory that cannot be placed (empty or relative).
func IsWorker(dir string) bool {
	if !filepath.IsAbs(dir) {
		return true
	}
	for d := filepath.Clean(dir); ; d = filepath.Dir(d) {
		if info, err := os.Lstat(filepath.Join(d, ".git")); err == nil {
			if !info.IsDir() {
				return true
			}
			if _, err := os.Lstat(filepath.Join(d, ".git", orqdir.MarcaClon)); !os.IsNotExist(err) {
				return true
			}
		}
		if filepath.Dir(d) == d {
			return false
		}
	}
}

// EsWorkerPropio is orq2's counting question, narrower than IsWorker: true only if some .git
// directory upward holds the marker of a clone orq made. A linked worktree (another tool's agent)
// does not count, and neither does a directory that cannot be placed. It only sets the head count
// for max_workers; IsWorker stays the gate's security question.
func EsWorkerPropio(dir string) bool {
	if !filepath.IsAbs(dir) {
		return false
	}
	for d := filepath.Clean(dir); ; d = filepath.Dir(d) {
		if info, err := os.Lstat(filepath.Join(d, ".git")); err == nil && info.IsDir() {
			if _, err := os.Lstat(filepath.Join(d, ".git", orqdir.MarcaClon)); !os.IsNotExist(err) {
				return true
			}
		}
		if filepath.Dir(d) == d {
			return false
		}
	}
}

// orq2Update is a command line that names orq2 (any path, any copy such as orq2-24f2438) and the
// word update. It is matched on the raw text, not the parsed words, so wrapping it in `bash -c`,
// `script -c`, a variable prefix or a cd does not hide it; an agent context fails closed on it.
var orq2Update = regexp.MustCompile(`orq2[^\s'"]*[\s'"].*\bupdate\b|\bupdate\b.*orq2`)

// nombraUpdate says whether cmd may run orq2 update.
func nombraUpdate(cmd string) bool { return orq2Update.MatchString(cmd) }
