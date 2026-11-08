package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"orq/internal/gate"
	"orq/internal/herdr"
	"orq/internal/limits"
	"orq/internal/orqdir"
	"orq/internal/plan"
	"orq/internal/sysinfo"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"orq/internal/canal"
	"orq/internal/memoria"
	"orq/internal/slice"
)

// memVigente es la memoria que vale hoy: sin reemplazados ni vencidos con slices cerrados.
func (p proyecto) memVigente() []memoria.Item {
	items, _ := memoria.Load(p.memPath())
	cerrados := map[string]bool{}
	raw, _ := os.ReadFile(filepath.Join(p.dir, "slices.jsonl"))
	for _, l := range strings.Split(string(raw), "\n") {
		var r slice.Registro
		if json.Unmarshal([]byte(l), &r) == nil && r.NN != "" {
			cerrados["S-"+r.NN] = true
		}
	}
	return memoria.SinVencer(memoria.Vigentes(items), cerrados)
}

// Relevo son los datos duros que una sesión nueva necesita, sin pasar por un modelo.
type Relevo struct {
	Rol         string   `json:"rol"`
	Objetivo    string   `json:"objetivo"`
	Restriccion []string `json:"restricciones"` // lo crítico del Owner, intacto
	Decisiones  []string `json:"decisiones"`
	Hipotesis   []string `json:"hipotesis,omitempty"`
	Slice       string   `json:"slice,omitempty"` // S-NN/aK · fase
	Worktree    string   `json:"worktree,omitempty"`
	Escritor    string   `json:"escritor,omitempty"`
	Head        string   `json:"head,omitempty"`
	Comprobado  string   `json:"comprobado"`
	Falla       string   `json:"falla,omitempty"`
	Reproducir  string   `json:"reproducir,omitempty"`
	Siguiente   string   `json:"siguiente"`
	Pendientes  []string `json:"mensajes_pendientes,omitempty"`
	Referencias []string `json:"referencias"`
}

func (p proyecto) datosRelevo(rol string) Relevo {
	rv := Relevo{Rol: rol, Objetivo: "(sin historia en curso)", Comprobado: "nada todavía",
		Referencias: []string{"orq slice estado", "orq slice recibo", "orq mem ver", "orq msg leer"}}
	ms, _ := canal.Load(p.canalPath())
	ms = canal.Vigentes(ms)
	for _, m := range ms {
		if m.Tipo == "historia" {
			rv.Objetivo = recortar(strings.ReplaceAll(m.Texto, "\n", " "), 300) + " (" + m.ID + ")"
		}
	}
	for _, m := range canal.Pendientes(ms) {
		rv.Pendientes = append(rv.Pendientes, fmt.Sprintf("%s %s→%s %s", m.ID, m.De, m.Para, m.Tipo))
	}
	for _, it := range memoria.Sorted(p.memVigente()) {
		switch {
		case it.Imp == "critica":
			rv.Restriccion = append(rv.Restriccion, it.Texto)
		case it.TipoEfectivo() == "decision":
			rv.Decisiones = append(rv.Decisiones, it.ID+" "+it.Texto)
		case it.TipoEfectivo() == "hipotesis" && len(rv.Hipotesis) < 5:
			rv.Hipotesis = append(rv.Hipotesis, it.ID+" "+it.Texto)
		}
	}
	ab, _ := p.slices().Abiertos()
	if len(ab) > 1 {
		var xs []string
		for _, x := range ab {
			xs = append(xs, fmt.Sprintf("%s · %s · %s · escritor %s", x.Attempt(), x.Fase, x.Worktree, valor(x.Escritor)))
		}
		rv.Slice = strings.Join(xs, " | ")
		rv.Siguiente = "varios slices abiertos: `orq slice estado` y trabaja cada uno con --slice NN"
		return rv
	}
	var e *slice.Estado
	if len(ab) == 1 {
		e = ab[0]
	}
	if e == nil {
		rv.Siguiente = "sin slice abierto: " + strings.Join(func() []string { f, d := p.fase(); return []string{f, d} }(), " · ")
		return rv
	}
	rv.Slice, rv.Worktree, rv.Escritor = e.Attempt()+" «"+e.Titulo+"» · "+e.Fase, e.Worktree, valor(e.Escritor)
	if h, err := slice.Git(e.Worktree, "rev-parse", "--short", "HEAD"); err == nil {
		rv.Head = h
	}
	rv.Reproducir = fmt.Sprintf("cd %s && %s", e.Worktree, e.Cmd)
	if e.Recibo != nil {
		rv.Comprobado = fmt.Sprintf("recibo %s en %s: exit %d, %d ejecutadas", e.Recibo.Attempt, e.Recibo.Head[:8], e.Recibo.Exit, e.Recibo.Ejecutados)
	}
	if e.Motivo != "" {
		rv.Falla = e.Motivo
	} else if e.Fallos > 0 {
		rv.Falla = fmt.Sprintf("%d verificación(es) fallida(s) en %s; salida en .orq/salidas/", e.Fallos, e.Attempt())
	}
	switch e.Fase {
	case slice.Implementando:
		rv.Siguiente = fmt.Sprintf("seguir en el MISMO worktree; escritor %s (un relevo usa `orq slice tomar <quien> --relevo`); luego `orq slice verificar`", valor(e.Escritor))
	case slice.Verificado:
		rv.Siguiente = "demo con el Owner vía Atena; después `orq slice cerrar --aceptado|--rechazado`"
	case slice.Bloqueado:
		rv.Siguiente = "decidir: `orq slice reintentar \"qué cambia\"`, preguntar al Atena o rechazar"
	default:
		rv.Siguiente = "repetir `orq slice cerrar --aceptado`: continúa donde quedó"
	}
	return rv
}

func lista(xs []string, vacio string) string {
	if len(xs) == 0 {
		return "- " + vacio
	}
	return "- " + strings.Join(xs, "\n- ")
}

func (rv Relevo) Markdown(fecha, ahora string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Relevo · %s · %s\n\n", rv.Rol, fecha)
	fmt.Fprintf(&b, "## Objetivo\n- %s\n\n## Lo que pide el Owner (crítico, intacto)\n%s\n\n", rv.Objetivo, lista(rv.Restriccion, "(sin requisitos críticos)"))
	fmt.Fprintf(&b, "## Decisiones vigentes\n%s\n\n", lista(rv.Decisiones, "(ninguna)"))
	if rv.Slice != "" {
		fmt.Fprintf(&b, "## Slice\n- %s\n- worktree: %s (no crear otro)\n- escritor: %s · commit: %s\n\n", rv.Slice, rv.Worktree, rv.Escritor, rv.Head)
	}
	fmt.Fprintf(&b, "## Comprobado\n- %s\n\n", rv.Comprobado)
	if rv.Falla != "" {
		fmt.Fprintf(&b, "## Qué falla\n- %s\n- reproducir: `%s`\n\n", rv.Falla, rv.Reproducir)
	}
	if len(rv.Hipotesis) > 0 {
		fmt.Fprintf(&b, "## Hipótesis (sin comprobar: no son reglas)\n%s\n\n", lista(rv.Hipotesis, ""))
	}
	if len(rv.Pendientes) > 0 {
		fmt.Fprintf(&b, "## Mensajes sin entrega confirmada\n%s\n- `orq msg reconciliar`\n\n", lista(rv.Pendientes, ""))
	}
	fmt.Fprintf(&b, "## Siguiente acción\n- %s\n\n", rv.Siguiente)
	if ahora != "" {
		fmt.Fprintf(&b, "## Resumen del escriba (propuesto, verifícalo)\n%s\n\n", ahora)
	}
	fmt.Fprintf(&b, "## Referencias\n- `%s`\n", strings.Join(rv.Referencias, "` · `"))
	return b.String()
}

// contrato es el Paso C del research: lo mínimo que un worker necesita, con los datos reales del slice.
func (p proyecto) contrato(nn, worker, alcance, suite, presupuesto string) (string, error) {
	e, err := p.slices().Load(nn)
	if err != nil || e == nil {
		return "", errors.Join(err, errors.New("no hay slice abierto"))
	}
	rv := p.datosRelevo("orquestador")
	rv.Objetivo = strings.TrimSpace(rv.Objetivo)
	if worker == "" {
		worker = valor(e.Escritor)
	}
	var crit []string
	for _, it := range p.memVigente() {
		if it.Slice == e.ID() && it.Fuente == "scout" {
			crit = append(crit, fmt.Sprintf("%s %s (%s)", it.ID, it.Texto, it.TipoEfectivo()))
		}
	}
	wt, rama := e.Worktree, e.Rama
	var yo *slice.Worker
	for i := range e.Equipo {
		if e.Equipo[i].Nombre == worker {
			yo = &e.Equipo[i]
			wt, rama = yo.Worktree, yo.Rama
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "CONTRATO %s · worker: %s\n", e.Attempt(), worker)
	fmt.Fprintf(&b, "Objetivo del slice: %s\n", rv.Objetivo)
	fmt.Fprintf(&b, "Worktree: %s · rama %s · base %s@%s. No crees otro; comprueba `git rev-parse --show-toplevel`.\n", wt, rama, e.Base, e.SHARojo[:8])
	if yo != nil {
		fmt.Fprintf(&b, "Tu zona: %s → solo cambias %s. Lo de otra zona se lo pides a su dueño; si tocas fuera, `orq slice juntar` rechaza tu trabajo.\n", yo.Zona, strings.Join(yo.Rutas, ", "))
		var otros []string
		for _, o := range e.Equipo {
			if o.Nombre != worker {
				otros = append(otros, fmt.Sprintf("%s (%s: %s)", o.Nombre, o.Zona, strings.Join(o.Rutas, ", ")))
			}
		}
		if len(otros) > 0 {
			fmt.Fprintf(&b, "Tu equipo, trabajando a la vez: %s. Acuerden contratos (endpoints, tipos, datos) temprano: `orq msg send --de %s --para <worker> --tipo pregunta|respuesta|estado \"...\"`.\n", strings.Join(otros, "; "), worker)
		}
		b.WriteString("El examen de aceptación pasa cuando el Orquestador junta a todo el equipo; tú corre tus pruebas de zona. Commitea seguido: el Orquestador junta lo commiteado.\n")
	}
	fmt.Fprintf(&b, "Alcance: %s. Nada de refactors fuera de esto.\n", alcance)
	fmt.Fprintf(&b, "Aceptación: `%s` (protegido, igual que %s: no lo edites).\n", e.Cmd, strings.Join(e.Bundle, ", "))
	if suite != "" {
		fmt.Fprintf(&b, "Suite rápida: `%s`\n", suite)
	}
	fmt.Fprintf(&b, "Decisiones vigentes:\n%s\n", lista(rv.Decisiones, "(ninguna)"))
	fmt.Fprintf(&b, "Criterios de scouts:\n%s\n", lista(crit, "(ninguno)"))
	if len(rv.Restriccion) > 0 {
		fmt.Fprintf(&b, "Lo que exige el Owner:\n%s\n", lista(rv.Restriccion, ""))
	}
	fmt.Fprintf(&b, "Presupuesto: %s · el examen tolera %d reparaciones; después se bloquea.\n", presupuesto, slice.MaxFallos-1)
	b.WriteString("Cómo: TDD con tests propios fuera del paquete; commits pequeños; ante fallo, reproduce antes de cambiar. No borres código ajeno.\n")
	b.WriteString("Para y pregunta si: ambigüedad de producto, cambia alcance o aceptación, más presupuesto, riesgo (datos, dinero, seguridad, irreversible) o el examen parece mal.\n")
	b.WriteString("No hagas: merge, push, limpiar worktrees, abrir o cerrar slices, hablar con el Owner.\n")
	b.WriteString("Responde ≤12 líneas: PIDO VERIFICACIÓN | BLOQUEADO | PREGUNTA · head · archivos · qué probaste y salida · qué no · riesgos.\n")
	fmt.Fprintf(&b, "Canal: `orq msg send --de %s --para orquestador --tipo pregunta|estado \"...\"`. Guía completa: `orq skill worker`.\n", worker)
	return b.String(), nil
}

// lanzarWorker abre un worker en su propio panel de herdr y le entrega el contrato. Sin zona, escribe
// directo en el worktree del slice (trabajo pequeño). Con zona, se suma al equipo del slice con su
// propia rama y worktree, y puede hablar con sus compañeros.
func (p proyecto) lanzarWorker(nn, agente, zona string, rutas []string, alcance, suite, pres, dificultad, tarea string) (string, error) {
	if !herdr.Inside() {
		return "", errors.New("hay que correrlo dentro de herdr (panel del Orquestador)")
	}
	if agente != "claude" && agente != "codex" {
		return "", errors.New("--agente claude|codex")
	}
	l, err := limits.Load(limits.DefaultPath())
	if err != nil {
		return "", err
	}
	procs, _ := sysinfo.ListProcs()
	if n := len(sysinfo.Workers(procs, l.WorkerProcessName, gate.IsLinkedWorktree)); n >= *l.MaxWorkers {
		return "", fmt.Errorf("hay %d worker(s) vivos y el tope es %d; espera a que uno termine", n, *l.MaxWorkers)
	}
	r := p.slices()
	e, err := r.Load(nn)
	if err != nil || e == nil {
		return "", errors.Join(err, errors.New("no hay slice abierto"))
	}
	// Con plan, la tarea manda: zona, rutas y dificultad salen de ella, y queda en curso.
	var pl *plan.Plan
	var tareaPl *plan.Tarea
	if tarea != "" {
		if pl, err = plan.Leer(p.planPath(e.NN)); err != nil {
			return "", fmt.Errorf("S-%s no tiene plan: orq plan cargar archivo.json", e.NN)
		}
		if !pl.Debatido {
			return "", fmt.Errorf("S-%s plan not agreed yet: Planner and Astra must reach consensus first (orq plan debatido)", e.NN)
		}
		if tarea == "siguiente" {
			listas, bloq := plan.Listas(pl)
			if len(listas) == 0 {
				return "", fmt.Errorf("ninguna tarea lista; %d pendientes esperan: %v", pl.Pendientes(), bloq)
			}
			tarea = listas[0].ID
		}
		for i := range pl.Tareas {
			if pl.Tareas[i].ID == tarea {
				tareaPl = &pl.Tareas[i]
			}
		}
		if tareaPl == nil {
			return "", fmt.Errorf("no existe la tarea %s", tarea)
		}
		zona, rutas, dificultad = tareaPl.Zona, tareaPl.Rutas, tareaPl.Dificultad
		if tareaPl.Que != "" {
			alcance = tareaPl.Que + ". " + alcance
		}
	}
	modelo, esfuerzo := plan.Modelo(agente, dificultad)
	nombre := fmt.Sprintf("worker-%s-s%s", agente, e.NN)
	dir := e.Worktree
	var w *slice.Worker
	if zona != "" {
		nombre = fmt.Sprintf("worker-%s-s%s", zona, e.NN)
		if e, w, err = r.Sumar(e.NN, nombre, agente, zona, rutas); err != nil {
			return "", err
		}
		dir = w.Worktree
	} else if _, err := r.Tomar(e.NN, nombre, false); err != nil {
		return "", err
	}
	arranco := false
	if tareaPl != nil {
		// Reserva bajo candado: otro lanzar concurrente no pisa esta reserva (H-04c).
		if _, err := plan.Actualizar(p.planPath(e.NN), func(pl *plan.Plan) error { _, err := plan.Tomar(pl, tareaPl.ID, nombre); return err }); err != nil {
			return "", err
		}
		// Si el worker no llega a arrancar, la tarea vuelve a pendiente: si no, nadie puede relanzarla.
		defer func() {
			if arranco {
				return
			}
			plan.Actualizar(p.planPath(e.NN), func(pl2 *plan.Plan) error { _, err := plan.Soltar(pl2, tareaPl.ID); return err })
		}()
	}
	contrato, err := p.contrato(e.NN, nombre, alcance, suite, pres)
	if err != nil {
		return "", err
	}
	ses, _ := canal.LoadSesiones(p.sesPath())
	registrarLlamador(ses, os.Getenv("ORQ_ROL"), herdr.CurrentPane(), time.Now())
	env := map[string]string{"ORQ_ROL": nombre}
	var pane, tab string
	if otro, ok := panelDelSlice(ses, e.NN); ok {
		tab = otro.Tab
		pane, err = herdr.SplitDe(otro.Pane, dir, env)
	} else {
		tab, pane, err = herdr.Tab(fmt.Sprintf("S-%s · %s", e.NN, orqdir.Slug(p.root)), dir, env)
	}
	if err != nil {
		return "", err
	}
	if err := herdr.Start(nombre, agente, pane, workerArgs(agente, modelo, esfuerzo, nombre, dir)); err != nil {
		// Sin contrato el agente no sirve aunque haya arrancado tarde: se cierra su panel.
		if herdr.ClosePane(pane) != nil {
			return "", fmt.Errorf("%v (el panel %s quedó abierto; ciérralo)", err, pane)
		}
		return "", fmt.Errorf("%v (panel %s cerrado; la tarea volvió a pendiente)", err, pane)
	}
	arranco = true
	ses[nombre] = canal.Sesion{Agente: nombre, Pane: pane, Tab: tab, Desde: time.Now().UTC().Truncate(time.Second), Gen: 1}
	canal.SaveSesiones(p.sesPath(), ses)
	if tareaPl != nil {
		contrato += fmt.Sprintf("\nTarea del plan: %s (%s). Al terminar avisa al Orquestador; él la marca hecha con `orq plan hecha %s`.\n", tareaPl.ID, tareaPl.Dificultad, tareaPl.ID)
	}
	prompt := contrato + "\nAntes de empezar corre `orq skill worker` y sigue esa guía. Cuando termines, pide verificación por el canal.\n"
	if err := herdr.Prompt(nombre, prompt); err != nil {
		return nombre, fmt.Errorf("worker abierto pero no recibió el contrato: %v", err)
	}
	return nombre, nil
}

// registrarLlamador deja al rol que lanza (el Orquestador, que suele vivir en otro repo) como destino
// en las sesiones del repo del slice: así `orq msg send --para orquestador` del worker le llega directo.
// herdr acepta el id del panel como destino.
func registrarLlamador(ses map[string]canal.Sesion, rol, pane string, now time.Time) {
	if rol == "" || pane == "" || strings.HasPrefix(rol, "worker-") {
		return
	}
	if s, ok := ses[rol]; ok && s.Pane == pane {
		return
	}
	ses[rol] = canal.Sesion{Agente: pane, Pane: pane, Desde: now.UTC().Truncate(time.Second), Gen: ses[rol].Gen}
}

// workerArgs suma a un worker Codex lo que su tarea necesita sin pedir aprobación a cada paso:
// escribir en su worktree y en el git común (git add/commit en un worktree enlazado) y red, para
// las pruebas contra la base de prueba local. Lo demás sigue pidiendo aprobación.
func workerArgs(kind, modelo, esfuerzo, name, dir string) []string {
	args := agentArgs(kind, modelo, esfuerzo, name)
	if kind != "codex" {
		return args
	}
	args = append(args, "-s", "workspace-write", "-a", "on-request",
		"-c", "sandbox_workspace_write.network_access=true")
	if out, err := exec.Command("git", "-C", dir, "rev-parse", "--path-format=absolute", "--git-common-dir").Output(); err == nil {
		if common := strings.TrimSpace(string(out)); common != "" {
			args = append(args, "--add-dir", common)
		}
	}
	return args
}
