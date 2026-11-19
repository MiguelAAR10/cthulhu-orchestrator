package sandbox

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// stubTool writes an executable script at p and returns p.
func stubTool(t *testing.T, p, body string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

// A stub toolchain under a temp HOME: ~/.local/bin/faketool links into ~/.local/lib/fake/bin, the
// way go or node install. Its directories are bound; ~/.local itself is not (it holds ~/.local/state),
// and a tool under ~/.config never is.
func TestHerramientasUnderHome(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	t.Setenv("HOME", home)
	t.Setenv("ORQ_STATE", filepath.Join(home, ".local", "state", "orq"))
	real := stubTool(t, filepath.Join(home, ".local", "lib", "fake", "bin", "faketool"), "echo fake")
	os.MkdirAll(filepath.Join(home, ".local", "bin"), 0o755)
	if err := os.Symlink(real, filepath.Join(home, ".local", "bin", "faketool")); err != nil {
		t.Fatal(err)
	}
	stubTool(t, filepath.Join(home, ".config", "evil", "bin", "eviltool"), "echo evil")
	stubTool(t, filepath.Join(home, ".claude-alt", "bin", "clautool"), "echo no")
	t.Setenv("PATH", strings.Join([]string{filepath.Join(home, ".local", "bin"), filepath.Join(home, ".config", "evil", "bin"),
		filepath.Join(home, ".claude-alt", "bin"), "/usr/bin", "/bin"}, ":"))

	hs := Herramientas(`out=$(faketool -l . 2>&1) && test -z "$out"`)
	ro := RODe(hs)
	want := []string{filepath.Join(home, ".local", "bin"), filepath.Join(home, ".local", "lib", "fake")}
	if !slices.Equal(ro, want) {
		t.Fatalf("ro = %v, want %v (hs %+v)", ro, want, hs)
	}
	if NoCorren(hs) != "" {
		t.Fatalf("faketool should run: %+v", hs)
	}
	for _, cmd := range []string{"eviltool --x", "clautool"} {
		hs := Herramientas(cmd)
		if len(RODe(hs)) != 0 || !strings.Contains(NoCorren(hs), "nunca entra en la caja") {
			t.Errorf("%s: %+v", cmd, hs)
		}
	}
	if hs := Herramientas("nosuchtool run"); !strings.Contains(NoCorren(hs), "nosuchtool no está en el PATH") {
		t.Errorf("missing tool: %+v", hs)
	}
	if hs := Herramientas("sh -c true"); NoCorren(hs) != "" || len(RODe(hs)) != 0 {
		t.Errorf("system tool: %+v", hs)
	}
}
