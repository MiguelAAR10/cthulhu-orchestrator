// Package state guarda lo que el gate lee en su camino caliente:
// la última medición de orq watch y el archivo HALT.
package state

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Snapshot struct {
	TS         time.Time          `json:"ts"`
	ScratchGB  float64            `json:"scratch_gb"`
	PerPathGB  map[string]float64 `json:"per_path_gb"`
	DiskFreeGB map[string]float64 `json:"disk_free_gb"`
}

// Dir: ORQ_STATE gana; si no, XDG_STATE_HOME/orq; si no, ~/.local/state/orq. Aislar XDG_CONFIG_HOME
// no aísla el estado: quien quiera una raíz aparte fija también ORQ_STATE o XDG_STATE_HOME.
func Dir() string {
	if p := os.Getenv("ORQ_STATE"); p != "" {
		return p
	}
	if x := os.Getenv("XDG_STATE_HOME"); x != "" {
		return filepath.Join(x, "orq")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "state", "orq")
}

func snapshotPath(dir string) string { return filepath.Join(dir, "state.json") }
func haltPath(dir string) string     { return filepath.Join(dir, "HALT") }

func LoadSnapshot(dir string) (Snapshot, error) {
	var s Snapshot
	raw, err := os.ReadFile(snapshotPath(dir))
	if err != nil {
		return s, err
	}
	return s, json.Unmarshal(raw, &s)
}

func SaveSnapshot(dir string, s Snapshot) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := snapshotPath(dir) + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, snapshotPath(dir))
}

// ReadHalt devuelve el motivo y true si HALT existe.
func ReadHalt(dir string) (string, bool) {
	raw, err := os.ReadFile(haltPath(dir))
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(raw)), true
}

func WriteHalt(dir, reason string, now time.Time) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	body := now.Format(time.RFC3339) + " " + reason + "\n"
	return os.WriteFile(haltPath(dir), []byte(body), 0o644)
}

func ClearHalt(dir string) error {
	err := os.Remove(haltPath(dir))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
