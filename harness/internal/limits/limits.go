// Package limits carga ~/.config/orq/limits.json.
// Regla: un campo ausente es un dato desconocido, y lo desconocido deniega.
// Por eso los campos obligatorios son punteros: nil = ausente, no cero.
package limits

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Scratch struct {
	Paths          []string `json:"paths"`
	QuotaGB        *float64 `json:"quota_gb"`
	CriticalFreeGB *float64 `json:"critical_free_gb"`
}

type Limits struct {
	MinFreeGB         map[string]float64 `json:"min_free_gb"`
	MaxWorkers        *int               `json:"max_workers"`
	Scratch           Scratch            `json:"scratch"`
	StateMaxAgeHours  *float64           `json:"state_max_age_hours"`
	Protected         []string           `json:"protected"`
	WorkerProcessName []string           `json:"worker_process_names"`
	// PushRamas: checkouts principales donde el Owner deja subir ramas (nunca main/master, nunca forzado).
	PushRamas []string `json:"push_ramas,omitempty"`
	// MaxSlices: slices abiertos a la vez por repo; 0 = el tope por defecto del paquete slice.
	MaxSlices int `json:"max_slices,omitempty"`
	// ConteoEstricto: todo slice exige --reporte/--formato; sin contadores no hay verde (por defecto, laxo).
	ConteoEstricto bool `json:"conteo_estricto,omitempty"`
}

// DefaultPath respeta ORQ_LIMITS para pruebas y usa ~/.config/orq/limits.json por defecto.
func DefaultPath() string {
	if p := os.Getenv("ORQ_LIMITS"); p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "orq", "limits.json")
}

// Load lee y valida. Devuelve error ante cualquier campo obligatorio ausente.
func Load(path string) (Limits, error) {
	var l Limits
	raw, err := os.ReadFile(path)
	if err != nil {
		return l, fmt.Errorf("limits: no se pudo leer %s: %w", path, err)
	}
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&l); err != nil {
		return l, fmt.Errorf("limits: JSON inválido en %s: %w", path, err)
	}
	if err := l.validate(); err != nil {
		return l, fmt.Errorf("limits: %s: %w", path, err)
	}
	home, _ := os.UserHomeDir()
	for i, p := range l.Scratch.Paths {
		l.Scratch.Paths[i] = expandHome(p, home)
	}
	if len(l.WorkerProcessName) == 0 {
		l.WorkerProcessName = []string{"codex", "claude"}
	}
	return l, nil
}

func (l Limits) validate() error {
	switch {
	case len(l.MinFreeGB) == 0:
		return fmt.Errorf("falta min_free_gb")
	case l.MaxWorkers == nil:
		return fmt.Errorf("falta max_workers")
	case l.StateMaxAgeHours == nil:
		return fmt.Errorf("falta state_max_age_hours")
	case len(l.Scratch.Paths) == 0:
		return fmt.Errorf("falta scratch.paths")
	case l.Scratch.QuotaGB == nil:
		return fmt.Errorf("falta scratch.quota_gb")
	case l.Scratch.CriticalFreeGB == nil:
		return fmt.Errorf("falta scratch.critical_free_gb")
	}
	return nil
}

func expandHome(p, home string) string {
	if p == "~" {
		return home
	}
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(home, p[2:])
	}
	return p
}
