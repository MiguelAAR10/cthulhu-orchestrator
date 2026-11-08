package perfil

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Profile is what routing needs: project type, stacks, stakes.
type Profile struct {
	ProjectType string   `json:"project_type"` // greenfield | brownfield | legacy
	Stacks      []string `json:"stacks"`
	Criticality string   `json:"criticality"` // trivial | low | medium | high
	Stage       string   `json:"stage"`       // development | production
	Signals     Signals  `json:"signals"`
}

// Signals are the raw evidence behind the detection; they never run project code.
type Signals struct {
	CodeFiles int      `json:"code_files"`
	Commits   int      `json:"commits"`
	AgeDays   int      `json:"age_days"`
	Authors   int      `json:"authors"`
	Tests     bool     `json:"tests"`
	CI        bool     `json:"ci"`
	Manifests []string `json:"manifests"`
	StageHint string   `json:"stage_hint"`
}

var manifests = []string{"package.json", "tsconfig.json", "pyproject.toml", "requirements.txt", "setup.py", "go.mod", "Cargo.toml", "Dockerfile", "fly.toml", "vercel.json", "netlify.toml", "Procfile", "app.yaml"}

func git(root string, args ...string) string {
	out, _ := exec.Command("git", append([]string{"-C", root}, args...)...).Output()
	return strings.TrimSpace(string(out))
}

// DetectProfile reads versioned files and git history only.
func DetectProfile(root string) Profile {
	old := Detect(root)
	s := Signals{CodeFiles: old.Fuentes}
	set := map[string]bool{}
	for _, x := range old.Senales {
		set[x] = true
	}
	for _, m := range manifests {
		if _, err := os.Stat(filepath.Join(root, m)); err == nil {
			s.Manifests = append(s.Manifests, m)
		}
	}
	has := func(m string) bool { return slices.Contains(s.Manifests, m) }
	switch {
	case has("package.json") && has("tsconfig.json"):
		set["typescript"] = true
	case has("package.json"):
		set["javascript"] = true
	}
	if has("pyproject.toml") || has("requirements.txt") || has("setup.py") {
		set["python"] = true
	}
	if has("go.mod") {
		set["go"] = true
	}
	for _, f := range strings.Split(git(root, "ls-files"), "\n") {
		b := filepath.Base(f)
		switch {
		case strings.HasSuffix(f, ".ts") || strings.HasSuffix(f, ".tsx"):
			set["typescript"] = true
		}
		if strings.HasPrefix(f, ".github/workflows/") || b == ".gitlab-ci.yml" || strings.HasPrefix(f, ".circleci/") || b == "Jenkinsfile" {
			s.CI = true
		}
		if strings.HasSuffix(f, "_test.go") || strings.HasPrefix(b, "test_") || strings.HasSuffix(b, "_test.py") ||
			strings.Contains(b, ".test.") || strings.Contains(b, ".spec.") || strings.HasPrefix(f, "tests/") || strings.Contains(f, "/tests/") || strings.Contains(f, "__tests__/") {
			s.Tests = true
		}
	}
	if set["typescript"] {
		delete(set, "javascript")
	}
	s.Commits, _ = strconv.Atoi(git(root, "rev-list", "--count", "HEAD"))
	if ts := strings.Split(git(root, "log", "--reverse", "--format=%ct"), "\n"); ts[0] != "" {
		if t0, err := strconv.ParseInt(ts[0], 10, 64); err == nil {
			s.AgeDays = int(time.Since(time.Unix(t0, 0)).Hours() / 24)
		}
	}
	authors := map[string]bool{}
	for _, a := range strings.Split(git(root, "log", "--format=%ae"), "\n") {
		if a != "" {
			authors[a] = true
		}
	}
	s.Authors = len(authors)
	s.StageHint = "development"
	deploy := has("Dockerfile") || has("fly.toml") || has("vercel.json") || has("netlify.toml") || has("Procfile") || has("app.yaml")
	if s.CI && (deploy || git(root, "tag") != "") {
		s.StageHint = "production"
	}
	p := Profile{Signals: s, Stage: s.StageHint, Criticality: "medium"}
	switch {
	case s.CodeFiles < 5 && s.Commits < 20:
		p.ProjectType = "greenfield"
	case s.AgeDays >= 3*365 && (!s.Tests || !s.CI):
		p.ProjectType = "legacy"
	default:
		p.ProjectType = "brownfield"
	}
	for x := range set {
		p.Stacks = append(p.Stacks, x)
	}
	sort.Strings(p.Stacks)
	return p
}

// Forced holds the Owner's overrides: type, criticality, stage, stacks (comma list).
type Forced map[string]string

var forcedValues = map[string][]string{
	"type":        {"greenfield", "brownfield", "legacy"},
	"criticality": {"trivial", "low", "medium", "high"},
	"stage":       {"development", "production"},
	"stacks":      nil,
}

func ParseForced(args []string) (Forced, error) {
	f := Forced{}
	for _, a := range args {
		k, v, ok := strings.Cut(a, "=")
		allowed, known := forcedValues[k]
		if !ok || !known || v == "" {
			return nil, fmt.Errorf("expected KEY=VALUE with KEY in type|criticality|stage|stacks, got %q", a)
		}
		if allowed != nil && !slices.Contains(allowed, v) {
			return nil, fmt.Errorf("%s must be one of %s", k, strings.Join(allowed, "|"))
		}
		f[k] = v
	}
	return f, nil
}

// ApplyForced: the Owner always wins; warnings say where the evidence disagrees.
func ApplyForced(det Profile, f Forced) (Profile, []string) {
	p := det
	var warn []string
	if v, ok := f["type"]; ok {
		if v != det.ProjectType && det.ProjectType != "" {
			warn = append(warn, fmt.Sprintf("type forced to %s but detection says %s (%d code files, %d commits, %d days)", v, det.ProjectType, det.Signals.CodeFiles, det.Signals.Commits, det.Signals.AgeDays))
		}
		p.ProjectType = v
	}
	if v, ok := f["stage"]; ok {
		if v != det.Signals.StageHint && det.Signals.StageHint != "" {
			warn = append(warn, fmt.Sprintf("stage forced to %s but signals suggest %s (CI, deploy manifests or tags)", v, det.Signals.StageHint))
		}
		p.Stage = v
	}
	if v, ok := f["criticality"]; ok {
		p.Criticality = v
	}
	if v, ok := f["stacks"]; ok {
		p.Stacks = strings.Split(v, ",")
		for _, d := range det.Stacks {
			if !slices.Contains(p.Stacks, d) {
				warn = append(warn, fmt.Sprintf("stacks forced to %s but %s was detected", v, d))
			}
		}
	}
	return p, warn
}

// rank: low=1 medium=2 high=3 (catalog minimums); trivial=0, critical=4 for profiles.
func rank(c string) int {
	return map[string]int{"trivial": 0, "low": 1, "medium": 2, "high": 3, "critical": 4}[c]
}

// Level is the effective stakes: production raises criticality by one (floor low).
func Level(p Profile) int {
	l := rank(p.Criticality)
	if p.Criticality == "" {
		l = 2
	}
	if p.Stage == "production" {
		l = max(l+1, 2)
	}
	return l
}

func anyOf(xs []string, f func(string) bool) bool {
	return slices.Contains(xs, "*") || slices.ContainsFunc(xs, f)
}

func matches(e Entry, p Profile, phase, role string, level int) bool {
	return e.Never == "" &&
		anyOf(e.Phases, func(x string) bool { return x == phase }) &&
		anyOf(e.Roles, func(x string) bool { return x == role }) &&
		anyOf(e.ProjectTypes, func(x string) bool { return x == p.ProjectType }) &&
		anyOf(e.Stacks, func(x string) bool { return slices.Contains(p.Stacks, x) }) &&
		rank(e.MinCriticality) <= max(level, 1)
}

// Plan is the minimal skill set for (type, stacks, stakes, phase, role), with requirements,
// in catalog order. Trivial work gets at most one skill; production-critical gets all that match.
func Plan(c Catalog, p Profile, phase, role string) []Entry {
	level := Level(p)
	var out []Entry
	seen := map[string]bool{}
	var push func(e Entry)
	push = func(e Entry) {
		if seen[e.Name] || e.Never != "" {
			return
		}
		seen[e.Name] = true
		out = append(out, e)
		for _, r := range e.Requires {
			if d, ok := c.Get(r); ok {
				push(d)
			}
		}
	}
	for _, e := range c.Skills {
		if matches(e, p, phase, role, level) {
			push(e)
		}
	}
	if level == 0 && len(out) > 1 {
		out = out[:1]
	}
	return out
}

// PlanAll is the union of Plan over every phase and role.
func PlanAll(c Catalog, p Profile) []Entry {
	var out []Entry
	seen := map[string]bool{}
	for _, ph := range Phases {
		for _, r := range Roles {
			for _, e := range Plan(c, p, ph, r) {
				if !seen[e.Name] {
					seen[e.Name] = true
					out = append(out, e)
				}
			}
		}
	}
	return out
}
