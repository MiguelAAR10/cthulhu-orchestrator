package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"orq/internal/orqdir"
)

// fixUpdate is a throwaway world for `orq2 update`: a source repo with a harness-v2 branch (two
// commits), a fake go that "builds" a script reporting the checked-out revision, an installed
// ~/bin/orq2 reporting the first commit, a rollback dir already holding an older binary, an empty
// /proc and a temp TMPDIR. Every path is overridden by env: nothing touches the real ~/bin,
// ~/.local/share or the main checkout.
type fixUpdate struct {
	root, repo, bin, rb, tmp, proc, wt string
	viejo, punta                       string // commits: installed, harness-v2 tip
	instalado                          []byte // bytes of the installed bin/orq2
}

func gitT(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func scriptVersion(rev string) []byte {
	return []byte("#!/bin/sh\necho '2.0.0-pilot revision=" + rev + " modified=false'\n")
}

func nuevoFixUpdate(t *testing.T) *fixUpdate {
	t.Helper()
	root := t.TempDir()
	f := &fixUpdate{root: root, repo: filepath.Join(root, "cthulhu"), bin: filepath.Join(root, "bin"),
		rb: filepath.Join(root, "rollback"), tmp: filepath.Join(root, "tmp"), proc: filepath.Join(root, "proc"), wt: filepath.Join(root, "wt")}
	for _, d := range []string{f.repo, filepath.Join(f.repo, "harness"), f.bin, f.rb, f.tmp, f.proc, f.wt} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for k, v := range map[string]string{"GIT_AUTHOR_NAME": "t", "GIT_AUTHOR_EMAIL": "t@t", "GIT_COMMITTER_NAME": "t", "GIT_COMMITTER_EMAIL": "t@t"} {
		t.Setenv(k, v)
	}
	gitT(t, f.repo, "init", "-q", "-b", "main")
	os.WriteFile(filepath.Join(f.repo, "harness", "a.txt"), []byte("1\n"), 0o644)
	gitT(t, f.repo, "add", "-A")
	gitT(t, f.repo, "commit", "-qm", "uno")
	f.viejo = gitT(t, f.repo, "rev-parse", "HEAD")
	gitT(t, f.repo, "checkout", "-q", "-b", "harness-v2")
	os.WriteFile(filepath.Join(f.repo, "harness", "a.txt"), []byte("2\n"), 0o644)
	gitT(t, f.repo, "commit", "-qam", "dos")
	f.punta = gitT(t, f.repo, "rev-parse", "HEAD")
	// The main checkout is left dirty and on another branch: update must build from the exact SHA.
	gitT(t, f.repo, "checkout", "-q", "main")
	os.WriteFile(filepath.Join(f.repo, "harness", "a.txt"), []byte("sucio\n"), 0o644)

	// Fake go: `go build -trimpath -o OUT ./cmd/orq` run in <clone>/harness writes OUT as a script
	// that reports the clone's HEAD and whether its tree is dirty, like the real -buildvcs stamp.
	gofalso := filepath.Join(root, "go")
	os.WriteFile(gofalso, []byte(`#!/bin/sh
out=""
while [ $# -gt 0 ]; do [ "$1" = "-o" ] && { shift; out="$1"; }; shift; done
[ -n "$out" ] || exit 2
echo "$PWD $GOTOOLCHAIN $GOPROXY" > "$out.args"; rm -f "$out.args"
rev=$(git rev-parse HEAD) || exit 3
mod=false; [ -n "$(git status --porcelain)" ] && mod=true
[ -n "$FALSO_MODIFICADO" ] && mod=true
printf '#!/bin/sh\necho "2.0.0-pilot revision=%s modified=%s"\n' "$rev" "$mod" > "$out"
chmod 755 "$out"
`), 0o755)

	f.instalado = scriptVersion(f.viejo)
	os.WriteFile(filepath.Join(f.bin, "orq2"), f.instalado, 0o755)
	os.WriteFile(filepath.Join(f.rb, "orq2-24f2438"), []byte("binario viejo del integrador\n"), 0o755)

	t.Setenv("ORQ_NOMBRE", "orq2")
	t.Setenv("ORQ_ROL", "")
	t.Setenv("ORQ_ROOT", "")
	t.Setenv("ORQ_UPDATE_REPO", f.repo)
	t.Setenv("ORQ_UPDATE_BIN", f.bin)
	t.Setenv("ORQ_UPDATE_ROLLBACK", f.rb)
	t.Setenv("ORQ_UPDATE_GO", gofalso)
	t.Setenv("ORQ_UPDATE_PROC", f.proc)
	t.Setenv("ORQ_UPDATE_PROYECTOS", "")
	t.Setenv("ORQ_WORKTREES", f.wt)
	t.Setenv("TMPDIR", f.tmp)
	t.Setenv("FALSO_MODIFICADO", "")

	viejo := esTerminal
	esTerminal = func(io.Reader) bool { return true }
	t.Cleanup(func() { esTerminal = viejo })
	return f
}

func (f *fixUpdate) registrar(t *testing.T, sha string) {
	t.Helper()
	out, errs, code := runIn(t, "update", "--registrar", sha, "--revisor", "opus-fresco", "--veredicto", "sin-high", "--ci-exit", "0", "--ci-log", "/tmp/ci.log")
	if code != 0 {
		t.Fatalf("registrar %s: %d\n%s%s", sha, code, out, errs)
	}
}

// runIn runs the CLI with an empty, non-terminal stdin (esTerminal decides what counts as a TTY).
func runIn(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	var out, errb bytes.Buffer
	code := Run(args, strings.NewReader(""), &out, &errb)
	return out.String(), errb.String(), code
}

func (f *fixUpdate) binIntacto(t *testing.T) {
	t.Helper()
	got, err := os.ReadFile(filepath.Join(f.bin, "orq2"))
	if err != nil || !bytes.Equal(got, f.instalado) {
		t.Fatalf("bin/orq2 changed (%v):\n%s", err, got)
	}
	if _, err := os.Stat(filepath.Join(f.rb, "orq2-anterior")); !os.IsNotExist(err) {
		t.Fatalf("a rollback was written on a refused update: %v", err)
	}
}

// foto is every file under dir with its mode, size, mtime and content hash.
func foto(t *testing.T, dir string) map[string]string {
	t.Helper()
	m := map[string]string{}
	filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		fi, err := os.Lstat(p)
		if err != nil {
			return err
		}
		h := ""
		if fi.Mode().IsRegular() {
			raw, _ := os.ReadFile(p)
			s := sha256.Sum256(raw)
			h = hex.EncodeToString(s[:])
		}
		m[p] = fmt.Sprintf("%v %d %d %s", fi.Mode(), fi.Size(), fi.ModTime().UnixNano(), h)
		return nil
	})
	return m
}

func igualFoto(t *testing.T, antes, despues map[string]string) {
	t.Helper()
	for p, v := range antes {
		if despues[p] != v {
			t.Errorf("changed: %s\n  before %s\n  after  %s", p, v, despues[p])
		}
	}
	for p := range despues {
		if _, ok := antes[p]; !ok {
			t.Errorf("new file: %s", p)
		}
	}
}

// An ungated tip is refused, and nothing is written.
func TestUpdateSinRegistroSeNiega(t *testing.T) {
	f := nuevoFixUpdate(t)
	_, errs, code := runIn(t, "update")
	if code == 0 || !strings.Contains(errs, f.punta[:12]) || !strings.Contains(errs, "registro de puerta") {
		t.Fatalf("ungated tip not refused: %d\n%s", code, errs)
	}
	f.binIntacto(t)
}

// A record for the parent does not count, and neither does a record attached to the tip that
// names another SHA.
func TestUpdateRegistroDeOtroSHASeNiega(t *testing.T) {
	f := nuevoFixUpdate(t)
	f.registrar(t, f.viejo)
	if _, errs, code := runIn(t, "update"); code == 0 || !strings.Contains(errs, "registro de puerta") {
		t.Fatalf("parent's record accepted for the tip: %d\n%s", code, errs)
	}
	f.binIntacto(t)
	falso, _ := json.Marshal(map[string]any{"esquema": 1, "sha": f.viejo, "fecha": "2026-11-20T00:00:00Z",
		"revision": map[string]string{"veredicto": "sin-high", "revisor": "x"}, "ci": map[string]any{"comando": "make ci", "exit": 0, "log": "x"}})
	gitT(t, f.repo, "notes", "--ref="+refPuerta, "add", "-m", string(falso), f.punta)
	if _, errs, code := runIn(t, "update"); code == 0 || !strings.Contains(errs, f.viejo[:12]) {
		t.Fatalf("record naming another SHA accepted: %d\n%s", code, errs)
	}
	f.binIntacto(t)
}

// The writer refuses a record that is not a pass, and an unknown SHA.
func TestUpdateRegistrarSoloPuertaVerde(t *testing.T) {
	f := nuevoFixUpdate(t)
	base := []string{"update", "--registrar", f.punta, "--revisor", "r", "--ci-log", "l"}
	for _, extra := range [][]string{
		{"--veredicto", "high", "--ci-exit", "0"},
		{"--veredicto", "sin-high", "--ci-exit", "2"},
		{"--veredicto", "sin-high"},
	} {
		if _, errs, code := runIn(t, append(base, extra...)...); code == 0 {
			t.Fatalf("%v recorded: %s", extra, errs)
		}
	}
	if _, errs, code := runIn(t, "update", "--registrar", strings.Repeat("ab", 20), "--revisor", "r", "--veredicto", "sin-high", "--ci-exit", "0", "--ci-log", "l"); code == 0 {
		t.Fatalf("unknown SHA recorded: %s", errs)
	}
	if out, _ := exec.Command("git", "-C", f.repo, "notes", "--ref="+refPuerta, "list").Output(); len(out) != 0 {
		t.Fatalf("records written: %s", out)
	}
	if _, errs, code := runIn(t, "update", "--registrar", f.punta[:10], "--revisor", "r", "--veredicto", "sin-high", "--ci-exit", "0", "--ci-log", "l"); code != 0 {
		t.Fatalf("short SHA not resolved: %s", errs)
	}
	nota := gitT(t, f.repo, "notes", "--ref="+refPuerta, "show", f.punta)
	if !strings.Contains(nota, `"sha": "`+f.punta+`"`) || !strings.Contains(nota, `"esquema": 1`) {
		t.Fatalf("record:\n%s", nota)
	}
}

func proyectoConSlice(t *testing.T, dir, nn, fase string) {
	t.Helper()
	os.MkdirAll(filepath.Join(dir, ".orq", "slices"), 0o755)
	raw, _ := json.Marshal(map[string]any{"nn": nn, "intento": 1, "fase": fase, "titulo": "t"})
	os.WriteFile(filepath.Join(dir, ".orq", "slices", nn+".json"), raw, 0o644)
}

// Any slice of any known project that is verifying or closing stops the update; the refusal
// says which projects it looked at.
func TestUpdateSliceVerificandoOCerrandoSeNiega(t *testing.T) {
	f := nuevoFixUpdate(t)
	f.registrar(t, f.punta)

	// Closing, in a project named by ORQ_UPDATE_PROYECTOS.
	p1 := filepath.Join(f.root, "tienda")
	proyectoConSlice(t, p1, "01", "CERRANDO")
	t.Setenv("ORQ_UPDATE_PROYECTOS", p1)
	_, errs, code := runIn(t, "update")
	if code == 0 || !strings.Contains(errs, p1) || !strings.Contains(errs, "S-01") || !strings.Contains(errs, "CERRANDO") {
		t.Fatalf("closing slice not refused: %d\n%s", code, errs)
	}
	f.binIntacto(t)

	// Integrating, in a project found through a worker clone's marker under ORQ_WORKTREES.
	t.Setenv("ORQ_UPDATE_PROYECTOS", "")
	p2 := filepath.Join(f.root, "agenda")
	proyectoConSlice(t, p2, "03", "INTEGRANDO")
	clon := filepath.Join(f.wt, "agenda-s03", ".git")
	os.MkdirAll(clon, 0o755)
	marca, _ := json.Marshal(map[string]any{"root": p2, "slice": "03"})
	os.WriteFile(filepath.Join(clon, orqdir.MarcaClon), marca, 0o644)
	if _, errs, code := runIn(t, "update"); code == 0 || !strings.Contains(errs, p2) || !strings.Contains(errs, "S-03") {
		t.Fatalf("integrating slice behind a clone marker not refused: %d\n%s", code, errs)
	}
	f.binIntacto(t)
	proyectoConSlice(t, p2, "03", "VERIFICADO")

	// Verifying: a running `orq2 slice verificar` (the long exam holds no lock).
	pid := filepath.Join(f.proc, "4242")
	os.MkdirAll(pid, 0o755)
	os.WriteFile(filepath.Join(pid, "cmdline"), []byte("/home/x/bin/orq2\x00slice\x00verificar\x00--slice\x0003\x00"), 0o644)
	os.Symlink(p2, filepath.Join(pid, "cwd"))
	if _, errs, code := runIn(t, "update"); code == 0 || !strings.Contains(errs, "verificar") || !strings.Contains(errs, "4242") {
		t.Fatalf("running verify not refused: %d\n%s", code, errs)
	}
	f.binIntacto(t)
	os.RemoveAll(pid)

	// A slice lock held by another orq process.
	lk, err := os.OpenFile(filepath.Join(p2, ".orq", "slice-03.lock"), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.Flock(int(lk.Fd()), syscall.LOCK_EX); err != nil {
		t.Fatal(err)
	}
	if _, errs, code := runIn(t, "update"); code == 0 || !strings.Contains(errs, "slice-03.lock") {
		t.Fatalf("held slice lock not refused: %d\n%s", code, errs)
	}
	f.binIntacto(t)
	syscall.Flock(int(lk.Fd()), syscall.LOCK_UN)
	lk.Close()

	// An unreadable slice state fails closed.
	os.WriteFile(filepath.Join(p2, ".orq", "slices", "04.json"), []byte("{roto"), 0o644)
	if _, errs, code := runIn(t, "update"); code == 0 || !strings.Contains(errs, "04.json") {
		t.Fatalf("unreadable slice state not refused: %d\n%s", code, errs)
	}
	f.binIntacto(t)
	os.Remove(filepath.Join(p2, ".orq", "slices", "04.json"))

	// Nothing busy: it goes through and names the projects it checked.
	out, errs, code := runIn(t, "update")
	if code != 0 || !strings.Contains(out, p2) {
		t.Fatalf("update with idle projects: %d\n%s%s", code, out, errs)
	}
}

// Not a terminal, a role panel or a worker directory: refused.
func TestUpdateSoloOwnerDesdeTTY(t *testing.T) {
	f := nuevoFixUpdate(t)
	f.registrar(t, f.punta)
	esTerminal = func(io.Reader) bool { return false }
	for _, args := range [][]string{{"update"}, {"update", "--volver"}, {"update", "--ver"}} {
		if _, errs, code := runIn(t, args...); code == 0 || !strings.Contains(errs, "terminal") {
			t.Fatalf("%v without a TTY: %d %s", args, code, errs)
		}
	}
	esTerminal = func(io.Reader) bool { return true }
	t.Setenv("ORQ_ROL", "worker-front-s01")
	if _, errs, code := runIn(t, "update"); code == 0 || !strings.Contains(errs, "worker-front-s01") {
		t.Fatalf("role panel: %d %s", code, errs)
	}
	t.Setenv("ORQ_ROL", "")
	// The real stdin of `go test` is not a terminal either.
	if esTerminalReal(strings.NewReader("")) {
		t.Fatal("a strings.Reader counts as a terminal")
	}
	f.binIntacto(t)
}

// Under identity orq, `update` is an unknown command exactly as in 1.0.0, and usage does not
// offer it.
func TestUpdateNoExisteBajoOrq(t *testing.T) {
	nuevoFixUpdate(t)
	t.Setenv("ORQ_NOMBRE", "orq")
	_, errU, codeU := runIn(t, "update")
	_, errX, codeX := runIn(t, "nocomando")
	if codeU != 1 || codeX != 1 || errU != errX || errU != usage {
		t.Fatalf("orq update differs from an unknown command: %d %d\n%s", codeU, codeX, errU)
	}
	if _, errs, _ := runIn(t, "update", "--ver"); errs != usage {
		t.Fatalf("orq update --ver: %s", errs)
	}
	if strings.Contains(usage, "update") {
		t.Fatal("orq usage offers update")
	}
	t.Setenv("ORQ_NOMBRE", "orq2")
	if !strings.Contains(uso(), "orq2 update") {
		t.Fatalf("orq2 usage does not offer update:\n%s", uso())
	}
}

// A gated tip installs atomically (rename in the same dir), keeps the old binary as rollback,
// never touches the integrator's older rollback, leaves no temp files and prints old -> new.
func TestUpdateInstalaAtomicoYDiceViejoNuevo(t *testing.T) {
	f := nuevoFixUpdate(t)
	f.registrar(t, f.punta)
	antes, _ := os.Stat(filepath.Join(f.bin, "orq2"))
	viejoRB, _ := os.ReadFile(filepath.Join(f.rb, "orq2-24f2438"))
	out, errs, code := runIn(t, "update")
	if code != 0 {
		t.Fatalf("update: %d\n%s%s", code, out, errs)
	}
	if !strings.Contains(out, f.viejo+" -> "+f.punta) {
		t.Fatalf("no old -> new line:\n%s", out)
	}
	despues, _ := os.Stat(filepath.Join(f.bin, "orq2"))
	if os.SameFile(antes, despues) {
		t.Fatal("bin/orq2 was rewritten in place, not replaced by rename")
	}
	v, err := exec.Command(filepath.Join(f.bin, "orq2"), "version").Output()
	if err != nil || !strings.Contains(string(v), "revision="+f.punta+" modified=false") {
		t.Fatalf("installed binary reports %q (%v)", v, err)
	}
	rb, _ := os.ReadFile(filepath.Join(f.rb, "orq2-anterior"))
	if !bytes.Equal(rb, f.instalado) {
		t.Fatalf("rollback is not the old binary:\n%s", rb)
	}
	if got, _ := os.ReadFile(filepath.Join(f.rb, "orq2-24f2438")); !bytes.Equal(got, viejoRB) {
		t.Fatal("the integrator's rollback was touched")
	}
	for _, d := range []string{f.bin, f.rb} {
		es, _ := os.ReadDir(d)
		for _, e := range es {
			if strings.HasPrefix(e.Name(), ".") {
				t.Errorf("temp file left: %s/%s", d, e.Name())
			}
		}
	}
	if es, _ := os.ReadDir(f.tmp); len(es) != 0 {
		t.Errorf("build clone left in TMPDIR: %v", es)
	}
	// Same tip again: nothing to do, nothing written.
	ft := foto(t, f.root)
	out, errs, code = runIn(t, "update")
	if code != 0 || !strings.Contains(out, "ya") {
		t.Fatalf("second update: %d\n%s%s", code, out, errs)
	}
	igualFoto(t, ft, foto(t, f.root))
}

// The candidate must report the exact SHA and modified=false, or nothing is installed.
func TestUpdateCandidatoDebeDecirLaRevision(t *testing.T) {
	f := nuevoFixUpdate(t)
	f.registrar(t, f.punta)
	t.Setenv("FALSO_MODIFICADO", "1")
	if _, errs, code := runIn(t, "update"); code == 0 || !strings.Contains(errs, "modified=true") {
		t.Fatalf("dirty candidate installed: %d\n%s", code, errs)
	}
	f.binIntacto(t)
	if es, _ := os.ReadDir(f.tmp); len(es) != 0 {
		t.Errorf("build clone left in TMPDIR: %v", es)
	}
}

// --volver puts back the rollback byte for byte, atomically, and prints the revisions.
func TestUpdateVolverRestauraIdentico(t *testing.T) {
	f := nuevoFixUpdate(t)
	if _, errs, code := runIn(t, "update", "--volver"); code == 0 || !strings.Contains(errs, "orq2-anterior") {
		t.Fatalf("--volver without a rollback: %d %s", code, errs)
	}
	f.registrar(t, f.punta)
	if _, errs, code := runIn(t, "update"); code != 0 {
		t.Fatalf("update: %s", errs)
	}
	antes, _ := os.Stat(filepath.Join(f.bin, "orq2"))
	out, errs, code := runIn(t, "update", "--volver")
	if code != 0 || !strings.Contains(out, f.punta+" -> "+f.viejo) {
		t.Fatalf("--volver: %d\n%s%s", code, out, errs)
	}
	got, _ := os.ReadFile(filepath.Join(f.bin, "orq2"))
	if !bytes.Equal(got, f.instalado) {
		t.Fatalf("restored binary is not byte-identical:\n%s", got)
	}
	despues, _ := os.Stat(filepath.Join(f.bin, "orq2"))
	if os.SameFile(antes, despues) {
		t.Fatal("--volver rewrote bin/orq2 in place")
	}
	if despues.Mode().Perm()&0o100 == 0 {
		t.Fatalf("restored binary not executable: %v", despues.Mode())
	}
	// A busy project also stops --volver.
	p := filepath.Join(f.root, "tienda")
	proyectoConSlice(t, p, "01", "CERRANDO")
	t.Setenv("ORQ_UPDATE_PROYECTOS", p)
	if _, errs, code := runIn(t, "update", "--volver"); code == 0 || !strings.Contains(errs, "S-01") {
		t.Fatalf("--volver while closing: %d %s", code, errs)
	}
}

// --ver reports and never writes: not the bin, not the rollback, not the repo, not TMPDIR.
func TestUpdateVerNoEscribe(t *testing.T) {
	f := nuevoFixUpdate(t)
	ft := foto(t, f.root)
	out, errs, code := runIn(t, "update", "--ver")
	if code != 0 || !strings.Contains(out, f.punta) || !strings.Contains(out, "sin registro de puerta") || !strings.Contains(out, f.viejo) {
		t.Fatalf("--ver ungated: %d\n%s%s", code, out, errs)
	}
	igualFoto(t, ft, foto(t, f.root))

	f.registrar(t, f.punta)
	ft = foto(t, f.root)
	out, errs, code = runIn(t, "update", "--ver")
	if code != 0 || !strings.Contains(out, "hay una versión nueva con puerta") {
		t.Fatalf("--ver gated: %d\n%s%s", code, out, errs)
	}
	igualFoto(t, ft, foto(t, f.root))

	// A third, ungated commit: the tip is ungated, but the gated one after the installed is named.
	gitT(t, f.repo, "checkout", "-q", "--", "harness/a.txt")
	gitT(t, f.repo, "checkout", "-q", "harness-v2")
	os.WriteFile(filepath.Join(f.repo, "harness", "a.txt"), []byte("3\n"), 0o644)
	gitT(t, f.repo, "commit", "-qam", "tres")
	tres := gitT(t, f.repo, "rev-parse", "HEAD")
	ft = foto(t, f.root)
	out, errs, code = runIn(t, "update", "--ver")
	if code != 0 || !strings.Contains(out, tres) || !strings.Contains(out, "sin registro de puerta") || !strings.Contains(out, f.punta) {
		t.Fatalf("--ver ungated tip after a gated one: %d\n%s%s", code, out, errs)
	}
	igualFoto(t, ft, foto(t, f.root))
}
