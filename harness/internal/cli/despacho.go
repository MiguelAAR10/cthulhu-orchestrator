package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"orq/internal/app"
	"orq/internal/terminal"
)

// usoDespachoOrq2 is the usage of `orq2 pendientes` and `orq2 despachar`, shown only under orq2.
const usoDespachoOrq2 = `  orq2 pendientes                 quién debe qué y desde cuándo: slices aprobados con tareas listas y sin
                                 worker, tarjetas y decisiones del Owner, bloqueos, cierres a medias,
                                 mensajes sin entrega y preguntas sin respuesta. Solo lee
  orq2 despachar --una-vez [--agente claude]   lanza, una vez, las tareas listas de los slices aprobados
                                 con plan debatido; repetirlo no lanza otro worker y retoma un
                                 lanzamiento cortado (anota la intención antes de lanzar)
`

// tareasTxt is "T1 lista" or "T1, T5 listas".
func tareasTxt(ids []string) string {
	if len(ids) == 1 {
		return ids[0] + " lista"
	}
	return strings.Join(ids, ", ") + " listas"
}

// lineaPendiente is one line of `orq2 pendientes`.
func lineaPendiente(x app.Pendiente, now time.Time) string {
	hace := edad(now.Sub(x.Desde))
	s := ""
	if x.Slice != "" {
		s = fmt.Sprintf("S-%s «%s» ", x.Slice, x.Titulo)
	}
	debe := func(cmd string) string { return " · debe: " + x.Debe + " · " + cmd }
	switch x.Tipo {
	case app.PendFaltaLanzar:
		return s + "aprobada hace " + hace + ", " + tareasTxt(x.Tareas) + ", sin worker" + debe("orq2 despachar --una-vez")
	case app.PendCortado:
		return s + fmt.Sprintf("%s en curso para %s, que nunca recibió el contrato (lanzamiento cortado; aprobada hace %s)", x.Tareas[0], x.Worker, hace) + debe("orq2 despachar --una-vez")
	case app.PendPausa:
		return s + "aprobada hace " + hace + ", " + tareasTxt(x.Tareas) + ", en pausa (" + x.Texto + ")" + debe("orq2 reanudar")
	case app.PendSinPlan:
		return s + "aprobada hace " + hace + ", sin plan ni worker" + debe("orq2 plan --slice "+x.Slice+" cargar ARCHIVO.json")
	case app.PendSinDebate:
		return s + "aprobada hace " + hace + ", plan sin debatir" + debe("orq2 plan --slice "+x.Slice+" debatido")
	case app.PendFixSinWorker:
		return s + "fix abierto hace " + hace + ", sin worker" + debe("orq2 slice lanzar --slice "+x.Slice)
	case app.PendTarjeta:
		return s + "tarjeta lista hace " + hace + ", sin aprobar" + debe("orq2 slice aprobar "+x.Slice)
	case app.PendDecision:
		return s + "verificado hace " + hace + ", sin decisión" + debe("orq2 slice decidir "+x.Slice)
	case app.PendBloqueado:
		return s + "bloqueado hace " + hace + ": " + app.Recortar(x.Texto, 80) + debe("orq2 slice reintentar --slice "+x.Slice+" \"qué cambia\" --causa worker|examen")
	case app.PendCierre:
		return s + "cierre a medias desde hace " + hace + debe("orq2 slice cerrar --slice "+x.Slice+" (repetirlo continúa)")
	case app.PendEntrega:
		return fmt.Sprintf("mensaje %s %s sin entrega confirmada hace %s", x.Ref, x.Texto, hace) + debe("orq2 msg reconciliar")
	case app.PendRespuesta:
		return fmt.Sprintf("pregunta %s sin respuesta hace %s: %s", x.Ref, hace, x.Texto) + debe("orq2 msg send --tipo respuesta --ref "+x.Ref)
	}
	return s + x.Tipo + " hace " + hace
}

// runPendientes prints who owes what since when. Read-only.
func runPendientes(args []string, stdout, stderr io.Writer) int {
	if len(args) != 0 {
		fmt.Fprintln(stderr, "uso: orq2 pendientes")
		return 1
	}
	p, err := abrirProyecto()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	ps, err := p.Pendientes()
	if err != nil {
		fmt.Fprintln(stderr, "orq pendientes:", err)
		return 1
	}
	if len(ps) == 0 {
		fmt.Fprintln(stdout, "pendientes: ninguno")
		return 0
	}
	now := time.Now()
	fmt.Fprintln(stdout, "pendientes (el más antiguo primero):")
	for _, x := range ps {
		fmt.Fprintln(stdout, "  "+lineaPendiente(x, now))
	}
	return 0
}

// runDespachar launches the ready tasks of approved slices, once.
func runDespachar(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("despachar", flag.ContinueOnError)
	fs.SetOutput(stderr)
	unaVez := fs.Bool("una-vez", false, "lanza las tareas listas una vez y termina")
	agente := fs.String("agente", "claude", "claude | codex")
	if fs.Parse(args) != nil || fs.NArg() != 0 || !*unaVez {
		fmt.Fprintln(stderr, "uso: orq2 despachar --una-vez [--agente claude]  (el despachador residente llega en v2.1)")
		return 1
	}
	p, err := abrirProyecto()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	ds, err := p.DespacharUnaVez(context.Background(), *agente)
	for _, d := range ds {
		l := fmt.Sprintf("S-%s %s → %s: %s", d.Slice, d.Tarea, d.Worker, d.Resultado)
		if d.Detalle != "" {
			l += " (" + d.Detalle + ")"
		}
		fmt.Fprintln(stdout, l)
	}
	if len(ds) == 0 && err == nil {
		fmt.Fprintln(stdout, "nada que despachar: ninguna tarea lista sin worker en slices aprobados")
	}
	if err != nil {
		fmt.Fprintln(stderr, "orq despachar:", err)
		if errors.Is(err, terminal.ErrDesconocido) {
			return 4 // mira el panel y repite: no lanza dos veces
		}
		return 1
	}
	return 0
}

// faltaLanzar prints, under orq2, the approved slices whose ready tasks have no worker (H30: an
// approved slice nobody launched is not "in construction"). nn filters to one slice; conTitulo is
// the status form, without it the indented `slice estado` form.
func faltaLanzar(p proyecto, nn string, stdout io.Writer, conTitulo bool) {
	ps, err := p.Pendientes()
	if err != nil {
		return
	}
	for _, x := range ps {
		if x.Tipo != app.PendFaltaLanzar || nn != "" && x.Slice != nn {
			continue
		}
		if conTitulo {
			fmt.Fprintf(stdout, "S-%s «%s» aprobada, falta lanzar: %s, sin worker · orq2 despachar --una-vez\n", x.Slice, x.Titulo, tareasTxt(x.Tareas))
		} else {
			fmt.Fprintf(stdout, "  aprobada, falta lanzar: %s, sin worker · orq2 despachar --una-vez\n", tareasTxt(x.Tareas))
		}
	}
}
