package slice

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"orq/internal/diario"
	"orq/internal/orqdir"
)

// Historia groups the child slices of a large story. Each child keeps its own card, exam and contract.
type Historia struct {
	ID      string    `json:"id"`
	Titulo  string    `json:"titulo"`
	Abierta time.Time `json:"abierta"`
	Pausada bool      `json:"pausada,omitempty"`
}

var historiaRe = regexp.MustCompile(`^[a-z0-9-]{1,30}$`)

func (r Repo) dirHistorias() string          { return filepath.Join(r.OrqDir, "historias") }
func (r Repo) historiaPath(id string) string { return filepath.Join(r.dirHistorias(), id+".json") }
func (r Repo) guardarHistoria(h *Historia) error {
	if err := os.MkdirAll(r.dirHistorias(), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(h, "", "  ")
	if err != nil {
		return err
	}
	return orqdir.WriteAtomic(r.historiaPath(h.ID), raw)
}

// AbrirHistoria creates a story; its id names it in `orq slice abrir --historia`.
func (r Repo) AbrirHistoria(id, titulo string, now time.Time) (*Historia, error) {
	if !historiaRe.MatchString(id) {
		return nil, fmt.Errorf("historia %q: el id usa solo a-z, 0-9 y -, hasta 30", id)
	}
	if strings.TrimSpace(titulo) == "" {
		return nil, errors.New("la historia necesita --titulo")
	}
	soltar, err := r.candado("abrir")
	if err != nil {
		return nil, err
	}
	defer soltar()
	if existe(r.historiaPath(id)) {
		return nil, fmt.Errorf("la historia %s ya existe", id)
	}
	h := &Historia{ID: id, Titulo: titulo, Abierta: now.UTC().Truncate(time.Second)}
	return h, r.guardarHistoria(h)
}

// LeerHistoria loads a story by id.
func (r Repo) LeerHistoria(id string) (*Historia, error) {
	if !historiaRe.MatchString(id) {
		return nil, fmt.Errorf("historia %q: el id usa solo a-z, 0-9 y -", id)
	}
	raw, err := os.ReadFile(r.historiaPath(id))
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("no existe la historia %s: créala con orq historia abrir %s --titulo \"...\"", id, id)
	}
	if err != nil {
		return nil, err
	}
	var h Historia
	if err := json.Unmarshal(raw, &h); err != nil {
		return nil, fmt.Errorf("%s: %v", r.historiaPath(id), err)
	}
	return &h, nil
}

// Historias lists every story, sorted by id.
func (r Repo) Historias() ([]*Historia, error) {
	m, _ := filepath.Glob(filepath.Join(r.dirHistorias(), "*.json"))
	slices.Sort(m)
	var out []*Historia
	for _, f := range m {
		h, err := r.LeerHistoria(strings.TrimSuffix(filepath.Base(f), ".json"))
		if err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, nil
}

// Hijos are the story's open slices and its closed ones, in slice order.
func (r Repo) Hijos(id string) (abiertos []*Estado, cerrados []Registro, err error) {
	ab, err := r.Abiertos()
	if err != nil {
		return nil, nil, err
	}
	for _, e := range ab {
		if e.Historia == id {
			abiertos = append(abiertos, e)
		}
	}
	for _, reg := range leerJSONL[Registro](r.logPath()) {
		if reg.Historia == id {
			cerrados = append(cerrados, reg)
		}
	}
	return abiertos, cerrados, nil
}

func (r Repo) pausaPath() string { return filepath.Join(r.OrqDir, "pausa") }

func (r Repo) anotarPausa(pausada bool, que string, now time.Time) error {
	res := "reanudada"
	if pausada {
		res = "pausada"
	}
	return diario.Anotar(diario.Ruta(r.OrqDir), diario.Evento{TS: now, Tipo: "pausa", Resultado: res, Detalle: que})
}

// PausarHistoria pauses or resumes one story: its children get no new worker while paused.
func (r Repo) PausarHistoria(id string, pausada bool, now time.Time) error {
	soltar, err := r.candado("abrir")
	if err != nil {
		return err
	}
	defer soltar()
	h, err := r.LeerHistoria(id)
	if err != nil {
		return err
	}
	h.Pausada = pausada
	if err := r.guardarHistoria(h); err != nil {
		return err
	}
	return r.anotarPausa(pausada, id, now)
}

// PausarTodo pauses or resumes every launch in the project. Nothing running is touched.
func (r Repo) PausarTodo(pausada bool, motivo string, now time.Time) error {
	if pausada {
		if err := orqdir.WriteAtomic(r.pausaPath(), []byte(motivo)); err != nil {
			return err
		}
	} else if err := os.Remove(r.pausaPath()); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return r.anotarPausa(pausada, "todo", now)
}

// PausaGlobal says whether the project is paused, and why.
func (r Repo) PausaGlobal() (motivo string, ok bool) {
	raw, err := os.ReadFile(r.pausaPath())
	return string(raw), err == nil
}

// Pausado refuses a launch for e while the project or its story is paused.
func (r Repo) Pausado(e *Estado) error {
	if motivo, ok := r.PausaGlobal(); ok {
		return fmt.Errorf("orq está en pausa (%s): no se lanzan workers. Reanuda con orq reanudar", motivo)
	}
	if e.Historia == "" {
		return nil
	}
	h, err := r.LeerHistoria(e.Historia)
	if err != nil {
		return err
	}
	if h.Pausada {
		return fmt.Errorf("la historia %s está en pausa: %s no lanza workers. Reanuda con orq historia reanudar %s", h.ID, e.ID(), h.ID)
	}
	return nil
}
