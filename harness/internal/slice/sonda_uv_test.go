package slice

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"orq/internal/sandbox"
)

// cajaGrabadora runs tar for real (copiaDe needs the copy) and records every other call.
type cajaGrabadora struct {
	specs  []sandbox.Spec
	argvs  [][]string
	salida string
}

func (c *cajaGrabadora) Command(ctx context.Context, s sandbox.Spec, argv ...string) (*exec.Cmd, error) {
	if argv[0] == "tar" {
		cmd := exec.CommandContext(ctx, "tar", argv[1:]...)
		cmd.Dir = s.Cwd
		return cmd, nil
	}
	c.specs, c.argvs = append(c.specs, s), append(c.argvs, argv)
	return exec.CommandContext(ctx, "sh", "-c", c.salida), nil
}

func repoUv(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, a := range [][]string{{"init", "-q", "-b", "main"}, {"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--allow-empty", "-m", "i"}} {
		if out, err := exec.Command("git", append([]string{"-C", root}, a...)...).CombinedOutput(); err != nil {
			t.Fatalf("%s", out)
		}
	}
	if err := os.MkdirAll(filepath.Join(root, ".venv", "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestSondarUvArgvYVenv(t *testing.T) {
	root := repoUv(t)
	caja := &cajaGrabadora{salida: "echo 'pytest 8.3.0'"}
	r := Repo{Root: root, Caja: caja}
	cmd := "PYTHONPATH=src uv run --offline --no-sync pytest -q -p no:cacheprovider"
	sonda, linea, ok, err := r.SondarUv(cmd)
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v linea=%q", ok, err, linea)
	}
	want := "PYTHONPATH=src uv run --offline --no-sync pytest --version"
	if sonda != want || linea != "pytest 8.3.0" {
		t.Fatalf("sonda %q linea %q", sonda, linea)
	}
	if len(caja.argvs) != 1 || !slices.Equal(caja.argvs[0], []string{"sh", "-c", want}) {
		t.Fatalf("argv: %q", caja.argvs)
	}
	if !slices.Contains(caja.specs[0].RO, filepath.Join(root, ".venv")) {
		t.Fatalf("the main .venv must be bound read-only: %q", caja.specs[0].RO)
	}
}

func TestSondarUvFalla(t *testing.T) {
	caja := &cajaGrabadora{salida: "echo 'error: Failed to spawn: mypy' ; echo more; exit 2"}
	r := Repo{Root: repoUv(t), Caja: caja}
	sonda, linea, ok, err := r.SondarUv("uv run --offline --no-sync mypy src && echo x")
	if err != nil || ok || linea != "error: Failed to spawn: mypy" || sonda != "uv run --offline --no-sync mypy --version" {
		t.Fatalf("sonda %q ok=%v linea=%q err=%v", sonda, ok, linea, err)
	}
}

func TestSondaUvDe(t *testing.T) {
	for cmd, want := range map[string]string{
		"PYTHONPATH=src uv run --offline --no-sync ruff check src tests && PYTHONPATH=src uv run --offline --no-sync ruff format --check src": "PYTHONPATH=src uv run --offline --no-sync ruff --version",
		"uv run --offline --no-sync python -m pytest":  "uv run --offline --no-sync python --version",
		"echo hi && uv run --no-sync --offline mypy .": "uv run --no-sync --offline mypy --version",
		"uv run --offline": "",
		"pytest -q":        "",
	} {
		if got := SondaUvDe(cmd); got != want {
			t.Errorf("SondaUvDe(%q) = %q, want %q", cmd, got, want)
		}
	}
}

// F5: the value of a uv flag is not the tool.
func TestSondaUvSaltaValorDeFlag(t *testing.T) {
	for cmd, want := range map[string]string{
		"uv run --with foo pytest -q":                   "uv run --with foo pytest --version",
		"uv run --directory sub --no-sync pytest":       "uv run --directory sub --no-sync pytest --version",
		"uv run -p 3.12 --offline --no-sync mypy src":   "uv run -p 3.12 --offline --no-sync mypy --version",
		"uv run --python=3.12 --no-sync ruff check .":   "uv run --python=3.12 --no-sync ruff --version",
		"uv run --no-sync -- pytest":                    "uv run --no-sync -- pytest --version",
		"uv run --group dev --extra x --no-sync pytest": "uv run --group dev --extra x --no-sync pytest --version",
		"uv run --no-sync -p":                           "",
		"uv run --offline --no-sync --with":             "",
	} {
		if got := SondaUvDe(cmd); got != want {
			t.Errorf("%q: got %q want %q", cmd, got, want)
		}
	}
	if got := HerramientasUv("uv run --with foo pytest && uv run --directory sub ruff check ."); !slices.Equal(got, []string{"pytest", "ruff"}) {
		t.Errorf("herramientas: %q", got)
	}
}

// F6: a host tool named again in a plain segment keeps its host lookup.
func TestHerramientasUvSoloTramosUv(t *testing.T) {
	if got := HerramientasUv("uv run --no-sync pytest && pytest"); len(got) != 0 {
		t.Errorf("pytest also runs from the host: %q", got)
	}
	uv, resto := TramosUv("uv run --offline --no-sync ruff check . && uv run ruff format --check . ; echo ok")
	if len(uv) != 2 || len(resto) != 1 {
		t.Errorf("uv %q resto %q", uv, resto)
	}
}
