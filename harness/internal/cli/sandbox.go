package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"orq/internal/sandbox"
)

const usoSandbox = `uso: orq sandbox selftest
     orq sandbox exec [--rw DIR]... [--ro DIR]... [--red] [--cwd DIR] -- CMD ARGS...
`

func runSandbox(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usoSandbox)
		return 1
	}
	switch args[0] {
	case "selftest":
		return runSandboxSelftest(stdout, stderr)
	case "exec":
		return runSandboxExec(args[1:], stdin, stdout, stderr)
	default:
		fmt.Fprint(stderr, usoSandbox)
		return 1
	}
}

func runSandboxSelftest(stdout, stderr io.Writer) int {
	ss, err := sandbox.Sondas()
	if err != nil {
		fmt.Fprintf(stderr, "orq: %v\n", err)
		return 1
	}
	code := 0
	for _, s := range ss {
		marca := "✔"
		if !s.OK {
			marca, code = "✘", 1
		}
		det := ""
		if s.Detalle != "" {
			det = " · " + s.Detalle
		}
		fmt.Fprintf(stdout, "  %s %s%s\n", marca, s.Nombre, det)
	}
	if code == 0 {
		fmt.Fprintln(stdout, "sandbox: autoprueba OK")
	} else {
		fmt.Fprintln(stdout, "sandbox: autoprueba FALLA; no corre nada encerrado hasta arreglarlo")
	}
	return code
}

// parseSandboxExec separa las opciones del comando, que va siempre después de --.
func parseSandboxExec(args []string) (sandbox.Spec, []string, error) {
	var s sandbox.Spec
	abs := func(p string) (string, error) {
		if p == "" {
			return "", errors.New("ruta vacía")
		}
		return filepath.Abs(p)
	}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			if i+1 >= len(args) {
				return s, nil, errors.New("falta el comando después de --")
			}
			return s, args[i+1:], nil
		}
		val := func() (string, error) {
			if k, v, ok := strings.Cut(a, "="); ok && strings.HasPrefix(k, "--") {
				return abs(v)
			}
			if i+1 >= len(args) {
				return "", fmt.Errorf("%s sin valor", a)
			}
			i++
			return abs(args[i])
		}
		nombre, _, _ := strings.Cut(a, "=")
		switch nombre {
		case "--rw", "--ro", "--cwd":
			v, err := val()
			if err != nil {
				return s, nil, err
			}
			switch nombre {
			case "--rw":
				s.RW = append(s.RW, v)
			case "--ro":
				s.RO = append(s.RO, v)
			default:
				s.Cwd = v
			}
		case "--red":
			s.Net = true
		default:
			return s, nil, fmt.Errorf("opción desconocida %q (el comando va después de --)", a)
		}
	}
	return s, nil, errors.New("falta -- y el comando")
}

// cwdPorDefecto: el cwd actual si queda dentro de lo montado; si no, el primer --rw; si no, $HOME.
func cwdPorDefecto(s sandbox.Spec) string {
	if wd, err := os.Getwd(); err == nil {
		for _, p := range append(append([]string{}, s.RW...), s.RO...) {
			if wd == p || strings.HasPrefix(wd, p+"/") {
				return wd
			}
		}
	}
	if len(s.RW) > 0 {
		return s.RW[0]
	}
	return ""
}

func runSandboxExec(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	s, argv, err := parseSandboxExec(args)
	if err != nil {
		fmt.Fprintf(stderr, "orq sandbox exec: %v\n%s", err, usoSandbox)
		return 1
	}
	if s.Cwd == "" {
		s.Cwd = cwdPorDefecto(s)
	}
	s.Env = map[string]string{}
	for _, k := range []string{"TERM", "LANG"} {
		if v := os.Getenv(k); v != "" {
			s.Env[k] = v
		}
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	cmd, err := sandbox.Command(ctx, s, argv...)
	if err != nil {
		fmt.Fprintf(stderr, "orq: %v\n", err)
		return 1
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = stdin, stdout, stderr
	err = cmd.Run()
	var ee *exec.ExitError
	switch {
	case err == nil:
		return 0
	case errors.As(err, &ee):
		if st, ok := ee.Sys().(syscall.WaitStatus); ok && st.Signaled() {
			return 128 + int(st.Signal())
		}
		return ee.ExitCode()
	default:
		fmt.Fprintf(stderr, "orq: sandbox: %v\n", err)
		return 1
	}
}
