package cli

import (
	"fmt"
	"io"
	"strings"

	"orq/internal/identidad"
	"orq/internal/orqdir"
	"orq/internal/plan"
	"orq/internal/slice"
)

// planDe carga el plan del slice; nn vacío = el único abierto.
func planDe(p proyecto, nn string) (*plan.Plan, string, error) {
	e, err := p.Slices().Load(nn)
	if err != nil || e == nil {
		return nil, "", fmt.Errorf("no hay slice abierto")
	}
	path := p.PlanPath(e.NN)
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
	uso := "uso: orq plan [--slice NN] cargar ARCHIVO.json | ver | revisar | hecha ID | soltar ID | debatido"
	switch args[0] {
	case "cargar":
		if len(args) != 2 {
			fmt.Fprintln(stderr, uso)
			return 1
		}
		e, err := p.Slices().Load(nn)
		if err != nil || e == nil {
			fmt.Fprintln(stderr, "orq plan: no hay slice abierto; el plan pertenece a un slice")
			return 1
		}
		if e.RutaEfectiva() == slice.RutaFix {
			fmt.Fprintf(stderr, "orq plan: %s va por la ruta fix: un solo worker, sin plan de tareas. Si el trabajo creció, súbelo: `orq slice ruta slice \"motivo\"`\n", e.ID())
			return 1
		}
		pl, err := plan.Leer(args[1])
		if err != nil {
			fmt.Fprintln(stderr, "orq plan:", err)
			return 1
		}
		pl.Slice = e.NN
		soltarPlan := func() {}
		defer func() { soltarPlan() }()
		if identidad.EsOrq2() {
			// The old plan is read and replaced under its lock: a task taken meanwhile is kept.
			soltar, err := orqdir.Candado(p.PlanPath(e.NN))
			if err != nil {
				fmt.Fprintln(stderr, "orq plan:", err)
				return 1
			}
			soltarPlan = soltar
		}
		var kept []string
		if viejo, err := plan.Leer(p.PlanPath(e.NN)); err == nil {
			kept = plan.Conservar(pl, viejo)
		}
		if err := plan.Validar(pl, e.Alcance); err != nil {
			fmt.Fprintf(stderr, "orq plan: el plan no sirve así:\n%s\n", err)
			return 1
		}
		if err := plan.Guardar(p.PlanPath(e.NN), pl); err != nil {
			fmt.Fprintln(stderr, "orq plan:", err)
			return 1
		}
		soltarPlan() // the plan lock is a leaf: it is not held while the slice lock is taken below
		soltarPlan = func() {}
		listas, _ := plan.Listas(pl)
		fmt.Fprintf(stdout, "S-%s: plan de %d tareas guardado; %d listas para despachar: orq slice lanzar --tarea ID\n", e.NN, len(pl.Tareas), len(listas))
		if len(kept) > 0 {
			fmt.Fprintf(stdout, "avance conservado: %s\n", strings.Join(kept, ", "))
		}
		// Una tarea ya lanzada que creció: su worker recibe las rutas nuevas, o juntar las rechaza.
		for _, t := range pl.Tareas {
			if t.Estado == "pendiente" {
				continue
			}
			if _, err := p.Slices().AmpliarZona(e.NN, t.Zona, t.Rutas); err != nil && !strings.Contains(err.Error(), "no tiene worker") {
				fmt.Fprintf(stderr, "aviso: zona %s de %s: %v\n", t.Zona, t.ID, err)
			}
		}
		return 0
	case "ver":
		pl, _, err := planDe(p, nn)
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
		pl, path, err := planDe(p, nn)
		if err != nil {
			fmt.Fprintln(stderr, "orq plan:", err)
			return 1
		}
		t, err := cambiarPlan(path, &pl, func(f *plan.Plan) (*plan.Tarea, error) { return plan.Hecha(f, args[1]) })
		if t == nil {
			fmt.Fprintln(stderr, "orq plan:", err)
			return 1
		}
		if err != nil {
			fmt.Fprintln(stderr, "orq plan: el plan no se guardó, nada cambió:", err)
			return 1
		}
		listas, _ := plan.Listas(pl)
		fmt.Fprintf(stdout, "%s hecha (%s). Quedan %d; listas ahora: %s\n", t.ID, t.Worker, pl.Pendientes(), ids(listas))
		return 0
	case "soltar":
		if len(args) != 2 {
			fmt.Fprintln(stderr, uso)
			return 1
		}
		pl, path, err := planDe(p, nn)
		if err != nil {
			fmt.Fprintln(stderr, "orq plan:", err)
			return 1
		}
		t, err := cambiarPlan(path, &pl, func(f *plan.Plan) (*plan.Tarea, error) { return plan.Soltar(f, args[1]) })
		if t == nil {
			fmt.Fprintln(stderr, "orq plan:", err)
			return 1
		}
		if err != nil {
			fmt.Fprintln(stderr, "orq plan: el plan no se guardó, nada cambió:", err)
			return 1
		}
		fmt.Fprintf(stdout, "%s vuelve a pendiente; relánzala con orq slice lanzar --tarea %s\n", t.ID, t.ID)
		return 0
	case "revisar":
		pl, _, err := planDe(p, nn)
		if err != nil {
			fmt.Fprintln(stderr, "orq plan:", err)
			return 1
		}
		_, a, err := p.Slices().Alinear(nn)
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
		pl, path, err := planDe(p, nn)
		if err != nil {
			fmt.Fprintln(stderr, "orq plan:", err)
			return 1
		}
		_, err = cambiarPlan(path, &pl, func(f *plan.Plan) (*plan.Tarea, error) {
			f.Debatido = true
			return &plan.Tarea{}, nil
		})
		if err != nil {
			fmt.Fprintln(stderr, "orq plan: el plan no se guardó, nada cambió:", err)
			return 1
		}
		fmt.Fprintln(stdout, "plan marcado como debatido")
		return 0
	}
	fmt.Fprintln(stderr, uso)
	return 1
}

// cambiarPlan applies cambio and saves. It returns a nil task when cambio refused (err says why)
// and a non-nil task with an error when the plan did not change on disk. orq changes the plan it read and saves it
// whole, ignoring a failed save, as 1.0.0 did; orq2 changes the plan on disk under its lock, so a
// change made meanwhile by a launch or another command is not lost, and *pl is that plan.
func cambiarPlan(path string, pl **plan.Plan, cambio func(*plan.Plan) (*plan.Tarea, error)) (*plan.Tarea, error) {
	if !identidad.EsOrq2() {
		t, err := cambio(*pl)
		if err != nil {
			return nil, err
		}
		plan.Guardar(path, *pl) // orq keeps 1.0.0
		return t, nil
	}
	var t *plan.Tarea
	var errC error
	f, err := plan.Actualizar(path, func(f *plan.Plan) error {
		t, errC = cambio(f)
		return errC
	})
	if errC != nil {
		return nil, errC
	}
	if err != nil {
		return &plan.Tarea{}, err // lock, read or save failed: nothing changed
	}
	*pl = f
	return t, nil
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
