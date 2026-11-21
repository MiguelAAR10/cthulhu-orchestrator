package cli

import (
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"

	"orq/internal/identidad"
	"orq/internal/perfil"
)

// casos son los literales de los `case` de cada switch sobre args[0] o sub en la función nombre.
func casos(t *testing.T, nombre string) []string {
	t.Helper()
	fset := token.NewFileSet()
	_, aqui, _, _ := runtime.Caller(0) // TestMain cambia de directorio: los fuentes están junto a este archivo
	dir := filepath.Dir(aqui)
	ents, _ := os.ReadDir(dir)
	var out []string
	for _, e := range ents {
		if !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, e.Name()), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Name.Name != nombre {
				continue
			}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				sw, ok := n.(*ast.SwitchStmt)
				if !ok || sw.Tag == nil {
					return true
				}
				switch tag := sw.Tag.(type) {
				case *ast.IndexExpr:
				case *ast.Ident:
					if tag.Name != "sub" && tag.Name != "rol" {
						return true
					}
				default:
					return true
				}
				for _, s := range sw.Body.List {
					for _, x := range s.(*ast.CaseClause).List {
						if bl, ok := x.(*ast.BasicLit); ok {
							v, _ := strconv.Unquote(bl.Value)
							out = append(out, v)
						}
					}
				}
				return true
			})
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// The table is what the dispatcher really routes: same top-level names, same subcommands.
func TestTablaIgualAlDespacho(t *testing.T) {
	top := casos(t, "Run")
	if want := slices.Sorted(maps.Keys(comandos)); !slices.Equal(top, want) {
		t.Fatalf("dispatch %q\ntable    %q", top, want)
	}
	funcs := map[string]string{"historia": "runHistoria", "config": "", "skills": "runSkills", "msg": "runMsg", "mem": "runMem",
		"slice": "runSlice", "sesion": "runSesion", "plan": "runPlan", "aprender": "runAprender", "sandbox": "runSandbox", "skill": "runSkill"}
	for c, subs := range comandos {
		f, ok := funcs[c]
		if subs == nil || !ok || f == "" {
			continue
		}
		got := casos(t, f)
		if c == "skill" { // the roles go through perfil.Roles, not a case
			got = append(got, perfil.Roles...)
			slices.Sort(got)
		}
		if want := slices.Sorted(slices.Values(subs)); !slices.Equal(got, want) {
			t.Errorf("%s: dispatch %q table %q", c, got, want)
		}
	}
}

var (
	codigo  = regexp.MustCompile("`[^`\n]+`")
	llamada = regexp.MustCompile(`\borq2? ([a-z-]+)(?: ([a-z]+))?`)
)

// noExiste dice qué nombra texto que orq no despacha ("" si todo existe).
func noExiste(texto string) []string {
	var malos []string
	for _, r := range removidos {
		if strings.Contains(texto, r) {
			malos = append(malos, r)
		}
	}
	for _, c := range codigo.FindAllString(texto, -1) {
		for _, m := range llamada.FindAllStringSubmatch(c, -1) {
			subs, ok := comandos[m[1]]
			switch {
			case !ok:
				malos = append(malos, "orq "+m[1])
			case subs != nil && m[2] != "" && !slices.Contains(subs, m[2]):
				malos = append(malos, "orq "+m[1]+" "+m[2])
			}
		}
	}
	return malos
}

// orq2's guides and skills name only commands, subcommands and flags that exist.
func TestGuiasYSkillsSoloNombranLoQueExiste(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq2")
	for _, rol := range perfil.Roles {
		raw, _ := guias.ReadFile("guias/" + rol + ".md")
		if m := noExiste(identidad.Texto(orq2Guia(rol, string(raw)))); len(m) > 0 {
			t.Errorf("guias/%s.md names %q", rol, m)
		}
	}
	ents, _ := orqSkills.ReadDir("orqskills")
	for _, d := range ents {
		raw, _ := orqSkills.ReadFile("orqskills/" + d.Name() + "/SKILL.md")
		// Rendered as instalarSkillsOrq does, so skillOrq2's inserted text (P9) is scanned too.
		if m := noExiste(identidad.Texto(orq2Guia(d.Name(), skillOrq2(d.Name(), string(raw))))); len(m) > 0 {
			t.Errorf("orqskills/%s names %q", d.Name(), m)
		}
	}
}
