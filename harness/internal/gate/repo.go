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

// gitEnvRepo are the variables that point git at a work tree or at files of a repository.
// GIT_DIR and GIT_COMMON_DIR must name the clone's own .git exactly.
var gitEnvRepo = conjunto("GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES")

// gitEnvCmd are the variables whose value git runs as a command. A worker sets them only to a
// command that runs nothing else.
var gitEnvCmd = conjunto("GIT_EXTERNAL_DIFF", "GIT_SSH", "GIT_SSH_COMMAND", "GIT_PAGER", "GIT_EDITOR",
	"GIT_SEQUENCE_EDITOR", "GIT_ASKPASS", "GIT_PROXY_COMMAND", "GIT_EXEC_PATH", "GIT_TEMPLATE_DIR")

// gitBuiltin are git's own subcommands (`git --list-cmds=main`, git 2.43) minus those that run a
// command given on the line or serve the repository. Anything else is an alias or a git-* program
// on PATH, whose text the gate does not read.
var gitBuiltin = conjunto("add", "am", "annotate", "apply", "archive", "bisect", "blame", "branch",
	"bugreport", "bundle", "cat-file", "check-attr", "check-ignore", "check-mailmap", "check-ref-format",
	"checkout", "checkout-index", "cherry", "cherry-pick", "clean", "clone", "column", "commit",
	"commit-graph", "commit-tree", "config", "count-objects", "credential", "describe", "diagnose", "diff",
	"diff-files", "diff-index", "diff-tree", "difftool", "fast-export", "fast-import", "fetch", "fetch-pack",
	"fmt-merge-msg", "for-each-ref", "format-patch", "fsck", "gc", "get-tar-commit-id", "grep", "hash-object",
	"help", "index-pack", "init", "interpret-trailers", "log", "ls-files", "ls-remote", "ls-tree", "mailinfo",
	"mailsplit", "maintenance", "merge", "merge-base", "merge-file", "merge-index", "merge-tree", "mergetool",
	"mktag", "mktree", "multi-pack-index", "mv", "name-rev", "notes", "pack-objects", "pack-refs", "patch-id",
	"prune", "prune-packed", "pull", "push", "range-diff", "read-tree", "rebase", "reflog", "remote", "repack",
	"replace", "rerere", "reset", "restore", "rev-list", "rev-parse", "revert", "rm", "shortlog", "show",
	"show-branch", "show-index", "show-ref", "sparse-checkout", "stage", "stash", "status", "stripspace",
	"submodule", "switch", "symbolic-ref", "tag", "unpack-file", "unpack-objects", "update-index",
	"update-ref", "update-server-info", "var", "verify-commit", "verify-pack", "verify-tag", "version",
	"whatchanged", "worktree", "write-tree")

// declaran are the builtins whose VAR=value arguments set (and may export) variables.
var declaran = conjunto("export", "declare", "typeset", "readonly", "local")

func ajeno(que string) (Decision, bool) {
	return deny("orq: a worker runs git only in its own repository (%s). Write literal paths inside your clone.", que), true
}

func ramaAjena(ref string) (Decision, bool) {
	return deny("orq: a worker moves only its own branch and the refs under it (%q is not). Commit on your branch; the orchestrator merges.", ref), true
}

func configAjena(que string) (Decision, bool) {
	return deny("orq: a worker sets no git config but user.name, user.email and color.* (%s can run a command or point git at another repository).", que), true
}

func oculto(que string) (Decision, bool) {
	return deny("orq: %s runs a command the gate does not read. Run that command yourself, as its own line.", que), true
}

// claveLibre is the git config a worker may set: identity and colors run nothing.
func claveLibre(k palabra) bool {
	s := strings.ToLower(k.s)
	return !k.dyn && (s == "user.name" || s == "user.email" || strings.HasPrefix(s, "color."))
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
		case strings.HasPrefix(k, "GIT_CONFIG"):
			return configAjena(k)
		case gitEnvCmd[k] && (a.dyn || !conjunto("", "cat", "true", ":")[v]):
			return configAjena(k)
		case k == "GIT_DIR" || k == "GIT_COMMON_DIR":
			if d, ok := otroGitDir(c, palabra{s: v, dyn: a.dyn}, raiz, k); ok {
				return d, ok
			}
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
	}
	if g := gitEntre(resolver("/", c.dir), resolver("/", raiz)); g != "" {
		return ajeno(g + " lies between " + c.dir + " and the clone root, and can point git at another repository")
	}
	if c.gitOpaco != "" {
		return configAjena(c.gitOpaco)
	}
	for _, x := range c.gitC {
		if k, _, _ := strings.Cut(x.s, "="); !claveLibre(palabra{s: k, dyn: x.dyn}) {
			return configAjena("-c " + x.s)
		}
	}
	for _, r := range c.gitDirs {
		if d, ok := otroGitDir(c, r, raiz, "--git-dir"); ok {
			return d, ok
		}
	}
	for _, r := range c.gitRepos {
		if d, ok := fuera(c, r, raiz, "--work-tree"); ok {
			return d, ok
		}
	}
	a := c.w[1:]
	if len(a) == 0 {
		return Decision{}, false
	}
	sub, r := a[0].s, a[1:]
	if a[0].dyn || !gitBuiltin[sub] {
		return deny("orq: %q is not a git builtin: a worker runs no git alias and no git-* program. Write the git command itself.", sub), true
	}
	t := textos(r)
	switch sub {
	case "config":
		return gitConfig(r)
	case "rebase":
		if hay(t, "-x", "--exec") || conPrefijo(t, "--exec=") || corta(t, 'x') {
			return oculto("git rebase --exec")
		}
	case "submodule":
		if hay(t, "foreach") {
			return oculto("git submodule foreach")
		}
	case "bisect":
		if hay(t, "run") {
			return oculto("git bisect run")
		}
	case "difftool", "mergetool":
		if hay(t, "-x", "--extcmd") || conPrefijo(t, "--extcmd=") {
			return oculto("git " + sub + " --extcmd")
		}
	case "update-ref":
		if hay(t, "--stdin") {
			return ramaAjena("--stdin")
		}
		if p := posicionales(r, conjunto("-m")); len(p) > 0 && !propia(p[0], rama) {
			return ramaAjena(p[0].s)
		}
	case "symbolic-ref":
		p := posicionales(r, conjunto("-m"))
		borra := hay(t, "-d", "--delete") || corta(t, 'd')
		if len(p) > 0 && (borra || len(p) > 1) {
			for _, x := range p {
				if !propia(x, rama) {
					return ramaAjena(x.s)
				}
			}
		}
	case "branch":
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

// gitConfig lets a worker read any git config but write only the keys claveLibre allows. It
// denies whole forms (edit, rename or remove a section) instead of reading what they change.
func gitConfig(r []palabra) (Decision, bool) {
	for _, x := range r {
		if x.dyn {
			return configAjena("git config " + x.s)
		}
	}
	t := textos(r)
	if hay(t, "-e", "--edit", "--rename-section", "--remove-section") {
		return configAjena("git config " + strings.Join(t, " "))
	}
	p := posicionales(r, conjunto("-f", "--file", "--blob", "--type", "--default", "--comment", "--value"))
	var clave palabra
	switch {
	case len(p) > 0 && conjunto("get", "list")[p[0].s]:
		return Decision{}, false
	case len(p) > 0 && conjunto("set", "unset")[p[0].s]:
		if len(p) < 2 {
			return Decision{}, false
		}
		clave = p[1]
	case len(p) > 0 && conjunto("edit", "rename-section", "remove-section")[p[0].s]:
		return configAjena("git config " + p[0].s)
	case hay(t, "--get", "--get-all", "--get-regexp", "--get-urlmatch", "--get-color", "--get-colorbool", "-l", "--list"):
		return Decision{}, false
	case hay(t, "--add", "--replace-all", "--unset", "--unset-all") && len(p) > 0, len(p) >= 2:
		clave = p[0]
	default:
		return Decision{}, false
	}
	if !claveLibre(clave) {
		return configAjena("git config " + clave.s)
	}
	return Decision{}, false
}

// gitEntre returns the first .git (directory, gitfile or symlink) in dir or a directory above it,
// below raiz. Such a .git sends git to whatever repository it names, even the main one.
func gitEntre(dir, raiz string) string {
	for d := dir; d != raiz && dentro(d, raiz); d = filepath.Dir(d) {
		if _, err := os.Lstat(filepath.Join(d, ".git")); err == nil {
			return filepath.Join(d, ".git")
		}
	}
	return ""
}

// otroGitDir denies a --git-dir, GIT_DIR or GIT_COMMON_DIR that is not the clone's own .git: any
// other directory can carry a commondir that points at the main repository.
func otroGitDir(c simple, p palabra, raiz, que string) (Decision, bool) {
	if raiz == "" {
		return ajeno("the repository of this directory cannot be placed")
	}
	if p.dyn || c.dirDyn && !filepath.IsAbs(p.s) {
		return ajeno(que + " is not written literally")
	}
	if destino, propio := resolver(c.dir, p.s), resolver("/", filepath.Join(raiz, ".git")); destino != propio {
		return ajeno(que + " " + destino + " is not " + propio)
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
