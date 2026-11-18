package gate

import (
	"strings"
	"testing"
	"time"
)

// TestDecisiones fija cada caso del sondeo del gate (evasiones y falsos positivos)
// con su decisión y el motivo esperado. El Env es inyectado: no toca disco real.
func TestDecisiones(t *testing.T) {
	const (
		R = "/x/repo"            // checkout principal
		W = "/x/wt"              // worktree del worker
		S = "/x/state"           // state.Dir()
		L = "/x/cfg/limits.json" // limits.DefaultPath()
	)
	env := Env{
		DiskFreeGB: func(string) (float64, error) { return 999, nil },
		Workers:    func() (int, error) { return 0, nil },
		Now:        time.Now,
		StateDir:   S,
		IsWorktree: func(cwd string) bool { return dentro(cwd, W) },
		LimitsPath: L,
		MainRoot:   func(string) string { return R },
	}
	bash := func(cmd string) Payload {
		return Payload{ToolName: "Bash", Cwd: W, ToolInput: map[string]any{"command": cmd}}
	}
	tool := func(name, k, v string) Payload {
		return Payload{ToolName: name, Cwd: W, ToolInput: map[string]any{k: v}}
	}
	const (
		protegido = "está protegido"
		literal   = "Escribe la ruta literal"
		comando   = "Escribe el comando literal"
		destruye  = "bloqueado"
		soloOrq   = "un worker no abre"
		lanza     = "un worker no lanza"
	)
	cases := []struct {
		p      Payload
		motivo string // "" = permitido
	}{
		// control
		{bash("echo x > tests/aceptacion/s01.test.mjs"), protegido},
		{bash("git push origin main"), destruye},
		{bash("orq slice cerrar --aceptado"), soloOrq},
		// evasiones del examen
		{bash(`echo x > tests/acep""tacion/s01.test.mjs`), protegido},
		{bash(`d=tests/aceptacion; echo x > $d/s01.test.mjs`), literal},
		{bash("echo tests/aceptacion/s01.test.mjs | xargs rm"), literal},
		{bash(`find tests -name "*.mjs" -delete`), protegido},
		{bash("cp -t tests/aceptacion /tmp/evil.mjs"), protegido},
		{bash("git apply /tmp/p.diff"), "Edit o Write"},
		{bash("patch -p1 < /tmp/p.diff"), "Edit o Write"},
		{bash("python3 /tmp/reescribe.py"), ""}, // un script que no se lee: lo cubre Verificar
		{bash(`node --eval "require(\"fs\").writeFileSync(\"tests/aceptacion/s01.test.mjs\",\"\")"`), "código en línea"},
		// evasiones destructivas y de solo-orquestador
		{bash(`git pu""sh origin main`), destruye},
		{bash("g=git; $g push --force origin main"), comando},
		{bash(`git re""set --hard HEAD~3`), destruye},
		{bash(`r""m -rf .git`), destruye},
		{bash("find . -name .git -exec rm -rf {} +"), destruye},
		{bash(`orq sl""ice cerrar --aceptado`), soloOrq},
		{bash(`o""rq halt --clear`), soloOrq},
		{bash(`cl""aude -p "haz otra cosa"`), lanza},
		{bash("npx @anthropic-ai/claude-code -p hola"), lanza},
		// estado y configuración del harness
		{bash("echo '{}' > " + R + "/.orq/slices/01.json"), protegido},
		{bash("echo '{}' > " + L), protegido},
		{bash("rm " + S + "/HALT"), protegido},
		{tool("Write", "file_path", R+"/.orq/slices/01.json"), protegido},
		{tool("Edit", "file_path", W+"/.claude/settings.json"), protegido},
		{tool("Edit", "file_path", L), protegido},
		{tool("Edit", "file_path", W+"/package.json"), ""}, // los archivos del runner los cuida otra actividad
		// subagentes
		{Payload{ToolName: "Agent", Cwd: W, ToolInput: map[string]any{"prompt": "ayudame"}}, lanza},
		// falsos positivos
		{bash(`grep -rn "git push" docs/`), ""},
		{bash(`echo "no uses rm en worktrees" >> notas.md`), ""},
		{bash(`git commit -m "docs: explica git push --force"`), ""},
		{bash("npm test 2>&1 | tee /tmp/salida.txt"), ""},
		{bash("rm -rf node_modules/.cache/worktrees-plugin"), ""},
		{bash("cat tests/aceptacion/s01.test.mjs > /tmp/copia.txt"), ""},
		// envoltorios y escrituras que el analizador abre
		{bash(`bash -c 'echo x > tests/aceptacion/s01.test.mjs'`), protegido},
		{bash(`sudo -u root env FOO=1 nohup rm tests/aceptacion/s01.test.mjs`), protegido},
		{bash(`eval "git push origin main"`), destruye},
		{bash(`echo $(git push origin main)`), destruye},
		{bash("mv -t tests/aceptacion /tmp/evil.mjs"), protegido},
		{bash("install -t tests/aceptacion /tmp/evil.mjs"), protegido},
		{bash("git stash pop"), "Edit o Write"},
		{bash("python3 - <<'EOF'\nopen('tests/aceptacion/s01.test.mjs','w')\nEOF"), "código en línea"},
		{bash("sh <<EOF\nrm -rf .git\nEOF"), destruye},
		{bash(`echo "echo x > tests/aceptacion/a" | sh`), comando},
		{bash("(cd /tmp) && echo x > tests/aceptacion/s01.test.mjs"), protegido},
		{bash("cd $HOME && echo x > notas.md"), literal},
		{bash("rm -rf tests"), protegido},
		{bash("echo ok > src/app.js && git status"), ""},
		{bash(`echo 'git push' | grep push`), ""},
		{bash("find src -name '*.tmp' -delete"), ""},
		{bash("ruby -Itest tests/aceptacion/s01_test.rb"), ""},
		{bash("mv /tmp/generado.js ."), ""},
		{bash("mv tests /tmp/fuera"), protegido},
		{bash("perl -pi -e 's/a/b/' tests/aceptacion/s01.test.mjs"), protegido},
		{bash(`perl -e'unlink "tests/aceptacion/s01.test.mjs"'`), "código en línea"},
		// MCP desde el worktree
		{Payload{ToolName: "mcp__fs__write_file", Cwd: W, ToolInput: map[string]any{"path": "tests/aceptacion/s01.test.mjs", "content": "x"}}, protegido},
		{Payload{ToolName: "mcp__fs__write_file", Cwd: W, ToolInput: map[string]any{"args": []any{map[string]any{"p": L}}}}, protegido},
		{Payload{ToolName: "mcp__fs__write_file", Cwd: W, ToolInput: map[string]any{"path": "src/a.js", "content": "x"}}, ""},
		// un subagente desde el checkout principal tampoco toca el harness
		{Payload{ToolName: "Edit", Cwd: R, AgentID: "a1", ToolInput: map[string]any{"file_path": L}}, protegido},
		{Payload{ToolName: "Bash", Cwd: R, AgentID: "a1", ToolInput: map[string]any{"command": "orq msg send --para atena hola"}}, ""},
		// exam, scope, rehash, reopen, lane, verify and convert belong to the orchestrator
		{bash("orq slice examen tests/s01.sh --aprobado \"req ATN-1\""), soloOrq},
		{bash("cd /main && orq slice examen tests/s01.sh --aprobado x"), soloOrq},
		{bash("orq slice alcance src/ --aprobado x"), soloOrq},
		{bash("orq slice rehuella"), soloOrq},
		{bash("orq slice reabrir 01"), soloOrq},
		{bash("orq slice verificar"), soloOrq},
		{bash("orq slice ruta slice \"x\""), soloOrq},
		{bash("orq slice convertir 01"), soloOrq},
		{Payload{ToolName: "Bash", Cwd: R, ToolInput: map[string]any{"command": "orq slice examen tests/s01.sh --aprobado \"req ATN-1\""}}, ""},
		{Payload{ToolName: "Bash", Cwd: R, ToolInput: map[string]any{"command": "cd /main && orq slice examen tests/s01.sh --aprobado x"}}, ""},
		{Payload{ToolName: "Bash", Cwd: R, ToolInput: map[string]any{"command": "orq slice alcance src/ --aprobado x"}}, ""},
		{Payload{ToolName: "Bash", Cwd: R, ToolInput: map[string]any{"command": "orq slice rehuella"}}, ""},
		{Payload{ToolName: "Bash", Cwd: R, ToolInput: map[string]any{"command": "orq slice reabrir 01"}}, ""},
		{Payload{ToolName: "Bash", Cwd: R, ToolInput: map[string]any{"command": "orq slice verificar"}}, ""},
		{Payload{ToolName: "Bash", Cwd: R, ToolInput: map[string]any{"command": "orq slice ruta slice \"x\""}}, ""},
		{Payload{ToolName: "Bash", Cwd: R, ToolInput: map[string]any{"command": "orq slice convertir 01"}}, ""},
		{bash(`orq halt --set "x"`), ""}, // pausing destroys nothing
		// fuera de un worktree no cambia nada
		{Payload{ToolName: "Bash", Cwd: R, ToolInput: map[string]any{"command": "g=git; $g status; echo x > $d/a"}}, ""},
		{Payload{ToolName: "Edit", Cwd: R, ToolInput: map[string]any{"file_path": L}}, ""},
	}
	for _, c := range cases {
		desc := c.p.ToolName + " " + func() string {
			if s, ok := c.p.ToolInput["command"].(string); ok {
				return s
			}
			return ""
		}()
		d := Evaluate(c.p, baseLimits(), env)
		switch {
		case c.motivo == "" && !d.Allow:
			t.Errorf("%s: negó y debía permitir (%s)", desc, d.Reason)
		case c.motivo != "" && d.Allow:
			t.Errorf("%s: permitió y debía negar por %q", desc, c.motivo)
		case c.motivo != "" && !strings.Contains(d.Reason, c.motivo):
			t.Errorf("%s: motivo %q no contiene %q", desc, d.Reason, c.motivo)
		}
	}
}
