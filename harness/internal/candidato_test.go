package internal_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The suites run by make ci must prove they run the candidate orq it just built, never an
// installed one that comes first in PATH (today both print 1.0.0).

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func head(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("git", "-C", repoRoot(t), "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}

// fakeOrq writes an orq in its own directory that prints line for `orq version`.
func fakeOrq(t *testing.T, line string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "orq")
	if err := os.WriteFile(p, []byte("#!/bin/sh\necho '"+line+"'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func run(t *testing.T, env []string, args ...string) (string, int) {
	t.Helper()
	c := exec.Command("bash", args...)
	c.Dir = t.TempDir()
	c.Env = append(os.Environ(), env...)
	out, err := c.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return string(out), code
}

func TestCandidatoGuard(t *testing.T) {
	lib := filepath.Join(repoRoot(t), "validacion", "candidato.sh")
	cand := fakeOrq(t, "1.0.0 revision="+head(t)+" modified=false")
	otro := fakeOrq(t, "1.0.0 revision="+head(t)+" modified=false")
	viejo := fakeOrq(t, "1.0.0 revision=deadbeef modified=false")
	cases := []struct {
		name, candidato, probado string
		ok                       bool
	}{
		{"the candidate", cand, cand, true},
		{"another path", cand, otro, false},
		{"another revision", viejo, viejo, false},
		{"no candidate named", "", cand, false},
	}
	for _, c := range cases {
		out, code := run(t, []string{"ORQ_CANDIDATO=" + c.candidato}, "-c", `. "$1" && candidato "$2"`, "_", lib, c.probado)
		if c.ok && code != 0 {
			t.Errorf("%s: refused (%d): %s", c.name, code, out)
		}
		if !c.ok && (code == 0 || !strings.Contains(out, "not the candidate")) {
			t.Errorf("%s: accepted or wrong message (%d): %s", c.name, code, out)
		}
	}
}

func TestE2EStopsOnAStaleOrqFirstInPath(t *testing.T) {
	cand := fakeOrq(t, "1.0.0 revision="+head(t)+" modified=false")
	stub := fakeOrq(t, "1.0.0")
	path := filepath.Dir(stub) + ":" + filepath.Dir(cand) + ":/usr/bin:/bin"
	out, code := run(t, []string{"PATH=" + path, "ORQ_CANDIDATO=" + cand},
		filepath.Join(repoRoot(t), "validacion", "e2e-v05.sh"), t.TempDir())
	if code == 0 || !strings.Contains(out, "not the candidate") || strings.Contains(out, "== S-01") {
		t.Fatalf("exit %d, the suite did not stop at the guard:\n%s", code, out)
	}
}

func TestAislamientoStopsOnAnotherOrq(t *testing.T) {
	cand := fakeOrq(t, "1.0.0 revision="+head(t)+" modified=false")
	stub := fakeOrq(t, "1.0.0")
	out, code := run(t, []string{"ORQ=" + stub, "ORQ_CANDIDATO=" + cand, "TMPDIR=" + t.TempDir()},
		filepath.Join(repoRoot(t), "validacion", "aislamiento.sh"))
	if code == 0 || !strings.Contains(out, "not the candidate") || strings.Contains(out, "== Parte 1") {
		t.Fatalf("exit %d, the suite did not stop at the guard:\n%s", code, out)
	}
}
