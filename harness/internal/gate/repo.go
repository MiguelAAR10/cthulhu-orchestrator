package gate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"orq/internal/orqdir"
)

// A worker runs git only against its own repository (its clone), and moves only the refs under
// its own slice/NN branch. In a 1.0.0 linked worktree the refs were the main repo's, so a worker
// could move main (review 3, finding B); a clone has its own refs, but `git -C <main repo>`,
// --git-dir, --work-tree, GIT_DIR= or a cd would still reach the main repo.

// gitEnvRepo are the variables that point git at a repository or work tree.
var gitEnvRepo = conjunto("GIT_DIR", "GIT_WORK_TREE", "GIT_COMMON_DIR", "GIT_INDEX_FILE", "GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES")

// declaran are the builtins whose VAR=value arguments set (and may export) variables.
var declaran = conjunto("export", "declare", "typeset", "readonly", "local")

func ajeno(que string) (Decision, bool) {
	return deny("orq: a worker runs git only in its own repository (%s). Write literal paths inside your clone.", que), true
}

func ramaAjena(ref string) (Decision, bool) {
	return deny("orq: a worker moves only its own branch and the refs under it (%q is not). Commit on your branch; the orchestrator merges.", ref), true
}

// gitAjeno checks one command of a worker against its repository raiz and branch rama.
func gitAjeno(c simple, raiz, rama string) (Decision, bool) {
	asig := c.asig
	if len(c.w) > 0 && declaran[nombre(c.w[0])] {
		for _, x := range c.w[1:] {
			if esAsignacion(x.s) {
				asig = append(asig, x)
			}
		}
	}
	for _, a := range asig {
		k, v, _ := strings.Cut(a.s, "=")
		k = strings.TrimSuffix(k, "+")
		switch {
		case k == "GIT_CONFIG_PARAMETERS" || k == "GIT_CONFIG_COUNT" || k == "GIT_CONFIG" || k == "GIT_CONFIG_GLOBAL" || k == "GIT_CONFIG_SYSTEM" ||
			strings.HasPrefix(k, "GIT_CONFIG_KEY_") || strings.HasPrefix(k, "GIT_CONFIG_VALUE_"):
			return ajeno(k + " can point git at another work tree")
		case gitEnvRepo[k]:
			if d, ok := fuera(c, palabra{s: v, dyn: a.dyn}, raiz, k); ok {
				return d, ok
			}
		}
	}
	if len(c.w) == 0 || nombre(c.w[0]) != "git" {
		return Decision{}, false
	}
	switch {
	case raiz == "":
		return ajeno("the repository of this directory cannot be placed")
	case c.dirDyn:
		return ajeno("it runs in a directory that is not written literally")
	case !dentro(resolver("/", c.dir), resolver("/", raiz)):
		return ajeno(c.dir + " is outside " + raiz)
	case c.gitOpaco != "":
		return ajeno(c.gitOpaco + " can point git at another work tree")
	}
	for _, r := range c.gitRepos {
		if d, ok := fuera(c, r, raiz, "--git-dir/--work-tree"); ok {
			return d, ok
		}
	}
	a := c.w[1:]
	if len(a) == 0 {
		return Decision{}, false
	}
	sub, r := a[0].s, a[1:]
	switch sub {
	case "config":
		for _, x := range r {
			if k := strings.ToLower(x.s); x.dyn || strings.HasPrefix(k, "core.worktree") || strings.HasPrefix(k, "include") {
				return ajeno("git config " + x.s + " can point git at another work tree")
			}
		}
	case "update-ref":
		if hay(textos(r), "--stdin") {
			return ramaAjena("--stdin")
		}
		if p := posicionales(r, conjunto("-m")); len(p) > 0 && !propia(p[0], rama) {
			return ramaAjena(p[0].s)
		}
	case "symbolic-ref":
		p := posicionales(r, conjunto("-m"))
		borra := hay(textos(r), "-d", "--delete") || corta(textos(r), 'd')
		if len(p) > 0 && (borra || len(p) > 1) {
			for _, x := range p {
				if !propia(x, rama) {
					return ramaAjena(x.s)
				}
			}
		}
	case "branch":
		t := textos(r)
		fuerza := hay(t, "-f", "--force") || corta(t, 'f')
		mueve := hay(t, "-M", "-C") || corta(t, 'M') || corta(t, 'C')
		p := posicionales(r, conjunto("-u", "--set-upstream-to", "-t", "--track", "--contains", "--no-contains", "--merged", "--no-merged", "--points-at", "--sort", "--format", "--color", "--column"))
		if mueve {
			for _, x := range p {
				if !propia(rama0(x), rama) {
					return ramaAjena(x.s)
				}
			}
		} else if fuerza && len(p) > 0 && !propia(rama0(p[0]), rama) {
			return ramaAjena(p[0].s)
		}
	}
	return Decision{}, false
}

// fuera denies a repository path that is not literal or resolves outside raiz.
func fuera(c simple, p palabra, raiz, que string) (Decision, bool) {
	if raiz == "" {
		return ajeno("the repository of this directory cannot be placed")
	}
	if p.dyn || c.dirDyn && !filepath.IsAbs(p.s) {
		return ajeno(que + " is not written literally")
	}
	if destino := resolver(c.dir, p.s); !dentro(destino, resolver("/", raiz)) {
		return ajeno(que + " " + destino + " is outside " + raiz)
	}
	return Decision{}, false
}

// posicionales returns the arguments that are not options; conValor options take the next one.
func posicionales(w []palabra, conValor map[string]bool) []palabra {
	var out []palabra
	for i := 0; i < len(w); i++ {
		switch s := w[i].s; {
		case s == "--":
			return append(out, w[i+1:]...)
		case strings.HasPrefix(s, "-") && !w[i].dyn:
			if conValor[s] {
				i++
			}
		default:
			out = append(out, w[i])
		}
	}
	return out
}

// rama0 turns a branch name into its ref.
func rama0(x palabra) palabra {
	x.s = "refs/heads/" + x.s
	return x
}

// propia says whether ref is the worker's own branch or under it.
func propia(ref palabra, rama string) bool {
	if ref.dyn || rama == "" {
		return false
	}
	own := "refs/heads/" + rama
	return ref.s == own || strings.HasPrefix(ref.s, own+"/")
}

// RepoPropio is the worker's own repository: the nearest directory upward whose .git holds the
// clone marker, with the branch the marker names. Without a marker it is the nearest repository
// (a linked worktree) and no branch: such a worker moves no ref. A relative or empty dir cannot
// be placed.
func RepoPropio(dir string) (raiz, rama string) {
	if !filepath.IsAbs(dir) {
		return "", ""
	}
	cercano := ""
	for d := filepath.Clean(dir); ; d = filepath.Dir(d) {
		if raw, err := os.ReadFile(filepath.Join(d, ".git", orqdir.MarcaClon)); err == nil {
			var m struct {
				Rama string `json:"rama"`
			}
			if json.Unmarshal(raw, &m) != nil {
				m.Rama = ""
			}
			return d, m.Rama
		}
		if _, err := os.Lstat(filepath.Join(d, ".git")); err == nil && cercano == "" {
			cercano = d
		}
		if filepath.Dir(d) == d {
			return cercano, ""
		}
	}
}
