package agente

import (
	"slices"
	"testing"
)

func TestArgsWorkerCodexSinAprobacionParaPruebas(t *testing.T) {
	a := ArgsWorker("codex", "m-codex", "medium", "w", "/repo/.git")
	for _, want := range []string{"workspace-write", "on-request", "sandbox_workspace_write.network_access=true", "/repo/.git"} {
		if !slices.Contains(a, want) {
			t.Fatalf("falta %q en %v", want, a)
		}
	}
	if c := ArgsWorker("claude", "m-claude", "high", "w", "/repo/.git"); slices.Contains(c, "workspace-write") || slices.Contains(c, "/repo/.git") {
		t.Fatalf("claude no lleva sandbox de codex: %v", c)
	}
	if got := Args("claude", "m", "high", "nombre"); !slices.Equal(got, []string{"--model", "m", "--effort", "high", "-n", "nombre"}) {
		t.Fatalf("%v", got)
	}
}
