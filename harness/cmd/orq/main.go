// orq: harness determinista de orquestador-2.
// Hace cumplir límites de disco, workers y archivos protegidos aunque el modelo se equivoque.
package main

import (
	"os"

	"orq/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
