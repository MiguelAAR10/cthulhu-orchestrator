package gate

import (
	"testing"
	"time"
)

func rolEnv(t *testing.T, rol string, wt bool) Env {
	now := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	env := fakes{free: map[string]float64{"/": 400, "/mnt/c": 100}, now: now, wt: wt}.env(t, freshState(t, now, 1))
	env.Rol = rol
	env.Bundle = func(string) []string { return []string{"tests/aceptacion/"} }
	return env
}

func TestTesterNoEscribe(t *testing.T) {
	env := rolEnv(t, "tester", false) // aun fuera de un worktree
	niega := []Payload{
		{ToolName: "Edit", Cwd: "/home/x/repo", ToolInput: map[string]any{"file_path": "src/app.go"}},
		{ToolName: "Write", Cwd: "/home/x/repo", ToolInput: map[string]any{"file_path": "/tmp/nota.md"}},
		{ToolName: "NotebookEdit", Cwd: "/home/x/repo", ToolInput: map[string]any{"notebook_path": "a.ipynb"}},
		bash("echo hola > src/app.go"),
		bash("sed -i s/a/b/ src/app.go"),
		bash("rm -r build"),
		bash("git commit -am arreglo"),
		bash("git add ."),
		bash("cd src && tee x.go"),
		bash("python3 -c 'open(\"a\",\"w\")'"),
		bash("mkdir nuevo"),
		bash("orq qa run S-07"),
		bash("orq slice cerrar --aceptado"),
	}
	for _, p := range niega {
		if d := Evaluate(p, baseLimits(), env); d.Allow {
			t.Errorf("tester debe ser de solo lectura: %+v", p.ToolInput)
		}
	}
	permite := []string{"go test ./... 2>&1", "npx playwright test > /dev/null", "cat README.md", "git log --oneline", "git diff HEAD~1",
		"orq qa finding add S-07 --requirement R1 --commit abc --repro 'go test' --observed x --expected y --impact z"}
	for _, c := range permite {
		if d := Evaluate(bash(c), baseLimits(), env); !d.Allow {
			t.Errorf("tester puede leer y probar: %q → %s", c, d.Reason)
		}
	}
	if d := Evaluate(Payload{ToolName: "Read", ToolInput: map[string]any{"file_path": "x"}}, baseLimits(), env); !d.Allow {
		t.Error("leer está permitido")
	}
}

func TestRepairerNoTocaExamen(t *testing.T) {
	env := rolEnv(t, "repairer", false) // protegido aunque no sea un worktree enlazado
	ed := Payload{ToolName: "Edit", Cwd: "/home/x/repo", ToolInput: map[string]any{"file_path": "/home/x/repo/tests/aceptacion/login_test.go"}}
	if d := Evaluate(ed, baseLimits(), env); d.Allow {
		t.Error("repairer no edita el examen fijado por huella")
	}
	prod := Payload{ToolName: "Edit", Cwd: "/home/x/repo", ToolInput: map[string]any{"file_path": "/home/x/repo/src/login.go"}}
	if d := Evaluate(prod, baseLimits(), env); !d.Allow {
		t.Errorf("repairer sí edita el producto: %s", d.Reason)
	}
	if d := Evaluate(bash("orq qa run S-07"), baseLimits(), env); d.Allow {
		t.Error("orq qa run es solo del orquestador")
	}
}

func TestQARunSoloOrquestador(t *testing.T) {
	env := rolEnv(t, "", true)
	if d := Evaluate(bash("orq qa run S-07"), baseLimits(), env); d.Allow {
		t.Error("desde un worktree no se corre orq qa run")
	}
	env = rolEnv(t, "", false)
	if d := Evaluate(bash("orq qa run S-07"), baseLimits(), env); !d.Allow {
		t.Errorf("el orquestador sí: %s", d.Reason)
	}
}
