package cli

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"orq/internal/limits"
	"orq/internal/qa"
	"orq/internal/stops"
	"orq/internal/workers"
)

// maxFronts: max_fronts de limits.json, o el defecto si no hay limits.
func maxFronts() int {
	l, _ := limits.Load(limits.DefaultPath())
	return l.TopeFronts()
}

// orq front brief|receipt F [--slice NN]: fronts son sub-orquestadores como DATO del plan.
// El orquestador principal solo consume su brief y su recibo; la admisión sigue siendo global.
func runFront(args []string, stdout, stderr io.Writer) int {
	uso := "uso: orq front brief|receipt FRONT [--slice NN]"
	nn := ""
	for i := 0; i < len(args); i++ {
		if args[i] == "--slice" && i+1 < len(args) {
			nn = strings.TrimPrefix(args[i+1], "S-")
			args = append(args[:i:i], args[i+2:]...)
			break
		}
	}
	if len(args) != 2 || (args[0] != "brief" && args[0] != "receipt") {
		fmt.Fprintln(stderr, uso)
		return 1
	}
	p, err := abrirProyecto()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	e, err := p.slices().Load(nn)
	if err != nil || e == nil {
		fmt.Fprintln(stderr, "orq front: no hay slice abierto")
		return 1
	}
	pl, _, err := p.planDe(e.NN)
	if err != nil {
		fmt.Fprintln(stderr, "orq front:", err)
		return 1
	}
	r, err := pl.Resumen(args[1])
	if err != nil {
		fmt.Fprintln(stderr, "orq front:", err)
		return 1
	}
	f := r.Front
	var b strings.Builder
	if args[0] == "brief" {
		fmt.Fprintf(&b, "# Front %s · S-%s %s\n\n", f.ID, e.NN, e.Titulo)
		fmt.Fprintf(&b, "Goal: %s\nOwner role: %s\nScope (only these paths): %s\n", f.Goal, def(f.OwnerRole, "worker"), strings.Join(f.Paths, ", "))
		fmt.Fprintf(&b, "Acceptance: the slice exam %s must turn green (orq slice verify); do not touch it.\n\nTasks:\n", def(e.Test, "(none)"))
		for _, t := range r.Tareas {
			dep := ""
			if len(t.DependeDe) > 0 {
				dep = " · after " + strings.Join(t.DependeDe, ", ")
			}
			fmt.Fprintf(&b, "- %s [%s] %s · %s%s\n", t.ID, t.Estado, def(t.Que, "-"), strings.Join(t.Rutas, ", "), dep)
		}
		fmt.Fprint(stdout, b.String())
		return 0
	}
	ids := map[string]bool{}
	for _, t := range r.Tareas {
		ids[t.ID] = true
	}
	pass, total := 0, 0
	ws, _ := workers.List(p.dir)
	for _, w := range ws {
		if w.Slice == e.ID() && ids[w.Task] && w.Tests != nil {
			pass += w.Tests.Passing
			total += w.Tests.Total
		}
	}
	all, _ := stops.Load(stops.Path(p.dir))
	var st []string
	for _, s := range stops.Open(all) {
		if s.Slice == e.ID() && (len(s.Tasks) == 0 || slices.ContainsFunc(s.Tasks, func(x string) bool { return ids[x] })) {
			st = append(st, fmt.Sprintf("%s %s %s: %s", s.ID, s.Severity, s.Type, s.Text))
		}
	}
	findings := 0
	rec, _ := qa.Recibo(p.dir, e.NN)
	for _, x := range qa.Findings(p.dir, e.NN) {
		if x.Confirmed && (rec == nil || !x.TS.Before(rec.TS)) {
			findings++
		}
	}
	fmt.Fprintf(&b, "# Front %s receipt · S-%s\n\nGoal: %s\n%d/%d tasks done\n", f.ID, e.NN, f.Goal, r.Done, r.Total)
	for _, t := range r.Tareas {
		fmt.Fprintf(&b, "- %s %s %s\n", t.ID, t.Estado, def(t.Worker, ""))
	}
	if total > 0 {
		fmt.Fprintf(&b, "tests %d/%d passing (workers' last receipts)\n", pass, total)
	} else {
		fmt.Fprintln(&b, "tests: no worker receipt yet")
	}
	fmt.Fprintf(&b, "findings: %d open (QA, slice-wide)\nstoppers: %d\n", findings, len(st))
	for _, s := range st {
		fmt.Fprintln(&b, "  "+s)
	}
	fmt.Fprint(stdout, b.String())
	return 0
}
