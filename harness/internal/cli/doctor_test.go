package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// git branch --merged marca con "+ " las ramas sacadas en otro worktree: doctor no debe proponer
// `git branch -d +` ni saltarse la rama; y sus líneas salen en inglés por defecto.
func TestDoctorRamaEnWorktree(t *testing.T) {
	root, _ := sliceConFake(t, false)
	t.Setenv("ORQ_LANG", "")
	t.Setenv("ORQ_ROL", "")
	wt := filepath.Join(os.Getenv("ORQ_WORKTREES"), "viejo")
	if out, err := exec.Command("git", "-C", root, "worktree", "add", "-q", "-b", "slice/99-viejo", wt).CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	out, errs, _ := run(t, "doctor")
	s := out + errs
	if !strings.Contains(s, "git branch -d slice/99-viejo") {
		t.Fatalf("doctor no vio la rama integrada en un worktree:\n%s", s)
	}
	if strings.Contains(s, "branch -d +") || strings.Contains(s, "-d *") {
		t.Fatalf("doctor propuso borrar un marcador de git:\n%s", s)
	}
	if m := marcasES.FindAllString(s, -1); len(m) > 0 || strings.Contains(s, "sobrante") || strings.Contains(s, "quitar") {
		t.Fatalf("español en doctor: %v\n%s", m, s)
	}
}
