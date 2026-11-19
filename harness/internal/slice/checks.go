package slice

import (
	"fmt"
	"strings"

	"orq/internal/config"
)

// Resultados de un check en el recibo. Omitido no es pasado: el proyecto no tiene ese comando.
const (
	CheckPaso    = "paso"
	CheckFallo   = "fallo"
	CheckOmitido = "omitido"
)

// LineasCheck acota la salida que el recibo guarda de cada check: las primeras, donde la
// herramienta dice qué está mal.
const LineasCheck = 10

// ResultadoCheck es lo que el recibo guarda de un check del proyecto.
type ResultadoCheck struct {
	Nombre    string `json:"nombre"`
	Cmd       string `json:"cmd,omitempty"`
	Exit      int    `json:"exit"`
	Resultado string `json:"resultado"`
	Motivo    string `json:"motivo,omitempty"`
	Salida    string `json:"salida,omitempty"`
}

// Correr corre un comando de shell y devuelve su salida y su código; err solo si no pudo arrancar.
type Correr func(cmd string) (out string, code int, err error)

// CorrerChecks corre cada check en orden con correr. Un error de arranque corta: es de la
// infraestructura, no un defecto del código.
func CorrerChecks(cs []config.Check, correr Correr) ([]ResultadoCheck, error) {
	var out []ResultadoCheck
	for _, c := range cs {
		if c.Interno() {
			continue // lo corre orq sobre el diff del slice, no un comando
		}
		if c.Cmd == "" {
			out = append(out, ResultadoCheck{Nombre: c.Nombre, Resultado: CheckOmitido, Motivo: c.Omitir})
			continue
		}
		salida, code, err := correr(c.Cmd)
		if err != nil {
			return out, fmt.Errorf("check %s no arrancó: %w", c.Nombre, err)
		}
		res := ResultadoCheck{Nombre: c.Nombre, Cmd: c.Cmd, Exit: code, Resultado: CheckPaso, Salida: primeras(salida, LineasCheck)}
		if code != 0 {
			res.Resultado = CheckFallo
		}
		out = append(out, res)
	}
	return out, nil
}

// FalloDeChecks dice qué checks fallaron, con las primeras líneas de la herramienta; "" si ninguno.
func FalloDeChecks(rs []ResultadoCheck) string {
	var xs []string
	for _, r := range rs {
		if r.Resultado == CheckFallo {
			x := fmt.Sprintf("check %s falla (exit %d): %s", r.Nombre, r.Exit, r.Salida)
			if r.Exit == 127 {
				x += " [infraestructura: la caja no ve el comando; suma su toolchain a sandbox.ro en limits.json]"
			}
			xs = append(xs, x)
		}
	}
	return strings.Join(xs, "; ")
}

func primeras(s string, n int) string {
	l := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(l) > n {
		l = l[:n]
	}
	return strings.Join(l, "\n")
}

// checksEn corre los checks del proyecto sobre una copia limpia de sha, en la caja, y las guardias
// de orq sobre el diff del rojo a sha.
func (r Repo) checksEn(e *Estado, sha string) ([]ResultadoCheck, error) {
	if len(r.Checks) == 0 {
		return nil, nil
	}
	dir, limpiar, err := r.copiaDe(sha)
	if err != nil {
		return nil, err
	}
	defer limpiar()
	rs, err := CorrerChecks(r.Checks, func(cmd string) (string, int, error) {
		out, code, _, err := r.correrEn(dir, cmd, e.limite())
		return out, code, err
	})
	if err != nil {
		return rs, err
	}
	for _, c := range r.Checks {
		if c.Interno() {
			res, err := r.supresiones(c, e.SHARojo, sha)
			if err != nil {
				return rs, fmt.Errorf("check %s: %w", c.Nombre, err)
			}
			rs = append(rs, res)
		}
	}
	return rs, nil
}
