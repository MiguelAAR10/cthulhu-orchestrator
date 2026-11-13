package sysinfo

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// Maquina es lo que orq mira antes de decidir cuántos agentes caben.
type Maquina struct {
	MemTotalGB      float64
	MemDisponibleGB float64 // MemAvailable: lo que el kernel estima que se puede pedir sin intercambiar
	CPUs            int
	Carga1          float64 // carga media del último minuto
}

// LeerMaquina lee memoria, núcleos y carga de /proc.
func LeerMaquina() (Maquina, error) { return LeerMaquinaDe("/proc") }

// LeerMaquinaDe lee de otra raíz (pruebas). Un dato que falta es un error: sin dato no se decide.
func LeerMaquinaDe(proc string) (Maquina, error) {
	var m Maquina
	f, err := os.Open(filepath.Join(proc, "meminfo"))
	if err != nil {
		return m, fmt.Errorf("no se pudo leer la memoria: %w", err)
	}
	defer f.Close()
	kb := map[string]float64{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		campo, resto, ok := strings.Cut(sc.Text(), ":")
		if !ok {
			continue
		}
		if fs := strings.Fields(resto); len(fs) > 0 {
			if v, err := strconv.ParseFloat(fs[0], 64); err == nil {
				kb[campo] = v
			}
		}
	}
	total, hayTotal := kb["MemTotal"]
	disp, hayDisp := kb["MemAvailable"]
	if !hayTotal || !hayDisp {
		return m, fmt.Errorf("%s no trae MemTotal y MemAvailable", filepath.Join(proc, "meminfo"))
	}
	m.MemTotalGB, m.MemDisponibleGB = total/1e6, disp/1e6

	raw, err := os.ReadFile(filepath.Join(proc, "loadavg"))
	if err != nil {
		return m, fmt.Errorf("no se pudo leer la carga: %w", err)
	}
	fs := strings.Fields(string(raw))
	if len(fs) == 0 {
		return m, fmt.Errorf("%s vacío", filepath.Join(proc, "loadavg"))
	}
	if m.Carga1, err = strconv.ParseFloat(fs[0], 64); err != nil {
		return m, fmt.Errorf("carga ilegible %q", fs[0])
	}

	// Núcleos: los que lista /proc/cpuinfo; si no se puede leer, los que ve el proceso.
	if raw, err := os.ReadFile(filepath.Join(proc, "cpuinfo")); err == nil {
		for _, l := range strings.Split(string(raw), "\n") {
			if strings.HasPrefix(l, "processor") {
				m.CPUs++
			}
		}
	}
	if m.CPUs == 0 {
		m.CPUs = runtime.NumCPU()
	}
	return m, nil
}
