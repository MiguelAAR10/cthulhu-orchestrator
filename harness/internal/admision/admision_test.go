package admision

import (
	"strings"
	"testing"
	"time"

	"orq/internal/limits"
	"orq/internal/state"
)

func ptr[T any](v T) *T { return &v }

func lims() limits.Limits {
	return limits.Limits{MinFreeGB: map[string]float64{"/": 60}, MaxWorkers: ptr(2), StateMaxAgeHours: ptr(24.0),
		Scratch: limits.Scratch{Paths: []string{"/x"}, QuotaGB: ptr(100.0), CriticalFreeGB: ptr(30.0)}, MaxWorktrees: ptr(3)}
}

func env(t *testing.T, now time.Time) Env {
	dir := t.TempDir()
	state.SaveSnapshot(dir, state.Snapshot{TS: now, ScratchGB: 1})
	return Env{DiskFreeGB: func(string) (float64, error) { return 500, nil }, Workers: func() (int, error) { return 0, nil },
		Worktrees: func() (int, error) { return 0, nil }, Now: func() time.Time { return now }, StateDir: dir}
}

func TestAdmiteEnCondicionesSanas(t *testing.T) {
	if err := Evaluar(lims(), env(t, time.Now())); err != nil {
		t.Fatal(err)
	}
}

func TestRechazos(t *testing.T) {
	now := time.Now()
	casos := map[string]func(*Env, *limits.Limits){
		"HALT":   func(e *Env, _ *limits.Limits) { state.WriteHalt(e.StateDir, "disco", now) },
		"mínimo": func(e *Env, _ *limits.Limits) { e.DiskFreeGB = func(string) (float64, error) { return 10, nil } },
		"medición": func(e *Env, _ *limits.Limits) {
			state.SaveSnapshot(e.StateDir, state.Snapshot{TS: now.Add(-48 * time.Hour)})
		},
		"cuota": func(e *Env, _ *limits.Limits) {
			state.SaveSnapshot(e.StateDir, state.Snapshot{TS: now, ScratchGB: 500})
		},
		"worker":    func(e *Env, _ *limits.Limits) { e.Workers = func() (int, error) { return 2, nil } },
		"worktrees": func(e *Env, _ *limits.Limits) { e.Worktrees = func() (int, error) { return 3, nil } },
		"límites":   func(_ *Env, l *limits.Limits) { l.MaxWorkers = nil },
	}
	for want, mod := range casos {
		e, l := env(t, now), lims()
		mod(&e, &l)
		err := Evaluar(l, e)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v", want, err)
		}
	}
}

func TestMaxWorktreesPorDefecto12(t *testing.T) {
	e, l := env(t, time.Now()), lims()
	l.MaxWorktrees = nil
	e.Worktrees = func() (int, error) { return 11, nil }
	if err := Evaluar(l, e); err != nil {
		t.Fatal(err)
	}
	e.Worktrees = func() (int, error) { return 12, nil }
	if err := Evaluar(l, e); err == nil {
		t.Fatal("12 de 12 admitido")
	}
}

func TestReservaCuentaYSeLibera(t *testing.T) {
	now := time.Now()
	e, l := env(t, now), lims()
	r1, err := Reservar(l, e)
	if err != nil {
		t.Fatal(err)
	}
	r2, err := Reservar(l, e)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Reservar(l, e); err == nil {
		t.Fatal("tercera reserva con tope 2")
	}
	r1()
	r3, err := Reservar(l, e)
	if err != nil {
		t.Fatal("liberar no devolvió el cupo:", err)
	}
	r2()
	r3()
	Reservar(l, e) // reservas de un proceso que murió sin liberar
	Reservar(l, e)
	e.Now = func() time.Time { return now.Add(time.Hour) }
	if _, err := Reservar(l, e); err != nil {
		t.Fatal("reservas vencidas no se reconciliaron:", err)
	}
}
