package app

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"orq/internal/canal"
	"orq/internal/diario"
	"orq/internal/identidad"
	"orq/internal/orqdir"
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
	if errors.Is(err, slice.ErrNoAbierto) || err != nil && !identidad.EsOrq2() {
		// orq keeps 1.0.0: any read error counts as closed.
		return "", nil // closed or never opened: nothing of this worker left to merge
	}
	if err != nil {
		// Unreadable is not finished: the worker's work may still need merging, so the pane stays.
		return "", fmt.Errorf("%s no se cierra: no se pudo leer el estado de S-%s (%w)", nombre, m[1], err)
	}
	if e == nil || e.Fase != slice.Implementando {
		return "", nil // past building: nothing of this worker left to merge
	}
	ses, err := canal.LoadSesiones(p.SesPath())
	if err != nil {
		return "", err
	}
	if _, vive := ses[nombre]; !vive {
		return "", fmt.Errorf("no hay sesión %s (orq sesion ver): nada se integra", nombre)
	}
	if len(e.Equipo) > 0 && !slices.ContainsFunc(e.Equipo, func(w slice.Worker) bool { return w.Nombre == nombre }) {
		return "", fmt.Errorf("%s no es del equipo de S-%s: nada se integra", nombre, e.NN)
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

// CerrarMiembro is `orq sesion cerrar NOMBRE`: the worker's merge and verification, then its pane.
// It returns the report, the sessions closed, and why it did not close. In orq2 the members of one
// slice close one at a time, from the check "is another member still open?" to their session
// leaving the registry: two closing at once never both wait for the other, so exactly one verifies.
// A launch in the slice takes the same lock (after lanzar-sNN; see LanzarWorker for the order).
func (p Proyecto) CerrarMiembro(ctx context.Context, nombre string) (string, []string, error) {
	if m := sliceDelWorker.FindStringSubmatch(nombre); m != nil && identidad.EsOrq2() {
		soltar, err := orqdir.Candado(filepath.Join(p.Dir, "cierre-s"+m[1]))
		if err != nil {
			return "", nil, err
		}
		defer soltar()
	}
	ses, err := canal.LoadSesiones(p.SesPath())
	if err != nil {
		return "", nil, err
	}
	informe, err := p.CerrarWorker(ctx, nombre)
	if err != nil {
		return informe, nil, err
	}
	cerradas, err := p.CerrarSesiones(ctx, ses, func(k string) bool { return k == nombre })
	return informe, cerradas, err
}
