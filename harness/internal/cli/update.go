package cli

import (
	"fmt"
	"io"
	"os"
)

// refPuerta is the git notes ref that holds the gate records.
const refPuerta = "orq2-gate"

// esTerminal says whether stdin is a terminal. Tests replace it.
var esTerminal = esTerminalReal

func esTerminalReal(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

// runUpdate is `orq2 update`. Stub: not built yet.
func runUpdate(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fmt.Fprintln(stderr, "orq2 update: no implementado")
	return 1
}
