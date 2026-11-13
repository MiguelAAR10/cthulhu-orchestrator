package cli

import (
	"fmt"
	"io"

	"orq/internal/config"
	"orq/internal/orqdir"
)

// runConfig muestra la configuración efectiva y de dónde salió cada valor.
func runConfig(args []string, stdout, stderr io.Writer) int {
	if len(args) > 1 || len(args) == 1 && args[0] != "ver" {
		fmt.Fprintln(stderr, "uso: orq config ver")
		return 1
	}
	root, _ := orqdir.Root() // fuera de un repo solo vale la capa del usuario
	c, err := config.Cargar(root)
	if err != nil {
		fmt.Fprintln(stderr, "orq:", err)
		return 1
	}
	fmt.Fprintln(stdout, "capas (la última gana campo por campo):")
	fmt.Fprintln(stdout, "  defecto   valores de orq "+Version)
	for i, a := range c.Archivos() {
		capa, estado := "usuario ", "no existe"
		if i > 0 {
			capa = "proyecto"
		}
		if a.Existe {
			estado = "leído"
		}
		fmt.Fprintf(stdout, "  %s  %s (%s)\n", capa, a.Ruta, estado)
	}
	fmt.Fprintln(stdout)
	for _, l := range c.Lineas() {
		fmt.Fprintf(stdout, "%-34s = %-16s (%s)\n", l.Clave, l.Valor, l.Origen)
	}
	// Con qué cuenta arranca cada uno: las variables de `entorno` que el lanzador pone en su panel.
	fmt.Fprintln(stdout, "\ncuentas:")
	for _, rol := range config.Roles {
		a := c.Rol(rol)
		fmt.Fprintf(stdout, "  %-13s %-7s %s\n", rol, a.Kind, config.Cuenta(a.Entorno))
	}
	for _, dif := range config.Dificultades {
		for _, kind := range config.Kinds {
			fmt.Fprintf(stdout, "  worker %-6s %-7s %s\n", dif, kind, config.Cuenta(c.WorkerEntorno(kind, dif)))
		}
	}
	return 0
}
