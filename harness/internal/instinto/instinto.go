// Package instinto es el aprendizaje del harness: lecciones cortas con confianza que suben
// cuando se repiten o se confirman, bajan cuando se refutan y se podan solas. Al arrancar,
// cada sesión recibe solo las pocas más confiables: aprender sin llenar el contexto.
package instinto

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"orq/internal/orqdir"
)

const (
	MaxTexto  = 160
	Inicial   = 0.5  // una lección nueva es hipótesis
	Umbral    = 0.7  // desde aquí se inyecta al arrancar
	Piso      = 0.3  // por debajo se poda
	Techo     = 0.95 // nada es certeza
	Inyectar  = 6    // máximo por sesión
	Repetida  = 0.1  // la misma lección vuelve a aparecer
	Confirmar = 0.15
	Refutar   = 0.3
)

type Instinto struct {
	ID        string    `json:"id"`
	Texto     string    `json:"texto"`
	Conf      float64   `json:"conf"`
	Visto     int       `json:"visto"`
	Fuente    string    `json:"fuente"` // reintento | rechazo | alineacion | orquestador | owner
	Evidencia string    `json:"evidencia,omitempty"`
	TS        time.Time `json:"ts"`
}

// Global es el archivo compartido entre proyectos: lo que vale en todos lados.
func Global() string {
	if d := os.Getenv("ORQ_CONFIG"); d != "" {
		return filepath.Join(d, "instintos.json")
	}
	h, _ := os.UserHomeDir()
	return filepath.Join(h, ".config", "orq", "instintos.json")
}

func clave(s string) string { return strings.ToLower(strings.Join(strings.Fields(s), " ")) }

func Leer(path string) ([]Instinto, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var l []Instinto
	return l, json.Unmarshal(raw, &l)
}

func guardar(path string, l []Instinto) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, _ := json.MarshalIndent(l, "", " ")
	return orqdir.WriteAtomic(path, raw)
}

func editar(path string, f func([]Instinto) ([]Instinto, error)) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	soltar, err := orqdir.Candado(path)
	if err != nil {
		return err
	}
	defer soltar()
	l, err := Leer(path)
	if err != nil {
		return err
	}
	if l, err = f(l); err != nil {
		return err
	}
	return guardar(path, l)
}

func acotar(c float64) float64 { return min(Techo, max(0, c)) }

// Aprender agrega una lección; si ya existe (mismo texto), sube su confianza en vez de duplicarla.
func Aprender(path, texto, fuente, evidencia string, now time.Time) (Instinto, error) {
	texto = strings.TrimSpace(texto)
	if texto == "" {
		return Instinto{}, errors.New("lección vacía")
	}
	if n := len([]rune(texto)); n > MaxTexto {
		return Instinto{}, fmt.Errorf("lección de %d caracteres; máximo %d: una idea, accionable", n, MaxTexto)
	}
	var out Instinto
	err := editar(path, func(l []Instinto) ([]Instinto, error) {
		for i := range l {
			if clave(l[i].Texto) == clave(texto) {
				l[i].Conf, l[i].Visto, l[i].TS = acotar(l[i].Conf+Repetida), l[i].Visto+1, now.UTC()
				if evidencia != "" {
					l[i].Evidencia = evidencia
				}
				out = l[i]
				return l, nil
			}
		}
		n := 0
		for _, x := range l {
			var k int
			fmt.Sscanf(x.ID, "i%d", &k)
			n = max(n, k)
		}
		out = Instinto{ID: fmt.Sprintf("i%d", n+1), Texto: texto, Conf: Inicial, Visto: 1, Fuente: fuente, Evidencia: evidencia, TS: now.UTC()}
		return append(l, out), nil
	})
	return out, err
}

// Ajustar confirma (delta>0) o refuta (delta<0). Lo que cae bajo el piso se poda y vuelve con ok=false.
func Ajustar(path, id string, delta float64) (Instinto, bool, error) {
	var out Instinto
	vive := true
	err := editar(path, func(l []Instinto) ([]Instinto, error) {
		for i := range l {
			if l[i].ID == id {
				l[i].Conf = acotar(l[i].Conf + delta)
				out = l[i]
				if out.Conf < Piso {
					vive = false
					return append(l[:i:i], l[i+1:]...), nil
				}
				return l, nil
			}
		}
		return nil, fmt.Errorf("no existe %s", id)
	})
	return out, vive, err
}

// Promover copia una lección del proyecto al archivo global con su confianza.
func Promover(desde, hacia, id string, now time.Time) (Instinto, error) {
	l, err := Leer(desde)
	if err != nil {
		return Instinto{}, err
	}
	for _, x := range l {
		if x.ID == id {
			g, err := Aprender(hacia, x.Texto, x.Fuente, x.Evidencia, now)
			if err == nil && g.Conf < x.Conf {
				g, _, err = Ajustar(hacia, g.ID, x.Conf-g.Conf)
			}
			return g, err
		}
	}
	return Instinto{}, fmt.Errorf("no existe %s", id)
}

// Top da las n lecciones más confiables sobre el umbral, de varios archivos, sin repetir texto.
func Top(n int, paths ...string) []Instinto {
	var todas []Instinto
	visto := map[string]bool{}
	for _, p := range paths {
		l, _ := Leer(p)
		for _, x := range l {
			if x.Conf >= Umbral && !visto[clave(x.Texto)] {
				visto[clave(x.Texto)] = true
				todas = append(todas, x)
			}
		}
	}
	sort.SliceStable(todas, func(i, j int) bool {
		if todas[i].Conf != todas[j].Conf {
			return todas[i].Conf > todas[j].Conf
		}
		return todas[i].TS.After(todas[j].TS)
	})
	if len(todas) > n {
		todas = todas[:n]
	}
	return todas
}
