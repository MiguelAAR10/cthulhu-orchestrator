package cli

import (
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"time"

	"orq/internal/app"
	"orq/internal/diario"
	"orq/internal/slice"
)

// nnYResto separa el NN posicional de los flags: `orq slice tarjeta 01 --archivo f`.
func nnYResto(nn string, args []string) (string, []string) {
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		return strings.TrimPrefix(args[0], "S-"), args[1:]
	}
	return nn, args
}

func imprimirTarjeta(w io.Writer, e *slice.Estado) {
	tj := e.Tarjeta
	estado := fmt.Sprintf("lista %s · espera al Owner: orq slice aprobar %s", tj.Lista.Local().Format("2006-01-02 15:04"), e.NN)
	if tj.EstaAprobada() {
		estado = "aprobada " + tj.Aprobada.Local().Format("2006-01-02 15:04")
	}
	fmt.Fprintf(w, "%s «%s» · tarjeta %s\n", e.ID(), e.Titulo, estado)
	for _, s := range []struct {
		titulo string
		lineas []string
	}{{"Historia", tj.Historia}, {"Fuera", tj.Fuera}, {"Examen", tj.Examen}, {"Zonas", tj.Zonas}, {"Preguntas", tj.Preguntas}} {
		fmt.Fprintf(w, "\n%s\n", s.titulo)
		for _, l := range s.lineas {
			fmt.Fprintf(w, "  %s\n", l)
		}
	}
}

// runTarjeta pone la tarjeta del slice desde un archivo o, sin --archivo, la muestra.
func runTarjeta(p proyecto, nn string, args []string, stdout, stderr io.Writer) int {
	nn, args = nnYResto(nn, args)
	archivo := ""
	switch {
	case len(args) == 2 && args[0] == "--archivo":
		archivo = args[1]
	case len(args) != 0:
		fmt.Fprintln(stderr, "uso: orq slice tarjeta [NN] [--archivo tarjeta.md]  (secciones ## Historia, Fuera, Examen, Zonas, Preguntas)")
		return 1
	}
	r := p.Slices()
	if archivo == "" {
		e, err := r.Load(nn)
		if err != nil {
			fmt.Fprintln(stderr, "orq slice:", err)
			return 1
		}
		if e == nil || e.Tarjeta == nil {
			fmt.Fprintln(stderr, "orq slice: sin tarjeta; escríbela con orq slice tarjeta NN --archivo tarjeta.md")
			return 1
		}
		imprimirTarjeta(stdout, e)
		return 0
	}
	raw, err := os.ReadFile(archivo)
	if err != nil {
		fmt.Fprintln(stderr, "orq slice:", err)
		return 1
	}
	tj, err := slice.LeerTarjeta(string(raw))
	if err != nil {
		fmt.Fprintln(stderr, "orq slice: tarjeta:", err)
		return 1
	}
	e, err := r.PonerTarjeta(nn, tj, time.Now())
	if err != nil {
		fmt.Fprintln(stderr, "orq slice:", err)
		return 1
	}
	fmt.Fprintf(stdout, "%s: tarjeta lista. El Owner la lee con orq slice tarjeta %s y la aprueba en su terminal con orq slice aprobar %s\n", e.ID(), e.NN, e.NN)
	return 0
}

// runAprobar sella la aprobación de la tarjeta. Es del Owner: no corre en un panel que abrió orq.
func runAprobar(p proyecto, nn string, args []string, stdout, stderr io.Writer) int {
	nn, args = nnYResto(nn, args)
	if len(args) != 0 {
		fmt.Fprintln(stderr, "uso: orq slice aprobar [NN]")
		return 1
	}
	if rol := os.Getenv("ORQ_ROL"); rol != "" {
		fmt.Fprintf(stderr, "orq slice: aprobar la tarjeta es del Owner; este panel es de %s. Córrelo en la terminal del Owner, fuera de los paneles de agentes.\n", rol)
		return 1
	}
	e, ya, err := p.Slices().AprobarTarjeta(nn, time.Now())
	if err != nil {
		fmt.Fprintln(stderr, "orq slice:", err)
		return 1
	}
	if ya {
		fmt.Fprintf(stdout, "%s: la tarjeta ya estaba aprobada (%s); no se sella otra vez\n", e.ID(), e.Tarjeta.Aprobada.Local().Format("2006-01-02 15:04"))
		return 0
	}
	fmt.Fprintf(stdout, "%s: tarjeta aprobada. Ya se puede lanzar el worker: orq slice lanzar --slice %s\n", e.ID(), e.NN)
	return 0
}

// tarjetasPorAprobar lista las tarjetas que esperan al Owner, la más antigua primero.
func tarjetasPorAprobar(p proyecto, now time.Time, stdout io.Writer) {
	ab, err := p.Slices().Abiertos()
	if err != nil {
		fmt.Fprintf(stdout, "tarjetas por aprobar: no se pudo leer (%v)\n", err)
		return
	}
	ab = slices.DeleteFunc(ab, func(e *slice.Estado) bool { return e.Tarjeta == nil || e.Tarjeta.EstaAprobada() })
	if len(ab) == 0 {
		fmt.Fprintln(stdout, "tarjetas por aprobar: ninguna")
		return
	}
	slices.SortStableFunc(ab, func(a, b *slice.Estado) int { return a.Tarjeta.Lista.Compare(b.Tarjeta.Lista) })
	fmt.Fprintln(stdout, "tarjetas por aprobar (antes que nada):")
	for _, e := range ab {
		fmt.Fprintf(stdout, "  %s «%s» lista hace %s · lee: orq slice tarjeta %s · aprueba: orq slice aprobar %s\n", e.ID(), e.Titulo, edad(now.Sub(e.Tarjeta.Lista)), e.NN, e.NN)
	}
}

// seccionTarjeta muestra en la página de decisión la tarjeta que aprobó el Owner y marca lo que se
// apartó de ella después: un examen cambiado o un archivo fuera de sus zonas.
func seccionTarjeta(pd *app.PaginaDecision) seccion {
	e := pd.Estado
	tj := e.Tarjeta
	s := seccion{Titulo: "Tarjeta", Fuente: "tarjeta del slice"}
	switch {
	case !tj.EstaAprobada() && e.RutaEfectiva() == slice.RutaFix:
		s.Lineas = append(s.Lineas, normal("ruta fix: sin tarjeta"))
		return s
	case !tj.EstaAprobada():
		s.Lineas = append(s.Lineas, alerta("sin tarjeta aprobada: el Owner no aprobó cómo se entendió la historia"))
		return s
	}
	s.Fuente += " · aprobada " + tj.Aprobada.Local().Format("2006-01-02 15:04")
	for _, x := range []struct {
		nombre string
		lineas []string
	}{{"fuera", tj.Fuera}, {"examen", tj.Examen}, {"zonas", tj.Zonas}, {"preguntas", tj.Preguntas}} {
		s.Lineas = append(s.Lineas, normal("%s: %s", x.nombre, strings.Join(x.lineas, " · ")))
	}
	for _, ev := range pd.ExamenTrasAprobar {
		s.Lineas = append(s.Lineas, alerta("examen cambiado tras aprobar la tarjeta (%s): %s · aprobado: %s", ev.TS.Local().Format("2006-01-02 15:04"), archivosDe(ev), ev.Detalle))
	}
	for _, f := range pd.FueraDeTarjeta {
		s.Lineas = append(s.Lineas, alerta("fuera de las zonas de la tarjeta: %s", f))
	}
	return s
}

// archivosDe lista los archivos de un evento de examen del diario.
func archivosDe(ev diario.Evento) string {
	var fs []string
	if xs, ok := ev.Datos["archivos"].([]any); ok {
		for _, x := range xs {
			fs = append(fs, fmt.Sprint(x))
		}
	}
	return strings.Join(fs, ", ")
}
