package state

import (
	"path/filepath"
	"testing"
)

// Precedencia: ORQ_STATE > XDG_STATE_HOME/orq > ~/.local/state/orq.
func TestDirPrecedencia(t *testing.T) {
	t.Setenv("HOME", "/h")
	t.Setenv("ORQ_STATE", "")
	t.Setenv("XDG_STATE_HOME", "")
	if got := Dir(); got != filepath.Join("/h", ".local", "state", "orq") {
		t.Fatal(got)
	}
	t.Setenv("XDG_STATE_HOME", "/x")
	if got := Dir(); got != filepath.Join("/x", "orq") {
		t.Fatalf("XDG_STATE_HOME ignorado: %s", got)
	}
	t.Setenv("ORQ_STATE", "/o")
	if got := Dir(); got != "/o" {
		t.Fatal(got)
	}
}
