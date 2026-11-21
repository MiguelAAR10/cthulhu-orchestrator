package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Under identity orq, G1's behaviour changes stay off: orq is byte-identical to 1.0.0.

// 1.0.0 deletes the channel on close and the next story starts again at c1.
func TestOrqCerrarBorraElCanalComo100(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq")
	sliceVerde(t)
	root := os.Getenv("ORQ_ROOT")
	t.Setenv("ORQ_SIN_DOC", "1")
	run(t, "msg", "send", "--de", "atena", "--para", "orquestador", "--tipo", "historia", "quiero el total")
	if out, errs, code := run(t, "slice", "cerrar", "--aceptado"); code != 0 || !strings.Contains(out, "Canal limpio") {
		t.Fatalf("cerrar: %d %s %s", code, out, errs)
	}
	if _, err := os.Stat(filepath.Join(root, ".orq", "canales")); !os.IsNotExist(err) {
		t.Fatalf("orq must not archive the channel: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".orq", "canal.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("orq deletes the channel on close: %v", err)
	}
	if out, _, _ := run(t, "msg", "send", "--de", "atena", "--para", "orquestador", "--tipo", "historia", "otra"); !strings.Contains(out, "guardado como c1,") {
		t.Fatalf("orq restarts ids at c1 after a close: %s", out)
	}
}

// 1.0.0 fails mem ver, mem add and mem compactar on the first unreadable line, with its message.
func TestOrqMemoriaRotaFallaComo100(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq")
	root := t.TempDir()
	t.Setenv("ORQ_ROOT", root)
	os.MkdirAll(filepath.Join(root, ".orq"), 0o755)
	mem := filepath.Join(root, ".orq", "memoria.jsonl")
	os.WriteFile(mem, []byte(memRota), 0o644)
	want := mem + ":2: unexpected end of JSON input\n"
	for _, args := range [][]string{{"mem", "ver"}, {"mem", "compactar"}} {
		out, errs, code := run(t, args...)
		if code != 1 || out != "" || errs != want {
			t.Errorf("%v: code %d out %q err %q; want 1.0.0 %q", args, code, out, errs, want)
		}
	}
	_, errs, code := run(t, "mem", "add", "--imp", "media", "--cat", "dominio", "--fuente", "atena", "talla única")
	if code != 1 || errs != "orq mem: "+want && errs != want {
		t.Errorf("mem add: code %d err %q", code, errs)
	}
	if raw, _ := os.ReadFile(mem); string(raw) != memRota {
		t.Fatalf("orq must not write to a memory it cannot read:\n%s", raw)
	}
}

// 1.0.0 treats an unreadable slice state as nothing to integrate.
func TestOrqCerrarWorkerEstadoIlegibleComo100(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq")
	root := repoCLI(t)
	t.Setenv("ORQ_STATE", t.TempDir())
	if _, errs, code := run(t, append(abrirBase, "--reporte", "out.tap", "--formato", "tap")...); code != 0 {
		t.Fatalf("abrir: %d %s", code, errs)
	}
	os.WriteFile(filepath.Join(root, ".orq", "slices", "01.json"), []byte("{roto"), 0o644)
	p, err := abrirProyecto()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.CerrarWorker(context.Background(), "worker-claude-s01"); err != nil {
		t.Fatalf("orq 1.0.0 returns nil here: %v", err)
	}
}
