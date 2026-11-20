package gate

// Reglas sobre comandos ya analizados. Cada regla mira la palabra de comando y sus
// opciones; el texto de los argumentos (un patrón de grep, un mensaje de commit) no cuenta.

import (
	"orq/internal/orqdir"
	"os"
	"path/filepath"
	"strings"

	"orq/internal/limits"
)

func textos(w []palabra) []string {
	out := make([]string, len(w))
	for i, x := range w {
		out[i] = x.s
	}
	return out
}

func hay(a []string, xs ...string) bool {
	for _, s := range a {
		for _, x := range xs {
			if s == x {
				return true
			}
		}
	}
	return false
}

// corta dice si alguna opción corta agrupada (-rf) lleva la letra.
func corta(a []string, letra byte) bool {
	for _, s := range a {
		if len(s) > 1 && s[0] == '-' && s[1] != '-' && strings.IndexByte(s, letra) > 0 {
			return true
		}
	}
	return false
}

// sueltos devuelve los argumentos que no son opciones; tras "--" todo es argumento.
func sueltos(w []palabra) []palabra {
	var out []palabra
	for i, x := range w {
		if x.s == "--" {
			return append(out, w[i+1:]...)
		}
		if !strings.HasPrefix(x.s, "-") || x.s == "-" {
			out = append(out, x)
		}
	}
	return out
}

func marcar(w []palabra, bajo bool) []palabra {
	out := append([]palabra{}, w...)
	for i := range out {
		out[i].bajo = out[i].bajo || bajo
	}
	return out
}

func valorDe(w []palabra, corto, largo string) (palabra, bool) {
	for i, x := range w {
		switch {
		case (x.s == corto || x.s == largo) && i+1 < len(w):
			return w[i+1], true
		case strings.HasPrefix(x.s, largo+"="):
			x.s = strings.TrimPrefix(x.s, largo+"=")
			return x, true
		case strings.HasPrefix(x.s, corto) && len(x.s) > len(corto):
			x.s = x.s[len(corto):]
			return x, true
		case len(corto) == 2 && len(x.s) > 2 && x.s[0] == '-' && x.s[1] != '-' && strings.IndexByte(x.s, corto[1]) > 0:
			// grouped short options (-qB main, -qBmain): the value follows the letter
			if rest := x.s[strings.IndexByte(x.s, corto[1])+1:]; rest != "" {
				x.s = rest
				return x, true
			}
			if i+1 < len(w) {
				return w[i+1], true
			}
		}
	}
	return palabra{}, false
}

func dentro(hijo, padre string) bool {
	return hijo == padre || strings.HasPrefix(hijo, strings.TrimSuffix(padre, "/")+"/")
}

// ---------- destructivos: nunca, venga de quien venga ----------

func destructivo(c simple, cwd string) (Decision, bool) {
	if len(c.w) == 0 {
		return Decision{}, false
	}
	no := func(por string) (Decision, bool) {
		return deny("orq: bloqueado (%s). Si de verdad hace falta, lo hace el Owner a mano.", por), true
	}
	a := textos(c.w[1:])
	switch nombre(c.w[0]) {
	case "git":
		if len(a) == 0 {
			break
		}
		sub, r := a[0], a[1:]
		switch {
		case sub == "push" && (hay(r, "--force", "-f") || corta(r, 'f') || conPrefijo(r, "--force-with-lease", "+")):
			return no("push forzado")
		case sub == "push":
			return no("push: el harness es local, publicar lo decide el Owner")
		case sub == "branch" && (hay(r, "-D") || corta(r, 'D') || (hay(r, "--delete") || corta(r, 'd')) && (hay(r, "--force") || corta(r, 'f'))):
			return no("borrar una rama a la fuerza; usa -d o deja que `orq slice cerrar` la mande a cuarentena")
		case sub == "worktree" && len(r) > 0 && (r[0] == "remove" || r[0] == "prune") && (hay(r, "--force") || corta(r, 'f')):
			return no("quitar un worktree a la fuerza")
		case sub == "reset" && hay(r, "--hard"), sub == "clean" && (hay(r, "--force") || corta(r, 'f')),
			sub == "checkout" && (hay(r, "--force") || corta(r, 'f')):
			return no("descartar trabajo sin respaldo")
		}
	case "rm":
		for _, x := range sueltos(c.w[1:]) {
			for _, comp := range strings.Split(filepath.ToSlash(x.s), "/") {
				if comp == ".git" || comp == ".orq" || comp == "worktrees" {
					return no("borrar worktrees, .orq o .git con rm")
				}
			}
			if !x.bajo && !x.dyn && !c.dirDyn && dentro(cwd, resolver(c.dir, x.s)) {
				return no("borrar el directorio de trabajo o uno que lo contiene")
			}
		}
	}
	return Decision{}, false
}

func conPrefijo(a []string, ps ...string) bool {
	for _, s := range a {
		for _, p := range ps {
			if strings.HasPrefix(s, p) {
				return true
			}
		}
	}
	return false
}

// ---------- transiciones que solo pide el orquestador ----------

// sliceOrquestador are the `orq slice` subcommands only the orchestrator runs.
var sliceOrquestador = conjunto("abrir", "cerrar", "reintentar", "tomar", "lanzar", "juntar",
	"examen", "alcance", "rehuella", "reabrir", "ruta", "verificar", "podar", "tarjeta", "aprobar")

// SliceSoloOrquestador says whether `orq slice sub` belongs to the orchestrator. The CLI refuses
// these inside a worker too, so a copied or renamed binary, an alias or a script cannot run them.
func SliceSoloOrquestador(sub string) bool { return sliceOrquestador[sub] }

// orqPelado says whether c runs the command named orq (any path, any subcommand): under orq2 that
// is orq 1.0.0. The form orq2 writes in its own hooks, ORQ_NOMBRE=orq2 <path>/orq, is orq2 itself.
func orqPelado(c simple) bool {
	if len(c.w) == 0 || filepath.Base(c.w[0].s) != "orq" {
		return false
	}
	for _, a := range c.asig {
		if a.s == "ORQ_NOMBRE=orq2" {
			return false
		}
	}
	return true
}

func soloOrquestador(c simple) bool {
	if len(c.w) == 0 || nombre(c.w[0]) != "orq" {
		return false
	}
	var a []string
	for i := 1; i < len(c.w); i++ {
		switch s := c.w[i].s; {
		case s == "--slice" || s == "--repo":
			i++
		case strings.HasPrefix(s, "--slice=") || strings.HasPrefix(s, "--repo="):
		default:
			a = append(a, s)
		}
	}
	if len(a) > 0 && hay(a[:1], "pausar", "reanudar") {
		return true
	}
	if len(a) < 2 {
		return false
	}
	switch a[0] {
	case "slice":
		return SliceSoloOrquestador(a[1])
	case "plan":
		return hay(a[1:2], "cargar", "hecha", "debatido")
	case "sesion":
		return hay(a[1:2], "abrir", "cerrar")
	case "aprender":
		return a[1] == "promover"
	case "msg":
		for i := 2; i < len(a); i++ {
			if a[1] == "send" && (a[i] == "--tipo=cierre" || a[i] == "--tipo" && i+1 < len(a) && a[i+1] == "cierre") {
				return true
			}
		}
	case "halt":
		return hay(a[1:], "--clear")
	case "historia":
		return hay(a[1:2], "abrir", "pausar", "reanudar")
	}
	return false
}

// ---------- lanzamientos: crean un worker, un panel o una copia de trabajo ----------

func esLanzamiento(c simple) bool {
	if len(c.w) == 0 {
		return false
	}
	a := textos(sueltos(c.w[1:]))
	sec := func(x string, ys ...string) bool { return len(a) > 1 && a[0] == x && hay(a[1:2], ys...) }
	switch nombre(c.w[0]) {
	case "codex":
		return true
	case "claude":
		return hay(textos(c.w[1:]), "-p", "--print")
	case "herdr":
		return sec("agent", "start", "prompt") || sec("pane", "run", "split") || sec("worktree", "create")
	case "orca":
		return sec("agent", "launch") || sec("orchestration", "worker-start", "dispatch", "coordinator-start") ||
			sec("terminal", "create", "split") || sec("worktree", "create")
	}
	return false
}

// ---------- escrituras a lo protegido ----------

// proteccion: lo que un worker no escribe.
type proteccion struct {
	rel []string // relativas a la raíz del worktree: examen y paquete del slice
	abs []string // rutas absolutas del harness
	cwd string
	wt  bool   // a worker: it creates no .git but its clone's own
	git string // the clone's own .git, resolved ("" if the clone cannot be placed)
}

func proteccionDe(p Payload, l limits.Limits, env Env, wt bool) proteccion {
	pr := proteccion{cwd: resolver("/", p.Cwd), wt: wt}
	if wt {
		if env.RepoPropio != nil {
			if raiz, _ := env.RepoPropio(p.Cwd); raiz != "" {
				pr.git = resolver("/", filepath.Join(raiz, ".git"))
			}
		}
		// The clone marker makes a clone a worker: moving or rewriting it would switch the gate off.
		pr.rel = append(pr.rel, ".git/"+orqdir.MarcaClon)
		// .git/config (core.worktree, include.path) and .git/commondir point git at another repository.
		pr.rel = append(pr.rel, ".git/config", ".git/commondir")
		pr.rel = append(pr.rel, l.Protected...)
		if env.Bundle != nil {
			pr.rel = append(pr.rel, env.Bundle(p.Cwd)...)
		}
	}
	harness := []string{env.StateDir, env.LimitsPath}
	if wt {
		// The global and system git config run commands (core.fsmonitor, core.pager) in every repository.
		harness = append(harness, "/etc/gitconfig")
		if h, err := os.UserHomeDir(); err == nil {
			harness = append(harness, filepath.Join(h, ".gitconfig"), filepath.Join(h, ".config/git/config"), filepath.Join(h, ".git-credentials"))
		}
		if x := os.Getenv("XDG_CONFIG_HOME"); filepath.IsAbs(x) {
			harness = append(harness, filepath.Join(x, "git/config"))
		}
	}
	if env.MainRoot != nil {
		if r := env.MainRoot(p.Cwd); r != "" {
			harness = append(harness, filepath.Join(r, ".orq"))
		}
	}
	for _, h := range harness {
		if h != "" {
			pr.abs = append(pr.abs, resolver("/", h))
		}
	}
	return pr
}

// cubre dice si escribir en ruta (ya resuelta) toca algo protegido.
// Con bajo, la escritura alcanza todo el árbol de ruta.
func (pr proteccion) cubre(ruta string, bajo bool) bool {
	if matchesProtected(ruta, pr.rel) {
		return true
	}
	if b := filepath.Base(ruta); (b == "settings.json" || b == "settings.local.json") && filepath.Base(filepath.Dir(ruta)) == ".claude" {
		return true
	}
	for _, a := range pr.abs {
		if dentro(ruta, a) || bajo && dentro(a, ruta) {
			return true
		}
	}
	if !bajo {
		return false
	}
	if filepath.Base(ruta) == ".claude" || len(pr.rel) > 0 && dentro(pr.cwd, ruta) {
		return true // la raíz del worktree está en cwd o encima: ruta la contiene
	}
	for _, pat := range pr.rel {
		parts := strings.Split(strings.Trim(filepath.ToSlash(filepath.Clean(pat)), "/"), "/")
		for k := 1; k < len(parts); k++ {
			if strings.HasSuffix(ruta, "/"+strings.Join(parts[:k], "/")) {
				return true
			}
		}
	}
	return false
}

// puntoGit says whether a worker's write reaches a .git other than its clone's own: creating,
// writing, moving or linking one (a gitfile, a symlink, a directory with a commondir) points git
// at another repository, even the main one. rutas are the literal and the resolved path.
func (pr proteccion) puntoGit(rutas ...string) bool {
	if !pr.wt {
		return false
	}
	for _, r := range rutas {
		for d := filepath.Clean(r); filepath.Dir(d) != d; d = filepath.Dir(d) {
			if filepath.Base(d) == ".git" && d != pr.git {
				return true
			}
		}
	}
	return false
}

func gitNuevo(ruta string) Decision {
	return deny("orq: a worker creates, writes, moves or links no .git but its clone's own (%q): any other .git can point git at another repository, even the main one.", ruta)
}

// literal joins a relative path to dir without resolving links.
func literal(dir, p string) string {
	if p = strings.Trim(p, `'"`); filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	return filepath.Join(dir, p)
}

// nombra dice si un texto (código en línea) menciona algo protegido.
func (pr proteccion) nombra(code string) bool {
	if strings.Contains(code, ".claude/settings") || strings.Contains(code, ".orq/") {
		return true
	}
	for _, p := range append(append([]string{}, pr.rel...), pr.abs...) {
		if p = strings.Trim(p, "/"); p != "" && strings.Contains(code, p) {
			return true
		}
	}
	return false
}

func protegido(ruta string) Decision {
	return deny("orq: %q está protegido para los workers (examen, estado o configuración del harness). Solo el orquestador, desde el checkout principal, lo edita.", ruta)
}

func escrituraProhibida(p Payload, cmds []simple, pr proteccion, wt bool) (Decision, bool) {
	switch {
	case p.ToolName == "Edit" || p.ToolName == "Write" || p.ToolName == "MultiEdit" || p.ToolName == "NotebookEdit":
		for _, k := range []string{"file_path", "notebook_path"} {
			fp, _ := p.ToolInput[k].(string)
			if fp != "" && pr.puntoGit(literal(p.Cwd, fp), resolver(p.Cwd, fp)) {
				return gitNuevo(fp), true
			}
			if fp != "" && pr.cubre(resolver(p.Cwd, fp), false) {
				return protegido(fp), true
			}
		}
	case p.ToolName == "Bash":
		for _, c := range cmds {
			if d, ok := escrituraDeShell(c, pr, wt); ok {
				return d, true
			}
		}
	case strings.HasPrefix(p.ToolName, "mcp__") && wt:
		var hallada string
		recorrer(p.ToolInput, func(s string) bool {
			if strings.TrimSpace(s) != "" && (pr.puntoGit(literal(p.Cwd, s), resolver(p.Cwd, s)) || pr.cubre(resolver(p.Cwd, s), false)) {
				hallada = s
			}
			return hallada != ""
		})
		if hallada != "" {
			return protegido(hallada), true
		}
	}
	return Decision{}, false
}

// recorrer visita cada cadena de un valor JSON hasta que f devuelva true.
func recorrer(v any, f func(string) bool) bool {
	switch x := v.(type) {
	case string:
		return f(x)
	case []any:
		for _, e := range x {
			if recorrer(e, f) {
				return true
			}
		}
	case map[string]any:
		for _, e := range x {
			if recorrer(e, f) {
				return true
			}
		}
	}
	return false
}

func escrituraDeShell(c simple, pr proteccion, wt bool) (Decision, bool) {
	if wt && len(c.w) > 0 && (c.w[0].dyn || len(c.w) > 1 && c.w[1].dyn && conjunto("git", "orq", "herdr", "orca")[nombre(c.w[0])]) {
		return deny("orq: no sé qué comando es %q: lleva una expansión ($var, ${...}, $(...) o `...`) o llega por stdin. Escribe el comando literal.", c.w[0].s), true
	}
	if wt && len(pr.rel) > 0 && opaco(c) {
		return deny("orq: %q escribe archivos que no se saben de antemano y el examen del slice está protegido. Haz el cambio con Edit o Write.", strings.Join(textos(c.w), " ")), true
	}
	if pr.wt && len(c.w) > 0 && conjunto("cp", "rsync", "install")[nombre(c.w[0])] {
		for _, x := range sueltos(c.w[1:]) {
			if filepath.Base(filepath.Clean(x.s)) == ".git" {
				return gitNuevo(x.s), true // copying a .git into a directory creates one there
			}
		}
	}
	for _, o := range objetivos(c) {
		if o.dyn || c.dirDyn && !filepath.IsAbs(o.s) {
			if wt {
				return deny("orq: no sé adónde escribe %q: lleva una expansión ($var, ${...}, $(...) o `...`) o llega por stdin. Escribe la ruta literal.", o.s), true
			}
			continue
		}
		if pr.puntoGit(literal(c.dir, o.s), resolver(c.dir, o.s)) {
			return gitNuevo(o.s), true
		}
		rutas := []string{resolver(c.dir, o.s)}
		if o.glob {
			if ms, _ := filepath.Glob(rutas[0]); len(ms) > 0 {
				rutas = ms
			}
		}
		for _, r := range rutas {
			if pr.cubre(resolver("/", r), o.bajo) {
				return protegido(o.s), true
			}
		}
	}
	if code := codigoEnLinea(c); code != "" && pr.nombra(code) {
		return deny("orq: el código en línea de %q nombra una ruta protegida. Solo el orquestador la edita.", c.w[0].s), true
	}
	return Decision{}, false
}

// opaco: aplica cambios cuyo destino no se ve en el comando.
func opaco(c simple) bool {
	if len(c.w) == 0 {
		return false
	}
	a := textos(c.w[1:])
	switch nombre(c.w[0]) {
	case "patch":
		return true
	case "git":
		return len(a) > 0 && (a[0] == "apply" || a[0] == "am" || a[0] == "stash" && len(a) > 1 && (a[1] == "pop" || a[1] == "apply"))
	}
	return false
}

// objetivos devuelve las rutas que un comando puede escribir, mover o borrar.
func objetivos(c simple) []palabra {
	var ts []palabra
	for _, r := range c.redirs {
		switch r.op {
		case ">", ">>", ">|", "&>", "&>>", "<>":
			ts = append(ts, r.a)
		case ">&":
			if r.a.s != "-" && strings.Trim(r.a.s, "0123456789") != "" {
				ts = append(ts, r.a)
			}
		}
	}
	if len(c.w) == 0 {
		return ts
	}
	args := c.w[1:]
	a, sl := textos(args), sueltos(args)
	recursivo := hay(a, "--recursive") || corta(a, 'r') || corta(a, 'R')
	switch n := nombre(c.w[0]); n {
	case "rm", "rmdir", "shred", "unlink":
		ts = append(ts, marcar(sl, recursivo || n == "rmdir")...)
	case "mv": // lo que se mueve se lleva su árbol; el destino recibe
		if len(sl) > 0 {
			ts = append(append(ts, marcar(sl[:len(sl)-1], true)...), sl[len(sl)-1])
		}
	case "tee", "truncate", "touch", "ln", "mkdir":
		ts = append(ts, sl...)
	case "chmod", "chown", "chgrp":
		ts = append(ts, marcar(sl, recursivo)...)
	case "cp", "install", "rsync":
		if t, ok := valorDe(args, "-t", "--target-directory"); ok {
			ts = append(ts, t)
		} else if len(sl) > 0 {
			ts = append(ts, sl[len(sl)-1])
		}
	case "dd":
		for _, x := range args {
			if v, ok := strings.CutPrefix(x.s, "of="); ok {
				x.s = v
				ts = append(ts, x)
			}
		}
	case "sed", "perl":
		if hay(a, "--in-place") || corta(a, 'i') {
			ts = append(ts, sl...)
		}
	case "git":
		if len(a) > 0 && hay(a[:1], "checkout", "restore", "rm", "mv", "reset", "clean") {
			ts = append(ts, marcar(sueltos(args[1:]), true)...)
		}
	}
	return ts
}

func esInterprete(n string) bool {
	return strings.HasPrefix(n, "python") || conjunto("node", "nodejs", "perl", "ruby", "php", "deno", "bun", "pypy", "pypy3")[n]
}

// codigoEnLinea junta el código que un intérprete recibe en la línea o por heredoc.
func codigoEnLinea(c simple) string {
	if len(c.w) == 0 || !esInterprete(nombre(c.w[0])) {
		return ""
	}
	code := []string{c.stdin}
	args := c.w[1:]
	for i, x := range args {
		s := x.s
		switch {
		case strings.HasPrefix(s, "--eval=") || strings.HasPrefix(s, "--print="):
			code = append(code, s)
		case s == "--eval" || s == "--print" || len(s) >= 2 && len(s) <= 4 && s[0] == '-' && s[1] != '-' && strings.IndexByte("cepr", s[len(s)-1]) >= 0:
			code = append(code, s) // -c, -e, -p, -r, o un grupo corto que termina en ellos (perl -pie)
			if i+1 < len(args) {
				code = append(code, args[i+1].s)
			}
		case strings.HasPrefix(s, "-e") || strings.HasPrefix(s, "-c"):
			code = append(code, s) // código pegado: -e'...'
		}
	}
	return strings.Join(code, "\n")
}

// CheckoutPrincipal devuelve la raíz del checkout principal del repo de dir,
// siguiendo el archivo .git de un worktree enlazado. "" si no hay repo.
func CheckoutPrincipal(dir string) string {
	if !filepath.IsAbs(dir) {
		return "" // cannot be placed; IsWorker already treats it as a worker
	}
	for d := filepath.Clean(dir); d != "/" && filepath.Dir(d) != d; d = filepath.Dir(d) {
		g := filepath.Join(d, ".git")
		info, err := os.Lstat(g)
		if err != nil {
			continue
		}
		if info.IsDir() {
			return d
		}
		raw, err := os.ReadFile(g)
		gd, ok := strings.CutPrefix(strings.TrimSpace(string(raw)), "gitdir:")
		if err != nil || !ok {
			return ""
		}
		if gd = strings.TrimSpace(gd); !filepath.IsAbs(gd) {
			gd = filepath.Join(d, gd)
		}
		common := filepath.Dir(filepath.Dir(gd)) // <repo>/.git/worktrees/<nombre>
		if c, err := os.ReadFile(filepath.Join(gd, "commondir")); err == nil {
			if common = strings.TrimSpace(string(c)); !filepath.IsAbs(common) {
				common = filepath.Join(gd, common)
			}
		}
		return filepath.Dir(filepath.Clean(common))
	}
	return ""
}
