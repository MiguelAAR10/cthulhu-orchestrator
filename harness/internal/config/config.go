// Package config lee y escribe .orq/config.json: autonomía, política de memoria y vigía.
// Conserva las claves que no conoce.
package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"orq/internal/identidad"
	"orq/internal/orqdir"
)

type Budget struct {
	MinutesPerTask int `json:"minutes_per_task,omitempty"`
	MaxWorkers     int `json:"max_workers,omitempty"`
	MaxAttempts    int `json:"max_attempts,omitempty"`
}

type Autonomy struct {
	Mode       string    `json:"mode"` // guided | free
	Scope      []string  `json:"scope,omitempty"`
	Budget     Budget    `json:"budget"`
	ApprovedBy string    `json:"approved_by,omitempty"`
	ApprovedAt time.Time `json:"approved_at,omitzero"`
	Firma      string    `json:"firma,omitempty"` // HMAC con el token del Owner
}

type Watch struct {
	StaleMin int `json:"stale_min,omitempty"`
}

type Config struct {
	Autonomy *Autonomy `json:"autonomy,omitempty"`
	Sync     string    `json:"sync,omitempty"` // local | commit | branch
	Watch    *Watch    `json:"watch,omitempty"`
	// AutonomiaSinFirma: el archivo dice free sin firma válida; no vale, el modo efectivo es guided.
	AutonomiaSinFirma bool `json:"-"`
	extra             map[string]json.RawMessage
}

var SyncModes = []string{"local", "commit", "branch"}

func Path(orqDir string) string { return filepath.Join(orqDir, "config.json") }

func Load(orqDir string) (Config, error) {
	var c Config
	raw, err := os.ReadFile(Path(orqDir))
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	if err := json.Unmarshal(raw, &c); err != nil {
		return c, err
	}
	json.Unmarshal(raw, &c.extra)
	if a := c.Autonomy; a != nil && a.Mode == "free" {
		f, cp := a.Firma, *a
		cp.Firma = ""
		c.AutonomiaSinFirma = !identidad.RegistroValido("autonomy", identidad.Alcance(orqDir), cp, f)
	}
	return c, nil
}

func Save(orqDir string, c Config) error {
	m := map[string]any{}
	for k, v := range c.extra {
		m[k] = v
	}
	delete(m, "autonomy")
	delete(m, "sync")
	delete(m, "watch")
	if c.Autonomy != nil {
		m["autonomy"] = c.Autonomy
	}
	if c.Sync != "" {
		m["sync"] = c.Sync
	}
	if c.Watch != nil {
		m["watch"] = c.Watch
	}
	raw, _ := json.MarshalIndent(m, "", "  ")
	os.MkdirAll(orqDir, 0o755)
	return orqdir.WriteAtomic(Path(orqDir), append(raw, '\n'))
}

func (c Config) Mode() string {
	if c.Autonomy == nil || c.Autonomy.Mode != "free" || c.AutonomiaSinFirma {
		return "guided"
	}
	return "free"
}

func (c Config) SyncMode() string {
	if c.Sync == "" {
		return "local"
	}
	return c.Sync
}

func (c Config) StaleMin() int {
	if c.Watch == nil || c.Watch.StaleMin <= 0 {
		return 10
	}
	return c.Watch.StaleMin
}

// FirmarAutonomia firma la aprobación. El llamador ya comprobó que es el Owner o Atena.
func (c *Config) FirmarAutonomia(orqDir string) {
	if c.Autonomy == nil {
		return
	}
	c.Autonomy.Firma = ""
	c.Autonomy.Firma = identidad.FirmarRegistro("autonomy", identidad.Alcance(orqDir), *c.Autonomy)
	c.AutonomiaSinFirma = false
}
