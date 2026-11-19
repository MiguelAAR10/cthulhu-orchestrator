package app

import (
	"strings"

	"orq/internal/canal"
	"orq/internal/slice"
)

// PaginaDecision suma a la página del slice la historia que el Atena aprobó, si el canal la tiene.
type PaginaDecision struct {
	*slice.Pagina
	Historia       []string // hasta 3 líneas
	FuenteHistoria string
}

// PaginaDecision arma la página con la que el Owner acepta o rechaza el slice nn.
func (p Proyecto) PaginaDecision(nn string) (*PaginaDecision, error) {
	pg, err := p.Slices().Pagina(nn)
	if err != nil {
		return nil, err
	}
	pd := &PaginaDecision{Pagina: pg, Historia: []string{pg.Estado.Titulo}, FuenteHistoria: "título del slice"}
	ms, _ := canal.Load(p.CanalPath())
	for i := len(ms) - 1; i >= 0; i-- {
		if ms[i].Tipo != "historia" {
			continue
		}
		var lineas []string
		for _, l := range strings.Split(ms[i].Texto, "\n") {
			if l = strings.TrimSpace(l); l != "" && len(lineas) < 3 {
				lineas = append(lineas, l)
			}
		}
		if len(lineas) > 0 {
			pd.Historia, pd.FuenteHistoria = lineas, "canal: historia "+ms[i].ID
		}
		break
	}
	return pd, nil
}
