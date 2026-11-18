package slice

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const ratifico = "Owner: S-01 was opened by orq 1.0.0, adopt it"

// aLegado turns a slice opened by this version into what orq 1.0.0 left on disk: a linked worktree
// instead of a clone, a state without the new fields (schema 0) and no contract in git.
func aLegado(t *testing.T, r Repo, e *Estado) {
	t.Helper()
	if err := os.RemoveAll(e.Worktree); err != nil {
		t.Fatal(err)
	}
	if _, err := Git(r.Root, "worktree", "add", "-q", e.Worktree, e.Rama); err != nil {
		t.Fatal(err)
	}
	if _, err := Git(r.Root, "update-ref", "-d", refContrato(e.NN)); err != nil {
		t.Fatal(err)
	}
	editarEstado(t, r, e.NN, func(m map[string]any) {
		for _, k := range []string{"sin_conteo", "en_sitio", "vigilados", "ruta", "ascendido", "ruta_motivo", "aislado", "esquema"} {
			delete(m, k)
		}
		m["procedencia"] = "git"
	})
}

// editarEstado rewrites the state on disk as someone with disk access would.
func editarEstado(t *testing.T, r Repo, nn string, f func(map[string]any)) {
	t.Helper()
	raw, err := os.ReadFile(r.estadoPath(nn))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	f(m)
	raw, _ = json.Marshal(m)
	if err := os.WriteFile(r.estadoPath(nn), raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func sinContrato(t *testing.T, r Repo, nn string) {
	t.Helper()
	if hay, err := r.leerBlob(refContrato(nn), &Contrato{}); err != nil || hay {
		t.Fatalf("a refused adoption must not write a contract: %v %v", hay, err)
	}
}

// Acceptance 6: a slice opened by orq 1.0.0 (schema 0, linked worktree, no contract in git) is never
// adopted by verify: verify names `orq slice convertir NN --ratifico`. Once converted with a
// ratification, it verifies and closes.
func TestEstadoViejoSigueVerificandoYCerrando(t *testing.T) {
	r := repoDeJuguete(t)
	e := abrir(t, r)
	aLegado(t, r, e)
	commitEn(t, e.Worktree, "total.txt", "170\n")
	_, _, err := r.Verificar("", t0)
	if !errors.Is(err, ErrSinContrato) || !strings.Contains(err.Error(), "orq slice convertir 01 --ratifico") {
		t.Fatalf("verify must not adopt a legacy slice and must name the conversion: %v", err)
	}
	sinContrato(t, r, "01")
	if got, _ := r.Load("01"); got.Procedencia == ProcedenciaLegado {
		t.Fatal("verify must not mark the slice as adopted")
	}
	if _, err := convertir(t, r, "01"); err != nil {
		t.Fatal(err)
	}
	got, _, err := r.Verificar("", t0)
	if err != nil || got.Fase != Verificado || got.Recibo.Contado || got.Procedencia != ProcedenciaLegado {
		t.Fatalf("%+v %v", got, err)
	}
	if reg, _, _, err := r.Cerrar("", true, "", t0.Add(time.Hour)); err != nil || reg.Result != "aceptado" || reg.Ruta != RutaSlice {
		t.Fatalf("%+v %v", reg, err)
	}
	if _, err := os.Stat(e.Worktree); !os.IsNotExist(err) {
		t.Fatal("the 1.0.0 linked worktree must be removed on close")
	}
}

// A legacy slice is verified like any other: on a clean copy of its commit in the box, never in the
// worktree. A file that only exists in the worktree does not make it pass.
func TestEstadoViejoVerificaEnCopiaLimpia(t *testing.T) {
	r := repoConReporte(t, `cmp -s total.txt tests/datos.txt || test -f ruido.txt
`)
	os.WriteFile(filepath.Join(r.Root, ".git", "info", "exclude"), []byte(".orq/\nruido.txt\n"), 0o644)
	e, _, err := r.Abrir("01", "t", "tests/s01.sh", examen, Opciones{SinConteo: "juguete"}, t0)
	if err != nil {
		t.Fatal(err)
	}
	aLegado(t, r, e)
	if _, err := convertir(t, r, "01"); err != nil {
		t.Fatal(err)
	}
	commitEn(t, e.Worktree, "otro.txt", "x\n")
	os.WriteFile(filepath.Join(e.Worktree, "ruido.txt"), []byte("x\n"), 0o644) // ignored: only in the worktree
	got, _, err := r.Verificar("", t0)
	if err == nil || got.Fase == Verificado {
		t.Fatalf("a legacy slice must verify on a clean copy, not in the worktree: %+v %v", got, err)
	}
	if rec := leerJSONL[Recibo](r.recibosPath()); len(rec) != 1 || !rec[0].Aislado {
		t.Fatalf("receipt must say it ran on a clean copy: %+v", rec)
	}
}

// Convertir refuses while the old processes hold the slice lock, without a ratification, and when the
// exam hash in the state does not match the exam at the red commit. It keeps the ratification in the
// contract.
func TestConvertirEstadoViejo(t *testing.T) {
	r := repoDeJuguete(t)
	e := abrir(t, r)
	aLegado(t, r, e)
	soltar, err := r.candado("slice-01")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Convertir("01", ratifico, codigoActual(t, r, "01"), io.Discard); err == nil || !strings.Contains(err.Error(), "stop") {
		t.Fatalf("adoption with the lock held must refuse and say to stop the processes: %v", err)
	}
	soltar()
	for _, vacio := range []string{"", "   "} {
		if _, err := r.Convertir("01", vacio, "", io.Discard); err == nil || !strings.Contains(err.Error(), "--ratifico") {
			t.Fatalf("adoption without a ratification must refuse: %v", err)
		}
	}
	sinContrato(t, r, "01")
	raw, _ := os.ReadFile(r.estadoPath("01"))
	bien := string(raw)
	os.WriteFile(r.estadoPath("01"), []byte(strings.Replace(bien, e.Huella, strings.Repeat("0", 64), 1)), 0o644)
	if _, err := r.Convertir("01", ratifico, codigoActual(t, r, "01"), io.Discard); err == nil || !strings.Contains(err.Error(), "hash") {
		t.Fatalf("a state whose exam hash does not match the red commit must not be adopted: %v", err)
	}
	sinContrato(t, r, "01")
	os.WriteFile(r.estadoPath("01"), []byte(bien), 0o644)
	var out bytes.Buffer
	got, err := r.Convertir("01", ratifico, codigoActual(t, r, "01"), &out)
	if err != nil || got.Procedencia != ProcedenciaLegado || r.confiable(got) != nil {
		t.Fatalf("%+v %v", got, err)
	}
	if !strings.Contains(out.String(), ratifico) { // what it seals is shown by the dry run (TestConvertirEnSecoYCodigo)
		t.Errorf("conversion must show the ratification; it showed:\n%s", out.String())
	}
	var c Contrato
	if hay, err := r.leerBlob(refContrato("01"), &c); err != nil || !hay || c.Ratifico != ratifico || c.Huella != e.Huella {
		t.Fatalf("the contract must keep the ratification: %v %v %+v", hay, err, c)
	}
	if _, err := r.Convertir("01", ratifico, codigoActual(t, r, "01"), io.Discard); err == nil || !strings.Contains(err.Error(), "nothing to convert") {
		t.Fatalf("converting a slice that already has a contract must say so: %v", err)
	}
}

// The real attack on "hash checked": an exam commit of the worker's own (tests/s01.sh = exit 0) with
// a hash computed over it, so hash and sha_examen agree. That commit is not on the base branch, so
// it is not adopted.
func TestConvertirRechazaExamenForjadoCoherente(t *testing.T) {
	r := repoDeJuguete(t)
	e := abrir(t, r)
	aLegado(t, r, e)
	commitEn(t, e.Worktree, "tests/s01.sh", "exit 0\n")
	falso, err := Git(e.Worktree, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	h, err := HuellaEnCommit(r.Root, falso, e.Bundle, e.Cmd)
	if err != nil {
		t.Fatal(err)
	}
	editarEstado(t, r, "01", func(m map[string]any) { m["sha_examen"], m["huella"] = falso, h })
	if _, err := r.Convertir("01", ratifico, codigoActual(t, r, "01"), io.Discard); err == nil || !strings.Contains(err.Error(), "base") {
		t.Fatalf("an exam commit off the base branch must not be adopted, even with a matching hash: %v", err)
	}
	sinContrato(t, r, "01")
}

// The red and exam commits must be full SHAs that name commits on the base branch: a ref name, a
// short SHA or a commit only on the worker's branch is refused, and nothing panics.
func TestConvertirExigeSHACompletoEnLaBase(t *testing.T) {
	r := repoDeJuguete(t)
	e := abrir(t, r)
	aLegado(t, r, e)
	commitEn(t, e.Worktree, "total.txt", "1\n")
	delWorker, _ := Git(e.Worktree, "rev-parse", "HEAD")
	casos := map[string][2]string{
		"red is a branch name":        {"slice/01", ""},
		"red is HEAD":                 {"HEAD", ""},
		"red is a short SHA":          {e.SHARojo[:6], ""},
		"red is a tree, not a commit": {mustGit(t, r.Root, "rev-parse", e.SHARojo+"^{tree}"), ""},
		"red only on the worker":      {delWorker, ""},
		"exam is a branch name":       {e.SHARojo, "slice/01"},
		"exam is a short SHA":         {e.SHARojo, e.SHARojo[:5]},
	}
	for nombre, c := range casos {
		editarEstado(t, r, "01", func(m map[string]any) {
			m["sha_rojo"] = c[0]
			if c[1] == "" {
				delete(m, "sha_examen")
			} else {
				m["sha_examen"] = c[1]
			}
		})
		if _, err := r.Convertir("01", ratifico, codigoActual(t, r, "01"), io.Discard); err == nil {
			t.Errorf("%s: adopted, must be refused", nombre)
		}
		sinContrato(t, r, "01")
	}
}

func mustGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := Git(dir, args...)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// Adoption refuses while a writer still holds the lease or a process works inside the worktree.
func TestConvertirSeNiegaConEscritorOProcesoDentro(t *testing.T) {
	r := repoDeJuguete(t)
	e := abrir(t, r)
	aLegado(t, r, e)
	editarEstado(t, r, "01", func(m map[string]any) { m["escritor"] = "worker-1" })
	if _, err := r.Convertir("01", ratifico, codigoActual(t, r, "01"), io.Discard); err == nil || !strings.Contains(err.Error(), "worker-1") {
		t.Fatalf("adoption with the writer lease held must refuse and name the writer: %v", err)
	}
	sinContrato(t, r, "01")
	editarEstado(t, r, "01", func(m map[string]any) { delete(m, "escritor") })
	antes := EnUso
	t.Cleanup(func() { EnUso = antes })
	EnUso = func(dir string) []string {
		if dir == e.Worktree {
			return []string{"claude (pid 4242)"}
		}
		return nil
	}
	if _, err := r.Convertir("01", ratifico, codigoActual(t, r, "01"), io.Discard); err == nil || !strings.Contains(err.Error(), "4242") {
		t.Fatalf("adoption with a process inside the worktree must refuse and name it: %v", err)
	}
	sinContrato(t, r, "01")
}

// orq never runs git with its cwd in a worker's copy: a clean filter the worker sets up in a 1.0.0
// linked worktree must not run when the slice is verified. Verify must reach the exam (a receipt is
// emitted), or the test proves nothing.
func TestEstadoViejoNoCorreGitEnElWorktree(t *testing.T) {
	r := repoDeJuguete(t)
	e := abrir(t, r)
	aLegado(t, r, e)
	if _, err := convertir(t, r, "01"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Ampliar("01", []string{".gitattributes"}); err != nil { // approved: verify must run
		t.Fatal(err)
	}
	centinela := filepath.Join(t.TempDir(), "filter-ran")
	if _, err := Git(r.Root, "config", "filter.x.clean", "touch "+centinela+"; cat"); err != nil {
		t.Fatal(err)
	}
	commitEn(t, e.Worktree, ".gitattributes", "*.txt filter=x\n")
	commitEn(t, e.Worktree, "total.txt", "170\n")
	os.Remove(centinela)
	time.Sleep(10 * time.Millisecond)
	os.WriteFile(filepath.Join(e.Worktree, "total.txt"), []byte("170\n"), 0o644) // new mtime: status would re-filter
	got, _, err := r.Verificar("", t0)
	if err != nil || got.Recibo == nil {
		t.Fatalf("verify must reach the exam: %+v %v", got, err)
	}
	if rec := leerJSONL[Recibo](r.recibosPath()); len(rec) != 1 {
		t.Fatalf("verify must emit one receipt: %+v", rec)
	}
	if _, err := os.Stat(centinela); err == nil {
		t.Fatal("verify ran git in the worker's worktree (its clean filter ran)")
	}
}

// The tip of a legacy slice must descend from its red commit, as a clone's does.
func TestEstadoViejoPuntaDesciendeDelRojo(t *testing.T) {
	r := repoDeJuguete(t)
	e := abrir(t, r)
	aLegado(t, r, e)
	if _, err := convertir(t, r, "01"); err != nil {
		t.Fatal(err)
	}
	otra, err := Git(r.Root, "commit-tree", "-m", "otra historia", e.SHARojo+"^{tree}")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Git(r.Root, "update-ref", "refs/heads/"+e.Rama, otra); err != nil {
		t.Fatal(err)
	}
	if _, _, err := r.Verificar("", t0); err == nil || !strings.Contains(err.Error(), "does not descend") {
		t.Fatalf("a tip that does not descend from the red commit must be refused: %v", err)
	}
}

// The base is never taken from the state: the reviewer's bypass (state base = the slice branch, an
// exam commit of the worker's own on it, a recomputed hash) and an alias branch made from the worktree
// are refused, naming both the state's base and the trusted one.
func TestConvertirBaseNoSaleDelEstado(t *testing.T) {
	r := repoDeJuguete(t)
	e := abrir(t, r)
	aLegado(t, r, e)
	commitEn(t, e.Worktree, "tests/s01.sh", "exit 0\n")
	falso := mustGit(t, e.Worktree, "rev-parse", "HEAD")
	h, err := HuellaEnCommit(r.Root, falso, e.Bundle, e.Cmd)
	if err != nil {
		t.Fatal(err)
	}
	mustGit(t, e.Worktree, "branch", "x", falso) // refs are shared with the main repo
	for _, base := range []string{e.Rama, "x"} {
		editarEstado(t, r, "01", func(m map[string]any) { m["base"], m["sha_examen"], m["huella"] = base, falso, h })
		_, err := r.Convertir("01", ratifico, codigoActual(t, r, "01"), io.Discard)
		if err == nil || !strings.Contains(err.Error(), `"`+base+`"`) || !strings.Contains(err.Error(), `"main"`) {
			t.Fatalf("state base %q: must be refused naming it and the trusted base main: %v", base, err)
		}
		sinContrato(t, r, "01")
	}
}

// The trusted base is the project config's `base` when it defines one, else main, else master; a
// slice/* branch or the slice's own branch is never trusted.
func TestBaseConfiable(t *testing.T) {
	r := repoDeJuguete(t)
	e := abrir(t, r)
	if b, err := r.baseConfiable(e); err != nil || b != "main" {
		t.Fatalf("default: %q %v", b, err)
	}
	mustGit(t, r.Root, "branch", "-m", "main", "master")
	if b, err := r.baseConfiable(e); err != nil || b != "master" {
		t.Fatalf("no main: %q %v", b, err)
	}
	mustGit(t, r.Root, "branch", "desarrollo")
	for cfg, ok := range map[string]bool{"desarrollo": true, "slice/09": false, e.Rama: false, "no-existe": false} {
		os.WriteFile(filepath.Join(r.Root, ".orq", "config.json"), []byte(`{"base":"`+cfg+`"}`), 0o644)
		b, err := r.baseConfiable(e)
		if ok != (err == nil) || ok && b != cfg {
			t.Errorf("config base %q: %q %v", cfg, b, err)
		}
	}
}

// codigoActual is the code a dry run shows now ("" if the dry run refuses).
func codigoActual(t *testing.T, r Repo, nn string) string {
	t.Helper()
	var out bytes.Buffer
	if _, err := r.Convertir(nn, "", "", &out); !errors.Is(err, ErrEnSeco) {
		return ""
	}
	_, c, _ := strings.Cut(out.String(), "code: ")
	return strings.TrimSpace(c)
}

func convertir(t *testing.T, r Repo, nn string) (*Estado, error) {
	t.Helper()
	return r.Convertir(nn, ratifico, codigoActual(t, r, nn), io.Discard)
}

// Without --ratifico convertir is a dry run: it shows cmd, bundle, red and exam commits, the trusted
// base and a code over them, seals nothing and fails. Sealing needs that code, recomputed at seal
// time: a wrong code, or a code shown before the state changed, is refused.
func TestConvertirEnSecoYCodigo(t *testing.T) {
	r := repoDeJuguete(t)
	e := abrir(t, r)
	aLegado(t, r, e)
	var out bytes.Buffer
	if _, err := r.Convertir("01", "", "", &out); !errors.Is(err, ErrEnSeco) {
		t.Fatalf("a dry run must fail with ErrEnSeco: %v", err)
	}
	sinContrato(t, r, "01")
	cod := codigoActual(t, r, "01")
	if len(cod) != 12 || strings.Trim(cod, "0123456789abcdef") != "" {
		t.Fatalf("code must be 12 hex: %q\n%s", cod, out.String())
	}
	for _, want := range []string{e.Cmd, "tests/s01.sh", "tests/datos.txt", e.SHARojo, "base: main", cod} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("dry run must show %q; it showed:\n%s", want, out.String())
		}
	}
	for _, mal := range []string{"", "000000000000", cod[:6]} {
		if _, err := r.Convertir("01", ratifico, mal, io.Discard); err == nil || !strings.Contains(err.Error(), "code") {
			t.Fatalf("code %q must be refused: %v", mal, err)
		}
		sinContrato(t, r, "01")
	}
	editarEstado(t, r, "01", func(m map[string]any) { m["sha_examen"] = e.SHARojo }) // valid, but not what was shown
	if _, err := r.Convertir("01", ratifico, cod, io.Discard); err == nil || !strings.Contains(err.Error(), "code") {
		t.Fatalf("a code shown before the state changed must be refused: %v", err)
	}
	sinContrato(t, r, "01")
	if _, err := r.Convertir("01", ratifico, codigoActual(t, r, "01"), io.Discard); err != nil {
		t.Fatal(err)
	}
}

// The in-use check looks at the worktree git reports for the slice branch, not the state's
// `worktree` field: a live worker that forges that field is still seen.
func TestConvertirEnUsoMiraElWorktreeDeGit(t *testing.T) {
	r := repoDeJuguete(t)
	e := abrir(t, r)
	aLegado(t, r, e)
	editarEstado(t, r, "01", func(m map[string]any) { m["worktree"] = filepath.Join(t.TempDir(), "x") })
	antes := EnUso
	t.Cleanup(func() { EnUso = antes })
	EnUso = func(dir string) []string {
		if dir == e.Worktree {
			return []string{"claude (pid 4242)"}
		}
		return nil
	}
	if _, err := r.Convertir("01", ratifico, codigoActual(t, r, "01"), io.Discard); err == nil || !strings.Contains(err.Error(), "4242") {
		t.Fatalf("a process in the worktree git reports must refuse, whatever the state says: %v", err)
	}
	sinContrato(t, r, "01")
}
