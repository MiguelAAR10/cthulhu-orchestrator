package cli

import (
	"fmt"
	"io"
	"path/filepath"
	"slices"

	"orq/internal/app"
	"orq/internal/limits"
	"orq/internal/sysinfo"
)

// runDoctorRAM is `orq2 doctor --ram`: read-only suggestions on what holds RAM.
func runDoctorRAM(stdout io.Writer) int {
	agentes := []string{"claude", "codex"}
	if l, err := limits.Load(limits.DefaultPath()); err == nil {
		agentes = l.WorkerProcessName
	}
	return doctorRAM(stdout, "/proc", agentes, app.EsWorkerContable())
}

// doctorRAM lists, from proc, the 8 processes holding the most RAM and the listening TCP ports,
// each line ending in a suggestion the Owner decides. It never stops anything. Idle agent panes
// are out of scope.
func doctorRAM(w io.Writer, proc string, agentes []string, propio func(cwd string) bool) int {
	if m, err := sysinfo.LeerMaquinaDe(proc); err == nil {
		fmt.Fprintf(w, "RAM libre %.1f GB de %.1f · solo sugerencias: orq2 no detiene nada, decides tú\n", m.MemDisponibleGB, m.MemTotalGB)
	} else {
		fmt.Fprintf(w, "RAM sin medición (%v) · solo sugerencias: orq2 no detiene nada, decides tú\n", err)
	}
	puertos, errP := sysinfo.PuertosEscuchaDe(proc)
	porPID := map[int][]int{}
	for _, p := range puertos {
		if p.PID != 0 {
			porPID[p.PID] = append(porPID[p.PID], p.Port)
		}
	}
	ps, err := sysinfo.ProcesosPorRSSDe(proc, 8)
	if err != nil {
		fmt.Fprintf(w, "procesos: no se pudo leer %s (%v)\n", proc, err)
		return 1
	}
	fmt.Fprintln(w, "procesos que más RAM usan:")
	for _, p := range ps {
		tipo, sug := "", "cerrarlo si no lo usas"
		switch {
		case slices.Contains(agentes, p.Comm) && p.Cwd != "" && propio(p.Cwd):
			tipo, sug = "worker orq2", "cerrar panel cuando entregue su tarea"
		case slices.Contains(agentes, p.Comm):
			tipo, sug = "agente", "cerrar panel si ya no lo usas"
		case len(porPID[p.PID]) > 0:
			sug = fmt.Sprintf("detener servidor en puerto %d", porPID[p.PID][0])
		}
		cwd := p.Cwd
		if cwd == "" {
			cwd = "?"
		}
		fmt.Fprintf(w, "  %-7d %-12s %5.1f GB  %-11s %s → %s\n", p.PID, p.Comm, p.RSSGB, tipo, filepath.Clean(cwd), sug)
	}
	if errP != nil {
		fmt.Fprintf(w, "puertos: no se pudo leer %s/net/tcp (%v)\n", proc, errP)
		return 0
	}
	fmt.Fprintln(w, "puertos TCP en escucha:")
	for _, p := range puertos {
		if p.PID == 0 {
			fmt.Fprintf(w, "  puerto %d · dueño no visible → revisar quién lo abrió (otro usuario o contenedor)\n", p.Port)
			continue
		}
		fmt.Fprintf(w, "  puerto %d · pid %d %s → detener servidor en puerto %d\n", p.Port, p.PID, p.Comm, p.Port)
	}
	return 0
}
