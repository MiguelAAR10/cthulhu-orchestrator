package gate

import (
	"os"
	"testing"
)

// Evaluador B: cada vía conocida para esquivar la lista negra queda negada a un worker.
func TestWorkerAllowlistDeOrq(t *testing.T) {
	wt := fakes{wt: true}.env(t, t.TempDir())
	porID := fakes{}.env(t, t.TempDir()) // checkout principal, pero el agente se lanzó como worker
	porID.WorkerID = "w1"
	porRol := fakes{}.env(t, t.TempDir())
	porRol.Rol = "w1"
	malos := []string{
		`orq sl''ice exam tests/easy.sh --approved m2`,
		`orq sl""ice ex""am tests/easy.sh`,
		`a=sli; orq ${a}ce exam t`,
		`orq --slice=01 slice exam t`,
		`orq --slice 01 slice exam t`,
		`orq -C /home/x/repo slice exam t`,
		`alias q=orq; q slice exam t`,
		`ln -s /home/u/bin/orq /tmp/q && /tmp/q slice exam t`,
		`cp $(which orq) /tmp/q`,
		`install -m755 /home/u/bin/orq /tmp/q`,
		`git -c alias.z='!orq slice exam t' z`,
		`git config alias.z '!orq slice exam t'`,
		`ORQ_WORKER_ID= ORQ_ROL=owner orq mem add --source owner --slice S-01 x`,
		`export ORQ_ROL=owner`,
		`unset ORQ_WORKER_ID; orq autonomy free --scope src`,
		`env -u ORQ_WORKER_ID orq stop resolve ST-0001 --choice a`,
		`env -i PATH=/usr/bin orq slice status`,
		`orq mem add --source owner --slice S-01 x`,
		`orq mem add --fuente=atena x`,
		`orq msg send --to orchestrator --type question --from atena x`,
		`orq msg send --para atena --tipo cierre listo`,
		`orq autonomy free --scope src`,
		`orq stop resolve ST-0001 --choice a`,
		`orq stop cancel ST-0001 --note x`,
		`orq slice close --accepted`,
		`orq slice scope src --approved m1`,
		`orq token`,
		`cat ~/.config/orq/owner.token`,
		`grep . $XDG_CONFIG_HOME/orq/owner.token`,
		`cat /proc/1234/environ`,
		`orq plan load plan.json`,
		`orq session open planner`,
		`orq init`,
		`orq up`,
		`find . -name x -exec orq slice exam t \;`,
	}
	for _, env := range []Env{wt, porID, porRol} {
		for _, c := range malos {
			if d := Evaluate(bash(c), baseLimits(), env); d.Allow {
				t.Errorf("un worker pudo: %q", c)
			}
		}
	}
	buenos := []string{
		`orq slice status`, `orq slice verify`, `orq slice --slice 01 verificar`, `orq slice receipt`, `orq slice contract`,
		`orq msg read`, `orq msg send --to orchestrator --type question "need X"`,
		`orq mem add --source scout --cat tecnico --slice S-01 "uses pgx"`, `orq mem show`,
		`orq worker beat --phase running --summary "half"`, `orq worker done --summary ok --ask-verification`,
		`orq stop raise --type api_key --severity blocking --slice S-01 --text "need key"`, `orq stop list --open`,
		`orq qa finding add S-07 --requirement R1 --commit abc --repro x --observed x --expected y --impact z`,
		`orq learn confirm i1`, `orq skill worker`, `orq board`, `orq plan show`,
		`go test ./...`, `git status`, `FOO=1 go test ./...`, `echo $HOME`,
	}
	for _, env := range []Env{wt, porID, porRol} {
		for _, c := range buenos {
			if d := Evaluate(bash(c), baseLimits(), env); !d.Allow {
				t.Errorf("bloqueó trabajo de worker: %q (%s)", c, d.Reason)
			}
		}
	}
	main := fakes{}.env(t, t.TempDir())
	for _, rol := range []string{"", "owner", "atena", "orquestador", "planificador", "astra"} {
		main.Rol = rol
		for _, c := range []string{`orq slice exam t --approved m1`, `orq autonomy free --scope src`, `orq --slice 01 slice close --accepted`, `orq token`} {
			if d := Evaluate(bash(c), baseLimits(), main); !d.Allow {
				t.Errorf("rol %q no pudo %q: %s", rol, c, d.Reason)
			}
		}
	}
}

func TestOrqPermitidoAWorker(t *testing.T) {
	si := [][]string{{}, {"version"}, {"gate"}, {"contexto"}, {"relevo", "--auto"}, {"slice"}, {"slice", "--slice", "01", "verify"}}
	no := [][]string{{"slice", "exam"}, {"--slice", "01", "slice", "verify"}, {"token"}, {"relevo", "orquestador"}, {"mem", "add", "--source=owner"}}
	for _, a := range si {
		if !OrqPermitidoAWorker(a) {
			t.Errorf("negó %v", a)
		}
	}
	for _, a := range no {
		if OrqPermitidoAWorker(a) {
			t.Errorf("permitió %v", a)
		}
	}
}

// Un worker no manda un cierre (ni historia ni plan), tampoco con flags de un guion o en inglés.
func TestWorkerMsgSoloTiposDeWorker(t *testing.T) {
	no := [][]string{
		{"msg", "send", "-de", "worker-w1", "-tipo", "cierre", "x"},
		{"msg", "send", "-tipo=cierre", "x"},
		{"msg", "send", "--type", "closing", "x"},
		{"msg", "send", "-type", "historia", "x"},
		{"msg", "send", "--tipo", "plan", "x"},
		{"mem", "add", "-source", "owner", "x"},
		{"mem", "add", "-fuente=atena", "x"},
		{"msg", "send", "-from", "orquestador", "--tipo", "estado", "x"},
	}
	for _, a := range no {
		if OrqPermitidoAWorker(a) {
			t.Errorf("permitió %v", a)
		}
	}
	for _, a := range [][]string{{"msg", "send", "--tipo", "estado", "x"}, {"msg", "send", "-type", "question", "x"}, {"msg", "read"}} {
		if !OrqPermitidoAWorker(a) {
			t.Errorf("negó %v", a)
		}
	}
}

// Defensa en profundidad para workers: la carpeta de config y /proc/*/environ no se leen con
// Read/Grep/Glob, y en Bash no hay globs sobre .config, expansiones ${!ORQ@}, ni escapes del árbol
// de procesos (setsid, nohup &, disown, systemd-run, at, tmux new, herdr).
func TestWorkerDefensaEnProfundidad(t *testing.T) {
	w := fakes{}.env(t, t.TempDir())
	w.WorkerID = "w1"
	home, _ := os.UserHomeDir()
	lee := []Payload{
		{ToolName: "Read", ToolInput: map[string]any{"file_path": home + "/.config/orq/owner.token"}},
		{ToolName: "Read", ToolInput: map[string]any{"file_path": "/proc/4242/environ"}},
		{ToolName: "Grep", ToolInput: map[string]any{"pattern": ".", "path": "~/.config"}},
		{ToolName: "Glob", ToolInput: map[string]any{"pattern": "~/.config/*/own*"}},
		{ToolName: "Glob", ToolInput: map[string]any{"pattern": "*/environ", "path": "/proc"}},
	}
	for _, p := range lee {
		p.Cwd = t.TempDir()
		if d := Evaluate(p, baseLimits(), w); d.Allow {
			t.Errorf("un worker leyó %v", p.ToolInput)
		}
	}
	malos := []string{
		`T=$(cat ~/.config/*/own?r.tok*); unset "${!ORQ@}"; export $(printf "ORQ_%s=%s ORQ_%s=%s" ROL owner TOKEN $T); ` + "`echo orq`" + ` mem add --source owner x`,
		`cat ~/.c?nfig/o*/*`,
		`ls ~/.config/*`,
		`unset "${!ORQ@}"`,
		`for v in ${!ORQ*}; do echo $v; done`,
		`export $(printf "ORQ_%s=x" ROL)`,
		`unset $(compgen -v | grep ORQ)`,
		`setsid sh -c 'orq token'`,
		`nohup sh -c x &`,
		`sleep 1 & disown`,
		`systemd-run --user sh -c x`,
		`echo x | at now`,
		`tmux new -d sh`,
		`tmux split-window sh`,
		`herdr pane split --current`,
		`python3 -c "import os; os.fork() or (os.setsid(), os.execv('/bin/sh',['sh']))"`,
		`python -c 'import subprocess; subprocess.Popen(["sh"], start_new_session=True)'`,
		`python3 - <<'EOF'` + "\nimport os\nos.setsid()\nEOF",
		`perl -e 'fork and exit; setsid(); exec "sh"'`,
		`node -e "require('child_process').spawn('sh',{detached:true})"`,
		`python3 -c "import os; os.system('or' 'q token')"; python3 -c 'import subprocess; subprocess.run(["orq","token"])'`,
		`python3 -c "import pty; pty.spawn('sh')"`,
		`script -qc sh /dev/null`,
		`ruby -e 'Process.daemon'`,
	}
	for _, c := range malos {
		if d := Evaluate(bash(c), baseLimits(), w); d.Allow {
			t.Errorf("un worker pudo: %q", c)
		}
	}
	for _, c := range []string{`echo look at this`, `go test ./... &`, `cat README.md`, `ls ~/project/*.go`, `grep -r ORQ_ROL docs`,
		`python3 -c "print(1+1)"`, `node -e "console.log(2)"`, `python3 -m pytest tests`, `python3 scripts/fork_report.py`} {
		if d := Evaluate(bash(c), baseLimits(), w); !d.Allow {
			t.Errorf("bloqueó %q: %s", c, d.Reason)
		}
	}
	// El Owner/Orquestador no recibe estas restricciones.
	m := fakes{}.env(t, t.TempDir())
	if d := Evaluate(Payload{ToolName: "Read", Cwd: "/", ToolInput: map[string]any{"file_path": home + "/.config/orq/owner.token"}}, baseLimits(), m); !d.Allow {
		t.Error("el Owner no lee su config")
	}
}
