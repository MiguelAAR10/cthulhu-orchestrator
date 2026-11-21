package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"orq/internal/agente"
	"orq/internal/canal"
	"orq/internal/config"
	"orq/internal/gitx"
	"orq/internal/identidad"
	"orq/internal/limits"
	"orq/internal/orqdir"
	"orq/internal/perfil"
	"orq/internal/plan"
	"orq/internal/skilleval"
	"orq/internal/slice"
	"orq/internal/state"
	"orq/internal/terminal"
)

// PedidoWorker es lo que `orq slice lanzar` pide.
type PedidoWorker struct {
	Slice       string // NN; vacío = el único abierto
	Agente      string // claude | codex
	Zona        string
	Rutas       []string
	Alcance     string
	Suite       string
	Presupuesto string
	Dificultad  string
	Tarea       string // id del plan o «siguiente»
	// RechazarOcupado refuses a new task for a zone worker that is alive and still has another
	// task in progress (for automatic dispatch). Without it, as in 1.0.0, the worker gets it too.
	RechazarOcupado bool
}

// gitComun es el directorio git compartido de un worktree enlazado ("" si no se pudo leer).
func (p Proyecto) gitComun(ctx context.Context, dir string) string {
	out, err := gitx.Salida(ctx, dir, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return ""
	}
	return out
}

// LanzarWorker abre un worker en su propio panel y le entrega el contrato. Sin zona, escribe directo
// en la copia de trabajo del slice (trabajo pequeño). Con zona, se suma al equipo del slice con su
// propia rama y copia, y puede hablar con sus compañeros.
//
// Es repetible: si el worker ya existe no se lanza otro, y el contrato se entrega una sola vez.
func (p Proyecto) LanzarWorker(ctx context.Context, w PedidoWorker) (nombre string, yaEstaba bool, err error) {
	if err := p.Dentro(); err != nil {
		return "", false, err
	}
	if err := config.ValidarKind(w.Agente); err != nil {
		return "", false, fmt.Errorf("--agente: %w", err)
	}
	l, err := limits.Load(limits.DefaultPath())
	if err != nil {
		return "", false, err
	}
	r := p.Slices()
	e, err := r.Load(w.Slice)
	if err != nil || e == nil {
		return "", false, errors.Join(err, errors.New("no hay slice abierto"))
	}
	// orq2: one capacity count at a time in the project, held until the new pane exists, so two
	// launches in different slices never both take the last free slot.
	soltarCap := func() {}
	defer func() { soltarCap() }()
	if identidad.EsOrq2() {
		s, err := orqdir.Candado(filepath.Join(p.Dir, "capacidad"))
		if err != nil {
			return "", false, err
		}
		soltarCap = s
		// One launch per slice at a time: the team, the plan and the sessions are read and written by
		// one launcher, and a second launch of the same worker sees the first one's worker alive.
		// It also waits for a member's close (cierre-sNN): a launch never joins a team that is being
		// verified, and after the close it sees the new phase. Lock order, the same everywhere:
		// capacidad → lanzar-sNN → cierre-sNN → slice-NN → integracion → leaf locks (plan, sesiones, avisos,
		// rol-*, diario, canal), which are released without taking another.
		soltar, err := orqdir.Candado(filepath.Join(p.Dir, "lanzar-s"+e.NN))
		if err != nil {
			return "", false, err
		}
		defer soltar()
		soltarCierre, err := orqdir.Candado(filepath.Join(p.Dir, "cierre-s"+e.NN))
		if err != nil {
			return "", false, err
		}
		defer soltarCierre()
		if e, err = r.Load(e.NN); err != nil || e == nil {
			return "", false, errors.Join(err, errors.New("no hay slice abierto"))
		}
	}
	if err := r.Confiar(e); err != nil {
		return "", false, err
	}
	if err := e.PuedeLanzar(); err != nil {
		return "", false, err
	}
	if err := r.Pausado(e); err != nil {
		return "", false, err
	}
	skills, err := p.skillsDelWorker(e)
	if err != nil {
		return "", false, err
	}
	// Con plan, la tarea manda: zona, rutas y dificultad salen de ella, y queda en curso.
	var pl *plan.Plan
	var tareaPl *plan.Tarea
	zona, rutas, dificultad, alcance, tarea := w.Zona, w.Rutas, w.Dificultad, w.Alcance, w.Tarea
	if tarea != "" {
		if pl, err = plan.Leer(p.PlanPath(e.NN)); err != nil {
			return "", false, fmt.Errorf("S-%s no tiene plan: orq plan cargar archivo.json", e.NN)
		}
		if !pl.Debatido {
			return "", false, fmt.Errorf("S-%s plan not agreed yet: Planner and Astra must reach consensus first (orq plan debatido)", e.NN)
		}
		if tarea == "siguiente" {
			listas, bloq := plan.Listas(pl)
			if len(listas) == 0 {
				return "", false, fmt.Errorf("ninguna tarea lista; %d pendientes esperan: %v", pl.Pendientes(), bloq)
			}
			tarea = listas[0].ID
		}
		for i := range pl.Tareas {
			if pl.Tareas[i].ID == tarea {
				tareaPl = &pl.Tareas[i]
			}
		}
		if tareaPl == nil {
			return "", false, fmt.Errorf("no existe la tarea %s", tarea)
		}
		zona, rutas, dificultad = tareaPl.Zona, tareaPl.Rutas, tareaPl.Dificultad
		if tareaPl.Que != "" {
			alcance = tareaPl.Que + ". " + alcance
		}
	}
	nombre = fmt.Sprintf("worker-%s-s%s", w.Agente, e.NN)
	if zona != "" {
		nombre = fmt.Sprintf("worker-%s-s%s", zona, e.NN)
	}
	// ¿Ya existe? Se pregunta antes de reservar nada: repetir el comando no ocupa cupo ni toca el plan.
	vida, errV := p.Term.Estado(ctx, nombre)
	if errV != nil && !errors.Is(errV, terminal.ErrNoExiste) || vida == terminal.Desconocida {
		return nombre, false, fmt.Errorf("no se sabe si %s ya existe en %s (%w); no se lanza otro", nombre, p.Term.Nombre(), errors.Join(terminal.ErrDesconocido, errV))
	}
	existe := vida == terminal.Vivo
	if !existe {
		// La máquina decide cuántos caben: el tope configurado, la RAM disponible y la CPU.
		c, _, err := Capacidad(p.Cfg, l, p.medir())
		if err != nil {
			return "", false, fmt.Errorf("sin medir la máquina no se lanza: %w", err)
		}
		if !c.Cabe() {
			return "", false, errors.New(c.Motivo())
		}
	}
	if w.RechazarOcupado && existe && tareaPl != nil && tareaPl.Worker != nombre {
		for _, t := range pl.Tareas {
			if t.Estado == "en_curso" && t.Worker == nombre && t.ID != tareaPl.ID {
				return nombre, false, fmt.Errorf("%s sigue con %s: %s espera a que termine", nombre, t.ID, tareaPl.ID)
			}
		}
	}
	modelo, esfuerzo := p.Cfg.Worker(w.Agente, dificultad)
	// Repetición pura: el worker vive y no trae tarea nueva. No se toca el equipo ni el plan.
	repeticion := existe && (tareaPl == nil || tareaPl.Worker == nombre)
	dir := e.Worktree
	if zona != "" {
		i := slices.IndexFunc(e.Equipo, func(m slice.Worker) bool { return m.Nombre == nombre })
		if repeticion && i >= 0 {
			dir = e.Equipo[i].Worktree
		} else {
			var miembro *slice.Worker
			if e, miembro, err = r.Sumar(e.NN, nombre, w.Agente, zona, rutas); err != nil {
				return "", false, err
			}
			dir = miembro.Worktree
		}
	} else if !repeticion {
		if _, err := r.Tomar(e.NN, nombre, false); err != nil {
			return "", false, err
		}
	}
	arranco := existe
	if tareaPl != nil && tareaPl.Worker != nombre {
		if err := p.tomarTarea(e.NN, pl, tareaPl.ID, nombre); err != nil {
			return "", false, err
		}
		// Si el worker no llega a arrancar, la tarea vuelve a pendiente: si no, nadie puede relanzarla.
		defer func() {
			if arranco {
				return
			}
			if identidad.EsOrq2() {
				_, errS := plan.Actualizar(p.PlanPath(e.NN), func(f *plan.Plan) error {
					_, err := plan.Soltar(f, tareaPl.ID)
					return err
				})
				err = errors.Join(err, errS)
				return
			}
			if pl2, errL := plan.Leer(p.PlanPath(e.NN)); errL == nil {
				if _, errS := plan.Soltar(pl2, tareaPl.ID); errS == nil {
					err = errors.Join(err, plan.Guardar(p.PlanPath(e.NN), pl2))
				}
			}
		}()
	}
	contrato, err := p.Contrato(e.NN, nombre, alcance, w.Suite, w.Presupuesto)
	if err != nil {
		return "", false, err
	}
	ses, err := canal.LoadSesiones(p.SesPath())
	if err != nil {
		return "", false, err
	}
	llamador := os.Getenv("ORQ_ROL")
	RegistrarLlamador(ses, llamador, p.Term.PanelActual(), p.ahora())
	s := sesionNueva{Nombre: nombre, Kind: w.Agente, Cwd: dir, Slice: e.NN, Intento: e.Intento,
		Args: agente.ArgsWorker(w.Agente, modelo, esfuerzo, nombre, p.gitComun(ctx, dir)),
		Env:  entornoDe(nombre, p.Cfg.WorkerEntorno(w.Agente, dificultad))}
	otro, hayOtro := PanelDelSlice(ses, e.NN)
	if hayOtro && otro.Agente != nombre {
		s.Junto = otro.Pane
	} else {
		s.Etiqueta = fmt.Sprintf("S-%s · %s", e.NN, orqdir.Slug(p.Root))
	}
	panel, res, err := p.abrirSesion(ctx, s)
	soltarCap() // the pane exists (or failed): the next count sees it
	soltarCap = func() {}
	if err != nil {
		// Desconocido: el agente pudo arrancar; la tarea no se suelta hasta saber.
		arranco = errors.Is(err, terminal.ErrDesconocido)
		return nombre, false, err
	}
	arranco = true
	if panel.Pestana == "" {
		panel.Pestana = otro.Tab
	}
	ses[nombre] = canal.Sesion{Agente: nombre, Pane: panel.ID, Tab: panel.Pestana, Desde: p.ahora().UTC().Truncate(time.Second), Gen: 1}
	claves := []string{nombre}
	if _, ok := ses[llamador]; ok && llamador != nombre {
		claves = append(claves, llamador)
	}
	if err := p.guardarSesiones(ses, claves...); err != nil {
		return nombre, false, err
	}
	if tareaPl != nil {
		contrato += fmt.Sprintf("\nTarea del plan: %s (%s). Al terminar avisa al Orquestador; él la marca hecha con `orq plan hecha %s`.\n", tareaPl.ID, tareaPl.Dificultad, tareaPl.ID)
	}
	prompt := contrato + skills + "\nAntes de empezar corre `orq skill worker` y sigue esa guía. Cuando termines, pide verificación por el canal.\n"
	paso := "contrato"
	if tareaPl != nil {
		paso += ":" + tareaPl.ID // una tarea nueva para un worker vivo es un contrato nuevo
	}
	entregado, err := p.avisarUnaVez(ctx, nombre, paso, nombre, panel.ID, prompt, e.NN, e.Intento)
	if err != nil {
		return nombre, false, fmt.Errorf("worker abierto pero no recibió el contrato: %w", err)
	}
	return nombre, res == terminal.YaEstaba && entregado, nil
}

// tomarTarea marks the task in progress for nombre. orq2 takes it on the plan on disk, under its
// lock: a task taken meanwhile by someone else is refused, and no other change to the plan is lost.
func (p Proyecto) tomarTarea(nn string, pl *plan.Plan, id, nombre string) error {
	if !identidad.EsOrq2() {
		if _, err := plan.Tomar(pl, id, nombre); err != nil { // orq keeps 1.0.0
			return err
		}
		return plan.Guardar(p.PlanPath(nn), pl)
	}
	_, err := plan.Actualizar(p.PlanPath(nn), func(f *plan.Plan) error {
		_, err := plan.Tomar(f, id, nombre)
		return err
	})
	return err
}

// skillsDelWorker es el párrafo del contrato con las skills del proyecto para el tipo de tarea del
// slice, de mejor a peor nota, y las que quedan fuera por skills.minimo.
func (p Proyecto) skillsDelWorker(e *slice.Estado) (string, error) {
	pf := p.Perfil()
	tipo := e.Tipo
	if tipo == "" {
		tipo = pf.Tipo()
	}
	var nombres []string
	for _, s := range perfil.ParaRol(perfil.Lista(pf), "worker") {
		nombres = append(nombres, s.Nombre)
	}
	// La lista de notas es un consejo: un archivo roto no frena el lanzamiento.
	ns, malas, err := skilleval.LeerNotas(state.Dir())
	if err != nil {
		fmt.Fprintf(os.Stderr, "orq: aviso: notas de skills ilegibles, se lanza sin notas: %v\n", err)
		ns = nil
	}
	for _, m := range malas {
		fmt.Fprintf(os.Stderr, "orq: aviso: línea de notas ignorada: %v\n", m)
	}
	hashes := map[string]string{}
	for _, n := range nombres {
		if h, err := skilleval.Hash(filepath.Join(p.Root, ".claude", "skills", n)); err == nil {
			hashes[n] = h
		}
	}
	elegidas, fuera := skilleval.Ordenar(nombres, ns, hashes, tipo, p.Cfg.Skills.Minimo)
	lista := func(es []skilleval.Eleccion) string {
		var ps []string
		for _, el := range es {
			if el.Nota == nil {
				ps = append(ps, el.Skill+" sin nota")
			} else {
				ps = append(ps, fmt.Sprintf("%s %.2f", el.Skill, *el.Nota))
			}
		}
		return strings.Join(ps, ", ")
	}
	txt := fmt.Sprintf("\nSkills para el tipo %s, de mejor a peor nota: %s.\n", tipo, lista(elegidas))
	if len(fuera) > 0 {
		txt += fmt.Sprintf("Fuera por nota bajo skills.minimo %.2f: %s.\n", *p.Cfg.Skills.Minimo, lista(fuera))
	}
	return txt, nil
}
