package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const resultadoStub = `{"schemaVersion":1,"claudeVersion":"2.1.328","costUsd":0.41,"partial":false,"aggregates":{"overallScore":0.75},"cases":[{"name":"c1","aggregates":{"score":1.0},"arms":{"with":[{},{},{}]}},{"name":"c2","aggregates":{"score":0.5},"arms":{"with":[{},{},{}]}}]}`

// claudeDeMentira pone en PATH un claude que deja un rastro y, si el plugin tiene forma, escribe json.
func claudeDeMentira(t *testing.T, json string) (rastro string) {
	t.Helper()
	bin := t.TempDir()
	rastro = filepath.Join(bin, "llamado")
	script := "#!/bin/sh\ntouch " + rastro + "\n" +
		`[ "$1 $2" = "plugin eval" ] && [ -f "$3/.claude-plugin/plugin.json" ] || { echo "args: $*" >&2; exit 9; }` + "\n"
	if json != "" {
		script += "cat > \"$5\" <<'JSON'\n" + json + "\nJSON\n"
	} else {
		script += "echo 'Error: plugin eval is currently in early access' >&2; exit 1\n"
	}
	os.WriteFile(filepath.Join(bin, "claude"), []byte(script), 0o755)
	t.Setenv("PATH", bin+":/usr/bin:/bin")
	t.Setenv("ORQ_STATE", t.TempDir())
	return rastro
}

func skillDePrueba(t *testing.T, cuerpo string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "mi-skill")
	os.MkdirAll(filepath.Join(dir, "evals", "c1"), 0o755)
	os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(cuerpo), 0o644)
	os.WriteFile(filepath.Join(dir, "evals", "c1", "prompt.md"), []byte("hola"), 0o644)
	return dir
}

func TestSkillEvalExigeMaxUSD(t *testing.T) {
	rastro := claudeDeMentira(t, resultadoStub)
	dir := skillDePrueba(t, "x")
	for _, args := range [][]string{{"skill", "eval", dir}, {"skill", "eval", dir, "--max-usd", "0"}} {
		_, errs, code := run(t, args...)
		if code != 1 || !strings.Contains(errs, "--max-usd es obligatorio: cada corrida gasta dinero real") {
			t.Fatalf("%v: %d %q", args, code, errs)
		}
	}
	if _, err := os.Stat(rastro); err == nil {
		t.Fatal("claude se llamó sin --max-usd")
	}
}

func TestSkillEvalGuardaYNotasLista(t *testing.T) {
	claudeDeMentira(t, resultadoStub)
	dir := skillDePrueba(t, "v1")
	out, errs, code := run(t, "skill", "eval", dir, "--tipo", "go", "--max-usd", "2")
	if code != 0 || !strings.Contains(out, "tipo go: nota 0.75 en 6 corridas · USD 0.41 · claude 2.1.328") || !strings.Contains(out, "c2 0.50") {
		t.Fatalf("%d %q %q", code, out, errs)
	}
	os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("v2"), 0o644)
	if _, errs, code := run(t, "skill", "eval", dir, "--tipo", "go", "--max-usd", "2"); code != 0 {
		t.Fatal(errs)
	}
	out, _, code = run(t, "skill", "notas")
	lineas := strings.Split(strings.TrimSpace(out), "\n")
	if code != 0 || len(lineas) != 2 || !strings.HasPrefix(lineas[0], "mi-skill") || !strings.Contains(lineas[0], " go ") || !strings.Contains(lineas[0], "0.75") {
		t.Fatalf("%q", out)
	}
	h1, h2 := strings.Fields(lineas[0])[1], strings.Fields(lineas[1])[1]
	if h1 == h2 {
		t.Fatalf("la skill editada tiene otro hash: %q", out)
	}
	if out, _, _ := run(t, "skill", "notas", "--tipo", "web"); !strings.Contains(out, "sin notas todavía") {
		t.Fatalf("%q", out)
	}
}

func TestSkillEvalNoDisponible(t *testing.T) {
	claudeDeMentira(t, "")
	_, errs, code := run(t, "skill", "eval", skillDePrueba(t, "x"), "--max-usd", "1")
	if code != 3 || !strings.Contains(errs, "acceso anticipado") || !strings.Contains(errs, "early access") {
		t.Fatalf("%d %q", code, errs)
	}
	if out, _, _ := run(t, "skill", "notas"); !strings.Contains(out, "sin notas todavía") {
		t.Fatalf("no se guarda nada: %q", out)
	}
}
