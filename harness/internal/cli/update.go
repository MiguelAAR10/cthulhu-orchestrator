package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"orq/internal/app"
	"orq/internal/gate"
	"orq/internal/sandbox"
)

// `orq2 update` (orq2 only) replaces ~/bin/orq2 with a build of the harness-v2 tip of the Cthulhu
// repo, and only when:
//   - the Owner runs it from a terminal (not a role panel, a worker directory or the box);
//   - that exact tip SHA has a gate record: independent review with no High and `make ci` exit 0
//     (git notes on refs/notes/orq2-gate, see app/update.go; the integrator writes them with
//     `orq2 update --registrar`);
//   - no slice of any project it can see is being verified or closed.
//
// The current binary is kept as rollback in ~/.local/share/orq2-rollback/orq2-anterior.

// esTerminal says whether stdin is a terminal. Tests replace it.
var esTerminal = esTerminalReal

// esTerminalReal asks the kernel (TCGETS) whether r is a terminal: a character device is not
// enough, /dev/null is one too.
func esTerminalReal(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok {
		return false
	}
	var t syscall.Termios
	_, _, e := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), uintptr(syscall.TCGETS), uintptr(unsafe.Pointer(&t)))
	return e == 0
}

const usoUpdate = `uso: orq2 update             instala la punta harness-v2 si su SHA tiene registro de puerta
     orq2 update --ver       solo dice si hay una versión nueva con puerta (no escribe nada)
     orq2 update --volver    pone de vuelta la versión anterior guardada
     orq2 update --registrar SHA --revisor QUIEN --veredicto sin-high --ci-exit 0 --ci-log REF
`

// prepararUpdate gives the configuration update runs with. Tests replace it; nothing outside the
// binary (env, flags) can.
var prepararUpdate = func() (app.ConfUpdate, error) { return app.LeerConfUpdate(), nil }

// runUpdate is `orq2 update`. Run only reaches it under identity orq2.
func runUpdate(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	c, err := prepararUpdate()
	if err != nil {
		fmt.Fprintln(stderr, "orq2 update:", err)
		return 1
	}
	if len(args) > 0 && args[0] == "--registrar" {
		return runRegistrarPuerta(c, args[1:], stdout, stderr)
	}
	modo := ""
	switch {
	case len(args) == 0:
	case len(args) == 1 && (args[0] == "--ver" || args[0] == "--volver"):
		modo = args[0]
	default:
		fmt.Fprint(stderr, usoUpdate)
		return 1
	}
	if err := soloOwner(stdin); err != nil {
		fmt.Fprintln(stderr, "orq2 update:", err)
		return 1
	}
	switch modo {
	case "--ver":
		return updateVer(c, stdout, stderr)
	case "--volver":
		return updateVolver(c, stdout, stderr)
	}
	return updateInstalar(c, stdout, stderr)
}

// soloOwner refuses anything that is not the Owner at a terminal.
func soloOwner(stdin io.Reader) error {
	if rol := os.Getenv("ORQ_ROL"); rol != "" {
		return fmt.Errorf("es del Owner; este panel es de %s. Córrelo en la terminal del Owner, fuera de los paneles de agentes", rol)
	}
	if sandbox.Dentro() {
		return errors.New("es del Owner; no corre dentro de la caja")
	}
	if cwd, err := os.Getwd(); err != nil || gate.IsWorker(cwd) {
		return errors.New("es del Owner; esta carpeta es de un worker. Córrelo desde la terminal del Owner")
	}
	if !esTerminal(stdin) {
		return errors.New("es del Owner, desde su terminal: la entrada no es una terminal")
	}
	return nil
}

// runRegistrarPuerta writes the gate record of one SHA. It is the integrator's: it may run from
// an agent session, never from a worker or inside the box. It only records a pass.
func runRegistrarPuerta(c app.ConfUpdate, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		fmt.Fprint(stderr, usoUpdate)
		return 1
	}
	ref, vals, resto := args[0], map[string]string{}, args[1:]
	for i := 0; i < len(resto); i += 2 {
		k := resto[i]
		if !slices.Contains([]string{"--revisor", "--veredicto", "--ci-exit", "--ci-log"}, k) || i+1 >= len(resto) {
			fmt.Fprint(stderr, usoUpdate)
			return 1
		}
		vals[k] = resto[i+1]
	}
	if strings.HasPrefix(os.Getenv("ORQ_ROL"), "worker") || sandbox.Dentro() {
		fmt.Fprintln(stderr, "orq2 update --registrar: es del integrador; un worker no registra puertas")
		return 1
	}
	if cwd, err := os.Getwd(); err != nil || gate.IsWorker(cwd) {
		fmt.Fprintln(stderr, "orq2 update --registrar: es del integrador; esta carpeta es de un worker")
		return 1
	}
	switch {
	case vals["--veredicto"] != app.VeredictoSinHigh:
		fmt.Fprintf(stderr, "orq2 update --registrar: solo se registra una revisión independiente sin High (--veredicto %s); recibí %q\n", app.VeredictoSinHigh, vals["--veredicto"])
		return 1
	case vals["--ci-exit"] != "0":
		fmt.Fprintf(stderr, "orq2 update --registrar: solo se registra make ci con exit 0 (--ci-exit 0); recibí %q\n", vals["--ci-exit"])
		return 1
	case strings.TrimSpace(vals["--revisor"]) == "" || strings.TrimSpace(vals["--ci-log"]) == "":
		fmt.Fprintln(stderr, "orq2 update --registrar: faltan --revisor y --ci-log")
		return 1
	}
	sha, err := c.RegistrarPuerta(ref, vals["--revisor"], vals["--ci-log"], time.Now())
	if err != nil {
		fmt.Fprintln(stderr, "orq2 update --registrar: no se escribió:", err)
		return 1
	}
	fmt.Fprintf(stdout, "registro de puerta de %s escrito en refs/notes/%s de %s\n", sha, app.RefPuerta, c.Repo)
	return 0
}

// negarSiOcupado prints the refusal and returns true when something is verifying or closing.
func negarSiOcupado(c app.ConfUpdate, stderr io.Writer) (app.Ocupacion, bool) {
	o := c.RevisarOcupados()
	if len(o.Motivos) == 0 {
		return o, false
	}
	fmt.Fprintln(stderr, "orq2 update: no se cambia orq2 mientras un slice se verifica o se cierra:")
	for _, m := range o.Motivos {
		fmt.Fprintln(stderr, "  -", m)
	}
	fmt.Fprintf(stderr, "proyectos que veo: %s\n(los busco en %s)\nVuelve a intentarlo cuando terminen.\n", o.Proyectos(), app.FuentesProyectos)
	return o, true
}

func updateInstalar(c app.ConfUpdate, stdout, stderr io.Writer) int {
	punta, err := c.Punta()
	if err != nil {
		fmt.Fprintln(stderr, "orq2 update:", err)
		return 1
	}
	if _, err := c.LeerPuerta(punta); err != nil {
		fmt.Fprintf(stderr, "orq2 update: la punta %s (%s) no tiene registro de puerta válido: %v.\n"+
			"Solo se instala un SHA con revisión independiente sin High y make ci exit 0 para ese SHA exacto;\n"+
			"el integrador lo registra con: orq2 update --registrar %s --revisor QUIEN --veredicto sin-high --ci-exit 0 --ci-log REF\n",
			punta, c.Rama, err, punta)
		return 1
	}
	if _, no := negarSiOcupado(c, stderr); no {
		return 1
	}
	viejo := app.RevisionDe(c.Instalado())
	if viejo.Limpia(punta) {
		fmt.Fprintf(stdout, "orq2 ya está en la punta %s (%s); nada que hacer\n", punta, c.Rama)
		return 0
	}
	dir, err := os.MkdirTemp("", "orq2-update-")
	if err != nil {
		fmt.Fprintln(stderr, "orq2 update:", err)
		return 1
	}
	defer os.RemoveAll(dir)
	fmt.Fprintf(stdout, "construyendo %s de %s en un clon limpio…\n", punta, c.Repo)
	cand, err := c.ConstruirCandidato(punta, dir)
	if err != nil {
		fmt.Fprintln(stderr, "orq2 update: no se instala nada:", err)
		return 1
	}
	// The build may have taken minutes: look again right before swapping.
	o, no := negarSiOcupado(c, stderr)
	if no {
		return 1
	}
	if viejo.Existe {
		if err := app.CopiarAtomico(c.Instalado(), c.RutaRollback()); err != nil {
			fmt.Fprintln(stderr, "orq2 update: no se pudo guardar la versión actual; no se instala nada:", err)
			return 1
		}
	}
	if err := app.CopiarAtomico(cand, c.Instalado()); err != nil {
		fmt.Fprintln(stderr, "orq2 update: no se pudo instalar (el binario actual sigue intacto):", err)
		return 1
	}
	fmt.Fprintf(stdout, "proyectos revisados: %s\n", o.Proyectos())
	fmt.Fprintf(stdout, "orq2 actualizado: %s -> %s\n", viejo.Rev, punta)
	if viejo.Existe {
		fmt.Fprintf(stdout, "la anterior quedó en %s; para volver: orq2 update --volver\n", c.RutaRollback())
	}
	if r := app.RevisionDe(c.Instalado()); !r.Limpia(punta) {
		fmt.Fprintf(stderr, "orq2 update: el instalado dice %q; vuelve con orq2 update --volver\n", r.Linea)
		return 1
	}
	return 0
}

func updateVolver(c app.ConfUpdate, stdout, stderr io.Writer) int {
	if _, err := os.Stat(c.RutaRollback()); err != nil {
		fmt.Fprintf(stderr, "orq2 update --volver: no hay versión anterior guardada en %s\n", c.RutaRollback())
		return 1
	}
	if _, no := negarSiOcupado(c, stderr); no {
		return 1
	}
	actual, anterior := app.RevisionDe(c.Instalado()), app.RevisionDe(c.RutaRollback())
	if err := app.CopiarAtomico(c.RutaRollback(), c.Instalado()); err != nil {
		fmt.Fprintln(stderr, "orq2 update --volver: no se pudo restaurar (el binario actual sigue intacto):", err)
		return 1
	}
	fmt.Fprintf(stdout, "orq2 vuelto atrás: %s -> %s\n", actual.Rev, anterior.Rev)
	return 0
}

// updateVer only reads: the installed revision, the tip and the gate records.
func updateVer(c app.ConfUpdate, stdout, stderr io.Writer) int {
	punta, err := c.Punta()
	if err != nil {
		fmt.Fprintln(stderr, "orq2 update --ver:", err)
		return 1
	}
	viejo := app.RevisionDe(c.Instalado())
	fmt.Fprintf(stdout, "instalada: %s (%s)\n", viejo.Linea, c.Instalado())
	reg, errP := c.LeerPuerta(punta)
	if errP == nil {
		fmt.Fprintf(stdout, "punta %s: %s · con registro de puerta (revisor %s, make ci %s, %s)\n", c.Rama, punta, reg.Revision.Revisor, reg.CI.Log, reg.Fecha)
	} else {
		fmt.Fprintf(stdout, "punta %s: %s · sin registro de puerta (%v)\n", c.Rama, punta, errP)
	}
	switch {
	case viejo.Limpia(punta):
		fmt.Fprintln(stdout, "estás en la punta: no hay versión nueva")
	case errP == nil:
		fmt.Fprintln(stdout, "hay una versión nueva con puerta: orq2 update la instala")
	default:
		if g := c.MasNuevaConPuerta(punta, viejo.Rev); g != "" {
			fmt.Fprintf(stdout, "la más nueva con puerta después de la instalada es %s, pero orq2 update solo instala la punta: falta el registro de %s\n", g, punta)
		} else {
			fmt.Fprintln(stdout, "no hay una versión nueva con puerta")
		}
	}
	return 0
}
