package gate

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"orq/internal/orqdir"
)

func gitInit(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--allow-empty", "-m", "init"},
	} {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

// clonMarcado builds a repo whose .git holds the worker-clone marker with the given content.
func clonMarcado(t *testing.T, contenido string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "clon")
	gitInit(t, dir)
	if err := os.WriteFile(filepath.Join(dir, ".git", orqdir.MarcaClon), []byte(contenido), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestIsWorkerClonWithMarker(t *testing.T) {
	dir := clonMarcado(t, `{"root":"/x/repo","slice":"01","rama":"slice/01"}`)
	sub := filepath.Join(dir, "sub", "dir")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if !IsWorker(sub) {
		t.Fatal("a directory inside a marked clone must be a worker")
	}
}

func TestIsWorkerCorruptMarker(t *testing.T) {
	for _, contenido := range []string{"{", ""} {
		if !IsWorker(clonMarcado(t, contenido)) {
			t.Fatalf("marker %q: a corrupt or empty marker must fail closed (worker)", contenido)
		}
	}
}

func TestIsWorkerPlainRepo(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "repo")
	gitInit(t, dir)
	if IsWorker(dir) {
		t.Fatal("a plain repo without the marker is the main checkout, not a worker")
	}
}

func TestIsWorkerLinkedWorktree(t *testing.T) {
	base := t.TempDir()
	repo, wt := filepath.Join(base, "repo"), filepath.Join(base, "wt")
	gitInit(t, repo)
	if out, err := exec.Command("git", "-C", repo, "worktree", "add", "-q", "-b", "w", wt).CombinedOutput(); err != nil {
		t.Fatalf("worktree add: %v\n%s", err, out)
	}
	if !IsWorker(wt) {
		t.Fatal("a linked worktree must be a worker")
	}
}

func TestGateDeniesInClone(t *testing.T) {
	base := t.TempDir()
	principal := filepath.Join(base, "main")
	gitInit(t, principal)
	clon := clonMarcado(t, `{"root":"`+principal+`","slice":"01","rama":"slice/01","bundle":["examen/s01.sh"]}`)
	env := Env{
		DiskFreeGB: func(string) (float64, error) { return 999, nil },
		Workers:    func() (int, error) { return 0, nil },
		Now:        time.Now,
		StateDir:   filepath.Join(base, "state"),
		IsWorktree: IsWorker,
		Bundle:     func(string) []string { return []string{"examen/s01.sh"} },
		LimitsPath: filepath.Join(base, "limits.json"),
		MainRoot:   func(string) string { return principal },
	}
	cases := []struct {
		nombre string
		p      Payload
	}{
		{"edit of the bundle file", Payload{ToolName: "Edit", ToolInput: map[string]any{"file_path": filepath.Join(clon, "examen/s01.sh")}}},
		{"shell write to the exam", Payload{ToolName: "Bash", ToolInput: map[string]any{"command": "echo x > tests/aceptacion/s01.sh"}}},
		{"subagent launch", Payload{ToolName: "Agent", ToolInput: map[string]any{"prompt": "help"}}},
		{"write to the main .orq", Payload{ToolName: "Write", ToolInput: map[string]any{"file_path": filepath.Join(principal, ".orq/slices/01.json")}}},
		{"close the slice", Payload{ToolName: "Bash", ToolInput: map[string]any{"command": "orq slice cerrar --aceptado"}}},
		// The marker is what makes the clone a worker: removing or rewriting it would turn the gate off.
		{"move the clone marker away", Payload{ToolName: "Bash", ToolInput: map[string]any{"command": "mv .git/orq-clon /tmp/x"}}},
		{"truncate the clone marker", Payload{ToolName: "Bash", ToolInput: map[string]any{"command": ": > .git/orq-clon"}}},
		{"write the clone marker", Payload{ToolName: "Write", ToolInput: map[string]any{"file_path": filepath.Join(clon, ".git", "orq-clon"), "content": "{}"}}},
		{"edit the clone marker", Payload{ToolName: "Edit", ToolInput: map[string]any{"file_path": ".git/orq-clon"}}},
		{"move the clone's .git", Payload{ToolName: "Bash", ToolInput: map[string]any{"command": "mv .git .git-old"}}},
	}
	for _, c := range cases {
		c.p.Cwd = clon
		if d := Evaluate(c.p, baseLimits(), env); d.Allow {
			t.Errorf("%s: allowed inside a worker clone, must be denied", c.nombre)
		}
	}
}

// A repo nested inside a worker clone (`git init sub`, `git clone . sub`) must not turn the gate
// off: the marker of an outer clone still makes every directory below it a worker.
func TestGateDeniesInRepoNestedInClone(t *testing.T) {
	base := t.TempDir()
	principal := filepath.Join(base, "main")
	gitInit(t, principal)
	clon := clonMarcado(t, `{"root":"`+principal+`","slice":"01","rama":"slice/01","bundle":["tests/aceptacion/s01.sh"]}`)
	iniciado := filepath.Join(clon, "sub")
	gitInit(t, iniciado)
	clonado := filepath.Join(clon, "copia")
	if out, err := exec.Command("git", "clone", "-q", clon, clonado).CombinedOutput(); err != nil {
		t.Fatalf("clone: %v\n%s", err, out)
	}
	os.Remove(filepath.Join(clonado, ".git", orqdir.MarcaClon)) // a clone copies no marker; be sure
	env := Env{
		DiskFreeGB: func(string) (float64, error) { return 999, nil },
		Workers:    func() (int, error) { return 0, nil },
		Now:        time.Now,
		StateDir:   filepath.Join(base, "state"),
		IsWorktree: IsWorker,
		Bundle:     func(string) []string { return []string{"tests/aceptacion/s01.sh"} },
		LimitsPath: filepath.Join(base, "limits.json"),
		MainRoot:   func(string) string { return principal },
	}
	for _, cwd := range []string{iniciado, clonado} {
		if !IsWorker(cwd) {
			t.Errorf("%s: a repo nested in a marked clone must be a worker", cwd)
		}
		cases := []struct {
			nombre string
			p      Payload
		}{
			{"edit of the exam", Payload{ToolName: "Edit", ToolInput: map[string]any{"file_path": filepath.Join(clon, "tests/aceptacion/s01.sh")}}},
			{"approve the exam", Payload{ToolName: "Bash", ToolInput: map[string]any{"command": "orq slice examen tests/aceptacion/s01.sh --aprobado x"}}},
			{"approve the exam from main", Payload{ToolName: "Bash", ToolInput: map[string]any{"command": "cd " + principal + " && orq slice examen tests/aceptacion/s01.sh --aprobado x"}}},
			{"write to the main .orq", Payload{ToolName: "Write", ToolInput: map[string]any{"file_path": filepath.Join(principal, ".orq/slices/01.json")}}},
		}
		for _, c := range cases {
			c.p.Cwd = cwd
			if d := Evaluate(c.p, baseLimits(), env); d.Allow {
				t.Errorf("%s from %s: allowed in a repo nested in a worker clone, must be denied", c.nombre, cwd)
			}
		}
	}
}

// An empty or relative cwd cannot be placed: it is a worker (fail closed), and it never hangs.
func TestIsWorkerCwdVacioORelativo(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, cwd := range []string{"", ".", "sub/dir"} {
		listo := make(chan bool, 1)
		go func() { listo <- IsWorker(cwd) }()
		select {
		case w := <-listo:
			if !w {
				t.Errorf("cwd %q: must be treated as a worker", cwd)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("cwd %q: IsWorker hangs", cwd)
		}
	}
}

// The whole gate with an empty or relative cwd answers (it does not hang) and treats it as a worker.
func TestGateCwdVacioORelativoNoSeCuelga(t *testing.T) {
	t.Chdir(t.TempDir())
	base := t.TempDir()
	env := Env{
		DiskFreeGB: func(string) (float64, error) { return 999, nil },
		Workers:    func() (int, error) { return 0, nil },
		Now:        time.Now,
		StateDir:   filepath.Join(base, "state"),
		IsWorktree: IsWorker,
		Bundle:     func(string) []string { return nil },
		LimitsPath: filepath.Join(base, "limits.json"),
		MainRoot:   CheckoutPrincipal,
	}
	for _, cwd := range []string{"", ".", "a/b"} {
		listo := make(chan Decision, 1)
		go func() {
			listo <- Evaluate(Payload{ToolName: "Bash", Cwd: cwd, ToolInput: map[string]any{"command": "orq slice examen x --aprobado y"}}, baseLimits(), env)
		}()
		select {
		case d := <-listo:
			if d.Allow {
				t.Errorf("cwd %q: a worker-only denial was allowed", cwd)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("cwd %q: the gate hangs", cwd)
		}
	}
}

// CheckoutPrincipal with an empty or relative dir answers "" instead of walking "." forever.
func TestCheckoutPrincipalCwdRelativoNoSeCuelga(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, cwd := range []string{"", ".", "a/b"} {
		listo := make(chan string, 1)
		go func() { listo <- CheckoutPrincipal(cwd) }()
		select {
		case r := <-listo:
			if r != "" {
				t.Errorf("cwd %q: %q", cwd, r)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("cwd %q: CheckoutPrincipal hangs", cwd)
		}
	}
}

// RepoPropio places the worker's repository at its marked clone, through nested repos, with the
// branch the marker names; an unmarked repo has no branch; a relative dir cannot be placed.
func TestRepoPropio(t *testing.T) {
	clon := clonMarcado(t, `{"root":"/x/repo","slice":"01","rama":"slice/01"}`)
	anidado := filepath.Join(clon, "sub")
	gitInit(t, anidado)
	for _, dir := range []string{clon, filepath.Join(clon, "a"), anidado} {
		if raiz, rama := RepoPropio(dir); raiz != clon || rama != "slice/01" {
			t.Fatalf("%s: got %q %q", dir, raiz, rama)
		}
	}
	plano := filepath.Join(t.TempDir(), "repo")
	gitInit(t, plano)
	if raiz, rama := RepoPropio(filepath.Join(plano, "x")); raiz != plano || rama != "" {
		t.Fatalf("plain repo: got %q %q", raiz, rama)
	}
	if raiz, rama := RepoPropio(filepath.Join(clonMarcado(t, "{"), "x")); raiz == "" || rama != "" {
		t.Fatalf("corrupt marker: the clone is still the repo, with no branch: %q %q", raiz, rama)
	}
	if raiz, _ := RepoPropio("rel/dir"); raiz != "" {
		t.Fatalf("relative dir: got %q", raiz)
	}
}

// From a real clone, git against the main repo is denied and git in the clone passes.
func TestWorkerGitContraElRepoPrincipal(t *testing.T) {
	principal := filepath.Join(t.TempDir(), "repo")
	gitInit(t, principal)
	clon := clonMarcado(t, `{"root":"`+principal+`","slice":"01","rama":"slice/01"}`)
	env := Env{Now: time.Now, IsWorktree: IsWorker, RepoPropio: RepoPropio}
	for cmd, permite := range map[string]bool{
		"git -C " + principal + " update-ref refs/heads/main HEAD": false,
		"cd " + principal + " && git commit --allow-empty -m x":    false,
		"git status && git -C . log":                               true,
	} {
		d := Evaluate(Payload{ToolName: "Bash", Cwd: clon, ToolInput: map[string]any{"command": cmd}}, baseLimits(), env)
		if d.Allow != permite {
			t.Fatalf("%s: allow=%v (%s)", cmd, d.Allow, d.Reason)
		}
	}
}
