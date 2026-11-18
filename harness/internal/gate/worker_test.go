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
	}
	for _, c := range cases {
		c.p.Cwd = clon
		if d := Evaluate(c.p, baseLimits(), env); d.Allow {
			t.Errorf("%s: allowed inside a worker clone, must be denied", c.nombre)
		}
	}
}
