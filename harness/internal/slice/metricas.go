package slice

import (
	"bufio"
	"encoding/json"
	"os"
	"slices"
	"strings"
	"time"

	"orq/internal/diario"
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
	// Del diario: minutos de verificado a decidido (mediana), cuántas decisiones la forman, cuántos
	// verificados llevan más de 24 h sin decisión y la causa de cada intento nuevo.
	DecisionMinMediana float64
	Decididos          int
	SinDecidir24h      int
	IntentosPorCausa   map[string]int
	// De la tarjeta: minutos de lista a aprobada (mediana) y cuántas aprobadas la forman; exámenes
	// corregidos tras abrir, en total y por cada 10 slices cerrados.
	TarjetaMinMediana  float64
	TarjetasAprobadas  int
	CorreccionesExamen int
	CorreccionesPor10  float64
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

func (r Repo) Metricas(desde, now time.Time) Metricas {
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
	evs, _ := diario.Leer(diario.Ruta(r.OrqDir))
	metricasDecision(&m, evs, desde, now)
	metricasTarjeta(&m, evs, desde)
	raw, _ := os.ReadFile(r.CuarentenaPath())
	m.Cuarentena = len(strings.Fields(strings.ReplaceAll(string(raw), " ", "_")))
	return m
}

// metricasDecision mide la espera del Owner. El reloj de un slice arranca con su última verificación
// que pasa y se detiene con su decisión; reabrirlo o cerrarlo sin decisión lo detiene sin contar.
func metricasDecision(m *Metricas, evs []diario.Evento, desde, now time.Time) {
	m.IntentosPorCausa = map[string]int{}
	verificado := map[string]time.Time{}
	var minutos []float64
	for _, e := range evs {
		switch {
		case e.Tipo == "verificacion" && e.Resultado == "paso":
			if _, ya := verificado[e.Slice]; !ya { // re-verifying keeps the first paso: the Owner has been waiting since then
				verificado[e.Slice] = e.TS
			}
		case e.Tipo == "decision":
			if v, ok := verificado[e.Slice]; ok && !e.TS.Before(desde) {
				minutos = append(minutos, e.TS.Sub(v).Minutes())
			}
			delete(verificado, e.Slice)
		case e.Tipo == "fase" && (e.De == Verificado && e.A != Integrando || e.A == "CERRADO"):
			delete(verificado, e.Slice)
		case e.Tipo == "intento" && !e.TS.Before(desde):
			if c, ok := e.Datos["causa"].(string); ok {
				m.IntentosPorCausa[c]++
			}
		}
	}
	for _, v := range verificado {
		if now.Sub(v) > 24*time.Hour {
			m.SinDecidir24h++
		}
	}
	m.Decididos, m.DecisionMinMediana = len(minutos), mediana(minutos)
}

// metricasTarjeta mide cuánto espera una tarjeta al Owner: desde su última versión lista hasta la
// aprobación. Las correcciones de examen son los exámenes sumados o corregidos tras abrir.
func metricasTarjeta(m *Metricas, evs []diario.Evento, desde time.Time) {
	lista := map[string]time.Time{}
	var minutos []float64
	for _, e := range evs {
		switch {
		case e.Tipo == "tarjeta" && e.Resultado == "lista":
			lista[e.Slice] = e.TS
		case e.Tipo == "tarjeta" && e.Resultado == "aprobada":
			if l, ok := lista[e.Slice]; ok && !e.TS.Before(desde) {
				minutos = append(minutos, e.TS.Sub(l).Minutes())
			}
			delete(lista, e.Slice)
		case e.Tipo == "examen" && !e.TS.Before(desde):
			m.CorreccionesExamen++
		}
	}
	m.TarjetasAprobadas, m.TarjetaMinMediana = len(minutos), mediana(minutos)
	if cerrados := m.Aceptados + m.Rechazados; cerrados > 0 {
		m.CorreccionesPor10 = 10 * float64(m.CorreccionesExamen) / float64(cerrados)
	}
}

func mediana(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	slices.Sort(xs)
	mitad := len(xs) / 2
	if len(xs)%2 == 0 {
		return (xs[mitad-1] + xs[mitad]) / 2
	}
	return xs[mitad]
}
