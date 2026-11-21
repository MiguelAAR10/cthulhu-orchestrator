package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"orq/internal/gitx"
	"orq/internal/orqdir"
	"orq/internal/slice"
)

// The mechanics behind `orq2 update`: gate records, the busy-slice check, the clean build and
// the atomic swap. The cli decides who may run it and what it prints.
//
// Gate records are git notes on refs/notes/orq2-gate in the source repo, one per commit, JSON
// with a schema number. A note hangs off one exact commit, so a record for a parent can never
// count for its child; writing one does not move harness-v2 (a record committed into the tree
// would itself create a new, ungated tip); and the notes ref keeps its own history, so every
// record is versioned and auditable with `git log -p refs/notes/orq2-gate`.

// RefPuerta is the git notes ref that holds the gate records (refs/notes/orq2-gate).
const RefPuerta = "orq2-gate"

// EsquemaPuerta is the version of the gate record format.
const EsquemaPuerta = 1

// VeredictoSinHigh is the only review verdict that lets a SHA through.
const VeredictoSinHigh = "sin-high"

// NombreRollback is the fixed name of the previous binary inside the rollback dir. Other files
// there (the integrator's orq2-24f2438) are never touched.
const NombreRollback = "orq2-anterior"

// ConfUpdate holds every path update uses; each one is overridable by env, so tests never touch
// the real ~/bin, ~/.local/share or the main checkout.
type ConfUpdate struct {
	Repo, Rama, Bin, Rollback, Go, Proc, Worktrees string
}

func envO(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// LeerConfUpdate: ORQ_UPDATE_REPO (the Cthulhu main checkout), ORQ_UPDATE_RAMA (harness-v2),
// ORQ_UPDATE_BIN (~/bin), ORQ_UPDATE_ROLLBACK (~/.local/share/orq2-rollback), ORQ_UPDATE_GO (go),
// ORQ_UPDATE_PROC (/proc) and ORQ_WORKTREES (~/worktrees).
func LeerConfUpdate() ConfUpdate {
	home, _ := os.UserHomeDir()
	return ConfUpdate{
		Repo:      envO("ORQ_UPDATE_REPO", filepath.Join(home, "projects", "MiguelAR10-Builder", "AI-Edge-Runners", "experiments", "cthulhu-orchestrator")),
		Rama:      envO("ORQ_UPDATE_RAMA", "harness-v2"),
		Bin:       envO("ORQ_UPDATE_BIN", filepath.Join(home, "bin")),
		Rollback:  envO("ORQ_UPDATE_ROLLBACK", filepath.Join(home, ".local", "share", "orq2-rollback")),
		Go:        envO("ORQ_UPDATE_GO", "go"),
		Proc:      envO("ORQ_UPDATE_PROC", "/proc"),
		Worktrees: envO("ORQ_WORKTREES", filepath.Join(home, "worktrees")),
	}
}

func (c ConfUpdate) Instalado() string    { return filepath.Join(c.Bin, "orq2") }
func (c ConfUpdate) RutaRollback() string { return filepath.Join(c.Rollback, NombreRollback) }

// RegistroPuerta is one gate record: for one exact SHA, the independent review verdict and who
// gave it, the `make ci` exit and where its log is, and when it was recorded.
type RegistroPuerta struct {
	Esquema  int    `json:"esquema"`
	SHA      string `json:"sha"`
	Revision struct {
		Veredicto string `json:"veredicto"`
		Revisor   string `json:"revisor"`
	} `json:"revision"`
	CI struct {
		Comando string `json:"comando"`
		Exit    *int   `json:"exit"`
		Log     string `json:"log"`
	} `json:"ci"`
	Fecha string `json:"fecha"`
}

// Validar says why r does not let sha through, or nil.
func (r RegistroPuerta) Validar(sha string) error {
	switch {
	case r.Esquema != EsquemaPuerta:
		return fmt.Errorf("esquema %d, se espera %d", r.Esquema, EsquemaPuerta)
	case r.SHA != sha:
		return fmt.Errorf("el registro es de %s, no de %s", r.SHA, sha)
	case r.Revision.Veredicto != VeredictoSinHigh:
		return fmt.Errorf("veredicto de revisión %q, se exige %q", r.Revision.Veredicto, VeredictoSinHigh)
	case strings.TrimSpace(r.Revision.Revisor) == "":
		return errors.New("no dice quién revisó")
	case r.CI.Exit == nil || *r.CI.Exit != 0:
		return errors.New("make ci no terminó con exit 0")
	case strings.TrimSpace(r.CI.Log) == "":
		return errors.New("no dice dónde está el log de make ci")
	}
	if _, err := time.Parse(time.RFC3339, r.Fecha); err != nil {
		return fmt.Errorf("fecha ilegible %q", r.Fecha)
	}
	return nil
}

// LeerPuerta reads and validates the gate record of sha. Read-only.
func (c ConfUpdate) LeerPuerta(sha string) (RegistroPuerta, error) {
	var r RegistroPuerta
	raw, err := gitx.Principal(context.Background(), c.Repo, "notes", "--ref="+RefPuerta, "show", sha)
	if err != nil {
		return r, errors.New("no hay registro")
	}
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		return r, fmt.Errorf("registro ilegible: %v", err)
	}
	return r, r.Validar(sha)
}

var shaCompleto = regexp.MustCompile(`^[0-9a-f]{40}$`)

// Commit resolves ref to a full commit SHA of the source repo.
func (c ConfUpdate) Commit(ref string) (string, error) {
	sha, err := gitx.Principal(context.Background(), c.Repo, "rev-parse", "--verify", "--quiet", ref+"^{commit}")
	if err != nil || !shaCompleto.MatchString(sha) {
		return "", fmt.Errorf("%s no es un commit de %s", ref, c.Repo)
	}
	return sha, nil
}

// Punta is the tip of the source branch.
func (c ConfUpdate) Punta() (string, error) { return c.Commit("refs/heads/" + c.Rama) }

// RegistrarPuerta writes a passing gate record for ref and returns its full SHA. It never
// overwrites an existing record.
func (c ConfUpdate) RegistrarPuerta(ref, revisor, ciLog string, now time.Time) (string, error) {
	sha, err := c.Commit(ref)
	if err != nil {
		return "", err
	}
	var r RegistroPuerta
	r.Esquema, r.SHA, r.Fecha = EsquemaPuerta, sha, now.UTC().Format(time.RFC3339)
	r.Revision.Veredicto, r.Revision.Revisor = VeredictoSinHigh, revisor
	cero := 0
	r.CI.Comando, r.CI.Exit, r.CI.Log = "make ci", &cero, ciLog
	raw, _ := json.MarshalIndent(r, "", "  ")
	if _, err := gitx.Principal(context.Background(), c.Repo, "notes", "--ref="+RefPuerta, "add", "-m", string(raw), sha); err != nil {
		return sha, fmt.Errorf("%v. Si ya existe y hay que corregirlo: git notes --ref=%s remove %s", err, RefPuerta, sha)
	}
	return sha, nil
}

// Revision is what a binary says about itself in `version`.
type Revision struct {
	Rev, Mod, Linea string
	Existe          bool
}

// Limpia says whether r is exactly sha, built from a clean tree.
func (r Revision) Limpia(sha string) bool { return r.Rev == sha && r.Mod == "false" }

// RevisionDe runs bin version. Existe=false if bin does not exist; Rev is "desconocida" if it
// runs but says nothing usable.
func RevisionDe(bin string) Revision {
	if _, err := os.Stat(bin); err != nil {
		return Revision{Rev: "ninguna", Linea: "ninguna"}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "version")
	cmd.Env = append(os.Environ(), "ORQ_NOMBRE=orq2")
	out, _ := cmd.Output()
	r := Revision{Rev: "desconocida", Mod: "desconocido", Linea: strings.TrimSpace(string(out)), Existe: true}
	for _, campo := range strings.Fields(r.Linea) {
		if v, ok := strings.CutPrefix(campo, "revision="); ok {
			r.Rev = v
		}
		if v, ok := strings.CutPrefix(campo, "modified="); ok {
			r.Mod = v
		}
	}
	if r.Linea == "" {
		r.Linea = "desconocida"
	}
	return r
}

// Ocupacion is what is being verified or closed right now in every project update can see.
type Ocupacion struct {
	Vistos  []string // project roots checked
	Motivos []string // one line per busy thing
}

// FuentesProyectos says where update looks for projects.
const FuentesProyectos = "ORQ_UPDATE_PROYECTOS (rutas separadas por :), ORQ_ROOT, la carpeta actual, las marcas de los clones de worker en ORQ_WORKTREES y la carpeta de cada proceso orq/orq2 que corre"

func (o Ocupacion) Proyectos() string {
	if len(o.Vistos) == 0 {
		return "ninguno"
	}
	return strings.Join(o.Vistos, ", ")
}

var opsOcupadas = []string{"verificar", "cerrar", "juntar"}

// RevisarOcupados looks at running orq/orq2 processes that verify or close, and at every known
// project's slices (INTEGRANDO/CERRANDO) and slice locks. Unreadable state counts as busy. It
// writes nothing.
func (c ConfUpdate) RevisarOcupados() Ocupacion {
	var o Ocupacion
	candidatos := strings.Split(os.Getenv("ORQ_UPDATE_PROYECTOS"), ":")
	candidatos = append(candidatos, os.Getenv("ORQ_ROOT"))
	if cwd, err := os.Getwd(); err == nil {
		candidatos = append(candidatos, cwd)
	}
	marcas, _ := filepath.Glob(filepath.Join(c.Worktrees, "*", ".git", orqdir.MarcaClon))
	for _, m := range marcas {
		if mc, ok := slice.LeerMarca(filepath.Dir(filepath.Dir(m))); ok {
			candidatos = append(candidatos, mc.Root)
		}
	}
	// Processes: the exam of `slice verificar` runs for minutes without holding any lock.
	yo := strconv.Itoa(os.Getpid())
	pids, _ := os.ReadDir(c.Proc)
	for _, d := range pids {
		if _, err := strconv.Atoi(d.Name()); err != nil || d.Name() == yo {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(c.Proc, d.Name(), "cmdline"))
		if err != nil {
			continue
		}
		argv := strings.Split(strings.TrimRight(string(raw), "\x00"), "\x00")
		if len(argv) < 3 || !slices.Contains([]string{"orq", "orq2"}, filepath.Base(argv[0])) {
			continue
		}
		cwd, _ := os.Readlink(filepath.Join(c.Proc, d.Name(), "cwd"))
		if ocupa(argv[1:]) {
			o.Motivos = append(o.Motivos, fmt.Sprintf("el proceso %s corre `%s` en %s", d.Name(), strings.Join(argv, " "), cwd))
		}
		candidatos = append(candidatos, cwd)
	}
	for _, cand := range candidatos {
		root := raizConOrq(cand)
		if root == "" || slices.Contains(o.Vistos, root) {
			continue
		}
		o.Vistos = append(o.Vistos, root)
		o.Motivos = append(o.Motivos, ocupadoEn(root)...)
	}
	slices.Sort(o.Vistos)
	return o
}

// ocupa says whether args (after the binary) verify or close a slice.
func ocupa(args []string) bool {
	switch args[0] {
	case "slice":
		return slices.ContainsFunc(args[1:], func(a string) bool { return slices.Contains(opsOcupadas, a) })
	case "sesion":
		return slices.Contains(args[1:], "cerrar")
	}
	return false
}

// raizConOrq is the nearest directory at or above dir with a .orq directory, or "".
func raizConOrq(dir string) string {
	if dir == "" || !filepath.IsAbs(dir) {
		return ""
	}
	for d := filepath.Clean(dir); ; d = filepath.Dir(d) {
		if fi, err := os.Stat(filepath.Join(d, ".orq")); err == nil && fi.IsDir() {
			return d
		}
		if filepath.Dir(d) == d {
			return ""
		}
	}
}

// ocupadoEn reads root's slice states and locks without writing anything (no migration, no
// lock files created).
func ocupadoEn(root string) []string {
	var out []string
	od := filepath.Join(root, ".orq")
	estados, _ := filepath.Glob(filepath.Join(od, "slices", "*.json"))
	if _, err := os.Stat(filepath.Join(od, "slice.json")); err == nil {
		estados = append(estados, filepath.Join(od, "slice.json"))
	}
	slices.Sort(estados)
	for _, f := range estados {
		var e struct {
			NN   string `json:"nn"`
			Fase string `json:"fase"`
		}
		raw, err := os.ReadFile(f)
		if err == nil {
			err = json.Unmarshal(raw, &e)
		}
		if err != nil {
			out = append(out, fmt.Sprintf("%s: no se pudo leer %s (%v); por seguridad cuenta como ocupado", root, f, err))
			continue
		}
		if e.Fase == slice.Integrando || e.Fase == slice.Cerrando {
			out = append(out, fmt.Sprintf("%s: S-%s está en %s", root, e.NN, e.Fase))
		}
	}
	locks, _ := filepath.Glob(filepath.Join(od, "slice-*.lock"))
	locks = append(locks, filepath.Join(od, "integracion.lock"))
	for _, l := range locks {
		f, err := os.Open(l)
		if err != nil {
			continue
		}
		if err := syscall.Flock(int(f.Fd()), syscall.LOCK_SH|syscall.LOCK_NB); err != nil {
			out = append(out, fmt.Sprintf("%s: %s está tomado (un comando de slice corre ahora)", root, filepath.Join(".orq", filepath.Base(l))))
		} else {
			syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		}
		f.Close()
	}
	return out
}

// CopiarAtomico copies src over dst through a temp file in dst's directory and a rename, so dst
// is always either the old file or the new one, never half written. Mode and mtime follow src.
func CopiarAtomico(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	fi, err := in.Stat()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(dst), "."+filepath.Base(dst)+".nuevo-*")
	if err != nil {
		return err
	}
	limpiar := func(e error) error { tmp.Close(); os.Remove(tmp.Name()); return e }
	if _, err := io.Copy(tmp, in); err != nil {
		return limpiar(err)
	}
	if err := tmp.Chmod(fi.Mode().Perm()); err != nil {
		return limpiar(err)
	}
	if err := tmp.Sync(); err != nil {
		return limpiar(err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	os.Chtimes(tmp.Name(), fi.ModTime(), fi.ModTime())
	if err := os.Rename(tmp.Name(), dst); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	if d, err := os.Open(filepath.Dir(dst)); err == nil {
		d.Sync()
		d.Close()
	}
	return nil
}

// ConstruirCandidato builds ./cmd/orq of sha the way make install2 does (go build -trimpath), in
// a throwaway clone under dir checked out at exactly sha, so the binary says revision=sha
// modified=false. It never builds from the main checkout, which may be dirty, and downloads
// nothing (GOTOOLCHAIN=local, GOPROXY=off).
func (c ConfUpdate) ConstruirCandidato(sha, dir string) (string, error) {
	src := filepath.Join(dir, "src")
	ctx := context.Background()
	if out, err := gitx.Principal(ctx, c.Repo, "clone", "-q", "--shared", "--no-checkout", c.Repo, src); err != nil {
		return "", fmt.Errorf("clon temporal: %v %s", err, out)
	}
	if out, err := gitx.Principal(ctx, src, "checkout", "-q", "--detach", sha); err != nil {
		return "", fmt.Errorf("checkout de %s: %v %s", sha, err, out)
	}
	bin := filepath.Join(dir, "orq2")
	ctxB, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctxB, c.Go, "build", "-trimpath", "-o", bin, "./cmd/orq")
	cmd.Dir = filepath.Join(src, "harness")
	cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOPROXY=off", "GOFLAGS=-mod=readonly -buildvcs=true")
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("go build: %v\n%s", err, out)
	}
	if r := RevisionDe(bin); !r.Limpia(sha) {
		return "", fmt.Errorf("el candidato dice %q; se esperaba revision=%s modified=false", r.Linea, sha)
	}
	return bin, nil
}

// MasNuevaConPuerta is the newest commit after the installed revision and before the tip with
// a valid gate record, or "". Read-only.
func (c ConfUpdate) MasNuevaConPuerta(punta, instalada string) string {
	ctx := context.Background()
	lista, err := gitx.Principal(ctx, c.Repo, "notes", "--ref="+RefPuerta, "list")
	if err != nil || lista == "" {
		return ""
	}
	con := map[string]bool{}
	for _, l := range strings.Split(lista, "\n") {
		if f := strings.Fields(l); len(f) == 2 {
			con[f[1]] = true
		}
	}
	args := []string{"rev-list", "--max-count=1000", punta}
	if shaCompleto.MatchString(instalada) {
		if _, err := c.Commit(instalada); err == nil {
			args = append(args, "^"+instalada)
		}
	}
	revs, err := gitx.Principal(ctx, c.Repo, args...)
	if err != nil {
		return ""
	}
	for _, r := range strings.Fields(revs) {
		if r != punta && con[r] {
			if _, err := c.LeerPuerta(r); err == nil {
				return r
			}
		}
	}
	return ""
}
