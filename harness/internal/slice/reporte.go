package slice

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"orq/internal/orqdir"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Conteo es lo que el runner dice que ejecutó. Sin conteo, un verde puede ser "no corrió nada".
type Conteo struct{ Ejecutados, Omitidos, Fallidos int }

var Formatos = []string{"tap", "junit", "gotest-json"}

// Contar lee el reporte del runner en uno de los formatos soportados.
func Contar(formato string, raw []byte) (Conteo, error) {
	var c Conteo
	switch formato {
	case "tap":
		sc := bufio.NewScanner(bytes.NewReader(raw))
		for sc.Scan() {
			l := strings.TrimSpace(sc.Text())
			skip := strings.Contains(strings.ToUpper(l), "# SKIP") || strings.Contains(strings.ToUpper(l), "# TODO")
			switch {
			case strings.HasPrefix(l, "not ok") && skip, strings.HasPrefix(l, "ok") && skip:
				c.Omitidos++
			case strings.HasPrefix(l, "not ok"):
				c.Ejecutados++
				c.Fallidos++
			case strings.HasPrefix(l, "ok"):
				c.Ejecutados++
			}
		}
	case "junit":
		dec := xml.NewDecoder(bytes.NewReader(raw))
		var enCaso, marcado bool
		for {
			tok, err := dec.Token()
			if err != nil {
				break
			}
			switch t := tok.(type) {
			case xml.StartElement:
				switch t.Name.Local {
				case "testcase":
					enCaso, marcado = true, false
				case "skipped":
					if enCaso && !marcado {
						c.Omitidos++
						marcado = true
					}
				case "failure", "error":
					if enCaso && !marcado {
						c.Ejecutados++
						c.Fallidos++
						marcado = true
					}
				}
			case xml.EndElement:
				if t.Name.Local == "testcase" {
					if !marcado {
						c.Ejecutados++
					}
					enCaso = false
				}
			}
		}
	case "gotest-json":
		sc := bufio.NewScanner(bytes.NewReader(raw))
		for sc.Scan() {
			var ev struct{ Action, Test string }
			if json.Unmarshal(sc.Bytes(), &ev) != nil || ev.Test == "" {
				continue
			}
			switch ev.Action {
			case "pass":
				c.Ejecutados++
			case "fail":
				c.Ejecutados++
				c.Fallidos++
			case "skip":
				c.Omitidos++
			}
		}
	default:
		return c, fmt.Errorf("formato %q: usa %s", formato, strings.Join(Formatos, "|"))
	}
	return c, nil
}

// TimeoutDefecto corta exámenes colgados.
const TimeoutDefecto = 600 * time.Second

// RunCmd corre un comando con sh y el tiempo por defecto.
func RunCmd(dir, cmd string) (string, int, bool) { return RunCmdT(dir, cmd, TimeoutDefecto) }

// RunCmdT corre el comando en su propio grupo de procesos; al vencer el tiempo mata el grupo entero.
func RunCmdT(dir, cmd string, limite time.Duration) (string, int, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), limite)
	defer cancel()
	c := exec.CommandContext(ctx, "sh", "-c", cmd)
	c.Dir = dir
	c.SysProcAttr = orqdir.GrupoPropio()
	c.Cancel = func() error { return orqdir.MatarGrupo(c.Process.Pid) }
	c.WaitDelay = 2 * time.Second
	var buf bytes.Buffer
	c.Stdout, c.Stderr = &buf, &buf
	err := c.Run()
	out := buf.String()
	if ctx.Err() != nil {
		return out + fmt.Sprintf("\n[orq] cortado tras %s\n", limite), -1, false
	}
	code := 0
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		code = ee.ExitCode()
	} else if err != nil {
		code = -1
	}
	return out, code, err == nil
}

// copiaAislada extrae el commit en una carpeta temporal propia: sin archivos ignorados ni sin commit.
func copiaAislada(worktree, head string) (string, error) {
	dir, err := os.MkdirTemp("", "orq-exec-")
	if err != nil {
		return "", err
	}
	c := exec.Command("sh", "-c", fmt.Sprintf("git archive %s | tar -x -C %q", head, dir))
	c.Dir = worktree
	if out, err := c.CombinedOutput(); err != nil {
		os.RemoveAll(dir)
		return "", fmt.Errorf("copia aislada: %s", out)
	}
	return dir, nil
}
