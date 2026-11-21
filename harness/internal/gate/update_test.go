package gate

import (
	"testing"
)

// H1: `orq2 update`, in every mode, is the Owner's. The gate denies it to a subagent, a worker
// (worktree or clone) and any panel orq opened (ORQ_ROL), however it is wrapped.
func TestGateNiegaOrq2UpdateAAgentes(t *testing.T) {
	// Under identity orq the gate stays 1.0.0's: none of these is denied for naming update.
	t.Setenv("ORQ_NOMBRE", "orq")
	t.Setenv("ORQ_ROL", "orquestador")
	for _, c := range []string{"orq2 update", "script -qec 'orq2 update' /dev/null"} {
		p := bash(c)
		p.AgentID = "sub-1"
		if d := Evaluate(p, baseLimits(), fakes{}.env(t, t.TempDir())); !d.Allow {
			t.Errorf("orq: %q denied, 1.0.0 allowed it: %s", c, d.Reason)
		}
	}
	for _, nombre := range []string{"orq2"} {
		t.Setenv("ORQ_NOMBRE", nombre)
		cmds := []string{
			"orq2 update",
			"orq2 update --ver",
			"orq2 update --volver",
			"orq2 update --registrar abc --revisor r --veredicto sin-high --ci-exit 0 --ci-log l",
			"cd /tmp && ORQ_ROL= ~/bin/orq2 update",
			"script -qec 'orq2 update' /dev/null",
			"bash -c \"orq2 update --volver\"",
			"/home/x/.local/share/orq2-rollback/orq2-24f2438 update",
		}
		for _, c := range cmds {
			t.Setenv("ORQ_ROL", "")
			p := bash(c)
			p.AgentID = "sub-1"
			if d := Evaluate(p, baseLimits(), fakes{}.env(t, t.TempDir())); d.Allow {
				t.Errorf("%s: subagent allowed %q", nombre, c)
			}
			if d := Evaluate(bash(c), baseLimits(), fakes{wt: true}.env(t, t.TempDir())); d.Allow {
				t.Errorf("%s: worker allowed %q", nombre, c)
			}
			t.Setenv("ORQ_ROL", "orquestador")
			if d := Evaluate(bash(c), baseLimits(), fakes{}.env(t, t.TempDir())); d.Allow {
				t.Errorf("%s: role panel allowed %q", nombre, c)
			}
		}
		// The Owner's own session (no agent id, not a worker, no role) is not stopped by the gate.
		t.Setenv("ORQ_ROL", "")
		if d := Evaluate(bash("orq2 update --ver"), baseLimits(), fakes{}.env(t, t.TempDir())); !d.Allow {
			t.Errorf("%s: Owner denied: %s", nombre, d.Reason)
		}
		// Words that merely mention it are not commands of a worker that need stopping... but an
		// agent context fails closed on any command line naming orq2 and update together.
		p := bash("echo orq2 && git log --oneline | grep update")
		p.AgentID = "sub-1"
		if d := Evaluate(p, baseLimits(), fakes{}.env(t, t.TempDir())); d.Allow {
			t.Errorf("%s: agent line naming orq2 and update allowed", nombre)
		}
	}
}
