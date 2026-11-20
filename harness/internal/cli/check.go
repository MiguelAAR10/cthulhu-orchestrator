package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"orq/internal/app"
	"orq/internal/sandbox"
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

// runCheckCaja corre, por check, una sonda de cada herramienta dentro de la caja real de orq slice
// verificar (command -v y su versión) e informa lo que vio. Sale 1 si alguna no corre.
func runCheckCaja(stdout, stderr io.Writer) int {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	cs, sinE2E, err := app.ChecksCaja(cwd)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	code := 0
	for _, c := range cs {
		hs := c.Herramientas
		if c.Navegadores != nil {
			hs = append(hs, *c.Navegadores)
		}
		if c.NoCorreUv != "" {
			code = 1
			fmt.Fprintf(stdout, "%s  no corre en la caja: %s\n", c.Nombre, c.NoCorreUv)
			continue
		}
		if no := sandbox.NoCorren(hs); no != "" {
			code = 1
			fmt.Fprintf(stdout, "%s  no corre en la caja: %s\n", c.Nombre, no)
			continue
		}
		var vistas, fallas []string
		for _, h := range c.Herramientas {
			s, err := sandbox.Sondear(h)
			switch {
			case err != nil:
				fallas = append(fallas, fmt.Sprintf("la sonda de %s no arrancó: %v", h.Nombre, err))
			case !s.Corre():
				fallas = append(fallas, fmt.Sprintf("la sonda de %s sale %d: %s", h.Nombre, s.Exit, s.Linea))
			default:
				v := h.Nombre + " → " + s.Ruta
				if s.Linea != "" {
					v += " · " + s.Linea
				}
				if s.Exit != 0 {
					v += fmt.Sprintf(" (exit %d)", s.Exit)
				}
				vistas = append(vistas, v)
			}
		}
		switch {
		case c.ErrUv != nil:
			fallas = append(fallas, fmt.Sprintf("la sonda `%s` no arrancó: %v", c.SondaUv, c.ErrUv))
		case c.SondaUv != "" && !c.OkUv:
			fallas = append(fallas, fmt.Sprintf("la sonda `%s` falla: %s", c.SondaUv, c.LineaUv))
		case c.SondaUv != "":
			vistas = append(vistas, fmt.Sprintf("sonda `%s` → %s (no prueba que resuelva cada import: eso lo prueba el check)", c.SondaUv, c.LineaUv))
		}
		if len(fallas) > 0 {
			code = 1
			fmt.Fprintf(stdout, "%s  no corre en la caja: %s\n", c.Nombre, strings.Join(fallas, "; "))
			continue
		}
		if c.Navegadores != nil {
			vistas = append(vistas, c.Navegadores.Nombre+" → "+c.Navegadores.Ruta)
		}
		monta := "nada fuera del sistema"
		ro := sandbox.RODe(hs)
		if c.VenvUv != "" {
			ro = append(ro, c.VenvUv)
		}
		if len(ro) > 0 {
			monta = strings.Join(ro, ", ")
		}
		fmt.Fprintf(stdout, "%s  corre en la caja: %s · monta %s\n", c.Nombre, strings.Join(vistas, "; "), monta)
	}
	if sinE2E != "" {
		fmt.Fprintf(stdout, "e2e  sin check: %s\n", sinE2E)
	}
	return code
}
