package recursos

import (
	"strings"
	"testing"

	"orq/internal/config"
	"orq/internal/sysinfo"
)

var reglas = config.Recursos{RAMPorAgenteGB: 2, ReservaRAMGB: 2, CPUPorAgente: 1, ReservaCPUs: 1, CargaMax: 1.5}

func TestCapacidadEsElMinimoYDiceQuienLimita(t *testing.T) {
	for _, c := range []struct {
		nombre     string
		max, vivos int
		m          sysinfo.Maquina
		tope       int
		limita     string
		cabe       bool
		enMotivo   string
	}{
		{"holgada: manda el tope configurado", 4, 0, sysinfo.Maquina{MemTotalGB: 64, MemDisponibleGB: 40, CPUs: 16, Carga1: 1}, 4, "max_workers", true, "max_workers"},
		{"poca RAM", 4, 0, sysinfo.Maquina{MemTotalGB: 8, MemDisponibleGB: 6.5, CPUs: 16}, 2, "RAM", true, "RAM"},
		{"RAM agotada con dos vivos", 4, 2, sysinfo.Maquina{MemTotalGB: 8, MemDisponibleGB: 3, CPUs: 16}, 2, "RAM", false, "ram_por_agente_gb"},
		{"menos disponible que la reserva", 4, 0, sysinfo.Maquina{MemTotalGB: 8, MemDisponibleGB: 1, CPUs: 16}, 0, "RAM", false, "RAM"},
		{"pocos núcleos", 4, 0, sysinfo.Maquina{MemTotalGB: 64, MemDisponibleGB: 40, CPUs: 2}, 1, "CPU", true, "CPU"},
		{"un núcleo: la reserva se lo lleva", 4, 0, sysinfo.Maquina{MemTotalGB: 64, MemDisponibleGB: 40, CPUs: 1}, 0, "CPU", false, "reserva_cpus"},
		{"máquina cargada: no entra nadie más", 4, 1, sysinfo.Maquina{MemTotalGB: 64, MemDisponibleGB: 40, CPUs: 4, Carga1: 9}, 1, "carga", false, "carga_max"},
		{"tope lleno", 2, 2, sysinfo.Maquina{MemTotalGB: 64, MemDisponibleGB: 40, CPUs: 16}, 2, "max_workers", false, "max_workers"},
	} {
		got := Calcular(reglas, c.max, c.vivos, c.m)
		if got.Tope != c.tope || got.Limita != c.limita || got.Cabe() != c.cabe {
			t.Errorf("%s: tope %d limita %s cabe %v; quiero %d %s %v", c.nombre, got.Tope, got.Limita, got.Cabe(), c.tope, c.limita, c.cabe)
		}
		if !strings.Contains(got.Motivo(), c.enMotivo) {
			t.Errorf("%s: el motivo no nombra %q: %s", c.nombre, c.enMotivo, got.Motivo())
		}
	}
}
