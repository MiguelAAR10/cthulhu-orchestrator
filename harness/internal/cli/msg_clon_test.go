package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"orq/internal/canal"
	"orq/internal/orqdir"
)

// H17: a worker inside its clone runs `orq msg send` with no ORQ_ROOT. The message must land in
// the project's .orq (where the Orchestrator reads), never in a clone-local .orq.
func TestMsgSendDesdeElClonLlegaAlProyecto(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq2")
	conTerminalFalso(t)
	t.Setenv("ORQ_ROOT", "")
	t.Setenv("ORQ_ROL", "worker-tienda-w1-s01")
	base := t.TempDir()
	main, clon := filepath.Join(base, "tienda"), filepath.Join(base, "wt", "tienda-s01")
	for _, d := range []string{main, clon} {
		if out, err := exec.Command("git", "init", "-q", d).CombinedOutput(); err != nil {
			t.Fatalf("git init: %s", out)
		}
	}
	os.MkdirAll(filepath.Join(main, ".orq"), 0o755)
	if err := os.WriteFile(filepath.Join(clon, ".git", orqdir.MarcaClon), []byte(`{"root":"`+main+`","slice":"01"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(clon)
	out, errs, code := run(t, "msg", "send", "--para", "orquestador", "--tipo", "pregunta", "¿el total incluye envío?")
	if code != 0 {
		t.Fatalf("send: %d %s %s", code, out, errs)
	}
	ms, _ := canal.Load(filepath.Join(main, ".orq", "canal.jsonl"))
	if len(ms) != 1 || !strings.Contains(ms[0].Texto, "total incluye") {
		t.Fatalf("the question did not reach the project channel: %+v (out %q)", ms, out)
	}
	if _, err := os.Stat(filepath.Join(clon, ".orq")); !os.IsNotExist(err) {
		t.Fatalf("a clone-local .orq was written: %v", err)
	}
}
