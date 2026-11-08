package worktrees

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"orq/internal/sysinfo"
)

func tgit(t *testing.T, dir string, a ...string) string {
	t.Helper()
	cmd := exec.Command("git", a...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %s", a, out)
	}
	return strings.TrimSpace(string(out))
}

type fixture struct {
	root, orq, base string
}

func nuevo(t *testing.T) fixture {
	t.Helper()
	d := t.TempDir()
	root := filepath.Join(d, "repo")
	os.MkdirAll(root, 0o755)
	tgit(t, root, "init", "-q", "-b", "main")
	tgit(t, root, "config", "user.email", "t@t")
	tgit(t, root, "config", "user.name", "t")
	os.WriteFile(filepath.Join(root, ".gitignore"), []byte("evidence/\nnode_modules/\n.env\n"), 0o644)
	os.WriteFile(filepath.Join(root, "a.txt"), []byte("a"), 0o644)
	tgit(t, root, "add", ".")
	tgit(t, root, "commit", "-qm", "base")
	return fixture{root, filepath.Join(root, ".orq"), filepath.Join(d, "wt")}
}

func (f fixture) crear(t *testing.T, nombre string, args ...string) string {
	t.Helper()
	p := filepath.Join(f.base, nombre)
	if err := Crear(f.root, f.orq, Entrada{Path: p, Kind: KindSlice, Slice: "S-01", OwnerRole: "orquestador"}, args...); err != nil {
		t.Fatal(err)
	}
	return p
}

func (f fixture) gc(t *testing.T, procs []sysinfo.Proc) []Resultado {
	t.Helper()
	res, err := GC(f.orq, Opciones{Now: time.Now().Add(100 * time.Hour), Procs: func() ([]sysinfo.Proc, error) { return procs, nil }})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func uno(t *testing.T, res []Resultado, path string) Resultado {
	t.Helper()
	for _, r := range res {
		if r.Path == path {
			return r
		}
	}
	t.Fatalf("sin resultado para %s: %+v", path, res)
	return Resultado{}
}

func TestCrearRegistraYCuenta(t *testing.T) {
	f := nuevo(t)
	p := f.crear(t, "s01", "-b", "slice/01")
	es, _ := Leer(f.orq)
	if len(es) != 1 || es[0].Path != p || es[0].Repo != f.root || es[0].Kind != KindSlice || es[0].TTL == "" || es[0].Created.IsZero() {
		t.Fatalf("%+v", es)
	}
	if n, _ := Contar(f.orq); n != 1 {
		t.Fatal(n)
	}
	if err := Crear(f.root, f.orq, Entrada{Path: filepath.Join(f.base, "x"), Kind: "raro"}); err == nil {
		t.Fatal("aceptó kind inválido")
	}
}

func TestSucioSeConserva(t *testing.T) {
	f := nuevo(t)
	p := f.crear(t, "s01", "-b", "slice/01")
	os.WriteFile(filepath.Join(p, "nuevo.txt"), []byte("x"), 0o644)
	if r := uno(t, f.gc(t, nil), p); r.Accion != Conservado || !strings.Contains(r.Motivo, "sin commit") {
		t.Fatalf("%+v", r)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatal("borró un worktree sucio")
	}
}

func TestActividadRecienteSeConserva(t *testing.T) {
	f := nuevo(t)
	p := f.crear(t, "s01", "-b", "slice/01")
	res, _ := GC(f.orq, Opciones{Now: time.Now(), Procs: func() ([]sysinfo.Proc, error) { return nil, nil }})
	if r := uno(t, res, p); r.Accion != Conservado || !strings.Contains(r.Motivo, "actividad") {
		t.Fatalf("%+v", r)
	}
}

func TestPunteroGitNoCuentaComoActividad(t *testing.T) {
	f := nuevo(t)
	p := f.crear(t, "s01", "-b", "slice/01")
	viejo := time.Now().Add(-200 * time.Hour)
	filepath.Walk(p, func(x string, _ os.FileInfo, _ error) error { os.Chtimes(x, viejo, viejo); return nil })
	os.Chtimes(filepath.Join(p, ".git"), time.Now(), time.Now())
	if m := ultimaActividad(p); m.After(viejo.Add(time.Hour)) {
		t.Fatalf("el puntero .git contó como actividad: %v", m)
	}
}

func TestEnRamaLimpioSeRetira(t *testing.T) {
	f := nuevo(t)
	p := f.crear(t, "s01", "-b", "slice/01")
	if r := uno(t, f.gc(t, nil), p); r.Accion != Retirado {
		t.Fatalf("%+v", r)
	}
	if _, err := os.Stat(p); err == nil {
		t.Fatal("sigue ahí")
	}
	if b := tgit(t, f.root, "branch", "--list", "slice/01"); b == "" {
		t.Fatal("borró la rama")
	}
	if es, _ := Leer(f.orq); len(es) != 0 {
		t.Fatalf("registro no podado: %+v", es)
	}
}

func TestDryRunNoToca(t *testing.T) {
	f := nuevo(t)
	p := f.crear(t, "s01", "-b", "slice/01")
	res, _ := GC(f.orq, Opciones{DryRun: true, Now: time.Now().Add(100 * time.Hour), Procs: func() ([]sysinfo.Proc, error) { return nil, nil }})
	if r := uno(t, res, p); r.Accion != Retiraria {
		t.Fatalf("%+v", r)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatal("dry-run borró")
	}
}

func TestHeadSinRefSeEmpaquetaYSeRetira(t *testing.T) {
	f := nuevo(t)
	p := f.crear(t, "int", "--detach", "HEAD")
	os.WriteFile(filepath.Join(p, "b.txt"), []byte("b"), 0o644)
	tgit(t, p, "add", ".")
	tgit(t, p, "commit", "-qm", "huérfano")
	head := tgit(t, p, "rev-parse", "HEAD")
	r := uno(t, f.gc(t, nil), p)
	if r.Accion != Retirado || r.Bundle == "" {
		t.Fatalf("%+v", r)
	}
	if !strings.HasPrefix(r.Bundle, filepath.Join(f.orq, "bundles")) {
		t.Fatal(r.Bundle)
	}
	out := tgit(t, f.root, "bundle", "list-heads", r.Bundle)
	if !strings.Contains(out, head) {
		t.Fatalf("bundle sin el HEAD: %s", out)
	}
}

func TestIgnoradoUnicoSeRespaldaYSeRetira(t *testing.T) {
	f := nuevo(t)
	p := f.crear(t, "s01", "-b", "slice/01")
	os.MkdirAll(filepath.Join(p, "evidence"), 0o755)
	os.WriteFile(filepath.Join(p, "evidence", "run.log"), []byte("prueba única"), 0o644)
	os.MkdirAll(filepath.Join(p, "node_modules", "x"), 0o755)
	os.WriteFile(filepath.Join(p, "node_modules", "x", "i.js"), []byte("dep"), 0o644)
	r := uno(t, f.gc(t, nil), p)
	if r.Accion != Retirado || r.Backup == "" {
		t.Fatalf("%+v", r)
	}
	fh, err := os.Open(r.Backup)
	if err != nil {
		t.Fatal(err)
	}
	gz, _ := gzip.NewReader(fh)
	tr := tar.NewReader(gz)
	var nombres []string
	for {
		h, err := tr.Next()
		if err != nil {
			break
		}
		nombres = append(nombres, h.Name)
	}
	if strings.Join(nombres, ",") != "evidence/run.log" {
		t.Fatalf("contenido del respaldo: %v", nombres)
	}
}

func TestSecretoIgnoradoSeConserva(t *testing.T) {
	f := nuevo(t)
	p := f.crear(t, "s01", "-b", "slice/01")
	os.WriteFile(filepath.Join(p, ".env"), []byte("API_KEY=abcdefghijklmnopqrstuvwxyz"), 0o644)
	if r := uno(t, f.gc(t, nil), p); r.Accion != Conservado || !strings.Contains(r.Motivo, "secreto") {
		t.Fatalf("%+v", r)
	}
	if _, err := os.Stat(filepath.Join(p, ".env")); err != nil {
		t.Fatal("perdió el secreto")
	}
	os.Remove(filepath.Join(p, ".env"))
	os.MkdirAll(filepath.Join(p, "evidence"), 0o755)
	os.WriteFile(filepath.Join(p, "evidence", "out.txt"), []byte("token = ghp_abcdefghijklmnopqrstuvwxyz0123456789"), 0o644)
	if r := uno(t, f.gc(t, nil), p); r.Accion != Conservado || !strings.Contains(r.Motivo, "secreto") {
		t.Fatalf("contenido sensible: %+v", r)
	}
}

func TestProcesoAdentroSeConserva(t *testing.T) {
	f := nuevo(t)
	p := f.crear(t, "s01", "-b", "slice/01")
	r := uno(t, f.gc(t, []sysinfo.Proc{{PID: 999999, Comm: "claude", Cwd: filepath.Join(p, "sub")}}), p)
	if r.Accion != Conservado || !strings.Contains(r.Motivo, "claude") {
		t.Fatalf("%+v", r)
	}
}

func TestSinPunteroGitVaACuarentenaSinForzar(t *testing.T) {
	f := nuevo(t)
	p := f.crear(t, "s01", "-b", "slice/01")
	os.Remove(filepath.Join(p, ".git"))
	r := uno(t, f.gc(t, nil), p)
	if r.Accion != Cuarentena {
		t.Fatalf("%+v", r)
	}
	if _, err := os.Stat(filepath.Join(p, "a.txt")); err != nil {
		t.Fatal("borró archivos de un worktree roto")
	}
	raw, _ := os.ReadFile(filepath.Join(f.orq, "cuarentena.jsonl"))
	if !strings.Contains(string(raw), p) {
		t.Fatalf("sin entrada de cuarentena: %s", raw)
	}
	if es, _ := Leer(f.orq); len(es) != 1 {
		t.Fatal("quitó del registro algo que sigue en disco")
	}
}

func TestRutaInexistenteSePodaDelRegistro(t *testing.T) {
	f := nuevo(t)
	p := f.crear(t, "s01", "-b", "slice/01")
	tgit(t, f.root, "worktree", "remove", p)
	if r := uno(t, f.gc(t, nil), p); r.Accion != Podado {
		t.Fatalf("%+v", r)
	}
	if es, _ := Leer(f.orq); len(es) != 0 {
		t.Fatal(es)
	}
}

func TestProtegidoSeConserva(t *testing.T) {
	f := nuevo(t)
	p := f.crear(t, "s01", "-b", "slice/01")
	res, _ := GC(f.orq, Opciones{Now: time.Now().Add(100 * time.Hour), Procs: func() ([]sysinfo.Proc, error) { return nil, nil },
		Protegido: func(x string) bool { return x == p }})
	if r := uno(t, res, p); r.Accion != Conservado {
		t.Fatalf("%+v", r)
	}
}
