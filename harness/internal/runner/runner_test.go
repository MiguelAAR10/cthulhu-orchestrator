package runner

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestSelectHerdrDentroSinoProcess(t *testing.T) {
	t.Setenv("ORQ_RUNNER", "")
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_PANE_ID", "p1")
	if r := Select(""); r.Name() != "herdr" {
		t.Fatalf("dentro de herdr: %s", r.Name())
	}
	t.Setenv("HERDR_ENV", "")
	t.Setenv("PATH", t.TempDir())
	if r := Select(""); r.Name() != "process" {
		t.Fatalf("sin herdr: %s", r.Name())
	}
	if r := Select("tmux"); r.Name() != "tmux" {
		t.Fatalf("config manda: %s", r.Name())
	}
	t.Setenv("ORQ_RUNNER", "process")
	if r := Select("tmux"); r.Name() != "process" {
		t.Fatalf("ORQ_RUNNER gana: %s", r.Name())
	}
}

func TestTmuxConstruyeComandos(t *testing.T) {
	var calls [][]string
	tm := &Tmux{Exec: func(a ...string) ([]byte, error) {
		calls = append(calls, a)
		if a[0] == "list-panes" {
			return []byte("%1 orquestador\n%7 worker-a-s01\n"), nil
		}
		return []byte("@3 %7\n"), nil
	}}
	tab, pane, err := tm.Open("S-01 · p", "/w", map[string]string{"ORQ_ROL": "w"})
	if err != nil || tab != "@3" || pane != "%7" {
		t.Fatalf("open: %q %q %v", tab, pane, err)
	}
	if !slices.Contains(calls[0], "new-window") || !slices.Contains(calls[0], "ORQ_ROL=w") || !slices.Contains(calls[0], "/w") {
		t.Fatalf("new-window: %v", calls[0])
	}
	calls = nil
	tm.Split("%1", "/w", nil)
	if strings.Join(calls[0][:3], " ") != "split-window -h -d" || !slices.Contains(calls[0], "%1") {
		t.Fatalf("split: %v", calls[0])
	}
	calls = nil
	if err := tm.Start("worker-a-s01", Launch{CLI: "claude", Args: []string{"--model", "it's"}}, "%7"); err != nil {
		t.Fatal(err)
	}
	if calls[0][0] != "select-pane" || calls[1][0] != "send-keys" || !strings.Contains(strings.Join(calls[1], " "), `claude --model 'it'\''s'`) {
		t.Fatalf("start: %v", calls)
	}
	calls = nil
	tm.Prompt("worker-a-s01", "hola")
	if calls[1][0] != "send-keys" || !slices.Contains(calls[1], "%7") || !slices.Contains(calls[1], "-l") || calls[2][len(calls[2])-1] != "Enter" {
		t.Fatalf("prompt: %v", calls)
	}
	calls = nil
	tm.Close("%7")
	if strings.Join(calls[0], " ") != "kill-pane -t %7" {
		t.Fatalf("close: %v", calls)
	}
}

func TestProcessLanzaConLogYPrompt(t *testing.T) {
	dir := t.TempDir()
	var got [][]string
	var stdin []string
	pr := &Process{Dir: dir, Spawn: func(p Proc) (int, error) {
		got = append(got, append([]string{p.Dir, p.Log}, p.Argv...))
		stdin = append(stdin, p.Stdin)
		if !slices.Contains(p.Env, "ORQ_ROL=w") {
			t.Fatalf("env: %v", p.Env)
		}
		return 4242, nil
	}}
	if !pr.Inside() {
		t.Fatal("process no necesita multiplexor")
	}
	_, pane, err := pr.Open("S-01", "/w", map[string]string{"ORQ_ROL": "w"})
	if err != nil {
		t.Fatal(err)
	}
	if err := pr.Start("w", Launch{CLI: "claude", Headless: []string{"-p", "--model", "sonnet"}, PromptMode: "stdin"}, pane); err != nil {
		t.Fatal(err)
	}
	if err := pr.Prompt("w", "contrato"); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0][0] != "/w" || strings.Join(got[0][2:], " ") != "claude -p --model sonnet" || stdin[0] != "contrato" {
		t.Fatalf("spawn: %v %v", got, stdin)
	}
	if filepath.Dir(got[0][1]) != dir {
		t.Fatalf("log fuera del dir: %s", got[0][1])
	}
	os.WriteFile(got[0][1], []byte("salida"), 0o644)
	if s, _ := pr.Read("w"); s != "salida" {
		t.Fatalf("read: %q", s)
	}
	// modo arg: el prompt reemplaza {prompt}
	pr.Start("c", Launch{CLI: "codex", Headless: []string{"exec", "{prompt}"}, PromptMode: "arg"}, pane)
	pr.Prompt("c", "haz X")
	if len(got) != 2 || strings.Join(got[1][2:], " ") != "codex exec haz X" || stdin[1] != "" {
		t.Fatalf("arg: %v", got)
	}
	if err := pr.Prompt("nadie", "x"); err == nil {
		t.Fatal("destino desconocido debe fallar")
	}
}

func TestFakeRegistraLlamadas(t *testing.T) {
	f := &Fake{}
	_, p, _ := f.Open("t", "/x", nil)
	f.Start("a", Launch{Kind: "claude"}, p)
	f.Prompt("a", "hola")
	if len(f.Calls) != 3 || f.Calls[2] != "prompt a hola" {
		t.Fatalf("%v", f.Calls)
	}
}
