package slice

import (
	"bufio"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"slices"
	"strings"
	"time"
)

// Conteo es lo que el runner dice que ejecutó. Sin conteo, un verde puede ser "no corrió nada".
// Fallaron y Pasaron nombran las pruebas: abrir guarda las que fallan en el rojo y verificar exige
// verlas pasar, así un test enfocado (only, grep, un alias) no deja fuera el examen.
type Conteo struct {
	Ejecutados, Omitidos, Fallidos int
	Fallaron, Pasaron              []string
}

// nombreTAP saca el nombre de una línea «ok 3 - nombre # directiva»: sin número, que cambia si corren menos.
func nombreTAP(l string) string {
	l = strings.TrimPrefix(strings.TrimPrefix(l, "not "), "ok")
	l = strings.TrimSpace(l)
	l = strings.TrimLeft(l, "0123456789")
	l = strings.TrimPrefix(strings.TrimSpace(l), "- ")
	if i := strings.Index(l, " # "); i >= 0 {
		l = l[:i]
	}
	return strings.TrimSpace(l)
}

func nombreJUnit(clase, nombre string) string {
	if clase == "" {
		return nombre
	}
	return clase + " › " + nombre
}

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
				c.Fallaron = append(c.Fallaron, nombreTAP(l))
			case strings.HasPrefix(l, "ok"):
				c.Ejecutados++
				c.Pasaron = append(c.Pasaron, nombreTAP(l))
			}
		}
	case "junit":
		dec := xml.NewDecoder(bytes.NewReader(raw))
		var enCaso, marcado bool
		var caso string
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
					var clase, nombre string
					for _, a := range t.Attr {
						switch a.Name.Local {
						case "classname":
							clase = a.Value
						case "name":
							nombre = a.Value
						}
					}
					caso = nombreJUnit(clase, nombre)
				case "skipped":
					if enCaso && !marcado {
						c.Omitidos++
						marcado = true
					}
				case "failure", "error":
					if enCaso && !marcado {
						c.Ejecutados++
						c.Fallidos++
						c.Fallaron = append(c.Fallaron, caso)
						marcado = true
					}
				}
			case xml.EndElement:
				if t.Name.Local == "testcase" {
					if enCaso && !marcado {
						c.Ejecutados++
						c.Pasaron = append(c.Pasaron, caso)
					}
					enCaso = false
				}
			}
		}
	case "gotest-json":
		sc := bufio.NewScanner(bytes.NewReader(raw))
		for sc.Scan() {
			var ev struct{ Action, Package, Test string }
			if json.Unmarshal(sc.Bytes(), &ev) != nil || ev.Test == "" {
				continue
			}
			switch ev.Action {
			case "pass":
				c.Ejecutados++
				c.Pasaron = append(c.Pasaron, strings.TrimSpace(ev.Package+" "+ev.Test))
			case "fail":
				c.Ejecutados++
				c.Fallidos++
				c.Fallaron = append(c.Fallaron, strings.TrimSpace(ev.Package+" "+ev.Test))
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

// MaxPruebasExamen acota cuántos nombres guarda el contrato.
const MaxPruebasExamen = 200

// pruebasDelRojo son las pruebas que fallan en el rojo, sin repetir y en orden.
func pruebasDelRojo(c *Conteo) []string {
	if c == nil {
		return nil
	}
	var out []string
	for _, n := range c.Fallaron {
		if n != "" && !slices.Contains(out, n) {
			out = append(out, n)
		}
	}
	slices.Sort(out)
	if len(out) > MaxPruebasExamen {
		out = out[:MaxPruebasExamen]
	}
	return out
}

// faltanDelExamen: las pruebas del examen que el reporte no trae como pasadas.
func faltanDelExamen(examen []string, c *Conteo) []string {
	if c == nil {
		return nil
	}
	var out []string
	for _, n := range examen {
		if !slices.Contains(c.Pasaron, n) {
			out = append(out, n)
		}
	}
	return out
}

// notaFaltan explica qué pruebas del examen no se vieron pasar; "" si están todas.
func notaFaltan(examen []string, c *Conteo) string {
	f := faltanDelExamen(examen, c)
	if len(f) == 0 {
		return ""
	}
	return fmt.Sprintf("%d prueba(s) del examen no aparecen como pasadas en el reporte (¿un test enfocado, --grep o un alias de .only?): %s", len(f), strings.Join(cabeza(f, 5), "; "))
}
