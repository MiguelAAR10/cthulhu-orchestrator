package app

import (
	"fmt"
	"strconv"

	"orq/internal/identidad"
	"orq/internal/limits"
	"orq/internal/sysinfo"
)

// AvisoRAM is orq2's warning line when available RAM is at or below aviso_ram_gb, or "".
// It only points to `orq2 doctor --ram`; the launch decision stays with capacity. orq never warns.
func AvisoRAM(l limits.Limits, m sysinfo.Maquina) string {
	if !identidad.EsOrq2() || m.MemDisponibleGB > l.AvisoRAM() {
		return ""
	}
	return fmt.Sprintf("RAM libre %.1f GB (aviso ≤%s): corre orq2 doctor --ram",
		m.MemDisponibleGB, strconv.FormatFloat(l.AvisoRAM(), 'f', -1, 64))
}
