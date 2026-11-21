package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"orq/internal/plan"
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

// orq keeps 1.0.0: plan hecha/soltar/debatido report success even when the save fails.
func TestOrqPlanSinGuardarComo100(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq")
	root := repoCLI(t)
	t.Setenv("ORQ_STATE", t.TempDir())
	if _, errs, code := run(t, append(abrirBase, "--reporte", "out.tap", "--formato", "tap")...); code != 0 {
		t.Fatalf("abrir: %d %s", code, errs)
	}
	dir := filepath.Join(root, ".orq", "planes")
	pl := &plan.Plan{Slice: "01", Tareas: []plan.Tarea{{ID: "T1", Zona: "back", Rutas: []string{"total.txt"}, Estado: "en_curso", Worker: "w1"}}}
	plan.Guardar(filepath.Join(dir, "01.json"), pl)
	os.Chmod(dir, 0o555)
	t.Cleanup(func() { os.Chmod(dir, 0o755) })
	for args, want := range map[string]string{
		"hecha T1":  "T1 hecha (w1). Quedan 0; listas ahora: ninguna\n",
		"soltar T1": "T1 vuelve a pendiente; relánzala con orq slice lanzar --tarea T1\n",
		"debatido":  "plan marcado como debatido\n",
	} {
		out, errs, code := run(t, append([]string{"plan"}, strings.Fields(args)...)...)
		if code != 0 || out != want || errs != "" {
			t.Errorf("plan %s: code %d out %q err %q; want 1.0.0's %q", args, code, out, errs, want)
		}
	}
}

// orq keeps 1.0.0: a close whose memory write fails still exits 0 and says the memory was updated.
func TestOrqCerrarSinMemoriaComo100(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq")
	sliceVerde(t)
	t.Setenv("ORQ_SIN_DOC", "1")
	mem := filepath.Join(os.Getenv("ORQ_ROOT"), ".orq", "memoria.jsonl")
	os.Remove(mem)
	os.Mkdir(mem, 0o755)
	out, errs, code := run(t, "slice", "cerrar", "--aceptado")
	if code != 0 || errs != "" || !strings.Contains(out, "S-01 cerrado como aceptado. Canal limpio; memoria actualizada.") {
		t.Fatalf("cerrar: code %d out %q err %q", code, out, errs)
	}
}

// orq keeps 1.0.0: with one bad memory line, contexto shows no memory and no warning, and
// doctor does not count it.
func TestOrqMemoriaRotaContextoComo100(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq")
	root := t.TempDir()
	t.Setenv("ORQ_ROOT", root)
	t.Setenv("ORQ_ROL", "")
	conTerminalFalso(t)
	os.MkdirAll(filepath.Join(root, ".orq"), 0o755)
	os.WriteFile(filepath.Join(root, ".orq", "memoria.jsonl"), []byte(memRota), 0o644)
	os.WriteFile(filepath.Join(root, ".orq", "proyecto.json"), []byte(`{"modo":"mejora"}`), 0o644)
	out, errs, _ := run(t, "contexto")
	if strings.Contains(out, "## Memoria") || strings.Contains(out+errs, "memoria.jsonl") || strings.Contains(out, "aviso") {
		t.Fatalf("orq contexto must match 1.0.0 (no memory, no warning): %q %q", out, errs)
	}
	if out, _, _ := run(t, "doctor"); strings.Contains(out, "memoria.jsonl") {
		t.Fatalf("orq doctor must not report the bad line: %q", out)
	}
}
