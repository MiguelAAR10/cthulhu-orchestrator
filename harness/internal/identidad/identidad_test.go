package identidad

import "testing"

func TestTexto(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq2")
	casos := map[string]string{
		"corre `orq slice verificar`":           "corre `orq2 slice verificar`",
		"orq init\n":                            "orq2 init\n",
		`run "orq skill worker"`:                `run "orq2 skill worker"`,
		"~/.config/orq/config.json":             "~/.config/orq/config.json",
		"$HOME/bin/orq gate":                    "$HOME/bin/orq gate",
		"skill orq-planner":                     "skill orq-planner",
		"[orq · historia de atena]":             "[orq · historia de atena]",
		"orq: fallo interno":                    "orq: fallo interno",
		".orq/relevo-worker.md":                 ".orq/relevo-worker.md",
		"(Responde con: orq msg send --para x)": "(Responde con: orq2 msg send --para x)",
	}
	for in, want := range casos {
		if got := Texto(in); got != want {
			t.Errorf("Texto(%q) = %q, want %q", in, got, want)
		}
	}
	t.Setenv("ORQ_NOMBRE", "orq")
	if got := Texto("`orq slice verificar`"); got != "`orq slice verificar`" {
		t.Errorf("orq rewrote its own text: %q", got)
	}
	if Binario() != "$HOME/bin/orq" {
		t.Errorf("orq hook binary changed: %q", Binario())
	}
}

func TestDefaults(t *testing.T) {
	t.Setenv("HOME", "/h")
	t.Setenv("ORQ_NOMBRE", "orq2")
	if ConfigDir() != "/h/.config/orq2" || StateDir() != "/h/.local/state/orq2" {
		t.Errorf("orq2 defaults: %s %s", ConfigDir(), StateDir())
	}
	t.Setenv("ORQ_NOMBRE", "")
	if ConfigDir() != "/h/.config/orq" || StateDir() != "/h/.local/state/orq" {
		t.Errorf("orq defaults: %s %s", ConfigDir(), StateDir())
	}
}
