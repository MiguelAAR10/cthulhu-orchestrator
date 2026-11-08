package perfil

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
)

func gitRepo(t *testing.T, files map[string]string, commits int, date string, authors ...string) string {
	t.Helper()
	root := repo(t, files)
	if len(authors) == 0 {
		authors = []string{"a"}
	}
	for i := 0; i < commits; i++ {
		os.WriteFile(filepath.Join(root, "log.txt"), []byte(strings.Repeat("x", i+1)), 0o644)
		a := authors[i%len(authors)]
		c := exec.Command("git", "-c", "user.name="+a, "-c", "user.email="+a+"@x", "commit", "-qam", "c", "--allow-empty")
		if i == 0 {
			exec.Command("git", "-C", root, "add", "-A").Run()
		}
		c.Dir = root
		c.Env = append(os.Environ(), "GIT_AUTHOR_DATE="+date, "GIT_COMMITTER_DATE="+date)
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("%v %s", err, out)
		}
	}
	return root
}

func names(es []Entry) []string {
	var n []string
	for _, e := range es {
		n = append(n, e.Name)
	}
	return n
}

// ---------- detection ----------

func TestDetectGreenfieldTypescript(t *testing.T) {
	root := gitRepo(t, map[string]string{"package.json": `{"devDependencies":{"typescript":"5"}}`, "tsconfig.json": "{}", "src/a.ts": "x"}, 2, "2026-10-01T00:00:00")
	p := DetectProfile(root)
	if p.ProjectType != "greenfield" || !slices.Contains(p.Stacks, "typescript") || slices.Contains(p.Stacks, "python") {
		t.Fatalf("%+v", p)
	}
	if p.Signals.Commits != 2 || p.Signals.Tests || p.Signals.CI {
		t.Fatalf("signals %+v", p.Signals)
	}
}

func TestDetectBrownfieldPythonWithTestsCIAuthors(t *testing.T) {
	files := map[string]string{"pyproject.toml": "[project]", ".github/workflows/ci.yml": "x", "tests/test_a.py": "x", "Dockerfile": "FROM x"}
	for _, f := range []string{"a", "b", "c", "d", "e", "f"} {
		files["app/"+f+".py"] = "x"
	}
	root := gitRepo(t, files, 30, "2025-01-01T00:00:00", "ana", "bob", "cy")
	p := DetectProfile(root)
	s := p.Signals
	if p.ProjectType != "brownfield" || !slices.Contains(p.Stacks, "python") || !s.Tests || !s.CI || s.Authors != 3 || s.Commits != 30 || s.CodeFiles < 6 {
		t.Fatalf("%+v", p)
	}
	if s.StageHint != "production" || !slices.Contains(s.Manifests, "pyproject.toml") {
		t.Fatalf("stage hint %+v", s)
	}
}

func TestDetectLegacyGoOldWithoutTests(t *testing.T) {
	files := map[string]string{"go.mod": "module x"}
	for _, f := range []string{"a", "b", "c", "d", "e"} {
		files[f+".go"] = "package x"
	}
	root := gitRepo(t, files, 25, "2016-01-01T00:00:00")
	p := DetectProfile(root)
	if p.ProjectType != "legacy" || !slices.Contains(p.Stacks, "go") || p.Signals.AgeDays < 3*365 {
		t.Fatalf("%+v", p)
	}
}

func TestForcedOverrideWinsAndWarnsOnContradiction(t *testing.T) {
	det := Profile{ProjectType: "brownfield", Stacks: []string{"python"}, Signals: Signals{CodeFiles: 40, Commits: 200, StageHint: "production"}}
	f, err := ParseForced([]string{"type=greenfield", "criticality=high", "stage=development"})
	if err != nil {
		t.Fatal(err)
	}
	p, warn := ApplyForced(det, f)
	if p.ProjectType != "greenfield" || p.Criticality != "high" || p.Stage != "development" {
		t.Fatalf("%+v", p)
	}
	if len(warn) != 2 || !strings.Contains(warn[0]+warn[1], "brownfield") || !strings.Contains(warn[0]+warn[1], "production") {
		t.Fatalf("warnings %v", warn)
	}
	if _, err := ParseForced([]string{"type=spaceship"}); err == nil {
		t.Fatal("invalid value accepted")
	}
	if _, w := ApplyForced(det, Forced{"type": "brownfield"}); len(w) != 0 {
		t.Fatalf("agreeing force warned: %v", w)
	}
}

// ---------- catalog ----------

func TestCatalogSchemaAndNeverList(t *testing.T) {
	c := DefaultCatalog()
	for _, n := range []string{"swarm", "arena", "dispatching-parallel-agents", "poteto-mode", "ship", "land-and-deploy", "canary", "worktree-cleanup", "using-superpowers"} {
		if r, ok := c.NeverReason(n); !ok || r == "" {
			t.Errorf("%s must be on the never-list", n)
		}
	}
	for _, n := range []string{"orq-board", "orq-supervisor", "orq-stopper", "typescript-best-practices", "python-best-practices", "test-driven-development", "playwright-cli"} {
		if _, ok := c.Get(n); !ok {
			t.Errorf("missing %s", n)
		}
	}
	for _, e := range c.Skills {
		if e.Never != "" {
			continue
		}
		if e.ID == "" || e.Name == "" || e.Source == "" || e.Install == "" || len(e.Phases) == 0 || len(e.Roles) == 0 || len(e.ProjectTypes) == 0 || len(e.Stacks) == 0 || rank(e.MinCriticality) < 1 || e.TokenTier == "" {
			t.Errorf("incomplete entry %+v", e)
		}
		if strings.HasPrefix(e.Source, "github.com/mattpocock") && e.Name != "code-review" && slices.ContainsFunc(e.Phases, func(p string) bool { return p != "T" && p != "C" && p != "A" }) {
			t.Errorf("Pocock skill %s outside the front: %v", e.Name, e.Phases)
		}
	}
	g, _ := c.Get("grilling")
	if !slices.Equal(g.Phases, []string{"T"}) || g.MinCriticality != "medium" {
		t.Fatalf("grilling %+v", g)
	}
}

func TestProjectOverrideCatalog(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "skills.catalog.json"), []byte(`{"version":1,"skills":[
	 {"id":"x.house-style","name":"house-style","source":"local","install":"copy:house-style","phases":["D"],"roles":["worker"],"project_types":["*"],"stacks":["*"],"min_criticality":"low","token_tier":"low"},
	 {"id":"superpowers.test-driven-development","name":"test-driven-development","never":"we use our own TDD"}]}`), 0o644)
	c, err := LoadCatalog(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := c.Get("house-style"); !ok {
		t.Fatal("override entry missing")
	}
	if r, ok := c.NeverReason("test-driven-development"); !ok || r != "we use our own TDD" {
		t.Fatal("override must replace the default entry by id")
	}
}

// ---------- routing matrix ----------

func TestRoutingMatrix(t *testing.T) {
	c := DefaultCatalog()
	ts := Profile{ProjectType: "greenfield", Stacks: []string{"typescript"}, Criticality: "medium", Stage: "development"}
	py := Profile{ProjectType: "greenfield", Stacks: []string{"python"}, Criticality: "medium", Stage: "development"}
	with := func(p Profile, f func(*Profile)) Profile { f(&p); return p }

	// 1. trivial ⇒ none or one skill
	triv := Plan(c, with(ts, func(p *Profile) { p.Criticality = "trivial" }), "D", "worker")
	if len(triv) > 1 {
		t.Errorf("trivial worker D: %v", names(triv))
	}
	// 2. production-critical ⇒ full ceremony, strictly more than trivial and medium
	crit := Plan(c, with(ts, func(p *Profile) { p.Criticality = "high"; p.Stage = "production" }), "D", "worker")
	med := Plan(c, ts, "D", "worker")
	if len(crit) <= len(med) || len(med) <= len(triv) {
		t.Errorf("ceremony not by stakes: trivial %v, medium %v, critical %v", names(triv), names(med), names(crit))
	}
	// 3/4. TS vs Python
	if n := names(med); !slices.Contains(n, "typescript-best-practices") || slices.Contains(n, "python-best-practices") {
		t.Errorf("ts worker: %v", n)
	}
	if n := names(Plan(c, py, "D", "worker")); !slices.Contains(n, "python-best-practices") || slices.Contains(n, "typescript-best-practices") {
		t.Errorf("py worker: %v", n)
	}
	// 5. grilling only in T, from medium criticality
	if n := names(Plan(c, ts, "T", "atena")); !slices.Contains(n, "grilling") {
		t.Errorf("atena T medium: %v", n)
	}
	if n := names(Plan(c, with(ts, func(p *Profile) { p.Criticality = "low" }), "T", "atena")); slices.Contains(n, "grilling") {
		t.Errorf("grilling on low stakes: %v", n)
	}
	for _, ph := range Phases {
		for _, r := range Roles {
			if ph != "T" && slices.Contains(names(Plan(c, crit0(ts), ph, r)), "grilling") {
				t.Errorf("grilling in %s/%s", ph, r)
			}
		}
	}
	// 6. brownfield vs greenfield scouts
	bf := with(ts, func(p *Profile) { p.ProjectType = "brownfield" })
	if n := names(Plan(c, bf, "C", "scout")); !slices.Contains(n, "improve-codebase-architecture") {
		t.Errorf("brownfield scout: %v", n)
	}
	if n := names(Plan(c, ts, "C", "scout")); slices.Contains(n, "improve-codebase-architecture") {
		t.Errorf("greenfield scout: %v", n)
	}
	// 7. production raises criticality: grill-with-docs (min high) appears with its requirements
	n := names(Plan(c, with(ts, func(p *Profile) { p.Stage = "production" }), "T", "atena"))
	if !slices.Contains(n, "grill-with-docs") || !slices.Contains(n, "domain-modeling") {
		t.Errorf("production atena T: %v", n)
	}
	// 8. never-list is never routed, no duplicates
	for _, ph := range Phases {
		for _, r := range Roles {
			seen := map[string]bool{}
			for _, e := range Plan(c, crit0(ts), ph, r) {
				if e.Never != "" || seen[e.Name] {
					t.Errorf("%s/%s: %s", ph, r, e.Name)
				}
				seen[e.Name] = true
			}
		}
	}
}

func crit0(p Profile) Profile { p.Criticality = "high"; p.Stage = "production"; return p }

// ---------- vetting ----------

const okSkill = "---\nname: house-style\ndescription: Use when writing code here.\n---\n# House style\nNever use `git push` yourself.\n"

func TestVetAcceptsCleanSkill(t *testing.T) {
	if ok, why := Vet([]byte(okSkill)); !ok {
		t.Fatalf("%v", why)
	}
}

func TestVetRejectsForcePushAndOthers(t *testing.T) {
	bad := "---\nname: shipit\ndescription: ship\n---\nRun `git push --force origin main` when done.\n"
	ok, why := Vet([]byte(bad))
	if ok || !strings.Contains(strings.Join(why, ";"), "git push") {
		t.Fatalf("%v %v", ok, why)
	}
	for body, want := range map[string]string{
		"# no frontmatter\n":                                                         "frontmatter",
		"---\nname: x\ndescription: y\n---\nrm -rf /\n":                              "rm -rf",
		"---\nname: x\ndescription: y\n---\nSpawn subagents for each task.\n":        "agents",
		"---\nname: x\ndescription: y\n---\n" + strings.Repeat("a", MaxSkillBytes+1): "size",
	} {
		if ok, why := Vet([]byte(body)); ok || !strings.Contains(strings.Join(why, ";"), want) {
			t.Errorf("%q: %v %v", want, ok, why)
		}
	}
}

// ---------- install ----------

func TestInstallRefusesNeverList(t *testing.T) {
	root := t.TempDir()
	c := DefaultCatalog()
	e, _ := c.Get("swarm")
	called := false
	_, err := Install(root, c, []Entry{e}, nil, func(string, []string) ([]byte, error) { called = true; return nil, nil }, nil)
	if err == nil || called || !strings.Contains(err.Error(), "never") {
		t.Fatalf("err=%v called=%v", err, called)
	}
}

func TestInstallCopiesLocalAndRunsNpxThenPins(t *testing.T) {
	root := t.TempDir()
	c := DefaultCatalog()
	board, _ := c.Get("orq-board")
	tdd, _ := c.Get("test-driven-development")
	local := fstest.MapFS{"orqskills/orq-board/SKILL.md": {Data: []byte(okSkill)}}
	var ran [][]string
	run := func(dir string, argv []string) ([]byte, error) {
		ran = append(ran, argv)
		os.MkdirAll(filepath.Join(dir, ".claude", "skills", "test-driven-development"), 0o755)
		return nil, os.WriteFile(filepath.Join(dir, ".claude", "skills", "test-driven-development", "SKILL.md"), []byte(okSkill), 0o644)
	}
	lock, err := Install(root, c, []Entry{board, tdd}, local, run, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(ran) != 1 || ran[0][0] != "npx" || !slices.Contains(ran[0], "test-driven-development") {
		t.Fatalf("ran %v", ran)
	}
	for _, n := range []string{"orq-board", "test-driven-development"} {
		for _, d := range []string{".claude", ".agents"} {
			if _, err := os.Stat(filepath.Join(root, d, "skills", n, "SKILL.md")); err != nil {
				t.Errorf("%s/%s: %v", d, n, err)
			}
		}
		if lock[n].Huella == "" || lock[n].ID == "" {
			t.Errorf("not pinned: %+v", lock[n])
		}
	}
	// changed content vs lock ⇒ refused
	os.WriteFile(filepath.Join(root, ".claude", "skills", "orq-board", "SKILL.md"), []byte(okSkill+"x"), 0o644)
	if _, err := Install(root, c, []Entry{board}, local, run, lock); err == nil || !strings.Contains(err.Error(), "pinned") {
		t.Fatalf("changed skill accepted: %v", err)
	}
}

func TestInstallRejectsSkillThatFailsVet(t *testing.T) {
	root := t.TempDir()
	c := DefaultCatalog()
	tdd, _ := c.Get("test-driven-development")
	run := func(dir string, argv []string) ([]byte, error) {
		d := filepath.Join(dir, ".claude", "skills", "test-driven-development")
		os.MkdirAll(d, 0o755)
		return nil, os.WriteFile(filepath.Join(d, "SKILL.md"), []byte("---\nname: t\ndescription: t\n---\ngit push --force\n"), 0o644)
	}
	if _, err := Install(root, c, []Entry{tdd}, nil, run, nil); err == nil {
		t.Fatal("unsafe skill installed")
	}
	if Instalada(root, "test-driven-development") {
		t.Fatal("rejected skill left in the project")
	}
}
