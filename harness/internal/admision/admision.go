// Package admision es la única puerta para lanzar un worker o crear su worktree.
// La usan el gate (hook) y `orq slice lanzar`. Falla cerrado: sin dato no hay lanzamiento.
package admision

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"orq/internal/lang"
	"orq/internal/limits"
	"orq/internal/orqdir"
	"orq/internal/state"
)

// Env inyecta lo que depende del sistema.
type Env struct {
	DiskFreeGB func(string) (float64, error)
	Workers    func() (int, error)
	Worktrees  func() (int, error) // worktrees vinculados que orq registró para el proyecto
	Now        func() time.Time
	StateDir   string
}

// VidaReserva: una reserva sin liberar más vieja que esto es de un proceso que murió.
const VidaReserva = 10 * time.Minute

// Evaluar decide sin reservar (lo usa el gate).
func Evaluar(l limits.Limits, env Env) error { return evaluar(l, env, 0) }

func evaluar(l limits.Limits, env Env, reservas int) error {
	if len(l.MinFreeGB) == 0 || l.MaxWorkers == nil || l.StateMaxAgeHours == nil || l.Scratch.QuotaGB == nil {
		return errors.New(lang.T("orq: incomplete limits (min_free_gb, max_workers, state_max_age_hours, scratch.quota_gb). No limits, no launch", "orq: límites incompletos (min_free_gb, max_workers, state_max_age_hours, scratch.quota_gb). Sin límites no hay lanzamiento"))
	}
	if reason, halted := state.ReadHalt(env.StateDir); halted {
		return fmt.Errorf(lang.T("orq: HALT active (%s). Free space, then `orq halt --clear`", "orq: HALT activo (%s). Libera espacio y luego `orq halt --clear`"), reason)
	}
	for path, min := range l.MinFreeGB {
		if env.DiskFreeGB == nil {
			return errors.New(lang.T("orq: no way to measure the disk. No data, no launch", "orq: no hay forma de medir el disco. Sin dato no hay lanzamiento"))
		}
		free, err := env.DiskFreeGB(path)
		if err != nil {
			return fmt.Errorf(lang.T("orq: could not measure the disk at %s (%v). No data, no launch", "orq: no pude medir el disco en %s (%v). Sin dato no hay lanzamiento"), path, err)
		}
		if free < min {
			return fmt.Errorf(lang.T("orq: %s has %.0f GB free and the minimum is %.0f GB. Free space (orq gc) before launching", "orq: %s tiene %.0f GB libres y el mínimo es %.0f GB. Libera espacio (orq gc) antes de lanzar"), path, free, min)
		}
	}
	snap, err := state.LoadSnapshot(env.StateDir)
	if err != nil {
		return errors.New(lang.T("orq: scratch measurement missing. Run `orq watch` and try again", "orq: falta la medición de scratch. Ejecuta `orq watch` y vuelve a intentar"))
	}
	if age := env.Now().Sub(snap.TS); age > time.Duration(*l.StateMaxAgeHours*float64(time.Hour)) {
		return fmt.Errorf(lang.T("orq: the scratch measurement is %.0f h old. Run `orq watch` and try again", "orq: la medición de scratch tiene %.0f h. Ejecuta `orq watch` y vuelve a intentar"), age.Hours())
	}
	if snap.ScratchGB > *l.Scratch.QuotaGB {
		return fmt.Errorf(lang.T("orq: scratch uses %.0f GB and the quota is %.0f GB. Clean up before launching", "orq: scratch ocupa %.0f GB y la cuota es %.0f GB. Limpia antes de lanzar"), snap.ScratchGB, *l.Scratch.QuotaGB)
	}
	if env.Workers == nil || env.Worktrees == nil {
		return errors.New(lang.T("orq: no way to count workers or worktrees. No data, no launch", "orq: no hay forma de contar workers o worktrees. Sin dato no hay lanzamiento"))
	}
	n, err := env.Workers()
	if err != nil {
		return fmt.Errorf(lang.T("orq: could not count workers (%v). No data, no launch", "orq: no pude contar workers (%v). Sin dato no hay lanzamiento"), err)
	}
	if n+reservas >= *l.MaxWorkers {
		return fmt.Errorf(lang.T("orq: %d live worker(s) and %d reserved; the cap is %d. Close panes before launching another", "orq: hay %d worker(s) vivos y %d reservado(s); el tope es %d. Cierra paneles antes de lanzar otro"), n, reservas, *l.MaxWorkers)
	}
	w, err := env.Worktrees()
	if err != nil {
		return fmt.Errorf(lang.T("orq: could not count worktrees (%v). No data, no launch", "orq: no pude contar worktrees (%v). Sin dato no hay lanzamiento"), err)
	}
	if max := l.TopeWorktrees(); w+reservas >= max {
		return fmt.Errorf(lang.T("orq: %d orq worktrees and %d reserved; max_worktrees is %d. Run `orq gc` or close slices before launching", "orq: hay %d worktrees de orq y %d reservado(s); el tope max_worktrees es %d. Corre `orq gc` o cierra slices antes de lanzar"), w, reservas, max)
	}
	return nil
}

type reserva struct {
	ID  string    `json:"id"`
	PID int       `json:"pid"`
	TS  time.Time `json:"ts"`
}

// Reservar evalúa y aparta un cupo bajo candado, para que dos lanzamientos simultáneos no
// pasen ambos por el último lugar. liberar() devuelve el cupo: al arrancar el worker (desde
// ahí cuenta como proceso vivo) o al fallar. Las reservas que nadie liberó vencen solas.
func Reservar(l limits.Limits, env Env) (liberar func(), err error) {
	if err := os.MkdirAll(env.StateDir, 0o755); err != nil {
		return nil, err
	}
	soltar, err := candado(env.StateDir)
	if err != nil {
		return nil, err
	}
	defer soltar()
	now := env.Now()
	rs := vigentes(leer(env.StateDir), now)
	if err := evaluar(l, env, len(rs)); err != nil {
		guardar(env.StateDir, rs)
		return nil, err
	}
	id := strconv.FormatInt(now.UnixNano(), 36) + "-" + strconv.Itoa(os.Getpid()) + "-" + strconv.Itoa(len(rs))
	rs = append(rs, reserva{ID: id, PID: os.Getpid(), TS: now})
	if err := guardar(env.StateDir, rs); err != nil {
		return nil, err
	}
	return func() {
		soltar, err := candado(env.StateDir)
		if err != nil {
			return
		}
		defer soltar()
		var keep []reserva
		for _, r := range leer(env.StateDir) {
			if r.ID != id {
				keep = append(keep, r)
			}
		}
		guardar(env.StateDir, keep)
	}, nil
}

func vigentes(rs []reserva, now time.Time) []reserva {
	var out []reserva
	for _, r := range rs {
		if d := now.Sub(r.TS); d < VidaReserva && d > -VidaReserva {
			out = append(out, r)
		}
	}
	return out
}

func path(dir string) string { return filepath.Join(dir, "reservas.json") }

func leer(dir string) []reserva {
	var rs []reserva
	if raw, err := os.ReadFile(path(dir)); err == nil {
		json.Unmarshal(raw, &rs)
	}
	return rs
}

func guardar(dir string, rs []reserva) error {
	raw, _ := json.Marshal(rs)
	tmp := path(dir) + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path(dir))
}

func candado(dir string) (func(), error) { return orqdir.Candado(filepath.Join(dir, "admision")) }
