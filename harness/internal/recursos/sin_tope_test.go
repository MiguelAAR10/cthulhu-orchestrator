package recursos

import (
	"strings"
	"testing"

	"orq/internal/config"
	"orq/internal/sysinfo"
)

// Without max_workers (orq2) the machine decides: RAM, CPU or load, never "max_workers".
func TestSinTopeDecideLaMaquina(t *testing.T) {
	for _, c := range []struct {
		m      sysinfo.Maquina
		vivos  int
		tope   int
		limita string
	}{
		{sysinfo.Maquina{MemTotalGB: 64, MemDisponibleGB: 40, CPUs: 16}, 0, 15, "CPU"},
		{sysinfo.Maquina{MemTotalGB: 64, MemDisponibleGB: 10, CPUs: 16}, 0, 4, "RAM"},
		{sysinfo.Maquina{MemTotalGB: 64, MemDisponibleGB: 40, CPUs: 4, Carga1: 9}, 1, 1, "carga"},
	} {
		got := Calcular(reglas, SinTope, c.vivos, c.m)
		if got.Tope != c.tope || got.Limita != c.limita {
			t.Errorf("%+v: tope %d limita %s; want %d %s", c.m, got.Tope, got.Limita, c.tope, c.limita)
		}
		if r := got.Resumen(); !strings.Contains(r, "limita "+c.limita) || !strings.Contains(r, "max_workers sin tope") {
			t.Errorf("Resumen must say what limits and that there is no head-count cap: %s", r)
		}
	}
}

// F7: an explicit -1 (orq) caps by max_workers as in 1.0.0, it is not SinTope.
func TestMenosUnoNoEsSinTope(t *testing.T) {
	r := config.Recursos{RAMPorAgenteGB: 1, CPUPorAgente: 1, CargaMax: 10}
	c := Calcular(r, -1, 0, sysinfo.Maquina{MemDisponibleGB: 64, MemTotalGB: 64, CPUs: 16})
	if c.Tope != 0 || c.Limita != "max_workers" {
		t.Fatalf("tope %d limita %q", c.Tope, c.Limita)
	}
}
