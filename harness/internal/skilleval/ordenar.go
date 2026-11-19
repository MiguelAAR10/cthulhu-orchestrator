package skilleval

import (
	"fmt"
	"sort"
)

// Eleccion es una skill con su última nota completa para un tipo; Nota nil = sin nota.
type Eleccion struct {
	Skill string
	Nota  *float64
}

func (e Eleccion) String() string {
	if e.Nota == nil {
		return e.Skill + ":-"
	}
	return fmt.Sprintf("%s:%.2f", e.Skill, *e.Nota)
}

// Ordenar pone primero las skills con nota para el tipo, de mayor a menor, y después las que no
// tienen nota en el orden dado. Una nota parcial no cuenta. Con minimo, las que quedan por debajo
// van a fuera.
func Ordenar(nombres []string, ns []Nota, tipo string, minimo *float64) (elegidas, fuera []Eleccion) {
	ultima := map[string]Nota{}
	for _, n := range ns {
		if n.Tipo != tipo || n.Parcial {
			continue
		}
		if u, ok := ultima[n.Skill]; !ok || !n.Fecha.Before(u.Fecha) {
			ultima[n.Skill] = n
		}
	}
	var conNota, sinNota []Eleccion
	for _, s := range nombres {
		u, ok := ultima[s]
		switch {
		case !ok:
			sinNota = append(sinNota, Eleccion{Skill: s})
		case minimo != nil && u.Nota < *minimo:
			fuera = append(fuera, Eleccion{Skill: s, Nota: &u.Nota})
		default:
			conNota = append(conNota, Eleccion{Skill: s, Nota: &u.Nota})
		}
	}
	sort.SliceStable(conNota, func(i, j int) bool {
		if *conNota[i].Nota != *conNota[j].Nota {
			return *conNota[i].Nota > *conNota[j].Nota
		}
		return conNota[i].Skill < conNota[j].Skill
	})
	return append(conNota, sinNota...), fuera
}
