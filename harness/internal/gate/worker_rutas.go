package gate

import (
	"os"
	"path"
	"path/filepath"
	"strings"

	"orq/internal/lang"
)

func negadoConfig() Decision {
	return deny("%s", lang.T("orq: a worker does not read or write orq's config directory or another process's environment.",
		"orq: un worker no lee ni escribe la carpeta de config de orq ni el entorno de otro proceso."))
}

// dirsConfig: la carpeta de config de orq (XDG y ~/.config), tal como se escribe y ya resuelta.
func dirsConfig() []string {
	var out []string
	add := func(base string, err error) {
		if err == nil && base != "" {
			d := filepath.Join(base, "orq")
			out = append(out, d, rutaReal(d, 0))
		}
	}
	add(os.UserConfigDir())
	if h, err := os.UserHomeDir(); err == nil && h != "" {
		add(filepath.Join(h, ".config"), nil)
	}
	return out
}

func dentroDe(r, dir string) bool { return r == dir || strings.HasPrefix(r, dir+"/") }

func tocaConfig(real string) bool {
	for _, d := range dirsConfig() {
		if dentroDe(real, d) {
			return true
		}
	}
	return false
}

// rutaVedada aplica a una ruta limpia las reglas de config y de /proc.
func rutaVedada(r string) bool {
	if strings.Contains(r, "/.config") || strings.Contains(r, "owner.token") || tocaConfig(r) {
		return true
	}
	if cfg, _ := os.UserConfigDir(); cfg != "" && dentroDe(r, cfg) {
		return true
	}
	return r == "/proc" || (strings.HasPrefix(r, "/proc/") && (strings.Contains(r, "environ") || strings.ContainsAny(r, "*?[")))
}

// limpia expande ~ y une contra base sin seguir enlaces: lo que el worker escribió.
func limpia(base, r string) string {
	if r == "~" || strings.HasPrefix(r, "~/") {
		if h, err := os.UserHomeDir(); err == nil {
			r = filepath.Join(h, r[1:])
		}
	}
	if !filepath.IsAbs(r) {
		r = filepath.Join(base, r)
	}
	return filepath.Clean(r)
}

// vedadaEscritaOReal: la regla vale para la ruta escrita y para la real (EvalSymlinks).
func vedadaEscritaOReal(base, r string) bool {
	return rutaVedada(limpia(base, r)) || rutaVedada(resolver(base, r))
}

// esAmplio: / o el home, o un ancestro de la config: una búsqueda recursiva desde ahí la alcanza.
func esAmplio(real string) bool {
	if h, err := os.UserHomeDir(); real == "/" || (err == nil && real == rutaReal(filepath.Clean(h), 0)) {
		return true
	}
	for _, d := range dirsConfig() {
		if strings.HasPrefix(d, strings.TrimSuffix(real, "/")+"/") {
			return true
		}
	}
	return false
}

// workerLee: las herramientas de archivos de un worker no tocan la config de orq ni el entorno de
// otros procesos, comparando siempre la ruta real: un enlace dentro del worktree no abre la puerta.
func workerLee(p Payload) (Decision, bool) {
	switch p.ToolName {
	case "Read", "Grep", "Glob", "LS", "NotebookRead", "Write", "Edit", "MultiEdit", "NotebookEdit":
	default:
		return Decision{}, false
	}
	base := p.Cwd
	for _, k := range []string{"file_path", "path", "notebook_path"} {
		v, _ := p.ToolInput[k].(string)
		if v == "" {
			continue
		}
		if vedadaEscritaOReal(p.Cwd, v) {
			return negadoConfig(), true
		}
		if k == "path" {
			base = limpia(p.Cwd, v)
			if p.ToolName == "Grep" && esAmplio(resolver(p.Cwd, v)) {
				return negadoConfig(), true
			}
		}
	}
	real := rutaReal(filepath.Clean(base), 0)
	if p.ToolName == "Glob" {
		if v, _ := p.ToolInput["pattern"].(string); v != "" {
			if vedadaEscritaOReal(base, v) || patronDeToken(sinWorktree(p.Cwd, v), esAmplio(real)) || alcanzaConfig(real, v) {
				return negadoToken(), true
			}
		}
		if v, _ := p.ToolInput["path"].(string); v != "" && patronDeToken(sinWorktree(p.Cwd, limpia(p.Cwd, v)), false) {
			return negadoToken(), true
		}
	}
	if p.ToolName == "Grep" {
		if v, _ := p.ToolInput["glob"].(string); v != "" && patronDeToken(v, esAmplio(real)) {
			return negadoToken(), true
		}
	}
	return Decision{}, false
}

func negadoToken() Decision {
	return deny("%s", lang.T("orq: a worker does not search for tokens or owner.* by pattern, nor in / or the home folder.",
		"orq: un worker no busca tokens ni owner.* por patrón, ni en / ni en el home."))
}

// sinWorktree quita el prefijo del cwd de un patrón o ruta absolutos: un repo llamado stock o
// tokyo no cuenta como búsqueda de tokens. Desde / o el home no se quita nada.
func sinWorktree(cwd, r string) string {
	if cwd == "" || !filepath.IsAbs(r) {
		return r
	}
	for _, c := range []string{filepath.Clean(cwd), rutaReal(filepath.Clean(cwd), 0)} {
		if h, _ := os.UserHomeDir(); c == "/" || c == filepath.Clean(h) {
			continue
		}
		if dentroDe(r, c) {
			return strings.TrimPrefix(strings.TrimPrefix(r, c), "/")
		}
	}
	return r
}

// llaves expande {a,b} como lo hace Glob.
func llaves(s string) []string {
	i := strings.IndexByte(s, '{')
	if i < 0 {
		return []string{s}
	}
	j := strings.IndexByte(s[i:], '}')
	if j < 0 {
		return []string{s}
	}
	var out []string
	for _, alt := range strings.Split(s[i+1:i+j], ",") {
		out = append(out, llaves(s[:i]+alt+s[i+j+1:])...)
	}
	return out
}

// literales cuenta letras y dígitos fuera de corchetes: `*` no es una búsqueda de owner.token, `own*` sí.
func literales(seg string) int {
	n, dentro := 0, false
	for _, r := range seg {
		switch {
		case r == '[':
			dentro = true
		case r == ']':
			dentro = false
		case !dentro && (r >= 'a' && r <= 'z' || r >= '0' && r <= '9'):
			n++
		}
	}
	return n
}

// patronDeToken: el patrón o la ruta nombra *tok* u owner.*, o algún segmento con comodines casa con
// owner.token (own?r.*, *t?k*). Desde / o el home (amplio) basta una letra (o*).
func patronDeToken(s string, amplio bool) bool {
	min := 2
	if amplio {
		min = 1
	}
	for _, alt := range llaves(strings.ToLower(s)) {
		for _, seg := range strings.Split(filepath.ToSlash(alt), "/") {
			if strings.Contains(seg, "tok") || strings.HasPrefix(seg, "owner.") {
				return true
			}
			if strings.ContainsAny(seg, "*?[") && literales(seg) >= min {
				if ok, err := path.Match(seg, "owner.token"); ok && err == nil {
					return true
				}
			}
		}
	}
	return false
}

// alcanzaConfig: el patrón, segmento a segmento desde base, llega a la config (.c*/o*/*).
func alcanzaConfig(base, pat string) bool {
	if filepath.IsAbs(pat) {
		return false // lo cubre la comparación de ruta
	}
	ps := strings.Split(filepath.ToSlash(pat), "/")
	for _, d := range dirsConfig() {
		if !strings.HasPrefix(d, strings.TrimSuffix(base, "/")+"/") {
			continue
		}
		ds := strings.Split(strings.TrimPrefix(d, strings.TrimSuffix(base, "/")+"/"), "/")
		ok, lit := true, 0
		for i := 0; i < len(ps) && i < len(ds) && ok; i++ {
			m, err := path.Match(ps[i], ds[i])
			ok = m && err == nil
			lit += literales(ps[i])
		}
		if ok && lit > 0 && ps[0] != "**" {
			return true
		}
	}
	return false
}

// buscadorDeToken: find/rg/fd/locate con un patrón de token, sea cual sea la raíz.
func buscadorDeToken(c string) bool {
	for _, seg := range segRe.Split(c, -1) {
		f := stripPrefixes(strings.Fields(seg))
		for _, t := range f {
			if strings.ContainsAny(t, "*?[") && patronDeToken(t, false) {
				return true // ls /x/owner.*, cat own?r.*
			}
		}
		if len(f) < 2 {
			continue
		}
		switch path.Base(f[0]) {
		case "fd", "fdfind", "locate", "mlocate", "plocate":
			for _, t := range f[1:] {
				if !strings.HasPrefix(t, "-") && patronDeToken(t, false) {
					return true
				}
			}
		case "find", "rg":
			for i, t := range f[1:] {
				if i+2 < len(f) && patronDeToken(f[i+2], false) {
					switch t {
					case "-name", "-iname", "-path", "-ipath", "-wholename", "-iwholename", "-regex", "-iregex", "-lname", "-ilname", "-g", "--glob", "--iglob":
						return true
					}
				}
			}
		}
	}
	return false
}

// workerBashRutas: nombrar por Bash un enlace hacia la config es lo mismo que nombrar la config.
// Sigue los `cd` del comando y expande los globs contra el disco. No ve rutas armadas en ejecución.
func workerBashRutas(cmd, cwd string) (Decision, bool) {
	c := strings.NewReplacer(`'`, "", `"`, "", `\`, "").Replace(cmd)
	if buscadorDeToken(c) {
		return negadoToken(), true
	}
	dir := cwd
	for _, seg := range segRe.Split(c, -1) {
		f := stripPrefixes(strings.Fields(seg))
		for _, t := range f {
			if strings.HasPrefix(t, "-") || strings.ContainsAny(t, "$`") {
				continue
			}
			if tocaConfig(resolver(dir, t)) {
				return negadoConfig(), true
			}
			if strings.ContainsAny(t, "*?[") {
				m, _ := filepath.Glob(limpia(dir, t))
				for _, x := range m {
					if tocaConfig(rutaReal(x, 0)) {
						return negadoConfig(), true
					}
				}
			}
		}
		if len(f) > 1 && (f[0] == "cd" || f[0] == "pushd") && !strings.ContainsAny(f[1], "$`") {
			dir = resolver(dir, f[1])
		}
	}
	return Decision{}, false
}
