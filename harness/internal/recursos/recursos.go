// Package recursos decide cuántos agentes caben en la máquina ahora mismo.
//
//	capacidad = mínimo de: el tope configurado (max_workers), lo que deja la RAM y lo que deja la CPU
//
// La RAM se mira sobre la memoria disponible, que ya descuenta a los agentes que corren; la CPU,
// sobre los núcleos menos la reserva. Con la carga por encima del umbral no se lanza nada nuevo.
package recursos

import (
	"fmt"
	"math"

	"orq/internal/config"
	"orq/internal/sysinfo"
)

// Capacidad es el resultado y de dónde sale, para poder explicarlo.
type Capacidad struct {
	Tope      int    // agentes que pueden estar vivos a la vez, ahora
	Vivos     int    // agentes vivos al medir
	PorConfig int    // max_workers
	PorRAM    int    // vivos + los que caben en la memoria disponible menos la reserva
	PorCPU    int    // los que caben en los núcleos menos la reserva
	Limita    string // max_workers | RAM | CPU | carga
	Maquina   sysinfo.Maquina
	Reglas    config.Recursos
}

// Calcular aplica las reglas a una medición.
func Calcular(r config.Recursos, maxWorkers, vivos int, m sysinfo.Maquina) Capacidad {
	c := Capacidad{Vivos: vivos, PorConfig: maxWorkers, Maquina: m, Reglas: r}
	libres := math.Floor((m.MemDisponibleGB - r.ReservaRAMGB) / r.RAMPorAgenteGB)
	c.PorRAM = vivos + int(math.Max(0, libres))
	c.PorCPU = int(math.Max(0, math.Floor((float64(m.CPUs)-r.ReservaCPUs)/r.CPUPorAgente)))
	c.Tope, c.Limita = c.PorConfig, "max_workers"
	if c.PorRAM < c.Tope {
		c.Tope, c.Limita = c.PorRAM, "RAM"
	}
	if c.PorCPU < c.Tope {
		c.Tope, c.Limita = c.PorCPU, "CPU"
	}
	// Máquina ya cargada: lo que corre se queda, pero no entra nadie más.
	if m.Carga1 > float64(m.CPUs)*r.CargaMax && vivos < c.Tope {
		c.Tope, c.Limita = vivos, "carga"
	}
	if c.Tope < 0 {
		c.Tope = 0
	}
	return c
}

// Cabe dice si se puede lanzar un agente más.
func (c Capacidad) Cabe() bool { return c.Vivos < c.Tope }

// Motivo explica por qué no cabe otro agente, con el número que lo decide y cómo cambiarlo.
func (c Capacidad) Motivo() string {
	m, r := c.Maquina, c.Reglas
	switch c.Limita {
	case "RAM":
		return fmt.Sprintf("no cabe otro agente por RAM: hay %.1f GB disponibles de %.1f, se reservan %.1f GB y cada agente presupuesta %.1f GB (caben %d, vivos %d). Cierra sesiones o ajusta recursos.ram_por_agente_gb / recursos.reserva_ram_gb en `orq config ver`",
			m.MemDisponibleGB, m.MemTotalGB, r.ReservaRAMGB, r.RAMPorAgenteGB, c.PorRAM, c.Vivos)
	case "CPU":
		return fmt.Sprintf("no cabe otro agente por CPU: %d núcleos, se reservan %.1f y cada agente presupuesta %.1f (caben %d, vivos %d). Ajusta recursos.cpu_por_agente / recursos.reserva_cpus en `orq config ver`",
			m.CPUs, r.ReservaCPUs, r.CPUPorAgente, c.PorCPU, c.Vivos)
	case "carga":
		return fmt.Sprintf("no se lanza con la máquina cargada: carga %.1f en %d núcleos y el umbral es %.1f por núcleo (recursos.carga_max). Espera a que baje",
			m.Carga1, m.CPUs, r.CargaMax)
	}
	return fmt.Sprintf("hay %d worker(s) vivos y el tope es %d (max_workers en limits.json)", c.Vivos, c.PorConfig)
}

// Resumen es una línea para `orq status`.
func (c Capacidad) Resumen() string {
	return fmt.Sprintf("%d agentes a la vez (limita %s): max_workers %d · RAM %d · CPU %d", c.Tope, c.Limita, c.PorConfig, c.PorRAM, c.PorCPU)
}
