package cli

import (
	"slices"
	"testing"
)

func TestWorkerArgsCodexSinAprobacionParaPruebas(t *testing.T) {
	a := workerArgs("codex", "gpt-6-luna", "medium", "w", t.TempDir())
	for _, want := range []string{"workspace-write", "on-request", "sandbox_workspace_write.network_access=true"} {
		if !slices.Contains(a, want) {
			t.Fatalf("falta %q en %v", want, a)
		}
	}
	if c := workerArgs("claude", "opus", "high", "w", t.TempDir()); slices.Contains(c, "workspace-write") {
		t.Fatalf("claude no lleva sandbox de codex: %v", c)
	}
}
