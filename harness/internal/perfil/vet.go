package perfil

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"orq/internal/orqdir"
)

// MaxSkillBytes keeps skills cheap: a SKILL.md larger than this is rejected.
const MaxSkillBytes = 20000

var forbidden = []struct {
	re  *regexp.Regexp
	why string
}{
	{regexp.MustCompile(`git\s+push`), "git push: nothing ships without the Owner"},
	{regexp.MustCompile(`--force\b|\bpush\s+-f\b`), "--force: orq never forces"},
	{regexp.MustCompile(`rm\s+-[a-zA-Z]*r[a-zA-Z]*f|rm\s+-[a-zA-Z]*f[a-zA-Z]*r`), "rm -rf: cleanup is `orq gc`"},
	{regexp.MustCompile(`reset\s+--hard|branch\s+-D\b`), "destructive git (reset --hard / branch -D)"},
	{regexp.MustCompile(`(?i)(spawn|dispatch|launch)\w*\s+(\w+\s+){0,3}(sub-?)?agents?\b|\bTask tool\b|\bAgent tool\b`), "spawning agents: only orq admits sessions (H-02)"},
	{regexp.MustCompile(`(curl|wget)[^|\n]*\|\s*(ba|z)?sh`), "pipes a download into a shell"},
}

var negation = regexp.MustCompile(`(?i)\b(never|do not|don't|must not|nunca|forbidden)\b`)

// Vet is a static check of a SKILL.md: frontmatter, size, forbidden actions.
// A line that forbids the action ("never git push") does not count.
func Vet(raw []byte) (bool, []string) {
	var why []string
	s := string(raw)
	if len(raw) > MaxSkillBytes {
		why = append(why, fmt.Sprintf("size %d bytes > %d", len(raw), MaxSkillBytes))
	}
	fm, _, ok := strings.Cut(strings.TrimPrefix(s, "---\n"), "\n---")
	if !strings.HasPrefix(s, "---\n") || !ok || !regexp.MustCompile(`(?m)^name:\s*\S`).MatchString(fm) || !regexp.MustCompile(`(?m)^description:\s*\S`).MatchString(fm) {
		why = append(why, "frontmatter missing or without name/description")
	}
	for _, line := range strings.Split(s, "\n") {
		if negation.MatchString(line) {
			continue
		}
		for _, f := range forbidden {
			if f.re.MatchString(line) {
				why = append(why, f.why+": "+strings.TrimSpace(line))
			}
		}
	}
	return len(why) == 0, why
}

// GalleryRecord is the vetting verdict of one skill.
type GalleryRecord struct {
	Verdict string `json:"verdict"` // validated | rejected
	Reason  string `json:"reason,omitempty"`
	SHA256  string `json:"sha256,omitempty"`
	At      string `json:"at"`
}

type Gallery map[string]GalleryRecord

func LoadGallery(path string) Gallery {
	g := Gallery{}
	if raw, err := os.ReadFile(path); err == nil {
		json.Unmarshal(raw, &g)
	}
	return g
}

func (g Gallery) Save(path string) error {
	raw, _ := json.MarshalIndent(g, "", "  ")
	return orqdir.WriteAtomic(path, raw)
}

func (g Gallery) Record(id string, ok bool, why []string, sum string) GalleryRecord {
	r := GalleryRecord{Verdict: "validated", SHA256: sum, At: time.Now().UTC().Format(time.RFC3339)}
	if !ok {
		r.Verdict, r.Reason = "rejected", strings.Join(why, "; ")
	}
	g[id] = r
	return r
}

// Runner runs a command in dir; tests inject a fake so nothing real is installed.
type Runner func(dir string, argv []string) ([]byte, error)

func ExecRunner(dir string, argv []string) ([]byte, error) {
	c := exec.Command(argv[0], argv[1:]...)
	c.Dir = dir
	return c.CombinedOutput()
}

// Install puts each entry into .claude/skills and .agents/skills of root, vets it and pins its hash.
// It refuses never-list skills, skills that fail vetting, and skills that differ from the catalog
// hash or from the hash already pinned in lock. local holds orq's own skills under orqskills/.
func Install(root string, c Catalog, es []Entry, local fs.FS, run Runner, lock map[string]Fijada) (map[string]Fijada, error) {
	for _, e := range es {
		if r, bad := c.NeverReason(e.Name); bad || e.Never != "" {
			return nil, fmt.Errorf("%s is on the never-list: %s", e.Name, r+e.Never)
		}
	}
	out := map[string]Fijada{}
	for k, v := range lock {
		out[k] = v
	}
	for _, e := range es {
		dst := filepath.Join(root, ".claude", "skills", e.Name)
		if !Instalada(root, e.Name) {
			if err := fetch(root, e, local, run); err != nil {
				return out, fmt.Errorf("%s: %w", e.Name, err)
			}
		}
		raw, err := os.ReadFile(filepath.Join(dst, "SKILL.md"))
		if err != nil {
			return out, fmt.Errorf("%s: install left no SKILL.md", e.Name)
		}
		if ok, why := Vet(raw); !ok {
			os.RemoveAll(dst)
			return out, fmt.Errorf("%s rejected by vetting: %s", e.Name, strings.Join(why, "; "))
		}
		h, err := HuellaSkill(root, e.Name)
		if err != nil {
			return out, err
		}
		if e.SHA256 != "" && h != e.SHA256 {
			return out, fmt.Errorf("%s: content hash %s differs from the catalog pin %s", e.Name, h[:12], e.SHA256)
		}
		if f, ok := lock[e.Name]; ok && f.Huella != h {
			return out, fmt.Errorf("%s differs from the pinned version: review the change, then `orq skills fijar`", e.Name)
		}
		if err := mirror(dst, filepath.Join(root, ".agents", "skills", e.Name)); err != nil {
			return out, err
		}
		out[e.Name] = Fijada{ID: e.ID, Ref: e.Ref, Repo: e.Source, Huella: h, Roles: e.Roles, Adapta: e.Adapt}
	}
	return out, nil
}

func fetch(root string, e Entry, local fs.FS, run Runner) error {
	dst := filepath.Join(root, ".claude", "skills", e.Name)
	if name, ok := strings.CutPrefix(e.Install, "copy:"); ok {
		var src fs.FS
		dir := "orqskills/" + name
		if local != nil {
			if _, err := fs.Stat(local, dir+"/SKILL.md"); err == nil {
				src = local
			}
		}
		if src == nil {
			src, dir = os.DirFS(filepath.Join(root, ".orq")), name
		}
		sub, err := fs.Sub(src, dir)
		if err != nil {
			return err
		}
		if _, err := fs.Stat(sub, "SKILL.md"); err != nil {
			return fmt.Errorf("no source for copy:%s (vendor it under .orq/%s/)", name, name)
		}
		return os.CopyFS(dst, sub)
	}
	argv := strings.Fields(e.Install)
	if len(argv) == 0 || argv[0] != "npx" {
		return errors.New("install must be `npx ...` or `copy:NAME`")
	}
	if out, err := run(root, argv); err != nil {
		return fmt.Errorf("%s: %v %s", e.Install, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func mirror(src, dst string) error {
	os.RemoveAll(dst)
	return os.CopyFS(dst, os.DirFS(src))
}
