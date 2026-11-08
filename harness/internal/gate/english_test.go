package gate

import (
	"strings"
	"testing"
)

// English command names are restricted exactly like their Spanish aliases.
func TestEnglishFormsEquallyRestricted(t *testing.T) {
	wt := fakes{wt: true}.env(t, t.TempDir())
	main := fakes{}.env(t, t.TempDir())
	for _, c := range []string{
		`orq slice exam tests/x.sh --approved "req ATN-1" --cmd "true"`,
		`orq slice --slice 01 exam tests/x.sh --approved m1`,
		`orq slice scope src --approved m1`,
		`orq slice open 02 --title x --test t.sh --cmd "sh t.sh"`,
		`orq slice close --accepted`,
		`orq slice retry "new idea"`,
		`orq slice take w1`,
		`orq slice launch --difficulty low`,
		`orq slice merge`,
		`orq plan load p.json`,
		`orq plan --slice 01 done T1`,
		`orq plan debated`,
		`orq session open planner`,
		`orq session close x`,
		`orq learn promote L1`,
		`orq msg send --to atena --type closing "done"`,
		`orq msg send --to atena --type cierre "done"`,
		`orq msg send --para atena --tipo closing "done"`,
	} {
		if d := Evaluate(bash(c), baseLimits(), wt); d.Allow {
			t.Errorf("a worker could: %q", c)
		}
		if d := Evaluate(bash(c), baseLimits(), main); !d.Allow {
			t.Errorf("the orchestrator could not: %q (%s)", c, d.Reason)
		}
	}
	for _, c := range []string{`orq slice status`, `orq slice verify`, `orq slice receipt`, `orq plan show`, `orq msg read`} {
		if d := Evaluate(bash(c), baseLimits(), wt); !d.Allow {
			t.Errorf("a worker should be able to: %q (%s)", c, d.Reason)
		}
	}
}

func TestGateMessagesEnglishByDefaultSpanishWithOrqLang(t *testing.T) {
	wt := fakes{wt: true}.env(t, t.TempDir())
	t.Setenv("ORQ_LANG", "")
	if d := Evaluate(bash("orq slice exam x"), baseLimits(), wt); d.Allow || !strings.Contains(d.Reason, "worker does not open") {
		t.Errorf("english: %q", d.Reason)
	}
	t.Setenv("ORQ_LANG", "es")
	if d := Evaluate(bash("orq slice examen x"), baseLimits(), wt); d.Allow || !strings.Contains(d.Reason, "un worker no abre") {
		t.Errorf("spanish: %q", d.Reason)
	}
}
