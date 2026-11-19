package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"time"

	"orq/internal/gate"
)

// enUnWorker refuses an orchestrator command inside a worker clone or worktree, as runSlice does.
func enUnWorker(cmd string, stderr io.Writer) bool {
	if wd, err := os.Getwd(); err != nil || gate.IsWorker(wd) {
		fmt.Fprintf(stderr, "orq: `orq %s` belongs to the orchestrator and does not run inside a worker clone or worktree.\n", cmd)
		return true
	}
	return false
}

func runHistoria(args []string, stdout, stderr io.Writer) int {
	uso := "uso: orq historia abrir H --titulo \"...\" | pausar H | reanudar H"
	if len(args) < 2 {
		fmt.Fprintln(stderr, uso)
		return 1
	}
	if enUnWorker("historia "+args[0], stderr) {
		return 1
	}
	p, err := abrirProyecto()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	r := p.Slices()
	switch args[0] {
	case "abrir":
		fs := flag.NewFlagSet("historia abrir", flag.ContinueOnError)
		fs.SetOutput(stderr)
		titulo := fs.String("titulo", "", "la historia en pocas palabras")
		if fs.Parse(args[2:]) != nil {
			fmt.Fprintln(stderr, uso)
			return 1
		}
		h, err := r.AbrirHistoria(args[1], *titulo, time.Now())
		if err != nil {
			fmt.Fprintln(stderr, "orq historia:", err)
			return 1
		}
		fmt.Fprintf(stdout, "historia %s «%s» abierta. Sus hijos: orq slice abrir NN --historia %s ...\n", h.ID, h.Titulo, h.ID)
		return 0
	case "pausar", "reanudar":
		if err := r.PausarHistoria(args[1], args[0] == "pausar", time.Now()); err != nil {
			fmt.Fprintln(stderr, "orq historia:", err)
			return 1
		}
		fmt.Fprintf(stdout, "historia %s: %s. Los workers que ya corren, sus clones y su estado no se tocan\n", args[1], map[bool]string{true: "en pausa, no se lanzan workers nuevos", false: "reanudada"}[args[0] == "pausar"])
		return 0
	}
	fmt.Fprintln(stderr, uso)
	return 1
}

func runPausa(pausar bool, args []string, stdout, stderr io.Writer) int {
	cmd := map[bool]string{true: "pausar", false: "reanudar"}[pausar]
	if enUnWorker(cmd, stderr) {
		return 1
	}
	p, err := abrirProyecto()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	motivo := strings.Join(args, " ")
	if pausar && motivo == "" {
		motivo = "sin motivo"
	}
	if err := p.Slices().PausarTodo(pausar, motivo, time.Now()); err != nil {
		fmt.Fprintln(stderr, "orq:", err)
		return 1
	}
	if pausar {
		fmt.Fprintln(stdout, "orq en pausa: no se lanzan workers nuevos. Los que corren, sus clones y el estado no se tocan. orq reanudar")
	} else {
		fmt.Fprintln(stdout, "orq reanudado")
	}
	return 0
}

// slicesPorHistoria prints one line per story with its children's states, then the slices of no story.
func slicesPorHistoria(p proyecto, stdout io.Writer) {
	r := p.Slices()
	hs, err := r.Historias()
	if err != nil {
		fmt.Fprintf(stdout, "!! historias: no se pudieron leer (%v)\n", err)
	}
	if len(hs) > 0 {
		fmt.Fprintln(stdout, "historias:")
	}
	for _, h := range hs {
		ab, cer, err := r.Hijos(h.ID)
		if err != nil {
			fmt.Fprintf(stdout, "  %s: %v\n", h.ID, err)
			continue
		}
		cab := fmt.Sprintf("%s «%s»", h.ID, h.Titulo)
		if h.Pausada {
			cab += " PAUSADA"
		}
		var hijos []string
		for _, reg := range cer {
			hijos = append(hijos, fmt.Sprintf("S-%s %s", reg.NN, reg.Result))
		}
		for _, e := range ab {
			hijos = append(hijos, e.ID()+" "+e.Fase)
		}
		slices.Sort(hijos)
		fmt.Fprintf(stdout, "  %s\n", strings.Join(append([]string{cab}, hijos...), " · "))
	}
	ab, err := r.Abiertos()
	if err != nil {
		return
	}
	conocida := map[string]bool{}
	for _, h := range hs {
		conocida[h.ID] = true
	}
	var sueltos []string
	huerfanas := map[string][]string{}
	var ids []string
	for _, e := range ab {
		linea := fmt.Sprintf("  %s «%s» %s", e.Attempt(), e.Titulo, e.Fase)
		switch {
		case e.Historia == "":
			sueltos = append(sueltos, linea)
		case !conocida[e.Historia]:
			if huerfanas[e.Historia] == nil {
				ids = append(ids, e.Historia)
			}
			huerfanas[e.Historia] = append(huerfanas[e.Historia], linea)
		}
	}
	slices.Sort(ids)
	for _, id := range ids {
		fmt.Fprintf(stdout, "!! historia desconocida: %s\n%s\n", id, strings.Join(huerfanas[id], "\n"))
	}
	if len(sueltos) > 0 {
		fmt.Fprintf(stdout, "slices sin historia:\n%s\n", strings.Join(sueltos, "\n"))
	}
}

// alertasArriba prints the drift alerts before anything else in orq status; none prints nothing.
func alertasArriba(p proyecto, stdout io.Writer) {
	as, err := p.Alertas(context.Background())
	if err != nil {
		fmt.Fprintf(stdout, "ALERTAS: no se pudieron calcular (%v)\n", err)
		return
	}
	if len(as) == 0 {
		return
	}
	fmt.Fprintf(stdout, "ALERTAS (%d):\n", len(as))
	for _, a := range as {
		fmt.Fprintf(stdout, "  %s\n", a)
	}
}
