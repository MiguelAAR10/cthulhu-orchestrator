package app

import (
	"fmt"

	"orq/internal/config"
	"orq/internal/gate"
	"orq/internal/identidad"
	"orq/internal/limits"
	"orq/internal/recursos"
	"orq/internal/sysinfo"
)

// Medir son las dos lecturas del sistema que deciden un lanzamiento. Se reemplazan en pruebas.
type Medir struct {
	Maquina func() (sysinfo.Maquina, error)
	Vivos   func(l limits.Limits) ([]sysinfo.Proc, error)
}

// MedirReal lee /proc.
func MedirReal() Medir {
	return Medir{
		Maquina: sysinfo.LeerMaquina,
		Vivos: func(l limits.Limits) ([]sysinfo.Proc, error) {
			procs, err := sysinfo.ListProcs()
			return sysinfo.Workers(procs, l.WorkerProcessName, EsWorkerContable()), err
		},
	}
}

// EsWorkerContable is the predicate that counts workers for max_workers. orq2 counts only its own
// clones (gate.EsWorkerPropio); orq keeps the 1.0.0 rule (gate.IsWorker: any linked worktree).
func EsWorkerContable() func(cwd string) bool {
	if identidad.EsOrq2() {
		return gate.EsWorkerPropio
	}
	return gate.IsWorker
}

// Capacidad mide la máquina y aplica las reglas de recursos de la configuración.
// Sin medición no hay número: el error lo decide quien llama (un lanzamiento se niega).
func Capacidad(cfg config.Config, l limits.Limits, m Medir) (recursos.Capacidad, []sysinfo.Proc, error) {
	vivos, err := m.Vivos(l)
	if err != nil {
		return recursos.Capacidad{}, nil, fmt.Errorf("no se pudo contar workers: %w", err)
	}
	maq, err := m.Maquina()
	if err != nil {
		return recursos.Capacidad{}, vivos, err
	}
	return recursos.Calcular(cfg.Recursos, l.TopeWorkers(), len(vivos), maq), vivos, nil
}

// CapacidadPara es la consulta del gate: con vivos ya contados, ¿cabe uno más en el proyecto de root?
// Devuelve "" si cabe o el motivo si no.
func CapacidadPara(root string, l limits.Limits, vivos int, m Medir) (string, error) {
	cfg, err := config.Cargar(root)
	if err != nil {
		return "", err
	}
	maq, err := m.Maquina()
	if err != nil {
		return "", err
	}
	if c := recursos.Calcular(cfg.Recursos, l.TopeWorkers(), vivos, maq); !c.Cabe() {
		return c.Motivo(), nil
	}
	return "", nil
}
