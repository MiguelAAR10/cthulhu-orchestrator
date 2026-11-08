// Package qa es el ciclo de QA independiente: el tester corre la pirámide de pruebas
// y registra hallazgos; un hallazgo solo cuenta si orq reproduce el fallo.
// El rigor es proporcional a la criticidad del proyecto.
package qa

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"orq/internal/lang"
	"orq/internal/perfil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Runner corre un comando en dir y devuelve salida y código de salida. Se inyecta en pruebas.
type Runner func(dir, cmd string, timeout time.Duration) (string, int)

// ShellRunner corre con bash -c y un tiempo máximo.
func ShellRunner(dir, cmd string, timeout time.Duration) (string, int) {
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	c := exec.CommandContext(ctx, "bash", "-c", cmd)
	c.Dir = dir
	c.WaitDelay = time.Second
	out, err := c.CombinedOutput()
	if ctx.Err() != nil {
		return string(out) + "\n[orq] tiempo agotado", 124
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return string(out), ee.ExitCode()
	}
	if err != nil {
		return string(out) + err.Error(), 127
	}
	return string(out), 0
}

// Niveles de criticidad, de menor a mayor.
var niveles = map[string]int{"low": 0, "medium": 1, "high": 2}

// normalizar acepta sinónimos; lo desconocido es medium.
func normalizar(c string) string {
	switch strings.ToLower(strings.TrimSpace(c)) {
	case "low", "trivial", "baja", "small", "prototype":
		return "low"
	case "high", "alta", "large", "critical", "production-critical":
		return "high"
	}
	return "medium"
}

func alMenos(c, min string) bool { return niveles[normalizar(c)] >= niveles[normalizar(min)] }

// Layer es una capa de la pirámide. MinCriticality vacía = siempre corre.
type Layer struct {
	Name           string `json:"name"`
	Cmd            string `json:"cmd"`
	MinCriticality string `json:"min_criticality,omitempty"`
	TimeoutSeg     int    `json:"timeout_seg,omitempty"`
}

type Config struct {
	Criticality string  `json:"-"`
	Note        string  `json:"-"` // aviso de deprecación si la criticidad vino de config.json
	Layers      []Layer `json:"layers"`
	RequireFrom string  `json:"require_receipt_from,omitempty"` // desde qué criticidad se exige recibo (medium)
	// Reports: archivos que las capas pueden escribir en el worktree (globs); out.tap siempre vale.
	Reports []string `json:"reports,omitempty"`
}

// Permitidos: los informes que una corrida de QA puede dejar en el worktree.
func (c Config) Permitidos() []string { return append([]string{"out.tap"}, c.Reports...) }

func (c Config) Crit() string { return normalizar(c.Criticality) }

// LoadConfig lee la pirámide de .orq/config.json; la criticidad viene de perfil.Criticality (profile.json gana).
func LoadConfig(orqDir string) Config {
	var c Config
	c.Criticality, c.Note = perfil.Criticality(orqDir)
	raw, err := os.ReadFile(filepath.Join(orqDir, "config.json"))
	if err != nil {
		return c
	}
	var m map[string]json.RawMessage
	if json.Unmarshal(raw, &m) != nil {
		return c
	}
	var q Config
	if json.Unmarshal(m["qa"], &q) == nil {
		c.Layers, c.RequireFrom, c.Reports = q.Layers, q.RequireFrom, q.Reports
	}
	return c
}

// LayerResult: lo que pasó en una capa.
type LayerResult struct {
	Name    string `json:"name"`
	Cmd     string `json:"cmd"`
	Skipped bool   `json:"skipped,omitempty"`
	Reason  string `json:"reason,omitempty"`
	Exit    int    `json:"exit"`
	Passed  int    `json:"passed"`
	Failed  int    `json:"failed"`
	Tail    string `json:"tail,omitempty"`
}

// Receipt es el recibo QA de un slice.
type Receipt struct {
	Slice       string        `json:"slice"`
	Head        string        `json:"head"`
	Criticality string        `json:"criticality"`
	Layers      []LayerResult `json:"layers"`
	Result      string        `json:"result"` // pass|fail|invalid
	TS          time.Time     `json:"ts"`
	// Tree: HEAD y git status del worktree antes y después de las capas. Si el tester cambió algo
	// fuera de los informes permitidos, el recibo es invalid: no prueba el código que se entregó.
	Tree *TreeCheck `json:"tree,omitempty"`
	// Isolation: la ejecución en una copia desechable y lo vigilado fuera de ella (ver RunAislado).
	Isolation *Vigilancia `json:"isolation,omitempty"`
}

type TreeCheck struct {
	Before  Arbol    `json:"before"`
	After   Arbol    `json:"after"`
	Changed []string `json:"changed,omitempty"`
}

// Arbol es la foto estructural de un worktree: HEAD más `git status --porcelain` (tracked y
// untracked no ignorados), sin los informes permitidos.
type Arbol struct {
	Head   string   `json:"head"`
	Status []string `json:"status"`
}

// FotoArbol toma la foto; fuera de un repo git devuelve un árbol vacío (Head "").
func FotoArbol(dir string, permitidos []string) Arbol {
	var a Arbol
	h, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		return a
	}
	a.Head = strings.TrimSpace(string(h))
	out, _ := exec.Command("git", "-C", dir, "status", "--porcelain=v1", "-z", "--untracked-files=all").Output()
	for _, e := range strings.Split(string(out), "\x00") {
		if len(e) < 4 {
			continue
		}
		ruta := e[3:]
		if permitido(ruta, permitidos) {
			continue
		}
		// El contenido cuenta, no solo el estado: un archivo ya sucio que el tester reescribe cambia.
		sum := ""
		if raw, err := os.ReadFile(filepath.Join(dir, ruta)); err == nil {
			sum = fmt.Sprintf(" %x", sha256.Sum256(raw))
		}
		a.Status = append(a.Status, e[:2]+" "+ruta+sum)
	}
	return a
}

func permitido(ruta string, pats []string) bool {
	for _, p := range pats {
		if ok, _ := filepath.Match(p, ruta); ok || ruta == p {
			return true
		}
		if ok, _ := filepath.Match(p, filepath.Base(ruta)); ok && !strings.Contains(p, "/") {
			return true
		}
	}
	return false
}

// Diferencias entre dos fotos.
func Diferencias(a, b Arbol) []string {
	var d []string
	if a.Head != b.Head {
		d = append(d, fmt.Sprintf("HEAD %.12s -> %.12s", a.Head, b.Head))
	}
	en := func(xs []string) map[string]bool {
		m := map[string]bool{}
		for _, x := range xs {
			m[x] = true
		}
		return m
	}
	ma, mb := en(a.Status), en(b.Status)
	for _, x := range b.Status {
		if !ma[x] {
			d = append(d, "+ "+x)
		}
	}
	for _, x := range a.Status {
		if !mb[x] {
			d = append(d, "- "+x)
		}
	}
	return d
}

// RunVigilado corre la pirámide entre dos fotos del worktree; si las capas cambiaron algo fuera de
// los informes permitidos, el recibo queda invalid.
func RunVigilado(dir, nn string, layers []Layer, crit string, run Runner, now time.Time, permitidos []string) Receipt {
	antes := FotoArbol(dir, permitidos)
	rec := Run(dir, nn, antes.Head, layers, crit, run, now)
	despues := FotoArbol(dir, permitidos)
	rec.Tree = &TreeCheck{Before: antes, After: despues, Changed: Diferencias(antes, despues)}
	if len(rec.Tree.Changed) > 0 {
		rec.Result = "invalid"
	}
	return rec
}

var (
	nPasaRe  = regexp.MustCompile(`(?i)(\d+)\s+(passed|passing)`)
	nFallaRe = regexp.MustCompile(`(?i)(\d+)\s+(failed|failing)`)
	goPasaRe = regexp.MustCompile(`(?m)^\s*--- PASS`)
	goFallRe = regexp.MustCompile(`(?m)^\s*--- FAIL`)
)

// contar extrae pasadas y fallidas de la salida (go test -v, jest/vitest/playwright/pytest).
func contar(out string, exit int) (int, int) {
	suma := func(re *regexp.Regexp) (int, bool) {
		ms := re.FindAllStringSubmatch(out, -1)
		t := 0
		for _, m := range ms {
			n, _ := strconv.Atoi(m[1])
			t += n
		}
		return t, len(ms) > 0
	}
	p, okp := suma(nPasaRe)
	f, okf := suma(nFallaRe)
	if okp || okf {
		return p, f
	}
	p, f = len(goPasaRe.FindAllString(out, -1)), len(goFallRe.FindAllString(out, -1))
	if p+f > 0 {
		return p, f
	}
	if exit == 0 {
		return 1, 0 // sin contadores: la capa cuenta como una comprobación
	}
	return 0, 1
}

func cola(s string, n int) string {
	l := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(l) > n {
		l = l[len(l)-n:]
	}
	return strings.Join(l, "\n")
}

func id(nn string) string { return "S-" + strings.TrimPrefix(nn, "S-") }

// Run corre la pirámide en orden. Las capas sobre la criticidad del proyecto quedan omitidas.
func Run(dir, nn, head string, layers []Layer, crit string, run Runner, now time.Time) Receipt {
	rec := Receipt{Slice: id(nn), Head: head, Criticality: normalizar(crit), Result: "pass", TS: now.UTC().Truncate(time.Second)}
	if len(layers) == 0 {
		rec.Result = "fail"
	}
	for _, l := range layers {
		lr := LayerResult{Name: l.Name, Cmd: l.Cmd}
		if l.MinCriticality != "" && !alMenos(crit, l.MinCriticality) {
			lr.Skipped, lr.Reason = true, fmt.Sprintf(lang.T("criticality %s < %s", "criticidad %s < %s"), rec.Criticality, normalizar(l.MinCriticality))
			rec.Layers = append(rec.Layers, lr)
			continue
		}
		out, code := run(dir, l.Cmd, time.Duration(l.TimeoutSeg)*time.Second)
		lr.Exit, lr.Tail = code, cola(out, 10)
		lr.Passed, lr.Failed = contar(out, code)
		if code != 0 || lr.Failed > 0 {
			rec.Result = "fail"
		}
		rec.Layers = append(rec.Layers, lr)
	}
	return rec
}

func dirDe(orqDir, nn string) string { return filepath.Join(orqDir, "qa", id(nn)) }

// Guardar escribe el recibo QA del slice.
func Guardar(orqDir string, rec Receipt) error {
	d := dirDe(orqDir, rec.Slice)
	if err := os.MkdirAll(d, 0o755); err != nil {
		return err
	}
	raw, _ := json.MarshalIndent(rec, "", "  ")
	return os.WriteFile(filepath.Join(d, "receipt.json"), append(raw, '\n'), 0o644)
}

// Recibo lee el recibo QA vigente, si existe.
func Recibo(orqDir, nn string) (*Receipt, error) {
	raw, err := os.ReadFile(filepath.Join(dirDe(orqDir, nn), "receipt.json"))
	if err != nil {
		return nil, err
	}
	var r Receipt
	return &r, json.Unmarshal(raw, &r)
}

// Finding es un hallazgo del tester. Sin todos los campos no se acepta.
type Finding struct {
	Requirement string    `json:"requirement"`
	Commit      string    `json:"commit"`
	Repro       string    `json:"repro"`
	Observed    string    `json:"observed"`
	Expected    string    `json:"expected"`
	Impact      string    `json:"impact"`
	Dir         string    `json:"-"`
	Confirmed   bool      `json:"confirmed"`
	ReproExit   int       `json:"repro_exit"`
	TS          time.Time `json:"ts"`
}

func (f Finding) Validar() error {
	var falta []string
	for _, c := range []struct{ n, v string }{{"requirement", f.Requirement}, {"commit", f.Commit}, {"repro", f.Repro},
		{"observed", f.Observed}, {"expected", f.Expected}, {"impact", f.Impact}} {
		if strings.TrimSpace(c.v) == "" {
			falta = append(falta, c.n)
		}
	}
	if len(falta) > 0 {
		return fmt.Errorf("hallazgo incompleto, falta: %s", strings.Join(falta, ", "))
	}
	return nil
}

// AddFinding valida, re-ejecuta el repro y guarda. Confirmado solo si el repro falla.
func AddFinding(orqDir, nn string, f Finding, run Runner, now time.Time) (Finding, error) {
	if err := f.Validar(); err != nil {
		return f, err
	}
	dir := f.Dir
	if dir == "" {
		dir = "."
	}
	_, f.ReproExit = run(dir, f.Repro, 2*time.Minute)
	f.Confirmed, f.TS = f.ReproExit != 0, now.UTC().Truncate(time.Second)
	d := dirDe(orqDir, nn)
	if err := os.MkdirAll(d, 0o755); err != nil {
		return f, err
	}
	fh, err := os.OpenFile(filepath.Join(d, "findings.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return f, err
	}
	defer fh.Close()
	raw, _ := json.Marshal(f)
	_, err = fh.Write(append(raw, '\n'))
	return f, err
}

func Findings(orqDir, nn string) []Finding {
	fh, err := os.Open(filepath.Join(dirDe(orqDir, nn), "findings.jsonl"))
	if err != nil {
		return nil
	}
	defer fh.Close()
	var out []Finding
	sc := bufio.NewScanner(fh)
	for sc.Scan() {
		var f Finding
		if json.Unmarshal(sc.Bytes(), &f) == nil {
			out = append(out, f)
		}
	}
	return out
}

func Confirmados(fs []Finding) int {
	n := 0
	for _, f := range fs {
		if f.Confirmed {
			n++
		}
	}
	return n
}

// PuedeCerrar: desde la criticidad configurada (medium por defecto) el cierre exige
// un recibo QA verde y ningún hallazgo confirmado posterior al recibo.
func PuedeCerrar(orqDir, nn string, c Config) error {
	from := c.RequireFrom
	if from == "" {
		from = "medium"
	}
	if !alMenos(c.Crit(), from) {
		return nil
	}
	rec, err := Recibo(orqDir, nn)
	if err != nil {
		return fmt.Errorf("%s: criticidad %s exige recibo QA; corre `orq qa run %s`", id(nn), c.Crit(), id(nn))
	}
	if rec.Result != "pass" {
		return fmt.Errorf("%s: el recibo QA es %s; repara y corre `orq qa run` de nuevo", id(nn), rec.Result)
	}
	for _, f := range Findings(orqDir, nn) {
		if f.Confirmed && !f.TS.Before(rec.TS) {
			return fmt.Errorf("%s: hallazgo confirmado abierto (%s); repara y corre `orq qa run` de nuevo", id(nn), f.Requirement)
		}
	}
	return nil
}
