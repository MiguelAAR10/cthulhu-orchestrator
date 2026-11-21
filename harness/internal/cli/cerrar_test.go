package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

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
