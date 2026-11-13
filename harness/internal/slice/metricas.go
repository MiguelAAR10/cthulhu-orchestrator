package slice

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"
	"time"
)

// Metricas del piloto. Lo que orq no registra (costo, tiempo humano) no se inventa.
type Metricas struct {
	Aceptados, Rechazados int
	Intentos              int // intentos totales de slices cerrados
	Verificaciones        int
	VerdesALaPrimera      int // slices aceptados en el intento 1
	FallosSinProgreso     int
	Cuarentena            int
	HorasMedias           float64                 // abierto → cerrado, aceptados
	PorRuta               map[string]MetricasRuta // fix|slice: aceptados, primer intento y horas por ruta
	Ascendidos            int                     // fixes que subieron a slice (cerrados, aceptados o no)
}

// MetricasRuta son los números de una ruta. Un fix ascendido cuenta en slice, que es donde terminó;
// Ascendidos dice cuántos fueron.
type MetricasRuta struct {
	Aceptados, VerdesALaPrimera int
	HorasMedias                 float64
}

func leerJSONL[T any](path string) []T {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []T
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		var v T
		if json.Unmarshal(sc.Bytes(), &v) == nil {
			out = append(out, v)
		}
	}
	return out
}

func (r Repo) Metricas(desde time.Time) Metricas {
	var m Metricas
	var horas float64
	m.PorRuta = map[string]MetricasRuta{RutaFix: {}, RutaSlice: {}}
	horasRuta := map[string]float64{}
	for _, reg := range leerJSONL[Registro](r.logPath()) {
		if reg.Cerrado.Before(desde) {
			continue
		}
		m.Intentos += max(reg.Intento, 1)
		if reg.Ascendido {
			m.Ascendidos++
		}
		if reg.Result == "aceptado" {
			ruta := RutaSlice // los registros anteriores no traen ruta
			if reg.Ruta == RutaFix {
				ruta = RutaFix
			}
			mr := m.PorRuta[ruta]
			mr.Aceptados++
			m.Aceptados++
			h := reg.Cerrado.Sub(reg.Abierto).Hours()
			horas += h
			horasRuta[ruta] += h
			if reg.Intento <= 1 {
				m.VerdesALaPrimera++
				mr.VerdesALaPrimera++
			}
			m.PorRuta[ruta] = mr
		} else {
			m.Rechazados++
		}
	}
	if m.Aceptados > 0 {
		m.HorasMedias = horas / float64(m.Aceptados)
	}
	for ruta, mr := range m.PorRuta {
		if mr.Aceptados > 0 {
			mr.HorasMedias = horasRuta[ruta] / float64(mr.Aceptados)
			m.PorRuta[ruta] = mr
		}
	}
	prev := map[string]string{} // attempt → head del último fallo
	for _, rc := range leerJSONL[Recibo](r.recibosPath()) {
		if rc.TS.Before(desde) {
			continue
		}
		m.Verificaciones++
		if rc.Resultado == "fallo" {
			if prev[rc.Attempt] == rc.Head {
				m.FallosSinProgreso++
			}
			prev[rc.Attempt] = rc.Head
		}
	}
	raw, _ := os.ReadFile(r.CuarentenaPath())
	m.Cuarentena = len(strings.Fields(strings.ReplaceAll(string(raw), " ", "_")))
	return m
}
