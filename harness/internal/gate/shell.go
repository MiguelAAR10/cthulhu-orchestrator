package gate

// Analizador de shell: parte un comando en comandos simples, con las palabras ya sin
// comillas como las vería bash, para que las reglas miren la palabra de comando y
// nunca el texto de los argumentos. Abre los envoltorios que corren otro comando
// (sh -c, eval, xargs, find -exec, env, sudo, nohup, time, command, npx...).

import (
	"os"
	"path/filepath"
	"strings"
)

// palabra es una palabra del shell ya sin comillas.
type palabra struct {
	s    string
	dyn  bool // tiene $var, ${...}, $(...), `...` o llega por stdin: su valor no se conoce
	glob bool // tiene *, ? o [ sin comillas
	bajo bool // la ruta cuenta con todo su árbol (rm -r, mv, find)
}

type redir struct {
	op string
	a  palabra
}

// simple es un comando simple ya desenvuelto.
type simple struct {
	w      []palabra
	redirs []redir
	stdin  string    // heredoc o here-string: lo que el comando lee como entrada
	piped  bool      // recibe la salida de otro comando por |
	dir    string    // directorio donde corre, tras los cd anteriores
	dirDyn bool      // hubo un cd a una ruta no resuelta: lo relativo no se conoce
	asig   []palabra // VAR=valor words before the command (also after env)
	// git only: repositories named by --git-dir, --work-tree or -c core.worktree, and why the
	// target repository cannot be known (-c include.*, --config-env).
	gitRepos []palabra
	gitOpaco string
}

const meta = " \t\n;&|<>()"

type tok struct {
	op   string // "" palabra, "redir", o un operador: ; && || | |& & \n ( )
	rop  string
	w    palabra
	subs []string // cuerpos de $(...), `...` y <(...) que corren como comandos
	here int      // índice del cuerpo del heredoc, o -1
}

type pendiente struct {
	delim        string
	strip, crudo bool
	idx          int
}

type lexer struct {
	s      string
	i      int
	toks   []tok
	bodies []string
	subsH  [][]string // sustituciones dentro de heredocs sin comillas
	pend   []pendiente
}

func lex(s string) *lexer {
	lx := &lexer{s: s}
	for lx.i < len(s) {
		c := s[lx.i]
		switch {
		case c == ' ' || c == '\t':
			lx.i++
		case c == '\\' && lx.i+1 < len(s) && s[lx.i+1] == '\n':
			lx.i += 2
		case c == '#':
			for lx.i < len(s) && s[lx.i] != '\n' {
				lx.i++
			}
		case c == '\n':
			lx.toks = append(lx.toks, tok{op: "\n", here: -1})
			lx.i++
			lx.heredocs()
		case (c == '<' || c == '>') && lx.i+1 < len(s) && s[lx.i+1] == '(':
			end := cierre(s, lx.i+2, '(', ')')
			lx.toks = append(lx.toks, tok{w: palabra{s: s[lx.i:end], dyn: true}, subs: []string{cuerpo(s, lx.i+2, end)}, here: -1})
			lx.i = end
		case c == '<' || c == '>' || (c == '&' && lx.i+1 < len(s) && s[lx.i+1] == '>') || esFDRedir(s, lx.i):
			lx.redir()
		case strings.IndexByte(";&|()", c) >= 0:
			for _, op := range []string{"&&", "||", "|&", ";;", ";", "&", "|", "(", ")"} {
				if strings.HasPrefix(s[lx.i:], op) {
					lx.toks = append(lx.toks, tok{op: op, here: -1})
					lx.i += len(op)
					break
				}
			}
		default:
			w, subs := lx.palabra()
			lx.toks = append(lx.toks, tok{w: w, subs: subs, here: -1})
		}
	}
	lx.heredocs()
	return lx
}

// esFDRedir: dígitos pegados a < o >, como en 2>&1.
func esFDRedir(s string, i int) bool {
	j := i
	for j < len(s) && s[j] >= '0' && s[j] <= '9' {
		j++
	}
	return j > i && j < len(s) && (s[j] == '<' || s[j] == '>')
}

func (lx *lexer) redir() {
	s := lx.s
	for lx.i < len(s) && s[lx.i] >= '0' && s[lx.i] <= '9' {
		lx.i++
	}
	op := ">"
	for _, o := range []string{"&>>", "&>", "<<<", "<<-", "<<", "<>", "<&", ">>", ">|", ">&", "<", ">"} {
		if strings.HasPrefix(s[lx.i:], o) {
			op = o
			break
		}
	}
	lx.i += len(op)
	for lx.i < len(s) && (s[lx.i] == ' ' || s[lx.i] == '\t') {
		lx.i++
	}
	if lx.i >= len(s) || strings.IndexByte(meta, s[lx.i]) >= 0 {
		return
	}
	ini := lx.i
	w, subs := lx.palabra()
	t := tok{op: "redir", rop: op, w: w, subs: subs, here: -1}
	if op == "<<" || op == "<<-" {
		t.here = len(lx.bodies)
		lx.bodies = append(lx.bodies, "")
		lx.subsH = append(lx.subsH, nil)
		crudo := strings.ContainsAny(s[ini:lx.i], `'"\`)
		lx.pend = append(lx.pend, pendiente{delim: w.s, strip: op == "<<-", crudo: crudo, idx: t.here})
	}
	lx.toks = append(lx.toks, t)
}

// heredocs lee, tras un salto de línea, los cuerpos de los heredocs pendientes.
func (lx *lexer) heredocs() {
	for _, p := range lx.pend {
		var b strings.Builder
		for lx.i < len(lx.s) {
			line, next := lx.s[lx.i:], len(lx.s)
			if j := strings.IndexByte(line, '\n'); j >= 0 {
				line, next = line[:j], lx.i+j+1
			}
			lx.i = next
			if p.strip {
				line = strings.TrimLeft(line, "\t")
			}
			if line == p.delim {
				break
			}
			b.WriteString(line + "\n")
		}
		lx.bodies[p.idx] = b.String()
		if !p.crudo { // sin comillas, bash expande $(...) dentro del heredoc
			lx.subsH[p.idx] = sustituciones(b.String())
		}
	}
	lx.pend = nil
}

func sustituciones(s string) []string {
	lx := &lexer{s: s}
	var out []string
	var b strings.Builder
	for lx.i < len(s) {
		switch s[lx.i] {
		case '\\':
			lx.i += 2
		case '$', '`':
			_, subs := lx.expansion(&b, false)
			out = append(out, subs...)
		default:
			lx.i++
		}
	}
	return out
}

func (lx *lexer) palabra() (palabra, []string) {
	var b strings.Builder
	var w palabra
	var subs []string
	s := lx.s
	if s[lx.i] == '~' && (lx.i+1 == len(s) || s[lx.i+1] == '/' || strings.IndexByte(meta, s[lx.i+1]) >= 0) {
		if h, err := os.UserHomeDir(); err == nil {
			b.WriteString(h)
			lx.i++
		}
	}
	llave := false
	for lx.i < len(s) && strings.IndexByte(meta, s[lx.i]) < 0 {
		switch c := s[lx.i]; c {
		case '\\':
			if lx.i+1 < len(s) && s[lx.i+1] != '\n' {
				b.WriteByte(s[lx.i+1])
			}
			lx.i += 2
		case '\'':
			end := strings.IndexByte(s[lx.i+1:], '\'')
			if end < 0 {
				end = len(s) - lx.i - 1
			}
			b.WriteString(s[lx.i+1 : lx.i+1+end])
			lx.i += end + 2
		case '"':
			lx.i++
			for lx.i < len(s) && s[lx.i] != '"' {
				switch s[lx.i] {
				case '\\':
					if lx.i+1 < len(s) && strings.IndexByte("$`\"\\\n", s[lx.i+1]) >= 0 {
						if s[lx.i+1] != '\n' {
							b.WriteByte(s[lx.i+1])
						}
						lx.i += 2
					} else {
						b.WriteByte('\\')
						lx.i++
					}
				case '$', '`':
					dyn, sb := lx.expansion(&b, true)
					w.dyn = w.dyn || dyn
					subs = append(subs, sb...)
				default:
					b.WriteByte(s[lx.i])
					lx.i++
				}
			}
			lx.i++
		case '$', '`':
			dyn, sb := lx.expansion(&b, false)
			w.dyn = w.dyn || dyn
			subs = append(subs, sb...)
		case '*', '?', '[':
			w.glob = true
			b.WriteByte(c)
			lx.i++
		case '{':
			llave = true
			b.WriteByte(c)
			lx.i++
		default:
			b.WriteByte(c)
			lx.i++
		}
	}
	lx.i = min(lx.i, len(s))
	w.s = b.String()
	if llave && strings.Contains(w.s, "}") && (strings.Contains(w.s, ",") || strings.Contains(w.s, "..")) {
		w.dyn = true // expansión de llaves: a{b,c}
	}
	return w, subs
}

// expansion consume una expansión que empieza en $ o `. Devuelve si su valor es
// desconocido y los cuerpos que se ejecutan como comandos.
func (lx *lexer) expansion(b *strings.Builder, enComillas bool) (bool, []string) {
	s, i := lx.s, lx.i
	if s[i] == '`' {
		j := i + 1
		for j < len(s) && s[j] != '`' {
			if s[j] == '\\' {
				j++
			}
			j++
		}
		j = min(j, len(s))
		lx.i = min(j+1, len(s))
		b.WriteString(s[i:lx.i])
		return true, []string{s[i+1 : j]}
	}
	if i+1 >= len(s) {
		b.WriteByte('$')
		lx.i++
		return false, nil
	}
	n := s[i+1]
	switch {
	case n == '(':
		end := cierre(s, i+2, '(', ')')
		b.WriteString(s[i:end])
		lx.i = end
		if strings.HasPrefix(s[i:], "$((") {
			return true, nil
		}
		return true, []string{cuerpo(s, i+2, end)}
	case n == '{':
		lx.i = cierre(s, i+2, '{', '}')
	case n == '\'' && !enComillas:
		j := i + 2
		for j < len(s) && s[j] != '\'' {
			if s[j] == '\\' {
				j++
			}
			j++
		}
		lx.i = min(j+1, len(s))
	case n == '_' || n >= 'a' && n <= 'z' || n >= 'A' && n <= 'Z':
		j := i + 1
		for j < len(s) && (s[j] == '_' || s[j] >= 'a' && s[j] <= 'z' || s[j] >= 'A' && s[j] <= 'Z' || s[j] >= '0' && s[j] <= '9') {
			j++
		}
		lx.i = j
	case n >= '0' && n <= '9' || strings.IndexByte("@*#?$!-", n) >= 0:
		lx.i = i + 2
	default: // un $ suelto es literal; $"..." se lee como "..."
		if n != '"' || enComillas {
			b.WriteByte('$')
		}
		lx.i++
		return false, nil
	}
	b.WriteString(s[i:lx.i])
	return true, nil
}

// cierre devuelve el índice tras el cierre que empareja la apertura ya consumida.
func cierre(s string, i int, open, close byte) int {
	depth := 1
	for ; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++
		case '\'':
			j := strings.IndexByte(s[i+1:], '\'')
			if j < 0 {
				return len(s)
			}
			i += j + 1
		case '"':
			j := i + 1
			for j < len(s) && s[j] != '"' {
				if s[j] == '\\' {
					j++
				}
				j++
			}
			i = j
		case open:
			depth++
		case close:
			if depth--; depth == 0 {
				return i + 1
			}
		}
	}
	return len(s)
}

func cuerpo(s string, ini, end int) string {
	if end > ini && end <= len(s) && s[end-1] == ')' {
		end--
	}
	return s[ini:max(ini, end)]
}

// ---------- de tokens a comandos simples ----------

const maxProfundidad = 8

type analizador struct{ out []simple }

// analizar devuelve los comandos simples de un script, con envoltorios abiertos
// y sustituciones incluidas, en el orden en que corren.
func analizar(script, dir string) []simple {
	a := &analizador{}
	a.script(script, dir, false, 0)
	return a.out
}

// desconocido es un comando cuya palabra de comando no se puede saber.
func desconocido(s, dir string) simple {
	return simple{w: []palabra{{s: s, dyn: true}}, dir: dir}
}

func (a *analizador) script(s, dir string, dirDyn bool, depth int) {
	if depth > maxProfundidad {
		a.out = append(a.out, desconocido(s, dir))
		return
	}
	lx := lex(s)
	type lugar struct {
		dir string
		dyn bool
	}
	var pila []lugar
	cur := simple{dir: dir, dirDyn: dirDyn}
	var subs []string
	piped := false
	cerrar := func() {
		for _, sb := range subs {
			a.script(sb, dir, dirDyn, depth+1)
		}
		if len(cur.w) > 0 || len(cur.redirs) > 0 {
			cur.piped = piped
			dir, dirDyn = a.comando(cur, depth)
		}
		cur, subs = simple{dir: dir, dirDyn: dirDyn}, nil
	}
	for _, t := range lx.toks {
		switch t.op {
		case "":
			cur.w = append(cur.w, t.w)
			subs = append(subs, t.subs...)
		case "redir":
			subs = append(subs, t.subs...)
			switch t.rop {
			case "<<", "<<-":
				cur.stdin += lx.bodies[t.here]
				subs = append(subs, lx.subsH[t.here]...)
			case "<<<":
				cur.stdin += t.w.s + "\n"
			}
			cur.redirs = append(cur.redirs, redir{op: t.rop, a: t.w})
		case "(":
			cerrar()
			pila = append(pila, lugar{dir, dirDyn})
		case ")":
			cerrar()
			if n := len(pila); n > 0 {
				dir, dirDyn, pila = pila[n-1].dir, pila[n-1].dyn, pila[:n-1]
				cur.dir, cur.dirDyn = dir, dirDyn
			}
		default:
			cerrar()
			piped = t.op == "|" || t.op == "|&"
		}
	}
	cerrar()
}

var (
	sudoValor  = conjunto("-u", "-g", "-C", "-D", "-h", "-p", "-U", "-r", "-t", "-T", "--user", "--group", "--chdir", "--host", "--prompt", "--role", "--type", "--other-user")
	niceValor  = conjunto("-n", "--adjustment")
	tiempoVal  = conjunto("-s", "-k", "--signal", "--kill-after")
	npxValor   = conjunto("-p", "--package", "--cache", "--registry")
	xargsValor = conjunto("-n", "-P", "-L", "-d", "-E", "-s", "-a", "--max-args", "--max-procs", "--max-lines", "--delimiter", "--eof", "--max-chars", "--arg-file", "--process-slot-var")
	gitValor   = conjunto("-C", "-c", "--git-dir", "--work-tree", "--namespace", "--config-env", "--super-prefix")
	shellValor = conjunto("-o", "+o", "-O", "+O", "--rcfile", "--init-file")
	transparen = conjunto("{", "}", "!", "if", "then", "else", "elif", "fi", "do", "done", "while", "until", "esac", "time", "nohup", "exec", "builtin")
	shells     = conjunto("sh", "bash", "zsh", "dash", "ksh")
)

func conjunto(xs ...string) map[string]bool {
	m := map[string]bool{}
	for _, x := range xs {
		m[x] = true
	}
	return m
}

// nombre devuelve el comando sin ruta ni versión (/usr/bin/git → git, @x/claude-code@1 → claude).
func nombre(w palabra) string {
	if w.dyn {
		return ""
	}
	s := w.s
	if i := strings.LastIndexByte(s, '@'); i > 0 {
		s = s[:i]
	}
	switch s = filepath.Base(s); s {
	case "claude-code":
		return "claude"
	case "orca-ide":
		return "orca"
	}
	return s
}

func esAsignacion(s string) bool {
	i := strings.IndexByte(s, '=')
	if i <= 0 {
		return false
	}
	for k, c := range strings.TrimSuffix(s[:i], "+") {
		if !(c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || k > 0 && c >= '0' && c <= '9') {
			return false
		}
	}
	return true
}

// saltarOpciones quita las opciones del principio; las de conValor se llevan el siguiente argumento.
func saltarOpciones(w []palabra, conValor map[string]bool) []palabra {
	for len(w) > 0 && strings.HasPrefix(w[0].s, "-") {
		if w[0].s == "--" {
			return w[1:]
		}
		if conValor[w[0].s] && len(w) > 1 {
			w = w[1:]
		}
		w = w[1:]
	}
	return w
}

func palabrasDe(s string) []palabra {
	var out []palabra
	for _, t := range lex(s).toks {
		if t.op == "" {
			out = append(out, t.w)
		}
	}
	return out
}

func cambiarDir(dir string, dyn bool, a palabra) (string, bool) {
	if a.dyn || a.s == "-" {
		return dir, true
	}
	if filepath.IsAbs(a.s) {
		return resolver("/", a.s), false
	}
	return resolver(dir, a.s), dyn
}

// comando desenvuelve c, lo agrega a la salida y devuelve el directorio tras él (por cd).
func (a *analizador) comando(c simple, depth int) (string, bool) {
	dir, dyn := c.dir, c.dirDyn
	w := c.w
bucle:
	for {
		for len(w) > 0 && esAsignacion(w[0].s) {
			c.asig = append(c.asig, w[0])
			w = w[1:]
		}
		if len(w) == 0 {
			break
		}
		n := nombre(w[0])
		switch {
		case transparen[n]:
			w = saltarOpciones(w[1:], nil)
		case n == "command":
			if len(w) > 1 && (w[1].s == "-v" || w[1].s == "-V") {
				w = nil
				break bucle
			}
			w = saltarOpciones(w[1:], nil)
		case n == "sudo" || n == "doas":
			w = saltarOpciones(w[1:], sudoValor)
		case n == "nice":
			w = saltarOpciones(w[1:], niceValor)
		case n == "timeout":
			if w = saltarOpciones(w[1:], tiempoVal); len(w) > 0 {
				w = w[1:] // la duración
			}
		case n == "npx" || n == "bunx":
			w = saltarOpciones(w[1:], npxValor)
		case (n == "npm" || n == "pnpm" || n == "yarn") && len(w) > 1 && conjunto("exec", "dlx", "x")[w[1].s]:
			w = saltarOpciones(w[2:], npxValor)
		case n == "env":
			w = w[1:]
		env:
			for len(w) > 0 && strings.HasPrefix(w[0].s, "-") {
				o := w[0].s
				w = w[1:]
				switch {
				case o == "--":
					break env
				case (o == "-u" || o == "--unset" || o == "-C" || o == "--chdir") && len(w) > 0:
					if o == "-C" || o == "--chdir" {
						c.dir, c.dirDyn = cambiarDir(c.dir, c.dirDyn, w[0])
					}
					w = w[1:]
				case strings.HasPrefix(o, "-S") || o == "--split-string":
					val := strings.TrimPrefix(o, "-S")
					if (val == "" || o == "--split-string") && len(w) > 0 {
						val, w = w[0].s, w[1:]
					}
					w = append(palabrasDe(val), w...)
				}
			}
		case n == "xargs":
			w = a.xargs(w[1:])
		case n == "eval":
			var partes []string
			for _, x := range w[1:] {
				if x.dyn {
					a.out = append(a.out, desconocido(x.s, c.dir))
				}
				partes = append(partes, x.s)
			}
			a.script(strings.Join(partes, " "), c.dir, c.dirDyn, depth+1)
			break bucle
		case shells[n]:
			a.shell(w[1:], c, depth)
			break bucle
		case n == "find":
			a.find(w[1:], c, depth)
			break bucle
		case n == "git":
			i := 1
			for i < len(w) && strings.HasPrefix(w[i].s, "-") {
				o, val := w[i].s, palabra{}
				if k := strings.IndexByte(o, '='); k > 0 && strings.HasPrefix(o, "--") {
					o, val = o[:k], palabra{s: o[k+1:], dyn: w[i].dyn}
				} else if gitValor[o] && i+1 < len(w) {
					val = w[i+1]
					i++
				}
				switch o {
				case "-C":
					c.dir, c.dirDyn = cambiarDir(c.dir, c.dirDyn, val)
				case "--git-dir", "--work-tree":
					c.gitRepos = append(c.gitRepos, val)
				case "-c":
					k, v, _ := strings.Cut(val.s, "=")
					switch k = strings.ToLower(k); {
					case val.dyn:
						c.gitOpaco = "-c " + val.s
					case k == "core.worktree":
						c.gitRepos = append(c.gitRepos, palabra{s: v})
					case strings.HasPrefix(k, "include"):
						c.gitOpaco = "-c " + val.s
					}
				case "--config-env":
					c.gitOpaco = "--config-env"
				}
				i++
			}
			w = append([]palabra{w[0]}, w[i:]...)
			break bucle
		case n == "cd" || n == "pushd":
			if args := saltarOpciones(w[1:], nil); len(args) == 0 {
				h, _ := os.UserHomeDir()
				dir, dyn = h, false
			} else {
				dir, dyn = cambiarDir(dir, dyn, args[0])
			}
			break bucle
		case n == "popd":
			dyn = true
			break bucle
		default:
			break bucle
		}
	}
	c.w = w
	a.out = append(a.out, c)
	return dir, dyn
}

// xargs: los argumentos llegan por stdin, así que su valor no se conoce.
func (a *analizador) xargs(w []palabra) []palabra {
	repl := ""
	for len(w) > 0 && strings.HasPrefix(w[0].s, "-") {
		o := w[0].s
		w = w[1:]
		switch {
		case o == "--":
			goto fin
		case o == "-I" || o == "--replace":
			if len(w) > 0 {
				repl, w = w[0].s, w[1:]
			}
		case strings.HasPrefix(o, "-I"):
			repl = o[2:]
		case o == "-i":
			repl = "{}"
		case strings.HasPrefix(o, "--replace="):
			repl = strings.TrimPrefix(o, "--replace=")
		case xargsValor[o] && len(w) > 0:
			w = w[1:]
		}
	}
fin:
	if len(w) == 0 {
		w = []palabra{{s: "echo"}}
	}
	w = append([]palabra{}, w...)
	if repl == "" {
		return append(w, palabra{s: "<stdin>", dyn: true})
	}
	for i := range w {
		if strings.Contains(w[i].s, repl) {
			w[i].dyn = true
		}
	}
	return w
}

// shell abre sh -c '...', y también un shell que lee comandos de un heredoc o de una tubería.
func (a *analizador) shell(w []palabra, c simple, depth int) {
	conC := false
	for len(w) > 0 && (strings.HasPrefix(w[0].s, "-") || strings.HasPrefix(w[0].s, "+")) {
		o := w[0].s
		w = w[1:]
		if o == "--" {
			break
		}
		if shellValor[o] && len(w) > 0 {
			w = w[1:]
			continue
		}
		if !strings.HasPrefix(o, "--") && strings.Contains(o, "c") {
			conC = true
		}
	}
	switch {
	case conC && len(w) > 0 && w[0].dyn:
		a.out = append(a.out, desconocido(w[0].s, c.dir))
	case conC && len(w) > 0:
		a.script(w[0].s, c.dir, c.dirDyn, depth+1)
	case len(w) > 0: // corre un archivo de script: lo cubre el paso Verificar
	case c.stdin != "":
		a.script(c.stdin, c.dir, c.dirDyn, depth+1)
	case c.piped:
		a.out = append(a.out, desconocido("<stdin>", c.dir))
	}
}

// find: -delete y -exec actúan sobre lo que haya bajo las raíces, o sobre lo que nombre -name.
func (a *analizador) find(args []palabra, c simple, depth int) {
	for len(args) > 0 && conjunto("-H", "-L", "-P")[args[0].s] {
		args = args[1:]
	}
	var raices []palabra
	for len(args) > 0 && !strings.HasPrefix(args[0].s, "-") && args[0].s != "(" && args[0].s != "!" {
		raices, args = append(raices, args[0]), args[1:]
	}
	if len(raices) == 0 {
		raices = []palabra{{s: "."}}
	}
	var nombres []string
	for i := 0; i+1 < len(args); i++ {
		if conjunto("-name", "-iname", "-path", "-ipath", "-wholename", "-regex", "-iregex")[args[i].s] {
			nombres = append(nombres, args[i+1].s)
		}
	}
	var cand []palabra
	for _, r := range raices {
		r.bajo = true
		cand = append(cand, r)
		for _, n := range nombres {
			cand = append(cand, palabra{s: filepath.Join(r.s, n), dyn: r.dyn, bajo: true})
		}
	}
	derivado := func(w []palabra) simple { return simple{w: w, dir: c.dir, dirDyn: c.dirDyn} }
	for i := 0; i < len(args); i++ {
		switch args[i].s {
		case "-delete":
			a.out = append(a.out, derivado(append([]palabra{{s: "rm"}, {s: "-r"}}, cand...)))
		case "-fprint", "-fprint0", "-fls", "-fprintf":
			if i+1 < len(args) {
				a.out = append(a.out, derivado([]palabra{{s: "touch"}, args[i+1]}))
			}
		case "-exec", "-execdir", "-ok", "-okdir":
			j := i + 1
			for j < len(args) && args[j].s != ";" && args[j].s != "+" {
				j++
			}
			for _, k := range cand {
				ws := append([]palabra{}, args[i+1:j]...)
				for x := range ws {
					if strings.Contains(ws[x].s, "{}") {
						ws[x] = palabra{s: strings.ReplaceAll(ws[x].s, "{}", k.s), dyn: k.dyn, bajo: true}
					}
				}
				a.comando(derivado(ws), depth+1)
			}
			i = j
		}
	}
}
