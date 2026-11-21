// Package memoria guarda la única memoria del proyecto: una línea JSON por hecho,
// con etiquetas cerradas y un tope de líneas. Lo crítico nunca se compacta.
package memoria

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"
	"time"

	"orq/internal/identidad"
	"orq/internal/orqdir"
)

var (
	Imps    = []string{"critica", "alta", "media", "baja"}
	Cats    = []string{"requisito", "decision", "dominio", "tecnico", "riesgo", "aprendizaje"}
	Fuentes = []string{"owner", "atena", "filtro", "liaison", "orquestador", "planificador", "astra", "scout", "worker", "escriba"}
	// Tipos separan lo decidido, lo comprobado y lo supuesto: una hipótesis nunca se lee como regla.
	Tipos = []string{"decision", "hecho", "hipotesis"}
)

const (
	MaxTexto = 200
	Tope     = 60
)

type Item struct {
	ID     string    `json:"id"`
	TS     time.Time `json:"ts"`
	Imp    string    `json:"imp"`
	Cat    string    `json:"cat"`
	Fuente string    `json:"fuente"`
	Texto  string    `json:"texto"`
	Slice  string    `json:"slice,omitempty"`
	// Conf dice si el hecho lo confirmó el Owner o el Atena, o si es una inferencia de agente.
	Conf string `json:"conf,omitempty"`
	// Reemplaza apunta al id que este hecho deja obsoleto; el obsoleto ya no se muestra.
	Reemplaza string `json:"reemplaza,omitempty"`
	Tipo      string `json:"tipo,omitempty"`
	Evidencia string `json:"evidencia,omitempty"` // recibo, commit, archivo:línea o id de mensaje
	Vence     string `json:"vence,omitempty"`     // S-NN: deja de valer al cerrarse ese slice
}

// TipoEfectivo da tipo también a líneas viejas: una decisión es decisión; lo inferido, hipótesis.
func (it Item) TipoEfectivo() string {
	switch {
	case it.Tipo != "":
		return it.Tipo
	case it.Cat == "decision":
		return "decision"
	case it.Conf == "inferido":
		return "hipotesis"
	}
	return "hecho"
}

// confirma dice qué fuentes pueden afirmar hechos confirmados.
func confirma(fuente string) bool {
	return fuente == "owner" || fuente == "atena" || fuente == "filtro"
}

func (it Item) Validate() error {
	switch {
	case !slices.Contains(Imps, it.Imp):
		return fmt.Errorf("imp %q: usa %s", it.Imp, strings.Join(Imps, "|"))
	case !slices.Contains(Cats, it.Cat):
		return fmt.Errorf("cat %q: usa %s", it.Cat, strings.Join(Cats, "|"))
	case !slices.Contains(Fuentes, it.Fuente):
		return fmt.Errorf("fuente %q: usa %s", it.Fuente, strings.Join(Fuentes, "|"))
	case strings.TrimSpace(it.Texto) == "":
		return errors.New("texto vacío")
	case len([]rune(it.Texto)) > MaxTexto:
		return fmt.Errorf("texto de %d caracteres; máximo %d: una idea por línea", len([]rune(it.Texto)), MaxTexto)
	case it.Imp == "critica" && !confirma(it.Fuente):
		return errors.New("solo el Owner o el Atena marcan algo como crítico; un agente lo propone al Atena")
	case it.Tipo != "" && !slices.Contains(Tipos, it.Tipo):
		return fmt.Errorf("tipo %q: usa %s", it.Tipo, strings.Join(Tipos, "|"))
	case it.Tipo == "hipotesis" && it.Imp == "critica":
		return errors.New("una hipótesis no puede ser crítica: compruébala primero")
	case it.Tipo == "hecho" && !confirma(it.Fuente) && it.Fuente != "escriba" && strings.TrimSpace(it.Evidencia) == "":
		return errors.New("un hecho de agente necesita --evidencia (recibo, commit, archivo:línea); si no la tienes, es --tipo hipotesis")
	}
	return nil
}

// LineasMalas reports memory lines that could not be read. Load still returns every good line:
// one torn line never hides the rest of the memory.
type LineasMalas struct {
	Path   string
	Lineas []int
	Errs   []string
}

func (e *LineasMalas) Error() string {
	var ls []string
	for i, n := range e.Lineas {
		ls = append(ls, fmt.Sprintf("%s:%d (%s)", e.Path, n, e.Errs[i]))
	}
	return fmt.Sprintf("memoria: %d línea(s) ilegibles, se omiten y el resto vale; corrígelas o bórralas a mano: %s", len(e.Lineas), strings.Join(ls, "; "))
}

// Load lee la memoria. Una línea ilegible se salta y se informa con *LineasMalas junto con los
// ítems buenos; solo un error de lectura del archivo devuelve nil.
func Load(path string) ([]Item, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Item
	var malas LineasMalas
	sc := bufio.NewScanner(bytes.NewReader(raw))
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var it Item
		if err := json.Unmarshal([]byte(line), &it); err != nil {
			malas.Lineas, malas.Errs = append(malas.Lineas, n), append(malas.Errs, err.Error())
			continue
		}
		out = append(out, it)
	}
	if err := sc.Err(); err != nil {
		return out, err
	}
	if len(malas.Lineas) > 0 {
		malas.Path = path
		if !identidad.EsOrq2() {
			return nil, Estricto(&malas) // orq keeps 1.0.0: one bad line fails the whole memory
		}
		return out, &malas
	}
	return out, nil
}

// Estricto is orq 1.0.0's reading: the first unreadable line fails the whole memory, with
// 1.0.0's message. orq (not orq2) keeps it so its output and exit codes do not change.
func Estricto(err error) error {
	var malas *LineasMalas
	if errors.As(err, &malas) {
		return fmt.Errorf("%s:%d: %s", malas.Path, malas.Lineas[0], malas.Errs[0])
	}
	return err
}

// Legible quita de err el aviso de líneas ilegibles: lo que queda impide confiar en la lectura.
func Legible(err error) error {
	var malas *LineasMalas
	if errors.As(err, &malas) {
		return nil
	}
	return err
}

func Save(path string, items []Item) error {
	var b bytes.Buffer
	for _, it := range items {
		line, err := json.Marshal(it)
		if err != nil {
			return err
		}
		b.Write(line)
		b.WriteByte('\n')
	}
	return orqdir.WriteAtomic(path, b.Bytes())
}

// Add valida, asigna id y agrega. Devuelve el ítem guardado y cuántas líneas hay.
func Add(path string, it Item, now time.Time) (Item, int, error) {
	if err := it.Validate(); err != nil {
		return it, 0, err
	}
	soltar, err := orqdir.Candado(path)
	if err != nil {
		return it, 0, err
	}
	defer soltar()
	// Lines nobody can read stay on disk as they are: Add appends and never rewrites the file.
	items, err := Load(path)
	if err := Legible(err); err != nil {
		return it, 0, err
	}
	if it.Reemplaza != "" {
		i := slices.IndexFunc(items, func(o Item) bool { return o.ID == it.Reemplaza })
		if i < 0 {
			return it, 0, fmt.Errorf("no existe %s para reemplazar", it.Reemplaza)
		}
		if (items[i].Imp == "critica" || items[i].Conf == "confirmado") && !confirma(it.Fuente) {
			return it, 0, fmt.Errorf("%s lo confirmó el Owner o el Atena: solo ellos lo reemplazan; propónselo al Atena", it.Reemplaza)
		}
	}
	it.Conf = "inferido"
	if confirma(it.Fuente) {
		it.Conf = "confirmado"
	}
	it.TS = now.UTC().Truncate(time.Second)
	it.ID = nextID(items)
	return it, len(items) + 1, agregar(path, it)
}

// agregar escribe una línea al final; si la última quedó cortada sin salto, primero la cierra.
func agregar(path string, it Item) error {
	line, err := json.Marshal(it)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if info, err := f.Stat(); err != nil {
		return err
	} else if info.Size() > 0 {
		last := make([]byte, 1)
		if _, err := f.ReadAt(last, info.Size()-1); err != nil {
			return err
		}
		if last[0] != '\n' {
			line = append([]byte{'\n'}, line...)
		}
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		return err
	}
	return f.Sync()
}

func nextID(items []Item) string {
	max := 0
	for _, it := range items {
		var n int
		if _, err := fmt.Sscanf(it.ID, "m%d", &n); err == nil && n > max {
			max = n
		}
	}
	return fmt.Sprintf("m%d", max+1)
}

func rank(imp string) int { return slices.Index(Imps, imp) }

// Sorted ordena por importancia y, dentro de cada nivel, lo más reciente primero.
func Sorted(items []Item) []Item {
	out := slices.Clone(items)
	sort.SliceStable(out, func(i, j int) bool {
		if rank(out[i].Imp) != rank(out[j].Imp) {
			return rank(out[i].Imp) < rank(out[j].Imp)
		}
		return out[i].TS.After(out[j].TS)
	})
	return out
}

// Vigentes quita los hechos que otro hecho reemplazó.
func Vigentes(items []Item) []Item {
	obsoletos := map[string]bool{}
	for _, it := range items {
		if it.Reemplaza != "" {
			obsoletos[it.Reemplaza] = true
		}
	}
	var out []Item
	for _, it := range items {
		if !obsoletos[it.ID] {
			out = append(out, it)
		}
	}
	return out
}

// SinVencer quita lo que vencía con un slice ya cerrado.
func SinVencer(items []Item, cerrados map[string]bool) []Item {
	var out []Item
	for _, it := range items {
		if it.Vence == "" || !cerrados[it.Vence] {
			out = append(out, it)
		}
	}
	return out
}

// Render es la vista compacta que cada sesión lee al arrancar: solo lo vigente.
func Render(items []Item, filtroCat string) string {
	var b strings.Builder
	for _, it := range Sorted(Vigentes(items)) {
		if filtroCat != "" && it.Cat != filtroCat {
			continue
		}
		s := ""
		if it.Slice != "" {
			s = " · " + it.Slice
		}
		if t := it.TipoEfectivo(); t == "hipotesis" {
			s += " · HIPÓTESIS"
		} else if it.Conf == "inferido" {
			s += " · inferido"
		}
		if it.Evidencia != "" {
			s += " · ev: " + it.Evidencia
		}
		fmt.Fprintf(&b, "%-4s [%s|%s] %s (%s%s)\n", it.ID, it.Imp, it.Cat, it.Texto, it.Fuente, s)
	}
	return b.String()
}

// Summarizer recibe lo no crítico y devuelve una versión más corta. Lo implementa el escriba.
type Summarizer func(rest []Item, max int) ([]Item, error)

// Compact deja intacto lo crítico y pide al resumidor que el resto quepa en el tope.
// Rechaza la salida si rompe etiquetas o se pasa del tope: la memoria no cambia.
func Compact(items []Item, tope int, sum Summarizer, now time.Time) ([]Item, error) {
	items = Vigentes(items)
	for i := range items {
		items[i].Reemplaza = ""
	}
	if len(items) <= tope {
		return items, nil
	}
	// Lo crítico y lo confirmado por el Owner o el Atena no se resume: se conserva tal cual.
	var crit, rest []Item
	for _, it := range items {
		if it.Imp == "critica" || it.Conf == "confirmado" {
			crit = append(crit, it)
		} else {
			rest = append(rest, it)
		}
	}
	room := tope - len(crit)
	if room < 1 {
		return nil, fmt.Errorf("hay %d líneas críticas o confirmadas y el tope es %d: el Owner debe decidir cuáles siguen vigentes", len(crit), tope)
	}
	out, err := sum(rest, room)
	if err != nil {
		return nil, err
	}
	if len(out) > room {
		return nil, fmt.Errorf("el resumen trajo %d líneas y caben %d; memoria sin cambios", len(out), room)
	}
	res := slices.Clone(crit)
	for _, it := range out {
		if it.Imp == "critica" {
			return nil, errors.New("el resumen inventó una línea crítica; memoria sin cambios")
		}
		// Un resumen es una inferencia: nunca asciende una hipótesis a hecho ni a confirmado.
		it.Fuente, it.Conf, it.Tipo = "escriba", "inferido", "hipotesis"
		if err := it.Validate(); err != nil {
			return nil, fmt.Errorf("línea del resumen inválida (%v); memoria sin cambios", err)
		}
		if it.TS.IsZero() {
			it.TS = now.UTC().Truncate(time.Second)
		}
		res = append(res, it)
	}
	for i := range res {
		if res[i].ID == "" {
			res[i].ID = nextID(res)
		}
	}
	return res, nil
}
