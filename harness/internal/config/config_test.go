package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultsWhenMissing(t *testing.T) {
	c, err := Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if c.Mode() != "guided" || c.SyncMode() != "local" || c.StaleMin() != 10 {
		t.Fatalf("defaults: %q %q %d", c.Mode(), c.SyncMode(), c.StaleMin())
	}
}

func TestSaveKeepsUnknownKeys(t *testing.T) {
	d := t.TempDir()
	os.WriteFile(filepath.Join(d, "config.json"), []byte(`{"otro":{"x":1},"sync":"commit"}`), 0o644)
	c, _ := Load(d)
	c.Sync = "branch"
	if err := Save(d, c); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(filepath.Join(d, "config.json"))
	c2, _ := Load(d)
	if c2.SyncMode() != "branch" || !strings.Contains(string(raw), `"otro"`) {
		t.Fatalf("got %s", raw)
	}
}
