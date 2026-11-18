package slice

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The marker of a worker clone is found through a repo nested inside it.
func TestLeerMarcaAtraviesaRepoAnidado(t *testing.T) {
	clon := filepath.Join(t.TempDir(), "clon")
	if _, err := Git(t.TempDir(), "init", "-q", clon); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(clon, ".git", MarcaClon), []byte(`{"root":"/x/repo","slice":"01","bundle":["tests/s01.sh"]}`), 0o644)
	sub := filepath.Join(clon, "sub")
	if _, err := Git(clon, "init", "-q", sub); err != nil {
		t.Fatal(err)
	}
	m, ok := LeerMarca(filepath.Join(sub))
	if !ok || m.Root != "/x/repo" || len(m.Bundle) != 1 {
		t.Fatalf("a repo nested in a clone must read the clone's marker: %+v %v", m, ok)
	}
	if _, ok := LeerMarca(filepath.Dir(clon)); ok {
		t.Fatal("outside any clone there is no marker")
	}
}

// An empty or relative directory cannot be placed: it counts as a worker and never hangs.
func TestLeerMarcaCwdVacioORelativo(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, dir := range []string{"", ".", "a/b"} {
		listo := make(chan bool, 1)
		go func() { _, ok := LeerMarca(dir); listo <- ok }()
		select {
		case ok := <-listo:
			if !ok {
				t.Errorf("dir %q: must count as a worker", dir)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("dir %q: LeerMarca hangs", dir)
		}
	}
}
