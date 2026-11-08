// Package board arma el tablero del Owner (orq board) solo leyendo .orq. Lo que no se puede
// calcular desde archivos es null en JSON y "unknown" en texto: nunca se estima.
package board

import (
	"bufio"
	"encoding/json"
	"fmt"
	"html"
	"math"
	"orq/internal/lang"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"orq/internal/config"
	"orq/internal/memoria"
	"orq/internal/plan"
	"orq/internal/slice"
	"orq/internal/stops"
	"orq/internal/watch"
	"orq/internal/workers"
)

const Schema = "orq.board/v1"

type Overall struct {
	Pct      *int `json:"pct"`
	Measured int  `json:"measured"`
	Total    int  `json:"total"`
	Rejected int  `json:"rejected"`
	Blocked  int  `json:"blocked"`
}

type Stopper struct {
	ID       string   `json:"id"`
	Type     string   `json:"type"`
	Severity string   `json:"severity"`
	Slice    string   `json:"slice"`
	Tasks    []string `json:"tasks"`
	Text     string   `json:"text"`
	AgeMin   int      `json:"age_min"`
	raised   time.Time
}

type TaskCount struct {
	Done        int `json:"done"`
	Total       int `json:"total"`
	WeightDone  int `json:"weight_done"`
	WeightTotal int `json:"weight_total"`
}

type Slice struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	Phase         string    `json:"phase"`
	Color         string    `json:"color"`
	Status        string    `json:"status"`
	Pct           *int      `json:"pct"`
	Tasks         TaskCount `json:"tasks"`
	Missing       []string  `json:"missing"`
	Who           []string  `json:"who"`
	Rework        int       `json:"rework"`
	BlockedReason string    `json:"blocked_reason"`
	// RejectedReason: por qué se cerró sin aceptar. Un rechazado cuenta 0 en el total: nada llegó.
	RejectedReason string  `json:"rejected_reason,omitempty"`
	Fronts         []Front `json:"fronts,omitempty"`
}

// Front agrupa las tareas de un slice por sub-orquestador (dato del plan).
type Front struct {
	ID      string    `json:"id"`
	Goal    string    `json:"goal"`
	Tasks   TaskCount `json:"tasks"`
	TaskIDs []string  `json:"task_ids"`
}

type WorkerTests struct {
	Passing int `json:"passing"`
	Total   int `json:"total"`
}

type Worker struct {
	ID            string       `json:"id"`
	Slice         string       `json:"slice"`
	Task          string       `json:"task"`
	Phase         string       `json:"phase"`
	Verdict       string       `json:"verdict"`
	HeartbeatAgeS *int         `json:"heartbeat_age_s"`
	Commits       int          `json:"commits"`
	Tests         *WorkerTests `json:"tests"`
	ElapsedMin    *int         `json:"elapsed_min"`
	BudgetMin     *int         `json:"budget_min"`
	ContextPct    *int         `json:"context_pct"`
	Summary       string       `json:"summary,omitempty"`
}

type Velocity struct {
	TasksPerHour *float64 `json:"tasks_per_hour"`
	WindowH      int      `json:"window_h"`
}

type Board struct {
	Schema      string     `json:"schema"`
	GeneratedAt time.Time  `json:"generated_at"`
	Overall     Overall    `json:"overall"`
	Stoppers    []Stopper  `json:"stoppers"`
	Slices      []Slice    `json:"slices"`
	Workers     []Worker   `json:"workers"`
	Velocity    Velocity   `json:"velocity"`
	Unverified  Unverified `json:"unverified"`
}

// Unverified cuenta los registros decisivos cuya firma no vale: se ignoran o se muestran marcados.
type Unverified struct {
	Memory   int  `json:"memory"`
	Stops    int  `json:"stops"`
	Tasks    int  `json:"tasks"`
	Autonomy bool `json:"autonomy"`
}

func (u Unverified) Any() bool { return u.Memory+u.Stops+u.Tasks > 0 || u.Autonomy }

var (
	colorRank = map[string]int{"red": 0, "amber": 1, "blue": 2, "grey": 3, "green": 4}
	statusOf  = map[string]string{"red": "BLOCKED", "amber": "AT RISK", "blue": "IN PROGRESS", "grey": "NOT PLANNED", "green": "DONE"}
	sevRank   = map[string]int{"blocking": 0, "soon": 1, "fyi": 2}
)

func weight(d string) int {
	switch d {
	case "baja":
		return 1
	case "alta":
		return 3
	}
	return 2 // media o vacío, igual que plan.Leer
}

func ip(n int) *int { return &n }

func hostname() string { h, _ := os.Hostname(); return h }

// Build lee .orq y arma el tablero. No escribe nada.
func Build(orqDir string, now time.Time) (Board, error) {
	b := Board{Schema: Schema, GeneratedAt: now.UTC(), Stoppers: []Stopper{}, Slices: []Slice{}, Workers: []Worker{}, Velocity: Velocity{WindowH: 24}}
	cfg, _ := config.Load(orqDir)

	all, err := stops.Load(stops.Path(orqDir))
	if err != nil {
		return b, err
	}
	open := stops.Open(all)
	for _, st := range all {
		if st.Unverified() {
			b.Unverified.Stops++
		}
	}
	b.Unverified.Autonomy = cfg.AutonomiaSinFirma
	for _, s := range open {
		b.Stoppers = append(b.Stoppers, Stopper{ID: s.ID, Type: s.Type, Severity: s.Severity, Slice: s.Slice,
			Tasks: append([]string{}, s.Tasks...), Text: s.Text, AgeMin: int(now.Sub(s.Raised).Minutes()), raised: s.Raised})
	}
	slices.SortStableFunc(b.Stoppers, func(x, y Stopper) int {
		if d := sevRank[x.Severity] - sevRank[y.Severity]; d != 0 {
			return d
		}
		return x.raised.Compare(y.raised)
	})

	if items, err := memoria.Load(filepath.Join(orqDir, "memoria.jsonl")); err == nil {
		for _, it := range items {
			if it.SinVerificar {
				b.Unverified.Memory++
			}
		}
	}

	// Workers y su veredicto vivo.
	ws, _ := workers.List(orqDir)
	verdictBySlice := map[string][]string{}
	host := hostname()
	for _, st := range ws {
		v := watch.Current(orqDir, st, now, cfg.StaleMin(), host)
		w := Worker{ID: st.ID, Slice: st.Slice, Task: st.Task, Phase: st.Phase, Verdict: v, Commits: st.Commits, ContextPct: st.ContextPct, Summary: st.Summary}
		if !st.Heartbeat.IsZero() {
			w.HeartbeatAgeS = ip(int(now.Sub(st.Heartbeat).Seconds()))
		}
		if !st.Started.IsZero() {
			w.ElapsedMin = ip(int(now.Sub(st.Started).Minutes()))
		}
		if st.BudgetMin > 0 {
			w.BudgetMin = ip(st.BudgetMin)
		}
		if st.Tests != nil {
			w.Tests = &WorkerTests{Passing: st.Tests.Passing, Total: st.Tests.Total}
		}
		b.Workers = append(b.Workers, w)
		verdictBySlice[st.Slice] = append(verdictBySlice[st.Slice], v)
	}

	events, _ := workers.LoadTaskEvents(orqDir)
	reopened := map[string]int{}
	var doneTS []time.Time
	for _, e := range events {
		if e.Event == "reopened" {
			reopened[e.Slice]++
		}
		if e.Event == "done" && !e.TS.After(now) && now.Sub(e.TS) <= 24*time.Hour {
			doneTS = append(doneTS, e.TS)
		}
	}
	if len(doneTS) >= 2 {
		slices.SortFunc(doneTS, func(a, c time.Time) int { return a.Compare(c) })
		h := math.Max(1, now.Sub(doneTS[0]).Hours())
		v := math.Round(float64(len(doneTS))/h*100) / 100
		b.Velocity.TasksPerHour = &v
	}

	// Slices: abiertos y cerrados (último registro de cada uno).
	type src struct {
		nn, title, phase, motivo string
		intento                  int
		accepted, closed         bool
	}
	var srcs []src
	seen := map[string]bool{}
	m, _ := filepath.Glob(filepath.Join(orqDir, "slices", "*.json"))
	slices.Sort(m)
	for _, f := range m {
		raw, err := os.ReadFile(f)
		var e slice.Estado
		if err != nil || json.Unmarshal(raw, &e) != nil || e.NN == "" {
			continue
		}
		seen[e.NN] = true
		srcs = append(srcs, src{nn: e.NN, title: e.Titulo, phase: e.Fase, motivo: e.Motivo, intento: e.Intento})
	}
	closed := map[string]slice.Registro{}
	var closedOrder []string
	if f, err := os.Open(filepath.Join(orqDir, "slices.jsonl")); err == nil {
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 1<<20), 1<<20)
		for sc.Scan() {
			var r slice.Registro
			if json.Unmarshal(sc.Bytes(), &r) == nil && r.NN != "" {
				if _, ok := closed[r.NN]; !ok {
					closedOrder = append(closedOrder, r.NN)
				}
				closed[r.NN] = r
			}
		}
		f.Close()
	}
	for _, nn := range closedOrder {
		if seen[nn] {
			continue
		}
		r := closed[nn]
		ph := "CLOSED"
		if r.Result != "aceptado" {
			ph = "CLOSED (rejected)"
		}
		srcs = append(srcs, src{nn: nn, title: r.Titulo, phase: ph, motivo: r.Motivo, intento: r.Intento, accepted: r.Result == "aceptado", closed: true})
	}

	sumW, sumWP := 0, 0
	var measuredPct []int
	sinPeso := false // un slice medido sin tareas no cabe en el promedio ponderado
	for _, s := range srcs {
		id := "S-" + s.nn
		c := Slice{ID: id, Title: s.title, Phase: s.phase, Missing: []string{}, Who: []string{}}
		c.Rework = reopened[id] + max(0, s.intento-1)
		hasPlan := false
		if pl, err := plan.Leer(plan.Path(orqDir, s.nn)); err == nil && len(pl.Tareas) > 0 {
			hasPlan = true
			for _, f := range pl.Fronts {
				g := Front{ID: f.ID, Goal: f.Goal, TaskIDs: []string{}}
				for _, t := range pl.Tareas {
					if t.Front != f.ID {
						continue
					}
					w := weight(t.Dificultad)
					g.Tasks.Total++
					g.Tasks.WeightTotal += w
					if t.Estado == "hecha" {
						g.Tasks.Done++
						g.Tasks.WeightDone += w
					}
					g.TaskIDs = append(g.TaskIDs, t.ID)
				}
				c.Fronts = append(c.Fronts, g)
			}
			for _, t := range pl.Tareas {
				if t.HechaSinFirma {
					b.Unverified.Tasks++
				}
				w := weight(t.Dificultad)
				c.Tasks.Total++
				c.Tasks.WeightTotal += w
				if t.Estado == "hecha" {
					c.Tasks.Done++
					c.Tasks.WeightDone += w
					continue
				}
				q := t.Que
				if q == "" {
					q = t.ID
				}
				c.Missing = append(c.Missing, q)
				if t.Estado == "en_curso" && t.Worker != "" && !slices.Contains(c.Who, t.Worker) {
					c.Who = append(c.Who, t.Worker)
				}
			}
			c.Pct = ip(int(math.Round(100 * float64(c.Tasks.WeightDone) / float64(c.Tasks.WeightTotal))))
		}
		done := s.phase == slice.Verificado || s.phase == slice.Integrando || s.accepted
		rejected := s.closed && !s.accepted
		if done {
			c.Pct = ip(100)
		}
		if rejected {
			c.Pct = ip(0)
			c.RejectedReason = s.motivo
			b.Overall.Rejected++
		}
		if !hasPlan && !done && !rejected {
			c.Missing = []string{lang.T("unknown (no plan)", "desconocido (sin plan)")}
		}
		if c.Pct != nil {
			measuredPct = append(measuredPct, *c.Pct)
			sumW += c.Tasks.WeightTotal
			sumWP += c.Tasks.WeightTotal * *c.Pct
			sinPeso = sinPeso || c.Tasks.WeightTotal == 0
		}
		var blocking, soon string
		for _, st := range open {
			if st.Slice != id {
				continue
			}
			if st.Severity == "blocking" && blocking == "" {
				blocking = st.ID
			}
			if st.Severity == "soon" && soon == "" {
				soon = st.ID
			}
		}
		risky := slices.ContainsFunc(verdictBySlice[id], func(v string) bool { return v == "stuck" || v == "dead" || v == "timeout" })
		switch {
		case !s.closed && (blocking != "" || s.phase == slice.Bloqueado):
			c.Color = "red"
			if blocking != "" {
				c.BlockedReason = lang.T("waiting on stop ", "esperando el stopper ") + blocking
			} else {
				c.BlockedReason = s.motivo
			}
		case !s.closed && (soon != "" || risky):
			c.Color = "amber"
		case done:
			c.Color = "green"
		case rejected:
			c.Color = "grey"
		case !hasPlan:
			c.Color = "grey"
		default:
			c.Color = "blue"
		}
		c.Status = statusOf[c.Color]
		if c.Color == "red" {
			b.Overall.Blocked++
		}
		if rejected {
			c.Status = "REJECTED"
		}
		b.Slices = append(b.Slices, c)
	}
	slices.SortStableFunc(b.Slices, func(x, y Slice) int { return colorRank[x.Color] - colorRank[y.Color] })

	b.Overall.Total, b.Overall.Measured = len(srcs), len(measuredPct)
	sumP := 0
	for _, p := range measuredPct {
		sumP += p
	}
	switch {
	case len(measuredPct) == 0:
	case len(measuredPct) == len(srcs) && sumW > 0 && !sinPeso:
		b.Overall.Pct = ip(int(math.Round(float64(sumWP) / float64(sumW))))
	default: // lo no medido cuenta 0: el total nunca habla solo de lo que se pudo medir
		b.Overall.Pct = ip(int(math.Round(float64(sumP) / float64(len(srcs)))))
	}
	if p := b.Overall.Pct; p != nil && *p >= 100 && (b.Overall.Blocked > 0 || b.Overall.Measured < b.Overall.Total) {
		*p = 99
	}
	return b, nil
}

func JSON(b Board) ([]byte, error) { return json.MarshalIndent(b, "", "  ") }

// ---------- texto ----------

var ansi = map[string]string{"red": "\x1b[31m", "amber": "\x1b[33m", "green": "\x1b[32m", "blue": "\x1b[34m", "grey": "\x1b[90m"}

func desconocido() string { return lang.T("unknown", "desconocido") }

// palabra: la etiqueta guardada dicha para el Owner, en su idioma. Los datos y el JSON no cambian.
var palabras = map[string][2]string{
	"BLOCKED": {"BLOCKED", "BLOQUEADO"}, "AT RISK": {"AT RISK", "EN RIESGO"}, "IN PROGRESS": {"IN PROGRESS", "EN CURSO"},
	"NOT PLANNED": {"NOT PLANNED", "SIN PLAN"}, "DONE": {"DONE", "LISTO"}, "REJECTED": {"REJECTED", "RECHAZADO"},
	slice.Implementando: {"being built", "en construcción"}, slice.Verificado: {"tests passed", "pruebas pasadas"},
	slice.Bloqueado: {"stopped", "detenido"}, slice.Integrando: {"joining the product", "integrándose"}, slice.Cerrando: {"closing", "cerrando"},
	"CLOSED": {"closed", "cerrado"}, "CLOSED (rejected)": {"closed, rejected", "cerrado, rechazado"},
	"running": {"working", "trabajando"}, "asks_verification": {"asks for review", "pide revisión"}, "blocked": {"blocked", "bloqueado"},
	"question": {"has a question", "tiene una pregunta"}, "exited": {"finished", "terminó"},
	"ok": {"ok", "bien"}, "stuck": {"stuck", "atascado"}, "dead": {"dead", "muerto"}, "timeout": {"over time", "fuera de tiempo"}, "unknown": {"unknown", "desconocido"},
	"blocking": {"BLOCKING", "BLOQUEANTE"}, "soon": {"SOON", "PRONTO"}, "fyi": {"FYI", "AVISO"},
	"credential": {"credential", "credencial"}, "api_key": {"api key", "clave de API"}, "decision": {"decision", "decisión"},
	"confirmation": {"confirmation", "confirmación"}, "access": {"access", "acceso"},
}

func palabra(x string) string {
	if p, ok := palabras[x]; ok {
		return lang.T(p[0], p[1])
	}
	return x
}

// veredicto: el estado del worker dicho una sola vez. Uno que terminó o murió ya no «trabaja».
func veredicto(w Worker) string {
	if w.Phase == "exited" || w.Verdict == "dead" {
		return palabra(w.Verdict)
	}
	return palabra(w.Phase) + " · " + palabra(w.Verdict)
}

func unknown(p *int, suffix string) string {
	if p == nil {
		return desconocido()
	}
	return fmt.Sprintf("%d%s", *p, suffix)
}

func bar(p *int) string {
	if p == nil {
		return "[" + strings.Repeat("?", 20) + "]"
	}
	n := *p / 5
	return "[" + strings.Repeat("#", n) + strings.Repeat("-", 20-n) + "]"
}

func missing(m []string) string {
	if len(m) == 0 {
		return lang.T("nothing", "nada")
	}
	if len(m) <= 3 {
		return strings.Join(m, "; ")
	}
	return strings.Join(m[:3], "; ") + fmt.Sprintf(lang.T("; +%d more", "; +%d más"), len(m)-3)
}

func faltante(c Slice) string {
	if c.Status == "REJECTED" {
		return lang.T("not delivered", "no entregado")
	}
	return missing(c.Missing)
}

func motivoRechazo(c Slice) string {
	if c.RejectedReason == "" {
		return lang.T("no reason recorded", "sin motivo registrado")
	}
	return lang.Traducir(c.RejectedReason)
}

func who(w []string) string {
	if len(w) == 0 {
		return lang.T("nobody", "nadie")
	}
	return strings.Join(w, ", ")
}

// detalleTotal: el total dice qué cuenta, para que un 50% no parezca trabajo a medias.
func detalleTotal(o Overall) string {
	d := fmt.Sprintf(lang.T("%d of %d slices measured", "%d de %d slices medidos"), o.Measured, o.Total)
	if n := o.Total - o.Measured; n > 0 {
		d += fmt.Sprintf(lang.T(", %d unmeasured counted as 0%%", ", %d sin medir cuentan 0%%"), n)
	}
	if o.Rejected > 0 {
		d += fmt.Sprintf(lang.T(", %d rejected and counted as 0%%", ", %d rechazados cuentan 0%%"), o.Rejected)
	}
	if o.Blocked > 0 {
		d += fmt.Sprintf(lang.T(", %d blocked", ", %d bloqueados"), o.Blocked)
	}
	return d
}

// avisoFirmas: lo que el Owner debe saber de los registros decisivos sin firma válida.
func avisoFirmas(u Unverified) string {
	return fmt.Sprintf(lang.T("WARNING · records without a valid signature, not trusted: %d memory, %d stoppers, %d tasks, autonomy %v (edited by hand or from before signing; see docs/v2/specs/identity.md)",
		"AVISO · registros sin firma válida, no se confía en ellos: %d de memoria, %d stoppers, %d tareas, autonomía %v (editados a mano o anteriores a la firma; ver docs/v2/specs/identity.md)"),
		u.Memory, u.Stops, u.Tasks, u.Autonomy)
}

// Text es el tablero para la terminal; color solo si se pide (TTY y sin NO_COLOR).
func Text(b Board, color bool) string {
	var s strings.Builder
	paint := func(c, txt string) string {
		if !color {
			return txt
		}
		return ansi[c] + txt + "\x1b[0m"
	}
	T := lang.T
	if len(b.Stoppers) > 0 {
		s.WriteString(T("STOPPERS · only you can unblock these\n", "STOPPERS · solo tú puedes destrabarlos\n"))
		for _, st := range b.Stoppers {
			c := map[string]string{"blocking": "red", "soon": "amber", "fyi": "grey"}[st.Severity]
			tasks := ""
			if len(st.Tasks) > 0 {
				tasks = T(" · holds ", " · retiene ") + strings.Join(st.Tasks, ",")
			}
			fmt.Fprintf(&s, T("  %s %s %s %s: %s (%dm ago%s)\n", "  %s %s %s %s: %s (hace %dm%s)\n"), paint(c, "["+palabra(st.Severity)+"]"), st.ID, st.Slice, palabra(st.Type), st.Text, st.AgeMin, tasks)
		}
	} else {
		s.WriteString(T("STOPPERS · none, nothing waits on you\n", "STOPPERS · ninguno, nadie te espera\n"))
	}
	if u := b.Unverified; u.Any() {
		fmt.Fprintf(&s, "\n%s\n", paint("amber", avisoFirmas(u)))
	}
	fmt.Fprintf(&s, "\n%s %s %s (%s)\n", T("OVERALL", "TOTAL"), unknown(b.Overall.Pct, "%"), bar(b.Overall.Pct), detalleTotal(b.Overall))
	s.WriteString("\nSLICES\n")
	if len(b.Slices) == 0 {
		s.WriteString(T("  none yet\n", "  ninguno todavía\n"))
	}
	for _, c := range b.Slices {
		fmt.Fprintf(&s, "  %s %s · %s (%s)\n", paint(c.Color, "■ "+palabra(c.Status)), c.ID, c.Title, palabra(c.Phase))
		if c.Status == "REJECTED" {
			fmt.Fprintf(&s, T("     not delivered · %d of %d tasks were done before rejection\n", "     no entregado · %d de %d tareas hechas antes del rechazo\n"), c.Tasks.Done, c.Tasks.Total)
		} else {
			fmt.Fprintf(&s, T("     %s done %s · %d of %d tasks · missing: %s\n", "     %s hecho %s · %d de %d tareas · falta: %s\n"), unknown(c.Pct, "%"), bar(c.Pct), c.Tasks.Done, c.Tasks.Total, missing(c.Missing))
		}
		for _, f := range c.Fronts {
			fmt.Fprintf(&s, T("     front %s: %s · %d of %d tasks\n", "     frente %s: %s · %d de %d tareas\n"), f.ID, f.Goal, f.Tasks.Done, f.Tasks.Total)
		}
		fmt.Fprintf(&s, T("     who: %s · sent back %d times", "     quién: %s · devuelto %d veces"), who(c.Who), c.Rework)
		if c.BlockedReason != "" {
			fmt.Fprintf(&s, T(" · blocked: %s", " · bloqueado: %s"), lang.Traducir(c.BlockedReason))
		}
		if c.Status == "REJECTED" {
			fmt.Fprintf(&s, T(" · rejected: %s", " · rechazado: %s"), motivoRechazo(c))
		}
		s.WriteString("\n")
	}
	s.WriteString(T("\nWORKERS\n", "\nTRABAJADORES\n"))
	if len(b.Workers) == 0 {
		s.WriteString(T("  none running\n", "  ninguno trabajando\n"))
	}
	for _, w := range b.Workers {
		tests := T("tests unknown", "pruebas: desconocido")
		if w.Tests != nil {
			tests = fmt.Sprintf(T("tests %d/%d", "pruebas %d/%d"), w.Tests.Passing, w.Tests.Total)
		}
		hb := T("last signal unknown", "última señal desconocida")
		if w.HeartbeatAgeS != nil {
			hb = fmt.Sprintf(T("last signal %dm ago", "última señal hace %dm"), *w.HeartbeatAgeS/60)
		}
		budget := T("time limit: none", "tope de tiempo: ninguno")
		if w.BudgetMin != nil {
			budget = fmt.Sprintf(T("%s of %dm", "%s de %dm"), unknown(w.ElapsedMin, "m"), *w.BudgetMin)
		} else if w.ElapsedMin != nil {
			budget = fmt.Sprintf(T("%dm, time limit: none", "%dm, tope de tiempo: ninguno"), *w.ElapsedMin)
		}
		vc := map[string]string{"ok": "green", "exited": "grey"}[w.Verdict]
		if vc == "" {
			vc = "amber"
		}
		fmt.Fprintf(&s, T("  %s %s/%s %s · %s · commits %d · %s · %s · context %s\n", "  %s %s/%s %s · %s · commits %d · %s · %s · contexto %s\n"), w.ID, w.Slice, w.Task, paint(vc, veredicto(w)), hb, w.Commits, tests, budget, unknown(w.ContextPct, "%"))
	}
	fmt.Fprintf(&s, T("\nPACE %s (last %dh) · sent back %d times\n", "\nRITMO %s (últimas %dh) · devuelto %d veces\n"), ritmo(b), b.Velocity.WindowH, totalDevuelto(b))
	return s.String()
}

func ritmo(b Board) string {
	if b.Velocity.TasksPerHour == nil {
		return desconocido()
	}
	return fmt.Sprintf(lang.T("%.2f tasks/hour", "%.2f tareas/hora"), *b.Velocity.TasksPerHour)
}

func totalDevuelto(b Board) int {
	n := 0
	for _, c := range b.Slices {
		n += c.Rework
	}
	return n
}

// ---------- html ----------

// esc escapa HTML y parte la palabra "http" para que el archivo no tenga ninguna URL.
func esc(s string) string {
	return strings.ReplaceAll(html.EscapeString(s), "http", "&#104;ttp")
}

var cssColor = map[string]string{"red": "#c62828", "amber": "#b26a00", "green": "#2e7d32", "blue": "#1565c0", "grey": "#616161"}

// HTML es un solo archivo estático: CSS en línea, sin scripts ni pedidos de red.
func HTML(b Board) string {
	var s strings.Builder
	s.WriteString(`<!doctype html><html lang="` + lang.T("en", "es") + `"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>orq board</title><style>
body{font:15px/1.45 system-ui,sans-serif;margin:0;padding:16px;background:#fafafa;color:#212121}
h1{font-size:20px}h2{font-size:16px;margin-top:24px}
.card{background:#fff;border-left:8px solid #999;border-radius:6px;padding:10px 14px;margin:8px 0;box-shadow:0 1px 2px #0002}
.w{font-weight:700}.bar{background:#e0e0e0;height:10px;border-radius:5px;overflow:hidden;max-width:420px}.bar i{display:block;height:10px;background:#424242}
table{border-collapse:collapse;width:100%;font-size:13px}td,th{border-bottom:1px solid #ddd;padding:4px 6px;text-align:left}
@media (prefers-color-scheme:dark){body{background:#121212;color:#eee}.card{background:#1e1e1e}.bar{background:#333}.bar i{background:#ccc}td,th{border-color:#333}}
</style></head><body>`)
	fmt.Fprintf(&s, lang.T("<h1>How are we doing?</h1><p>Generated %s</p>", "<h1>¿Cómo vamos?</h1><p>Generado %s</p>"), esc(b.GeneratedAt.Format(time.RFC3339)))
	s.WriteString(lang.T("<h2>Stoppers: only you can unblock these</h2>", "<h2>Stoppers: solo tú puedes destrabarlos</h2>"))
	if len(b.Stoppers) == 0 {
		s.WriteString(lang.T("<p>None. Nothing waits on you.</p>", "<p>Ninguno. Nadie te espera.</p>"))
	}
	for _, st := range b.Stoppers {
		c := map[string]string{"blocking": "red", "soon": "amber", "fyi": "grey"}[st.Severity]
		fmt.Fprintf(&s, `<div class="card" style="border-color:%s"><span class="w" style="color:%s">%s</span> %s · %s · %s<br>%s<br><small>%d min</small></div>`,
			cssColor[c], cssColor[c], esc(palabra(st.Severity)), esc(st.ID), esc(st.Slice), esc(palabra(st.Type)), esc(st.Text), st.AgeMin)
	}
	pct := func(p *int) string {
		if p == nil {
			return "<p>" + desconocido() + "</p>"
		}
		return fmt.Sprintf(`<div class="bar"><i style="width:%d%%"></i></div>%d%%`, *p, *p)
	}
	if u := b.Unverified; u.Any() {
		fmt.Fprintf(&s, `<div class="card" style="border-color:%s">%s</div>`, cssColor["amber"], esc(avisoFirmas(u)))
	}
	fmt.Fprintf(&s, "<h2>%s</h2>%s <small>(%s)</small>", lang.T("Overall", "Total"), pct(b.Overall.Pct), esc(detalleTotal(b.Overall)))
	s.WriteString(lang.T("<h2>What we are building</h2>", "<h2>Qué estamos construyendo</h2>"))
	for _, c := range b.Slices {
		fmt.Fprintf(&s, `<div class="card" style="border-color:%s"><span class="w" style="color:%s">%s</span> %s · <b>%s</b> <small>(%s)</small>%s<br>%s: %s<br>%s: %s · %s %d`,
			cssColor[c.Color], cssColor[c.Color], esc(palabra(c.Status)), esc(c.ID), esc(c.Title), esc(palabra(c.Phase)), pct(c.Pct), lang.T("Missing", "Falta"), esc(faltante(c)), lang.T("Who", "Quién"), esc(who(c.Who)), lang.T("sent back, times:", "devuelto, veces:"), c.Rework)
		if c.BlockedReason != "" {
			fmt.Fprintf(&s, lang.T("<br>Blocked: %s", "<br>Bloqueado: %s"), esc(c.BlockedReason))
		}
		if c.Status == "REJECTED" {
			fmt.Fprintf(&s, lang.T("<br>Rejected: %s", "<br>Rechazado: %s"), esc(motivoRechazo(c)))
		}
		s.WriteString("</div>")
	}
	s.WriteString(lang.T("<h2>Workers</h2><table><tr><th>worker</th><th>task</th><th>state</th><th>last signal</th><th>commits</th><th>tests</th><th>time</th><th>context</th></tr>", "<h2>Trabajadores</h2><table><tr><th>trabajador</th><th>tarea</th><th>estado</th><th>última señal</th><th>commits</th><th>pruebas</th><th>tiempo</th><th>contexto</th></tr>"))
	for _, w := range b.Workers {
		tests := desconocido()
		if w.Tests != nil {
			tests = fmt.Sprintf("%d/%d", w.Tests.Passing, w.Tests.Total)
		}
		hb := desconocido()
		if w.HeartbeatAgeS != nil {
			hb = fmt.Sprintf(lang.T("%dm ago", "hace %dm"), *w.HeartbeatAgeS/60)
		}
		budget := lang.T("time limit: none", "tope de tiempo: ninguno")
		if w.BudgetMin != nil {
			budget = fmt.Sprintf(lang.T("%s of %dm", "%s de %dm"), unknown(w.ElapsedMin, "m"), *w.BudgetMin)
		}
		fmt.Fprintf(&s, "<tr><td>%s</td><td>%s/%s</td><td>%s</td><td>%s</td><td>%d</td><td>%s</td><td>%s</td><td>%s</td></tr>",
			esc(w.ID), esc(w.Slice), esc(w.Task), esc(veredicto(w)), hb, w.Commits, tests, budget, unknown(w.ContextPct, "%"))
	}
	s.WriteString("</table>")
	fmt.Fprintf(&s, lang.T("<h2>Pace</h2><p>%s (last %dh)</p></body></html>\n", "<h2>Ritmo</h2><p>%s (últimas %dh)</p></body></html>\n"), ritmo(b), b.Velocity.WindowH)
	return s.String()
}

// MarcarSesionesMuertas: lo que el runner sabe y el estado del worker no dice. Un worker que no ha
// terminado y cuya sesión ya no existe está muerto, y su slice en riesgo.
func MarcarSesionesMuertas(b *Board, muerta func(id string) bool) {
	for i := range b.Workers {
		w := &b.Workers[i]
		if w.Phase == "exited" || w.Phase == "asks_verification" || !muerta(w.ID) {
			continue
		}
		w.Verdict = "dead"
		for j := range b.Slices {
			if c := &b.Slices[j]; c.ID == w.Slice && c.Color == "blue" {
				c.Color, c.Status = "amber", statusOf["amber"]
			}
		}
	}
	slices.SortStableFunc(b.Slices, func(x, y Slice) int { return colorRank[x.Color] - colorRank[y.Color] })
}

// MarcarSesionesSinComprobar: fuera del runner no se puede preguntar si la sesión vive. Un worker con
// sesión registrada y sin otra señal en contra se lee «unknown», nunca «ok».
func MarcarSesionesSinComprobar(b *Board, registrada func(id string) bool) {
	for i := range b.Workers {
		if w := &b.Workers[i]; w.Verdict == "ok" && w.Phase != "asks_verification" && registrada(w.ID) {
			w.Verdict = "unknown"
		}
	}
}
