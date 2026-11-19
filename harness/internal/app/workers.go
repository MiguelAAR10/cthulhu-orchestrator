package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"time"

	"orq/internal/agente"
	"orq/internal/canal"
	"orq/internal/config"
	"orq/internal/gitx"
	"orq/internal/limits"
	"orq/internal/orqdir"
	"orq/internal/plan"
	"orq/internal/slice"
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
		if _, err := plan.Tomar(pl, tareaPl.ID, nombre); err != nil {
			return "", false, err
		}
		if err := plan.Guardar(p.PlanPath(e.NN), pl); err != nil {
			return "", false, err
		}
		// Si el worker no llega a arrancar, la tarea vuelve a pendiente: si no, nadie puede relanzarla.
		defer func() {
			if arranco {
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
	RegistrarLlamador(ses, os.Getenv("ORQ_ROL"), p.Term.PanelActual(), p.ahora())
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
	if err := canal.SaveSesiones(p.SesPath(), ses); err != nil {
		return nombre, false, err
	}
	if tareaPl != nil {
		contrato += fmt.Sprintf("\nTarea del plan: %s (%s). Al terminar avisa al Orquestador; él la marca hecha con `orq plan hecha %s`.\n", tareaPl.ID, tareaPl.Dificultad, tareaPl.ID)
	}
	prompt := contrato + "\nAntes de empezar corre `orq skill worker` y sigue esa guía. Cuando termines, pide verificación por el canal.\n"
	paso := "contrato"
	if tareaPl != nil {
		paso += ":" + tareaPl.ID // una tarea nueva para un worker vivo es un contrato nuevo
	}
	entregado, err := p.avisarUnaVez(ctx, nombre, paso, nombre, prompt, e.NN, e.Intento)
	if err != nil {
		return nombre, false, fmt.Errorf("worker abierto pero no recibió el contrato: %w", err)
	}
	return nombre, res == terminal.YaEstaba && entregado, nil
}
