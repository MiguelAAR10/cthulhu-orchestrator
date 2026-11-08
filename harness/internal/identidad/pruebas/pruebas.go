// Package pruebas prepara, para los TestMain, un Owner hermético: config y estado temporales,
// token, ORQ_TOKEN y el proceso de prueba anclado. Nunca toca la config real.
package pruebas

import (
	"os"

	"orq/internal/identidad"
)

func Owner() {
	cfg, _ := os.MkdirTemp("", "orq-cfg")
	st, _ := os.MkdirTemp("", "orq-state")
	os.Setenv("XDG_CONFIG_HOME", cfg)
	os.Setenv("ORQ_STATE", st)
	tok, err := identidad.Asegurar()
	if err != nil {
		panic(err)
	}
	os.Setenv("ORQ_TOKEN", tok)
	if identidad.Tabla != nil {
		if _, err := identidad.Anclar(identidad.Tabla, os.Getpid(), "owner"); err != nil {
			panic(err)
		}
	}
}
