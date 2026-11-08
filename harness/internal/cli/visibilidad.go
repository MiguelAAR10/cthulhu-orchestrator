package cli

// Slice 5 · visibilidad: estado de workers, stoppers y autonomía, tablero, handoff y sync de memoria.

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"orq/internal/identidad"
	"orq/internal/lang"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"orq/internal/board"
	"orq/internal/canal"
	"orq/internal/config"
	"orq/internal/gate"
	"orq/internal/memoria"
	"orq/internal/memsync"
	"orq/internal/plan"
	"orq/internal/stops"
	"orq/internal/watch"
	"orq/internal/workers"
)

// rolStop: con ORQ_WORKER_ID es un worker; dentro de un worktree enlazado también lo es, aunque
// falte el entorno (nunca owner ni atena desde ahí); sin rol registrado, quien teclea es el Owner.
func (p proyecto) rolStop() string {
	if id, ok := workerEfectivo(); ok {
		if strings.HasPrefix(id, "worker") {
			return id
		}
		return "worker-" + id
	}
	if wd, _ := os.Getwd(); gate.IsLinkedWorktree(wd) || gate.IsLinkedWorktree(p.root) {
		return "worker-worktree"
	}
	if r := p.rolActual(); r != "" {
		return r
	}
	return "owner"
}

// rolPrivilegiado: el rol para acciones que cambian el contrato, la autonomía o resuelven stoppers.
// Un worker sigue siendo worker; fuera de eso hace falta un rol explícito (ORQ_ROL o el panel
// registrado). Sin rol devuelve "": el cwd del checkout principal nunca equivale a Owner.
// Además, el rol reclamado solo vale con el token del Owner (ORQ_TOKEN): ORQ_ROL sale del entorno y
// cualquiera lo escribe; el token vive fuera del repo y orq nunca se lo pasa a un worker.
func (p proyecto) rolPrivilegiado() string {
	if r := p.rolStop(); strings.HasPrefix(r, "worker") {
		return r
	}
	r := p.rolActual()
	if r != "" {
		if err := identidad.Autorizado(); err != nil {
			return r + " without a valid ORQ_TOKEN (" + err.Error() + ")"
		}
	}
	return r
}

// exigirToken: acciones del Orquestador o del Owner (cerrar un slice) piden el token, nunca un worker.
func (p proyecto) exigirToken(accion string) error {
	if r := p.rolStop(); strings.HasPrefix(r, "worker") {
		return fmt.Errorf("a worker cannot %s (role %q): ask the Orchestrator", accion, r)
	}
	if err := identidad.Autorizado(); err != nil {
		return fmt.Errorf("%s requires the Owner's token from a trusted session (%v): export ORQ_TOKEN=\"$(orq token)\" in the Owner's or Orchestrator's terminal", accion, err)
	}
	return nil
}

func nnDe(sliceID string) string { return strings.TrimPrefix(sliceID, "S-") }

func listaCSV(s string) []string {
	var out []string
	for _, x := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == '|' }) {
		if x = strings.TrimSpace(x); x != "" {
			out = append(out, x)
		}
	}
	return out
}

// ---------- orq worker ----------

func runWorker(args []string, stdout, stderr io.Writer) int {
	p, err := abrirProyecto()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	uso := "uso: orq worker beat [--phase running|blocked|question] [--summary \"...\"] [--context-pct N] [--stop ST-NNNN]\n" +
		"     orq worker done --summary \"...\" [--ask-verification]\n     orq worker list [--json]"
	if len(args) == 0 {
		fmt.Fprintln(stderr, uso)
		return 1
	}
	now := time.Now()
	id := os.Getenv("ORQ_WORKER_ID")
	switch args[0] {
	case "list":
		ls, _ := workers.List(p.dir)
		cfg, _ := config.Load(p.dir)
		host, _ := os.Hostname()
		if slices.Contains(args[1:], "--json") {
			type row struct {
				workers.State
				Verdict string `json:"verdict"`
			}
			rows := []row{}
			for _, st := range ls {
				rows = append(rows, row{st, watch.Current(p.dir, st, now, cfg.StaleMin(), host)})
			}
			raw, _ := json.MarshalIndent(rows, "", "  ")
			fmt.Fprintln(stdout, string(raw))
			return 0
		}
		if len(ls) == 0 {
			fmt.Fprintln(stdout, "no workers")
		}
		for _, st := range ls {
			v := watch.Current(p.dir, st, now, cfg.StaleMin(), host)
			fmt.Fprintf(stdout, "%-14s %s/%s %-17s %-7s beat %dm ago · commits %d · %s\n", st.ID, st.Slice, st.Task, st.Phase, v,
				int(now.Sub(st.Heartbeat).Minutes()), st.Commits, st.Summary)
		}
		return 0
	case "beat":
		fs := nuevoFS("worker beat", flag.ContinueOnError)
		fs.SetOutput(stderr)
		phase := fs.String("phase", "", "running|blocked|question")
		summary := fs.String("summary", "", "one plain line")
		ctx := fs.Int("context-pct", -1, "context used, 0..100")
		stop := fs.String("stop", "", "open stopper id, required with --phase blocked")
		if fs.Parse(args[1:]) != nil {
			return 1
		}
		st0, err := workers.Load(p.dir, id)
		if err != nil {
			fmt.Fprintln(stderr, "orq worker:", err)
			return 1
		}
		o := workers.BeatOpts{Phase: *phase, Summary: *summary, Stop: *stop}
		if *ctx >= 0 {
			o.ContextPct = ctx
		}
		if *phase == "blocked" {
			if *stop == "" { // el último stopper abierto que levantó este worker
				all, _ := stops.Load(stops.Path(p.dir))
				for _, s := range stops.Open(all) {
					if s.By == "worker-"+id || s.By == id {
						o.Stop = s.ID
					}
				}
			}
			all, _ := stops.Load(stops.Path(p.dir))
			if s := stops.Find(all, o.Stop); o.Stop != "" && (s == nil || s.State != "open") {
				fmt.Fprintf(stderr, "orq worker: %s is not an open stopper\n", o.Stop)
				return 1
			}
		}
		if e, err := p.slices().Load(nnDe(st0.Slice)); err == nil && e != nil && e.Recibo != nil && e.Recibo.Contado {
			o.Tests = &workers.Tests{Passing: e.Recibo.Ejecutados - e.Recibo.Fallidos, Total: e.Recibo.Ejecutados}
		}
		st, err := workers.Beat(p.dir, id, o, now)
		if err != nil {
			fmt.Fprintln(stderr, "orq worker:", err)
			return 1
		}
		if *phase == "question" {
			m := canal.Msg{TS: now, De: "worker-" + id, Para: "orquestador", Tipo: "pregunta", Texto: fmt.Sprintf("%s %s/%s: %s", id, st.Slice, st.Task, def(st.Summary, "has a question"))}
			if err := canal.Append(p.canalPath(), &m); err != nil && !errors.Is(err, canal.ErrDuplicado) {
				fmt.Fprintln(stderr, "orq worker: question not posted:", err)
			}
		}
		fmt.Fprintf(stdout, "%s %s · head %.8s · commits %d\n", st.ID, st.Phase, st.Head, st.Commits)
		return 0
	case "done":
		fs := nuevoFS("worker done", flag.ContinueOnError)
		fs.SetOutput(stderr)
		summary := fs.String("summary", "", "what is done, one plain line")
		ask := fs.Bool("ask-verification", false, "ask the Orchestrator to run orq slice verificar")
		if fs.Parse(args[1:]) != nil || *summary == "" {
			fmt.Fprintln(stderr, uso)
			return 1
		}
		st, err := workers.Done(p.dir, id, *summary, *ask, now)
		if err != nil {
			fmt.Fprintln(stderr, "orq worker:", err)
			return 1
		}
		fmt.Fprintf(stdout, "%s %s · commits %d\n", st.ID, st.Phase, st.Commits)
		if st.Task != "" {
			path := plan.Path(p.dir, nnDe(st.Slice))
			if _, err := plan.Actualizar(path, func(pl *plan.Plan) error { _, err := plan.Hecha(pl, st.Task); return err }); err != nil {
				fmt.Fprintf(stdout, "task %s not marked done: %v\n", st.Task, err)
				return 0
			}
			workers.AppendTaskEvent(p.dir, st.Slice, st.Task, "done", st.ID, now)
			fmt.Fprintf(stdout, "task %s marked done\n", st.Task)
		}
		return 0
	}
	fmt.Fprintln(stderr, uso)
	return 1
}

// ---------- orq stop ----------

func runStop(args []string, stdout, stderr io.Writer) int {
	p, err := abrirProyecto()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	uso := `uso: orq stop raise --type credential|api_key|decision|confirmation|access --severity blocking|soon|fyi --slice S-NN [--tasks T1,T2] --text "..." [--options "a|b"] [--where ENV_NAME]
     orq stop list [--open] [--slice S-NN] [--json]
     orq stop resolve ST-NNNN [--provided-at env:NAME|file:PATH|vault:REF] [--choice a] [--note "..."]
     orq stop cancel ST-NNNN --note "..."`
	if len(args) == 0 {
		fmt.Fprintln(stderr, uso)
		return 1
	}
	path := stops.Path(p.dir)
	now := time.Now()
	rol := p.rolStop()
	secreto := func(err error) int {
		fmt.Fprintln(stderr, "orq stop:", err)
		if errors.Is(err, stops.ErrSecret) {
			return 2
		}
		return 1
	}
	switch args[0] {
	case "raise":
		fs := nuevoFS("stop raise", flag.ContinueOnError)
		fs.SetOutput(stderr)
		typ := fs.String("type", "", strings.Join(stops.Types, "|"))
		sev := fs.String("severity", "", "blocking|soon|fyi")
		sl := fs.String("slice", "", "S-NN")
		tasks := fs.String("tasks", "", "T1,T2")
		text := fs.String("text", "", "one plain sentence for the Owner")
		opts := fs.String("options", "", "a|b")
		where := fs.String("where", "", "ENV_NAME where the Owner should put it")
		if fs.Parse(args[1:]) != nil {
			return 1
		}
		if !strings.HasPrefix(*sl, "S-") && *sl != "" {
			*sl = "S-" + *sl
		}
		r := stops.Raise{Type: *typ, Severity: *sev, Slice: *sl, Tasks: listaCSV(*tasks), By: rol, Text: *text, Options: listaCSV(*opts), Where: *where}
		if len(r.Tasks) > 0 {
			pl, err := plan.Leer(plan.Path(p.dir, nnDe(r.Slice)))
			if err != nil {
				fmt.Fprintf(stderr, "orq stop: %s has no plan; --tasks must name plan tasks\n", r.Slice)
				return 1
			}
			for _, t := range r.Tasks {
				if !slices.ContainsFunc(pl.Tareas, func(x plan.Tarea) bool { return x.ID == t }) {
					fmt.Fprintf(stderr, "orq stop: task %s is not in the plan of %s\n", t, r.Slice)
					return 1
				}
			}
		}
		s, dup, err := stops.Add(path, r, now)
		if err != nil {
			return secreto(err)
		}
		if dup {
			fmt.Fprintf(stdout, "%s already open (same slice, type and text)\n", s.ID)
			return 0
		}
		hold := "holds nothing; work continues"
		if s.Severity == "blocking" && len(s.Tasks) > 0 {
			hold = "holds " + strings.Join(s.Tasks, ", ") + "; the rest continues"
		}
		fmt.Fprintf(stdout, "%s raised (%s %s on %s): %s\n", s.ID, s.Severity, s.Type, s.Slice, hold)
		return 0
	case "list":
		all, err := stops.Load(path)
		if err != nil {
			fmt.Fprintln(stderr, "orq stop:", err)
			return 1
		}
		onlyOpen, asJSON, sl := false, false, ""
		for i := 1; i < len(args); i++ {
			switch args[i] {
			case "--open":
				onlyOpen = true
			case "--json":
				asJSON = true
			case "--slice":
				if i+1 < len(args) {
					sl = "S-" + nnDe(args[i+1])
					i++
				}
			}
		}
		out := []stops.Stop{}
		for _, s := range all {
			if (!onlyOpen || s.State == "open") && (sl == "" || s.Slice == sl) {
				out = append(out, s)
			}
		}
		if asJSON {
			raw, _ := json.MarshalIndent(out, "", "  ")
			fmt.Fprintln(stdout, string(raw))
			return 0
		}
		if len(out) == 0 {
			fmt.Fprintln(stdout, lang.T("no stoppers", "sin stoppers"))
		}
		for _, s := range out {
			fmt.Fprintf(stdout, "%s %-9s %-10s %s %-12s %s", s.ID, verStop(s.State), verStop(s.Severity), s.Slice, verStop(s.Type), s.Text)
			if len(s.Tasks) > 0 {
				fmt.Fprintf(stdout, lang.T(" [tasks %s]", " [tareas %s]"), strings.Join(s.Tasks, ","))
			}
			if s.Where != "" {
				fmt.Fprintf(stdout, lang.T(" [put it in env %s]", " [ponlo en la variable %s]"), s.Where)
			}
			if len(s.Options) > 0 {
				fmt.Fprintf(stdout, lang.T(" [options %s]", " [opciones %s]"), strings.Join(s.Options, "|"))
			}
			if s.ClosureIgnored {
				fmt.Fprint(stdout, lang.T(" [UNVERIFIED: a closure without a valid signature was ignored]", " [SIN VERIFICAR: se ignoró un cierre sin firma válida]"))
			}
			if s.RaisedUnverified {
				fmt.Fprint(stdout, lang.T(" [UNVERIFIED: this record has no valid signature; treated as blocking]", " [SIN VERIFICAR: este registro no tiene firma válida; se trata como bloqueante]"))
			}
			fmt.Fprintln(stdout)
		}
		return 0
	case "resolve":
		if len(args) < 2 {
			fmt.Fprintln(stderr, uso)
			return 1
		}
		fs := nuevoFS("stop resolve", flag.ContinueOnError)
		fs.SetOutput(stderr)
		at := fs.String("provided-at", "", "env:NAME | file:PATH | vault:REF")
		choice := fs.String("choice", "", "chosen option of a decision")
		note := fs.String("note", "", "short note")
		if fs.Parse(args[2:]) != nil {
			return 1
		}
		all, _ := stops.Load(path)
		s := stops.Find(all, args[1])
		if s == nil {
			fmt.Fprintf(stderr, "orq stop: no stopper %s\n", args[1])
			return 1
		}
		cfg, _ := config.Load(p.dir)
		rol := p.rolPrivilegiado()
		if err := stops.CanResolve(rol, *s, cfg, *note); err != nil {
			fmt.Fprintln(stderr, "orq stop:", err)
			return 1
		}
		if *at != "" {
			ignored := func(abs string) bool {
				return exec.Command("git", "-C", p.root, "check-ignore", "-q", abs).Run() == nil
			}
			if err := stops.CheckProvidedAt(*at, p.root, ignored); err != nil {
				return secreto(err)
			}
		}
		got, err := stops.Resolve(path, s.ID, stops.Resolution{By: rol, ProvidedAt: *at, Choice: *choice, Note: *note}, now)
		if err != nil {
			return secreto(err)
		}
		fmt.Fprintf(stdout, "%s resolved", got.ID)
		if got.Provided == "yes" {
			fmt.Fprintf(stdout, " · provided at %s", got.ProvidedAt)
			if name, ok := strings.CutPrefix(got.ProvidedAt, "env:"); ok {
				if _, set := os.LookupEnv(name); set {
					fmt.Fprint(stdout, " (set here)")
				} else {
					fmt.Fprint(stdout, " (not set here)")
				}
			}
		}
		fmt.Fprintln(stdout)
		if got.Type == "decision" || strings.HasPrefix(*note, "auto: ") {
			fuente := "owner"
			if rol == "orquestador" || rol == "planificador" {
				fuente = rol
			}
			txt := fmt.Sprintf("%s: %s → %s", got.ID, got.Text, def(got.Choice, *note))
			if r := []rune(txt); len(r) > memoria.MaxTexto {
				txt = string(r[:memoria.MaxTexto-1]) + "…"
			}
			it := memoria.Item{Imp: "alta", Cat: "decision", Tipo: "decision", Fuente: fuente, Texto: txt, Slice: got.Slice, Evidencia: got.ID, Por: rol}
			if _, _, err := memoria.Add(p.memPath(), it, now); err != nil {
				fmt.Fprintln(stderr, "orq stop: decision not saved to memory:", err)
			}
		}
		return 0
	case "cancel":
		if len(args) < 2 {
			fmt.Fprintln(stderr, uso)
			return 1
		}
		fs := nuevoFS("stop cancel", flag.ContinueOnError)
		fs.SetOutput(stderr)
		note := fs.String("note", "", "why it is no longer needed")
		if fs.Parse(args[2:]) != nil {
			return 1
		}
		s, err := stops.Cancel(path, args[1], p.rolPrivilegiado(), *note, now)
		if err != nil {
			return secreto(err)
		}
		fmt.Fprintf(stdout, "%s cancelled\n", s.ID)
		return 0
	}
	fmt.Fprintln(stderr, uso)
	return 1
}

// verStop: estado, severidad y tipo de un stopper en español con ORQ_LANG=es; el dato no cambia.
func verStop(x string) string {
	if !lang.ES() {
		return x
	}
	es := map[string]string{"open": "abierto", "resolved": "resuelto", "cancelled": "cancelado", "blocking": "bloqueante", "soon": "pronto", "fyi": "aviso",
		"credential": "credencial", "api_key": "clave-api", "decision": "decisión", "confirmation": "confirmación", "access": "acceso"}
	if e, ok := es[x]; ok {
		return e
	}
	return x
}

// ---------- orq autonomy ----------

func runAutonomy(args []string, stdout, stderr io.Writer) int {
	p, err := abrirProyecto()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	cfg, _ := config.Load(p.dir)
	always := []string{
		"any blocking stopper, and any credential, api_key or access stopper",
		"product acceptance: the demo of a VERIFICADO slice",
		"anything irreversible: shared/production data deletion or migration, push to a remote, publishing, spending money, messaging real users",
		"any change to the acceptance contract or exam (command, bundle, scope)",
		"anything outside scope or over budget (a task over minutes_per_task raises confirmation/soon)",
	}
	if len(args) == 0 || args[0] == "ver" || args[0] == "show" {
		fmt.Fprintf(stdout, "autonomy: %s\n", cfg.Mode())
		if cfg.AutonomiaSinFirma {
			fmt.Fprintln(stdout, lang.T("UNVERIFIED: config.json says free without a valid signature; ignored. The Owner re-approves with `orq autonomy free --scope ...`",
				"SIN VERIFICAR: config.json dice free sin firma válida; se ignora. El Owner la vuelve a aprobar con `orq autonomy free --scope ...`"))
		}
		if a := cfg.Autonomy; a != nil && cfg.Mode() == "free" {
			fmt.Fprintf(stdout, "scope: %s\nbudget: %d min/task, %d workers, %d attempts\napproved by %s at %s\n",
				strings.Join(a.Scope, ", "), a.Budget.MinutesPerTask, a.Budget.MaxWorkers, a.Budget.MaxAttempts, a.ApprovedBy, a.ApprovedAt.Format(time.RFC3339))
		}
		fmt.Fprintln(stdout, "always reaches the Owner, in any mode:")
		for i, r := range always {
			fmt.Fprintf(stdout, "  %d. %s\n", i+1, r)
		}
		return 0
	}
	if r := p.rolPrivilegiado(); r != "owner" && r != "atena" {
		fmt.Fprintf(stderr, "orq autonomy: only the Owner (through Atena) changes autonomy; role %q cannot (set ORQ_ROL=owner|atena)\n", r)
		return 1
	}
	switch args[0] {
	case "guided":
		cfg.Autonomy = &config.Autonomy{Mode: "guided", ApprovedBy: "owner", ApprovedAt: time.Now().UTC()}
	case "free":
		fs := nuevoFS("autonomy free", flag.ContinueOnError)
		fs.SetOutput(stderr)
		scope := fs.String("scope", "", "paths the Owner pre-approves, comma separated")
		mins := fs.Int("minutes", 45, "minutes per task")
		wk := fs.Int("workers", 4, "max workers")
		att := fs.Int("attempts", 3, "max attempts")
		if fs.Parse(args[1:]) != nil || *scope == "" {
			fmt.Fprintln(stderr, "uso: orq autonomy free --scope \"path/**,docs/**\" [--minutes 45] [--workers 4] [--attempts 3]")
			return 1
		}
		cfg.Autonomy = &config.Autonomy{Mode: "free", Scope: listaCSV(*scope), Budget: config.Budget{MinutesPerTask: *mins, MaxWorkers: *wk, MaxAttempts: *att},
			ApprovedBy: "owner", ApprovedAt: time.Now().UTC()}
	default:
		fmt.Fprintln(stderr, "uso: orq autonomy [ver] | guided | free --scope ...")
		return 1
	}
	cfg.FirmarAutonomia(p.dir)
	if err := config.Save(p.dir, cfg); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "autonomy set to %s (approved by owner)\n", cfg.Mode())
	return 0
}

// ---------- orq board ----------

func esTTY(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

func runBoard(args []string, stdout, stderr io.Writer) int {
	p, err := abrirProyecto()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fs := nuevoFS("board", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "schema orq.board/v1")
	asHTML := fs.Bool("html", false, "one static file, no scripts or network")
	out := fs.String("out", "", "HTML output path (default .orq/board.html)")
	if fs.Parse(args) != nil {
		return 1
	}
	b, err := board.Build(p.dir, time.Now())
	if err != nil {
		fmt.Fprintln(stderr, "orq board:", err)
		return 1
	}
	if ses, err := canal.LoadSesiones(p.sesPath()); err == nil {
		if r := rn(); r.Inside() {
			board.MarcarSesionesMuertas(&b, func(id string) bool {
				s, ok := ses[id]
				return ok && !r.Alive(s.Agente)
			})
		} else {
			board.MarcarSesionesSinComprobar(&b, func(id string) bool { _, ok := ses[id]; return ok })
		}
	}
	switch {
	case *asJSON:
		raw, _ := board.JSON(b)
		fmt.Fprintln(stdout, string(raw))
	case *asHTML:
		path := *out
		if path == "" {
			path = filepath.Join(p.dir, "board.html")
		}
		if err := os.WriteFile(path, []byte(board.HTML(b)), 0o644); err != nil {
			fmt.Fprintln(stderr, "orq board:", err)
			return 1
		}
		fmt.Fprintf(stdout, "board written to %s (open it in any browser, works offline)\n", path)
	default:
		fmt.Fprint(stdout, board.Text(b, esTTY(stdout) && os.Getenv("NO_COLOR") == ""))
	}
	return 0
}

// ---------- orq handoff ----------

func runHandoff(args []string, stdout, stderr io.Writer) int {
	p, err := abrirProyecto()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fs := nuevoFS("handoff", flag.ContinueOnError)
	fs.SetOutput(stderr)
	out := fs.String("out", filepath.Join(p.root, "HANDOFF.md"), "output path")
	if fs.Parse(args) != nil {
		return 1
	}
	now := time.Now()
	b, err := board.Build(p.dir, now)
	if err != nil {
		fmt.Fprintln(stderr, "orq handoff:", err)
		return 1
	}
	cfg, _ := config.Load(p.dir)
	in := board.HandoffInput{Now: now, Root: p.root, Board: b, SyncMode: cfg.SyncMode(), SyncCmds: memsync.Commands(cfg.SyncMode())}
	items, _ := memoria.Load(p.memPath())
	var decs []string
	for _, it := range items {
		marca := ""
		if it.SinVerificar {
			marca = " [" + memoria.Marca() + "]"
		}
		if it.Cat == "requisito" && in.Goal == "" {
			in.Goal = it.Texto + marca
		}
		if it.TipoEfectivo() == "decision" {
			decs = append(decs, fmt.Sprintf("%s %s (%s): %s%s", it.TS.Format("2006-01-02"), it.ID, it.Fuente, it.Texto, marca))
		}
	}
	if len(decs) > 5 {
		decs = decs[len(decs)-5:]
	}
	in.Decisions = decs
	if ab, err := p.slices().Abiertos(); err == nil {
		for _, e := range ab {
			ready := "no plan yet"
			if pl, err := plan.Leer(plan.Path(p.dir, e.NN)); err == nil {
				ls, _ := plan.Listas(pl)
				ready = "next ready tasks: " + ids(ls)
			}
			in.Active = append(in.Active, fmt.Sprintf("Active slice %s · %s (%s) · %s", e.ID(), e.Titulo, e.Fase, ready))
		}
	}
	if err := os.WriteFile(*out, []byte(board.Handoff(in)), 0o644); err != nil {
		fmt.Fprintln(stderr, "orq handoff:", err)
		return 1
	}
	fmt.Fprintf(stdout, "handoff written to %s\n", *out)
	return 0
}

// ---------- orq sync ----------

func runSync(args []string, stdout, stderr io.Writer) int {
	p, err := abrirProyecto()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	cfg, _ := config.Load(p.dir)
	if cfg.SyncMode() != "branch" {
		fmt.Fprintf(stderr, "orq sync: memory sync is %q; push/pull only apply to sync=branch (orq init --sync=branch)\n", cfg.SyncMode())
		return 1
	}
	var msg string
	switch {
	case len(args) > 0 && args[0] == "push":
		msg, err = memsync.Push(p.root, time.Now())
	case len(args) > 0 && args[0] == "pull":
		msg, err = memsync.Pull(p.root, slices.Contains(args[1:], "--theirs"))
	default:
		fmt.Fprintln(stderr, "uso: orq sync push | pull [--theirs]")
		return 1
	}
	if msg != "" {
		fmt.Fprintln(stdout, msg)
	}
	if err != nil {
		fmt.Fprintln(stderr, "orq sync:", err)
		return 1
	}
	return 0
}

// initSync aplica --sync=MODO / --memoria=MODO de orq init. Sin flag conserva el modo guardado.
func (p proyecto) initSync(args []string, stdout io.Writer) error {
	mode := ""
	for i, a := range args {
		for _, pre := range []string{"--sync=", "--memoria="} {
			if v, ok := strings.CutPrefix(a, pre); ok {
				mode = v
			}
		}
		if (a == "--sync" || a == "--memoria") && i+1 < len(args) {
			mode = args[i+1]
		}
	}
	cfg, _ := config.Load(p.dir)
	if mode == "" {
		mode = cfg.SyncMode()
	}
	if !slices.Contains(config.SyncModes, mode) {
		return fmt.Errorf("--sync %q: use local|commit|branch", mode)
	}
	if err := memsync.SetMode(p.root, mode); err != nil {
		return err
	}
	cfg.Sync = mode
	if err := config.Save(p.dir, cfg); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "memory sync: %s · %s\n", mode, strings.Join(memsync.Commands(mode), " "))
	return nil
}

// ---------- vigía de workers en orq watch ----------

func (p proyecto) vigilarWorkers(stdout io.Writer) {
	cfg, _ := config.Load(p.dir)
	host, _ := os.Hostname()
	env := watch.WorkerEnv{
		Now: time.Now(), StaleMin: cfg.StaleMin(), Host: host, Alive: watch.PIDAlive,
		Notify: func(w, v, reason string, seq int) error {
			m := canal.Msg{TS: time.Now(), De: "vigia", Para: "orquestador", Tipo: "estado",
				Texto: fmt.Sprintf("worker %s: %s (%s). The watcher never kills or removes anything; act with orq worker list.", w, v, def(reason, "back to normal")),
				Clave: "watch:" + w + "+" + v + "#" + strconv.Itoa(seq)}
			err := canal.Append(p.canalPath(), &m)
			if errors.Is(err, canal.ErrDuplicado) {
				return nil
			}
			return err
		},
		Raise: func(st workers.State, reason string) error {
			var tasks []string
			if st.Task != "" {
				tasks = []string{st.Task}
			}
			_, _, err := stops.Add(stops.Path(p.dir), stops.Raise{Type: "confirmation", Severity: "soon", Slice: st.Slice, Tasks: tasks, By: "vigia",
				Text: fmt.Sprintf("Worker %s is over 1.5 times its time budget on %s %s. Keep it going or stop it?", st.ID, st.Slice, st.Task)}, time.Now())
			return err
		},
	}
	vs, err := watch.Workers(p.dir, env)
	if err != nil {
		fmt.Fprintf(stdout, "workers: %v\n", err)
		return
	}
	for _, v := range vs {
		if v.Changed || v.Verdict != "ok" {
			fmt.Fprintf(stdout, "worker %s: %s %s\n", v.Worker, v.Verdict, v.Reason)
		}
	}
}
