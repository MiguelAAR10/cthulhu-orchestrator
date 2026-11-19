package slice

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"orq/internal/diario"
)

// Pagina es lo que el Owner necesita para decidir sobre un slice verificado, tomado de lo que orq ya
// guarda: el estado y su recibo, la alineación con el alcance y los cambios del examen en el diario.
type Pagina struct {
	Estado     *Estado
	Alineacion *Alineacion
	Examen     []diario.Evento // exámenes sumados o corregidos desde que se abrió
	Avisos     []string        // desvíos de la página misma, p. ej. la punta ya no es el commit verificado
}

// Pagina arma la página de decisión del slice nn; solo existe para un slice VERIFICADO.
func (r Repo) Pagina(nn string) (*Pagina, error) {
	e, err := r.Load(nn)
	if err != nil {
		return nil, err
	}
	if e == nil {
		return nil, errors.New("no hay slice abierto")
	}
	if e.Fase != Verificado || e.Recibo == nil {
		return nil, fmt.Errorf("%s está en %s; la página de decisión es para un slice %s", e.ID(), e.Fase, Verificado)
	}
	// The receipt shown is the one orq sealed in git, not whatever the state file says.
	rec, err := r.reciboConfiable(e)
	if err != nil {
		return nil, err
	}
	e, a, err := r.Alinear(e.NN)
	if err != nil {
		return nil, err
	}
	e.Recibo = rec
	var avisos []string
	if len(e.Equipo) == 0 && esClon(e.Worktree) {
		if in, err := r.ingerir(e, e.Worktree, e.Rama, "slice"); err == nil { // fetch only, as Alinear does
			if tip, err := r.git("rev-parse", "--verify", in.Punta+"^{commit}"); err == nil && tip != rec.Head {
				avisos = append(avisos, fmt.Sprintf("la punta del clon %s no es el commit verificado %s: los archivos de abajo son del verificado", Corto(tip), Corto(rec.Head)))
			}
		}
	}
	if fs, err := r.tocadas(e.SHARojo, rec.Head); err == nil {
		a.Cambiados = fs
		var fuera []string
		for _, f := range a.Fuera {
			if slices.Contains(fs, f) {
				fuera = append(fuera, f)
			}
		}
		a.Fuera = fuera
	}
	evs, err := diario.Leer(diario.Ruta(r.OrqDir))
	if err != nil {
		return nil, err
	}
	pg := &Pagina{Estado: e, Alineacion: a, Avisos: avisos}
	for _, ev := range evs {
		if ev.Tipo == "examen" && ev.Slice == e.NN && !ev.TS.Before(e.Abierto) {
			pg.Examen = append(pg.Examen, ev)
		}
	}
	return pg, nil
}

// Planeado son las rutas que el slice declaró: alcance y zonas del equipo.
func (p *Pagina) Planeado() []string {
	out := append([]string{}, p.Estado.Alcance...)
	for _, w := range p.Estado.Equipo {
		out = append(out, w.Rutas...)
	}
	return out
}

// RunnerAceptado dice si la verificación aceptó un cambio en los archivos del runner.
func (p *Pagina) RunnerAceptado() bool {
	return strings.Contains(p.Estado.Recibo.Nota, "runner cambiado y aceptado")
}

// Esperando devuelve los slices verificados que esperan decisión, el más antiguo primero.
func (r Repo) Esperando() ([]*Estado, error) {
	ab, err := r.Abiertos()
	if err != nil {
		return nil, err
	}
	var out []*Estado
	for _, e := range ab {
		if e.Fase == Verificado && e.Recibo != nil {
			out = append(out, e)
		}
	}
	slices.SortStableFunc(out, func(a, b *Estado) int { return a.Recibo.TS.Compare(b.Recibo.TS) })
	return out, nil
}
