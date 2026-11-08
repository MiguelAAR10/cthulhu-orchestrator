// Package memsync decide dónde vive la memoria de orq (.orq/): solo en esta máquina (local),
// versionada con el código (commit) o en la rama orq/state (branch). En branch, push y pull usan
// un worktree aparte (.orq-sync) y nunca tocan el HEAD, el índice ni las ramas del usuario.
// Nunca hay --force, +refspec ni rebase: si diverge, se detiene.
package memsync

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

const (
	Branch     = "orq/state"
	SyncDir    = ".orq-sync"
	beginBlock = "# >>> orq memory sync (managed by orq init)"
	endBlock   = "# <<< orq memory sync"
)

var Modes = []string{"local", "commit", "branch"}

// ErrDiverged: la rama orq/state local y la remota divergieron.
var ErrDiverged = errors.New("orq/state diverged from origin; resolve by hand (orq never forces)")

func ignoreLines(mode string) []string {
	switch mode {
	case "commit":
		return []string{"/.orq/workers/", "/.orq/*.lock", "/.orq/**/*.lock", "/.orq/**/*.tmp*", "/.orq/board.html", "/.orq/salidas/", "/.orq-sync/"}
	case "branch":
		return []string{"/.orq/", "/.orq-sync/"}
	}
	return []string{"/.orq/"}
}

// SetMode escribe el bloque de .gitignore del modo. En commit, saca /.orq/ de .git/info/exclude.
func SetMode(root, mode string) error {
	if !slices.Contains(Modes, mode) {
		return fmt.Errorf("sync mode %q: use local|commit|branch", mode)
	}
	p := filepath.Join(root, ".gitignore")
	raw, _ := os.ReadFile(p)
	var keep []string
	in := false
	for _, l := range strings.Split(strings.TrimRight(string(raw), "\n"), "\n") {
		switch {
		case l == beginBlock:
			in = true
		case l == endBlock:
			in = false
		case !in && l != "":
			keep = append(keep, l)
		case !in && l == "" && len(keep) > 0:
			keep = append(keep, l)
		}
	}
	for len(keep) > 0 && keep[len(keep)-1] == "" {
		keep = keep[:len(keep)-1]
	}
	block := append([]string{beginBlock, "# sync=" + mode}, ignoreLines(mode)...)
	block = append(block, endBlock)
	out := strings.Join(append(keep, block...), "\n") + "\n"
	if len(keep) > 0 {
		out = strings.Join(keep, "\n") + "\n\n" + strings.Join(block, "\n") + "\n"
	}
	if err := os.WriteFile(p, []byte(out), 0o644); err != nil {
		return err
	}
	if mode == "commit" {
		ex := filepath.Join(root, ".git", "info", "exclude")
		if raw, err := os.ReadFile(ex); err == nil {
			var ls []string
			for _, l := range strings.Split(string(raw), "\n") {
				if l != "/.orq/" {
					ls = append(ls, l)
				}
			}
			os.WriteFile(ex, []byte(strings.Join(ls, "\n")), 0o644)
		}
	}
	return nil
}

// Commands describe, para HANDOFF.md, cómo se mueve la memoria en cada modo.
func Commands(mode string) []string {
	switch mode {
	case "commit":
		return []string{"`.orq/` is versioned with the code (except workers/, locks, board.html, outputs).", "Commit it as usual: `git add .orq && git commit`. orq never commits on its own."}
	case "branch":
		return []string{"`.orq/` lives on the `orq/state` branch, never on working branches.", "Save: `orq sync push` · Restore on another machine: `orq sync pull` (`--theirs` to overwrite newer local files)."}
	}
	return []string{"`.orq/` stays on this machine only (git-ignored). Copy the folder by hand to move it."}
}

type gitRunner struct{ dir string }

func (g gitRunner) run(args ...string) (string, error) {
	c := exec.Command("git", args...)
	c.Dir = g.dir
	c.Env = os.Environ()
	if out, _ := exec.Command("git", "-C", g.dir, "config", "user.email").Output(); strings.TrimSpace(string(out)) == "" {
		c.Env = append(c.Env, "GIT_AUTHOR_NAME=orq", "GIT_AUTHOR_EMAIL=orq@localhost", "GIT_COMMITTER_NAME=orq", "GIT_COMMITTER_EMAIL=orq@localhost")
	}
	out, err := c.CombinedOutput()
	s := strings.TrimSpace(string(out))
	if err != nil {
		return s, fmt.Errorf("git %s: %s", strings.Join(args, " "), s)
	}
	return s, nil
}

func (g gitRunner) ok(args ...string) bool { _, err := g.run(args...); return err == nil }

const localRef, remoteRef = "refs/heads/" + Branch, "refs/remotes/origin/" + Branch

// fetch trae origin/orq/state si existe. Devuelve si existe en el remoto.
func fetch(g gitRunner) (bool, error) {
	if !g.ok("remote", "get-url", "origin") {
		return false, errors.New("no remote 'origin'")
	}
	out, err := g.run("ls-remote", "--heads", "origin", Branch)
	if err != nil {
		return false, err
	}
	if out == "" {
		return false, nil
	}
	_, err = g.run("fetch", "-q", "origin", localRef+":"+remoteRef)
	return err == nil, err
}

// fastForward lleva la rama local al remoto si es ancestro; detiene si divergieron.
func fastForward(g gitRunner) error {
	if !g.ok("rev-parse", "--verify", "-q", remoteRef) {
		return nil
	}
	if !g.ok("rev-parse", "--verify", "-q", localRef) {
		_, err := g.run("update-ref", localRef, remoteRef)
		return err
	}
	if g.ok("merge-base", "--is-ancestor", localRef, remoteRef) {
		_, err := g.run("update-ref", localRef, remoteRef)
		return err
	}
	if g.ok("merge-base", "--is-ancestor", remoteRef, localRef) {
		return nil // local adelante: push lo publica
	}
	return ErrDiverged
}

// worktree abre .orq-sync en orq/state (crea la rama huérfana con plumbing la primera vez).
func worktree(g gitRunner, root string) (string, func(), error) {
	dir := filepath.Join(root, SyncDir)
	if _, err := os.Stat(dir); err == nil {
		return "", nil, fmt.Errorf("%s already exists: a previous sync was interrupted; check it and remove it with `git worktree remove %s`", SyncDir, SyncDir)
	}
	if !g.ok("rev-parse", "--verify", "-q", localRef) {
		tree, err := g.run("mktree")
		if err != nil {
			return "", nil, err
		}
		c, err := g.run("commit-tree", tree, "-m", "orq state: start")
		if err != nil {
			return "", nil, err
		}
		if _, err := g.run("update-ref", localRef, c, ""); err != nil {
			return "", nil, err
		}
	}
	if _, err := g.run("worktree", "add", "-q", dir, Branch); err != nil {
		return "", nil, err
	}
	return dir, func() { g.run("worktree", "remove", dir); g.run("worktree", "prune") }, nil
}

func skip(rel string, d fs.DirEntry) bool {
	base := filepath.Base(rel)
	return (d.IsDir() && rel == "workers") || strings.HasSuffix(base, ".lock") || strings.Contains(base, ".tmp") || base == "board.html"
}

// mirror copia src en dst (dst queda igual a src, sin lo excluido).
func mirror(src, dst string) error {
	os.RemoveAll(dst)
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		if rel == "." {
			return os.MkdirAll(dst, 0o755)
		}
		if skip(rel, d) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		if !d.Type().IsRegular() {
			return nil
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dst, rel), raw, 0o644)
	})
}

// Push guarda .orq en orq/state y lo publica solo por avance rápido.
func Push(root string, now time.Time) (string, error) {
	g := gitRunner{root}
	var log []string
	if _, err := fetch(g); err != nil {
		return "", err
	}
	if err := fastForward(g); err != nil {
		return "", err
	}
	dir, done, err := worktree(g, root)
	if err != nil {
		return "", err
	}
	defer done()
	if err := mirror(filepath.Join(root, ".orq"), filepath.Join(dir, "orq")); err != nil {
		return "", err
	}
	w := gitRunner{dir}
	if _, err := w.run("add", "-A", "orq"); err != nil {
		return "", err
	}
	if w.ok("diff", "--cached", "--quiet") {
		log = append(log, "no changes in .orq since the last push")
	} else if _, err := w.run("commit", "-q", "-m", "orq state "+now.UTC().Format(time.RFC3339)); err != nil {
		return "", err
	} else {
		log = append(log, "committed .orq to "+Branch)
	}
	if _, err := g.run("push", "-q", "origin", localRef+":"+localRef); err != nil {
		// Rechazado: pull y un solo reintento.
		if _, ferr := fetch(g); ferr != nil {
			return strings.Join(log, "\n"), ferr
		}
		if ferr := fastForward(g); ferr != nil {
			return strings.Join(log, "\n"), ferr
		}
		if _, err := g.run("push", "-q", "origin", localRef+":"+localRef); err != nil {
			return strings.Join(log, "\n"), fmt.Errorf("push rejected twice, stopping: %v", err)
		}
	}
	log = append(log, "pushed "+Branch+" (fast-forward)")
	return strings.Join(log, "\n"), nil
}

// Pull trae orq/state y copia orq/ en .orq. Un archivo local más nuevo que el commit se
// conserva salvo con theirs.
func Pull(root string, theirs bool) (string, error) {
	g := gitRunner{root}
	exists, err := fetch(g)
	if err != nil {
		return "", err
	}
	if !exists {
		return "origin has no " + Branch + " yet: nothing to pull", nil
	}
	if !g.ok("rev-parse", "--verify", "-q", localRef) || g.ok("merge-base", "--is-ancestor", localRef, remoteRef) {
		if _, err := g.run("update-ref", localRef, remoteRef); err != nil {
			return "", err
		}
	} else if !g.ok("merge-base", "--is-ancestor", remoteRef, localRef) {
		return "", ErrDiverged
	}
	ct, _ := g.run("log", "-1", "--format=%ct", localRef)
	sec, _ := strconv.ParseInt(ct, 10, 64)
	stamp := time.Unix(sec, 0)
	dir, done, err := worktree(g, root)
	if err != nil {
		return "", err
	}
	defer done()
	src := filepath.Join(dir, "orq")
	dst := filepath.Join(root, ".orq")
	var kept []string
	n := 0
	err = filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return filepath.SkipDir
			}
			return err
		}
		rel, _ := filepath.Rel(src, p)
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		target := filepath.Join(dst, rel)
		if fi, err := os.Stat(target); err == nil && fi.ModTime().After(stamp) && !theirs {
			kept = append(kept, rel)
			return nil
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		n++
		return os.WriteFile(target, raw, 0o644)
	})
	if err != nil {
		return "", err
	}
	out := fmt.Sprintf("pulled %s: %d files copied into .orq", Branch, n)
	if len(kept) > 0 {
		out += fmt.Sprintf("\nkept %d newer local files (use --theirs to overwrite): %s", len(kept), strings.Join(kept, ", "))
	}
	return out, nil
}
