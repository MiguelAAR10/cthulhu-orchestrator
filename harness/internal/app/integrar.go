package app

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"orq/internal/canal"
	"orq/internal/diario"
	"orq/internal/slice"
)

var sliceDelWorker = regexp.MustCompile(`^worker-.+-s(\d+)$`)

// CerrarWorker runs before a worker's pane closes: orq merges the team's commits into the slice
// and verifies, and writes one ledger row. A failure returns its reason and the pane must stay.
// Verification waits while another team member still has a session. Outside IMPLEMENTANDO there
// is nothing to integrate.
func (p Proyecto) CerrarWorker(ctx context.Context, nombre string) (string, error) {
	m := sliceDelWorker.FindStringSubmatch(nombre)
	if m == nil {
		return "", nil
	}
	r := p.Slices()
	e, err := r.Load(m[1])
	if err != nil || e == nil || e.Fase != slice.Implementando {
		return "", nil // closed, missing or past building: nothing of this worker left to merge
	}
	ses, err := canal.LoadSesiones(p.SesPath())
	if err != nil {
		return "", err
	}
	fila := diario.Evento{Tipo: "integracion", Paso: "worker", Actor: nombre, Slice: e.NN, Intento: e.Intento, Ruta: e.RutaEfectiva()}
	var informe []string
	fallar := func(err error) (string, error) {
		fila.Resultado, fila.Detalle = "fallo", err.Error()
		if sha, errP := r.Punta(e); errP == nil {
			fila.Datos = map[string]any{"sha": sha}
		}
		return strings.Join(informe, "\n"), errors.Join(fmt.Errorf("%s no se cierra: %w", nombre, err), p.anotar(fila))
	}
	if len(e.Equipo) > 0 {
		var errJ error
		e, informe, errJ = r.Juntar(e.NN)
		if errJ != nil {
			return fallar(errJ)
		}
		for k := range ses {
			if k != nombre && sliceDelWorker.MatchString(k) && strings.HasSuffix(k, "-s"+e.NN) {
				sha, errP := r.Punta(e)
				if errP != nil {
					return fallar(errP)
				}
				fila.Resultado, fila.Datos = "ok", map[string]any{"sha": sha, "verificado": false}
				informe = append(informe, fmt.Sprintf("juntado en %s; se verifica cuando cierre el último del equipo (%s sigue)", slice.Corto(sha), k))
				return strings.Join(informe, "\n"), p.anotar(fila)
			}
		}
	}
	e, out, err := r.Verificar(e.NN, p.ahora())
	if out != "" {
		informe = append(informe, out)
	}
	if err != nil {
		return fallar(err)
	}
	fila.Resultado, fila.Datos = "ok", map[string]any{"sha": e.Recibo.Head, "verificado": true}
	informe = append(informe, fmt.Sprintf("%s verificado en %s", e.Attempt(), slice.Corto(e.Recibo.Head)))
	return strings.Join(informe, "\n"), p.anotar(fila)
}
