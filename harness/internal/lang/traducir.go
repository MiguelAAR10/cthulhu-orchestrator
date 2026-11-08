package lang

import (
	"io"
	"regexp"
	"sort"
	"strings"
	"sync"
)

type patron struct {
	re  *regexp.Regexp
	en  []string // literal pieces of the English template, around the captures
	lit int
}

var (
	verboRe  = regexp.MustCompile(`%[-+# 0]*[0-9]*(\.[0-9]+)?[a-zA-Z]`)
	patrones []patron
	unaVez   sync.Once
)

func compilar() {
	for _, p := range tabla {
		es, en := strings.Split(p.es, "\n"), strings.Split(p.en, "\n")
		if len(es) != len(en) {
			continue
		}
		for i := range es {
			lit := verboRe.ReplaceAllString(es[i], "")
			if len(strings.TrimSpace(lit)) < 4 {
				continue
			}
			partes := verboRe.Split(es[i], -1)
			var b strings.Builder
			for j, s := range partes {
				if j > 0 {
					if j == len(partes)-1 && s == "" {
						b.WriteString("(.*)")
					} else {
						b.WriteString("(.*?)")
					}
				}
				b.WriteString(regexp.QuoteMeta(strings.ReplaceAll(s, "%%", "%")))
			}
			ens := verboRe.Split(en[i], -1)
			if len(ens) != len(partes) {
				continue
			}
			for j := range ens {
				ens[j] = strings.ReplaceAll(ens[j], "%%", "%")
			}
			patrones = append(patrones, patron{regexp.MustCompile(b.String()), ens, len(lit)})
		}
	}
	sort.SliceStable(patrones, func(a, b int) bool { return patrones[a].lit > patrones[b].lit })
}

// Traducir passes orq's Spanish messages to English, line by line (unless ORQ_LANG=es).
// Unknown text is left as is.
func Traducir(s string) string {
	if ES() || s == "" {
		return s
	}
	unaVez.Do(compilar)
	lineas := strings.Split(s, "\n")
	for i, l := range lineas {
		lineas[i] = traducirLinea(l)
	}
	return strings.Join(lineas, "\n")
}

// terminos: fragmentos que llegan como datos (fase, motivo, resultado) dentro de una línea ya
// traducida. Se reemplazan después del patrón; los datos guardados siguen en español.
var terminos = strings.NewReplacer(
	"el examen no pasa; falla igual sin cambios reales: sin progreso", "the exam does not pass; it fails the same with no real changes: no progress",
	"el examen no pasa", "the exam does not pass",
	"IMPLEMENTANDO", "IMPLEMENTING", "VERIFICADO", "VERIFIED", "BLOQUEADO", "BLOCKED",
	"INTEGRANDO", "INTEGRATING", "CERRANDO", "CLOSING",
	"closed as rechazado", "closed as rejected", "closed as aceptado", "closed as accepted",
	"(nadie)", "(nobody)",
	"mode: nuevo", "mode: new", "mode: mejora", "mode: improve",
)

func traducirLinea(l string) string {
	return terminos.Replace(traducirPatron(l))
}

func traducirPatron(l string) string {
	for _, p := range patrones {
		m := p.re.FindStringSubmatchIndex(l)
		if m == nil {
			continue
		}
		var b strings.Builder
		b.WriteString(l[:m[0]])
		for j, s := range p.en {
			if j > 0 {
				b.WriteString(l[m[2*j]:m[2*j+1]])
			}
			b.WriteString(s)
		}
		b.WriteString(l[m[1]:])
		return b.String()
	}
	return l
}

// LineWriter buffers until each newline and writes the translated line.
type LineWriter struct {
	w   io.Writer
	buf []byte
}

func Writer(w io.Writer) *LineWriter { return &LineWriter{w: w} }

func (t *LineWriter) Write(p []byte) (int, error) {
	t.buf = append(t.buf, p...)
	if i := strings.LastIndexByte(string(t.buf), '\n'); i >= 0 {
		out := Traducir(string(t.buf[:i+1]))
		t.buf = append([]byte{}, t.buf[i+1:]...)
		if _, err := io.WriteString(t.w, out); err != nil {
			return 0, err
		}
	}
	return len(p), nil
}

// Flush writes what is left without a trailing newline.
func (t *LineWriter) Flush() {
	if len(t.buf) > 0 {
		io.WriteString(t.w, Traducir(string(t.buf)))
		t.buf = nil
	}
}
