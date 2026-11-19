package skilleval

import (
	"fmt"
	"strings"
)

// Veredicto dice si la versión nueva de una skill se queda: su nota sube más que el margen.
type Veredicto struct {
	Viejo, Nuevo Nota
	Delta        float64
	Conservar    bool
}

// Comparar toma la última nota completa de cada hash (o prefijo único) y conserva la nueva solo si
// sube más que margen: el juez es ruidoso y una diferencia chica no prueba nada.
func Comparar(ns []Nota, viejo, nuevo, tipo string, margen float64) (Veredicto, error) {
	v, err := ultimaDe(ns, viejo, tipo)
	if err != nil {
		return Veredicto{}, err
	}
	n, err := ultimaDe(ns, nuevo, tipo)
	if err != nil {
		return Veredicto{}, err
	}
	if v.Tipo != n.Tipo {
		return Veredicto{}, fmt.Errorf("%s es de tipo %s y %s de tipo %s; elige uno con --tipo", Corto(v.Hash), v.Tipo, Corto(n.Hash), n.Tipo)
	}
	d := n.Nota - v.Nota
	return Veredicto{Viejo: v, Nuevo: n, Delta: d, Conservar: d > margen}, nil
}

// ultimaDe es la nota completa más nueva del hash que empieza con prefijo, del tipo si se da.
func ultimaDe(ns []Nota, prefijo, tipo string) (Nota, error) {
	hash := ""
	for _, n := range ns {
		if !strings.HasPrefix(n.Hash, prefijo) || n.Hash == hash {
			continue
		}
		if hash != "" {
			return Nota{}, fmt.Errorf("el prefijo %s es ambiguo: %s y %s", prefijo, Corto(hash), Corto(n.Hash))
		}
		hash = n.Hash
	}
	var mejor Nota
	var hay, parcial bool
	for _, n := range ns {
		if n.Hash != hash || hash == "" || tipo != "" && n.Tipo != tipo {
			continue
		}
		if n.Parcial {
			parcial = true
			continue
		}
		if !hay || n.Fecha.After(mejor.Fecha) {
			mejor, hay = n, true
		}
	}
	switch {
	case hay:
		return mejor, nil
	case parcial:
		return Nota{}, fmt.Errorf("%s solo tiene notas parciales (tope de gasto): evalúala de nuevo", prefijo)
	case tipo != "":
		return Nota{}, fmt.Errorf("sin nota para %s de tipo %s", prefijo, tipo)
	}
	return Nota{}, fmt.Errorf("sin nota para %s: orq skill notas", prefijo)
}

// Corto es el prefijo de un hash que se muestra y que comparar acepta.
func Corto(h string) string {
	if len(h) > 12 {
		return h[:12]
	}
	return h
}
