package plan

import (
	"errors"
	"fmt"
	"strings"
)

// MaxFrontsDefecto: más de 3 frentes en un slice ya no cabe en la cabeza de un orquestador.
const MaxFrontsDefecto = 3

// Front es un sub-orquestador como dato: una meta en palabras de producto, las rutas que le
// pertenecen (sin solaparse con otros fronts) y el rol dueño. No es un proceso: la admisión
// sigue siendo global y el orquestador principal solo consume su brief y su recibo.
type Front struct {
	ID        string   `json:"id"`
	Goal      string   `json:"goal"`
	Paths     []string `json:"paths"`
	OwnerRole string   `json:"owner_role,omitempty"`
}

func (p *Plan) front(id string) *Front {
	for i := range p.Fronts {
		if p.Fronts[i].ID == id {
			return &p.Fronts[i]
		}
	}
	return nil
}

// ValidarFronts: como mucho max fronts (0 = MaxFrontsDefecto), ids únicos, meta y rutas,
// rutas que no se solapan entre fronts; con fronts, cada tarea nombra el suyo y vive en sus rutas.
func ValidarFronts(p *Plan, max int) error {
	if max <= 0 {
		max = MaxFrontsDefecto
	}
	var errs []string
	if len(p.Fronts) > max {
		errs = append(errs, fmt.Sprintf("%d fronts; max_fronts es %d", len(p.Fronts), max))
	}
	ids := map[string]bool{}
	for i, f := range p.Fronts {
		switch {
		case !idRe.MatchString(f.ID):
			errs = append(errs, fmt.Sprintf("front id %q inválido", f.ID))
		case ids[f.ID]:
			errs = append(errs, "front repetido: "+f.ID)
		}
		ids[f.ID] = true
		if strings.TrimSpace(f.Goal) == "" {
			errs = append(errs, f.ID+": front sin goal (en palabras de producto)")
		}
		if len(f.Paths) == 0 {
			errs = append(errs, f.ID+": front sin paths")
		}
		for _, g := range p.Fronts[:i] {
			if s := solapan(f.Paths, g.Paths); len(s) > 0 {
				errs = append(errs, fmt.Sprintf("fronts %s y %s solapan rutas: %s", g.ID, f.ID, strings.Join(s, ", ")))
			}
		}
	}
	for _, t := range p.Tareas {
		if t.Front == "" {
			if len(p.Fronts) > 0 {
				errs = append(errs, t.ID+": sin front; con fronts, toda tarea nombra el suyo")
			}
			continue
		}
		f := p.front(t.Front)
		if f == nil {
			errs = append(errs, fmt.Sprintf("%s: front %s no existe", t.ID, t.Front))
			continue
		}
		for _, r := range t.Rutas {
			if !dentro(r, f.Paths) {
				errs = append(errs, fmt.Sprintf("%s: %s está fuera de las rutas del front %s", t.ID, r, f.ID))
			}
		}
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "\n"))
	}
	return nil
}

// ResumenFront: lo que el plan sabe de un front (sus tareas y cuántas están hechas).
type ResumenFront struct {
	Front  Front
	Done   int
	Total  int
	Tareas []Tarea
}

func (p *Plan) Resumen(id string) (ResumenFront, error) {
	f := p.front(id)
	if f == nil {
		return ResumenFront{}, fmt.Errorf("no existe el front %s", id)
	}
	r := ResumenFront{Front: *f}
	for _, t := range p.Tareas {
		if t.Front != id {
			continue
		}
		r.Total++
		if t.Estado == "hecha" {
			r.Done++
		}
		r.Tareas = append(r.Tareas, t)
	}
	return r, nil
}
