package perfil

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"orq/internal/config"
)

// gofmt -l sale 0 aunque haya archivos sin formato: el check falla cuando lista alguno.
const lintGo = "! gofmt -l . | grep ."

// testPorDefecto es lo que `npm init` escribe en scripts.test: no es una prueba.
const testPorDefecto = "no test specified"

// Checks detecta en los archivos de root los comandos de lint, tipos y pruebas que el proyecto ya
// tiene. Nunca inventa un script: lo que no existe queda omitido con su motivo.
func Checks(root string) map[string]config.Check {
	if existe(root, "go.mod") {
		return map[string]config.Check{
			"lint":  {Cmd: lintGo},
			"types": {Cmd: "go vet ./..."},
			"tests": {Cmd: "go test ./..."},
		}
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
