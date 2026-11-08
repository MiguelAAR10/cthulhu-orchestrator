package cli

import (
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"orq/internal/instinto"
)

func (p proyecto) instPath() string { return filepath.Join(p.dir, "instintos.json") }

// aprenderSolo registra una lección sin frenar el comando que la produjo.
func (p proyecto) aprenderSolo(texto, fuente, evidencia string) {
	instinto.Aprender(p.instPath(), recortar(texto, instinto.MaxTexto), fuente, evidencia, time.Now())
}

// lecciones es lo que entra al arrancar: pocas, confiables, del proyecto y globales.
func (p proyecto) lecciones() string {
	top := instinto.Top(instinto.Inyectar, p.instPath(), instinto.Global())
	if len(top) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n## Lecciones aprendidas (aplícalas; si una falla: orq aprender refutar ID)\n")
	for _, x := range top {
		fmt.Fprintf(&b, "- [%s %.2f] %s\n", x.ID, x.Conf, x.Texto)
	}
	return b.String()
}

func runAprender(args []string, stdout, stderr io.Writer) int {
	p, err := abrirProyecto()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	path := p.instPath()
	if len(args) > 0 && args[0] == "--global" {
		path, args = instinto.Global(), args[1:]
	}
	if len(args) == 0 {
		args = []string{"ver"}
	}
	uso := "uso: orq aprender [--global] ver | add \"lección\" [--evidencia E] | confirmar ID | refutar ID | promover ID"
	switch args[0] {
	case "ver":
		l, err := instinto.Leer(path)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if len(l) == 0 {
			fmt.Fprintln(stdout, "sin lecciones todavía: se aprenden solas de reintentos, rechazos y desalineaciones")
		}
		for _, x := range l {
			marca := "hipótesis"
			if x.Conf >= instinto.Umbral {
				marca = "se inyecta"
			}
			fmt.Fprintf(stdout, "%s %.2f ×%d %-10s %s  (%s)\n", x.ID, x.Conf, x.Visto, x.Fuente, x.Texto, marca)
		}
		return 0
	case "add":
		fs := nuevoFS("aprender add", flag.ContinueOnError)
		fs.SetOutput(stderr)
		ev := fs.String("evidencia", "", "recibo, commit o S-NN/aK que la respalda")
		fuente := fs.String("fuente", "orquestador", "quién la aprendió")
		texto := strings.Join(flagsAlFinal(fs, args[1:]), " ")
		if texto == "" {
			fmt.Fprintln(stderr, uso)
			return 1
		}
		x, err := instinto.Aprender(path, texto, *fuente, *ev, time.Now())
		if err != nil {
			fmt.Fprintln(stderr, "orq aprender:", err)
			return 1
		}
		fmt.Fprintf(stdout, "%s confianza %.2f (se inyecta desde %.2f)\n", x.ID, x.Conf, instinto.Umbral)
		return 0
	case "confirmar", "refutar":
		if len(args) != 2 {
			fmt.Fprintln(stderr, uso)
			return 1
		}
		d := instinto.Confirmar
		if args[0] == "refutar" {
			d = -instinto.Refutar
		}
		x, vive, err := instinto.Ajustar(path, args[1], d)
		if err != nil {
			fmt.Fprintln(stderr, "orq aprender:", err)
			return 1
		}
		if !vive {
			fmt.Fprintf(stdout, "%s podada: ya no es confiable\n", x.ID)
		} else {
			fmt.Fprintf(stdout, "%s confianza %.2f\n", x.ID, x.Conf)
		}
		return 0
	case "promover":
		if len(args) != 2 {
			fmt.Fprintln(stderr, uso)
			return 1
		}
		g, err := instinto.Promover(p.instPath(), instinto.Global(), args[1], time.Now())
		if err != nil {
			fmt.Fprintln(stderr, "orq aprender:", err)
			return 1
		}
		fmt.Fprintf(stdout, "global %s (%.2f): vale para todos tus proyectos\n", g.ID, g.Conf)
		return 0
	}
	fmt.Fprintln(stderr, uso)
	return 1
}

// flagsAlFinal deja escribir el texto antes o después de las opciones.
func flagsAlFinal(fs *flag.FlagSet, args []string) []string {
	var texto, opts []string
	for i := 0; i < len(args); i++ {
		if strings.HasPrefix(args[i], "--") {
			opts = append(opts, args[i])
			if !strings.Contains(args[i], "=") && i+1 < len(args) {
				opts = append(opts, args[i+1])
				i++
			}
		} else {
			texto = append(texto, args[i])
		}
	}
	if fs.Parse(opts) != nil {
		return nil
	}
	return texto
}

func runAlinear(args []string, stdout, stderr io.Writer) int {
	p, err := abrirProyecto()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	nn := ""
	if len(args) == 2 && args[0] == "--slice" {
		nn = strings.TrimPrefix(args[1], "S-")
	}
	return p.alinear(nn, stdout, stderr)
}

// alinear devuelve 3 si hay desvío: el Orquestador debe preguntar al Atena antes de seguir.
func (p proyecto) alinear(nn string, stdout, stderr io.Writer) int {
	e, a, err := p.slices().Alinear(nn)
	if err != nil {
		fmt.Fprintln(stderr, "orq alinear:", err)
		return 1
	}
	if len(a.Avisos) == 0 {
		fmt.Fprintf(stdout, "%s alineado: %d archivo(s), todos dentro de lo que la historia justifica\n", e.Attempt(), len(a.Cambiados))
		return 0
	}
	fmt.Fprintf(stdout, "⚠ %s posible desalineación con la historia «%s»:\n", e.Attempt(), e.Titulo)
	for _, x := range a.Avisos {
		fmt.Fprintln(stdout, "  - "+x)
	}
	fmt.Fprintf(stdout, "Antes de seguir, pregunta al Atena:\n  orq msg send --para atena --tipo pregunta \"%s toca %s. ¿Esto sirve al objetivo o nos desviamos?\"\n",
		e.Attempt(), strings.Join(cabeza(a.Fuera, 3), ", "))
	p.aprenderSolo("Antes de tocar fuera del alcance de la historia, preguntar al Atena", "alineacion", e.Attempt())
	return 3
}

func cabeza(l []string, n int) []string {
	if len(l) > n {
		return l[:n]
	}
	return l
}
