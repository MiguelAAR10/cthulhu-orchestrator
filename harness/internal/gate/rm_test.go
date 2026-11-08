package gate

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWorkerRmRecursivoFueraDelWorktree(t *testing.T) {
	repo, _, env := escenario(t)
	sub := filepath.Join(repo, "src")
	os.Symlink("/etc", filepath.Join(repo, "afuera"))
	malos := []string{
		`rm -rf /`, `rm -fr /`, `rm -r /`, `rm -Rf /`, `rm --recursive /`, `rm --recursive --force /`, `rm --recur /`,
		`rm -r -f /`, `rm -f -r /`, `rm -rf -- /`, `rm -rf --no-preserve-root /`, `rm -v -rf /`, `/bin/rm -rf /`, `sudo rm -rf /`,
		`\rm -rf /`, `r''m -rf /`, `rm -rf /*`, `rm -rf ~`, `rm -rf ~/`, `rm -rf ~/x`, `rm -rf ~root`, `rm -rf $HOME`, `rm -rf "$HOME"`, `rm -rf ${HOME}/x`,
		`rm -rf ..`, `rm -rf ../`, `rm -rf ../otro`, `rm -rf src/../../otro`, `rm -rf ../*`, `rm -rf /tmp/x`,
		`rm -rf .`, `rm -rf ./`, `rm -rf afuera/x`, `rm -rf src build /etc`,
		`cd / && rm -rf *`, `cd .. ; rm -rf repo`, `cd ~; rm -rf x`, `cd $X && rm -rf y`, `cd src && rm -rf ../..`,
		`rm -rf $X`, `rm -rf $(pwd)/..`, "rm -rf `pwd`", `rm -rf {/,x}`, `sh -c 'rm -rf /'`, `bash -c "cd /; rm -rf *"`,
		`echo / | xargs rm -rf`, `find / -name x -exec rm -rf {} +`, `find .. -exec rm -r {} \;`,
		`true; rm -rf /`, `x=1 rm -rf /`,
	}
	for _, c := range malos {
		if d := herr(repo, "Bash", map[string]any{"command": c}, env); d.Allow {
			t.Errorf("%q pasó", c)
		}
	}
	buenos := []string{
		`rm -rf build`, `rm -rf build dist node_modules`, `rm -rf ./build/`, `rm -r src/old`, `rm -rf src/*`, `cd src && rm -rf *`, `rm -rf $PWD/build`,
		`cd src && rm -rf gen`, `cd src && rm -rf ../build`, `find . -name '*.pyc' -exec rm -rf {} +`, `find build -exec rm -r {} \;`,
		`rm -f a.txt`, `rm a.txt`, `rm -f /tmp/fuera.txt`, `git rm -r src/old`, `echo done`,
	}
	for _, c := range buenos {
		if d := herr(repo, "Bash", map[string]any{"command": c}, env); !d.Allow {
			t.Errorf("%q negado: %s", c, d.Reason)
		}
	}
	// Desde un subdirectorio la raíz sigue siendo el worktree, no el cwd.
	if d := herr(sub, "Bash", map[string]any{"command": `rm -rf ../build`}, env); !d.Allow {
		t.Errorf("subdir: %s", d.Reason)
	}
	if d := herr(sub, "Bash", map[string]any{"command": `rm -rf ../../x`}, env); d.Allow {
		t.Error("subdir: escapó con ../..")
	}
	// Quien no es worker no pasa por esta regla.
	if d := herr(repo, "Bash", map[string]any{"command": `rm -rf /tmp/x`}, fakes{}.env(t, t.TempDir())); !d.Allow {
		t.Errorf("sesión negada: %s", d.Reason)
	}
}
