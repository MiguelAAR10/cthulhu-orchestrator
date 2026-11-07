package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"time"
)

func runMetricas(args []string, stdout, stderr io.Writer) int {
	p, err := abrirProyecto()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	var desde time.Time
	jsonOut := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			jsonOut = true
		case "--desde":
			if i+1 < len(args) {
				desde, _ = time.Parse("2006-01-02", args[i+1])
				i++
			}
		}
	}
	m := p.slices().Metricas(desde)
	if jsonOut {
		raw, _ := json.MarshalIndent(m, "", "  ")
		fmt.Fprintln(stdout, string(raw))
		return 0
	}
	cerrados := m.Aceptados + m.Rechazados
	fmt.Fprintf(stdout, "slices aceptados e integrados: %d · rechazados: %d\n", m.Aceptados, m.Rechazados)
	if cerrados >= 5 {
		fmt.Fprintf(stdout, "aceptados al primer intento: %.0f %%\n", 100*float64(m.VerdesALaPrimera)/float64(max(m.Aceptados, 1)))
	} else {
		fmt.Fprintf(stdout, "aceptados al primer intento: %d (con menos de 5 slices no se dan porcentajes)\n", m.VerdesALaPrimera)
	}
	fmt.Fprintf(stdout, "intentos: %d · verificaciones: %d · fallos sin progreso: %d\n", m.Intentos, m.Verificaciones, m.FallosSinProgreso)
	fmt.Fprintf(stdout, "horas de punta a punta (media, aceptados): %.1f\n", m.HorasMedias)
	fmt.Fprintf(stdout, "en cuarentena: %d · recursos sobrantes: `orq doctor`\n", m.Cuarentena)
	fmt.Fprintln(stdout, "costo por slice y tiempo humano: n/d (orq no los registra todavía)")
	return 0
}
