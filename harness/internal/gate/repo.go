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
	"GIT_SEQUENCE_EDITOR", "GIT_ASKPASS", "GIT_PROXY_COMMAND", "GIT_EXEC_PATH", "GIT_TEMPLATE_DIR", "PAGER", "EDITOR", "VISUAL")

// gitPermitido is all the git a worker runs (review 5: a denylist of builtins that run commands
// missed twice). tag and worktree only list; branch, checkout and switch move only its own refs.
var gitPermitido = conjunto("status", "add", "commit", "log", "diff", "show", "branch", "switch", "checkout",
	"restore", "rm", "mv", "reset", "stash", "rev-parse", "ls-files", "merge-base", "cat-file", "blame", "grep",
	"describe", "shortlog", "tag", "worktree", "version", "help", "reflog", "ls-tree", "rev-list", "check-ignore")

const listaGit = "status, add, commit, log, diff, show, branch, switch, checkout, restore, rm, mv, reset, stash, " +
	"rev-parse, ls-files, merge-base, cat-file, blame, grep, describe, shortlog, tag -l, worktree list, version, help, reflog (show/list), ls-tree, rev-list, check-ignore"

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
	t := textos(r)
	p := posicionales(r, conjunto("-u", "--set-upstream-to", "-t", "--track", "--contains", "--no-contains", "--merged",
		"--no-merged", "--points-at", "--sort", "--format", "--color", "--column"))
	switch {
	case a[0].dyn || !gitPermitido[sub],
		sub == "tag" && len(r) > 0 && !hay(t, "-l", "--list"),
		sub == "worktree" && (len(p) == 0 || p[0].s != "list"):
		return deny("orq: a worker runs only these git commands: %s (not %q). Ask the orchestrator for anything else.", listaGit, sub), true
	case saleArchivo(t):
		return deny("orq: git --output writes a file the gate cannot check. Use `git %s > file` instead; the gate checks the redirect.", sub), true
	case dinamicaAntes(sub, r) != "":
		return deny("orq: `git %s` has a word before `--` that is not written literally (%q); it could become an option such as --output. Write it literally or put it after `--`.", sub, dinamicaAntes(sub, r)), true
	case sub == "reflog" && len(p) > 0 && !reflogLeer(p[0].s):
		return deny("orq: a worker runs only `git reflog` with no subcommand, show, list, exists or a ref name such as HEAD or refs/heads/x (not %q).", p[0].s), true
	case hay(t, "--ext-diff", "--textconv"):
		return oculto("git " + sub + " --ext-diff/--textconv")
	case sub == "grep" && (conPrefijo(t, "-O", "--open-files-in-pager") || corta(t, 'O')):
		return oculto("git grep --open-files-in-pager")
	}
	fuerza := hay(t, "-f", "--force", "-B", "-C", "--force-create") || conPrefijo(t, "--force-create=") || corta(t, 'f') || corta(t, 'B') || corta(t, 'C')
	switch {
	case sub == "branch" && (hay(t, "-M", "-C") || corta(t, 'M') || corta(t, 'C')):
		for _, x := range p {
			if !propia(rama0(x), rama) {
				return ramaAjena(x.s)
			}
		}
	case sub == "branch" && fuerza && len(p) > 0 && !propia(rama0(p[0]), rama):
		return ramaAjena(p[0].s)
	case (sub == "checkout" || sub == "switch") && fuerza:
		if x, ok := valorDe(r, "-B", "--force-create"); ok && !propia(rama0(x), rama) {
			return ramaAjena(x.s)
		}
		if x, ok := valorDe(r, "-C", "--force-create"); ok && !propia(rama0(x), rama) {
			return ramaAjena(x.s)
		}
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

// saleArchivo reports a git --output option (any form, including git's unambiguous abbreviations
// such as --outp), which writes to any path without the gate seeing it.
func saleArchivo(t []string) bool {
	for _, x := range t {
		if x == "--" {
			return false
		}
		if len(x) >= 4 && strings.HasPrefix("--output", strings.SplitN(x, "=", 2)[0]) {
			return true
		}
	}
	return false
}

// dinamicaAntes returns the first word before `--` that could become an option or subcommand: one
// whose value the gate cannot read (a variable, $(...), backticks, arithmetic) or a glob that
// starts with a glob character or `-`. After `--` git takes every word as a path, so those are
// not checked. Only `git commit` has an exception: a literal option that takes a value consumes the
// next word, which may be dynamic (-m "$msg", -am "$m"); attached forms (--author="$a", -m"$m")
// may have a dynamic suffix. Any other dynamic word is denied, in every subcommand.
func dinamicaAntes(sub string, w []palabra) string {
	for i := 0; i < len(w); i++ {
		x := w[i]
		if x.s == "--" {
			return ""
		}
		if sub == "commit" && !x.dyn {
			if consume, _ := opcionValor(x.s); consume {
				i++
				continue
			}
		}
		if !(x.dyn || x.glob && (strings.HasPrefix(x.s, "-") || strings.ContainsAny(x.s[:1], "*?["))) {
			continue
		}
		if sub == "commit" {
			if _, adjunto := opcionValor(x.s); adjunto {
				continue
			}
		}
		return x.s
	}
	return ""
}

// opcionValor classifies a `git commit` word that starts with a literal value-taking option:
// consume when the next word is its value, adjunto when the value is joined to it.
func opcionValor(s string) (consume, adjunto bool) {
	if strings.HasPrefix(s, "--") {
		o, _, junto := strings.Cut(s, "=")
		if !conjunto("--message", "--file", "--author", "--date")[o] {
			return false, false
		}
		return !junto, junto
	}
	if len(s) < 2 || s[0] != '-' {
		return false, false
	}
	for i := 1; i < len(s); i++ {
		if strings.IndexByte("mFCc", s[i]) >= 0 {
			return i == len(s)-1, i < len(s)-1
		}
	}
	return false, false
}

// reflogLeer says whether the first positional of `git reflog` only reads: show, list, exists or a
// ref name. A ref name is recognised by its shape (HEAD, or it holds one of / @ ~ ^ . :), so a bare
// word such as a future subcommand is denied. Checked against git 2.43, whose subcommands are
// show, expire, delete and exists; `drop` (2.50), `list` and `write` come later. Reading is
// allowlisted rather than denylisting writers so a subcommand added after 2.50 stays denied.
func reflogLeer(s string) bool {
	switch s {
	case "show", "list", "exists", "HEAD":
		return true
	}
	return strings.ContainsAny(s, "/@~^.:")
}
