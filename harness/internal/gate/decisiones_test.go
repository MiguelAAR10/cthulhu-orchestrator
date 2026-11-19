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
		RepoPropio: func(cwd string) (string, string) {
			if dentro(cwd, W) {
				return W, "slice/01"
			}
			return "", ""
		},
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
		propio    = "only in its own repository"
		rama      = "only its own branch"
		config    = "sets no git config"
		alias     = "is not a git builtin"
		puntoGit  = "no .git"
		oculto    = "runs a command the gate does not read"
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
		{Payload{ToolName: "Bash", Cwd: R, ToolInput: map[string]any{"command": "orq slice examen tests/s01.sh --aprobado \"req ATN-1\""}}, ""},
		{Payload{ToolName: "Bash", Cwd: R, ToolInput: map[string]any{"command": "cd /main && orq slice examen tests/s01.sh --aprobado x"}}, ""},
		{Payload{ToolName: "Bash", Cwd: R, ToolInput: map[string]any{"command": "orq slice alcance src/ --aprobado x"}}, ""},
		{Payload{ToolName: "Bash", Cwd: R, ToolInput: map[string]any{"command": "orq slice rehuella"}}, ""},
		{Payload{ToolName: "Bash", Cwd: R, ToolInput: map[string]any{"command": "orq slice reabrir 01"}}, ""},
		{Payload{ToolName: "Bash", Cwd: R, ToolInput: map[string]any{"command": "orq slice verificar"}}, ""},
		{Payload{ToolName: "Bash", Cwd: R, ToolInput: map[string]any{"command": "orq slice ruta slice \"x\""}}, ""},
		{bash(`orq halt --set "x"`), ""}, // pausing destroys nothing
		// a worker runs git only against its own repository (review 3, finding B)
		{bash("git -C /x/repo status"), propio},
		{bash("git -C ../repo commit -am x"), propio},
		{bash("git -C src -C ../../repo log"), propio},
		{bash("git --git-dir=/x/repo/.git log"), propio},
		{bash("git --git-dir /x/repo/.git log"), propio},
		{bash("git --work-tree=/x/repo checkout main -- ."), propio},
		{bash("git --work-tree /x/repo status"), propio},
		{bash("git -c core.worktree=/x/repo status"), config},
		{bash("git -c include.path=/tmp/c status"), config},
		{bash("git --config-env=core.worktree=WT status"), config},
		{bash("git config core.worktree /x/repo"), config},
		{bash("GIT_DIR=/x/repo/.git git log"), propio},
		{bash("env GIT_DIR=/x/repo/.git git log"), propio},
		{bash("export GIT_DIR=/x/repo/.git"), propio},
		{bash("GIT_WORK_TREE=/x/repo git status"), propio},
		{bash("GIT_COMMON_DIR=/x/repo/.git git log"), propio},
		{bash("GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=core.worktree GIT_CONFIG_VALUE_0=/x/repo git status"), config},
		{bash("GIT_DIR=$D git log"), propio},
		{bash("cd /x/repo && git commit -am x"), propio},
		{bash("(cd /x/repo; git log)"), propio},
		{bash("cd $D && git status"), propio},
		{bash("git -C $D status"), propio},
		{bash(`bash -c "git -C /x/repo log"`), propio},
		{bash("echo x > .git/commondir"), protegido},
		{tool("Write", "file_path", W+"/.git/config"), protegido},
		{bash("git -C src status"), ""},
		{bash("git -C /x/wt/sub log"), ""},
		{bash("cd src && git diff"), ""},
		{bash("git --git-dir=.git log"), ""},
		{bash("GIT_DIR=.git git log"), ""},
		{bash(`grep -rn "git -C /x/repo" docs/`), ""},
		{Payload{ToolName: "Bash", Cwd: R, ToolInput: map[string]any{"command": "git -C /x/other log"}}, ""},
		// ... and moves only refs under its own slice/NN branch
		{bash("git update-ref refs/heads/main abc"), rama},
		{bash("git update-ref -m x refs/heads/slice/02 abc"), rama},
		{bash("git update-ref refs/heads/slice/01-x abc"), rama},
		{bash("git update-ref -d refs/heads/main"), rama},
		{bash("git update-ref HEAD abc"), rama},
		{bash("git update-ref --stdin <<< 'update refs/heads/main abc'"), rama},
		{bash("git update-ref $R abc"), rama},
		{bash("git branch -f main abc"), rama},
		{bash("git branch --force main abc"), rama},
		{bash("git branch -fq main abc"), rama},
		{bash("git branch -M main"), rama},
		{bash("git branch -M slice/01 main"), rama},
		{bash("git branch -C slice/01 main"), rama},
		{bash("git branch -M main slice/01"), rama},
		{bash("git symbolic-ref HEAD refs/heads/main"), rama},
		{bash("git symbolic-ref -d HEAD"), rama},
		{bash("git update-ref refs/heads/slice/01 abc"), ""},
		{bash("git update-ref refs/heads/slice/01/tmp abc"), ""},
		{bash("git branch -f slice/01 abc"), ""},
		{bash("git branch -M slice/01/a slice/01/b"), ""},
		{bash("git branch tmp abc"), ""},
		{bash("git symbolic-ref --short HEAD"), ""},
		{Payload{ToolName: "Bash", Cwd: R, ToolInput: map[string]any{"command": "git update-ref refs/heads/main abc"}}, ""},
		// review 4: podar is orchestrator-only
		{bash("orq slice podar w-front"), soloOrq},
		{bash("orq --slice 01 slice podar w-front"), soloOrq},
		// review 4, finding 1: a worker creates, writes, moves or links no .git inside its clone
		{bash(`mkdir -p sub && printf 'gitdir: /x/repo/.git\n' > sub/.git`), puntoGit},
		{bash("mkdir -p sub/.git/refs"), puntoGit},
		{bash("ln -s /x/repo/.git sub/.git"), puntoGit},
		{bash("ln -s /x/repo/.git sub/"), puntoGit},
		{bash("cp -r /x/repo/.git sub/"), puntoGit},
		{bash("mv sub/x sub/.git"), puntoGit},
		{bash("touch src/.git"), puntoGit},
		{tool("Write", "file_path", W+"/sub/.git"), puntoGit},
		{tool("Edit", "file_path", "sub/.git/HEAD"), puntoGit},
		{bash("git --git-dir=sub/.git log"), propio},
		{bash("GIT_DIR=sub git log"), propio},
		{bash("echo x > .git/info/exclude"), ""},
		{bash("echo x > src/.gitignore"), ""},
		// review 4, finding 2: no -c, no git config writes, no aliases, no command-running env
		{bash(`git -c alias.x='!git --git-dir=/x/repo/.git update-ref refs/heads/hacked2 HEAD' x`), config},
		{bash(`git config alias.y '!git --git-dir=/x/repo/.git update-ref refs/heads/hacked3 HEAD'`), config},
		{bash("git y"), alias},
		{bash(`git -c alias.v='!orq slice verificar' v`), config},
		{bash("git -c core.fsmonitor=/tmp/x status"), config},
		{bash("git -c core.pager=/tmp/x log"), config},
		{bash("git -c core.sshCommand=/tmp/x fetch"), config},
		{bash("git -c core.hooksPath=/tmp/h commit -m x"), config},
		{bash("git -c diff.x.textconv=/tmp/x diff"), config},
		{bash("git -c filter.x.clean=/tmp/x add ."), config},
		{bash("git -c sequence.editor=/tmp/x rebase -i HEAD~1"), config},
		{bash("git -c $K status"), config},
		{bash("git --config-env=alias.x=X x"), config},
		{bash("git config --add alias.z x"), config},
		{bash("git config set alias.z x"), config},
		{bash("git config --global core.pager /tmp/x"), config},
		{bash("git config -e"), config},
		{bash("git config --rename-section user alias"), config},
		{bash(`GIT_CONFIG_PARAMETERS="'alias.x'='!sh'" git x`), config},
		{bash("GIT_CONFIG_GLOBAL=/tmp/c git status"), config},
		{bash("export GIT_CONFIG_COUNT=1"), config},
		{bash("GIT_EXTERNAL_DIFF=/tmp/x git diff"), config},
		{bash("GIT_SSH_COMMAND=/tmp/x git fetch"), config},
		{bash("git --exec-path=/tmp/x status"), config},
		{bash("git $S"), alias},
		{bash("git rebase --exec 'git -C /x/repo log' HEAD~1"), oculto},
		{bash("git rebase -x /tmp/x HEAD~1"), oculto},
		{bash("git submodule foreach /tmp/x"), oculto},
		{bash("git bisect run /tmp/x"), oculto},
		{bash("git -c user.name=w -c user.email=w@x commit -m y"), ""},
		{bash("git -c color.ui=never log"), ""},
		{bash("git config user.email w@x"), ""},
		{bash("git config --get alias.x"), ""},
		{bash("git config --list"), ""},
		{bash("git config core.pager"), ""},
		{bash("git config get core.pager"), ""},
		{bash("GIT_PAGER=cat git log"), ""},
		{bash("git --no-pager log --oneline"), ""},
		{bash("git rebase -i HEAD~1"), ""},
		{bash("git submodule update"), ""},
		{Payload{ToolName: "Bash", Cwd: R, ToolInput: map[string]any{"command": "git -c alias.x='!true' x"}}, ""},
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
