package gate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"orq/internal/limits"
)

// escenario: XDG_CONFIG_HOME falso con orq/, un repo y un enlace repo/lnk hacia la config.
func escenario(t *testing.T) (repo, cfg string, env Env) {
	t.Helper()
	base, _ := filepath.EvalSymlinks(t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(base, "cfg"))
	t.Setenv("HOME", filepath.Join(base, "home"))
	cfg = filepath.Join(base, "cfg", "orq")
	repo = filepath.Join(base, "home", "repo")
	for _, d := range []string{filepath.Join(cfg, "sessions"), filepath.Join(repo, ".git"), filepath.Join(repo, "src")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	os.WriteFile(filepath.Join(cfg, "owner.token"), []byte("x"), 0o600)
	if err := os.Symlink(cfg, filepath.Join(repo, "lnk")); err != nil {
		t.Skip(err)
	}
	env = fakes{}.env(t, t.TempDir())
	env.WorkerID = "w1"
	return repo, cfg, env
}

func herr(repo, tool string, in map[string]any, env Env) Decision {
	return Evaluate(Payload{ToolName: tool, Cwd: repo, ToolInput: in}, limits.Limits{}, env)
}

// Item 1: la misma regla, con la ruta real, en todas las herramientas de archivos.
func TestWorkerSymlinkAConfigNegadoEnTodasLasHerramientas(t *testing.T) {
	repo, cfg, env := escenario(t)
	os.Symlink(filepath.Join(cfg, "sessions", "nuevo.json"), filepath.Join(repo, "colgante"))
	malos := []struct {
		tool string
		in   map[string]any
	}{
		{"Read", map[string]any{"file_path": "lnk/owner.token"}},
		{"Read", map[string]any{"file_path": filepath.Join(repo, "lnk", "sessions")}},
		{"Write", map[string]any{"file_path": filepath.Join(repo, "lnk", "sessions", "x.json")}},
		{"Write", map[string]any{"file_path": "lnk/a/b/c/nuevo.json"}},
		{"Write", map[string]any{"file_path": "colgante"}},
		{"Edit", map[string]any{"file_path": "lnk/config.json"}},
		{"MultiEdit", map[string]any{"file_path": "lnk/config.json"}},
		{"NotebookEdit", map[string]any{"notebook_path": "lnk/n.ipynb"}},
		{"Grep", map[string]any{"pattern": ".", "path": "lnk"}},
		{"Grep", map[string]any{"pattern": ".", "path": "lnk/sessions"}},
		{"Glob", map[string]any{"pattern": "*", "path": "lnk"}},
		{"Glob", map[string]any{"pattern": "lnk/*"}},
		{"LS", map[string]any{"path": "lnk"}},
	}
	for _, m := range malos {
		if d := herr(repo, m.tool, m.in, env); d.Allow {
			t.Errorf("%s %v pasó", m.tool, m.in)
		}
	}
	buenos := []struct {
		tool string
		in   map[string]any
	}{
		{"Read", map[string]any{"file_path": "src/a.go"}},
		{"Write", map[string]any{"file_path": "src/nuevo/dir/a.go"}},
		{"Grep", map[string]any{"pattern": ".", "path": "src"}},
		{"Glob", map[string]any{"pattern": "**/*.go"}},
		{"LS", map[string]any{"path": "src"}},
	}
	for _, m := range buenos {
		if d := herr(repo, m.tool, m.in, env); !d.Allow {
			t.Errorf("%s %v negado: %s", m.tool, m.in, d.Reason)
		}
	}
	// Quien no es worker sigue sin esta regla.
	sesion := fakes{}.env(t, t.TempDir())
	if d := herr(repo, "Read", map[string]any{"file_path": "lnk/owner.token"}, sesion); !d.Allow {
		t.Errorf("sesión negada: %s", d.Reason)
	}
}

// Un Grep recursivo sobre / o el home alcanza la config aunque no la nombre.
func TestWorkerGrepSobreAncestroDeConfig(t *testing.T) {
	repo, _, env := escenario(t)
	for _, p := range []string{"/", "~", filepath.Dir(filepath.Dir(repo))} {
		if d := herr(repo, "Grep", map[string]any{"pattern": ".", "path": p}, env); d.Allow {
			t.Errorf("Grep path=%s pasó", p)
		}
	}
}

// El mismo enlace por Bash: nombrarlo llega a la config igual que Read.
func TestWorkerBashPorSymlinkAConfig(t *testing.T) {
	repo, _, env := escenario(t)
	for _, c := range []string{`cat lnk/own*`, `cat lnk/*`, `head -c 64 lnk/o*`, `ls lnk/`, `tar cf - lnk`, `grep -r . lnk/`,
		`cp /dev/stdin lnk/sessions/9-9.json`, `cd lnk && cat own*`, `cat l*/own*`, `cd src; cat ../lnk/x`} {
		if d := herr(repo, "Bash", map[string]any{"command": c}, env); d.Allow {
			t.Errorf("%q pasó", c)
		}
	}
	for _, c := range []string{`ls src`, `cat src/*`, `go test ./...`, `echo hi`} {
		if d := herr(repo, "Bash", map[string]any{"command": c}, env); !d.Allow {
			t.Errorf("%q negado: %s", c, d.Reason)
		}
	}
}

// Sin LS en el matcher del hook, el gate nunca lo vería.
func TestHookIncluyeLS(t *testing.T) {
	raw, err := os.ReadFile("../../../templates/claude-settings.json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "Glob|LS") {
		t.Fatal(string(raw))
	}
}

// Item 2: buscar tokens o owner.* por patrón, con los casos exactos de identity.md.
func TestWorkerGlobDeTokensNegado(t *testing.T) {
	repo, _, env := escenario(t)
	malos := []map[string]any{
		{"pattern": "**/*.tok*", "path": "/"},
		{"pattern": "**/*.tok*", "path": "~"},
		{"pattern": "*.token"},
		{"pattern": "**/owner.*"},
		{"pattern": "owner.*"},
		{"pattern": "*tok*"},
		{"pattern": "**/own?r.tok*"},
		{"pattern": "**/own[e]r.*"},
		{"pattern": "**/*t?k*"},
		{"pattern": "**/OWNER.*"},
		{"pattern": "**/o{wn,x}er.*"},
		{"pattern": "**/o*", "path": "/"},
		{"pattern": ".c*/o*/*", "path": "~"},
		{"pattern": "*.go", "path": "tokens"},
		{"pattern": "*", "path": "/somewhere/owner.d/owner.x"},
	}
	for _, in := range malos {
		if d := herr(repo, "Glob", in, env); d.Allow {
			t.Errorf("Glob %v pasó", in)
		}
	}
	for _, in := range []map[string]any{{"pattern": "x", "glob": "**/owner.*"}, {"pattern": "x", "glob": "*.tok*", "path": "src"}} {
		if d := herr(repo, "Grep", in, env); d.Allow {
			t.Errorf("Grep %v pasó", in)
		}
	}
	buenos := []map[string]any{{"pattern": "**/*.go"}, {"pattern": "src/*.ts", "path": repo}, {"pattern": "**/*"}, {"pattern": "**/*.md", "path": "src"}}
	for _, in := range buenos {
		if d := herr(repo, "Glob", in, env); !d.Allow {
			t.Errorf("Glob %v negado: %s", in, d.Reason)
		}
	}
	if d := herr(repo, "Glob", map[string]any{"pattern": "*.token"}, fakes{}.env(t, t.TempDir())); !d.Allow {
		t.Errorf("sesión negada: %s", d.Reason)
	}
}

// Un repo que se llama stock o tokyo no deja al worker sin Glob dentro de su worktree.
func TestWorkerGlobRepoConTokEnElNombre(t *testing.T) {
	_, _, env := escenario(t)
	h, _ := os.UserHomeDir()
	repo := filepath.Join(h, "stock-app")
	os.MkdirAll(filepath.Join(repo, "src"), 0o755)
	for _, in := range []map[string]any{{"pattern": "**/*.go"}, {"pattern": "*.go", "path": filepath.Join(repo, "src")}, {"pattern": filepath.Join(repo, "*.go")}} {
		if d := herr(repo, "Glob", in, env); !d.Allow {
			t.Errorf("Glob %v negado: %s", in, d.Reason)
		}
	}
}

func TestWorkerFindDeTokensNegado(t *testing.T) {
	repo, _, env := escenario(t)
	for _, c := range []string{`find / -name "owner.*"`, `find / -iname 'OWNER.*'`, `find ~ -name 'own?r.*'`, `find . -name "*tok*"`, `find / -path '*/owner.*'`,
		`ls /x/owner.*`, `rg --files -g 'owner.*' /`, `fd 'owner.' /`, `locate owner.token`, `locate owner.`} {
		if d := herr(repo, "Bash", map[string]any{"command": c}, env); d.Allow {
			t.Errorf("%q pasó", c)
		}
	}
	for _, c := range []string{`find . -name "*.go"`, `find src -type f`, `rg --files -g '*.ts'`} {
		if d := herr(repo, "Bash", map[string]any{"command": c}, env); !d.Allow {
			t.Errorf("%q negado: %s", c, d.Reason)
		}
	}
}
