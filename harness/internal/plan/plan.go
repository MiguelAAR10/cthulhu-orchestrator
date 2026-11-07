// Package plan es el grafo de tareas de un slice, escrito para agentes: cada tarea tiene zona,
// rutas que le pertenecen, dependencias y dificultad. Sale de lo que funcionó en OdontoFlow
// (DISPATCH.yaml): una tarea se despacha solo cuando sus dependencias están hechas y nadie en
// curso reclama sus rutas. Así ningún worker agota su contexto ni pisa a otro.
package plan

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"orq/internal/orqdir"
)

var (
	Dificultades = []string{"baja", "media", "alta"}
	Estados      = []string{"pendiente", "en_curso", "hecha"}
	zonaRe       = regexp.MustCompile(`^[a-z0-9-]{1,20}$`)
	idRe         = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,30}$`)
)

type Tarea struct {
	ID         string   `json:"id"`
	Zona       string   `json:"zona"`
	Rutas      []string `json:"rutas"`
	DependeDe  []string `json:"depende_de,omitempty"`
	Dificultad string   `json:"dificultad,omitempty"` // baja | media | alta → modelo y esfuerzo
	Que        string   `json:"que,omitempty"`        // una línea: qué deja hecho
	Estado     string   `json:"estado,omitempty"`     // pendiente | en_curso | hecha
	Worker     string   `json:"worker,omitempty"`
}

type Plan struct {
	Slice    string  `json:"slice"`
	Debatido bool    `json:"debatido,omitempty"` // un adversario lo atacó y se corrigió
	Tareas   []Tarea `json:"tareas"`
}

func Path(orqDir, nn string) string { return filepath.Join(orqDir, "planes", nn+".json") }

func Leer(path string) (*Plan, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var p Plan
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("plan: JSON inválido: %v", err)
	}
	for i := range p.Tareas {
		if p.Tareas[i].Estado == "" {
			p.Tareas[i].Estado = "pendiente"
		}
		if p.Tareas[i].Dificultad == "" {
			p.Tareas[i].Dificultad = "media"
		}
	}
	return &p, nil
}

func Guardar(path string, p *Plan) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, _ := json.MarshalIndent(p, "", " ")
	return orqdir.WriteAtomic(path, raw)
}

func dentro(ruta string, permitidas []string) bool {
	ruta = filepath.Clean(ruta)
	for _, p := range permitidas {
		p = strings.TrimSuffix(filepath.Clean(p), "/")
		if ruta == p || strings.HasPrefix(ruta, p+"/") {
			return true
		}
	}
	return false
}

// Validar rechaza lo que haría fallar el despacho: ids repetidos, dependencias que no existen,
// ciclos, zonas inválidas, tareas sin rutas y rutas fuera del alcance de la historia.
func Validar(p *Plan, alcance []string) error {
	if len(p.Tareas) == 0 {
		return errors.New("plan sin tareas")
	}
	ids := map[string]bool{}
	var errs []string
	for _, t := range p.Tareas {
		switch {
		case !idRe.MatchString(t.ID):
			errs = append(errs, fmt.Sprintf("id %q inválido", t.ID))
		case ids[t.ID]:
			errs = append(errs, "id repetido: "+t.ID)
		}
		ids[t.ID] = true
		if !zonaRe.MatchString(t.Zona) {
			errs = append(errs, t.ID+": zona en minúsculas, números o guiones")
		}
		if len(t.Rutas) == 0 {
			errs = append(errs, t.ID+": sin rutas; di qué archivos le pertenecen")
		}
		if !slices.Contains(Dificultades, t.Dificultad) {
			errs = append(errs, t.ID+": dificultad baja|media|alta")
		}
		if !slices.Contains(Estados, t.Estado) {
			errs = append(errs, t.ID+": estado pendiente|en_curso|hecha")
		}
		if len(alcance) > 0 {
			for _, r := range t.Rutas {
				if !dentro(r, alcance) {
					errs = append(errs, fmt.Sprintf("%s: %s está fuera del alcance de la historia", t.ID, r))
				}
			}
		}
	}
	for _, t := range p.Tareas {
		for _, d := range t.DependeDe {
			if !ids[d] {
				errs = append(errs, fmt.Sprintf("%s depende de %s, que no existe", t.ID, d))
			}
		}
	}
	if c := ciclo(p); c != "" {
		errs = append(errs, "ciclo: "+c)
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "\n"))
	}
	return nil
}

func ciclo(p *Plan) string {
	deps := map[string][]string{}
	for _, t := range p.Tareas {
		deps[t.ID] = t.DependeDe
	}
	estado := map[string]int{} // 0 sin ver, 1 en pila, 2 listo
	var pila []string
	var visitar func(string) string
	visitar = func(id string) string {
		estado[id] = 1
		pila = append(pila, id)
		for _, d := range deps[id] {
			switch estado[d] {
			case 1:
				i := slices.Index(pila, d)
				return strings.Join(append(pila[i:], d), " → ")
			case 0:
				if c := visitar(d); c != "" {
					return c
				}
			}
		}
		pila = pila[:len(pila)-1]
		estado[id] = 2
		return ""
	}
	for _, t := range p.Tareas {
		if estado[t.ID] == 0 {
			if c := visitar(t.ID); c != "" {
				return c
			}
		}
	}
	return ""
}

func (p *Plan) tarea(id string) *Tarea {
	for i := range p.Tareas {
		if p.Tareas[i].ID == id {
			return &p.Tareas[i]
		}
	}
	return nil
}

func solapan(a, b []string) []string {
	var out []string
	for _, x := range a {
		for _, y := range b {
			if dentro(x, []string{y}) || dentro(y, []string{x}) {
				out = append(out, x)
				break
			}
		}
	}
	return out
}

// Listas son las tareas que se pueden despachar ya: pendientes, con dependencias hechas y
// sin rutas que otra tarea en curso esté tocando. Bloqueadas dice por qué cada otra espera.
func Listas(p *Plan) (listas []Tarea, bloqueadas map[string]string) {
	bloqueadas = map[string]string{}
	var enCurso []Tarea
	for _, t := range p.Tareas {
		if t.Estado == "en_curso" {
			enCurso = append(enCurso, t)
		}
	}
	for _, t := range p.Tareas {
		if t.Estado != "pendiente" {
			continue
		}
		var faltan []string
		for _, d := range t.DependeDe {
			if x := p.tarea(d); x == nil || x.Estado != "hecha" {
				faltan = append(faltan, d)
			}
		}
		if len(faltan) > 0 {
			bloqueadas[t.ID] = "espera a " + strings.Join(faltan, ", ")
			continue
		}
		choque := ""
		for _, c := range enCurso {
			if s := solapan(t.Rutas, c.Rutas); len(s) > 0 {
				choque = fmt.Sprintf("%s (%s) está tocando %s", c.ID, c.Worker, strings.Join(s, ", "))
				break
			}
		}
		if choque != "" {
			bloqueadas[t.ID] = choque
			continue
		}
		listas = append(listas, t)
	}
	return listas, bloqueadas
}

// Tomar marca una tarea en curso para un worker. Solo si está lista.
func Tomar(p *Plan, id, worker string) (*Tarea, error) {
	t := p.tarea(id)
	if t == nil {
		return nil, fmt.Errorf("no existe la tarea %s", id)
	}
	if t.Estado != "pendiente" {
		return nil, fmt.Errorf("%s está %s", id, strings.ReplaceAll(t.Estado, "_", " "))
	}
	listas, bloq := Listas(p)
	if !slices.ContainsFunc(listas, func(x Tarea) bool { return x.ID == id }) {
		return nil, fmt.Errorf("%s no está lista: %s", id, bloq[id])
	}
	t.Estado, t.Worker = "en_curso", worker
	return t, nil
}

// Hecha la cierra. Solo desde en curso: una tarea no se da por hecha sin que alguien la tomara.
func Hecha(p *Plan, id string) (*Tarea, error) {
	t := p.tarea(id)
	if t == nil {
		return nil, fmt.Errorf("no existe la tarea %s", id)
	}
	if t.Estado != "en_curso" {
		return nil, fmt.Errorf("%s está %s, no en curso", id, t.Estado)
	}
	t.Estado = "hecha"
	return t, nil
}

// Conservar lleva al plan recargado el avance del guardado: una tarea en curso o hecha no vuelve a pendiente.
// Rehacer una tarea hecha es una tarea nueva, con otro id.
func Conservar(nuevo, viejo *Plan) []string {
	var kept []string
	for i := range nuevo.Tareas {
		t := &nuevo.Tareas[i]
		if v := viejo.tarea(t.ID); v != nil && v.Estado != "pendiente" && t.Estado == "pendiente" {
			t.Estado, t.Worker = v.Estado, v.Worker
			kept = append(kept, t.ID+" "+v.Estado)
		}
	}
	return kept
}

// Soltar devuelve a pendiente una tarea en curso cuyo worker no llegó a trabajar (arranque fallido).
func Soltar(p *Plan, id string) (*Tarea, error) {
	t := p.tarea(id)
	if t == nil {
		return nil, fmt.Errorf("no existe la tarea %s", id)
	}
	if t.Estado != "en_curso" {
		return nil, fmt.Errorf("%s está %s, no en curso", id, t.Estado)
	}
	t.Estado, t.Worker = "pendiente", ""
	return t, nil
}

func (p *Plan) Pendientes() int {
	n := 0
	for _, t := range p.Tareas {
		if t.Estado != "hecha" {
			n++
		}
	}
	return n
}

// Modelo traduce dificultad a modelo y esfuerzo por agente. Lo barato donde la tarea es fácil;
// lo potente donde es core. Tabla heredada del protocolo de OdontoFlow (2026-10).
func Modelo(agente, dificultad string) (modelo, esfuerzo string) {
	if dificultad == "" {
		dificultad = "media"
	}
	if agente == "codex" {
		switch dificultad {
		case "baja":
			return "gpt-6-luna", "medium"
		case "alta":
			return "gpt-6.1-sol", "high"
		}
		return "gpt-6.1-sol", "medium"
	}
	switch dificultad {
	case "baja":
		return "haiku", "medium"
	case "alta":
		return "opus", "high"
	}
	return "sonnet", "medium"
}

// Desvio compara el plan con lo que de verdad cambió en el slice. Es lo que el Planificador
// mira para mantener el plan mínimo y al día, sin reescribirlo entero.
type Desvio struct {
	SinPlan    []string // archivos cambiados que ninguna tarea cubre
	HechaVacia []string // tareas hechas sin ningún cambio en sus rutas
	Pendientes int
	EnCurso    []string
}

func (d Desvio) Hay() bool { return len(d.SinPlan) > 0 || len(d.HechaVacia) > 0 }

func Revisar(p *Plan, cambiados []string) Desvio {
	var d Desvio
	var todas []string
	for _, t := range p.Tareas {
		todas = append(todas, t.Rutas...)
		switch t.Estado {
		case "en_curso":
			d.EnCurso = append(d.EnCurso, t.ID+" ("+t.Worker+")")
		case "hecha":
			toco := false
			for _, f := range cambiados {
				if dentro(f, t.Rutas) {
					toco = true
					break
				}
			}
			if !toco {
				d.HechaVacia = append(d.HechaVacia, t.ID)
			}
		}
	}
	for _, f := range cambiados {
		if !dentro(f, todas) {
			d.SinPlan = append(d.SinPlan, f)
		}
	}
	d.Pendientes = p.Pendientes()
	return d
}
