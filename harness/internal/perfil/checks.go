package perfil

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"orq/internal/config"
)

// gofmt -l sale 0 aunque haya archivos sin formato: el check falla cuando lista alguno, con el código de gofmt
// cuando no puede leer un archivo o no corre (127: la caja no lo ve). Nunca pasa sin que gofmt haya corrido.
const lintGo = `out=$(gofmt -l . 2>&1); c=$?; printf '%s\n' "$out"; if [ $c -ne 0 ]; then exit $c; fi; test -z "$out"`

// testPorDefecto es lo que `npm init` escribe en scripts.test: no es una prueba.
const testPorDefecto = "no test specified"

// Checks detecta en los archivos de root los comandos de lint, tipos y pruebas que el proyecto ya
// tiene. Nunca inventa un script: lo que no existe queda omitido con su motivo.
func Checks(root string) map[string]config.Check { return ChecksPara(root, false) }

// ChecksPara es Checks con la rama de Python: python=true (solo orq2) propone los comandos de un
// proyecto uv (pyproject.toml y uv.lock); orq sigue omitiéndolos como en 1.0.0.
func ChecksPara(root string, python bool) map[string]config.Check {
	if existe(root, "go.mod") {
		return map[string]config.Check{
			"lint":  {Cmd: lintGo},
			"types": {Cmd: "go vet ./..."},
			"tests": {Cmd: "go test ./..."},
		}
	}
	if python && existe(root, "pyproject.toml") {
		return checksUv(root)
	}
	raw, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err != nil {
		sin := config.Check{Omitir: "stack sin comandos conocidos: escribe el comando en .orq/config.json"}
		return map[string]config.Check{"lint": sin, "types": sin, "tests": sin}
	}
	var pj struct {
		Scripts map[string]string `json:"scripts"`
		Deps    map[string]string `json:"dependencies"`
		DevDeps map[string]string `json:"devDependencies"`
	}
	json.Unmarshal(raw, &pj)
	out := map[string]config.Check{}
	switch {
	case pj.Scripts["lint"] != "":
		out["lint"] = config.Check{Cmd: "npm run lint"}
	default:
		out["lint"] = config.Check{Omitir: "package.json no tiene script lint"}
	}
	_, ts := pj.DevDeps["typescript"]
	if _, dep := pj.Deps["typescript"]; dep {
		ts = true
	}
	switch {
	case pj.Scripts["typecheck"] != "":
		out["types"] = config.Check{Cmd: "npm run typecheck"}
	case ts && existe(root, "tsconfig.json"):
		out["types"] = config.Check{Cmd: "npx --no-install tsc --noEmit"}
	default:
		out["types"] = config.Check{Omitir: "package.json no tiene script typecheck ni typescript con tsconfig.json"}
	}
	switch t := pj.Scripts["test"]; {
	case t != "" && !strings.Contains(t, testPorDefecto):
		out["tests"] = config.Check{Cmd: "npm test"}
	default:
		out["tests"] = config.Check{Omitir: "package.json no tiene script test"}
	}
	return out
}

func existe(root, f string) bool {
	_, err := os.Stat(filepath.Join(root, f))
	return err == nil
}

// checksUv propone los checks de un proyecto uv. La caja no tiene red y monta el .venv del checkout
// principal solo lectura: uv no sincroniza (--no-sync) y el código se importa de la copia (PYTHONPATH=src).
// pyproject.toml se lee como texto: una herramienta cuenta si aparece «[tool.<nombre>» o «"<nombre>».
func checksUv(root string) map[string]config.Check {
	if !existe(root, "uv.lock") {
		sin := config.Check{Omitir: "Python sin uv.lock: escribe el comando en .orq/config.json"}
		return map[string]config.Check{"lint": sin, "types": sin, "tests": sin}
	}
	raw, _ := os.ReadFile(filepath.Join(root, "pyproject.toml"))
	txt := string(raw)
	nombra := func(h string) bool { return strings.Contains(txt, "[tool."+h) || strings.Contains(txt, `"`+h) }
	pre, fuente := "uv run --offline --no-sync ", "."
	if info, err := os.Stat(filepath.Join(root, "src")); err == nil && info.IsDir() {
		pre, fuente = "PYTHONPATH=src "+pre, "src"
	}
	var dirs []string
	for _, d := range []string{"src", "tests"} {
		if info, err := os.Stat(filepath.Join(root, d)); err == nil && info.IsDir() {
			dirs = append(dirs, d)
		}
	}
	d := "."
	if len(dirs) > 0 {
		d = strings.Join(dirs, " ")
	}
	out := map[string]config.Check{}
	por := func(clave, h, cmd string) {
		if nombra(h) {
			out[clave] = config.Check{Cmd: cmd}
		} else {
			out[clave] = config.Check{Omitir: "pyproject.toml no nombra " + h}
		}
	}
	por("lint", "ruff", pre+"ruff check "+d+" && "+pre+"ruff format --check "+d)
	por("types", "mypy", pre+"mypy "+fuente)
	por("tests", "pytest", pre+"pytest -q -p no:cacheprovider")
	return out
}
