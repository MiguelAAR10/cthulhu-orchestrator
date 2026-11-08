package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"orq/internal/orqdir"
	"orq/internal/plan"
	"orq/internal/workers"
)

func (p proyecto) planPath(nn string) string { return plan.Path(p.dir, nn) }

// planDe carga el plan del slice; nn vacío = el único abierto.
func (p proyecto) planDe(nn string) (*plan.Plan, string, error) {
	e, err := p.slices().Load(nn)
	if err != nil || e == nil {
		return nil, "", fmt.Errorf("no hay slice abierto")
	}
	path := p.planPath(e.NN)
	pl, err := plan.Leer(path)
	if err != nil {
		return nil, path, fmt.Errorf("S-%s no tiene plan: orq plan cargar archivo.json", e.NN)
	}
	return pl, path, nil
}

func runPlan(args []string, stdout, stderr io.Writer) int {
	p, err := abrirProyecto()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	nn := ""
	for i := 0; i < len(args); i++ {
		if args[i] == "--slice" && i+1 < len(args) {
			nn = strings.TrimPrefix(args[i+1], "S-")
			args = append(args[:i:i], args[i+2:]...)
			break
		}
	}
	if len(args) == 0 {
		args = []string{"ver"}
	}
	uso := "uso: orq plan [--slice NN] cargar ARCHIVO.json | ver | listas | revisar | hecha ID | soltar ID | debatido"
	switch args[0] {
	case "cargar":
		if len(args) != 2 {
			fmt.Fprintln(stderr, uso)
			return 1
		}
		e, err := p.slices().Load(nn)
		if err != nil || e == nil {
			fmt.Fprintln(stderr, "orq plan: no hay slice abierto; el plan pertenece a un slice")
			return 1
		}
		pl, err := plan.Leer(args[1])
		if err != nil {
			fmt.Fprintln(stderr, "orq plan:", err)
			return 1
		}
		pl.Slice = e.NN
		os.MkdirAll(filepath.Dir(p.planPath(e.NN)), 0o755)
		soltar, err := orqdir.Candado(p.planPath(e.NN)) // mismo candado que plan.Actualizar
		if err != nil {
			fmt.Fprintln(stderr, "orq plan:", err)
			return 1
		}
		defer soltar()
		var kept, reabiertas []string
		if viejo, err := plan.Leer(p.planPath(e.NN)); err == nil {
			kept, reabiertas = plan.Conservar(pl, viejo)
		}
		err = plan.Validar(pl, e.Alcance)
		if ferr := plan.ValidarFronts(pl, maxFronts()); ferr != nil {
			err = errors.Join(err, ferr)
		}
		if err != nil {
			fmt.Fprintf(stderr, "orq plan: el plan no sirve así:\n%s\n", err)
			return 1
		}
		if err := plan.Guardar(p.planPath(e.NN), pl); err != nil {
			fmt.Fprintln(stderr, "orq plan:", err)
			return 1
		}
		listas, _ := plan.Listas(pl)
		fmt.Fprintf(stdout, "S-%s: plan de %d tareas guardado; %d listas para despachar: orq slice lanzar --tarea ID\n", e.NN, len(pl.Tareas), len(listas))
		if len(kept) > 0 {
			fmt.Fprintf(stdout, "avance conservado: %s\n", strings.Join(kept, ", "))
		}
		if len(reabiertas) > 0 {
			fmt.Fprintf(stdout, "vuelven a pendiente (su contrato cambió; lo hecho no prueba lo nuevo): %s\n", strings.Join(reabiertas, ", "))
		}
		if !pl.Debatido {
			fmt.Fprintln(stdout, "plan sin debatir en esta revisión: Planificador y Astra deben acordarlo (orq plan debatido)")
		}
		// Una tarea ya lanzada que creció: su worker recibe las rutas nuevas, o juntar las rechaza.
		for _, t := range pl.Tareas {
			if t.Estado == "pendiente" {
				continue
			}
			if _, err := p.slices().AmpliarZona(e.NN, t.Zona, t.Rutas); err != nil && !strings.Contains(err.Error(), "no tiene worker") {
				fmt.Fprintf(stderr, "aviso: zona %s de %s: %v\n", t.Zona, t.ID, err)
			}
		}
		return 0
	case "ver":
		pl, _, err := p.planDe(nn)
		if err != nil {
			fmt.Fprintln(stderr, "orq plan:", err)
			return 1
		}
		mostrarPlan(pl, stdout)
		return 0
	case "hecha":
		if len(args) != 2 {
			fmt.Fprintln(stderr, uso)
			return 1
		}
		_, path, err := p.planDe(nn)
		if err != nil {
			fmt.Fprintln(stderr, "orq plan:", err)
			return 1
		}
		var t *plan.Tarea
		pl, err := plan.Actualizar(path, func(pl *plan.Plan) (err error) { t, err = plan.Hecha(pl, args[1]); return err })
		if err != nil {
			fmt.Fprintln(stderr, "orq plan:", err)
			return 1
		}
		workers.AppendTaskEvent(p.dir, "S-"+pl.Slice, t.ID, "done", t.Worker, time.Now())
		listas, _ := plan.Listas(pl)
		fmt.Fprintf(stdout, "%s hecha (%s). Quedan %d; listas ahora: %s\n", t.ID, t.Worker, pl.Pendientes(), ids(listas))
		return 0
	case "soltar":
		if len(args) != 2 {
			fmt.Fprintln(stderr, uso)
			return 1
		}
		_, path, err := p.planDe(nn)
		if err != nil {
			fmt.Fprintln(stderr, "orq plan:", err)
			return 1
		}
		var t *plan.Tarea
		var w string
		pl, err := plan.Actualizar(path, func(pl *plan.Plan) (err error) {
			if x := slices.IndexFunc(pl.Tareas, func(x plan.Tarea) bool { return x.ID == args[1] }); x >= 0 {
				w = pl.Tareas[x].Worker
			}
			t, err = plan.Soltar(pl, args[1])
			return err
		})
		if err != nil {
			fmt.Fprintln(stderr, "orq plan:", err)
			return 1
		}
		workers.AppendTaskEvent(p.dir, "S-"+pl.Slice, t.ID, "reopened", w, time.Now())
		fmt.Fprintf(stdout, "%s vuelve a pendiente; relánzala con orq slice lanzar --tarea %s\n", t.ID, t.ID)
		return 0
	case "listas":
		pl, _, err := p.planDe(nn)
		if err != nil {
			fmt.Fprintln(stderr, "orq plan:", err)
			return 1
		}
		listas, bloq := plan.Listas(pl)
		fmt.Fprintf(stdout, "ready: %s\n", ids(listas))
		for _, t := range pl.Tareas {
			if r, ok := bloq[t.ID]; ok {
				fmt.Fprintf(stdout, "  %s waits: %s\n", t.ID, r)
			}
		}
		return 0
	case "revisar":
		pl, _, err := p.planDe(nn)
		if err != nil {
			fmt.Fprintln(stderr, "orq plan:", err)
			return 1
		}
		_, a, err := p.slices().Alinear(nn)
		if err != nil {
			fmt.Fprintln(stderr, "orq plan:", err)
			return 1
		}
		d := plan.Revisar(pl, a.Cambiados)
		fmt.Fprintf(stdout, "S-%s plan check: %d pending, in progress: %s\n", pl.Slice, d.Pendientes, def(strings.Join(d.EnCurso, ", "), "none"))
		if !d.Hay() {
			fmt.Fprintln(stdout, "plan matches reality: no update needed")
			return 0
		}
		if len(d.SinPlan) > 0 {
			fmt.Fprintf(stdout, "changed but no task covers it: %s\n", strings.Join(cabeza(d.SinPlan, 8), ", "))
		}
		if len(d.HechaVacia) > 0 {
			fmt.Fprintf(stdout, "marked done with no changes in its paths: %s\n", strings.Join(d.HechaVacia, ", "))
		}
		fmt.Fprintln(stdout, "update the plan minimally (orq plan cargar), or tell the Orchestrator it's drift (orq msg send --para orquestador --tipo plan)")
		return 3
	case "debatido":
		_, path, err := p.planDe(nn)
		if err != nil {
			fmt.Fprintln(stderr, "orq plan:", err)
			return 1
		}
		pl, err := plan.Actualizar(path, func(pl *plan.Plan) error { pl.MarcarDebatido(); return nil })
		if err != nil {
			fmt.Fprintln(stderr, "orq plan:", err)
			return 1
		}
		fmt.Fprintf(stdout, "plan marcado como debatido (revisión %s); cambiar el plan lo borra\n", pl.Revision())
		return 0
	}
	fmt.Fprintln(stderr, uso)
	return 1
}

func ids(ts []plan.Tarea) string {
	if len(ts) == 0 {
		return "ninguna"
	}
	var s []string
	for _, t := range ts {
		s = append(s, t.ID)
	}
	return strings.Join(s, ", ")
}

func mostrarPlan(pl *plan.Plan, w io.Writer) {
	listas, bloq := plan.Listas(pl)
	lista := map[string]bool{}
	for _, t := range listas {
		lista[t.ID] = true
	}
	deb := "sin debatir"
	if pl.Debatido {
		deb = "debatido"
	}
	fmt.Fprintf(w, "plan S-%s · %d tareas, %d pendientes · %s\n", pl.Slice, len(pl.Tareas), pl.Pendientes(), deb)
	for _, t := range pl.Tareas {
		marca := "·"
		nota := ""
		switch {
		case t.Estado == "hecha":
			marca = "✓"
		case t.Estado == "en_curso":
			marca = "▶"
			nota = t.Worker
		case lista[t.ID]:
			marca = "○"
			nota = "lista"
		default:
			nota = bloq[t.ID]
		}
		fmt.Fprintf(w, " %s %-10s %-8s %-5s %-40s %s\n", marca, t.ID, t.Zona, t.Dificultad, strings.Join(t.Rutas, ","), nota)
	}
}
