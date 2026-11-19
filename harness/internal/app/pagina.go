package app

import (
	"slices"
	"strings"

	"orq/internal/canal"
	"orq/internal/slice"
)

// PaginaDecision suma a la página del slice la historia que el Atena aprobó, si el canal la tiene.
type PaginaDecision struct {
	*slice.Pagina
	Historia       []string // hasta 3 líneas
	FuenteHistoria string
	AvisosHistoria []string // the story may not be this slice's
}

// PaginaDecision arma la página con la que el Owner acepta o rechaza el slice nn.
func (p Proyecto) PaginaDecision(nn string) (*PaginaDecision, error) {
	pg, err := p.Slices().Pagina(nn)
	if err != nil {
		return nil, err
	}
	pd := &PaginaDecision{Pagina: pg, Historia: []string{pg.Estado.Titulo}, FuenteHistoria: "título del slice"}
	if tj := pg.Estado.Tarjeta; tj != nil {
		pd.Historia, pd.FuenteHistoria = tj.Historia, "tarjeta del slice"
		return pd, nil
	}
	ms, _ := canal.Load(p.CanalPath())
	// A historia linked to this slice (Attempt S-NN/aK) wins; the latest one is only a fallback.
	vinculo := func(m canal.Msg) bool { return strings.HasPrefix(m.Attempt, "S-"+pg.Estado.NN+"/") }
	enlazada := slices.ContainsFunc(ms, func(m canal.Msg) bool { return m.Tipo == "historia" && vinculo(m) })
	for i := len(ms) - 1; i >= 0; i-- {
		if ms[i].Tipo != "historia" || enlazada && !vinculo(ms[i]) {
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
		if !enlazada {
			if ab, err := p.Slices().Abiertos(); err == nil && len(ab) > 1 {
				pd.AvisosHistoria = append(pd.AvisosHistoria, "story may belong to another slice: the channel does not link it to "+pg.Estado.ID()+" and several slices are open")
			}
		}
		break
	}
	return pd, nil
}
