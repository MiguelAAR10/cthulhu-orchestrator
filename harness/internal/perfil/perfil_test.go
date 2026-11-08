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

func TestRepoVacioEsNuevoSinSenales(t *testing.T) {
	p := Detect(repo(t, map[string]string{"README.md": "x"}))
	if p.Modo != "nuevo" || len(p.Senales) != 0 {
		t.Fatalf("%+v", p)
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
	var n []string
	for _, s := range Lista(p) {
		n = append(n, s.Nombre)
	}
	if !slices.Contains(n, "vercel-react-best-practices") || !slices.Contains(n, "playwright-cli") || slices.Contains(n, "codebase-design") || !slices.Contains(n, "improve-codebase-architecture") {
		t.Fatalf("%v", n)
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

func TestCriticalityAccessor(t *testing.T) {
	dir := t.TempDir()
	if v, _ := Criticality(dir); v != "" {
		t.Fatalf("sin nada: %q", v)
	}
	os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"project":{"criticality":"high"}}`), 0o644)
	if v, note := Criticality(dir); v != "high" || note == "" {
		t.Fatalf("legado: %q %q", v, note)
	}
	os.WriteFile(filepath.Join(dir, "profile.json"), []byte(`{"criticality":"low"}`), 0o644)
	if v, note := Criticality(dir); v != "low" || note != "" {
		t.Fatalf("profile gana: %q %q", v, note)
	}
}
