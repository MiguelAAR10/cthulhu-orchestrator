package cli

import (
	"flag"
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
	if _, w := workerEfectivo(); w || !rolesConToken[os.Getenv("ORQ_ROL")] || gate.IsLinkedWorktree(wd) {
		fmt.Fprintln(stderr, "orq token: only the Owner, from a terminal outside any worker, gets the token")
		return 1
	}
	// Primer arranque: quien crea el token es el Owner, y su shell (el padre) queda anclado como
	// sesión privilegiada. Con el token ya creado, solo lo obtiene una sesión confiable (ancla o
	// terminal de persona): un proceso huérfano sin TTY no lo vuelve a pedir.
	if identidad.Leer() != "" {
		if err := identidad.ConfiableAqui(); err != nil {
			fmt.Fprintln(stderr, "orq token:", err)
			return 1
		}
	}
	t, err := tokenInicial()
	if err != nil {
		fmt.Fprintln(stderr, "orq token:", err)
		return 1
	}
	fmt.Fprintln(stdout, t)
	return 0
}

// tokenInicial asegura el token; si lo crea ahora (primer arranque, `orq init` o `orq token`), quien
// lo crea es el Owner y su shell (el padre) queda anclado como sesión privilegiada.
func tokenInicial() (string, error) {
	nuevo := identidad.Leer() == ""
	t, err := identidad.Asegurar()
	if err == nil && nuevo && identidad.Tabla != nil {
		if _, w := workerEfectivo(); !w {
			_, err = identidad.Anclar(identidad.Tabla, os.Getppid(), "owner")
		}
	}
	return t, err
}

// runSesionRegistrar: `orq session register [--pid N] [--role owner|atena|orchestrator]`.
// Ancla un proceso (por defecto, el shell que llama) como sesión privilegiada: sus descendientes
// pueden ejercer roles privilegiados con el token aunque no tengan terminal. Exige lo mismo que
// una acción privilegiada: token y sesión confiable, y que el proceso anclado no sea un worker.
func runSesionRegistrar(args []string, stdout, stderr io.Writer) int {
	fs := nuevoFS("session register", flag.ContinueOnError)
	fs.SetOutput(stderr)
	pid := fs.Int("pid", os.Getppid(), "process to anchor (default: the calling shell)")
	rol := fs.String("role", os.Getenv("ORQ_ROL"), "owner|atena|orchestrator")
	if fs.Parse(args) != nil {
		return 1
	}
	if *rol == "" {
		*rol = "owner"
	}
	wd, _ := os.Getwd()
	if _, w := workerEfectivo(); w || gate.IsLinkedWorktree(wd) || !rolesConToken[*rol] {
		fmt.Fprintln(stderr, "orq session register: only the Owner, Atena or the Orchestrator register a session, never a worker")
		return 1
	}
	if err := identidad.Autorizado(); err != nil {
		fmt.Fprintln(stderr, "orq session register:", err)
		return 1
	}
	if identidad.Tabla == nil {
		fmt.Fprintln(stderr, "orq session register: no process table on this platform; the token alone is checked")
		return 1
	}
	if id, ok := identidad.WorkerAncestro(identidad.Tabla, *pid); ok {
		fmt.Fprintf(stderr, "orq session register: process %d descends from worker %q\n", *pid, id)
		return 1
	}
	if env, err := identidad.Tabla.Environ(*pid); err == nil {
		if id, ok := identidad.MarcaWorker(env); ok {
			fmt.Fprintf(stderr, "orq session register: process %d is worker %q\n", *pid, id)
			return 1
		}
	}
	a, err := identidad.Anclar(identidad.Tabla, *pid, *rol)
	if err != nil {
		fmt.Fprintln(stderr, "orq session register:", err)
		return 1
	}
	fmt.Fprintf(stdout, "registered %s session: pid %d (start %d)\n", a.Rol, a.PID, a.Start)
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

// workerEfectivo: quién es worker sin fiarse del entorno actual. Vale el propio ORQ_WORKER_ID o el
// shim, pero también el entorno inicial de cualquier ancestro (/proc/<pid>/environ): un hijo puede
// hacer unset/export de ORQ_* en su entorno, no en el de sus padres. Y un cwd dentro de un worktree
// enlazado de un slice (.orq/worktrees o el registro de slices) también es un worker.
func workerEfectivo() (string, bool) {
	if id := os.Getenv("ORQ_WORKER_ID"); id != "" {
		return id, true
	}
	if os.Getenv("ORQ_SHIM") != "" {
		return "shim", true
	}
	if id, ok := identidad.WorkerHeredado(); ok {
		return id, true
	}
	if wd, err := os.Getwd(); err == nil && worktreeDeOrq(wd) {
		return "worktree", true
	}
	return "", false
}

// limiteDeWorker: dentro de un worker (ORQ_WORKER_ID o el shim) orq mismo aplica la lista blanca.
func limiteDeWorker(args []string, stderr io.Writer) bool {
	if _, w := workerEfectivo(); !w {
		return false
	}
	if gate.OrqPermitidoAWorker(args) {
		return false
	}
	fmt.Fprintf(stderr, "orq: a worker cannot run `orq %s`; ask the Orchestrator\n", strings.Join(args, " "))
	return true
}
