package app

import (
	"testing"

	"orq/internal/limits"
	"orq/internal/sysinfo"
)

// orq2 warns at or below aviso_ram_gb (default 5) of available RAM; orq never does.
func TestAvisoRAM(t *testing.T) {
	m := func(gb float64) sysinfo.Maquina { return sysinfo.Maquina{MemDisponibleGB: gb, MemTotalGB: 32} }
	t.Setenv("ORQ_NOMBRE", "")
	if a := AvisoRAM(limits.Limits{}, m(1)); a != "" {
		t.Fatalf("orq must not warn: %q", a)
	}
	t.Setenv("ORQ_NOMBRE", "orq2")
	if a := AvisoRAM(limits.Limits{}, m(5)); a != "RAM libre 5.0 GB (aviso ≤5): corre orq2 doctor --ram" {
		t.Fatalf("warning at 5 GB: %q", a)
	}
	if a := AvisoRAM(limits.Limits{}, m(5.1)); a != "" {
		t.Fatalf("no warning above 5 GB: %q", a)
	}
	ocho := 8.0
	if a := AvisoRAM(limits.Limits{AvisoRAMGB: &ocho}, m(6.25)); a != "RAM libre 6.2 GB (aviso ≤8): corre orq2 doctor --ram" {
		t.Fatalf("configured threshold: %q", a)
	}
}
