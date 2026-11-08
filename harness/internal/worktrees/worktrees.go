// Package worktrees registra cada worktree que orq crea (.orq/worktrees.jsonl) y los retira
// sin perder nada (GC). Es el único lugar donde orq corre `git worktree add`.
//
// El protocolo de GC es el de la limpieza manual que liberó 43 GB (~/worktree-cleanup-audit/limpiar.py):
// nunca --force, nada con procesos adentro, nada sucio, nada activo, HEAD en ref duradera o en un
// bundle verificado, ignorados únicos respaldados y verificados, secretos ⇒ se conserva y se avisa.
package worktrees

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"orq/internal/sysinfo"
)

const (
	KindSlice = "slice"
	KindZone  = "zone"
	KindInt   = "int"
	KindScout = "scout"
	KindE2E   = "e2e"
)

// TTLDefecto: sin actividad durante esto, un worktree limpio y respaldado se puede retirar.
const TTLDefecto = 72 * time.Hour

type Entrada struct {
	Path      string    `json:"path"`
	Repo      string    `json:"repo"`
	OwnerRole string    `json:"owner_role"`
	Slice     string    `json:"slice"`
	Kind      string    `json:"kind"`
	Created   time.Time `json:"created"`
	TTL       string    `json:"ttl"`
}

func (e Entrada) ttl() time.Duration {
	if d, err := time.ParseDuration(e.TTL); err == nil && d > 0 {
		return d
	}
	return TTLDefecto
}

func registro(orqDir string) string { return filepath.Join(orqDir, "worktrees.jsonl") }

// Crear corre `git worktree add <path> <args...>` desde repo y, si git lo creó, lo registra.
func Crear(repo, orqDir string, e Entrada, args ...string) error {
	if !slices.Contains([]string{KindSlice, KindZone, KindInt, KindScout, KindE2E}, e.Kind) {
		return fmt.Errorf("worktree: kind %q no es slice|zone|int|scout|e2e", e.Kind)
	}
	if e.Path == "" {
		return errors.New("worktree: falta la ruta")
	}
	if _, err := git(repo, append([]string{"worktree", "add", e.Path}, args...)...); err != nil {
		return err
	}
	e.Repo = repo
	if e.OwnerRole == "" {
		e.OwnerRole = os.Getenv("ORQ_ROL")
	}
	if e.Created.IsZero() {
		e.Created = time.Now().UTC().Truncate(time.Second)
	}
	if e.TTL == "" {
		e.TTL = TTLDefecto.String()
	}
	if err := os.MkdirAll(orqDir, 0o755); err != nil {
		return err
	}
	soltar, err := candado(orqDir)
	if err != nil {
		return err
	}
	defer soltar()
	line, _ := json.Marshal(e)
	f, err := os.OpenFile(registro(orqDir), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(line, '\n'))
	return err
}

// Leer devuelve el registro (vacío si no existe).
func Leer(orqDir string) ([]Entrada, error) {
	f, err := os.Open(registro(orqDir))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []Entrada
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		var e Entrada
		if json.Unmarshal(sc.Bytes(), &e) == nil && e.Path != "" {
			out = append(out, e)
		}
	}
	return out, sc.Err()
}

// Contar: worktrees registrados que siguen en disco (lo que cuenta para max_worktrees).
func Contar(orqDir string) (int, error) {
	es, err := Leer(orqDir)
	n := 0
	for _, e := range es {
		if existe(e.Path) {
			n++
		}
	}
	return n, err
}

const (
	Retirado   = "retirado"
	Retiraria  = "retiraría"
	Conservado = "conservado"
	Cuarentena = "cuarentena"
	Podado     = "podado"
)

type Resultado struct {
	Path, Accion, Motivo string
	Bundle, Backup       string
}

type Opciones struct {
	DryRun    bool
	Now       time.Time
	Procs     func() ([]sysinfo.Proc, error) // nil = /proc real
	Protegido func(path string) bool         // worktrees de slices abiertos: no se tocan
}

// GC revisa cada entrada del registro y retira solo lo que es seguro retirar.
func GC(orqDir string, o Opciones) ([]Resultado, error) {
	if o.Now.IsZero() {
		o.Now = time.Now()
	}
	if o.Procs == nil {
		o.Procs = sysinfo.ListProcs
	}
	es, err := Leer(orqDir)
	if err != nil {
		return nil, err
	}
	procs, err := o.Procs()
	if err != nil {
		return nil, fmt.Errorf("gc: no pude listar procesos (%v); sin dato no se retira nada", err)
	}
	var out []Resultado
	quitar := map[string]bool{}
	for _, e := range es {
		r := revisar(orqDir, e, procs, o)
		out = append(out, r)
		if !o.DryRun && (r.Accion == Retirado || r.Accion == Podado) {
			quitar[e.Path] = true
		}
		if !o.DryRun && r.Accion == Cuarentena {
			cuarentena(orqDir, e, r.Motivo, o.Now)
		}
	}
	if len(quitar) > 0 {
		if err := podar(orqDir, quitar); err != nil {
			return out, err
		}
	}
	return out, nil
}

func revisar(orqDir string, e Entrada, procs []sysinfo.Proc, o Opciones) Resultado {
	d := filepath.Clean(e.Path)
	r := Resultado{Path: e.Path}
	keep := func(m string) Resultado { r.Accion, r.Motivo = Conservado, m; return r }
	quar := func(m string) Resultado { r.Accion, r.Motivo = Cuarentena, m; return r }
	if _, err := os.Lstat(d); errors.Is(err, fs.ErrNotExist) {
		r.Accion, r.Motivo = Podado, "la carpeta ya no existe"
		return r
	}
	if o.Protegido != nil && o.Protegido(d) {
		return keep("es de un slice abierto")
	}
	if u := sysinfo.Usan(procs, d); len(u) > 0 {
		return keep(fmt.Sprintf("en uso por %s (pid %d)", u[0].Comm, u[0].PID))
	}
	raw, err := os.ReadFile(filepath.Join(d, ".git"))
	if err != nil {
		return quar("sin puntero .git: no es un worktree vinculado sano; revisar a mano (nunca se fuerza)")
	}
	gd := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(string(raw)), "gitdir:"))
	if st, err := os.Stat(gd); err != nil || !st.IsDir() {
		return quar("registro de git roto (" + gd + ")")
	}
	if existe(filepath.Join(gd, "locked")) {
		return keep("bloqueado con git worktree lock")
	}
	st, err := git(d, "status", "--porcelain", "--untracked-files=all", "--ignore-submodules=none")
	if err != nil {
		return quar("git status falló: " + err.Error())
	}
	if st != "" {
		return keep("cambios sin commit")
	}
	act := ultimaActividad(d)
	if lc, err := git(d, "log", "-1", "--format=%ct"); err == nil {
		if s, err := strconv.ParseInt(lc, 10, 64); err == nil && time.Unix(s, 0).After(act) {
			act = time.Unix(s, 0)
		}
	}
	if o.Now.Sub(act) < e.ttl() {
		return keep(fmt.Sprintf("actividad hace %.0f h (< ttl %s)", o.Now.Sub(act).Hours(), e.ttl()))
	}
	head, err := git(d, "rev-parse", "HEAD")
	if err != nil {
		return quar("sin HEAD legible: " + err.Error())
	}
	cont, _ := git(d, "for-each-ref", "--contains", head, "--format=%(refname)", "refs/remotes", "refs/heads")
	ign, motivo, err := ignorados(d, e.Repo)
	if err != nil {
		return quar("no pude listar ignorados: " + err.Error())
	}
	if motivo != "" {
		return keep(motivo)
	}
	if o.DryRun {
		r.Accion = Retiraria
		if cont == "" {
			r.Motivo = "HEAD sin ref duradera: se empaquetaría en un bundle"
		}
		if len(ign) > 0 {
			r.Motivo = strings.TrimPrefix(r.Motivo+fmt.Sprintf("; %d ignorado(s) único(s) se respaldarían", len(ign)), "; ")
		}
		return r
	}
	nombre := fmt.Sprintf("%s-%s", filepath.Base(d), o.Now.UTC().Format("20060102T150405"))
	if cont == "" {
		b, err := empaquetar(orqDir, e.Repo, d, head, nombre)
		if err != nil {
			return quar("HEAD sin ref duradera y el bundle falló: " + err.Error())
		}
		r.Bundle = b
	}
	if len(ign) > 0 {
		b, err := respaldar(orqDir, d, head, nombre, ign)
		if err != nil {
			return quar("respaldo de ignorados falló: " + err.Error())
		}
		r.Backup = b
	}
	if st, _ := git(d, "status", "--porcelain", "--untracked-files=all"); st != "" {
		return keep("cambió durante el proceso")
	}
	if _, err := git(e.Repo, "worktree", "remove", d); err != nil {
		return quar("git worktree remove (sin --force) falló: " + err.Error())
	}
	r.Accion = Retirado
	return r
}

// Retirar quita un worktree concreto con el mismo cuidado que GC (sin ttl ni registro): nunca
// fuerza, empaqueta un HEAD sin ref duradera y respalda (verificado) los ignorados únicos antes
// de `git worktree remove`. Si algo no es seguro, devuelve error y no borra nada.
func Retirar(orqDir, repo, d string, now time.Time) (Resultado, error) {
	r := Resultado{Path: d}
	head, err := git(d, "rev-parse", "HEAD")
	if err != nil {
		return r, fmt.Errorf("sin HEAD legible: %w", err)
	}
	ign, motivo, err := ignorados(d, repo)
	if err != nil {
		return r, fmt.Errorf("no pude listar ignorados: %w", err)
	}
	if motivo != "" {
		return r, errors.New(motivo)
	}
	nombre := fmt.Sprintf("%s-%s", filepath.Base(d), now.UTC().Format("20060102T150405"))
	for i, base := 2, nombre; existe(filepath.Join(orqDir, "backups", nombre+".tgz")) || existe(filepath.Join(orqDir, "bundles", nombre+".bundle")); i++ {
		nombre = fmt.Sprintf("%s-%d", base, i) // la misma ruta, retirada otra vez en el mismo segundo
	}
	if cont, _ := git(d, "for-each-ref", "--contains", head, "--format=%(refname)", "refs/remotes", "refs/heads"); cont == "" {
		if r.Bundle, err = empaquetar(orqDir, repo, d, head, nombre); err != nil {
			return r, fmt.Errorf("HEAD sin ref duradera y el bundle falló: %w", err)
		}
	}
	if len(ign) > 0 {
		if r.Backup, err = respaldar(orqDir, d, head, nombre, ign); err != nil {
			return r, fmt.Errorf("respaldo de ignorados falló: %w", err)
		}
	}
	if _, err := git(repo, "worktree", "remove", d); err != nil {
		return r, fmt.Errorf("git worktree remove (sin --force) falló: %w", err)
	}
	r.Accion = Retirado
	return r, nil
}

var saltar = map[string]bool{"node_modules": true, ".venv": true, "venv": true, ".git": true, "__pycache__": true, ".next": true}

// ultimaActividad: el archivo más nuevo, sin dependencias y sin el puntero .git de la raíz
// (git lo reescribe al listar o reparar worktrees: no es actividad de nadie).
func ultimaActividad(d string) time.Time {
	var m time.Time
	filepath.WalkDir(d, func(p string, de fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if de.IsDir() && p != d && saltar[de.Name()] {
			return filepath.SkipDir
		}
		if de.IsDir() || p == filepath.Join(d, ".git") {
			return nil
		}
		if info, err := de.Info(); err == nil && info.ModTime().After(m) {
			m = info.ModTime()
		}
		return nil
	})
	return m
}

var (
	regen    = regexp.MustCompile(`(^|/)(node_modules|__pycache__|\.venv|venv|\.next|\.pytest_cache|\.ruff_cache|\.mypy_cache|\.turbo|dist|build|coverage|[^/]*\.egg-info)(/|$)|\.tsbuildinfo$|(^|/)next-env\.d\.ts$|\.pyc$`)
	conserva = regexp.MustCompile(`(^|/)results\.json$`)
	secNom   = regexp.MustCompile(`(?i)(^|/)(\.env[^/]*|[^/]*\.(pem|key|p12|pfx|db|sqlite3?|dump)|[^/]*(secret|credential|token|passw)[^/]*|id_rsa[^/]*)$`)
	secTxt   = regexp.MustCompile(`sk-[A-Za-z0-9_-]{20,}|AKIA[0-9A-Z]{16}|ghp_[A-Za-z0-9]{30,}|xox[bpa]-[A-Za-z0-9-]{10,}|-----BEGIN [A-Z ]*PRIVATE KEY|eyJ[A-Za-z0-9_-]{20,}\.[A-Za-z0-9_-]{20,}|(?i:(api[_-]?key|secret|password|token)\s*[=:]\s*["']?[A-Za-z0-9/+_-]{16,})`)
)

type archivo struct {
	Rel    string `json:"ruta"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

// ignorados devuelve los ignorados únicos que hay que respaldar. Lo regenerable se omite;
// lo que ya está igual en el checkout principal también. Un secreto detiene todo: motivo != "".
func ignorados(d, repo string) ([]archivo, string, error) {
	raw, err := gitRaw(d, "ls-files", "-z", "-o", "-i", "--exclude-standard")
	if err != nil {
		return nil, "", err
	}
	var out []archivo
	for _, b := range bytes.Split(raw, []byte{0}) {
		rel := string(b)
		if rel == "" {
			continue
		}
		p := filepath.Join(d, rel)
		info, err := os.Lstat(p)
		if err != nil || !info.Mode().IsRegular() {
			continue // enlaces y especiales no se respaldan
		}
		if regen.MatchString(rel) && !conserva.MatchString(rel) {
			continue
		}
		if secNom.MatchString(rel) {
			return nil, "secreto ignorado (" + rel + "): no se respalda ni se borra; revísalo a mano", nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, "", err
		}
		if secTxt.Match(data[:min(len(data), 4<<20)]) {
			return nil, "secreto ignorado por contenido (" + rel + "): no se respalda ni se borra; revísalo a mano", nil
		}
		h := hash(data)
		if prin, err := os.ReadFile(filepath.Join(repo, rel)); err == nil && hash(prin) == h {
			continue // duplicado en el checkout principal
		}
		out = append(out, archivo{Rel: rel, SHA256: h, Bytes: info.Size()})
	}
	return out, "", nil
}

func empaquetar(orqDir, repo, d, head, nombre string) (string, error) {
	dir := filepath.Join(orqDir, "bundles")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	b := filepath.Join(dir, nombre+".bundle")
	if _, err := git(d, "bundle", "create", b, "HEAD"); err != nil {
		return "", err
	}
	if _, err := git(repo, "bundle", "verify", b); err != nil {
		return "", fmt.Errorf("bundle no verifica: %v", err)
	}
	if heads, err := git(repo, "bundle", "list-heads", b); err != nil || !strings.Contains(heads, head) {
		return "", fmt.Errorf("el bundle no contiene %s", head)
	}
	return b, nil
}

// respaldar escribe .orq/backups/<nombre>.tgz con su manifiesto y lo verifica releyéndolo.
func respaldar(orqDir, d, head, nombre string, fs []archivo) (string, error) {
	dir := filepath.Join(orqDir, "backups")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	tgz := filepath.Join(dir, nombre+".tgz")
	if existe(tgz) {
		return "", fmt.Errorf("%s ya existe", tgz)
	}
	f, err := os.OpenFile(tgz, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return "", err
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for _, a := range fs {
		data, err := os.ReadFile(filepath.Join(d, a.Rel))
		if err == nil && hash(data) != a.SHA256 {
			err = fmt.Errorf("%s cambió mientras se respaldaba", a.Rel)
		}
		if err == nil {
			err = tw.WriteHeader(&tar.Header{Name: a.Rel, Mode: 0o600, Size: int64(len(data)), ModTime: time.Now(), Typeflag: tar.TypeReg, Format: tar.FormatPAX})
		}
		if err == nil {
			_, err = tw.Write(data)
		}
		if err != nil {
			f.Close()
			return "", err
		}
	}
	errs := errors.Join(tw.Close(), gz.Close(), f.Sync(), f.Close())
	if errs != nil {
		return "", errs
	}
	// Verificar releyendo: cada archivo con su sha256, ni uno más ni uno menos.
	got, err := leerTgz(tgz)
	if err != nil {
		return "", err
	}
	want := map[string]string{}
	for _, a := range fs {
		want[a.Rel] = a.SHA256
	}
	if len(got) != len(want) {
		return "", errors.New("respaldo con discrepancia de archivos")
	}
	for k, v := range want {
		if got[k] != v {
			return "", fmt.Errorf("respaldo con discrepancia en %s", k)
		}
	}
	raw, _ := os.ReadFile(tgz)
	man, _ := json.MarshalIndent(map[string]any{"worktree": d, "head": head, "destino": tgz, "tgz_sha256": hash(raw), "archivos": fs}, "", " ")
	if err := os.WriteFile(strings.TrimSuffix(tgz, ".tgz")+".manifest.json", man, 0o600); err != nil {
		return "", err
	}
	return tgz, nil
}

func leerTgz(p string) (map[string]string, error) {
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	tr := tar.NewReader(gz)
	out := map[string]string{}
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return out, nil
		}
		if err != nil {
			return nil, err
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			return nil, err
		}
		out[h.Name] = hash(data)
	}
}

type cuarentenaReg struct {
	Ruta   string    `json:"ruta"`
	Slice  string    `json:"slice"`
	Motivo string    `json:"motivo"`
	TS     time.Time `json:"ts"`
}

func cuarentena(orqDir string, e Entrada, motivo string, now time.Time) {
	line, _ := json.Marshal(cuarentenaReg{Ruta: e.Path, Slice: e.Slice, Motivo: "gc: " + motivo, TS: now.UTC().Truncate(time.Second)})
	f, err := os.OpenFile(filepath.Join(orqDir, "cuarentena.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	f.Write(append(line, '\n'))
}

// podar reescribe el registro sin las rutas dadas, y solo si ya no existen en disco.
func podar(orqDir string, quitar map[string]bool) error {
	soltar, err := candado(orqDir)
	if err != nil {
		return err
	}
	defer soltar()
	es, err := Leer(orqDir)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	for _, e := range es {
		if quitar[e.Path] && !existe(e.Path) {
			continue
		}
		line, _ := json.Marshal(e)
		buf.Write(append(line, '\n'))
	}
	tmp := registro(orqDir) + ".tmp"
	if err := os.WriteFile(tmp, buf.Bytes(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, registro(orqDir))
}

func candado(orqDir string) (func(), error) {
	f, err := os.OpenFile(filepath.Join(orqDir, "worktrees.lock"), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, err
	}
	return func() { syscall.Flock(int(f.Fd()), syscall.LOCK_UN); f.Close() }, nil
}

func gitRaw(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return out, fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

func git(dir string, args ...string) (string, error) {
	out, err := gitRaw(dir, args...)
	return strings.TrimSpace(string(out)), err
}

func hash(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }

func existe(p string) bool { _, err := os.Lstat(p); return err == nil }
