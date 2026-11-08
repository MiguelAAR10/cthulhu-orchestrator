// Package stops guarda los pedidos de los agentes al Owner (stoppers): credenciales, claves,
// decisiones, confirmaciones y accesos. .orq/stops.jsonl es solo de agregar y se pliega por id.
// Nunca guarda un secreto: rechaza el texto que lo parece y solo registra dónde quedó.
package stops

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"orq/internal/config"
	"orq/internal/orqdir"
)

var (
	Types      = []string{"credential", "api_key", "decision", "confirmation", "access"}
	Severities = []string{"blocking", "soon", "fyi"}
	envName    = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)
	prefixes   = []string{"sk-", "ghp_", "xox", "AKIA", "AIza", "-----BEGIN"}
)

// ErrSecret es el rechazo de texto que parece un secreto (exit 2 en la CLI).
var ErrSecret = errors.New("looks like a secret; say where it is, not what it is")

// Event es una línea de stops.jsonl.
type Event struct {
	ID         string    `json:"id"`
	TS         time.Time `json:"ts"`
	Event      string    `json:"event"` // raised | resolved | cancelled
	Type       string    `json:"type,omitempty"`
	Severity   string    `json:"severity,omitempty"`
	Slice      string    `json:"slice,omitempty"`
	Tasks      []string  `json:"tasks,omitempty"`
	By         string    `json:"by,omitempty"`
	Text       string    `json:"text,omitempty"`
	Options    []string  `json:"options,omitempty"`
	Where      string    `json:"where,omitempty"`
	Provided   string    `json:"provided,omitempty"`
	ProvidedAt string    `json:"provided_at,omitempty"`
	Choice     string    `json:"choice,omitempty"`
	Note       string    `json:"note,omitempty"`
}

// Stop es el estado plegado de un id.
type Stop struct {
	ID         string    `json:"id"`
	State      string    `json:"state"` // open | resolved | cancelled
	Type       string    `json:"type"`
	Severity   string    `json:"severity"`
	Slice      string    `json:"slice"`
	Tasks      []string  `json:"tasks,omitempty"`
	By         string    `json:"by,omitempty"`
	Text       string    `json:"text"`
	Options    []string  `json:"options,omitempty"`
	Where      string    `json:"where,omitempty"`
	Raised     time.Time `json:"raised"`
	Closed     time.Time `json:"closed,omitzero"`
	ClosedBy   string    `json:"closed_by,omitempty"`
	Provided   string    `json:"provided"`
	ProvidedAt string    `json:"provided_at,omitempty"`
	Choice     string    `json:"choice,omitempty"`
	Note       string    `json:"note,omitempty"`
}

type Raise struct {
	Type, Severity, Slice, By, Text, Where string
	Tasks, Options                         []string
}

type Resolution struct {
	By, ProvidedAt, Choice, Note string
}

func Path(orqDir string) string { return filepath.Join(orqDir, "stops.jsonl") }

func Load(path string) ([]Stop, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []Stop
	idx := map[string]int{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		var e Event
		if json.Unmarshal(bytes.TrimSpace(sc.Bytes()), &e) != nil || e.ID == "" {
			continue
		}
		i, ok := idx[e.ID]
		switch {
		case e.Event == "raised" && !ok:
			idx[e.ID] = len(out)
			out = append(out, Stop{ID: e.ID, State: "open", Type: e.Type, Severity: e.Severity, Slice: e.Slice,
				Tasks: e.Tasks, By: e.By, Text: e.Text, Options: e.Options, Where: e.Where, Raised: e.TS, Provided: "no"})
		case ok && (e.Event == "resolved" || e.Event == "cancelled"):
			s := &out[i]
			s.State, s.Closed, s.ClosedBy, s.Note = map[string]string{"resolved": "resolved", "cancelled": "cancelled"}[e.Event], e.TS, e.By, e.Note
			if e.Provided != "" {
				s.Provided, s.ProvidedAt = e.Provided, e.ProvidedAt
			}
			s.Choice = e.Choice
		}
	}
	return out, sc.Err()
}

func Open(all []Stop) []Stop {
	var out []Stop
	for _, s := range all {
		if s.State == "open" {
			out = append(out, s)
		}
	}
	return out
}

func Find(all []Stop, id string) *Stop {
	for i := range all {
		if all[i].ID == id {
			return &all[i]
		}
	}
	return nil
}

// Holds: tarea → "stop ST-NNNN" para los stoppers blocking abiertos del slice.
func Holds(all []Stop, sliceID string) map[string]string {
	h := map[string]string{}
	for _, s := range all {
		if s.State == "open" && s.Severity == "blocking" && s.Slice == sliceID {
			for _, t := range s.Tasks {
				if _, ya := h[t]; !ya {
					h[t] = "stop " + s.ID
				}
			}
		}
	}
	return h
}

// HoldsFor lee stops.jsonl del directorio .orq; un archivo ilegible no retiene nada.
func HoldsFor(orqDir, sliceID string) map[string]string {
	all, _ := Load(Path(orqDir))
	return Holds(all, sliceID)
}

func entropy(s string) float64 {
	n := map[rune]float64{}
	for _, r := range s {
		n[r]++
	}
	l := float64(len([]rune(s)))
	h := 0.0
	for _, c := range n {
		p := c / l
		h -= p * math.Log2(p)
	}
	return h
}

// LooksSecret: prefijo conocido, o un token de ≥20 caracteres con entropía > 3.5 bits/carácter.
// Una ruta o nombre con '/' o '.' y sin dígitos no cuenta como token aleatorio.
func LooksSecret(s string) bool {
	if strings.Contains(s, "-----BEGIN") {
		return true
	}
	for _, tok := range strings.Fields(s) {
		tok = strings.Trim(tok, "\"'`()[]{}<>,;:!?")
		for _, p := range prefixes {
			if strings.HasPrefix(tok, p) && len(tok) > len(p) {
				return true
			}
		}
		if len(tok) < 20 {
			continue
		}
		if strings.ContainsAny(tok, "/.") && !strings.ContainsAny(tok, "0123456789") {
			continue
		}
		if entropy(tok) > 3.5 {
			return true
		}
	}
	return false
}

func checkFree(fields ...string) error {
	for _, f := range fields {
		if LooksSecret(f) {
			return ErrSecret
		}
	}
	return nil
}

// CheckProvidedAt acepta env:NAME, file:RUTA fuera del repo o ignorada por git, o vault:REF.
func CheckProvidedAt(loc, repoRoot string, ignored func(string) bool) error {
	kind, ref, ok := strings.Cut(loc, ":")
	if !ok || ref == "" {
		return fmt.Errorf("--provided-at %q: use env:NAME, file:PATH or vault:REF", loc)
	}
	switch kind {
	case "env":
		if !envName.MatchString(ref) {
			return fmt.Errorf("env name %q must match [A-Z_][A-Z0-9_]*", ref)
		}
	case "vault":
	case "file":
		p := ref
		if strings.HasPrefix(p, "~/") {
			home, _ := os.UserHomeDir()
			p = filepath.Join(home, p[2:])
		}
		p = strings.Fields(p)[0]
		abs, _ := filepath.Abs(p)
		rel, err := filepath.Rel(repoRoot, abs)
		inside := err == nil && rel != ".." && !strings.HasPrefix(rel, "../")
		if inside && (ignored == nil || !ignored(abs)) {
			return fmt.Errorf("file %s is inside the repo and not git-ignored: keep secrets out of the repo", ref)
		}
		return nil
	default:
		return fmt.Errorf("--provided-at %q: use env:NAME, file:PATH or vault:REF", loc)
	}
	return checkFree(ref)
}

func next(all []Stop) string { return fmt.Sprintf("ST-%04d", len(all)+1) }

func appendEvent(path string, e Event) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	line, _ := json.Marshal(e)
	_, err = f.Write(append(line, '\n'))
	return err
}

func (r Raise) validate() error {
	switch {
	case !slices.Contains(Types, r.Type):
		return fmt.Errorf("--type %q: use %s", r.Type, strings.Join(Types, "|"))
	case !slices.Contains(Severities, r.Severity):
		return fmt.Errorf("--severity %q: use %s", r.Severity, strings.Join(Severities, "|"))
	case !regexp.MustCompile(`^S-\d+$`).MatchString(r.Slice):
		return fmt.Errorf("--slice %q: use S-NN", r.Slice)
	case strings.TrimSpace(r.Text) == "":
		return errors.New("--text is required: one plain sentence for the Owner")
	case r.Type == "decision" && len(r.Options) < 2:
		return errors.New("a decision needs --options \"a|b\"")
	case r.Where != "" && !envName.MatchString(r.Where):
		return fmt.Errorf("--where %q must be an env var name [A-Z_][A-Z0-9_]*", r.Where)
	}
	return checkFree(append([]string{r.Text}, r.Options...)...)
}

// Add levanta un stopper. Con uno abierto igual (slice, tipo, texto) devuelve ese y dup=true.
func Add(path string, r Raise, now time.Time) (Stop, bool, error) {
	if err := r.validate(); err != nil {
		return Stop{}, false, err
	}
	unlock, err := orqdir.Candado(path)
	if err != nil {
		return Stop{}, false, err
	}
	defer unlock()
	all, err := Load(path)
	if err != nil {
		return Stop{}, false, err
	}
	for _, s := range all {
		if s.State == "open" && s.Slice == r.Slice && s.Type == r.Type && s.Text == r.Text {
			return s, true, nil
		}
	}
	e := Event{ID: next(all), TS: now.UTC(), Event: "raised", Type: r.Type, Severity: r.Severity, Slice: r.Slice,
		Tasks: r.Tasks, By: r.By, Text: r.Text, Options: r.Options, Where: r.Where, Provided: "no"}
	if err := appendEvent(path, e); err != nil {
		return Stop{}, false, err
	}
	all, _ = Load(path)
	return *Find(all, e.ID), false, nil
}

func closeStop(path, id string, e Event, check func(Stop) error) (Stop, error) {
	unlock, err := orqdir.Candado(path)
	if err != nil {
		return Stop{}, err
	}
	defer unlock()
	all, err := Load(path)
	if err != nil {
		return Stop{}, err
	}
	s := Find(all, id)
	if s == nil {
		return Stop{}, fmt.Errorf("no stopper %s", id)
	}
	if s.State != "open" {
		return *s, fmt.Errorf("%s is already %s", id, s.State)
	}
	if check != nil {
		if err := check(*s); err != nil {
			return *s, err
		}
	}
	e.ID = id
	if err := appendEvent(path, e); err != nil {
		return Stop{}, err
	}
	all, _ = Load(path)
	return *Find(all, id), nil
}

// Resolve cierra un stopper. La CLI comprueba antes el rol (CanResolve) y la ubicación.
func Resolve(path, id string, r Resolution, now time.Time) (Stop, error) {
	if err := checkFree(r.Note, r.Choice); err != nil {
		return Stop{}, err
	}
	e := Event{TS: now.UTC(), Event: "resolved", By: r.By, Choice: r.Choice, Note: r.Note}
	if r.ProvidedAt != "" {
		e.Provided, e.ProvidedAt = "yes", r.ProvidedAt
	}
	return closeStop(path, id, e, func(s Stop) error {
		if s.Type == "decision" {
			if r.Choice == "" {
				return errors.New("a decision is resolved with --choice")
			}
			if !slices.Contains(s.Options, r.Choice) {
				return fmt.Errorf("--choice %q is not one of %s", r.Choice, strings.Join(s.Options, "|"))
			}
		}
		return nil
	})
}

// Cancel: quien lo levantó ya no lo necesita.
func Cancel(path, id, by, note string, now time.Time) (Stop, error) {
	if strings.TrimSpace(note) == "" {
		return Stop{}, errors.New("cancel needs --note")
	}
	if err := checkFree(note); err != nil {
		return Stop{}, err
	}
	return closeStop(path, id, Event{TS: now.UTC(), Event: "cancelled", By: by, Note: note}, nil)
}

// AlwaysOwner dice si un stopper debe llegar al Owner en cualquier modo (regla 1).
func AlwaysOwner(s Stop) bool {
	return s.Severity == "blocking" || s.Type == "credential" || s.Type == "api_key" || s.Type == "access"
}

// CanResolve aplica quién puede resolver: atena y owner siempre; el orquestador o el planificador
// solo en modo free, decision/confirmation soon/fyi, y con --note "auto: motivo".
func CanResolve(role string, s Stop, c config.Config, note string) error {
	switch role {
	case "atena", "owner":
		return nil
	case "orquestador", "planificador":
		if c.Mode() != "free" {
			return errors.New("guided mode: only Atena or the Owner resolve stoppers")
		}
		if AlwaysOwner(s) || (s.Type != "decision" && s.Type != "confirmation") {
			return fmt.Errorf("%s %s always reaches the Owner: only Atena or the Owner resolve it", s.Type, s.Severity)
		}
		if !strings.HasPrefix(note, "auto: ") {
			return errors.New("free mode: resolve with --note \"auto: <reason>\"")
		}
		return nil
	}
	return fmt.Errorf("role %q cannot resolve stoppers: only Atena or the Owner", role)
}
