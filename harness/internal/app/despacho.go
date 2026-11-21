package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"orq/internal/canal"
	"orq/internal/config"
	"orq/internal/diario"
	"orq/internal/orqdir"
	"orq/internal/plan"
	"orq/internal/slice"
	"orq/internal/terminal"
)

// Defaults of a worker's contract, the same as `orq slice lanzar`.
const (
	AlcancePorDefecto     = "solo lo necesario para que el examen pase"
	PresupuestoPorDefecto = "45 min"
)

// NombreWorker is the name LanzarWorker gives a worker: by zone when it has one, else by agent.
func NombreWorker(agente, nn, zona string) string {
	if zona != "" {
		return fmt.Sprintf("worker-%s-s%s", zona, nn)
	}
	return fmt.Sprintf("worker-%s-s%s", agente, nn)
}

// Kinds of Pendiente: who owes what.
const (
	PendFaltaLanzar  = "falta_lanzar"        // approved slice, agreed plan, ready tasks, nobody on them
	PendCortado      = "lanzamiento_cortado" // task taken, its worker never got the contract
	PendReintento    = "reintento"           // task en_curso from an earlier attempt: the orchestrator decides
	PendPausa        = "pausa"               // ready tasks, but the project or story is paused
	PendSinPlan      = "sin_plan"            // approved slice (full route) with no plan and no worker
	PendSinDebate    = "plan_sin_debatir"    // plan loaded, Planner and Astra have not agreed
	PendFixSinWorker = "fix_sin_worker"      // a fix with nobody writing it
	PendTarjeta      = "tarjeta"             // card waits for the Owner's approval
	PendDecision     = "decision"            // verified slice waits for the Owner's decision
	PendBloqueado    = "bloqueado"           // blocked slice waits for the orchestrator
	PendCierre       = "cierre"              // a close that stopped half way
	PendEntrega      = "entrega"             // message saved, delivery not confirmed
	PendRespuesta    = "respuesta"           // question nobody answered
)

// Pendiente is one thing someone owes, since when.
type Pendiente struct {
	Tipo   string
	Debe   string // orquestador, owner, planificador, a role or a worker
	Slice  string // NN, or "" for messages
	Titulo string
	Desde  time.Time
	Tareas []string // plan task ids
	Worker string
	Ref    string // message id
	Texto  string // short detail
}

// contratoEntregado says whether the worker got the contract of that task in this attempt.
func contratoEntregado(evs []diario.Evento, worker, nn string, intento int, tarea string) bool {
	return slices.ContainsFunc(evs, func(ev diario.Evento) bool {
		return ev.Tipo == "lanzamiento" && ev.Actor == worker && ev.Slice == nn && ev.Intento == intento &&
			ev.Paso == "contrato:"+tarea && ev.Resultado == "ok"
	})
}

// intentoDeTarea is the latest attempt with a trace of that task for that worker (its contract or
// a despacho intent); 0 when there is none.
func intentoDeTarea(evs []diario.Evento, worker, nn, tarea string) int {
	k := 0
	for _, ev := range evs {
		if ev.Slice != nn || ev.Intento <= k {
			continue
		}
		contrato := ev.Tipo == "lanzamiento" && ev.Actor == worker && ev.Paso == "contrato:"+tarea
		intento := ev.Tipo == "despacho" && ev.Paso == "intento" && fmt.Sprint(ev.Datos["tarea"]) == tarea
		if contrato || intento {
			k = ev.Intento
		}
	}
	return k
}

// Kinds of a task en_curso, for the current attempt.
const (
	tareaTrabaja   = iota // its worker got the contract in this attempt
	tareaCortada          // taken in this attempt (or with no trace), contract never delivered
	tareaReintento        // taken in an earlier attempt: after reintentar the orchestrator decides
)

func claseEnCurso(evs []diario.Evento, e *slice.Estado, t plan.Tarea) int {
	if contratoEntregado(evs, t.Worker, e.NN, e.Intento, t.ID) {
		return tareaTrabaja
	}
	if k := intentoDeTarea(evs, t.Worker, e.NN, t.ID); k > 0 && k < e.Intento {
		return tareaReintento
	}
	return tareaCortada
}

// trabajaEnElSlice says whether anyone writes the slice without a plan: a writer, a team member or a
// worker session of the slice.
func trabajaEnElSlice(e *slice.Estado, ses map[string]canal.Sesion) bool {
	if e.Escritor != "" || len(e.Equipo) > 0 {
		return true
	}
	for k := range ses {
		if strings.HasPrefix(k, "worker-") && strings.HasSuffix(k, "-s"+e.NN) {
			return true
		}
	}
	return false
}

// desdeFase is when the slice entered its current phase, from the journal (Abierto otherwise).
func desdeFase(e *slice.Estado, evs []diario.Evento) time.Time {
	if u, ok := diario.Ultimo(evs, func(ev diario.Evento) bool {
		return ev.Tipo == "fase" && ev.Slice == e.NN && ev.A == e.Fase
	}); ok {
		return u.TS
	}
	return e.Abierto
}

// AbrirLectura opens the project of cwd without creating anything: no .orq, no terminal. ok is
// false when the project has no .orq yet.
func AbrirLectura() (p Proyecto, ok bool, err error) {
	root, err := orqdir.Root()
	if err != nil {
		return Proyecto{}, false, errors.New("orq: no estás dentro de un repositorio git")
	}
	dir := filepath.Join(root, ".orq")
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		return Proyecto{Root: root, Dir: dir}, false, nil
	}
	cfg, err := config.Cargar(root)
	if err != nil {
		return Proyecto{}, false, fmt.Errorf("orq: %w. Revísala con `orq config ver`", err)
	}
	return Proyecto{Root: root, Dir: dir, Cfg: cfg, Ahora: time.Now}, true, nil
}

// Pendientes lists who owes what since when, the oldest first. It only reads: no journal line,
// no migration, no terminal call.
func (p Proyecto) Pendientes() ([]Pendiente, error) {
	r := p.Slices()
	ab, err := r.Leidos()
	if err != nil {
		return nil, err
	}
	evs, err := diario.Leer(p.DiarioPath())
	if err != nil {
		return nil, err
	}
	ses, err := canal.LoadSesiones(p.SesPath())
	if err != nil {
		return nil, err
	}
	var out []Pendiente
	for _, e := range ab {
		base := Pendiente{Slice: e.NN, Titulo: e.Titulo}
		add := func(x Pendiente) { out = append(out, x) }
		if err := e.CierrePendiente(); err != nil && e.Fase != slice.Integrando && e.Fase != slice.Cerrando {
			// A close the Owner decided stopped half way: that is what is owed, not more work.
			x := base
			x.Tipo, x.Debe, x.Desde, x.Texto = PendCierre, "orquestador", e.Cierre.TS, e.Cierre.Resultado
			add(x)
			continue
		}
		switch e.Fase {
		case slice.Verificado:
			x := base
			x.Tipo, x.Debe, x.Desde = PendDecision, "owner", desdeFase(e, evs)
			if e.Recibo != nil {
				x.Desde = e.Recibo.TS
			}
			add(x)
			continue
		case slice.Bloqueado:
			x := base
			x.Tipo, x.Debe, x.Desde, x.Texto = PendBloqueado, "orquestador", desdeFase(e, evs), e.Motivo
			add(x)
			continue
		case slice.Integrando, slice.Cerrando:
			x := base
			x.Tipo, x.Debe, x.Desde = PendCierre, "orquestador", desdeFase(e, evs)
			add(x)
			continue
		case slice.Implementando:
		default:
			continue
		}
		if err := e.PuedeLanzar(); err != nil {
			if e.Tarjeta != nil && !e.Tarjeta.Lista.IsZero() {
				x := base
				x.Tipo, x.Debe, x.Desde = PendTarjeta, "owner", e.Tarjeta.Lista
				add(x)
			}
			continue
		}
		desde := e.Abierto
		if e.Tarjeta.EstaAprobada() {
			desde = e.Tarjeta.Aprobada
		}
		base.Desde = desde
		pl, errP := plan.Leer(p.PlanPath(e.NN))
		switch {
		case errors.Is(errP, fs.ErrNotExist):
			if trabajaEnElSlice(e, ses) {
				continue
			}
			x := base
			x.Tipo, x.Debe = PendSinPlan, "planificador"
			if e.RutaEfectiva() == slice.RutaFix {
				x.Tipo, x.Debe = PendFixSinWorker, "orquestador"
			}
			add(x)
			continue
		case errP != nil:
			return nil, fmt.Errorf("S-%s: %w", e.NN, errP)
		case !pl.Debatido:
			x := base
			x.Tipo, x.Debe = PendSinDebate, "planificador y astra"
			add(x)
			continue
		}
		for _, t := range pl.Tareas {
			if t.Estado != "en_curso" || t.Worker == "" {
				continue
			}
			switch claseEnCurso(evs, e, t) {
			case tareaCortada:
				x := base
				x.Tipo, x.Debe, x.Tareas, x.Worker = PendCortado, "orquestador", []string{t.ID}, t.Worker
				add(x)
			case tareaReintento:
				x := base
				x.Tipo, x.Debe, x.Tareas, x.Worker = PendReintento, "orquestador", []string{t.ID}, t.Worker
				x.Texto = fmt.Sprintf("a%d", intentoDeTarea(evs, t.Worker, e.NN, t.ID))
				add(x)
			}
		}
		listas, _ := plan.Listas(pl)
		if len(listas) == 0 {
			continue
		}
		x := base
		x.Tipo, x.Debe = PendFaltaLanzar, "orquestador"
		for _, t := range listas {
			x.Tareas = append(x.Tareas, t.ID)
		}
		if motivo, pausa := r.PausaGlobal(); pausa {
			x.Tipo, x.Debe, x.Texto = PendPausa, "owner", motivo
		} else if err := r.Pausado(e); err != nil {
			x.Tipo, x.Debe, x.Texto = PendPausa, "owner", err.Error()
		}
		add(x)
	}
	ms, err := canal.Load(p.CanalPath())
	if err != nil {
		return nil, err
	}
	respondidas := map[string]bool{}
	for _, m := range ms {
		if m.Ref != "" {
			respondidas[m.Ref] = true
		}
	}
	for _, m := range ms {
		if m.Estado == "pendiente" {
			out = append(out, Pendiente{Tipo: PendEntrega, Debe: m.Para, Desde: m.TS, Ref: m.ID, Texto: m.De + "→" + m.Para + " " + m.Tipo})
		}
		if m.Tipo == "pregunta" && !respondidas[m.ID] {
			out = append(out, Pendiente{Tipo: PendRespuesta, Debe: m.Para, Desde: m.TS, Ref: m.ID, Texto: Recortar(m.Texto, 80)})
		}
	}
	slices.SortStableFunc(out, func(a, b Pendiente) int { return a.Desde.Compare(b.Desde) })
	return out, nil
}

// Despacho is what `despachar --una-vez` did with one task.
type Despacho struct {
	Slice, Tarea, Worker string
	Resultado            string // lanzado | adoptado | tarea_nueva | espera | fallo | desconocido
	Detalle              string
}

// DespacharUnaVez launches, once, the ready tasks of every approved slice with an agreed plan, through
// LanzarWorker. It is repeatable: a task already taken is not launched again, except when its
// worker never got the contract (a launch cut short), which LanzarWorker then reconciles with the
// terminal: a live agent is adopted, a missing one is started. Each launch writes its intent to the
// journal before starting and its result after.
func (p Proyecto) DespacharUnaVez(ctx context.Context, agente string) ([]Despacho, error) {
	if err := p.Dentro(); err != nil {
		return nil, err
	}
	r := p.Slices()
	ab, err := r.Abiertos()
	if err != nil {
		return nil, err
	}
	var out []Despacho
	var errs []error
	for _, e := range ab {
		if e.Fase != slice.Implementando || e.PuedeLanzar() != nil || r.Pausado(e) != nil {
			continue
		}
		vistos := map[string]bool{}
		for {
			pl, err := plan.Leer(p.PlanPath(e.NN))
			if errors.Is(err, fs.ErrNotExist) {
				break
			}
			if err != nil {
				errs = append(errs, fmt.Errorf("S-%s: %w", e.NN, err))
				break
			}
			if !pl.Debatido {
				break
			}
			evs, err := diario.Leer(p.DiarioPath())
			if err != nil {
				return out, errors.Join(append(errs, err)...)
			}
			t, nombre := siguienteTarea(pl, e, evs, agente, vistos)
			if t == nil {
				break
			}
			vistos[t.ID] = true
			d, err := p.despacharTarea(ctx, e, *t, nombre, agente)
			out = append(out, d)
			if err != nil {
				errs = append(errs, fmt.Errorf("S-%s %s: %w", e.NN, t.ID, err))
			}
		}
	}
	return out, errors.Join(errs...)
}

// siguienteTarea picks the next task to launch: first a taken task whose worker never got the
// contract in this attempt, then a ready one. A busy worker is refused under the launch lock.
func siguienteTarea(pl *plan.Plan, e *slice.Estado, evs []diario.Evento, agente string, vistos map[string]bool) (*plan.Tarea, string) {
	for i, t := range pl.Tareas {
		if t.Estado == "en_curso" && t.Worker != "" && !vistos[t.ID] && claseEnCurso(evs, e, t) == tareaCortada {
			return &pl.Tareas[i], t.Worker
		}
	}
	listas, _ := plan.Listas(pl)
	for _, t := range listas {
		if !vistos[t.ID] {
			return &t, NombreWorker(agente, e.NN, t.Zona)
		}
	}
	return nil, ""
}

func (p Proyecto) despacharTarea(ctx context.Context, e *slice.Estado, t plan.Tarea, nombre, agente string) (Despacho, error) {
	d := Despacho{Slice: e.NN, Tarea: t.ID, Worker: nombre}
	ev := diario.Evento{Tipo: "despacho", Paso: "intento", Slice: e.NN, Intento: e.Intento,
		Datos: map[string]any{"tarea": t.ID, "worker": nombre}}
	anotado := false
	// The intent is written under the launch lock, after the refusals: a busy worker or a full
	// machine leaves no line. What happened is decided there too (Lanzamiento), so two runs at the
	// same time never both record a launch.
	l, err := p.Lanzar(ctx, PedidoWorker{Slice: e.NN, Agente: agente, Tarea: t.ID,
		Alcance: AlcancePorDefecto, Presupuesto: PresupuestoPorDefecto, RechazarOcupado: true,
		Intencion: func(string) error {
			anotado = true
			return p.anotar(ev)
		}})
	switch {
	case errors.Is(err, ErrOcupado) || errors.Is(err, ErrSinCapacidad):
		d.Resultado, d.Detalle = "espera", err.Error()
		return d, nil
	case err == nil && l.Arranco:
		d.Resultado = "lanzado"
	case err == nil && l.Contrato && l.TareaNueva:
		d.Resultado = "tarea_nueva" // a live worker took a new task: a new contract
	case err == nil && l.Contrato:
		d.Resultado = "adoptado" // a cut-short launch whose agent was alive: adopted, contract delivered
	case err == nil:
		d.Resultado = "ya_estaba" // another run did it meanwhile: nothing new
	case errors.Is(err, terminal.ErrDesconocido):
		d.Resultado, d.Detalle = "desconocido", err.Error()
	default:
		d.Resultado, d.Detalle = "fallo", err.Error()
	}
	if !anotado {
		return d, err
	}
	ev.Paso, ev.Resultado, ev.Detalle = "resultado", d.Resultado, d.Detalle
	return d, errors.Join(err, p.anotar(ev))
}
