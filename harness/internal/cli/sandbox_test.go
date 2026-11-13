package cli

import (
	"path/filepath"
	"slices"
	"testing"
)

func TestParseSandboxExec(t *testing.T) {
	s, argv, err := parseSandboxExec([]string{"--rw", "/a", "--ro=/b", "--red", "--cwd", "/a", "--", "git", "--rw", "x"})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(s.RW, []string{"/a"}) || !slices.Equal(s.RO, []string{"/b"}) || !s.Net || s.Cwd != "/a" {
		t.Fatalf("spec mal leído: %+v", s)
	}
	if !slices.Equal(argv, []string{"git", "--rw", "x"}) {
		t.Fatalf("lo que sigue a -- es del comando: %v", argv)
	}
	s, _, err = parseSandboxExec([]string{"--rw", "rel", "--", "true"})
	if err != nil || !filepath.IsAbs(s.RW[0]) {
		t.Fatalf("una ruta relativa se vuelve absoluta: %v %v", s.RW, err)
	}
	for _, mal := range [][]string{{"true"}, {"--rw", "/a"}, {"--rw"}, {"--"}, {"--net", "--", "true"}} {
		if _, _, err := parseSandboxExec(mal); err == nil {
			t.Errorf("debía fallar: %v", mal)
		}
	}
}
