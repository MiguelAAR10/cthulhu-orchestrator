package slice

// Verificación honesta: un solo veredicto, runner vigilado, copia aislada con dependencias y rutas.

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// veredicto es la única regla que decide si un examen pasó. La usan Verificar (en la copia del
// worker) e integrar (en el merge): lo que valía en uno vale en el otro.
// salioBien: el comando salió con 0. c: lo que dice el reporte del runner (nil si no hay reporte).
// La nota explica un fallo que el código de salida solo no mostraba.
func veredicto(salioBien bool, c *Conteo, omitir bool) (bool, string) {
	if !salioBien || c == nil {
		return salioBien, ""
	}
	switch {
	case c.Ejecutados == 0:
		return false, "el comando salió bien pero corrió 0 pruebas"
	case c.Omitidos > 0 && !omitir:
		return false, fmt.Sprintf("%d pruebas omitidas y el slice no permite omitir", c.Omitidos)
	case c.Fallidos > 0:
		return false, fmt.Sprintf("salió 0 pero el reporte trae %d fallidas", c.Fallidos)
	}
	return true, ""
}

// Rutas del slice: fix es un solo worker sin equipo ni plan; slice es el flujo completo.
const (
	RutaFix   = "fix"
	RutaSlice = "slice"
)

// RutaEfectiva: los estados anteriores no traen ruta y son slices completos.
func (e *Estado) RutaEfectiva() string {
	if e.Ruta == RutaFix {
		return RutaFix
	}
	return RutaSlice
}

func (e *Estado) errRutaFix(que string) error {
	return fmt.Errorf("%s va por la ruta fix: un solo worker, sin %s. Si el trabajo creció, súbelo: `orq slice ruta slice \"motivo\"`", e.ID(), que)
}

// CambiarRuta sube un fix abierto a slice completo. Solo sube: bajar de slice a fix no existe.
func (r Repo) CambiarRuta(nn, ruta, motivo string) (*Estado, error) {
	soltar, err := r.bloqueo(nn)
	if err != nil {
		return nil, err
	}
	defer soltar()
	e, err := r.Load(nn)
	if err != nil || e == nil {
		return nil, errors.Join(err, errors.New("no hay slice abierto"))
	}
	switch {
	case ruta != RutaSlice:
		return e, fmt.Errorf("la ruta solo sube a %q; recibí %q", RutaSlice, ruta)
	case e.RutaEfectiva() == RutaSlice:
		return e, fmt.Errorf("%s ya va por la ruta slice", e.ID())
	case strings.TrimSpace(motivo) == "":
		return e, errors.New("di por qué el fix creció; subir de ruta sin motivo no vale")
	}
	e.Ruta, e.Ascendido, e.RutaMotivo = RutaSlice, true, strings.TrimSpace(motivo)
	return e, r.save(e)
}

// Vigilado es un archivo que decide qué significa el comando del examen (package.json, Makefile…),
// con el hash de su contenido en el commit del rojo.
type Vigilado struct {
	Ruta string `json:"ruta"`
	Blob string `json:"blob"`
}

var (
	separadorCmd = regexp.MustCompile(`&&|\|\||\||;|\n`)
	asignacionRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)
)

func conExtensiones(base string, exts ...string) []string {
	var out []string
	for _, x := range exts {
		out = append(out, base+"."+x)
	}
	return out
}

// archivosDelRunner deriva, de la línea de comando del examen, los archivos de la raíz que le dan
// sentido. Son candidatos: Abrir se queda solo con los que existen versionados.
func archivosDelRunner(cmd string) []string {
	var out []string
	add := func(l ...string) {
		for _, f := range l {
			if f != "" && !slices.Contains(out, f) {
				out = append(out, f)
			}
		}
	}
	jsExts := []string{"ts", "js", "mjs", "cjs", "mts", "cts"}
	for _, seg := range separadorCmd.Split(cmd, -1) {
		var toks []string
		for _, t := range strings.Fields(seg) {
			toks = append(toks, strings.Trim(t, `"'`))
		}
		for len(toks) > 0 && (asignacionRe.MatchString(toks[0]) || slices.Contains([]string{"time", "env", "exec"}, toks[0])) {
			toks = toks[1:]
		}
		if len(toks) == 0 {
			continue
		}
		primero := filepath.Base(toks[0])
		switch primero {
		case "npm", "pnpm", "yarn", "npx", "pnpx", "bun", "bunx":
			add("package.json")
		case "make", "gmake":
			add("Makefile", "makefile", "GNUmakefile")
			for i, t := range toks {
				if t == "-f" && i+1 < len(toks) {
					add(filepath.Clean(toks[i+1]))
				}
			}
		case "sh", "bash", "zsh", "dash":
			for _, t := range toks[1:] {
				if t == "-c" {
					break
				}
				if !strings.HasPrefix(t, "-") {
					add(filepath.Clean(t))
					break
				}
			}
		default:
			if strings.HasPrefix(toks[0], "./") {
				add(filepath.Clean(toks[0]))
			}
		}
		for _, t := range toks {
			switch filepath.Base(t) {
			case "pytest", "py.test":
				add("pyproject.toml", "pytest.ini", "setup.cfg", "tox.ini", "conftest.py")
			case "vitest":
				add(conExtensiones("vitest.config", jsExts...)...)
				add(conExtensiones("vite.config", jsExts...)...)
			case "jest":
				add(conExtensiones("jest.config", append(jsExts, "json")...)...)
			case "playwright":
				add(conExtensiones("playwright.config", jsExts...)...)
			}
		}
	}
	return out
}

// ErrRunnerCambio: un archivo que decide qué significa el comando cambió desde el rojo. No se corrió
// nada y no cuenta como reparación fallida.
type ErrRunnerCambio struct {
	ID       string
	Archivos []string
}

func (e *ErrRunnerCambio) Error() string {
	return fmt.Sprintf("%s: el runner cambió desde el rojo (%s). Esos archivos deciden qué significa el comando, así que no se verificó y no cuenta como reparación fallida. "+
		"Si el cambio es legítimo, apruébalo y queda en el recibo: `orq slice verificar --acepto-runner \"motivo\"`", e.ID, strings.Join(e.Archivos, ", "))
}

// VerificarOpc son las opciones de una verificación.
type VerificarOpc struct {
	AceptoRunner string // motivo por el que se acepta un cambio en los archivos vigilados
}

// blobEn devuelve el hash de ruta en el commit sha del repo principal, o "" si ahí no hay un archivo versionado.
func (r Repo) blobEn(sha, ruta string) string {
	out, err := r.git("ls-tree", sha, "--", ruta)
	if err != nil {
		return ""
	}
	campos := strings.Fields(out) // <modo> blob <hash>\t<ruta>
	if len(campos) < 3 || campos[1] != "blob" {
		return ""
	}
	return campos[2]
}

// vigiladosEn deriva los archivos del runner del comando, deja los versionados en sha y toma su hash.
// Excluye lo que ya forma parte del paquete del examen: eso ya está en la huella.
func (r Repo) vigiladosEn(sha, cmd string, bundle []string) []Vigilado {
	var out []Vigilado
	for _, f := range archivosDelRunner(cmd) {
		if slices.Contains(bundle, f) {
			continue
		}
		if b := r.blobEn(sha, f); b != "" {
			out = append(out, Vigilado{Ruta: f, Blob: b})
		}
	}
	return out
}

// runnerCambiado lista los archivos vigilados cuyo contenido en head ya no es el del rojo.
func (r Repo) runnerCambiado(e *Estado, head string) ([]string, error) {
	var cambiados []string
	for _, v := range e.Vigilados {
		out, err := r.git("ls-tree", head, "--", v.Ruta)
		if err != nil {
			return nil, err
		}
		campos := strings.Fields(out)
		switch {
		case len(campos) < 3 || campos[1] != "blob":
			cambiados = append(cambiados, v.Ruta+" (borrado)")
		case campos[2] != v.Blob:
			cambiados = append(cambiados, v.Ruta)
		}
	}
	return cambiados, nil
}
