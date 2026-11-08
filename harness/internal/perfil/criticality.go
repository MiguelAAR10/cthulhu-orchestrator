package perfil

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Criticality is the single source of truth for the project's stakes.
// .orq/profile.json (written by `orq profile set criticality=…`) wins; the old
// criticality in .orq/config.json (root or project.criticality) is only a legacy
// fallback and comes back with a deprecation note. "" means not set.
func Criticality(orqDir string) (value, note string) {
	if raw, err := os.ReadFile(filepath.Join(orqDir, "profile.json")); err == nil {
		var f Forced
		if json.Unmarshal(raw, &f) == nil && f["criticality"] != "" {
			return f["criticality"], ""
		}
	}
	raw, err := os.ReadFile(filepath.Join(orqDir, "config.json"))
	if err != nil {
		return "", ""
	}
	var m struct {
		Criticality any `json:"criticality"`
		Project     struct {
			Criticality string `json:"criticality"`
		} `json:"project"`
	}
	if json.Unmarshal(raw, &m) != nil {
		return "", ""
	}
	v := m.Project.Criticality
	if s, ok := m.Criticality.(string); ok && v == "" {
		v = s
	}
	if v == "" {
		return "", ""
	}
	return v, "deprecated: criticality in .orq/config.json; use `orq profile set criticality=" + v + "`"
}
