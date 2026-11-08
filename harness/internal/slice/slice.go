// Package slice lleva el único slice abierto como una máquina de estados.
// El agente pide la transición; este paquete la valida. Nada se borra a la fuerza:
// lo dudoso queda en cuarentena y el cierre se puede repetir hasta terminar.
package slice

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"syscall"
	"time"

	"orq/internal/orqdir"
	"orq/internal/sysinfo"
)

// Fases del slice. ACORDADO vive en el canal (historia aprobada); aquí empieza en IMPLEMENTANDO,
// porque Abrir ya comprobó base sana y aceptación roja y dejó el worktree provisionado.
const (
	Implementando = "IMPLEMENTANDO"
	Verificado    = "VERIFICADO" // listo para demo
	Bloqueado     = "BLOQUEADO"  // vuelve al Orquestador o al Atena
	Integrando    = "INTEGRANDO"
	Cerrando      = "CERRANDO"
)

// fallaEn simula una interrupción real (el proceso muere) para probar que el cierre se retoma.
// Solo se activa con ORQ_FALLA_EN=<etapa> en pruebas.
func fallaEn(etapa string) {
	if os.Getenv("ORQ_FALLA_EN") == etapa {
		fmt.Fprintf(os.Stderr, "[orq] interrupción simulada en %q\n", etapa)
		os.Exit(9)
	}
}

// ErrBaseAvanzo: la rama base cambió durante el cierre; hay que integrar de nuevo.
var ErrBaseAvanzo = errors.New("la base avanzó")

// MaxFallos: verificaciones fallidas por intento antes de bloquear (la primera más 2 reparaciones).
const MaxFallos = 3

// MaxIntentos: a partir de aquí, un intento nuevo lo decide el Owner.
const MaxIntentos = 3

// Version identifica al verificador en los recibos.
var Version = "orq"

type Estado struct {
	NN          string    `json:"nn"`
	Intento     int       `json:"intento"`
	Fase        string    `json:"fase"`
	Titulo      string    `json:"titulo"`
	Test        string    `json:"test"`
	Bundle      []string  `json:"bundle"` // examen + config del runner + fixtures
	Cmd         string    `json:"cmd"`
	Sano        string    `json:"sano,omitempty"`       // comando de base sana
	Base        string    `json:"base"`                 // rama donde se integra
	SHARojo     string    `json:"sha_rojo"`             // base_sha
	SHAExamen   string    `json:"sha_examen,omitempty"` // commit donde se aprobó el examen vigente (si se sumó o corrigió)
	Rama        string    `json:"rama"`
	Worktree    string    `json:"worktree"`
	Procedencia string    `json:"procedencia"` // quién creó el worktree: git|herdr|orca
	Host        string    `json:"host"`
	Escritor    string    `json:"escritor,omitempty"` // writer lease
	Abierto     time.Time `json:"abierto"`
	Huella      string    `json:"huella"`
	SalidaRojo  string    `json:"salida_rojo"`
	Fallos      int       `json:"fallos,omitempty"`
	HeadFallo   string    `json:"head_fallo,omitempty"`
	FirmaFallo  string    `json:"firma_fallo,omitempty"`
	Reporte     string    `json:"reporte,omitempty"` // archivo que escribe el runner
	Formato     string    `json:"formato,omitempty"` // tap|junit|gotest-json
	Omitir      bool      `json:"permite_omitir,omitempty"`
	Aislado     bool      `json:"aislado,omitempty"`
	TimeoutSeg  int       `json:"timeout_seg,omitempty"`
	Recibo      *Recibo   `json:"recibo,omitempty"`
	Integrado   string    `json:"integrado,omitempty"` // SHA del merge revalidado
	IntDir      string    `json:"int_dir,omitempty"`   // worktree de integración en uso
	Equipo      []Worker  `json:"equipo,omitempty"`    // workers en paralelo dentro del slice
	Motivo      string    `json:"motivo,omitempty"`    // por qué está bloqueado
	Alcance     []string  `json:"alcance,omitempty"`   // rutas que la historia justifica tocar
}

func (e *Estado) ID() string      { return fmt.Sprintf("S-%s", e.NN) }
func (e *Estado) Attempt() string { return fmt.Sprintf("S-%s/a%d", e.NN, e.Intento) }

// Worker es un miembro del equipo de un slice: su zona, su rama y su copia de trabajo.
type Worker struct {
	Nombre   string   `json:"nombre"`
	Agente   string   `json:"agente"` // claude | codex
	Zona     string   `json:"zona"`   // front, back, test, ...
	Rutas    []string `json:"rutas"`  // lo único que puede cambiar
	Rama     string   `json:"rama"`
	Worktree string   `json:"worktree"`
	Juntado  string   `json:"juntado,omitempty"` // último commit suyo ya integrado en el slice
}

// Recibo es la evidencia de una verificación. "Terminé" no vale: vale el recibo.
type Recibo struct {
	Attempt    string    `json:"attempt"`
	Head       string    `json:"head"`
	Bundle     string    `json:"bundle_hash"`
	Runner     string    `json:"runner"`
	Cmd        string    `json:"cmd"`
	Exit       int       `json:"exit"`
	Salida     string    `json:"salida"`
	TS         time.Time `json:"ts"`
	Resultado  string    `json:"resultado"` // paso|fallo
	Contado    bool      `json:"contado"`   // hubo reporte del runner
	Ejecutados int       `json:"ejecutados"`
	Omitidos   int       `json:"omitidos"`
	Fallidos   int       `json:"fallidos"`
	Artefacto  string    `json:"artefacto,omitempty"` // salida completa
	Nota       string    `json:"nota,omitempty"`
}

// Huella une el contenido de cada archivo del paquete del examen y el comando.
func Huella(dir string, bundle []string, cmd string) (string, error) {
	h := sha256.New()
	for _, f := range bundle {
		raw, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			return "", err
		}
		fmt.Fprintf(h, "%s\x00%d\x00", f, len(raw))
		h.Write(raw)
	}
	h.Write([]byte(cmd))
	return hex.EncodeToString(h.Sum(nil)), nil
}

// HuellaEnCommit calcula la huella sobre el contenido commiteado en sha, no sobre la copia de trabajo:
// una edición sin commit (o hecha mientras corre el rojo) no puede colarse en la huella.
func HuellaEnCommit(dir, sha string, bundle []string, cmd string) (string, error) {
	h := sha256.New()
	for _, f := range bundle {
		raw, err := exec.Command("git", "-C", dir, "show", sha+":"+f).Output()
		if err != nil {
			return "", fmt.Errorf("no leo %s en %s: %v", f, sha, err)
		}
		fmt.Fprintf(h, "%s\x00%d\x00", f, len(raw))
		h.Write(raw)
	}
	h.Write([]byte(cmd))
	return hex.EncodeToString(h.Sum(nil)), nil
}

func cola(s string, n int) string {
	l := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(l) > n {
		l = l[len(l)-n:]
	}
	return strings.Join(l, "\n")
}

// Repo agrupa las rutas con las que trabaja el paquete.
type Repo struct {
	Root      string // checkout principal (del dueño)
	OrqDir    string
	Worktree  string // carpeta base de worktrees
	MaxSlices int    // abiertos a la vez; 0 = MaxSlicesDefecto
}

func (r Repo) dirSlices() string           { return filepath.Join(r.OrqDir, "slices") }
func (r Repo) estadoPath(nn string) string { return filepath.Join(r.dirSlices(), nn+".json") }
func (r Repo) viejoPath() string           { return filepath.Join(r.OrqDir, "slice.json") } // v0.4
func (r Repo) logPath() string             { return filepath.Join(r.OrqDir, "slices.jsonl") }
func (r Repo) recibosPath() string         { return filepath.Join(r.OrqDir, "recibos.jsonl") }
func (r Repo) CuarentenaPath() string      { return filepath.Join(r.OrqDir, "cuarentena.jsonl") }
func (r Repo) intPath(e *Estado) string {
	if e.IntDir != "" {
		return e.IntDir
	}
	return e.Worktree + "-int"
}

func Git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	s := strings.TrimSpace(string(out))
	if err != nil {
		return s, fmt.Errorf("git %s: %s", strings.Join(args, " "), s)
	}
	return s, nil
}

// MaxSlicesDefecto: slices abiertos a la vez (uno por worker en paralelo). El Orquestador usa
// solo los necesarios: 1 si el trabajo es pequeño, hasta 4 si se parte en piezas independientes.
const MaxSlicesDefecto = 4

func (r Repo) max() int {
	if r.MaxSlices > 0 {
		return r.MaxSlices
	}
	return MaxSlicesDefecto
}

func leerEstado(path string) (*Estado, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var e Estado
	if err := json.Unmarshal(raw, &e); err != nil {
		return nil, fmt.Errorf("%s: %v", path, err)
	}
	if len(e.Bundle) == 0 { // estado de v0.3
		e.Bundle = []string{e.Test}
	}
	return &e, nil
}

// migrar mueve el slice.json de v0.4 a .orq/slices/NN.json.
func (r Repo) migrar() error {
	e, err := leerEstado(r.viejoPath())
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := r.save(e); err != nil {
		return err
	}
	return os.Remove(r.viejoPath())
}

// Abiertos devuelve los slices abiertos, ordenados por número.
func (r Repo) Abiertos() ([]*Estado, error) {
	if err := r.migrar(); err != nil {
		return nil, err
	}
	m, _ := filepath.Glob(filepath.Join(r.dirSlices(), "*.json"))
	slices.Sort(m)
	var out []*Estado
	for _, f := range m {
		e, err := leerEstado(f)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil
}

// ErrVarios: hay más de un slice abierto y no se dijo cuál.
var ErrVarios = errors.New("hay varios slices abiertos")

// Load devuelve el slice nn; con nn vacío, el único abierto (nil si no hay ninguno).
func (r Repo) Load(nn ...string) (*Estado, error) {
	if len(nn) > 0 && nn[0] != "" {
		if err := r.migrar(); err != nil {
			return nil, err
		}
		e, err := leerEstado(r.estadoPath(strings.TrimPrefix(nn[0], "S-")))
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("S-%s no está abierto", strings.TrimPrefix(nn[0], "S-"))
		}
		return e, err
	}
	ab, err := r.Abiertos()
	switch {
	case err != nil:
		return nil, err
	case len(ab) == 0:
		return nil, nil
	case len(ab) > 1:
		var ids []string
		for _, e := range ab {
			ids = append(ids, e.ID())
		}
		return nil, fmt.Errorf("%w (%s): di cuál con --slice NN", ErrVarios, strings.Join(ids, ", "))
	}
	return ab[0], nil
}

func (r Repo) save(e *Estado) error {
	os.MkdirAll(r.dirSlices(), 0o755)
	raw, _ := json.MarshalIndent(e, "", "  ")
	return orqdir.WriteAtomic(r.estadoPath(e.NN), raw)
}

// candado toma un flock exclusivo sobre .orq/<nombre>.lock hasta que se llame a la función devuelta.
func (r Repo) candado(nombre string) (func(), error) {
	f, err := os.OpenFile(filepath.Join(r.OrqDir, nombre+".lock"), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, err
	}
	return func() { syscall.Flock(int(f.Fd()), syscall.LOCK_UN); f.Close() }, nil
}

func appendJSON(path string, v any) error {
	line, _ := json.Marshal(v)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(line, '\n'))
	return err
}

func existe(p string) bool { _, err := os.Stat(p); return err == nil }

// Opciones extra al abrir.
type Opciones struct {
	Bundle []string // archivos además del examen que no se pueden tocar
	Sano   string   // comando que demuestra que la base está sana antes de empezar
	// Reporte y Formato: el runner deja un reporte que orq cuenta (sin él, verde puede ser "no corrió nada").
	Reporte, Formato string
	PermiteOmitir    bool
	Aislado          bool     // corre el examen en una copia del commit, sin archivos del worker
	Timeout          int      // segundos; 0 = 600
	Alcance          []string // rutas que la historia justifica; fuera de ellas, alarma de desalineación
}

func (e *Estado) limite() time.Duration {
	if e.TimeoutSeg > 0 {
		return time.Duration(e.TimeoutSeg) * time.Second
	}
	return TimeoutDefecto
}

// correr ejecuta el examen y cuenta las pruebas si hay reporte.
func (e *Estado) correr(dir string) (string, int, bool, *Conteo, error) {
	if e.Reporte != "" {
		borrarReporte(dir, e.Reporte)
	}
	out, code, ok := RunCmdT(dir, e.Cmd, e.limite())
	if e.Reporte == "" {
		return out, code, ok, nil, nil
	}
	raw, err := os.ReadFile(filepath.Join(dir, e.Reporte))
	if err != nil {
		return out, code, ok, &Conteo{}, nil // sin reporte: 0 pruebas
	}
	c, err := Contar(e.Formato, raw)
	return out, code, ok, &c, err
}

// Abrir descubre antes de crear, exige base sana y aceptación roja, y provisiona un worktree.
func (r Repo) Abrir(nn, titulo, test, cmd string, o Opciones, now time.Time) (*Estado, string, error) {
	soltar, err := r.candado("abrir")
	if err != nil {
		return nil, "", err
	}
	defer soltar()
	ab, err := r.Abiertos()
	if err != nil {
		return nil, "", err
	}
	if len(ab) >= r.max() {
		return nil, "", fmt.Errorf("ya hay %d slice(s) abierto(s) y el tope es %d. Cierra uno antes", len(ab), r.max())
	}
	for _, e := range ab {
		if e.NN == nn {
			return nil, "", fmt.Errorf("%s ya está abierto", e.ID())
		}
	}
	bundle := append([]string{test}, o.Bundle...)
	for _, f := range bundle {
		if !existe(filepath.Join(r.Root, f)) {
			return nil, "", fmt.Errorf("no existe %s", f)
		}
		if _, err := Git(r.Root, "ls-files", "--error-unmatch", "--", f); err != nil {
			return nil, "", fmt.Errorf("%s no está versionado (¿ignorado por git?): el paquete del examen tiene que estar en el commit", f)
		}
		if out, _ := Git(r.Root, "status", "--porcelain", "--", f); out != "" {
			return nil, "", fmt.Errorf("%s no está commiteado; haz commit del rojo antes de abrir", f)
		}
	}
	rama := "slice/" + nn
	wt := filepath.Join(r.Worktree, orqdir.Slug(r.Root)+"-s"+nn)
	// Paso A: descubrir. Si algo ya ocupa el nombre, no se crea otro encima.
	if out, _ := Git(r.Root, "branch", "--list", rama); out != "" {
		return nil, "", fmt.Errorf("la rama %s ya existe (¿intento anterior?). Revísala con `orq doctor`; no se crea otra", rama)
	}
	if existe(wt) {
		return nil, "", fmt.Errorf("la carpeta %s ya existe. Revísala con `orq doctor`; no se crea otra", wt)
	}
	base, err := Git(r.Root, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil || base == "HEAD" {
		return nil, "", errors.New("el checkout principal no está en una rama")
	}
	// Paso B: base sana separada de aceptación roja.
	if o.Sano != "" {
		if out, _, ok := RunCmd(r.Root, o.Sano); !ok {
			return nil, out, errors.New("la base no está sana: el comando --sano falla antes de empezar. Arregla la base primero")
		}
	}
	if (o.Reporte == "") != (o.Formato == "") {
		return nil, "", errors.New("--reporte y --formato van juntos")
	}
	if o.Formato != "" && !slices.Contains(Formatos, o.Formato) {
		return nil, "", fmt.Errorf("formato %q: usa %s", o.Formato, strings.Join(Formatos, "|"))
	}
	prov := &Estado{Cmd: cmd, Reporte: o.Reporte, Formato: o.Formato, TimeoutSeg: o.Timeout}
	out, _, ok, cnt, err := prov.correr(r.Root)
	if o.Reporte != "" {
		borrarReporte(r.Root, o.Reporte)
	}
	if err != nil {
		return nil, out, err
	}
	if ok {
		return nil, out, errors.New("el examen ya pasa: no está en rojo, así que no mide nada todavía")
	}
	if cnt != nil && cnt.Fallidos == 0 {
		return nil, out, errors.New("el examen falla, pero no por aserción (0 pruebas fallidas en el reporte: ¿no compila o no arrancó?). Un rojo válido es una prueba que corre y falla")
	}
	sha, err := Git(r.Root, "rev-parse", "HEAD")
	if err != nil {
		return nil, out, err
	}
	huella, err := HuellaEnCommit(r.Root, sha, bundle, cmd)
	if err != nil {
		return nil, out, err
	}
	host, _ := os.Hostname()
	e := &Estado{NN: nn, Intento: 1, Fase: Implementando, Titulo: titulo, Test: test, Bundle: bundle, Cmd: cmd,
		Sano: o.Sano, Alcance: o.Alcance, Base: base, SHARojo: sha, Rama: rama, Worktree: wt, Procedencia: "git", Host: host,
		Abierto: now.UTC().Truncate(time.Second), Huella: huella, SalidaRojo: cola(out, 20),
		Reporte: o.Reporte, Formato: o.Formato, Omitir: o.PermiteOmitir, Aislado: o.Aislado, TimeoutSeg: o.Timeout}
	if _, err := Git(r.Root, "worktree", "add", e.Worktree, "-b", e.Rama); err == nil {
		prepararDeps(r.Root, e.Worktree)
	} else {
		return nil, out, err
	}
	return e, out, r.save(e)
}

// Tomar asigna el único escritor del slice. Un relevo hereda el mismo worktree, no crea otro.
func (r Repo) Tomar(nn, quien string, relevo bool) (*Estado, error) {
	soltar, err := r.bloqueo(nn)
	if err != nil {
		return nil, err
	}
	defer soltar()
	e, err := r.Load(nn)
	if err != nil || e == nil {
		return nil, errors.Join(err, errors.New("no hay slice abierto"))
	}
	if e.Fase != Implementando {
		return e, fmt.Errorf("%s está en %s; solo se toma en %s", e.ID(), e.Fase, Implementando)
	}
	if e.Escritor != "" && e.Escritor != quien && !relevo {
		return e, fmt.Errorf("%s ya tiene escritor: %s. Un solo escritor por slice; usa --relevo si lo reemplazas", e.ID(), e.Escritor)
	}
	e.Escritor = quien
	return e, r.save(e)
}

// Ampliar suma rutas al alcance con una aprobación ya dada (Atena u Owner); el examen nunca entra.
func (r Repo) Ampliar(nn string, rutas []string) (*Estado, error) {
	soltar, err := r.bloqueo(nn)
	if err != nil {
		return nil, err
	}
	defer soltar()
	e, err := r.Load(nn)
	if err != nil || e == nil {
		return nil, errors.Join(err, errors.New("no hay slice abierto"))
	}
	// Sin alcance el slice ya puede tocar todo: sumar rutas lo encogería a solo esas.
	libre := len(e.Alcance) == 0
	for _, b := range rutas {
		for _, f := range e.Bundle {
			if dentro(b, f) {
				return e, fmt.Errorf("la ruta %s incluye el examen %s, que nadie edita", b, f)
			}
		}
		if !libre && !slices.Contains(e.Alcance, b) {
			e.Alcance = append(e.Alcance, b)
		}
	}
	return e, r.save(e)
}

// baseExamen es el commit contra el que el examen no puede cambiar: el rojo, o el último examen aprobado.
func (e *Estado) baseExamen() string {
	if e.SHAExamen != "" {
		return e.SHAExamen
	}
	return e.SHARojo
}

// Verificar corre el examen y deja un recibo. Falla dos veces o sin progreso → BLOQUEADO.
func (r Repo) Verificar(nn string, now time.Time) (*Estado, string, error) {
	e, err := r.Load(nn)
	if err != nil || e == nil {
		return nil, "", errors.Join(err, errors.New("no hay slice abierto"))
	}
	if e.Fase != Implementando && e.Fase != Verificado {
		return e, "", fmt.Errorf("%s está en %s; no se verifica ahora", e.ID(), e.Fase)
	}
	if out, _ := Git(e.Worktree, "status", "--porcelain"); out != "" {
		return e, out, errors.New("el worktree tiene cambios sin commit; el worker debe commitear todo")
	}
	if diff, _ := Git(e.Worktree, append([]string{"diff", "--stat", e.baseExamen() + "..HEAD", "--"}, e.Bundle...)...); diff != "" {
		return e, diff, r.bloquear(e, "el paquete del examen fue modificado desde el rojo: trampa")
	}
	if h, _ := Huella(e.Worktree, e.Bundle, e.Cmd); h != e.Huella {
		return e, "", r.bloquear(e, "la huella del examen cambió desde el rojo: trampa")
	}
	head, err := Git(e.Worktree, "rev-parse", "HEAD")
	if err != nil {
		return e, "", err
	}
	dir := e.Worktree
	if e.Aislado {
		if dir, err = copiaAislada(e.Worktree, head); err != nil {
			return e, "", err
		}
		defer os.RemoveAll(dir) // carpeta temporal propia de orq
	}
	out, code, ok, cnt, err := e.correr(dir)
	if err != nil {
		return e, out, err
	}
	// Una escritura por shell durante la corrida también cuenta como trampa.
	if h, _ := Huella(dir, e.Bundle, e.Cmd); h != e.Huella {
		return e, out, r.bloquear(e, "el examen se modificó mientras corría: trampa")
	}
	rec := &Recibo{Attempt: e.Attempt(), Head: head, Bundle: e.Huella, Runner: Version, Cmd: e.Cmd,
		Exit: code, Salida: cola(out, 20), TS: now.UTC().Truncate(time.Second), Resultado: "paso"}
	if cnt != nil {
		rec.Contado, rec.Ejecutados, rec.Omitidos, rec.Fallidos = true, cnt.Ejecutados, cnt.Omitidos, cnt.Fallidos
		switch {
		case ok && cnt.Ejecutados == 0:
			ok, rec.Nota = false, "el comando salió bien pero corrió 0 pruebas"
		case ok && cnt.Omitidos > 0 && !e.Omitir:
			ok, rec.Nota = false, fmt.Sprintf("%d pruebas omitidas y el slice no permite omitir", cnt.Omitidos)
		case ok && cnt.Fallidos > 0:
			ok, rec.Nota = false, fmt.Sprintf("salió 0 pero el reporte trae %d fallidas", cnt.Fallidos)
		}
	} else {
		rec.Nota = "sin contadores: declara --reporte y --formato para saber cuántas pruebas corrieron"
	}
	salidas := filepath.Join(r.OrqDir, "salidas")
	os.MkdirAll(salidas, 0o755)
	rec.Artefacto = filepath.Join(salidas, fmt.Sprintf("%s-a%d-%d.txt", e.ID(), e.Intento, now.UnixNano()))
	os.WriteFile(rec.Artefacto, []byte(out), 0o644)
	if !ok {
		rec.Resultado = "fallo"
	}
	// La corrida pudo durar minutos: si el slice cambió mientras tanto (relevo, rechazo, reintento), el recibo no vale.
	soltar, err := r.bloqueo(e.NN)
	if err != nil {
		return e, out, err
	}
	defer soltar()
	if ahora, err := r.Load(e.NN); err != nil || ahora.Intento != e.Intento || ahora.Fase != e.Fase {
		return e, out, fmt.Errorf("%s cambió mientras se verificaba (relevo, reintento o cierre); recibo descartado, vuelve a verificar", e.Attempt())
	} else {
		e.Escritor = ahora.Escritor
	}
	appendJSON(r.recibosPath(), rec)
	if ok {
		e.Fase, e.Recibo = Verificado, rec
		return e, out, r.save(e)
	}
	motivo := "el examen no pasa"
	if rec.Nota != "" && rec.Contado {
		motivo = rec.Nota
	}
	firma := firmaDe(out)
	sinProgreso := e.HeadFallo == head
	if !sinProgreso && e.HeadFallo != "" && firma == e.FirmaFallo {
		_, errDiff := Git(e.Worktree, "diff", "-w", "--quiet", e.HeadFallo, head)
		sinProgreso = errDiff == nil // solo cambiaron espacios o commits vacíos
	}
	e.Fallos, e.HeadFallo, e.FirmaFallo, e.Recibo, e.Fase = e.Fallos+1, head, firma, nil, Implementando
	switch {
	case sinProgreso:
		return e, out, r.bloquear(e, motivo+"; falla igual sin cambios reales: sin progreso")
	case e.Fallos >= MaxFallos:
		return e, out, r.bloquear(e, fmt.Sprintf("%s; %d verificaciones fallidas en %s", motivo, e.Fallos, e.Attempt()))
	}
	return e, out, errors.Join(r.save(e), fmt.Errorf("%s (reparación %d de %d)", motivo, e.Fallos, MaxFallos-1))
}

func (r Repo) bloquear(e *Estado, motivo string) error {
	e.Fase, e.Motivo, e.Escritor = Bloqueado, motivo, ""
	if err := r.save(e); err != nil {
		return err
	}
	return fmt.Errorf("%s BLOQUEADO: %s. Decide el Orquestador (o el Atena): `orq slice reintentar \"qué cambia\"` o cerrar como rechazado", e.ID(), motivo)
}

// Reintentar abre un intento nuevo en el mismo worktree. Recibos viejos ya no valen.
func firmaDe(out string) string {
	h := sha256.Sum256([]byte(strings.Join(strings.Fields(cola(out, 30)), " ")))
	return hex.EncodeToString(h[:8])
}

func (r Repo) Reintentar(nn, cambio string, decisionOwner bool) (*Estado, error) {
	soltar, err := r.bloqueo(nn)
	if err != nil {
		return nil, err
	}
	defer soltar()
	e, err := r.Load(nn)
	if err != nil || e == nil {
		return nil, errors.Join(err, errors.New("no hay slice abierto"))
	}
	if e.Fase != Bloqueado {
		return e, fmt.Errorf("%s está en %s; solo se reintenta desde %s", e.ID(), e.Fase, Bloqueado)
	}
	if strings.TrimSpace(cambio) == "" {
		return e, errors.New("di qué cambia en este intento; repetir lo mismo no es reintentar")
	}
	if e.Intento >= MaxIntentos && !decisionOwner {
		return e, fmt.Errorf("%s ya lleva %d intentos: el siguiente lo decide el Owner (el Atena pregunta y luego --decision-owner), o se parte la historia", e.ID(), e.Intento)
	}
	e.Intento++
	e.FirmaFallo = ""
	e.Fase, e.Fallos, e.HeadFallo, e.Recibo, e.Motivo, e.Escritor = Implementando, 0, "", nil, "", ""
	return e, r.save(e)
}

// Rehuella repara una huella mal tomada al abrir: solo desde BLOQUEADO, solo si el examen del
// worktree es idéntico byte a byte al commiteado en el rojo y ningún commit del slice lo tocó.
// La huella nueva sale del commit del rojo, así que no sirve para colar un examen cambiado.
func (r Repo) Rehuella(nn, motivo string) (*Estado, error) {
	soltar, err := r.bloqueo(nn)
	if err != nil {
		return nil, err
	}
	defer soltar()
	e, err := r.Load(nn)
	if err != nil || e == nil {
		return nil, errors.Join(err, errors.New("no hay slice abierto"))
	}
	if e.Fase != Bloqueado {
		return e, fmt.Errorf("%s está en %s; la huella solo se repara desde %s", e.ID(), e.Fase, Bloqueado)
	}
	if strings.TrimSpace(motivo) == "" {
		return e, errors.New("di por qué la huella estaba mal y quién lo aprobó")
	}
	if diff, _ := Git(e.Worktree, append([]string{"diff", "--stat", e.baseExamen() + "..HEAD", "--"}, e.Bundle...)...); diff != "" {
		return e, errors.New("el slice modificó el examen desde el rojo: eso es trampa, no una huella mal tomada")
	}
	enRojo, err := HuellaEnCommit(e.Worktree, e.baseExamen(), e.Bundle, e.Cmd)
	if err != nil {
		return e, err
	}
	if h, _ := Huella(e.Worktree, e.Bundle, e.Cmd); h != enRojo {
		return e, errors.New("el examen del worktree no es idéntico al del rojo: no se repara")
	}
	e.Huella, e.Fase, e.Motivo, e.Escritor = enRojo, Implementando, "huella reparada: "+motivo, ""
	return e, r.save(e)
}

// SumarExamen protege un examen nuevo, o corrige uno protegido, dentro de un slice abierto, con aprobación.
// Los archivos nombrados están commiteados y limpios y fuera de toda zona; el resto del examen no cambió.
// La huella y la base del examen pasan a HEAD: desde ahí nadie lo edita y la verificación lo corre.
func (r Repo) SumarExamen(nn string, nuevos []string, cmd, aprobado string) (*Estado, error) {
	soltar, err := r.bloqueo(nn)
	if err != nil {
		return nil, err
	}
	defer soltar()
	e, err := r.Load(nn)
	if err != nil || e == nil {
		return nil, errors.Join(err, errors.New("no hay slice abierto"))
	}
	switch {
	case e.Fase != Implementando:
		return e, fmt.Errorf("%s está en %s; un examen se suma mientras se implementa", e.ID(), e.Fase)
	case strings.TrimSpace(aprobado) == "":
		return e, errors.New("di quién aprobó el examen nuevo (req de Atena u Owner)")
	case len(nuevos) == 0:
		return e, errors.New("di qué archivos de examen se suman")
	}
	intactos := []string{}
	for _, f := range e.Bundle {
		if !slices.Contains(nuevos, f) {
			intactos = append(intactos, f)
		}
	}
	if len(intactos) > 0 {
		if diff, _ := Git(e.Worktree, append([]string{"diff", "--stat", e.baseExamen() + "..HEAD", "--"}, intactos...)...); diff != "" {
			return e, errors.New("el slice modificó un examen que no nombraste: no se suma ni se corrige nada")
		}
	}
	bundle := append([]string{}, e.Bundle...)
	for _, f := range nuevos {
		if out, _ := Git(e.Worktree, "ls-files", "--error-unmatch", f); out == "" {
			return e, fmt.Errorf("%s no está commiteado en el slice", f)
		}
		if st, _ := Git(e.Worktree, "status", "--porcelain", "--", f); st != "" {
			return e, fmt.Errorf("%s tiene cambios sin commit", f)
		}
		for _, w := range e.Equipo {
			for _, ruta := range w.Rutas {
				if dentro(ruta, f) {
					return e, fmt.Errorf("%s está dentro de la zona %s: un examen no es de ningún worker", f, w.Zona)
				}
			}
		}
		if !slices.Contains(bundle, f) {
			bundle = append(bundle, f)
		}
	}
	head, err := Git(e.Worktree, "rev-parse", "HEAD")
	if err != nil {
		return e, err
	}
	if strings.TrimSpace(cmd) != "" {
		e.Cmd = cmd
	}
	h, err := Huella(e.Worktree, bundle, e.Cmd)
	if err != nil {
		return e, err
	}
	// El contrato cambió: un recibo anterior ya no prueba nada; hay que volver a verificar.
	e.Bundle, e.Huella, e.SHAExamen, e.Motivo, e.Recibo = bundle, h, strings.TrimSpace(head), "examen sumado o corregido: "+aprobado, nil
	return e, r.save(e)
}

// Reabrir vuelve a construir un slice verificado para una reparación pedida por Atena u Owner:
// mismo intento, mismo worktree, mismo equipo y sus commits. El recibo deja de valer.
func (r Repo) Reabrir(nn, motivo string) (*Estado, error) {
	soltar, err := r.bloqueo(nn)
	if err != nil {
		return nil, err
	}
	defer soltar()
	e, err := r.Load(nn)
	if err != nil || e == nil {
		return nil, errors.Join(err, errors.New("no hay slice abierto"))
	}
	if e.Fase != Verificado && e.Fase != Integrando {
		return e, fmt.Errorf("%s está en %s; solo se reabre desde %s o %s", e.ID(), e.Fase, Verificado, Integrando)
	}
	if strings.TrimSpace(motivo) == "" {
		return e, errors.New("di qué se repara; reabrir sin motivo no vale")
	}
	e.Fase, e.Recibo, e.Motivo = Implementando, nil, motivo
	if len(e.Equipo) > 0 {
		e.Escritor = "equipo"
	}
	return e, r.save(e)
}

type Registro struct {
	NN      string    `json:"nn"`
	Intento int       `json:"intento"`
	Titulo  string    `json:"titulo"`
	Result  string    `json:"resultado"`
	Motivo  string    `json:"motivo,omitempty"`
	SHA     string    `json:"sha,omitempty"`
	Abierto time.Time `json:"abierto"`
	Cerrado time.Time `json:"cerrado"`
}

// Cuarentena es algo que el harness no se atreve a borrar.
type Cuarentena struct {
	Ruta   string    `json:"ruta"`
	Rama   string    `json:"rama,omitempty"`
	Slice  string    `json:"slice"`
	Motivo string    `json:"motivo"`
	TS     time.Time `json:"ts"`
}

// EnUso lista quién trabaja dentro de una carpeta (proceso o panel); se reemplaza en pruebas.
var EnUso = func(dir string) []string {
	procs, err := sysinfo.ListProcs()
	if err != nil {
		return nil
	}
	var out []string
	for _, p := range sysinfo.Usan(procs, dir) {
		out = append(out, fmt.Sprintf("%s (pid %d)", p.Comm, p.PID))
	}
	return out
}

// ErrEnUso: no se retira un worktree con un proceso adentro; se cierra su panel y se repite.
var ErrEnUso = errors.New("worktree en uso")

// quitarWorktree nunca fuerza: si git se niega, el worktree va a cuarentena.
func (r Repo) quitarWorktree(e *Estado, ruta string, now time.Time) string {
	if !existe(ruta) { // nada que quitar; no se hace prune global: tocaría worktrees ajenos
		return ""
	}
	if _, err := Git(r.Root, "worktree", "remove", ruta); err != nil {
		appendJSON(r.CuarentenaPath(), Cuarentena{Ruta: ruta, Slice: e.ID(), Motivo: err.Error(), TS: now.UTC().Truncate(time.Second)})
		return ruta
	}
	return ""
}

// Cerrar con aceptado integra en un worktree aparte, revalida, publica la rama base y limpia.
// Cada paso queda guardado: si se interrumpe, volver a correrlo continúa donde quedó.
// ErrYaCerrado: otro cierre del mismo slice terminó primero; repetir no hace nada.
var ErrYaCerrado = errors.New("el slice ya estaba cerrado")

func (r Repo) Cerrar(nn string, aceptado bool, motivo string, now time.Time) (*Registro, []string, string, error) {
	soltar, err := r.bloqueo(nn)
	if err != nil {
		return nil, nil, "", err
	}
	defer soltar()
	e, err := r.Load(nn)
	if nn != "" && err != nil && strings.Contains(err.Error(), "no está abierto") {
		for _, reg := range leerJSONL[Registro](r.logPath()) {
			if reg.NN == strings.TrimPrefix(nn, "S-") {
				return &reg, nil, "", ErrYaCerrado
			}
		}
	}
	if err != nil || e == nil {
		return nil, nil, "", errors.Join(err, errors.New("no hay slice abierto"))
	}
	reg := &Registro{NN: e.NN, Intento: e.Intento, Titulo: e.Titulo, Abierto: e.Abierto, Cerrado: now.UTC().Truncate(time.Second), Motivo: motivo}
	var out string
	var cuarentena []string
	if aceptado {
		if !slices.Contains([]string{Verificado, Integrando, Cerrando}, e.Fase) {
			return nil, nil, "", fmt.Errorf("%s está en %s; solo se integra con un recibo de verificación", e.ID(), e.Fase)
		}
		if e.Fase == Verificado {
			head, _ := Git(e.Worktree, "rev-parse", "HEAD")
			if e.Recibo == nil || e.Recibo.Attempt != e.Attempt() || e.Recibo.Head != head {
				return nil, nil, "", errors.New("el recibo no es de este intento o de este commit: corre `orq slice verificar`")
			}
			e.Fase = Integrando
			if err := r.save(e); err != nil {
				return nil, nil, "", err
			}
		}
		// Integrar y publicar van en serie: dos cierres nunca integran a la vez.
		soltar, err := r.candado("integracion")
		if err != nil {
			return nil, nil, "", err
		}
		defer soltar()
		if e2, err := r.Load(e.NN); err == nil { // otro proceso pudo avanzar este cierre mientras esperábamos
			e = e2
		}
		fallaEn("integrar")
		if e.Fase == Integrando {
			if out, err = r.integrar(e); err != nil {
				return nil, nil, out, err
			}
			fallaEn("integrado")
			e.Fase = Cerrando
			if err := r.save(e); err != nil {
				return nil, nil, out, err
			}
		}
		if err := r.publicar(e); err != nil {
			if errors.Is(err, ErrBaseAvanzo) {
				e.Fase = Integrando // la próxima vez integra de nuevo sobre la base nueva
				r.save(e)
			}
			return nil, nil, out, err
		}
		fallaEn("publicado")
		if err := r.libre(e); err != nil {
			return nil, nil, out, err
		}
		reg.Result, reg.SHA = "aceptado", e.Integrado
		if q := r.quitarWorktree(e, r.intPath(e), now); q != "" {
			cuarentena = append(cuarentena, q)
		}
	} else {
		if !slices.Contains([]string{Implementando, Verificado, Bloqueado}, e.Fase) {
			return nil, nil, "", fmt.Errorf("%s está en %s: ya se está integrando; termina el cierre aceptado", e.ID(), e.Fase)
		}
		reg.Result = "rechazado"
		if err := r.libre(e); err != nil {
			return nil, nil, "", err
		}
		if q := r.quitarWorktree(e, r.intPath(e), now); q != "" {
			cuarentena = append(cuarentena, q)
		}
	}
	cuarentena = append(cuarentena, r.limpiarEquipo(e, aceptado, now)...)
	if q := r.quitarWorktree(e, e.Worktree, now); q != "" {
		cuarentena = append(cuarentena, q)
	}
	fallaEn("limpiando")
	if aceptado {
		if b, _ := Git(r.Root, "branch", "--list", e.Rama); b != "" {
			// -d depende de la rama que el Owner tenga abierta; se comprueba contra la base y se borra solo si ya está integrada.
			sha, _ := Git(r.Root, "rev-parse", e.Rama)
			if _, err := Git(r.Root, "merge-base", "--is-ancestor", e.Rama, e.Base); err != nil {
				appendJSON(r.CuarentenaPath(), Cuarentena{Rama: e.Rama, Slice: e.ID(), Motivo: "no está integrada en " + e.Base, TS: reg.Cerrado})
				cuarentena = append(cuarentena, e.Rama)
			} else if _, err := Git(r.Root, "update-ref", "-d", "refs/heads/"+e.Rama, sha); err != nil {
				appendJSON(r.CuarentenaPath(), Cuarentena{Rama: e.Rama, Slice: e.ID(), Motivo: err.Error(), TS: reg.Cerrado})
				cuarentena = append(cuarentena, e.Rama)
			}
		}
	}
	if err := appendJSON(r.logPath(), reg); err != nil {
		return nil, cuarentena, out, err
	}
	return reg, cuarentena, out, os.Remove(r.estadoPath(e.NN))
}

// bloqueo serializa los cambios de estado de un slice.
func (r Repo) bloqueo(nn string) (func(), error) {
	if nn == "" {
		if e, err := r.Load(); err == nil && e != nil {
			nn = e.NN
		}
	}
	return r.candado("slice-" + strings.TrimPrefix(nn, "S-"))
}

// borrarReporte quita el reporte anterior del runner, pero nunca un archivo versionado.
func borrarReporte(dir, rel string) {
	if _, err := Git(dir, "ls-files", "--error-unmatch", "--", rel); err == nil {
		return
	}
	os.Remove(filepath.Join(dir, rel))
}

// libre comprueba que nadie trabaje en las carpetas que se van a quitar.
func (r Repo) libre(e *Estado) error {
	ds := []string{e.Worktree, r.intPath(e)}
	for _, w := range e.Equipo {
		ds = append(ds, w.Worktree)
	}
	for _, d := range ds {
		if u := EnUso(d); len(u) > 0 && existe(d) {
			return fmt.Errorf("%w: %s lo usa %s. Cierra ese panel o sesión y repite el cierre (continúa donde quedó)", ErrEnUso, d, strings.Join(u, ", "))
		}
	}
	return nil
}

// integrar hace el merge en un worktree propio, nunca en el checkout del dueño, y revalida allí.
func (r Repo) integrar(e *Estado) (string, error) {
	ip := r.intPath(e)
	if existe(ip) { // quedó de un cierre interrumpido: se quita sin forzar; si no se puede, a cuarentena y otra carpeta
		if q := r.quitarWorktree(e, ip, time.Now()); q != "" {
			ip = fmt.Sprintf("%s-int-%d", e.Worktree, time.Now().UnixNano())
		}
	}
	e.IntDir = ip
	if err := r.save(e); err != nil {
		return "", err
	}
	if _, err := Git(r.Root, "worktree", "add", "--detach", ip, e.Base); err == nil {
		prepararDeps(r.Root, ip)
	} else {
		return "", err
	}
	if e.Recibo == nil {
		return "", errors.New("no hay recibo de verificación: no se integra")
	}
	if _, err := Git(ip, "merge", "--no-ff", e.Recibo.Head, "-m", fmt.Sprintf("slice %s: %s", e.Attempt(), e.Titulo)); err != nil {
		Git(ip, "merge", "--abort")
		return "", fmt.Errorf("conflicto al integrar con %s: %v. Nada se borró; el slice sigue en %s", e.Base, err, Integrando)
	}
	if h, _ := Huella(ip, e.Bundle, e.Cmd); h != e.Huella {
		return "", fmt.Errorf("tras integrar, el paquete del examen difiere del rojo (la base cambió). Nada se borró")
	}
	out, _, ok, cnt, _ := e.correr(ip)
	if ok && cnt != nil && (cnt.Ejecutados == 0 || cnt.Fallidos > 0) {
		ok = false
	}
	if !ok {
		return out, fmt.Errorf("integrado en %s, pero el examen falla ahí. Tu rama %s no se tocó; nada se borró", ip, e.Base)
	}
	if e.Sano != "" {
		if o2, _, ok := RunCmd(ip, e.Sano); !ok {
			return o2, fmt.Errorf("integrado, pero la base dejó de estar sana (--sano falla). Nada se borró")
		}
	}
	sha, err := Git(ip, "rev-parse", "HEAD")
	e.Integrado = sha
	return out, err
}

// publicar avanza la rama base al merge revalidado. Si el dueño tiene la rama abierta y limpia,
// solo avanza (fast-forward); si tiene cambios, espera sin tocar nada.
func (r Repo) publicar(e *Estado) error {
	if _, err := Git(r.Root, "merge-base", "--is-ancestor", e.Integrado, e.Base); err == nil {
		return nil // ya publicado: repetir el cierre no hace nada dos veces
	}
	actual, _ := Git(r.Root, "rev-parse", "--abbrev-ref", "HEAD")
	if actual == e.Base {
		if st, _ := Git(r.Root, "status", "--porcelain", "--untracked-files=no"); st != "" {
			return fmt.Errorf("integrado y revalidado (%s), pero tu checkout tiene cambios sin commit en %s. Commitéalos o guárdalos y vuelve a correr el cierre: continúa donde quedó", e.Integrado[:8], e.Base)
		}
		if _, err := Git(r.Root, "merge", "--ff-only", e.Integrado); err != nil {
			return fmt.Errorf("%w: %s avanzó mientras tanto; vuelve a correr el cierre e integra de nuevo", ErrBaseAvanzo, e.Base)
		}
		return nil
	}
	if wl, _ := Git(r.Root, "worktree", "list", "--porcelain"); strings.Contains(wl+"\n", "branch refs/heads/"+e.Base+"\n") {
		return fmt.Errorf("integrado y revalidado (%s), pero %s está abierta en otro worktree. Cámbiate a %s en el checkout principal y vuelve a correr el cierre", e.Integrado[:8], e.Base, e.Base)
	}
	tip, err := Git(r.Root, "rev-parse", e.Base)
	if err != nil {
		return err
	}
	if _, err := Git(r.Root, "merge-base", "--is-ancestor", tip, e.Integrado); err != nil {
		return fmt.Errorf("%w: %s avanzó mientras tanto; vuelve a correr el cierre e integra de nuevo", ErrBaseAvanzo, e.Base)
	}
	_, err = Git(r.Root, "update-ref", "refs/heads/"+e.Base, e.Integrado, tip)
	return err
}

// Siguiente es el número del próximo slice: uno más que los cerrados.
// Siguiente es el número del próximo slice: uno más que el mayor cerrado o abierto.
func (r Repo) Siguiente() string {
	n := 0
	for _, reg := range leerJSONL[Registro](r.logPath()) {
		var x int
		fmt.Sscanf(reg.NN, "%d", &x)
		n = max(n, x)
	}
	ab, _ := r.Abiertos()
	for _, e := range ab {
		var x int
		fmt.Sscanf(e.NN, "%d", &x)
		n = max(n, x)
	}
	return fmt.Sprintf("%02d", n+1)
}

// Huerfanos lista worktrees que no son el principal ni los del slice abierto.
func (r Repo) Huerfanos() ([]string, error) {
	out, err := Git(r.Root, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	ab, _ := r.Abiertos()
	propio := map[string]bool{r.Root: true}
	for _, e := range ab {
		propio[e.Worktree], propio[r.intPath(e)] = true, true
		for _, w := range e.Equipo {
			propio[w.Worktree] = true
		}
	}
	var res []string
	for _, l := range strings.Split(out, "\n") {
		p, ok := strings.CutPrefix(l, "worktree ")
		if !ok || propio[p] {
			continue
		}
		res = append(res, p)
	}
	return res, nil
}

// MaxEquipo: workers a la vez dentro de un slice.
const MaxEquipo = 4

var zonaRe = regexp.MustCompile(`^[a-z0-9-]{1,20}$`)

// Sumar agrega un worker al equipo del slice, con su rama y worktree propios desde la punta del slice.
func (r Repo) Sumar(nn, nombre, agente, zona string, rutas []string) (*Estado, *Worker, error) {
	soltar, err := r.bloqueo(nn)
	if err != nil {
		return nil, nil, err
	}
	defer soltar()
	e, err := r.Load(nn)
	if err != nil || e == nil {
		return nil, nil, errors.Join(err, errors.New("no hay slice abierto"))
	}
	switch {
	case e.Fase != Implementando:
		return e, nil, fmt.Errorf("%s está en %s; el equipo trabaja en %s", e.ID(), e.Fase, Implementando)
	case !zonaRe.MatchString(zona):
		return e, nil, errors.New("zona: minúsculas, números o guiones (front, back, test…)")
	case len(rutas) == 0:
		return e, nil, errors.New("di qué rutas son de esta zona (--rutas src/web,src/ui)")
	}
	var propio *Worker
	for i := range e.Equipo {
		w := e.Equipo[i]
		if w.Zona == zona {
			if w.Nombre != nombre {
				return e, nil, fmt.Errorf("la zona %s ya es de %s", zona, w.Nombre)
			}
			// La zona sigue con su worker: la tarea siguiente del plan reusa su rama y su worktree.
			propio = &e.Equipo[i]
			continue
		}
		for _, a := range w.Rutas {
			for _, b := range rutas {
				if dentro(a, b) || dentro(b, a) {
					return e, nil, fmt.Errorf("las rutas %s y %s se pisan con la zona %s: dos workers no escriben lo mismo", b, a, w.Zona)
				}
			}
		}
	}
	for _, b := range rutas {
		for _, f := range e.Bundle {
			if dentro(b, f) {
				return e, nil, fmt.Errorf("la ruta %s incluye el examen %s, que nadie edita", b, f)
			}
		}
	}
	if propio != nil && !existe(propio.Worktree) && r.podable(e, *propio) {
		// Ya podada: su trabajo está en el slice, así que la zona arranca de nuevo desde ahí.
		e.Equipo = slices.DeleteFunc(e.Equipo, func(w Worker) bool { return w.Zona == zona })
		propio = nil
	}
	if propio != nil {
		return r.reusar(e, propio, agente, rutas)
	}
	if len(e.Equipo) >= MaxEquipo {
		return e, nil, fmt.Errorf("%s ya tiene %d workers (tope %d)", e.ID(), len(e.Equipo), MaxEquipo)
	}
	w := Worker{Nombre: nombre, Agente: agente, Zona: zona, Rutas: rutas, Rama: e.Rama + "-" + zona, Worktree: e.Worktree + "-" + zona}
	if out, _ := Git(r.Root, "branch", "--list", w.Rama); out != "" || existe(w.Worktree) {
		return e, nil, fmt.Errorf("ya existe %s o %s; revisa con `orq doctor`", w.Rama, w.Worktree)
	}
	tip, err := Git(e.Worktree, "rev-parse", "HEAD")
	if err != nil {
		return e, nil, err
	}
	if _, err := Git(r.Root, "worktree", "add", w.Worktree, "-b", w.Rama, tip); err == nil {
		prepararDeps(r.Root, w.Worktree)
	} else {
		return e, nil, err
	}
	e.Equipo = append(e.Equipo, w)
	e.Escritor = "equipo"
	return e, &w, r.save(e)
}

// AmpliarZona suma rutas aprobadas a la zona de un worker ya lanzado (su tarea creció con un GO).
// No pisa otras zonas ni el examen; sin esto, juntar rechaza lo aprobado como «fuera de su zona».
func (r Repo) AmpliarZona(nn, zona string, rutas []string) (*Estado, error) {
	soltar, err := r.bloqueo(nn)
	if err != nil {
		return nil, err
	}
	defer soltar()
	e, err := r.Load(nn)
	if err != nil || e == nil {
		return nil, errors.Join(err, errors.New("no hay slice abierto"))
	}
	var w *Worker
	for i := range e.Equipo {
		if e.Equipo[i].Zona == zona {
			w = &e.Equipo[i]
		}
	}
	if w == nil {
		return e, fmt.Errorf("%s no tiene worker en la zona %s", e.ID(), zona)
	}
	for _, b := range rutas {
		for _, o := range e.Equipo {
			if o.Zona == zona {
				continue
			}
			for _, a := range o.Rutas {
				if dentro(a, b) || dentro(b, a) {
					return e, fmt.Errorf("las rutas %s y %s se pisan con la zona %s: dos workers no escriben lo mismo", b, a, o.Zona)
				}
			}
		}
		for _, f := range e.Bundle {
			if dentro(b, f) {
				return e, fmt.Errorf("la ruta %s incluye el examen %s, que nadie edita", b, f)
			}
		}
		nueva := true
		for _, a := range w.Rutas {
			nueva = nueva && !dentro(a, b)
		}
		if nueva {
			w.Rutas = append(w.Rutas, b)
		}
	}
	return e, r.save(e)
}

// reusar le da a la zona su tarea siguiente: suma las rutas nuevas y, si su worktree está limpio,
// lo avanza a la punta del slice para que vea lo que juntaron las otras zonas.
func (r Repo) reusar(e *Estado, w *Worker, agente string, rutas []string) (*Estado, *Worker, error) {
	if !existe(w.Worktree) {
		return e, nil, fmt.Errorf("la zona %s perdió su worktree %s; revisa con `orq doctor`", w.Zona, w.Worktree)
	}
	if st, _ := Git(w.Worktree, "status", "--porcelain"); st != "" {
		return e, nil, fmt.Errorf("%s tiene cambios sin commit; júntalos o deshazlos antes de la tarea siguiente", w.Worktree)
	}
	for _, b := range rutas {
		nueva := true
		for _, a := range w.Rutas {
			nueva = nueva && !dentro(a, b)
		}
		if nueva {
			w.Rutas = append(w.Rutas, b)
		}
	}
	if tip, err := Git(e.Worktree, "rev-parse", "HEAD"); err == nil {
		Git(w.Worktree, "merge", "--ff-only", "-q", tip) // si no avanza rápido, sigue desde su propia punta
	}
	w.Agente = agente
	return e, w, r.save(e)
}

// prepararDeps da a un worktree nuevo las dependencias ignoradas del repo: node_modules como copia con
// enlaces duros (las herramientas de node resuelven rutas reales) y .venv como enlace simbólico.
// Si el enlace no queda ignorado por git, se quita: un worktree sucio no se junta.
func prepararDeps(root, wt string) {
	for _, d := range []struct {
		nombre string
		duro   bool
	}{{"node_modules", true}, {".venv", false}} {
		src, dst := filepath.Join(root, d.nombre), filepath.Join(wt, d.nombre)
		if fi, err := os.Stat(src); err != nil || !fi.IsDir() {
			continue
		}
		if _, err := os.Lstat(dst); err == nil {
			continue
		}
		if d.duro {
			if exec.Command("cp", "-al", src, dst).Run() != nil {
				os.RemoveAll(dst) // copia a medias: mejor un enlace que nada
				os.Symlink(src, dst)
			}
		} else {
			os.Symlink(src, dst)
		}
		if _, err := Git(wt, "check-ignore", "-q", d.nombre); err != nil {
			if fi, _ := os.Lstat(dst); fi != nil && fi.Mode()&os.ModeSymlink != 0 {
				os.Remove(dst)
			} else {
				os.RemoveAll(dst)
			}
		}
	}
}

// dentro dice si la ruta p está dentro de base (o es la misma).
func dentro(base, p string) bool {
	base, p = strings.Trim(filepath.Clean(base), "/"), strings.Trim(filepath.Clean(p), "/")
	return base == "." || p == base || strings.HasPrefix(p, base+"/")
}

// Juntar integra en el worktree del slice lo nuevo de cada worker. Rechaza a quien tocó fuera de su zona
// o el examen, y se detiene ante un conflicto sin dejar nada a medias.
func (r Repo) Juntar(nn string) (*Estado, []string, error) {
	soltar, err := r.bloqueo(nn)
	if err != nil {
		return nil, nil, err
	}
	defer soltar()
	e, err := r.Load(nn)
	if err != nil || e == nil {
		return nil, nil, errors.Join(err, errors.New("no hay slice abierto"))
	}
	if len(e.Equipo) == 0 {
		return e, nil, errors.New("este slice no tiene equipo: el worker escribe directo en el worktree del slice")
	}
	if st, _ := Git(e.Worktree, "status", "--porcelain"); st != "" {
		return e, nil, errors.New("el worktree del slice tiene cambios sin commit; con equipo, nadie escribe ahí directo")
	}
	var informe []string
	var fallo error
	for i := range e.Equipo {
		w := &e.Equipo[i]
		if st, _ := Git(w.Worktree, "status", "--porcelain"); st != "" {
			informe = append(informe, fmt.Sprintf("%s: tiene cambios sin commit; no se junta", w.Nombre))
			continue
		}
		head, err := Git(w.Worktree, "rev-parse", "HEAD")
		if err != nil || head == w.Juntado {
			informe = append(informe, fmt.Sprintf("%s: nada nuevo", w.Nombre))
			continue
		}
		base, _ := Git(e.Worktree, "merge-base", "HEAD", head)
		cambios, _ := Git(e.Worktree, "diff", "--name-only", base, head)
		var fuera []string
		for _, f := range strings.Fields(cambios) {
			ok := false
			for _, ruta := range w.Rutas {
				ok = ok || dentro(ruta, f)
			}
			for _, b := range e.Bundle {
				if f == b {
					ok = false
				}
			}
			if !ok {
				fuera = append(fuera, f)
			}
		}
		if len(fuera) > 0 {
			informe = append(informe, fmt.Sprintf("%s: tocó fuera de su zona (%s); no se junta. Que lo deshaga o lo pida al dueño de esa zona", w.Nombre, strings.Join(fuera, ", ")))
			fallo = errors.New("hay workers fuera de su zona")
			continue
		}
		if _, err := Git(e.Worktree, "merge", "--no-ff", head, "-m", fmt.Sprintf("%s: junta %s (%s)", e.Attempt(), w.Zona, w.Nombre)); err != nil {
			Git(e.Worktree, "merge", "--abort")
			informe = append(informe, fmt.Sprintf("%s: conflicto al juntar; nada cambió. Que se coordinen por el canal", w.Nombre))
			fallo = errors.New("conflicto al juntar")
			continue
		}
		w.Juntado = head
		informe = append(informe, fmt.Sprintf("%s: juntado %s", w.Nombre, head[:8]))
	}
	return e, informe, errors.Join(r.save(e), fallo)
}

// limpiarEquipo quita los worktrees del equipo; las ramas solo si ya están en la base.
func (r Repo) limpiarEquipo(e *Estado, aceptado bool, now time.Time) []string {
	var q []string
	for _, w := range e.Equipo {
		if x := r.quitarWorktree(e, w.Worktree, now); x != "" {
			q = append(q, x)
		}
		if !aceptado {
			continue
		}
		if _, err := Git(r.Root, "merge-base", "--is-ancestor", w.Rama, e.Base); err == nil {
			sha, _ := Git(r.Root, "rev-parse", w.Rama)
			Git(r.Root, "update-ref", "-d", "refs/heads/"+w.Rama, sha)
		} else if b, _ := Git(r.Root, "branch", "--list", w.Rama); b != "" {
			appendJSON(r.CuarentenaPath(), Cuarentena{Rama: w.Rama, Slice: e.ID(), Motivo: "trabajo del worker no juntado en el slice", TS: now.UTC().Truncate(time.Second)})
			q = append(q, w.Rama)
		}
	}
	return q
}

// MaxArchivos es el tamaño a partir del cual un slice deja de ser "delgado" y se pregunta por qué.
const MaxArchivos = 25

// Alineacion compara lo que el slice realmente cambió con lo que la historia justifica.
type Alineacion struct {
	Cambiados []string
	Fuera     []string // archivos fuera del alcance y de las zonas del equipo
	Avisos    []string // vacío = alineado
}

// Alinear no bloquea: detecta desvío para que el Orquestador pregunte al Atena antes de seguir.
func (r Repo) Alinear(nn string) (*Estado, *Alineacion, error) {
	e, err := r.Load(nn)
	if err != nil {
		return nil, nil, err
	}
	permitido := append([]string{e.Test}, e.Bundle...)
	permitido = append(permitido, e.Alcance...)
	acotado := len(e.Alcance) > 0
	dirs := []string{e.Worktree}
	for _, w := range e.Equipo {
		permitido = append(permitido, w.Rutas...)
		acotado = acotado || len(w.Rutas) > 0
		dirs = append(dirs, w.Worktree)
	}
	a := &Alineacion{}
	visto := map[string]bool{}
	for _, d := range dirs {
		if !existe(d) {
			continue
		}
		out, _ := Git(d, "diff", "--name-only", e.SHARojo+"..HEAD")
		for _, f := range strings.Fields(out) {
			if visto[f] {
				continue
			}
			visto[f] = true
			a.Cambiados = append(a.Cambiados, f)
			if !acotado {
				continue
			}
			ok := false
			for _, p := range permitido {
				p = strings.TrimSuffix(filepath.Clean(p), "/")
				if f == p || strings.HasPrefix(f, p+"/") {
					ok = true
					break
				}
			}
			if !ok {
				a.Fuera = append(a.Fuera, f)
			}
		}
	}
	if len(a.Fuera) > 0 {
		a.Avisos = append(a.Avisos, fmt.Sprintf("%d archivo(s) fuera de lo que la historia justifica: %s", len(a.Fuera), strings.Join(cabeza(a.Fuera, 5), ", ")))
	}
	if len(a.Cambiados) > MaxArchivos {
		a.Avisos = append(a.Avisos, fmt.Sprintf("%d archivos cambiados: ya no es un slice delgado", len(a.Cambiados)))
	}
	return e, a, nil
}

func cabeza(l []string, n int) []string {
	if len(l) > n {
		return append(l[:n:n], fmt.Sprintf("… %d más", len(l)-n))
	}
	return l
}

// podable: la rama del worker ya está dentro del slice (o ya no existe). Su trabajo no se pierde.
func (r Repo) podable(e *Estado, w Worker) bool {
	if b, _ := Git(r.Root, "branch", "--list", w.Rama); b == "" {
		return true
	}
	_, err := Git(r.Root, "merge-base", "--is-ancestor", w.Rama, e.Rama)
	return err == nil
}

// Podar retira workers que terminaron: quita su worktree y su rama y los saca del equipo, solo si su
// trabajo ya está juntado en el slice y no dejaron cambios sin commit. Lo demás queda y se informa.
func (r Repo) Podar(nn string, nombres []string) (*Estado, []string, error) {
	soltar, err := r.bloqueo(nn)
	if err != nil {
		return nil, nil, err
	}
	defer soltar()
	e, err := r.Load(nn)
	if err != nil || e == nil {
		return nil, nil, errors.Join(err, errors.New("no hay slice abierto"))
	}
	var informe []string
	for _, nombre := range nombres {
		i := slices.IndexFunc(e.Equipo, func(w Worker) bool { return w.Nombre == nombre })
		if i < 0 {
			informe = append(informe, nombre+": no está en el equipo")
			continue
		}
		w := e.Equipo[i]
		if existe(w.Worktree) {
			if st, _ := Git(w.Worktree, "status", "--porcelain"); st != "" {
				informe = append(informe, nombre+": tiene cambios sin commit; no se poda")
				continue
			}
		}
		if !r.podable(e, w) {
			informe = append(informe, nombre+": su rama no está juntada en el slice; no se poda")
			continue
		}
		if existe(w.Worktree) {
			if _, err := Git(r.Root, "worktree", "remove", w.Worktree); err != nil {
				informe = append(informe, nombre+": no se pudo quitar el worktree: "+err.Error())
				continue
			}
		}
		if sha, err := Git(r.Root, "rev-parse", "--verify", "-q", "refs/heads/"+w.Rama); err == nil && sha != "" {
			Git(r.Root, "update-ref", "-d", "refs/heads/"+w.Rama, sha)
		}
		e.Equipo = slices.Delete(e.Equipo, i, i+1)
		informe = append(informe, nombre+": podado (worktree y rama "+w.Rama+")")
	}
	return e, informe, r.save(e)
}
