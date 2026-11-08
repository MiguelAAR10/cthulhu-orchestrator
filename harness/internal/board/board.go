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
				c.BlockedReason = "waiting on stop " + blocking
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
		if rejected {
			c.Status = "REJECTED"
		}
		b.Slices = append(b.Slices, c)
	}
	slices.SortStableFunc(b.Slices, func(x, y Slice) int { return colorRank[x.Color] - colorRank[y.Color] })

	b.Overall.Total, b.Overall.Measured = len(srcs), len(measuredPct)
	switch {
	case len(measuredPct) == 0:
	case sumW > 0 && !sinPeso:
		b.Overall.Pct = ip(int(math.Round(float64(sumWP) / float64(sumW))))
	default:
		t := 0
		for _, p := range measuredPct {
			t += p
		}
		b.Overall.Pct = ip(int(math.Round(float64(t) / float64(len(measuredPct)))))
	}
	return b, nil
}

func JSON(b Board) ([]byte, error) { return json.MarshalIndent(b, "", "  ") }

// ---------- texto ----------

var ansi = map[string]string{"red": "\x1b[31m", "amber": "\x1b[33m", "green": "\x1b[32m", "blue": "\x1b[34m", "grey": "\x1b[90m"}

func unknown(p *int, suffix string) string {
	if p == nil {
		return "unknown"
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
		return "nothing"
	}
	if len(m) <= 3 {
		return strings.Join(m, "; ")
	}
	return strings.Join(m[:3], "; ") + fmt.Sprintf("; +%d more", len(m)-3)
}

func faltante(c Slice) string {
	if c.Status == "REJECTED" {
		return "not delivered"
	}
	return missing(c.Missing)
}

func motivoRechazo(c Slice) string {
	if c.RejectedReason == "" {
		return "no reason recorded"
	}
	return lang.Traducir(c.RejectedReason)
}

func who(w []string) string {
	if len(w) == 0 {
		return "nobody"
	}
	return strings.Join(w, ", ")
}

// rechazados: el total lo dice, para que un 50% no parezca trabajo a medias.
func rechazados(o Overall) string {
	if o.Rejected == 0 {
		return ""
	}
	return fmt.Sprintf(", %d rejected and counted as 0%%", o.Rejected)
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
	if len(b.Stoppers) > 0 {
		s.WriteString("STOPPERS · only you can unblock these\n")
		for _, st := range b.Stoppers {
			c := map[string]string{"blocking": "red", "soon": "amber", "fyi": "grey"}[st.Severity]
			tasks := ""
			if len(st.Tasks) > 0 {
				tasks = " · holds " + strings.Join(st.Tasks, ",")
			}
			fmt.Fprintf(&s, "  %s %s %s %s: %s (%dm ago%s)\n", paint(c, "["+strings.ToUpper(st.Severity)+"]"), st.ID, st.Slice, st.Type, st.Text, st.AgeMin, tasks)
		}
	} else {
		s.WriteString("STOPPERS · none, nothing waits on you\n")
	}
	if u := b.Unverified; u.Any() {
		fmt.Fprintf(&s, "\n%s\n", paint("amber", avisoFirmas(u)))
	}
	fmt.Fprintf(&s, "\nOVERALL %s %s (%d of %d slices measured%s)\n", unknown(b.Overall.Pct, "%"), bar(b.Overall.Pct), b.Overall.Measured, b.Overall.Total, rechazados(b.Overall))
	s.WriteString("\nSLICES\n")
	if len(b.Slices) == 0 {
		s.WriteString("  none yet\n")
	}
	for _, c := range b.Slices {
		fmt.Fprintf(&s, "  %s %s · %s (%s)\n", paint(c.Color, "■ "+c.Status), c.ID, c.Title, lang.Traducir(c.Phase))
		if c.Status == "REJECTED" {
			fmt.Fprintf(&s, "     not delivered · %d of %d tasks were done before rejection\n", c.Tasks.Done, c.Tasks.Total)
		} else {
			fmt.Fprintf(&s, "     %s done %s · %d of %d tasks · missing: %s\n", unknown(c.Pct, "%"), bar(c.Pct), c.Tasks.Done, c.Tasks.Total, missing(c.Missing))
		}
		for _, f := range c.Fronts {
			fmt.Fprintf(&s, "     front %s: %s · %d of %d tasks\n", f.ID, f.Goal, f.Tasks.Done, f.Tasks.Total)
		}
		fmt.Fprintf(&s, "     who: %s · rework %d", who(c.Who), c.Rework)
		if c.BlockedReason != "" {
			fmt.Fprintf(&s, " · blocked: %s", lang.Traducir(c.BlockedReason))
		}
		if c.Status == "REJECTED" {
			fmt.Fprintf(&s, " · rejected: %s", motivoRechazo(c))
		}
		s.WriteString("\n")
	}
	s.WriteString("\nWORKERS\n")
	if len(b.Workers) == 0 {
		s.WriteString("  none running\n")
	}
	for _, w := range b.Workers {
		tests := "tests unknown"
		if w.Tests != nil {
			tests = fmt.Sprintf("tests %d/%d", w.Tests.Passing, w.Tests.Total)
		}
		hb := "beat unknown"
		if w.HeartbeatAgeS != nil {
			hb = fmt.Sprintf("beat %dm ago", *w.HeartbeatAgeS/60)
		}
		budget := "budget: none"
		if w.BudgetMin != nil {
			budget = fmt.Sprintf("%s of %dm", unknown(w.ElapsedMin, "m"), *w.BudgetMin)
		} else if w.ElapsedMin != nil {
			budget = fmt.Sprintf("%dm, budget: none", *w.ElapsedMin)
		}
		vc := map[string]string{"ok": "green", "exited": "grey"}[w.Verdict]
		if vc == "" {
			vc = "amber"
		}
		fmt.Fprintf(&s, "  %s %s/%s %s · %s · %s · commits %d · %s · %s · context %s\n", w.ID, w.Slice, w.Task, w.Phase, paint(vc, w.Verdict), hb, w.Commits, tests, budget, unknown(w.ContextPct, "%"))
	}
	vel := "unknown"
	if b.Velocity.TasksPerHour != nil {
		vel = fmt.Sprintf("%.2f tasks/hour", *b.Velocity.TasksPerHour)
	}
	rw := 0
	for _, c := range b.Slices {
		rw += c.Rework
	}
	fmt.Fprintf(&s, "\nVELOCITY %s (last %dh) · rework %d\n", vel, b.Velocity.WindowH, rw)
	return s.String()
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
	s.WriteString(`<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>orq board</title><style>
body{font:15px/1.45 system-ui,sans-serif;margin:0;padding:16px;background:#fafafa;color:#212121}
h1{font-size:20px}h2{font-size:16px;margin-top:24px}
.card{background:#fff;border-left:8px solid #999;border-radius:6px;padding:10px 14px;margin:8px 0;box-shadow:0 1px 2px #0002}
.w{font-weight:700}.bar{background:#e0e0e0;height:10px;border-radius:5px;overflow:hidden;max-width:420px}.bar i{display:block;height:10px;background:#424242}
table{border-collapse:collapse;width:100%;font-size:13px}td,th{border-bottom:1px solid #ddd;padding:4px 6px;text-align:left}
@media (prefers-color-scheme:dark){body{background:#121212;color:#eee}.card{background:#1e1e1e}.bar{background:#333}.bar i{background:#ccc}td,th{border-color:#333}}
</style></head><body>`)
	fmt.Fprintf(&s, "<h1>How are we doing?</h1><p>Generated %s</p>", esc(b.GeneratedAt.Format(time.RFC3339)))
	s.WriteString("<h2>Stoppers: only you can unblock these</h2>")
	if len(b.Stoppers) == 0 {
		s.WriteString("<p>None. Nothing waits on you.</p>")
	}
	for _, st := range b.Stoppers {
		c := map[string]string{"blocking": "red", "soon": "amber", "fyi": "grey"}[st.Severity]
		fmt.Fprintf(&s, `<div class="card" style="border-color:%s"><span class="w" style="color:%s">%s</span> %s · %s · %s<br>%s<br><small>%d min ago</small></div>`,
			cssColor[c], cssColor[c], esc(strings.ToUpper(st.Severity)), esc(st.ID), esc(st.Slice), esc(st.Type), esc(st.Text), st.AgeMin)
	}
	pct := func(p *int) string {
		if p == nil {
			return `<p>unknown</p>`
		}
		return fmt.Sprintf(`<div class="bar"><i style="width:%d%%"></i></div>%d%%`, *p, *p)
	}
	if u := b.Unverified; u.Any() {
		fmt.Fprintf(&s, `<div class="card" style="border-color:%s">%s</div>`, cssColor["amber"], esc(avisoFirmas(u)))
	}
	fmt.Fprintf(&s, "<h2>Overall</h2>%s <small>(%d of %d slices measured%s)</small>", pct(b.Overall.Pct), b.Overall.Measured, b.Overall.Total, rechazados(b.Overall))
	s.WriteString("<h2>What we are building</h2>")
	for _, c := range b.Slices {
		fmt.Fprintf(&s, `<div class="card" style="border-color:%s"><span class="w" style="color:%s">%s</span> %s · <b>%s</b> <small>(%s)</small>%s<br>Missing: %s<br>Who: %s · rework %d`,
			cssColor[c.Color], cssColor[c.Color], esc(c.Status), esc(c.ID), esc(c.Title), esc(lang.Traducir(c.Phase)), pct(c.Pct), esc(faltante(c)), esc(who(c.Who)), c.Rework)
		if c.BlockedReason != "" {
			fmt.Fprintf(&s, "<br>Blocked: %s", esc(c.BlockedReason))
		}
		if c.Status == "REJECTED" {
			fmt.Fprintf(&s, "<br>Rejected: %s", esc(motivoRechazo(c)))
		}
		s.WriteString("</div>")
	}
	s.WriteString("<h2>Workers</h2><table><tr><th>worker</th><th>task</th><th>state</th><th>last beat</th><th>commits</th><th>tests</th><th>time</th><th>context</th></tr>")
	for _, w := range b.Workers {
		tests := "unknown"
		if w.Tests != nil {
			tests = fmt.Sprintf("%d/%d", w.Tests.Passing, w.Tests.Total)
		}
		hb := "unknown"
		if w.HeartbeatAgeS != nil {
			hb = fmt.Sprintf("%dm ago", *w.HeartbeatAgeS/60)
		}
		budget := "budget: none"
		if w.BudgetMin != nil {
			budget = fmt.Sprintf("%s of %dm", unknown(w.ElapsedMin, "m"), *w.BudgetMin)
		}
		fmt.Fprintf(&s, "<tr><td>%s</td><td>%s/%s</td><td>%s · %s</td><td>%s</td><td>%d</td><td>%s</td><td>%s</td><td>%s</td></tr>",
			esc(w.ID), esc(w.Slice), esc(w.Task), esc(w.Phase), esc(w.Verdict), hb, w.Commits, tests, budget, unknown(w.ContextPct, "%"))
	}
	s.WriteString("</table>")
	vel := "unknown"
	if b.Velocity.TasksPerHour != nil {
		vel = fmt.Sprintf("%.2f tasks/hour", *b.Velocity.TasksPerHour)
	}
	fmt.Fprintf(&s, "<h2>Velocity</h2><p>%s (last %dh)</p></body></html>\n", vel, b.Velocity.WindowH)
	return s.String()
}
