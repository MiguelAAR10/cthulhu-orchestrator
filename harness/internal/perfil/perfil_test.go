package perfil

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
)

func repo(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for f, body := range files {
		os.MkdirAll(filepath.Dir(filepath.Join(root, f)), 0o755)
		os.WriteFile(filepath.Join(root, f), []byte(body), 0o644)
	}
	for _, a := range [][]string{{"init", "-q"}, {"add", "-A"}} {
		c := exec.Command("git", a...)
		c.Dir = root
		c.Run()
	}
	return root
}

func TestRepoVacioEsNuevoYSoloLlevaLaBase(t *testing.T) {
	p := Detect(repo(t, map[string]string{"README.md": "x"}))
	if p.Modo != "nuevo" || len(p.Senales) != 0 {
		t.Fatalf("%+v", p)
	}
	r := Recomendadas(p)
	if len(r) != 2 || !slices.Contains(r["obra/superpowers"], "test-driven-development") || !slices.Contains(r["mattpocock/skills"], "grill-with-docs") || !slices.Contains(r["mattpocock/skills"], "codebase-design") {
		t.Fatalf("%v", r)
	}
}

func TestReactConCodigoEsMejoraYActivaSkillsDeReactYWeb(t *testing.T) {
	files := map[string]string{"package.json": `{"dependencies":{"react":"19","next":"16"}}`}
	for _, f := range []string{"a", "b", "c", "d", "e"} {
		files["src/"+f+".tsx"] = "x"
	}
	p := Detect(repo(t, files))
	if p.Modo != "mejora" || !slices.Contains(p.Senales, "react") || !slices.Contains(p.Senales, "web") {
		t.Fatalf("%+v", p)
	}
	r := Recomendadas(p)
	if !slices.Contains(r["vercel-labs/agent-skills"], "vercel-react-best-practices") || !slices.Contains(r["microsoft/playwright-cli"], "playwright-cli") {
		t.Fatalf("%v", r)
	}
	if slices.Contains(r["mattpocock/skills"], "codebase-design") || !slices.Contains(r["mattpocock/skills"], "improve-codebase-architecture") {
		t.Fatalf("modo mal aplicado: %v", r)
	}
}

func TestComandoEsPorProyecto(t *testing.T) {
	c := ComandoInstalar("mattpocock/skills", []string{"tdd", "handoff"})
	if slices.Contains(c, "-g") || !slices.Contains(c, "handoff") {
		t.Fatalf("%v", c)
	}
}

func TestCatalogoSinProhibidasYConDependencias(t *testing.T) {
	ls := Lista(Perfil{Modo: "nuevo", Senales: []string{"web", "react"}})
	nombres := map[string]bool{}
	for _, s := range ls {
		if _, malo := Nunca[s.Nombre]; malo {
			t.Errorf("el catálogo recomienda %s", s.Nombre)
		}
		if nombres[s.Nombre] {
			t.Errorf("repetida %s", s.Nombre)
		}
		nombres[s.Nombre] = true
		if len(s.Roles) == 0 {
			t.Errorf("%s no dice qué rol la usa", s.Nombre)
		}
	}
	for _, s := range []string{"grilling", "domain-modeling", "test-driven-development", "playwright-cli"} {
		if !nombres[s] {
			t.Errorf("falta %s", s)
		}
	}
	if w := ParaRol(ls, "worker"); len(w) == 0 || slices.ContainsFunc(w, func(s Skill) bool { return s.Nombre == "grilling" }) {
		t.Fatalf("worker: %+v", w)
	}
}

func TestVerificarDetectaFaltaDepCambioYNunca(t *testing.T) {
	root := t.TempDir()
	poner := func(n, body string) {
		os.MkdirAll(filepath.Join(root, ".claude", "skills", n), 0o755)
		os.WriteFile(filepath.Join(root, ".claude", "skills", n, "SKILL.md"), []byte(body), 0o644)
	}
	poner("grill-with-docs", "a")
	poner("tdd", "x")
	h, _ := HuellaSkill(root, "grill-with-docs")
	ls := []Skill{{Nombre: "grill-with-docs", Requiere: []string{"grilling"}}, {Nombre: "to-spec"}}
	ps := Verificar(root, ls, map[string]Fijada{"grill-with-docs": {Huella: h}})
	tipos := ""
	for _, p := range ps {
		tipos += p.Tipo + " "
	}
	if tipos != "DEP FALTA NUNCA " {
		t.Fatalf("%q", tipos)
	}
	poner("grill-with-docs", "cambiada")
	ps = Verificar(root, ls[:1], map[string]Fijada{"grill-with-docs": {Huella: h}})
	if !slices.ContainsFunc(ps, func(p Problema) bool { return p.Tipo == "CAMBIO" }) {
		t.Fatalf("%+v", ps)
	}
}
