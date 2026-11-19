package sandbox

import (
	"context"
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

// This machine's layout: ~/.local/bin/go -> ~/.local/lib/go/bin/go and ~/.local/lib/go -> go-1.25.8.
// Every hop must be visible in the box, and the toolchain root that holds the real binary's siblings.
func TestHerramientasSiguenCadaEnlace(t *testing.T) {
	exigeBwrap(t)
	home := filepath.Join(t.TempDir(), "home")
	t.Setenv("HOME", home)
	lib := filepath.Join(home, ".local", "lib")
	root := filepath.Join(lib, "go-1.25.8")
	stubTool(t, filepath.Join(root, "bin", "go"), `cat "$(dirname "$(readlink -f "$0")")/../VERSION"`)
	os.WriteFile(filepath.Join(root, "VERSION"), []byte("go1.25.8\n"), 0o644)
	if err := os.Symlink("go-1.25.8", filepath.Join(lib, "go")); err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(filepath.Join(home, ".local", "bin"), 0o755)
	if err := os.Symlink(filepath.Join(lib, "go", "bin", "go"), filepath.Join(home, ".local", "bin", "go")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", filepath.Join(home, ".local", "bin")+":/usr/bin:/bin")

	hs := Herramientas("go vet ./...")
	if NoCorren(hs) != "" {
		t.Fatalf("go should run: %+v", hs)
	}
	ro := RODe(hs)
	cmd, err := Command(context.Background(), Spec{RO: ro}, "sh", "-c", "command -v go && go version")
	if err != nil {
		t.Fatal(err)
	}
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "go1.25.8") {
		t.Fatalf("in the box with %v: %v\n%s", ro, err, out)
	}

	// A hop through a denied directory keeps the tool out, with the reason.
	stubTool(t, filepath.Join(home, ".config", "tc", "bin", "mal"), "echo no")
	if err := os.Symlink(filepath.Join(home, ".config", "tc", "bin", "mal"), filepath.Join(home, ".local", "bin", "mal")); err != nil {
		t.Fatal(err)
	}
	if hs := Herramientas("mal"); len(RODe(hs)) != 0 || !strings.Contains(NoCorren(hs), "nunca entra en la caja") {
		t.Fatalf("hop into ~/.config: %+v", hs)
	}
}
