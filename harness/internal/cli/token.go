package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"orq/internal/gate"
	"orq/internal/identidad"
)

// rolesConToken: los únicos roles a los que orq entrega el token del Owner.
var rolesConToken = map[string]bool{"": true, "owner": true, "atena": true, "orquestador": true, "orchestrator": true}

// runToken imprime el token del Owner (lo crea si falta) para `export ORQ_TOKEN="$(orq token)"`.
// Es una acción del Owner en su terminal: un worker nunca la obtiene (el gate además lo bloquea).
func runToken(stdout, stderr io.Writer) int {
	wd, _ := os.Getwd()
	if os.Getenv("ORQ_WORKER_ID") != "" || os.Getenv("ORQ_SHIM") != "" || !rolesConToken[os.Getenv("ORQ_ROL")] || gate.IsLinkedWorktree(wd) {
		fmt.Fprintln(stderr, "orq token: only the Owner, from a terminal outside any worker, gets the token")
		return 1
	}
	t, err := identidad.Asegurar()
	if err != nil {
		fmt.Fprintln(stderr, "orq token:", err)
		return 1
	}
	fmt.Fprintln(stdout, t)
	return 0
}

// escribirShim deja .orq/workers/<id>/bin/orq: quita ORQ_TOKEN, fija ORQ_WORKER_ID y marca ORQ_SHIM,
// así Run aplica la lista blanca de worker (gate.OrqPermitidoAWorker) aunque el agente no tenga hook.
func escribirShim(orqDir, id string) (string, error) {
	real, err := os.Executable()
	if err != nil {
		return "", err
	}
	bin := filepath.Join(orqDir, "workers", id, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		return "", err
	}
	sh := fmt.Sprintf("#!/bin/sh\n# orq shim for worker %s: drops the Owner's token and keeps the worker identity.\nunset ORQ_TOKEN\nORQ_WORKER_ID=%s ORQ_SHIM=1 exec %s \"$@\"\n", id, shQuote(id), shQuote(real))
	return bin, os.WriteFile(filepath.Join(bin, "orq"), []byte(sh), 0o755)
}

func shQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

// limiteDeWorker: dentro de un worker (ORQ_WORKER_ID o el shim) orq mismo aplica la lista blanca.
func limiteDeWorker(args []string, stderr io.Writer) bool {
	if os.Getenv("ORQ_WORKER_ID") == "" && os.Getenv("ORQ_SHIM") == "" {
		return false
	}
	if gate.OrqPermitidoAWorker(args) {
		return false
	}
	fmt.Fprintf(stderr, "orq: a worker cannot run `orq %s`; ask the Orchestrator\n", strings.Join(args, " "))
	return true
}
