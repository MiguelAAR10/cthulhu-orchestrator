package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"orq/internal/canal"
	"orq/internal/diario"
	"orq/internal/gitx"
	"orq/internal/slice"
)

// PlazoArranque is how long a launched worker has to start and receive its contract.
const PlazoArranque = 60 * time.Second

// Alerta is one drift on an open slice. A healthy slice has none.
type Alerta struct {
	Slice   string
	Tipo    string // fuera_de_alcance | examen_tras_aprobar | no_arranco | sin_progreso
	Actor   string
	Detalle string
}

func (a Alerta) String() string {
	s := "S-" + a.Slice + " " + a.Tipo
	if a.Actor != "" {
		s += " " + a.Actor
	}
	return s + ": " + a.Detalle
}

// Alertas computes the drift of every open slice and journals each alert the first time it is seen.
func (p Proyecto) Alertas(ctx context.Context) ([]Alerta, error) {
	r := p.Slices()
	ab, err := r.Abiertos()
	if err != nil {
		return nil, err
	}
	evs, err := diario.Leer(p.DiarioPath())
	if err != nil {
		return nil, err
	}
	now := p.ahora()
	var out []Alerta
	for _, e := range ab {
		if e.Esquema == 0 {
			continue // an orq 1.0.0 slice: this version does not read it
		}
		ws, err := p.alertasDeWorkers(ctx, e, evs, now)
		if err != nil {
			return nil, err
		}
		out = append(out, ws...)
		_, al, err := r.Alinear(e.NN)
		if err != nil {
			return nil, err
		}
		if len(al.Fuera) > 0 {
			out = append(out, Alerta{Slice: e.NN, Tipo: "fuera_de_alcance", Detalle: strings.Join(al.Fuera, ", ")})
		}
		if e.Tarjeta.EstaAprobada() {
			for _, ev := range evs {
				if ev.Tipo == "examen" && ev.Slice == e.NN && ev.TS.After(e.Tarjeta.Aprobada) {
					out = append(out, Alerta{Slice: e.NN, Tipo: "examen_tras_aprobar", Detalle: ev.TS.UTC().Format(time.RFC3339) + " · " + ev.Detalle})
				}
			}
		}
	}
	vistas := map[Alerta]bool{}
	for _, ev := range evs {
		if ev.Tipo == "alerta" {
			vistas[Alerta{Slice: ev.Slice, Tipo: ev.Paso, Actor: ev.Actor, Detalle: ev.Detalle}] = true
		}
	}
	for _, a := range out {
		if vistas[a] {
			continue
		}
		vistas[a] = true
		if err := p.anotar(diario.Evento{Tipo: "alerta", Paso: a.Tipo, Slice: a.Slice, Actor: a.Actor, Detalle: a.Detalle}); err != nil {
			return out, err
		}
	}
	return out, nil
}

// alertasDeWorkers flags a worker that did not start within PlazoArranque of its launch, and a
// started one, still registered, with no commit and no journal event for alertas.sin_progreso_min.
func (p Proyecto) alertasDeWorkers(ctx context.Context, e *slice.Estado, evs []diario.Evento, now time.Time) ([]Alerta, error) {
	type lanzado struct {
		intento, arranque time.Time
		contrato          bool
		ultimo            time.Time
	}
	ws := map[string]*lanzado{}
	var orden []string
	for _, ev := range evs {
		if ev.Actor == "" || !strings.HasPrefix(ev.Actor, "worker-") {
			continue
		}
		w := ws[ev.Actor]
		if ev.Tipo == "lanzamiento" && ev.Slice == e.NN && ev.Intento == e.Intento {
			if w == nil {
				w = &lanzado{}
				ws[ev.Actor] = w
				orden = append(orden, ev.Actor)
			}
			switch {
			case ev.Paso == "intento":
				w.intento, w.arranque = ev.TS, time.Time{}
			case ev.Paso == "arranque" && (ev.Resultado == "ok" || ev.Resultado == "ya_estaba"):
				w.arranque = ev.TS
			case strings.HasPrefix(ev.Paso, "contrato") && ev.Resultado == "ok":
				w.contrato = true
			}
		}
		if w != nil && ev.TS.After(w.ultimo) {
			w.ultimo = ev.TS
		}
	}
	ses, err := canal.LoadSesiones(p.SesPath())
	if err != nil {
		return nil, err
	}
	plazo := time.Duration(p.Cfg.Alertas.SinProgresoMin) * time.Minute
	var out []Alerta
	for _, nombre := range orden {
		w := ws[nombre]
		if w.arranque.IsZero() || !w.contrato {
			if !w.intento.IsZero() && now.Sub(w.intento) >= PlazoArranque {
				out = append(out, Alerta{Slice: e.NN, Tipo: "no_arranco", Actor: nombre,
					Detalle: fmt.Sprintf("lanzado %s sin arrancar ni recibir el contrato", w.intento.Local().Format("15:04:05"))})
			}
			continue
		}
		if _, vive := ses[nombre]; !vive || e.Fase != slice.Implementando {
			continue // a closed worker makes no more progress by design
		}
		ultimo := w.ultimo
		if c := ultimoCommit(ctx, clonDe(e, nombre)); c.After(ultimo) {
			ultimo = c
		}
		if now.Sub(ultimo) >= plazo {
			out = append(out, Alerta{Slice: e.NN, Tipo: "sin_progreso", Actor: nombre,
				Detalle: fmt.Sprintf("sin commit ni evento desde %s", ultimo.Local().Format("2006-01-02 15:04"))})
		}
	}
	return out, nil
}

// clonDe is the copy a worker writes in: its team clone, or the slice's own.
func clonDe(e *slice.Estado, nombre string) string {
	for _, w := range e.Equipo {
		if w.Nombre == nombre {
			return w.Worktree
		}
	}
	return e.Worktree
}

func ultimoCommit(ctx context.Context, dir string) time.Time {
	out, err := gitx.Salida(ctx, dir, "log", "-1", "--format=%cI")
	if err != nil {
		return time.Time{}
	}
	t, _ := time.Parse(time.RFC3339, out)
	return t
}
