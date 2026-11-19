package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"orq/internal/app"
	"orq/internal/slice"
)

// runCheck corre los checks del proyecto donde está el worker e imprime cada resultado con las
// primeras líneas de la herramienta. Sale 0 solo si ninguno falla.
func runCheck(stdout, stderr io.Writer) int {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	rs, err := app.Check(cwd)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	var pasan, fallan, omitidos int
	for _, r := range rs {
		switch r.Resultado {
		case slice.CheckPaso:
			pasan++
			fmt.Fprintf(stdout, "%s  paso  · %s\n", r.Nombre, r.Cmd)
		case slice.CheckFallo:
			fallan++
			fmt.Fprintf(stdout, "%s  fallo (exit %d)  · %s\n", r.Nombre, r.Exit, r.Cmd)
			for _, l := range strings.Split(r.Salida, "\n") {
				fmt.Fprintf(stdout, "    %s\n", l)
			}
		default:
			omitidos++
			fmt.Fprintf(stdout, "%s  omitido: %s\n", r.Nombre, r.Motivo)
		}
	}
	fmt.Fprintf(stdout, "%d pasan · %d fallan · %d omitidos (omitido no cuenta como pasado)\n", pasan, fallan, omitidos)
	if fallan > 0 {
		return 1
	}
	return 0
}
