package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const memRota = `{"id":"m1","imp":"critica","cat":"requisito","fuente":"owner","texto":"el total incluye envío"}
{"id":"m2","imp":"alta","cat":"deci
{"id":"m3","imp":"alta","cat":"decision","fuente":"atena","texto":"pago con tarjeta"}
`

// H13/H27: one torn memory line no longer hides all memory from a new session; it is reported.
func TestMemoriaConLineaRotaSeVeYSeAvisa(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq2")
	root := t.TempDir()
	t.Setenv("ORQ_ROOT", root)
	t.Setenv("ORQ_ROL", "")
	conTerminalFalso(t)
	os.MkdirAll(filepath.Join(root, ".orq"), 0o755)
	os.WriteFile(filepath.Join(root, ".orq", "memoria.jsonl"), []byte(memRota), 0o644)
	os.WriteFile(filepath.Join(root, ".orq", "proyecto.json"), []byte(`{"modo":"mejora"}`), 0o644)

	out, errs, _ := run(t, "contexto")
	if !strings.Contains(out, "el total incluye envío") || !strings.Contains(out, "pago con tarjeta") {
		t.Fatalf("contexto must keep the readable memory: %q %q", out, errs)
	}
	if !strings.Contains(out+errs, "memoria.jsonl:2") {
		t.Fatalf("contexto must name the bad line: %q %q", out, errs)
	}
	out, errs, _ = run(t, "mem", "ver")
	if !strings.Contains(out, "pago con tarjeta") || !strings.Contains(errs, "memoria.jsonl:2") {
		t.Fatalf("mem ver: %q %q", out, errs)
	}
	out, _, _ = run(t, "doctor")
	if !strings.Contains(out, "memoria.jsonl:2") {
		t.Fatalf("doctor must count the bad line as a problem: %q", out)
	}
	// Compaction rewrites the file: it refuses rather than drop the line nobody fixed.
	if _, errs, code := run(t, "mem", "compactar"); code == 0 || !strings.Contains(errs, "memoria.jsonl:2") {
		t.Fatalf("compactar with a bad line: %d %q", code, errs)
	}
	if _, _, code := run(t, "mem", "add", "--imp", "media", "--cat", "dominio", "--fuente", "atena", "talla única"); code != 0 {
		t.Fatal("mem add must still work with a bad line on disk")
	}
	if raw, _ := os.ReadFile(filepath.Join(root, ".orq", "memoria.jsonl")); !strings.Contains(string(raw), `"cat":"deci`+"\n") {
		t.Fatalf("the bad line must stay on disk: %s", raw)
	}
}
