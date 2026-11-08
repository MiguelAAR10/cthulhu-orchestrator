package perfil

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
)

//go:embed skills.catalog.json
var catalogJSON []byte

// Entry is one skill of the curated gallery. Never != "" means it is on the never-list.
type Entry struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"` // folder under .claude/skills
	Source         string   `json:"source"`
	Install        string   `json:"install,omitempty"` // "npx ..." or "copy:NAME"
	Ref            string   `json:"ref,omitempty"`     // pinned upstream ref ("" = pin on first install)
	SHA256         string   `json:"sha256,omitempty"`  // content hash (HuellaSkill); "" = trust on first install, then the lock rules
	Phases         []string `json:"phases,omitempty"`  // T C A D V DOC or *
	Roles          []string `json:"roles,omitempty"`
	ProjectTypes   []string `json:"project_types,omitempty"`
	Stacks         []string `json:"stacks,omitempty"`
	Requires       []string `json:"requires,omitempty"`
	MinCriticality string   `json:"min_criticality,omitempty"` // low | medium | high
	TokenTier      string   `json:"token_tier,omitempty"`      // low | med | high
	Summary        string   `json:"summary,omitempty"`
	Adapt          string   `json:"adapt,omitempty"`
	Never          string   `json:"never,omitempty"`
}

type Catalog struct {
	Version int     `json:"version"`
	Skills  []Entry `json:"skills"`
}

// Phases of TCADD plus independent verification.
var Phases = []string{"T", "C", "A", "D", "V", "DOC"}

func DefaultCatalog() Catalog {
	var c Catalog
	if err := json.Unmarshal(catalogJSON, &c); err != nil {
		panic(err) // broken embedded catalog: a build error of the harness
	}
	return c
}

// LoadCatalog is the embedded catalog with orqDir/skills.catalog.json on top (same id replaces, new id appends).
func LoadCatalog(orqDir string) (Catalog, error) {
	c := DefaultCatalog()
	raw, err := os.ReadFile(filepath.Join(orqDir, "skills.catalog.json"))
	if os.IsNotExist(err) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	var o Catalog
	if err := json.Unmarshal(raw, &o); err != nil {
		return c, fmt.Errorf("%s: %w", filepath.Join(orqDir, "skills.catalog.json"), err)
	}
	for _, e := range o.Skills {
		if i := slices.IndexFunc(c.Skills, func(x Entry) bool { return x.ID == e.ID }); i >= 0 {
			c.Skills[i] = e
		} else {
			c.Skills = append(c.Skills, e)
		}
	}
	return c, nil
}

// Get finds an entry by id or by name.
func (c Catalog) Get(key string) (Entry, bool) {
	for _, e := range c.Skills {
		if e.ID == key || e.Name == key {
			return e, true
		}
	}
	return Entry{}, false
}

func (c Catalog) NeverReason(name string) (string, bool) {
	for _, e := range c.Skills {
		if (e.Name == name || e.ID == name) && e.Never != "" {
			return e.Never, true
		}
	}
	return "", false
}

func (c Catalog) neverMap() map[string]string {
	m := map[string]string{}
	for _, e := range c.Skills {
		if e.Never != "" {
			m[e.Name] = e.Never
		}
	}
	return m
}
