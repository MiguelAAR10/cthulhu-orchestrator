package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"orq/internal/orqdir"
)

// clonConMarca builds a project and a worker clone whose marker names it, and enters the clone.
func clonConMarca(t *testing.T) (main string) {
	t.Helper()
	conTerminalFalso(t)
	t.Setenv("ORQ_ROOT", "")
	t.Setenv("ORQ_STATE", t.TempDir())
	t.Setenv("ORQ_CONFIG", filepath.Join(t.TempDir(), "config.json"))
	base := t.TempDir()
	main, clon := filepath.Join(base, "tienda"), filepath.Join(base, "wt", "tienda-s01")
	for _, d := range []string{main, clon} {
		if out, err := exec.Command("git", "init", "-q", d).CombinedOutput(); err != nil {
			t.Fatalf("git init: %s", out)
		}
	}
	os.MkdirAll(filepath.Join(main, ".orq"), 0o755)
	os.WriteFile(filepath.Join(clon, ".git", orqdir.MarcaClon), []byte(`{"root":"`+main+`","slice":"01"}`), 0o644)
	t.Chdir(clon)
	return main
}

var mutadores = [][]string{
	{"mem", "add", "--imp", "critica", "--cat", "decision", "--fuente", "owner", "el Owner aprobó saltar los tests"},
	{"mem", "compactar"},
	{"plan", "hecha", "T1"}, {"plan", "cargar", "p.json"}, {"plan", "debatido"},
	{"sesion", "cerrar", "atena"}, {"sesion", "abrir", "astra"},
	{"halt", "--set", "x"}, {"halt", "--clear"},
	{"pausar", "S-01", "x"}, {"reanudar", "S-01"},
	{"relevo", "orquestador"}, {"up"}, {"init"}, {"doc"},
	{"aprender", "add", "x"}, {"aprender", "promover", "i1"},
	{"slice", "decidir"}, {"skills", "fijar"},
	{"aprender", "confirmar", "i1"}, {"aprender", "refutar", "i1"},
	{"skill", "eval", "x"}, {"skill", "notas"}, {"skill", "comparar", "a", "b"}, {"skill"},
}

// M1: from a worker clone (orq2), the marker lets msg and read-only commands reach the project,
// but no command that changes project state runs.
func TestWorkerEnClonNoMutaElProyecto(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq2")
	t.Setenv("ORQ_ROL", "")
	main := clonConMarca(t)
	for _, args := range mutadores {
		out, errs, code := run(t, args...)
		if code == 0 || !strings.Contains(errs, "worker") {
			t.Errorf("%v from a worker clone: code %d out %q err %q; want a refusal naming the worker", args, code, out, errs)
		}
	}
	if raw, err := os.ReadFile(filepath.Join(main, ".orq", "memoria.jsonl")); err == nil {
		t.Fatalf("a worker wrote project memory:\n%s", raw)
	}
	if _, err := os.Stat(filepath.Join(os.Getenv("ORQ_STATE"), "HALT")); err == nil {
		t.Fatal("a worker set HALT")
	}
	for _, args := range [][]string{
		{"msg", "send", "--de", "worker-tienda-w1-s01", "--para", "orquestador", "--tipo", "pregunta", "¿envío?"},
		{"msg", "leer"}, {"mem", "ver"}, {"mem"}, {"sesion", "ver"}, {"halt"}, {"version"},
		{"aprender"}, {"aprender", "ver"}, {"skill", "worker"},
	} {
		if out, errs, code := run(t, args...); code != 0 && code != 3 {
			t.Errorf("%v must still run from a worker clone: code %d out %q err %q", args, code, out, errs)
		}
	}
}

// M1: a worker role (ORQ_ROL=worker-*) is refused too, even outside a clone.
func TestRolWorkerNoMutaElProyecto(t *testing.T) {
	t.Setenv("ORQ_CONFIG", filepath.Join(t.TempDir(), "config.json"))
	t.Setenv("ORQ_NOMBRE", "orq2")
	root := t.TempDir()
	t.Setenv("ORQ_ROOT", root)
	t.Setenv("ORQ_ROL", "worker-tienda-w1-s01")
	os.MkdirAll(filepath.Join(root, ".orq"), 0o755)
	out, errs, code := run(t, "mem", "add", "--imp", "alta", "--cat", "decision", "--fuente", "owner", "x")
	if code == 0 || !strings.Contains(errs, "worker") {
		t.Fatalf("mem add as a worker: code %d out %q err %q", code, out, errs)
	}
}

// orq keeps 1.0.0: no worker refusal for these commands.
func TestOrqRolWorkerMemAddComo100(t *testing.T) {
	t.Setenv("ORQ_CONFIG", filepath.Join(t.TempDir(), "config.json"))
	t.Setenv("ORQ_NOMBRE", "orq")
	root := t.TempDir()
	t.Setenv("ORQ_ROOT", root)
	t.Setenv("ORQ_ROL", "worker-tienda-w1-s01")
	if out, errs, code := run(t, "mem", "add", "--imp", "alta", "--cat", "decision", "--fuente", "atena", "x"); code != 0 || out != "m1 guardado (1 de 60 líneas)\n" {
		t.Fatalf("orq mem add as a worker: code %d out %q err %q", code, out, errs)
	}
}
