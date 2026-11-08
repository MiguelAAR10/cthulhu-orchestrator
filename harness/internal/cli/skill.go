package cli

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"orq/internal/orqdir"
	"orq/internal/perfil"
)

//go:embed guias/*.md
var guias embed.FS

//go:embed orqskills
var orqSkills embed.FS

// skillRunner runs `npx ...` installs; tests replace it so nothing real is installed.
var skillRunner perfil.Runner = perfil.ExecRunner

// instalarSkillsOrq deja las skills propias de orq en el repo; no pisa una versión editada.
func instalarSkillsOrq(root string) (nuevas, distintas []string) {
	ents, _ := orqSkills.ReadDir("orqskills")
	for _, d := range ents {
		raw, _ := orqSkills.ReadFile("orqskills/" + d.Name() + "/SKILL.md")
		dst := filepath.Join(root, ".claude", "skills", d.Name(), "SKILL.md")
		actual, err := os.ReadFile(dst)
		switch {
		case err != nil:
			os.MkdirAll(filepath.Dir(dst), 0o755)
			if os.WriteFile(dst, raw, 0o644) == nil {
				nuevas = append(nuevas, d.Name())
			}
		case string(actual) != string(raw):
			distintas = append(distintas, d.Name())
		}
	}
	return
}

// ---------- profile ----------

func (p proyecto) forcedPath() string  { return filepath.Join(p.dir, "profile.json") }
func (p proyecto) galleryPath() string { return filepath.Join(p.dir, "skills.gallery.json") }

func (p proyecto) forced() perfil.Forced {
	f := perfil.Forced{}
	if raw, err := os.ReadFile(p.forcedPath()); err == nil {
		json.Unmarshal(raw, &f)
	}
	return f
}

// profile is the detected profile with the Owner's forced values on top.
func (p proyecto) profile() (perfil.Profile, []string) {
	return perfil.ApplyForced(perfil.DetectProfile(p.root), p.forced())
}

func (p proyecto) catalog() perfil.Catalog {
	c, err := perfil.LoadCatalog(p.dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "warning:", err, "(using the embedded catalog)")
	}
	return c
}

// orq profile [set KEY=VALUE...]
func runProfile(args []string, stdout, stderr io.Writer) int {
	p, err := abrirProyecto()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if len(args) > 0 && args[0] == "set" {
		nf, err := perfil.ParseForced(args[1:])
		if err != nil || len(nf) == 0 {
			fmt.Fprintln(stderr, "usage: orq profile set type=greenfield|brownfield|legacy criticality=trivial|low|medium|high stage=development|production stacks=a,b")
			if err != nil {
				fmt.Fprintln(stderr, err)
			}
			return 1
		}
		f := p.forced()
		for k, v := range nf {
			f[k] = v
		}
		raw, _ := json.MarshalIndent(f, "", "  ")
		if err := orqdir.WriteAtomic(p.forcedPath(), raw); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
	} else if len(args) > 0 {
		fmt.Fprintln(stderr, "usage: orq profile [set KEY=VALUE...]")
		return 1
	}
	pr, warn := p.profile()
	f := p.forced()
	tag := func(k string) string {
		if _, ok := f[k]; ok {
			return " (forced)"
		}
		return ""
	}
	s := pr.Signals
	fmt.Fprintf(stdout, "project_type: %s%s\nstacks: %s%s\ncriticality: %s%s\nstage: %s%s\n",
		pr.ProjectType, tag("type"), strings.Join(pr.Stacks, ","), tag("stacks"), pr.Criticality, tag("criticality"), pr.Stage, tag("stage"))
	fmt.Fprintf(stdout, "signals: %d code files · %d commits · %d days · %d authors · tests=%v · ci=%v · manifests=%s · stage hint=%s\n",
		s.CodeFiles, s.Commits, s.AgeDays, s.Authors, s.Tests, s.CI, strings.Join(s.Manifests, ","), s.StageHint)
	for _, w := range warn {
		fmt.Fprintln(stdout, "warning:", w)
	}
	return 0
}

// ---------- role guide ----------

// phaseCode: roles that live in one phase use it (scout C, planner/astra A, worker D, verifier V);
// Atena and the Orchestrator follow the project phase from `orq fase`.
func (p proyecto) phaseCode(rol string) string {
	if ph, ok := map[string]string{"scout": "C", "planificador": "A", "astra": "A", "worker": "D", "verificador": "V"}[rol]; ok {
		return ph
	}
	f, _ := p.fase()
	code, _, _ := strings.Cut(f, "·")
	switch code {
	case "D1":
		return "D"
	case "D2":
		return "DOC"
	}
	return code
}

// roleSkills renders the phase×role skill set, nothing else.
func (p proyecto) roleSkills(rol string) string {
	ph := p.phaseCode(rol)
	pr, _ := p.profile()
	ls := perfil.Plan(p.catalog(), pr, ph, rol)
	if len(ls) == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "\n## Skills for %s in phase %s (%s · %s · criticality %s)\n", rol, ph, pr.ProjectType, strings.Join(pr.Stacks, ","), pr.Criticality)
	for _, s := range ls {
		mark := "✓"
		if !perfil.Instalada(p.root, s.Name) {
			mark = "· missing (orq skills install " + s.Name + ")"
		}
		fmt.Fprintf(&b, "- %s %s", s.Name, mark)
		if s.Adapt != "" {
			fmt.Fprintf(&b, " — harness rule: %s", s.Adapt)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// runSkill imprime la guía del rol y solo las skills de su fase×rol.
func runSkill(args []string, stdout, stderr io.Writer) int {
	rol := os.Getenv("ORQ_ROL")
	if len(args) > 0 {
		rol = args[0]
	}
	if !slices.Contains(perfil.Roles, rol) {
		fmt.Fprintf(stderr, "uso: orq skill %s\n", strings.Join(perfil.Roles, "|"))
		return 1
	}
	g, _ := guias.ReadFile("guias/" + rol + ".md")
	fmt.Fprint(stdout, string(g))
	if p, err := abrirProyecto(); err == nil {
		fmt.Fprint(stdout, p.roleSkills(rol))
	}
	fmt.Fprintf(stdout, "\nNever load: %s.\n", strings.Join(nunca(), ", "))
	return 0
}

func nunca() []string {
	var n []string
	for k := range perfil.Nunca {
		n = append(n, k)
	}
	slices.Sort(n)
	return n
}

func (p proyecto) lockPath() string { return filepath.Join(p.dir, "skills.lock.json") }

func (p proyecto) fijadas() map[string]perfil.Fijada {
	m := map[string]perfil.Fijada{}
	raw, err := os.ReadFile(p.lockPath())
	if err == nil {
		json.Unmarshal(raw, &m)
	}
	return m
}

// ---------- skills ----------

const skillsUsage = `usage:
  orq skills                                  every skill this project may load, by role
  orq skills plan --phase T|C|A|D|V|DOC --role R   minimal set for that phase×role
  orq skills install [ID...] [--phase X --role R]  install only the selection (never-list refused), pin hashes
  orq skills vet ID [--file SKILL.md]         static check; verdict goes to .orq/skills.gallery.json
  orq skills fijar | verificar                pin installed hashes | report FALTA/DEP/CAMBIO/NUNCA
`

func runSkills(args []string, stdout, stderr io.Writer) int {
	p, err := abrirProyecto()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	cat := p.catalog()
	pr, warn := p.profile()
	for _, w := range warn {
		fmt.Fprintln(stderr, "warning:", w)
	}
	sub := ""
	if len(args) > 0 {
		sub, args = args[0], args[1:]
	}
	fs := flag.NewFlagSet("skills", flag.ContinueOnError)
	fs.SetOutput(stderr)
	phase := fs.String("phase", "", "T|C|A|D|V|DOC")
	role := fs.String("role", "", strings.Join(perfil.Roles, "|"))
	file := fs.String("file", "", "SKILL.md to vet")
	var pos []string
	for len(args) > 0 {
		if err := fs.Parse(args); err != nil {
			return 1
		}
		args = fs.Args()
		if len(args) > 0 {
			pos, args = append(pos, args[0]), args[1:]
		}
	}
	switch sub {
	case "plan":
		if !slices.Contains(perfil.Phases, *phase) || !slices.Contains(perfil.Roles, *role) {
			fmt.Fprint(stderr, skillsUsage)
			return 1
		}
		ls := perfil.Plan(cat, pr, *phase, *role)
		fmt.Fprintf(stdout, "%s × %s · %s · %s · criticality %s · stage %s: %d skills\n", *phase, *role, pr.ProjectType, strings.Join(pr.Stacks, ","), pr.Criticality, pr.Stage, len(ls))
		for _, e := range ls {
			fmt.Fprintf(stdout, "  %-32s %-5s %s\n", e.Name, e.TokenTier, e.Summary)
		}
		return 0
	case "install", "instalar":
		return p.skillsInstall(cat, pr, pos, *phase, *role, stdout, stderr)
	case "vet":
		if len(pos) != 1 {
			fmt.Fprint(stderr, skillsUsage)
			return 1
		}
		return p.skillsVet(cat, pos[0], *file, stdout, stderr)
	case "fijar", "pin":
		m := map[string]perfil.Fijada{}
		for _, e := range perfil.PlanAll(cat, pr) {
			if h, err := perfil.HuellaSkill(p.root, e.Name); err == nil && perfil.Instalada(p.root, e.Name) {
				m[e.Name] = perfil.Fijada{ID: e.ID, Ref: e.Ref, Repo: e.Source, Huella: h, Roles: e.Roles, Adapta: e.Adapt}
			}
		}
		raw, _ := json.MarshalIndent(m, "", "  ")
		if err := orqdir.WriteAtomic(p.lockPath(), raw); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintf(stdout, "%d skills pinned in %s (a change will be reported)\n", len(m), p.lockPath())
		return 0
	case "verificar", "verify":
		ps := perfil.Verificar(p.root, perfil.AsSkills(perfil.PlanAll(cat, pr)), p.fijadas())
		for _, x := range ps {
			fmt.Fprintf(stdout, "%-6s %-32s %s\n", x.Tipo, x.Skill, x.Detalle)
		}
		if len(ps) == 0 {
			fmt.Fprintln(stdout, "skills in order: the project's, with their dependencies and unchanged")
			return 0
		}
		return 1
	case "", "list":
		ls := perfil.PlanAll(cat, pr)
		faltan := 0
		for _, e := range ls {
			mark := "✓"
			if !perfil.Instalada(p.root, e.Name) {
				mark = "·"
				faltan++
			}
			fmt.Fprintf(stdout, "  %s %-32s %-5s %-26s %s\n", mark, e.Name, strings.Join(e.Phases, ","), strings.Join(e.Roles, ","), e.Source)
		}
		fmt.Fprintf(stdout, "%d of %d installed. Roles load only their phase×role set: `orq skills plan --phase X --role R`\n", len(ls)-faltan, len(ls))
		return 0
	}
	fmt.Fprint(stderr, skillsUsage)
	return 1
}

func (p proyecto) skillsInstall(cat perfil.Catalog, pr perfil.Profile, ids []string, phase, role string, stdout, stderr io.Writer) int {
	var sel []perfil.Entry
	switch {
	case len(ids) > 0:
		for _, id := range ids {
			e, ok := cat.Get(id)
			if !ok {
				fmt.Fprintf(stderr, "%s is not in the catalog\n", id)
				return 1
			}
			sel = append(sel, e)
			for _, r := range e.Requires {
				if d, ok := cat.Get(r); ok {
					sel = append(sel, d)
				}
			}
		}
	case phase != "" && role != "":
		sel = perfil.Plan(cat, pr, phase, role)
	default:
		sel = perfil.PlanAll(cat, pr)
	}
	g := perfil.LoadGallery(p.galleryPath())
	for _, e := range sel {
		if r, ok := g[e.ID]; ok && r.Verdict == "rejected" {
			fmt.Fprintf(stderr, "%s was rejected in the gallery: %s\n", e.Name, r.Reason)
			return 1
		}
	}
	lock, err := perfil.Install(p.root, cat, sel, orqSkills, skillRunner, p.fijadas())
	if lock != nil {
		raw, _ := json.MarshalIndent(lock, "", "  ")
		if werr := orqdir.WriteAtomic(p.lockPath(), raw); werr != nil {
			fmt.Fprintln(stderr, werr)
			return 1
		}
	}
	if err != nil {
		fmt.Fprintln(stderr, "orq skills install:", err)
		return 1
	}
	for _, e := range sel {
		g.Record(e.ID, true, nil, lock[e.Name].Huella)
	}
	g.Save(p.galleryPath())
	var n []string
	for _, e := range sel {
		n = append(n, e.Name)
	}
	sort.Strings(n)
	fmt.Fprintf(stdout, "installed and pinned in .claude/skills and .agents/skills: %s\n", strings.Join(n, ", "))
	return 0
}

func (p proyecto) skillsVet(cat perfil.Catalog, id, file string, stdout, stderr io.Writer) int {
	key := id
	e, inCat := cat.Get(id)
	if inCat {
		key = e.ID
	}
	g := perfil.LoadGallery(p.galleryPath())
	var ok bool
	var why []string
	sum := ""
	if r, bad := cat.NeverReason(id); bad {
		ok, why = false, []string{"never-list: " + r}
	} else {
		if file == "" {
			name := id
			if inCat {
				name = e.Name
			}
			file = filepath.Join(p.root, ".claude", "skills", name, "SKILL.md")
		}
		raw, err := os.ReadFile(file)
		if err != nil {
			fmt.Fprintf(stderr, "cannot read %s: %v (install it or pass --file)\n", file, err)
			return 1
		}
		ok, why = perfil.Vet(raw)
		if inCat && perfil.Instalada(p.root, e.Name) {
			sum, _ = perfil.HuellaSkill(p.root, e.Name)
		}
	}
	r := g.Record(key, ok, why, sum)
	if err := g.Save(p.galleryPath()); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "%s: %s", key, r.Verdict)
	if r.Reason != "" {
		fmt.Fprintf(stdout, " — %s", r.Reason)
	}
	fmt.Fprintln(stdout)
	if !ok {
		return 1
	}
	return 0
}
