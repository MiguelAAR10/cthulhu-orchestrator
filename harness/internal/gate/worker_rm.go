package gate

import (
	"os"
	"path"
	"path/filepath"
	"strings"

	"orq/internal/lang"
)

// raizWorktree: el directorio con .git más cercano al cwd (archivo en un worktree enlazado).
// Sin .git, o si cae en / o el home, la raíz es el cwd.
func raizWorktree(cwd string) string {
	real := rutaReal(filepath.Clean(cwd), 0)
	h, _ := os.UserHomeDir()
	for d := real; d != "/" && d != "."; d = filepath.Dir(d) {
		if _, err := os.Lstat(filepath.Join(d, ".git")); err == nil {
			if h != "" && d == rutaReal(filepath.Clean(h), 0) {
				return real
			}
			return d
		}
		if filepath.Dir(d) == d {
			break
		}
	}
	return real
}

// objetivo traduce un argumento a ruta real. ok=false si no se puede saber antes de correr.
func objetivo(dir, t string) (string, bool) {
	for _, v := range []string{"$HOME", "${HOME}"} {
		if t == v || strings.HasPrefix(t, v+"/") {
			t = "~" + t[len(v):]
		}
	}
	for _, v := range []string{"$PWD", "${PWD}"} {
		if t == v || strings.HasPrefix(t, v+"/") {
			t = dir + t[len(v):]
		}
	}
	if strings.ContainsAny(t, "$`{}") || (strings.HasPrefix(t, "~") && t != "~" && !strings.HasPrefix(t, "~/")) {
		return "", false
	}
	return resolver(dir, t), true
}

// rmRecursivo: sus objetivos, si es recursivo. Admite -rf, -fr, -R, -r -f, --recursive y sus abreviaturas.
func rmRecursivo(args []string) (targets []string, recursivo bool) {
	soloArgs := false
	for _, a := range args {
		switch {
		case soloArgs || !strings.HasPrefix(a, "-") || a == "-":
			targets = append(targets, a)
		case a == "--":
			soloArgs = true
		case strings.HasPrefix(a, "--"):
			recursivo = recursivo || (len(a) >= 3 && strings.HasPrefix("--recursive", a))
		default:
			recursivo = recursivo || strings.ContainsAny(a, "rR")
		}
	}
	return targets, recursivo
}

// rmFuera: algún rm recursivo del comando apunta a /, al home o fuera del worktree, o a algo que no
// se puede verificar (variable, sustitución, llaves, stdin de xargs). Falla cerrado.
func rmFuera(cmd, cwd string) bool {
	c := strings.NewReplacer(`'`, "", `"`, "", `\`, "").Replace(cmd)
	if cwd == "" {
		cwd = "/"
	}
	raiz := raizWorktree(cwd)
	dentro := func(t string) bool { return t != raiz && strings.HasPrefix(t, raiz+"/") }
	dir, dirOK := cwd, true
	for _, seg := range segRe.Split(c, -1) {
		f := strings.Fields(seg)
		for i, tok := range f {
			if path.Base(tok) != "rm" {
				continue
			}
			targets, rec := rmRecursivo(f[i+1:])
			if !rec {
				continue
			}
			if !dirOK || (i > 0 && contiene(f[:i], "xargs")) {
				return true
			}
			for _, t := range targets {
				if t == "{}" { // find -exec: valen las raíces del find
					if !raicesFindDentro(f[:i], dir, dentro) {
						return true
					}
					continue
				}
				if r, ok := objetivo(dir, t); !ok || !dentro(r) {
					return true
				}
			}
			break
		}
		if g := stripPrefixes(f); len(g) > 0 && (g[0] == "cd" || g[0] == "pushd") {
			dest := "~"
			if a := nonFlags(g[1:]); len(a) > 0 {
				dest = a[0]
			}
			r, ok := objetivo(dir, dest)
			dir, dirOK = r, dirOK && ok && dest != "-"
		}
	}
	return false
}

func contiene(xs []string, q string) bool {
	for _, x := range xs {
		if path.Base(x) == q {
			return true
		}
	}
	return false
}

// raicesFindDentro: `find a b -exec rm -r {}`: a y b deben estar dentro; sin raíz, find usa el cwd.
// La raíz misma cuenta: find incluye el punto de partida, pero `find .` borra lo que su filtro diga.
func raicesFindDentro(antes []string, dir string, dentro func(string) bool) bool {
	for i, t := range antes {
		if path.Base(t) != "find" {
			continue
		}
		for _, a := range antes[i+1:] {
			if strings.HasPrefix(a, "-") || a == "!" {
				break
			}
			r, ok := objetivo(dir, a)
			if !ok || !(dentro(r) || r == raizWorktree(dir)) {
				return false
			}
		}
		return true
	}
	return false
}

func negadoRm() Decision {
	return deny("%s", lang.T("orq: a worker only deletes recursively inside its own worktree, with explicit paths (no /, ~, $HOME, .. escapes, variables or xargs).",
		"orq: un worker solo borra en recursivo dentro de su worktree, con rutas explícitas (sin /, ~, $HOME, salidas con .., variables ni xargs)."))
}
