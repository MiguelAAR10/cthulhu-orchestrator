package stops

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"orq/internal/config"
)

var t0 = time.Date(2026, 11, 8, 10, 0, 0, 0, time.UTC)

func raise(t *testing.T, path string, r Raise) Stop {
	t.Helper()
	s, _, err := Add(path, r, t0)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestRaiseFoldsAndDedupes(t *testing.T) {
	p := filepath.Join(t.TempDir(), "stops.jsonl")
	r := Raise{Type: "api_key", Severity: "blocking", Slice: "S-04", Tasks: []string{"T2"}, By: "w1", Text: "Need a Stripe test key"}
	a := raise(t, p, r)
	b, dup, err := Add(p, r, t0)
	if err != nil || !dup || a.ID != b.ID || a.ID != "ST-0001" {
		t.Fatalf("dedupe: %v %v %s %s", err, dup, a.ID, b.ID)
	}
	all, _ := Load(p)
	if len(all) != 1 || all[0].State != "open" {
		t.Fatalf("%+v", all)
	}
	if h := Holds(all, "S-04"); h["T2"] != "stop ST-0001" {
		t.Fatalf("holds %v", h)
	}
	if _, err := Resolve(p, a.ID, Resolution{By: "atena", ProvidedAt: "env:STRIPE_TEST_KEY"}, t0); err != nil {
		t.Fatal(err)
	}
	all, _ = Load(p)
	if all[0].State != "resolved" || all[0].Provided != "yes" || all[0].ProvidedAt != "env:STRIPE_TEST_KEY" {
		t.Fatalf("%+v", all[0])
	}
	if len(Holds(all, "S-04")) != 0 {
		t.Fatal("resolved stopper still holds")
	}
	// a new raise after resolution gets a new id
	c := raise(t, p, r)
	if c.ID != "ST-0002" {
		t.Fatal(c.ID)
	}
}

func TestSoonDoesNotHold(t *testing.T) {
	p := filepath.Join(t.TempDir(), "stops.jsonl")
	raise(t, p, Raise{Type: "decision", Severity: "soon", Slice: "S-01", Tasks: []string{"T1"}, By: "w", Text: "which color", Options: []string{"a", "b"}})
	all, _ := Load(p)
	if len(Holds(all, "S-01")) != 0 {
		t.Fatal("soon must not hold")
	}
}

func TestValidation(t *testing.T) {
	p := filepath.Join(t.TempDir(), "stops.jsonl")
	bad := []Raise{
		{Type: "x", Severity: "soon", Slice: "S-01", Text: "t"},
		{Type: "decision", Severity: "soon", Slice: "S-01", Text: "pick"},                             // no options
		{Type: "api_key", Severity: "nah", Slice: "S-01", Text: "t"},                                  // severity
		{Type: "api_key", Severity: "soon", Slice: "S-01", Text: "use sk-live-abc123def456ghi789jkl"}, // secret
		{Type: "api_key", Severity: "soon", Slice: "S-01", Text: "ok", Where: "lower-case"},
	}
	for i, r := range bad {
		if _, _, err := Add(p, r, t0); err == nil {
			t.Fatalf("case %d accepted", i)
		}
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatal("rejected raises must not touch the file")
	}
}

func TestLooksSecret(t *testing.T) {
	yes := []string{"use sk-live-abc123def456ghi789jkl", "ghp_x", "AKIAIOSFODNN7EXAMPLE", "-----BEGIN PRIVATE", "token Zq8vR2mX9kLp4TnW7sYb3HcF"}
	no := []string{"I need a Stripe test API key to check payments. Put it in env var STRIPE_TEST_KEY.", "harness/internal/plan/plan.go", "aaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	for _, s := range yes {
		if !LooksSecret(s) {
			t.Errorf("should flag %q", s)
		}
	}
	for _, s := range no {
		if LooksSecret(s) {
			t.Errorf("should not flag %q", s)
		}
	}
}

func TestProvidedAt(t *testing.T) {
	root := t.TempDir()
	ok := []string{"env:STRIPE_TEST_KEY", "vault:prod/stripe", "file:/etc/x"}
	bad := []string{"env:lower", "STRIPE", "file:" + filepath.Join(root, "secret.txt"), "file:"}
	for _, s := range ok {
		if err := CheckProvidedAt(s, root, nil); err != nil {
			t.Errorf("%s: %v", s, err)
		}
	}
	for _, s := range bad {
		if err := CheckProvidedAt(s, root, func(string) bool { return false }); err == nil {
			t.Errorf("%s accepted", s)
		}
	}
	if err := CheckProvidedAt("file:"+filepath.Join(root, ".env"), root, func(string) bool { return true }); err != nil {
		t.Errorf("gitignored file inside repo must be accepted: %v", err)
	}
}

func TestCanResolve(t *testing.T) {
	free := config.Config{Autonomy: &config.Autonomy{Mode: "free"}}
	guided := config.Config{}
	dec := Stop{Type: "decision", Severity: "soon"}
	blk := Stop{Type: "decision", Severity: "blocking"}
	key := Stop{Type: "api_key", Severity: "fyi"}
	cases := []struct {
		role string
		s    Stop
		c    config.Config
		note string
		ok   bool
	}{
		{"atena", blk, guided, "", true},
		{"owner", key, guided, "", true},
		{"worker-front-s01", dec, free, "auto: x", false},
		{"orquestador", dec, guided, "auto: x", false},
		{"orquestador", dec, free, "auto: x", true},
		{"orquestador", dec, free, "", false}, // needs auto: note
		{"planificador", dec, free, "auto: y", true},
		{"orquestador", blk, free, "auto: x", false},
		{"orquestador", key, free, "auto: x", false},
	}
	for i, c := range cases {
		if err := CanResolve(c.role, c.s, c.c, c.note); (err == nil) != c.ok {
			t.Errorf("case %d: %v", i, err)
		}
	}
}

func TestCancelAndOrder(t *testing.T) {
	p := filepath.Join(t.TempDir(), "stops.jsonl")
	a := raise(t, p, Raise{Type: "access", Severity: "fyi", Slice: "S-01", By: "w", Text: "a"})
	raise(t, p, Raise{Type: "access", Severity: "blocking", Slice: "S-02", By: "w", Text: "b"})
	if _, err := Cancel(p, a.ID, "w", "not needed", t0); err != nil {
		t.Fatal(err)
	}
	all, _ := Load(p)
	if all[0].State != "cancelled" || len(Open(all)) != 1 {
		t.Fatalf("%+v", all)
	}
	if _, err := Cancel(p, a.ID, "w", "again", t0); err == nil {
		t.Fatal("cancel twice")
	}
}
