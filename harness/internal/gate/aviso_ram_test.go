package gate

import (
	"testing"
	"time"
)

// The RAM warning rides on an allowed launch; it never refuses. Only capacity refuses.
func TestAvisoRAMNoNiega(t *testing.T) {
	now := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)
	l := baseLimits()
	l.MaxWorkers = nil
	env := fakes{free: map[string]float64{"/": 400, "/mnt/c": 100}, now: now}.env(t, freshState(t, now, 5))
	env.Capacidad = func(string, int) (string, error) { return "", nil }
	env.AvisoRAM = func() string { return "RAM libre 4.0 GB (aviso ≤5): corre orq2 doctor --ram" }
	d := Evaluate(Payload{ToolName: "Agent"}, l, env)
	if !d.Allow || d.Aviso != "RAM libre 4.0 GB (aviso ≤5): corre orq2 doctor --ram" {
		t.Fatalf("allowed launch must carry the warning: %+v", d)
	}
	env.Capacidad = func(string, int) (string, error) { return "no cabe otro agente por RAM", nil }
	if d := Evaluate(Payload{ToolName: "Agent"}, l, env); d.Allow {
		t.Fatal("hard capacity still refuses")
	}
	env.Capacidad = func(string, int) (string, error) { return "", nil }
	env.AvisoRAM = nil
	if d := Evaluate(Payload{ToolName: "Agent"}, l, env); !d.Allow || d.Aviso != "" {
		t.Fatalf("no warning func, no warning: %+v", d)
	}
}
