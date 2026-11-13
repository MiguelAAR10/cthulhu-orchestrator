package internal_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Capas de orq. Un paquete solo importa paquetes de su capa o de una más baja:
//
//	cli → app → dominio → adaptadores
var capa = map[string]int{
	"cli": 3,
	"app": 2,
	// dominio: máquinas de estado y reglas
	"slice": 1, "plan": 1, "canal": 1, "memoria": 1, "instinto": 1, "perfil": 1, "gate": 1,
	"config": 1, "diario": 1, "recursos": 1, "watch": 1, "escriba": 1, "calidad": 1, "tui": 1,
	// adaptadores y base: todo lo que ejecuta un proceso o lee el sistema
	"terminal": 0, "terminal/herdr": 0, "terminal/orca": 0, "terminal/fake": 0, "agente": 0,
	"sandbox": 0, "gitx": 0, "sysinfo": 0, "orqdir": 0, "limits": 0, "state": 0,
}

func TestLosImportsSoloBajan(t *testing.T) {
	fset := token.NewFileSet()
	err := filepath.WalkDir(".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".go") {
			return err
		}
		pkg := filepath.ToSlash(filepath.Dir(p))
		if pkg == "." {
			return nil
		}
		nivel, ok := capa[pkg]
		if !ok {
			t.Errorf("%s: el paquete %q no tiene capa en arch_test.go", p, pkg)
			return nil
		}
		f, err := parser.ParseFile(fset, p, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		prueba := strings.HasSuffix(p, "_test.go")
		for _, im := range f.Imports {
			ruta, _ := strconv.Unquote(im.Path.Value)
			dep, interno := strings.CutPrefix(ruta, "orq/internal/")
			if !interno {
				continue
			}
			nd, ok := capa[dep]
			switch {
			case !ok:
				t.Errorf("%s importa %q, que no tiene capa", p, dep)
			case nd > nivel:
				t.Errorf("%s (capa %d) importa %q (capa %d): los imports solo bajan", p, nivel, dep, nd)
			case pkg == "cli" && !prueba && (dep == "terminal/herdr" || dep == "terminal/orca" || dep == "gitx"):
				t.Errorf("%s importa %q: la cli no habla con el multiplexor ni con git; eso es de app", p, dep)
			case dep == "terminal/fake" && !prueba:
				t.Errorf("%s importa el terminal de mentira fuera de una prueba", p)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
