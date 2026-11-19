package slice

import (
	"os"
	"path/filepath"
	"slices"
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

// Corto never panics on a short, empty or ref-name value read from the state.
func TestCortoNoEntraEnPanico(t *testing.T) {
	for in, want := range map[string]string{"": "", "abc": "abc", "slice/01": "slice/01", "0123456789abcdef": "01234567"} {
		if got := Corto(in); got != want {
			t.Errorf("Corto(%q) = %q, want %q", in, got, want)
		}
	}
}

// The report names each test; a TAP number is dropped, since it shifts when fewer tests run.
func TestContarNombres(t *testing.T) {
	casos := []struct{ formato, raw, fallo, paso string }{
		{"tap", "not ok 1 - cart total # expected 170\nok 2 page loads\n", "cart total", "page loads"},
		{"junit", `<testsuites><testcase classname="a.spec.ts" name="x"><failure/></testcase><testcase name="y"/></testsuites>`, "a.spec.ts › x", "y"},
		{"gotest-json", `{"Action":"fail","Package":"p","Test":"TestX"}` + "\n" + `{"Action":"pass","Package":"p","Test":"TestY"}`, "p TestX", "p TestY"},
	}
	for _, c := range casos {
		got, err := Contar(c.formato, []byte(c.raw))
		if err != nil || !slices.Equal(got.Fallaron, []string{c.fallo}) || !slices.Equal(got.Pasaron, []string{c.paso}) {
			t.Fatalf("%s: %+v %v", c.formato, got, err)
		}
	}
	if f := faltanDelExamen([]string{"cart total"}, &Conteo{Pasaron: []string{"page loads"}}); !slices.Equal(f, []string{"cart total"}) {
		t.Fatalf("faltan = %v", f)
	}
}
