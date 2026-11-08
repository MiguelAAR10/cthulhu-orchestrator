package gate

import (
	"os"
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
	if p.ToolName == "Glob" {
		if v, _ := p.ToolInput["pattern"].(string); v != "" && vedadaEscritaOReal(base, v) {
			return negadoConfig(), true
		}
	}
	return Decision{}, false
}

// workerBashRutas: nombrar por Bash un enlace hacia la config es lo mismo que nombrar la config.
// Sigue los `cd` del comando y expande los globs contra el disco. No ve rutas armadas en ejecución.
func workerBashRutas(cmd, cwd string) (Decision, bool) {
	c := strings.NewReplacer(`'`, "", `"`, "", `\`, "").Replace(cmd)
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
