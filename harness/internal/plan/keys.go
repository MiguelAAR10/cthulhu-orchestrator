package plan

import "encoding/json"

// Plans are written with English keys; files with the original Spanish keys
// (tareas, zona, rutas, depende_de, dificultad, que, estado, debatido_rev) still load.
// English values (pending|in_progress|done, low|medium|high) are accepted too.

var valorES = map[string]string{
	"pending": "pendiente", "in_progress": "en_curso", "done": "hecha",
	"low": "baja", "medium": "media", "high": "alta",
}

func esValor(v string) string {
	if es, ok := valorES[v]; ok {
		return es
	}
	return v
}

func (t *Tarea) UnmarshalJSON(b []byte) error {
	type plano Tarea
	var en plano
	if err := json.Unmarshal(b, &en); err != nil {
		return err
	}
	var es struct {
		Zona       string   `json:"zona"`
		Rutas      []string `json:"rutas"`
		DependeDe  []string `json:"depende_de"`
		Dificultad string   `json:"dificultad"`
		Que        string   `json:"que"`
		Estado     string   `json:"estado"`
	}
	if err := json.Unmarshal(b, &es); err != nil {
		return err
	}
	*t = Tarea(en)
	t.Zona = primero(t.Zona, es.Zona)
	if t.Rutas == nil {
		t.Rutas = es.Rutas
	}
	if t.DependeDe == nil {
		t.DependeDe = es.DependeDe
	}
	t.Dificultad = esValor(primero(t.Dificultad, es.Dificultad))
	t.Que = primero(t.Que, es.Que)
	t.Estado = esValor(primero(t.Estado, es.Estado))
	return nil
}

func (p *Plan) UnmarshalJSON(b []byte) error {
	type plano Plan
	var en plano
	if err := json.Unmarshal(b, &en); err != nil {
		return err
	}
	var es struct {
		Tareas      []Tarea `json:"tareas"`
		DebatidoRev string  `json:"debatido_rev"`
	}
	if err := json.Unmarshal(b, &es); err != nil {
		return err
	}
	*p = Plan(en)
	if p.Tareas == nil {
		p.Tareas = es.Tareas
	}
	p.DebatidoRev = primero(p.DebatidoRev, es.DebatidoRev)
	return nil
}

func primero(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
