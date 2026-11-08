package cli

import (
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"

	"orq/internal/canal"
)

// Distribución de herdr, siempre igual y con nombre:
//
//	pestaña del Owner      → atena-<proyecto>
//	«orq · <proyecto>»     → orquestador-<proyecto>
//	«plan · <proyecto>»    → planificador-<proyecto> | astra-<proyecto>
//	«S-NN · <proyecto>»    → worker-<zona>-sNN (uno al lado del otro)
const layout = `atena-P (Owner's tab) · "orq · P" orquestador-P · "plan · P" planificador-P | astra-P · "S-NN · P" worker-<zone>-sNN`

func runSesion(args []string, stdout, stderr io.Writer) int {
	p, err := abrirProyecto()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	uso := "uso: orq sesion ver | abrir planificador|astra [--kind AGENTE --modelo M --esfuerzo E] | cerrar NOMBRE"
	if len(args) == 0 {
		args = []string{"ver"}
	}
	ses, err := canal.LoadSesiones(p.sesPath())
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	switch args[0] {
	case "ver":
		fmt.Fprintln(stdout, "layout: "+layout)
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
		fs := nuevoFS("sesion abrir", flag.ContinueOnError)
		fs.SetOutput(stderr)
		a := flagsArranque(fs)
		if len(args) < 2 || (args[1] != "planificador" && args[1] != "astra") || fs.Parse(args[2:]) != nil {
			fmt.Fprintln(stderr, uso)
			return 1
		}
		if !rn().Inside() {
			fmt.Fprintln(stderr, "orq sesion: corre esto dentro del runner (panel del Orquestador)")
			return 1
		}
		rol := args[1]
		if s, ok := ses[rol]; ok {
			fmt.Fprintf(stderr, "ya hay %s (%s, panel %s); ciérrala antes con orq sesion cerrar %s\n", rol, s.Agente, s.Pane, rol)
			return 1
		}
		s, err := lanzar(p, ses, rol, a)
		if err != nil {
			fmt.Fprintln(stderr, "orq sesion:", err)
			return 1
		}
		fmt.Fprintf(stdout, "%s: %s (%s %s) en panel %s\n", rol, s.Agente, a.kind, a.modelo, s.Pane)
		return 0
	case "cerrar":
		if len(args) != 2 {
			fmt.Fprintln(stderr, uso)
			return 1
		}
		cerradas, err := p.cerrarSesiones(ses, func(k string) bool { return k == args[1] })
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
	}
	fmt.Fprintln(stderr, uso)
	return 1
}

// cerrarSesiones cierra los paneles que cumplen el filtro y los quita del registro. Atena es del
// Owner y el Orquestador se releva, no se cierra: nunca entran aquí.
func (p proyecto) cerrarSesiones(ses map[string]canal.Sesion, sirve func(string) bool) ([]string, error) {
	var cerradas []string
	for k, s := range ses {
		if k == "atena" || k == "orquestador" || !sirve(k) {
			continue
		}
		if s.Pane != "" && rn().Inside() {
			if err := rn().Close(s.Pane); err != nil && !strings.Contains(err.Error(), "not found") {
				return cerradas, fmt.Errorf("%s: %v", k, err)
			}
		}
		delete(ses, k)
		cerradas = append(cerradas, k)
	}
	sort.Strings(cerradas)
	return cerradas, canal.SaveSesiones(p.sesPath(), ses)
}

// panelDelSlice da un panel ya abierto de ese slice: los workers comparten la pestaña «S-NN · P».
func panelDelSlice(ses map[string]canal.Sesion, nn string) (canal.Sesion, bool) {
	for k, s := range ses {
		if strings.HasPrefix(k, "worker-") && strings.HasSuffix(k, "-s"+nn) && s.Pane != "" {
			return s, true
		}
	}
	return canal.Sesion{}, false
}
