package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"orq/internal/canal"
)

// sliceVerde opens slice 01 on the toy repo, turns its exam green in the worker copy and verifies it.
func sliceVerde(t *testing.T) {
	t.Helper()
	root := repoCLI(t)
	t.Setenv("ORQ_STATE", t.TempDir())
	if _, errs, code := run(t, append(abrirBase, "--reporte", "out.tap", "--formato", "tap")...); code != 0 {
		t.Fatalf("abrir: %d %s", code, errs)
	}
	run(t, "slice", "tomar", "w1")
	wt := filepath.Join(filepath.Dir(root), "wt", "tienda-s01")
	if err := os.WriteFile(filepath.Join(wt, "total.txt"), []byte("170\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, c := range [][]string{{"add", "-A"}, {"commit", "-qm", "verde"}} {
		cmd := exec.Command("git", c...)
		cmd.Dir = wt
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", c, out)
		}
	}
	if out, errs, code := run(t, "slice", "verificar"); code != 0 {
		t.Fatalf("verificar: %d %s %s", code, out, errs)
	}
}

// pathSinClaude returns PATH without any directory that holds a `claude` executable.
func pathSinClaude() string {
	var dirs []string
	for _, d := range filepath.SplitList(os.Getenv("PATH")) {
		if _, err := os.Stat(filepath.Join(d, "claude")); err != nil {
			dirs = append(dirs, d)
		}
	}
	return strings.Join(dirs, string(os.PathListSeparator))
}

func TestCerrarSinDocNoLlamaAlEscriba(t *testing.T) {
	t.Run("ORQ_SIN_DOC set: the scribe is not called", func(t *testing.T) {
		sliceVerde(t)
		bin, centinela := t.TempDir(), filepath.Join(t.TempDir(), "claude-was-called")
		guion := "#!/bin/sh\necho called > " + centinela + "\nexit 1\n"
		if err := os.WriteFile(filepath.Join(bin, "claude"), []byte(guion), 0o755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", bin+string(os.PathListSeparator)+pathSinClaude())
		t.Setenv("ORQ_SIN_DOC", "1")
		out, errs, code := run(t, "slice", "cerrar", "--aceptado")
		if code != 0 {
			t.Fatalf("cerrar: %d %s %s", code, out, errs)
		}
		if _, err := os.Stat(centinela); err == nil {
			t.Fatal("cerrar called claude although ORQ_SIN_DOC is set")
		}
		if !strings.Contains(out, "project card not updated (ORQ_SIN_DOC set)") {
			t.Fatalf("cerrar must say the card was skipped: %s", out)
		}
	})
	t.Run("no claude on PATH: closes with a warning", func(t *testing.T) {
		sliceVerde(t)
		t.Setenv("PATH", pathSinClaude())
		t.Setenv("ORQ_SIN_DOC", "")
		out, errs, code := run(t, "slice", "cerrar", "--aceptado")
		if code != 0 {
			t.Fatalf("cerrar: %d %s %s", code, out, errs)
		}
		if !strings.Contains(out, "aviso: la ficha del proyecto no se actualizó") || !strings.Contains(out, "cerrado como") {
			t.Fatalf("cerrar must warn and still close: %s", out)
		}
	})
}

// H2: closing a story archives its channel instead of deleting it, and the next story's
// message ids continue after the archived ones.
func TestCerrarArchivaElCanalYLosIdsSiguen(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq2")
	sliceVerde(t)
	root := os.Getenv("ORQ_ROOT")
	t.Setenv("ORQ_SIN_DOC", "1")
	out, errs, code := run(t, "msg", "send", "--de", "atena", "--para", "orquestador", "--tipo", "historia", "quiero el total")
	if code != 0 {
		t.Fatalf("send: %d %s %s", code, out, errs)
	}
	for _, m := range mustLoad(t, filepath.Join(root, ".orq", "canal.jsonl")) {
		canal.MarcarEntregado(filepath.Join(root, ".orq", "canal.jsonl"), m.ID, time.Now())
	}
	if out, errs, code := run(t, "slice", "cerrar", "--aceptado"); code != 0 {
		t.Fatalf("cerrar: %d %s %s", code, out, errs)
	}
	arch := filepath.Join(root, ".orq", "canales", "S-01.jsonl")
	raw, err := os.ReadFile(arch)
	if err != nil || !strings.Contains(string(raw), "quiero el total") {
		t.Fatalf("closing must archive the channel at %s: %v %q", arch, err, raw)
	}
	viejos, _ := canal.Load(arch)
	out, errs, code = run(t, "msg", "send", "--de", "atena", "--para", "orquestador", "--tipo", "historia", "otra historia")
	if code != 0 {
		t.Fatalf("send: %d %s", code, errs)
	}
	want := fmt.Sprintf("c%d", len(viejos)+1)
	if !strings.Contains(out, want) {
		t.Fatalf("first id after the close must be %s (ids never restart): %s", want, out)
	}
}

// M2: closing S-01 leaves another story's messages and every undelivered message in the live channel.
func TestCerrarArchivaSoloSuHistoria(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq2")
	sliceVerde(t)
	root := os.Getenv("ORQ_ROOT")
	t.Setenv("ORQ_SIN_DOC", "1")
	path := filepath.Join(root, ".orq", "canal.jsonl")
	add := func(attempt, texto string, entregado bool) {
		m := &canal.Msg{De: "atena", Para: "orquestador", Tipo: "estado", Texto: texto, Attempt: attempt}
		if err := canal.Append(path, m); err != nil {
			t.Fatal(err)
		}
		if entregado {
			canal.MarcarEntregado(path, m.ID, time.Now())
		}
	}
	add("S-01/a1", "de S-01 entregado", true)
	add("S-01/a1", "de S-01 sin entregar", false)
	add("S-02/a1", "de S-02", true)
	if out, errs, code := run(t, "slice", "cerrar", "--aceptado"); code != 0 {
		t.Fatalf("cerrar: %d %s %s", code, out, errs)
	}
	arch, _ := os.ReadFile(filepath.Join(root, ".orq", "canales", "S-01.jsonl"))
	vivo, _ := os.ReadFile(path)
	if !strings.Contains(string(arch), "de S-01 entregado") || strings.Contains(string(arch), "S-02") || strings.Contains(string(arch), "sin entregar") {
		t.Fatalf("archive must hold only S-01's delivered messages:\n%s", arch)
	}
	if !strings.Contains(string(vivo), "de S-02") || !strings.Contains(string(vivo), "de S-01 sin entregar") {
		t.Fatalf("live channel must keep S-02 and undelivered messages:\n%s", vivo)
	}
}

func mustLoad(t *testing.T, path string) []canal.Msg {
	t.Helper()
	ms, err := canal.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return ms
}

// N4: if the open stories cannot be listed at close, untagged messages stay live (fail safe).
func TestCerrarSinListaDeAbiertosNoArchivaSinEtiqueta(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq2")
	sliceVerde(t)
	root := os.Getenv("ORQ_ROOT")
	t.Setenv("ORQ_SIN_DOC", "1")
	path := filepath.Join(root, ".orq", "canal.jsonl")
	m := &canal.Msg{De: "atena", Para: "orquestador", Tipo: "estado", Texto: "sin etiqueta"}
	if err := canal.Append(path, m); err != nil {
		t.Fatal(err)
	}
	canal.MarcarEntregado(path, m.ID, time.Now())
	// Another story's state that cannot be read: the list of open stories fails.
	if err := os.WriteFile(filepath.Join(root, ".orq", "slices", "02.json"), []byte("{roto"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, errs, _ := run(t, "slice", "--slice", "01", "cerrar", "--aceptado")
	if !strings.Contains(out, "cerrado como") {
		t.Fatalf("the close itself must run: %s %s", out, errs)
	}
	if vivo, _ := os.ReadFile(path); !strings.Contains(string(vivo), "sin etiqueta") {
		t.Fatalf("an untagged message left the live channel although open stories were unknown: %s %s\n%s", out, errs, vivo)
	}
}

// G3: a close whose memory line was lost (the process died, or the memory could not be written)
// gets it on the re-run of an already closed slice, once; nothing else is repeated.
func TestCerrarRepetidoCompletaLaLineaDeMemoriaUnaVez(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq2")
	sliceVerde(t)
	root := os.Getenv("ORQ_ROOT")
	t.Setenv("ORQ_SIN_DOC", "1")
	mem := filepath.Join(root, ".orq", "memoria.jsonl")
	if err := os.MkdirAll(mem, 0o755); err != nil { // the memory cannot be written
		t.Fatal(err)
	}
	if out, errs, code := run(t, "slice", "--slice", "01", "cerrar", "--aceptado"); code != 1 || !strings.Contains(errs, "memoria") {
		t.Fatalf("the first close must report the lost memory line: %d %s %s", code, out, errs)
	}
	os.RemoveAll(mem)
	contar := func() int {
		raw, _ := os.ReadFile(mem)
		return strings.Count(string(raw), "S-01 aceptado")
	}
	for i := 0; i < 2; i++ {
		out, errs, code := run(t, "slice", "--slice", "01", "cerrar", "--aceptado")
		if code != 0 || !strings.Contains(out, "ya estaba cerrado como aceptado") {
			t.Fatalf("re-run %d: %d %s %s", i, code, out, errs)
		}
		if n := contar(); n != 1 {
			t.Fatalf("re-run %d: one memory line expected, got %d", i, n)
		}
	}
}

// orq keeps 1.0.0: a re-run of a closed slice repeats nothing, the memory line included.
func TestCerrarRepetidoNoTocaLaMemoriaComo100(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq")
	sliceVerde(t)
	root := os.Getenv("ORQ_ROOT")
	t.Setenv("ORQ_SIN_DOC", "1")
	if _, errs, code := run(t, "slice", "--slice", "01", "cerrar", "--aceptado"); code != 0 {
		t.Fatalf("cerrar: %d %s", code, errs)
	}
	mem := filepath.Join(root, ".orq", "memoria.jsonl")
	os.Remove(mem)
	out, _, code := run(t, "slice", "--slice", "01", "cerrar", "--aceptado")
	if code != 0 || out != "S-01 ya estaba cerrado como aceptado; no se repite nada\n" {
		t.Fatalf("%d %q", code, out)
	}
	if _, err := os.Stat(mem); !os.IsNotExist(err) {
		t.Fatalf("orq wrote memory on a re-run: %v", err)
	}
}

// G3 review 2: revoking a pending rejection is the Owner's, from a terminal.
func TestRevocaRechazoEsDelOwner(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq2")
	sliceVerde(t)
	viejo := esTerminal
	esTerminal = func(io.Reader) bool { return false }
	t.Cleanup(func() { esTerminal = viejo })
	_, errs, code := run(t, "slice", "reabrir", "--revoca-rechazo", "otra vuelta")
	if code != 1 || !strings.Contains(errs, "es del Owner") {
		t.Fatalf("a non-terminal must not revoke: %d %s", code, errs)
	}
	t.Setenv("ORQ_ROL", "orquestador")
	esTerminal = func(io.Reader) bool { return true }
	if _, errs, code := run(t, "slice", "reintentar", "x", "--causa", "worker", "--revoca-rechazo"); code != 1 || !strings.Contains(errs, "es del Owner") {
		t.Fatalf("an agent pane must not revoke: %d %s", code, errs)
	}
}
