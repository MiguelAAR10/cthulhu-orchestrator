package internal_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// docs/harness-v2/attacks.md has one line per known attack: "- **ID** … → refs" where every
// `path:Test` or `path:Test/row` names the test that reproduces it, or "→ limit until step N
// (accepted: …)". For a .go file the test function must exist and the row, a literal fragment
// of the row's source, must appear in the file; for a .sh file the part after ':' is a case label.

var tick = regexp.MustCompile("`([^`]*)`")

func checkIndex(root, text string) []string {
	var bad []string
	for _, line := range strings.Split(text, "\n") {
		if !strings.HasPrefix(line, "- **") {
			continue
		}
		_, refs, ok := strings.Cut(line, " → ")
		switch {
		case !ok:
			bad = append(bad, "no → in: "+line)
			continue
		case strings.HasPrefix(refs, "limit until step"):
			if !strings.Contains(refs, "(accepted:") {
				bad = append(bad, "limit without (accepted: …): "+line)
			}
			continue
		}
		spans := tick.FindAllStringSubmatch(refs, -1)
		if len(spans) == 0 {
			bad = append(bad, "neither a test nor a limit: "+line)
		}
		for _, s := range spans {
			if p := checkRef(root, s[1]); p != "" {
				bad = append(bad, p)
			}
		}
	}
	return bad
}

func checkRef(root, ref string) string {
	file, name, ok := strings.Cut(ref, ":")
	if !ok {
		return "not path:Test: " + ref
	}
	raw, err := os.ReadFile(filepath.Join(root, file))
	if err != nil {
		return "missing file: " + ref
	}
	src := string(raw)
	if strings.HasSuffix(file, ".sh") {
		if !strings.Contains(src, name) {
			return "missing label: " + ref
		}
		return ""
	}
	test, row, _ := strings.Cut(name, "/")
	switch {
	case !strings.Contains(src, "func "+test+"("):
		return "missing test: " + ref
	case row != "" && !strings.Contains(src, row):
		return "missing row: " + ref
	}
	return ""
}

func TestCheckIndexFindsBadLines(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "a_test.go"), []byte("func TestA(t *testing.T) { rows := []string{\"git log x\"} }\n"), 0o644)
	os.WriteFile(filepath.Join(root, "s.sh"), []byte(`ok "$(q x)" 0 "a label"`+"\n"), 0o644)
	bad := []string{
		"- **A1** no arrow",
		"- **A2** x → see the review",
		"- **A3** x → limit until step 2",
		"- **A4** x → `nope_test.go:TestA`",
		"- **A5** x → `a_test.go:TestB`",
		"- **A6** x → `a_test.go:TestA/git log y`",
		"- **A7** x → `s.sh:other label`",
	}
	if got := checkIndex(root, strings.Join(bad, "\n")); len(got) != len(bad) {
		t.Errorf("%d problems, want %d: %q", len(got), len(bad), got)
	}
	good := "- **G1** x → `a_test.go:TestA/git log x`, `s.sh:a label`\n- **G2** y → limit until step 2 (accepted: review 8)"
	if got := checkIndex(root, good); len(got) != 0 {
		t.Errorf("good lines flagged: %q", got)
	}
}

func TestAttackIndex(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "docs", "harness-v2", "attacks.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "\n- **") {
		t.Fatal("attacks.md lists no attack")
	}
	for _, p := range checkIndex(root, string(raw)) {
		t.Error(p)
	}
}
