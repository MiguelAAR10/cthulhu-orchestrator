package slice

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"orq/internal/diario"
)

// Tarjeta es la ficha de entendimiento que el Owner lee y aprueba antes de que empiece un worker:
// la historia, lo que no se hará, el examen en palabras, las zonas que se tocan y lo que falta saber.
// Vive en el contrato: una vez aprobada no cambia.
type Tarjeta struct {
	Historia  []string  `json:"historia"`
	Fuera     []string  `json:"fuera"`
	Examen    []string  `json:"examen"`
	Zonas     []string  `json:"zonas"`
	Preguntas []string  `json:"preguntas"`
	Lista     time.Time `json:"lista"`
	Aprobada  time.Time `json:"aprobada,omitempty"`
}

func (t *Tarjeta) EstaAprobada() bool { return t != nil && !t.Aprobada.IsZero() }

// Topes de la tarjeta: se lee en 3 minutos.
const (
	maxLineasHistoria = 3
	maxLineasSeccion  = 5
	maxCaracteres     = 200
	maxPalabras       = 400
)

// TopesTarjeta dice los topes en una frase, con los mismos números que LeerTarjeta aplica.
func TopesTarjeta() string {
	return fmt.Sprintf("`## Historia` ≤%d lines, every other section ≤%d lines, every line ≤%d characters, ≤%d words in all",
		maxLineasHistoria, maxLineasSeccion, maxCaracteres, maxPalabras)
}

var seccionesTarjeta = []string{"Historia", "Fuera", "Examen", "Zonas", "Preguntas"}

// LeerTarjeta lee una tarjeta en markdown: una sección «## Nombre» por parte, en cualquier orden.
func LeerTarjeta(texto string) (Tarjeta, error) {
	partes := map[string][]string{}
	actual, palabras := "", 0
	for _, l := range strings.Split(texto, "\n") {
		l = strings.TrimSpace(l)
		if titulo, ok := strings.CutPrefix(l, "## "); ok {
			actual = ""
			for _, s := range seccionesTarjeta {
				if strings.EqualFold(strings.TrimSpace(titulo), s) {
					actual = s
				}
			}
			if actual == "" {
				return Tarjeta{}, fmt.Errorf("sección desconocida %q: la tarjeta tiene %s", titulo, strings.Join(seccionesTarjeta, ", "))
			}
			if _, ya := partes[actual]; ya {
				return Tarjeta{}, fmt.Errorf("la sección %s está dos veces", actual)
			}
			partes[actual] = nil
			continue
		}
		if l == "" || actual == "" && strings.HasPrefix(l, "# ") {
			continue
		}
		if actual == "" {
			return Tarjeta{}, fmt.Errorf("texto fuera de una sección: %q", l)
		}
		l = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(l, "- "), "* "))
		if n := utf8.RuneCountInString(l); n > maxCaracteres {
			return Tarjeta{}, fmt.Errorf("en %s, una línea tiene %d caracteres; el tope es %d", actual, n, maxCaracteres)
		}
		palabras += len(strings.Fields(l))
		partes[actual] = append(partes[actual], l)
	}
	if palabras > maxPalabras {
		return Tarjeta{}, fmt.Errorf("la tarjeta tiene %d palabras; el tope es %d (se lee en 3 minutos)", palabras, maxPalabras)
	}
	for _, s := range seccionesTarjeta {
		tope := maxLineasSeccion
		if s == "Historia" {
			tope = maxLineasHistoria
		}
		switch n := len(partes[s]); {
		case n == 0:
			return Tarjeta{}, fmt.Errorf("falta la sección %s o está vacía (si no hay nada, escribe «ninguna»)", s)
		case n > tope:
			return Tarjeta{}, fmt.Errorf("la sección %s tiene %d líneas; el tope es %d", s, n, tope)
		}
	}
	for _, z := range partes["Zonas"] {
		if filepath.IsAbs(z) || z == ".." || strings.HasPrefix(filepath.Clean(z), "../") {
			return Tarjeta{}, fmt.Errorf("en Zonas, %q no es una ruta dentro del repo", z)
		}
		if c := strings.Trim(filepath.Clean(z), "/"); c == "." || c == "" {
			return Tarjeta{}, fmt.Errorf("en Zonas, %q cubre todo el repo: nombra las carpetas o archivos que se tocan, no la raíz", z)
		}
	}
	return Tarjeta{Historia: partes["Historia"], Fuera: partes["Fuera"], Examen: partes["Examen"], Zonas: partes["Zonas"], Preguntas: partes["Preguntas"]}, nil
}

// PonerTarjeta guarda la tarjeta del slice nn y la sella con el contrato. Se reescribe mientras
// espera al Owner; aprobada, queda congelada.
func (r Repo) PonerTarjeta(nn string, t Tarjeta, now time.Time) (*Estado, error) {
	soltar, err := r.bloqueo(nn)
	if err != nil {
		return nil, err
	}
	defer soltar()
	e, err := r.Load(nn)
	if err != nil || e == nil {
		return nil, errors.Join(err, errors.New("no hay slice abierto"))
	}
	if err := r.confiable(e); err != nil {
		return e, err
	}
	if e.Tarjeta.EstaAprobada() {
		return e, fmt.Errorf("la tarjeta de %s ya está aprobada y quedó congelada con el contrato; un cambio de examen va por orq slice examen y se ve al cerrar", e.ID())
	}
	t.Lista, t.Aprobada = now.UTC().Truncate(time.Millisecond), time.Time{}
	e.Tarjeta = &t
	return e, r.guardarTarjeta(e, "lista", t.Lista)
}

// AprobarTarjeta sella la aprobación del Owner. Repetirla no sella otra vez (ya=true).
func (r Repo) AprobarTarjeta(nn string, now time.Time) (e *Estado, ya bool, err error) {
	soltar, err := r.bloqueo(nn)
	if err != nil {
		return nil, false, err
	}
	defer soltar()
	if e, err = r.Load(nn); err != nil || e == nil {
		return nil, false, errors.Join(err, errors.New("no hay slice abierto"))
	}
	if err := r.confiable(e); err != nil {
		return e, false, err
	}
	if e.Tarjeta == nil {
		return e, false, fmt.Errorf("%s no tiene tarjeta: el Planificador la escribe con orq slice tarjeta %s --archivo tarjeta.md", e.ID(), e.NN)
	}
	if e.Tarjeta.EstaAprobada() {
		return e, true, nil
	}
	e.Tarjeta.Aprobada = now.UTC().Truncate(time.Millisecond)
	return e, false, r.guardarTarjeta(e, "aprobada", e.Tarjeta.Aprobada)
}

func (r Repo) guardarTarjeta(e *Estado, resultado string, ts time.Time) error {
	if err := r.sellar(e); err != nil {
		return err
	}
	if err := r.save(e); err != nil {
		return err
	}
	return diario.Anotar(diario.Ruta(r.OrqDir), diario.Evento{TS: ts, Tipo: "tarjeta", Slice: e.NN, Intento: e.Intento, Ruta: e.RutaEfectiva(), Resultado: resultado})
}

// PuedeLanzar dice si un worker puede empezar: un fix no pide tarjeta; un slice, una aprobada.
func (e *Estado) PuedeLanzar() error {
	switch {
	case e.RutaEfectiva() == RutaFix || e.Tarjeta.EstaAprobada():
		return nil
	case e.Tarjeta == nil:
		return fmt.Errorf("%s no tiene tarjeta de entendimiento: escríbela con orq slice tarjeta %s --archivo tarjeta.md y que el Owner la apruebe antes de lanzar", e.ID(), e.NN)
	}
	return fmt.Errorf("la tarjeta de %s espera al Owner: la lee con orq slice tarjeta %s y la aprueba con orq slice aprobar %s; luego se lanza", e.ID(), e.NN, e.NN)
}
