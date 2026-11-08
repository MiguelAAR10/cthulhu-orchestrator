// Package watch mide los directorios de scratch y el disco, guarda la medición
// y escribe HALT cuando se supera la cuota o el disco baja del crítico.
package watch

import (
	"fmt"
	"os"
	"time"

	"orq/internal/limits"
	"orq/internal/state"
	"orq/internal/sysinfo"
)

type Result struct {
	Snapshot state.Snapshot
	Halt     string   // vacío si no hay motivo de HALT
	Avisos   []string // disco bajo el mínimo de lanzamiento: avisa, no detiene
}

func Run(l limits.Limits, stateDir string, now time.Time) (Result, error) {
	snap := state.Snapshot{TS: now, PerPathGB: map[string]float64{}, DiskFreeGB: map[string]float64{}}
	for _, p := range l.Scratch.Paths {
		if _, err := os.Stat(p); err != nil {
			continue // un scratch que no existe ocupa 0
		}
		bytes, _ := sysinfo.DirSizeBytes(p)
		gb := float64(bytes) / 1e9
		snap.PerPathGB[p] = gb
		snap.ScratchGB += gb
	}
	var halt string
	var avisos []string
	for path, min := range l.MinFreeGB {
		free, err := sysinfo.FreeGB(path)
		if err != nil {
			continue
		}
		snap.DiskFreeGB[path] = free
		if free < min {
			avisos = append(avisos, fmt.Sprintf("disco bajo: %s con %.0f GB libres (mínimo para lanzar %.0f GB). Corre `orq gc`", path, free, min))
		}
		if free < *l.Scratch.CriticalFreeGB && halt == "" {
			halt = fmt.Sprintf("disco crítico: %s con %.0f GB libres (crítico %.0f GB)", path, free, *l.Scratch.CriticalFreeGB)
		}
	}
	if snap.ScratchGB > *l.Scratch.QuotaGB && halt == "" {
		halt = fmt.Sprintf("scratch excede la cuota: %.0f GB de %.0f GB", snap.ScratchGB, *l.Scratch.QuotaGB)
	}
	if err := state.SaveSnapshot(stateDir, snap); err != nil {
		return Result{}, err
	}
	if halt != "" {
		if err := state.WriteHalt(stateDir, halt, now); err != nil {
			return Result{}, err
		}
	}
	return Result{Snapshot: snap, Halt: halt, Avisos: avisos}, nil
}
