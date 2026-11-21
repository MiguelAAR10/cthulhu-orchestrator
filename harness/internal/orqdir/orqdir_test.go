package orqdir

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func gitInit(t *testing.T, dir string) {
	t.Helper()
	if out, err := exec.Command("git", "init", "-q", dir).CombinedOutput(); err != nil {
		t.Fatalf("git init: %s", out)
	}
}

// H17: inside a worker clone (its own .git, with orq's marker) the project is the main checkout
// named by the marker, so `msg send` and every .orq write reach the project, not the clone.
func TestRootEnUnClonDeWorkerEsElCheckoutPrincipal(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq2")
	t.Setenv("ORQ_ROOT", "")
	base := t.TempDir()
	main, clon := filepath.Join(base, "main"), filepath.Join(base, "wt", "main-s01")
	gitInit(t, main)
	gitInit(t, clon)
	if err := os.WriteFile(filepath.Join(clon, ".git", MarcaClon), []byte(`{"root":"`+main+`","slice":"01"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(filepath.Join(clon, "sub"), 0o755)
	t.Chdir(filepath.Join(clon, "sub"))
	root, err := Root()
	if err != nil || root != main {
		t.Fatalf("Root() = %q, %v; want the main checkout %q", root, err, main)
	}
	d, err := Dir()
	if err != nil || d != filepath.Join(main, ".orq") {
		t.Fatalf("Dir() = %q, %v", d, err)
	}
	if _, err := os.Stat(filepath.Join(clon, ".orq")); !os.IsNotExist(err) {
		t.Fatalf("a clone-local .orq was created: %v", err)
	}
}

// A marker that names no usable project fails loudly instead of writing into the clone.
func TestRootConMarcaRotaFalla(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq2")
	t.Setenv("ORQ_ROOT", "")
	clon := filepath.Join(t.TempDir(), "clon")
	gitInit(t, clon)
	for _, marca := range []string{"{roto", `{"root":""}`, `{"root":"relativo"}`, `{"root":"/no/existe/orq"}`} {
		os.WriteFile(filepath.Join(clon, ".git", MarcaClon), []byte(marca), 0o644)
		t.Chdir(clon)
		if root, err := Root(); err == nil {
			t.Errorf("marker %s: Root() = %q, want an error", marca, root)
		}
	}
}

// A plain repository (no marker) keeps its own root.
func TestRootSinMarca(t *testing.T) {
	t.Setenv("ORQ_ROOT", "")
	dir := filepath.Join(t.TempDir(), "repo")
	gitInit(t, dir)
	t.Chdir(dir)
	root, err := Root()
	want, _ := filepath.EvalSymlinks(dir)
	if got, _ := filepath.EvalSymlinks(root); err != nil || got != want {
		t.Fatalf("Root() = %q, %v; want %q", root, err, dir)
	}
}

// A linked worktree (.git is a file) has no marker and keeps the main checkout from git.
func TestRootEnWorktreeEnlazado(t *testing.T) {
	t.Setenv("ORQ_ROOT", "")
	base := t.TempDir()
	main, wt := filepath.Join(base, "main"), filepath.Join(base, "wt")
	gitInit(t, main)
	for _, args := range [][]string{{"-C", main, "-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "--allow-empty", "-m", "x"}, {"-C", main, "worktree", "add", "-q", wt}} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
	t.Chdir(wt)
	root, err := Root()
	want, _ := filepath.EvalSymlinks(main)
	if got, _ := filepath.EvalSymlinks(root); err != nil || got != want {
		t.Fatalf("Root() = %q, %v; want %q", root, err, main)
	}
}

// orq keeps 1.0.0: inside a worker clone the root is the clone itself.
func TestRootOrqEnClonComo100(t *testing.T) {
	t.Setenv("ORQ_ROOT", "")
	t.Setenv("ORQ_NOMBRE", "orq")
	base := t.TempDir()
	main, clon := filepath.Join(base, "main"), filepath.Join(base, "clon")
	gitInit(t, main)
	gitInit(t, clon)
	os.WriteFile(filepath.Join(clon, ".git", MarcaClon), []byte(`{"root":"`+main+`"}`), 0o644)
	t.Chdir(clon)
	root, err := Root()
	want, _ := filepath.EvalSymlinks(clon)
	if got, _ := filepath.EvalSymlinks(root); err != nil || got != want {
		t.Fatalf("orq Root() = %q, %v; want the clone %q", root, err, clon)
	}
}
