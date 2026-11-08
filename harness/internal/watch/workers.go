package watch

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"orq/internal/workers"
)

// El vigía de workers es código, no un LLM: lee state.json, da un veredicto y avisa solo
// cuando el veredicto cambia. Nunca mata procesos ni borra worktrees (eso es orq gc).

type WorkerEnv struct {
	Now      time.Time
	StaleMin int
	Host     string
	Alive    func(pid int) bool
	// Notify manda un mensaje al orquestador; seq hace única la clave de idempotencia por cambio.
	Notify func(worker, verdict, reason string, seq int) error
	// Raise levanta confirmation/soon cuando un worker pasa 1.5× su presupuesto.
	Raise func(st workers.State, reason string) error
}

type WorkerVerdict struct {
	Worker  string `json:"worker"`
	Verdict string `json:"verdict"`
	Reason  string `json:"reason"`
	Changed bool   `json:"-"`
}

// memoria del vigía por worker (fuera de state.json).
type mem struct {
	Head    string `json:"head"`
	Verdict string `json:"verdict"`
	Acted   bool   `json:"timeout_acted,omitempty"`
}

func WatchLogPath(orqDir string) string { return filepath.Join(orqDir, "watch.jsonl") }
func memPath(orqDir, id string) string  { return filepath.Join(workers.Dir(orqDir), id, "watch.json") }

// PIDAlive: kill(pid, 0) sin enviar señal.
func PIDAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

// Judge aplica la tabla del spec. act=true cuando el timeout pasa de 1.5× el presupuesto.
func Judge(st workers.State, prevHead string, havePrev bool, now time.Time, staleMin int, host string, alive func(int) bool) (verdict, reason string, act bool) {
	done := st.Phase == "exited" || st.Phase == "asks_verification"
	switch {
	case st.Phase == "exited":
		return "exited", "phase exited", false
	case st.PID != nil && st.Host == host && alive != nil && !alive(*st.PID) && !done:
		return "dead", fmt.Sprintf("pid %d is gone", *st.PID), false
	case st.Phase == "running" && now.Sub(st.Heartbeat) > time.Duration(staleMin)*time.Minute && havePrev && st.Head == prevHead:
		return "stuck", fmt.Sprintf("no heartbeat for %dm and no new commits", int(now.Sub(st.Heartbeat).Minutes())), false
	case st.BudgetMin > 0 && !done:
		el := now.Sub(st.Started)
		b := time.Duration(st.BudgetMin) * time.Minute
		if el > b*3/2 {
			return "timeout", fmt.Sprintf("%dm of %dm budget (over 1.5x)", int(el.Minutes()), st.BudgetMin), true
		}
		if el > b {
			return "timeout", fmt.Sprintf("%dm of %dm budget", int(el.Minutes()), st.BudgetMin), false
		}
	}
	return "ok", "", false
}

// Current da el veredicto de ahora sin escribir nada (para el tablero).
func Current(orqDir string, st workers.State, now time.Time, staleMin int, host string) string {
	var m mem
	raw, err := os.ReadFile(memPath(orqDir, st.ID))
	have := err == nil && json.Unmarshal(raw, &m) == nil
	v, _, _ := Judge(st, m.Head, have, now, staleMin, host, PIDAlive)
	return v
}

// Workers hace una pasada sobre todos los workers.
func Workers(orqDir string, env WorkerEnv) ([]WorkerVerdict, error) {
	ls, err := workers.List(orqDir)
	if err != nil {
		return nil, err
	}
	var out []WorkerVerdict
	for _, st := range ls {
		var m mem
		raw, err := os.ReadFile(memPath(orqDir, st.ID))
		have := err == nil && json.Unmarshal(raw, &m) == nil
		v, reason, act := Judge(st, m.Head, have, env.Now, env.StaleMin, env.Host, env.Alive)
		wv := WorkerVerdict{Worker: st.ID, Verdict: v, Reason: reason}
		prev := m.Verdict
		if !have {
			prev = "ok" // un worker nuevo y sano no genera ruido
		}
		if v != prev {
			wv.Changed = true
			seq, err := appendLog(orqDir, env.Now, wv)
			if err != nil {
				return out, err
			}
			if env.Notify != nil {
				env.Notify(st.ID, v, reason, seq)
			}
		}
		if act && !m.Acted && env.Raise != nil {
			if err := env.Raise(st, reason); err == nil {
				m.Acted = true
			}
		}
		m.Head, m.Verdict = st.Head, v
		b, _ := json.Marshal(m)
		os.WriteFile(memPath(orqDir, st.ID), b, 0o644)
		out = append(out, wv)
	}
	return out, nil
}

// appendLog agrega el cambio y devuelve cuántos cambios lleva ese worker.
func appendLog(orqDir string, now time.Time, v WorkerVerdict) (int, error) {
	raw, _ := os.ReadFile(WatchLogPath(orqDir))
	seq := 1
	for _, l := range splitLines(raw) {
		var e WorkerVerdict
		if json.Unmarshal(l, &e) == nil && e.Worker == v.Worker {
			seq++
		}
	}
	f, err := os.OpenFile(WatchLogPath(orqDir), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	line, _ := json.Marshal(struct {
		TS time.Time `json:"ts"`
		WorkerVerdict
	}{now.UTC(), v})
	_, err = f.Write(append(line, '\n'))
	return seq, err
}

func splitLines(b []byte) [][]byte {
	var out [][]byte
	start := 0
	for i, c := range b {
		if c == '\n' {
			if i > start {
				out = append(out, b[start:i])
			}
			start = i + 1
		}
	}
	return out
}
