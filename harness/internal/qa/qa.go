// Package qa es el ciclo de QA independiente: el tester corre la pirámide de pruebas
// y registra hallazgos; un hallazgo solo cuenta si orq reproduce el fallo.
// El rigor es proporcional a la criticidad del proyecto.
package qa

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
}

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
		c.Layers, c.RequireFrom = q.Layers, q.RequireFrom
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
	Result      string        `json:"result"` // pass|fail
	TS          time.Time     `json:"ts"`
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
			lr.Skipped, lr.Reason = true, fmt.Sprintf("criticidad %s < %s", rec.Criticality, normalizar(l.MinCriticality))
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
