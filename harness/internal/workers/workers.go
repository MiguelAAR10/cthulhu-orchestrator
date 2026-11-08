// Package workers guarda el estado vivo de cada worker en .orq/workers/<id>/state.json
// (contrato orq.worker/v1). Solo escriben el propio worker (orq worker) y el vigía.
// head y commits los calcula orq desde git; nunca los informa el worker.
package workers

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"orq/internal/orqdir"
)

const Schema = "orq.worker/v1"

var Phases = []string{"running", "asks_verification", "blocked", "question", "exited"}

type Tests struct {
	Added   *int `json:"added"`
	Passing int  `json:"passing"`
	Total   int  `json:"total"`
}

type State struct {
	Schema     string    `json:"schema"`
	ID         string    `json:"id"`
	Slice      string    `json:"slice"`
	Task       string    `json:"task"`
	Agent      string    `json:"agent"`
	Model      string    `json:"model"`
	PID        *int      `json:"pid"`
	Host       string    `json:"host"`
	Worktree   string    `json:"worktree"`
	Branch     string    `json:"branch"`
	Base       string    `json:"base,omitempty"` // desde dónde se cuentan los commits
	Started    time.Time `json:"started"`
	Heartbeat  time.Time `json:"heartbeat"`
	BudgetMin  int       `json:"budget_min"`
	Phase      string    `json:"phase"`
	ExitCode   *int      `json:"exit_code"`
	Head       string    `json:"head"`
	Commits    int       `json:"commits"`
	Tests      *Tests    `json:"tests"`
	ContextPct *int      `json:"context_pct"`
	Summary    string    `json:"summary"`
	Stop       string    `json:"stop"`
}

func Dir(orqDir string) string      { return filepath.Join(orqDir, "workers") }
func Path(orqDir, id string) string { return filepath.Join(Dir(orqDir), id, "state.json") }
func validID(id string) bool {
	return id != "" && !strings.ContainsAny(id, "/\\") && id != "." && id != ".."
}
func ErrNoWorker(id string) error { return fmt.Errorf("no worker %q (ORQ_WORKER_ID)", id) }
func intp(n int) *int             { return &n }

func Load(orqDir, id string) (State, error) {
	var st State
	if !validID(id) {
		return st, ErrNoWorker(id)
	}
	raw, err := os.ReadFile(Path(orqDir, id))
	if errors.Is(err, os.ErrNotExist) {
		return st, ErrNoWorker(id)
	}
	if err != nil {
		return st, err
	}
	return st, json.Unmarshal(raw, &st)
}

// Save escribe state.json.tmp y lo renombra, bajo el candado del archivo.
func Save(orqDir string, st State) error {
	if !validID(st.ID) {
		return ErrNoWorker(st.ID)
	}
	p := Path(orqDir, st.ID)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	unlock, err := orqdir.Candado(p)
	if err != nil {
		return err
	}
	defer unlock()
	return save(p, st)
}

func save(p string, st State) error {
	st.Schema = Schema
	raw, _ := json.MarshalIndent(st, "", "  ")
	if err := os.WriteFile(p+".tmp", append(raw, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(p+".tmp", p)
}

// update lee, modifica y guarda bajo candado.
func update(orqDir, id string, f func(*State) error) (State, error) {
	p := Path(orqDir, id)
	if _, err := Load(orqDir, id); err != nil {
		return State{}, err
	}
	unlock, err := orqdir.Candado(p)
	if err != nil {
		return State{}, err
	}
	defer unlock()
	st, err := Load(orqDir, id)
	if err != nil {
		return st, err
	}
	if err := f(&st); err != nil {
		return st, err
	}
	return st, save(p, st)
}

func List(orqDir string) ([]State, error) {
	ents, err := os.ReadDir(Dir(orqDir))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	var out []State
	for _, e := range ents {
		if !e.IsDir() {
			continue
		}
		if st, err := Load(orqDir, e.Name()); err == nil {
			out = append(out, st)
		}
	}
	slices.SortFunc(out, func(a, b State) int { return strings.Compare(a.ID, b.ID) })
	return out, err
}

// Launched crea el estado de un worker recién lanzado (phase=running). Lo llama lanzarWorker
// tras arrancar el agente, que recibe ORQ_WORKER_ID=<id>. ExitCode queda null: hoy ningún runner
// informa a orq cómo salió el agente.
func Launched(orqDir string, st State, now time.Time) error {
	st.Phase, st.Started, st.Heartbeat = "running", now.UTC(), now.UTC()
	if st.Host == "" {
		st.Host, _ = os.Hostname()
	}
	if st.Worktree != "" {
		st.Head, st.Commits, _ = GitInfo(st.Worktree, st.Base)
	}
	return Save(orqDir, st)
}

// GitInfo: HEAD del worktree y commits desde base (git rev-list --count base..HEAD).
func GitInfo(worktree, base string) (string, int, error) {
	out, err := exec.Command("git", "-C", worktree, "rev-parse", "HEAD").Output()
	if err != nil {
		return "", 0, err
	}
	head := strings.TrimSpace(string(out))
	if base == "" {
		return head, 0, nil
	}
	out, err = exec.Command("git", "-C", worktree, "rev-list", "--count", base+"..HEAD").Output()
	if err != nil {
		return head, 0, err
	}
	n, _ := strconv.Atoi(strings.TrimSpace(string(out)))
	return head, n, nil
}

type BeatOpts struct {
	Phase      string // running | blocked | question
	Summary    string
	ContextPct *int
	Stop       string // stopper abierto, obligatorio con blocked
	Tests      *Tests // del último recibo del runner, si hay
}

func Beat(orqDir, id string, o BeatOpts, now time.Time) (State, error) {
	if o.Phase != "" && !slices.Contains([]string{"running", "blocked", "question"}, o.Phase) {
		return State{}, fmt.Errorf("--phase %q: use running|blocked|question", o.Phase)
	}
	if o.Phase == "blocked" && o.Stop == "" {
		return State{}, errors.New("--phase blocked needs an open stopper: orq stop raise ... first")
	}
	if o.ContextPct != nil && (*o.ContextPct < 0 || *o.ContextPct > 100) {
		return State{}, errors.New("--context-pct must be 0..100")
	}
	return update(orqDir, id, func(st *State) error {
		st.Heartbeat = now.UTC()
		if o.Phase != "" {
			st.Phase = o.Phase
			st.Stop = ""
			if o.Phase == "blocked" {
				st.Stop = o.Stop
			}
		}
		if o.Summary != "" {
			st.Summary = o.Summary
		}
		if o.ContextPct != nil {
			st.ContextPct = o.ContextPct
		}
		if o.Tests != nil {
			st.Tests = o.Tests
		}
		if st.Worktree != "" {
			if h, n, err := GitInfo(st.Worktree, st.Base); err == nil {
				st.Head, st.Commits = h, n
			}
		}
		return nil
	})
}

// Done: phase=asks_verification (o exited) y exit_code=0.
func Done(orqDir, id, summary string, askVerification bool, now time.Time) (State, error) {
	return update(orqDir, id, func(st *State) error {
		st.Heartbeat, st.ExitCode, st.Summary, st.Stop = now.UTC(), intp(0), summary, ""
		st.Phase = "exited"
		if askVerification {
			st.Phase = "asks_verification"
		}
		if st.Worktree != "" {
			if h, n, err := GitInfo(st.Worktree, st.Base); err == nil {
				st.Head, st.Commits = h, n
			}
		}
		return nil
	})
}

// TaskEvent es una línea de .orq/tasks.jsonl.
type TaskEvent struct {
	TS     time.Time `json:"ts"`
	Slice  string    `json:"slice"`
	Task   string    `json:"task"`
	Event  string    `json:"event"` // done | reopened
	Worker string    `json:"worker,omitempty"`
}

func TasksPath(orqDir string) string { return filepath.Join(orqDir, "tasks.jsonl") }

func AppendTaskEvent(orqDir, sliceID, task, event, worker string, now time.Time) error {
	f, err := os.OpenFile(TasksPath(orqDir), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	line, _ := json.Marshal(TaskEvent{TS: now.UTC(), Slice: sliceID, Task: task, Event: event, Worker: worker})
	_, err = f.Write(append(line, '\n'))
	return err
}

func LoadTaskEvents(orqDir string) ([]TaskEvent, error) {
	f, err := os.Open(TasksPath(orqDir))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []TaskEvent
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var e TaskEvent
		if json.Unmarshal(sc.Bytes(), &e) == nil && e.Event != "" {
			out = append(out, e)
		}
	}
	return out, sc.Err()
}
