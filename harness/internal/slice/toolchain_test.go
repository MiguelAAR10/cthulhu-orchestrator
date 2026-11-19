package slice

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// homeConToolchain: a temp HOME where ~/.local/bin/<tool> -> ~/.local/lib/<tool>/bin/<tool> and
// ~/.local/lib/<tool> -> <tool>-1.0, as go installs on this machine. body runs as the tool.
func homeConToolchain(t *testing.T, tool, body string) string {
	t.Helper()
	home := filepath.Join(t.TempDir(), "home")
	t.Setenv("HOME", home)
	t.Setenv("ORQ_STATE", filepath.Join(t.TempDir(), "state"))
	t.Setenv("ORQ_LIMITS", filepath.Join(t.TempDir(), "limits.json"))
	lib := filepath.Join(home, ".local", "lib")
	real := filepath.Join(lib, tool+"-1.0", "bin", tool)
	os.MkdirAll(filepath.Dir(real), 0o755)
	if err := os.WriteFile(real, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	os.Symlink(tool+"-1.0", filepath.Join(lib, tool))
	os.MkdirAll(filepath.Join(home, ".local", "bin"), 0o755)
	if err := os.Symlink(filepath.Join(lib, tool, "bin", tool), filepath.Join(home, ".local", "bin", tool)); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", filepath.Join(home, ".local", "bin")+":"+os.Getenv("PATH"))
	return home
}

// The exam gets the same toolchain binding as the checks: an exam that calls a tool under $HOME runs.
func TestExamenVeSuToolchain(t *testing.T) {
	homeConToolchain(t, "fakego", `cmp -s total.txt tests/datos.txt`)
	r := repoDeJuguete(t)
	e, _, err := r.Abrir("01", "total", "tests/s01.sh", "fakego", Opciones{SinConteo: "juguete", Bundle: []string{"tests/datos.txt"}, Sano: "test -f total.txt"}, t0)
	if err != nil {
		t.Fatal(err)
	}
	commitEn(t, e.Worktree, "total.txt", "170\n")
	e, out, err := r.Verificar("", t0)
	if err != nil || e.Fase != Verificado {
		t.Fatalf("%v %s\n%s", err, e.Fase, strings.TrimSpace(out))
	}
}
