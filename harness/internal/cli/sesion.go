package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"

	"orq/internal/app"
	"orq/internal/canal"
	"orq/internal/config"
	"orq/internal/terminal"
)

func runSesion(args []string, stdout, stderr io.Writer) int {
	p, err := abrirProyecto()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	uso := "uso: orq sesion ver | abrir planificador|astra [--kind claude|codex --modelo M --esfuerzo E] | cerrar NOMBRE | reenviar NOMBRE"
	if len(args) == 0 {
		args = []string{"ver"}
	}
	ses, err := canal.LoadSesiones(p.SesPath())
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	switch args[0] {
	case "ver":
		fmt.Fprintln(stdout, "layout: "+app.Layout)
		var ks []string
		for k := range ses {
			ks = append(ks, k)
		}
		sort.Strings(ks)
		for _, k := range ks {
			s := ses[k]
			fmt.Fprintf(stdout, "  %-24s %-28s pane %-8s tab %s\n", k, s.Agente, s.Pane, s.Tab)
		}
		return 0
	case "abrir":
		fs := flag.NewFlagSet("sesion abrir", flag.ContinueOnError)
		fs.SetOutput(stderr)
		a := flagsArranque(fs)
		if len(args) < 2 || (args[1] != "planificador" && args[1] != "astra") || fs.Parse(args[2:]) != nil {
			fmt.Fprintln(stderr, uso)
			return 1
		}
		if err := p.Dentro(); err != nil {
			fmt.Fprintln(stderr, "orq sesion:", err)
			return 1
		}
		rol := args[1]
		if s, ok := ses[rol]; ok {
			fmt.Fprintf(stderr, "ya hay %s (%s, panel %s); ciérrala antes con orq sesion cerrar %s\n", rol, s.Agente, s.Pane, rol)
			return 1
		}
		s, usado, err := p.LanzarRol(context.Background(), ses, rol, *a)
		if err != nil {
			fmt.Fprintln(stderr, "orq sesion:", err)
			if errors.Is(err, terminal.ErrDesconocido) {
				return 4
			}
			return 1
		}
		fmt.Fprintf(stdout, "%s: %s (%s %s, cuenta: %s) en panel %s\n", rol, s.Agente, usado.Kind, usado.Modelo, config.Cuenta(usado.Entorno), s.Pane)
		return 0
	case "cerrar":
		if len(args) != 2 {
			fmt.Fprintln(stderr, uso)
			return 1
		}
		// A worker's close merges and verifies first; a failure keeps its pane open.
		informe, err := p.CerrarWorker(context.Background(), args[1])
		if informe != "" {
			fmt.Fprintln(stdout, informe)
		}
		if err != nil {
			fmt.Fprintln(stderr, "orq sesion:", err)
			return 1
		}
		cerradas, err := p.CerrarSesiones(context.Background(), ses, func(k string) bool { return k == args[1] })
		if err != nil {
			fmt.Fprintln(stderr, "orq sesion:", err)
			return 1
		}
		if len(cerradas) == 0 {
			fmt.Fprintf(stderr, "no hay sesión %s (orq sesion ver)\n", args[1])
			return 1
		}
		fmt.Fprintf(stdout, "cerrada: %s\n", strings.Join(cerradas, ", "))
		return 0
	case "reenviar":
		// Tras mirar el panel: el arranque o el contrato no llegó, y el próximo comando lo repite.
		if len(args) != 2 {
			fmt.Fprintln(stderr, uso)
			return 1
		}
		if err := p.Reenviar(args[1]); err != nil {
			fmt.Fprintln(stderr, "orq sesion:", err)
			return 1
		}
		fmt.Fprintf(stdout, "%s: el próximo lanzamiento vuelve a entregar su arranque o contrato\n", args[1])
		return 0
	}
	fmt.Fprintln(stderr, uso)
	return 1
}
