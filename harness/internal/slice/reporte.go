package slice

import (
	"bufio"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
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
