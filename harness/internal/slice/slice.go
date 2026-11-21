// Package slice lleva el único slice abierto como una máquina de estados.
// El agente pide la transición; este paquete la valida. Nada se borra a la fuerza:
// lo dudoso queda en cuarentena y el cierre se puede repetir hasta terminar.
package slice

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"orq/internal/config"
	"orq/internal/gitx"
	"orq/internal/sysinfo"
)

// EsquemaActual marca los estados que abre esta versión: el worker trabaja en un clon independiente,
// el examen corre siempre en la caja y el contrato vive también en git (refs/orq/contrato/NN).
// A state without a schema (0) was opened by orq 1.0.0: this version refuses it (Load), it is
// closed with orq 1.0.0.
const EsquemaActual = 2

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
		morir(etapa)
	}
}

// morir ends the process at a simulated interruption. Tests replace it to stop the call in place.
var morir = func(etapa string) {
	fmt.Fprintf(os.Stderr, "[orq] interrupción simulada en %q\n", etapa)
	os.Exit(9)
}

// tras lets a test act at a named point of a long command (another process writing meanwhile).
// It does nothing outside tests.
var tras = func(punto string) {}

// ErrBaseAvanzo: la rama base cambió durante el cierre; hay que integrar de nuevo.
var ErrBaseAvanzo = errors.New("la base avanzó")

// MaxFallos: verificaciones fallidas por intento antes de bloquear (la primera más 2 reparaciones).
const MaxFallos = 3

// MaxIntentos: a partir de aquí, un intento nuevo lo decide el Owner.
const MaxIntentos = 3

// Version identifica al verificador en los recibos.
var Version = "orq"

type Estado struct {
	NN          string     `json:"nn"`
	Intento     int        `json:"intento"`
	Fase        string     `json:"fase"`
	Titulo      string     `json:"titulo"`
	Test        string     `json:"test"`
	Bundle      []string   `json:"bundle"` // examen + config del runner + fixtures
	Cmd         string     `json:"cmd"`
	Sano        string     `json:"sano,omitempty"`       // comando de base sana
	Base        string     `json:"base"`                 // rama donde se integra
	SHARojo     string     `json:"sha_rojo"`             // base_sha
	SHAExamen   string     `json:"sha_examen,omitempty"` // commit donde se aprobó el examen vigente (si se sumó o corrigió)
	Rama        string     `json:"rama"`
	Worktree    string     `json:"worktree"`
	Procedencia string     `json:"procedencia"` // quién creó el worktree: git|herdr|orca
	Host        string     `json:"host"`
	Escritor    string     `json:"escritor,omitempty"` // writer lease
	Abierto     time.Time  `json:"abierto"`
	Huella      string     `json:"huella"`
	SalidaRojo  string     `json:"salida_rojo"`
	Fallos      int        `json:"fallos,omitempty"`
	HeadFallo   string     `json:"head_fallo,omitempty"`
	FirmaFallo  string     `json:"firma_fallo,omitempty"`
	Reporte     string     `json:"reporte,omitempty"` // archivo que escribe el runner
	Formato     string     `json:"formato,omitempty"` // tap|junit|gotest-json
	Omitir      bool       `json:"permite_omitir,omitempty"`
	Esquema     int        `json:"esquema,omitempty"`    // 0 = abierto por orq 1.0.0; EsquemaActual = abierto por esta versión
	SinConteo   string     `json:"sin_conteo,omitempty"` // por qué este slice no cuenta pruebas (sin reporte del runner)
	EnSitio     string     `json:"en_sitio,omitempty"`   // por qué el examen corre en el worktree y no en una copia aislada
	Vigilados   []Vigilado `json:"vigilados,omitempty"`  // archivos del runner y su hash en el rojo
	// PruebasExamen: las pruebas que fallaban en el rojo, por nombre del reporte; verificar las exige pasadas.
	PruebasExamen []string `json:"pruebas_examen,omitempty"`
	Ruta          string   `json:"ruta,omitempty"`      // fix|slice ("" = slice)
	Tipo          string   `json:"tipo,omitempty"`      // tipo de tarea para elegir skills ("" = el stack)
	Ascendido     bool     `json:"ascendido,omitempty"` // un fix que subió a slice
	RutaMotivo    string   `json:"ruta_motivo,omitempty"`
	TimeoutSeg    int      `json:"timeout_seg,omitempty"`
	Recibo        *Recibo  `json:"recibo,omitempty"`
	Integrado     string   `json:"integrado,omitempty"` // SHA del merge revalidado
	Equipo        []Worker `json:"equipo,omitempty"`    // workers en paralelo dentro del slice
	Motivo        string   `json:"motivo,omitempty"`    // por qué está bloqueado
	Alcance       []string `json:"alcance,omitempty"`   // rutas que la historia justifica tocar
	Demo          []string `json:"demo,omitempty"`      // hasta 3 pasos para ver el resultado funcionando
	Tarjeta       *Tarjeta `json:"tarjeta,omitempty"`   // ficha de entendimiento que aprueba el Owner
	Historia      string   `json:"historia,omitempty"`  // story this slice is a child of
	// Cierre is the Owner's close decision, recorded before anything is done (orq2 only), so a re-run
	// after a crash resumes that same decision.
	Cierre *IntencionCierre `json:"cierre,omitempty"`
}

// IntencionCierre is a close the Owner asked for: accepted or rejected, why, and when.
type IntencionCierre struct {
	Resultado string    `json:"resultado"` // aceptado | rechazado
	Motivo    string    `json:"motivo,omitempty"`
	TS        time.Time `json:"ts"`
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
	Aislado    bool      `json:"aislado"`   // corrió en una copia limpia del commit
	Ejecutados int       `json:"ejecutados"`
	Omitidos   int       `json:"omitidos"`
	Fallidos   int       `json:"fallidos"`
	Artefacto  string    `json:"artefacto,omitempty"` // salida completa
	Nota       string    `json:"nota,omitempty"`
	// Checks son lint, tipos, pruebas y guardias del proyecto, corridos sobre el mismo commit.
	Checks []ResultadoCheck `json:"checks,omitempty"`
	// DefinicionesChecks: archivos que definen lo que corre un check y que el slice cambió. El cierre
	// aceptado pide --acepto-checks.
	DefinicionesChecks []string `json:"definiciones_checks,omitempty"`
	// TestsBorrados: archivos de prueba que el slice borró o renombró.
	TestsBorrados []string `json:"tests_borrados,omitempty"`
}

// Huella une el contenido de cada archivo del paquete del examen y el comando.
func Huella(dir string, bundle []string, cmd string) (string, error) {
	var datos [][]byte
	for _, f := range bundle {
		raw, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			return "", err
		}
		datos = append(datos, raw)
	}
	return huellaDe(bundle, datos, cmd), nil
}

func huellaDe(bundle []string, datos [][]byte, cmd string) string {
	h := sha256.New()
	for i, f := range bundle {
		fmt.Fprintf(h, "%s\x00%d\x00", f, len(datos[i]))
		h.Write(datos[i])
	}
	h.Write([]byte(cmd))
	return hex.EncodeToString(h.Sum(nil))
}

// HuellaEnCommit calcula la huella sobre el contenido commiteado en sha del repo de dir, no sobre una
// copia de trabajo: una edición sin commit (o hecha mientras corre el examen) no puede colarse.
func HuellaEnCommit(dir, sha string, bundle []string, cmd string) (string, error) {
	var datos [][]byte
	for _, f := range bundle {
		raw, err := gitx.Bytes(context.Background(), dir, "show", sha+":"+f)
		if err != nil {
			return "", fmt.Errorf("no leo %s en %s: %v", f, sha, err)
		}
		datos = append(datos, raw)
	}
	return huellaDe(bundle, datos, cmd), nil
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
	Worktree  string         // carpeta base de las copias de trabajo de los workers
	MaxSlices int            // abiertos a la vez; 0 = MaxSlicesDefecto
	Caja      Caja           // sandbox donde corre todo lo que escribió un worker; nil = bubblewrap
	Checks    []config.Check // del proyecto, en orden; vacío = no se corre ninguno
	// Definiciones son archivos que el proyecto suma a los que definen lo que corre un check.
	Definiciones []string
	// Carpetas son las subcarpetas con checks propios (corren dentro de ellas) o exentas.
	Carpetas []config.Carpeta
	// Cobertura (orq2): una carpeta tocada sin checks ni exención no abre ni verifica, y los checks
	// se vuelven a correr sobre el commit de merge al integrar.
	Cobertura bool
}

func (r Repo) dirSlices() string           { return filepath.Join(r.OrqDir, "slices") }
func (r Repo) estadoPath(nn string) string { return filepath.Join(r.dirSlices(), nn+".json") }
func (r Repo) viejoPath() string           { return filepath.Join(r.OrqDir, "slice.json") } // v0.4
func (r Repo) logPath() string             { return filepath.Join(r.OrqDir, "slices.jsonl") }
func (r Repo) recibosPath() string         { return filepath.Join(r.OrqDir, "recibos.jsonl") }
func (r Repo) CuarentenaPath() string      { return filepath.Join(r.OrqDir, "cuarentena.jsonl") }

// Git corre git en dir con tiempo máximo. El paquete lo usa solo para el repo principal (r.git);
// queda exportado para las pruebas y para quien necesite leerlo.
func Git(dir string, args ...string) (string, error) {
	return gitx.Salida(context.Background(), dir, args...)
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

// ErrNoAbierto: the slice has no state file, so it is closed or was never opened.
var ErrNoAbierto = errors.New("no está abierto")

// Load devuelve el slice nn; con nn vacío, el único abierto (nil si no hay ninguno).
func (r Repo) Load(nn ...string) (*Estado, error) {
	if len(nn) > 0 && nn[0] != "" {
		if err := r.migrar(); err != nil {
			return nil, err
		}
		e, err := leerEstado(r.estadoPath(strings.TrimPrefix(nn[0], "S-")))
		if errors.Is(err, os.ErrNotExist) {
			//lint:ignore ST1005 the message starts with a slice id (S-NN), uppercase by design
			return nil, fmt.Errorf("S-%s %w", strings.TrimPrefix(nn[0], "S-"), ErrNoAbierto)
		}
		if err != nil {
			return nil, err
		}
		if err := e.deEstaVersion(); err != nil {
			return nil, err
		}
		return e, nil
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
	if err := ab[0].deEstaVersion(); err != nil {
		return nil, err
	}
	return ab[0], nil
}

// deEstaVersion refuses a state opened by orq 1.0.0 (schema 0): v2 does not inherit those slices.
func (e *Estado) deEstaVersion() error {
	if e.Esquema == 0 {
		return fmt.Errorf("%w: %s was opened by orq 1.0.0; close it with orq 1.0.0 before using this version", ErrOrq1, e.ID())
	}
	return nil
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
	Aislado          bool     // obsoleto, sin efecto: la copia aislada es el único modo
	SinConteo        string   // motivo para abrir sin --reporte/--formato (el runner no puede dejar reporte)
	EnSitio          string   // ya no existe: el examen corre siempre en la caja; decirlo es un error
	Ruta             string   // fix|slice; vacío = slice
	Tipo             string   // tipo de tarea con que se eligen las skills del worker; vacío = el stack
	Timeout          int      // segundos; 0 = 600
	Alcance          []string // rutas que la historia justifica; fuera de ellas, alarma de desalineación
	Demo             []string // pasos para ver el resultado funcionando; la página de decisión los muestra
	Historia         string   // story the slice is a child of; it must exist
}

func (e *Estado) limite() time.Duration {
	if e.TimeoutSeg > 0 {
		return time.Duration(e.TimeoutSeg) * time.Second
	}
	return TimeoutDefecto
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

// baseExamen es el commit contra el que el examen no puede cambiar: el rojo, o el último examen aprobado.
func (e *Estado) baseExamen() string {
	if e.SHAExamen != "" {
		return e.SHAExamen
	}
	return e.SHARojo
}

func (r Repo) bloquear(e *Estado, motivo string) error {
	e.Fase, e.Motivo, e.Escritor = Bloqueado, motivo, ""
	if err := r.save(e); err != nil {
		return err
	}
	return fmt.Errorf("%s BLOQUEADO: %s. Decide el Orquestador (o el Atena): `orq slice reintentar --causa worker|examen \"qué cambia\"` o cerrar como rechazado", e.ID(), motivo)
}

func firmaDe(out string) string {
	h := sha256.Sum256([]byte(strings.Join(strings.Fields(cola(out, 30)), " ")))
	return hex.EncodeToString(h[:8])
}

type Registro struct {
	NN         string    `json:"nn"`
	Intento    int       `json:"intento"`
	Titulo     string    `json:"titulo"`
	Result     string    `json:"resultado"`
	Motivo     string    `json:"motivo,omitempty"`
	SHA        string    `json:"sha,omitempty"`
	Abierto    time.Time `json:"abierto"`
	Cerrado    time.Time `json:"cerrado"`
	Ruta       string    `json:"ruta,omitempty"`        // fix|slice con la que se cerró
	Ascendido  bool      `json:"ascendido,omitempty"`   // empezó como fix y subió a slice
	RutaMotivo string    `json:"ruta_motivo,omitempty"` // por qué subió
	Historia   string    `json:"historia,omitempty"`
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

// Cerrar con aceptado integra en un worktree aparte, revalida, publica la rama base y limpia.
// Cada paso queda guardado: si se interrumpe, volver a correrlo continúa donde quedó.
// ErrYaCerrado: otro cierre del mismo slice terminó primero; repetir no hace nada.
var ErrYaCerrado = errors.New("el slice ya estaba cerrado")

// bloqueo serializa los cambios de estado de un slice.
func (r Repo) bloqueo(nn string) (func(), error) {
	if nn == "" {
		if e, err := r.Load(); err == nil && e != nil {
			nn = e.NN
		}
	}
	return r.candado("slice-" + strings.TrimPrefix(nn, "S-"))
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

// MaxEquipo: workers a la vez dentro de un slice.
const MaxEquipo = 4

var zonaRe = regexp.MustCompile(`^[a-z0-9-]{1,20}$`)

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

// dentro dice si la ruta p está dentro de base (o es la misma).
func dentro(base, p string) bool {
	base, p = strings.Trim(filepath.Clean(base), "/"), strings.Trim(filepath.Clean(p), "/")
	return base == "." || p == base || strings.HasPrefix(p, base+"/")
}

// MaxArchivos es el tamaño a partir del cual un slice deja de ser "delgado" y se pregunta por qué.
const MaxArchivos = 25

// Alineacion compara lo que el slice realmente cambió con lo que la historia justifica.
type Alineacion struct {
	Cambiados []string
	Fuera     []string // archivos fuera del alcance y de las zonas del equipo
	Avisos    []string // vacío = alineado
}

func cabeza(l []string, n int) []string {
	if len(l) > n {
		return append(l[:n:n], fmt.Sprintf("… %d más", len(l)-n))
	}
	return l
}

// AbiertosOrq1 lists, without migrating or writing anything, the slices in orqDir that orq 1.0.0
// opened (schema 0), including a v0.4 slice.json not yet moved to slices/.
func AbiertosOrq1(orqDir string) []string {
	r := Repo{OrqDir: orqDir}
	m, _ := filepath.Glob(filepath.Join(r.dirSlices(), "*.json"))
	slices.Sort(m)
	if existe(r.viejoPath()) {
		m = append(m, r.viejoPath())
	}
	var ids []string
	for _, f := range m {
		if e, err := leerEstado(f); err == nil && e.Esquema == 0 && !slices.Contains(ids, e.ID()) {
			ids = append(ids, e.ID())
		}
	}
	return ids
}
