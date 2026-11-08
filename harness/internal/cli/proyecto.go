package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"orq/internal/plan"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"orq/internal/canal"
	"orq/internal/escriba"
	"orq/internal/gate"
	"orq/internal/herdr"
	"orq/internal/limits"
	"orq/internal/memoria"
	"orq/internal/orqdir"
	"orq/internal/slice"
	"orq/internal/state"
)

type proyecto struct {
	root, dir string
}

func abrirProyecto() (proyecto, error) {
	root, err := orqdir.Root()
	if err != nil {
		return proyecto{}, errors.New("orq: no estás dentro de un repositorio git")
	}
	dir, err := orqdir.Dir()
	return proyecto{root, dir}, err
}

func (p proyecto) memPath() string        { return filepath.Join(p.dir, "memoria.jsonl") }
func (p proyecto) canalPath() string      { return filepath.Join(p.dir, "canal.jsonl") }
func (p proyecto) sesPath() string        { return filepath.Join(p.dir, "sesiones.json") }
func (p proyecto) relevo(r string) string { return filepath.Join(p.dir, "relevo-"+r+".md") }

func (p proyecto) slices() slice.Repo {
	home, _ := os.UserHomeDir()
	base := os.Getenv("ORQ_WORKTREES")
	if base == "" {
		base = filepath.Join(home, "worktrees")
	}
	r := slice.Repo{Root: p.root, OrqDir: p.dir, Worktree: base}
	if l, err := limits.Load(limits.DefaultPath()); err == nil {
		r.MaxSlices = l.MaxSlices
	}
	return r
}

// excluir deja .orq fuera de git sin tocar archivos versionados del proyecto.
func (p proyecto) excluir() error {
	path := filepath.Join(p.root, ".git", "info", "exclude")
	raw, _ := os.ReadFile(path)
	if strings.Contains(string(raw), "/.orq/") {
		return nil
	}
	os.MkdirAll(filepath.Dir(path), 0o755)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString("/.orq/\n")
	return err
}

func texto(args []string, stdin io.Reader) string {
	if len(args) == 1 && args[0] == "-" {
		raw, _ := io.ReadAll(stdin)
		return strings.TrimSpace(string(raw))
	}
	return strings.TrimSpace(strings.Join(args, " "))
}

// ---------- mem ----------

func runMem(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	p, err := abrirProyecto()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if len(args) == 0 {
		args = []string{"ver"}
	}
	switch args[0] {
	case "add":
		fs := flag.NewFlagSet("mem add", flag.ContinueOnError)
		fs.SetOutput(stderr)
		imp := fs.String("imp", "", strings.Join(memoria.Imps, "|"))
		cat := fs.String("cat", "", strings.Join(memoria.Cats, "|"))
		fuente := fs.String("fuente", os.Getenv("ORQ_ROL"), strings.Join(memoria.Fuentes, "|"))
		sl := fs.String("slice", "", "S-NN")
		tipo := fs.String("tipo", "", strings.Join(memoria.Tipos, "|"))
		ev := fs.String("evidencia", "", "recibo, commit, archivo:línea o id de mensaje")
		vence := fs.String("vence", "", "S-NN: deja de valer al cerrarse ese slice")
		reemplaza := fs.String("reemplaza", "", "id que este hecho deja obsoleto")
		if fs.Parse(args[1:]) != nil {
			return 1
		}
		it, n, err := memoria.Add(p.memPath(), memoria.Item{Imp: *imp, Cat: *cat, Fuente: *fuente, Slice: *sl, Tipo: *tipo, Evidencia: *ev, Vence: *vence, Reemplaza: *reemplaza, Texto: texto(fs.Args(), stdin)}, time.Now())
		if err != nil {
			fmt.Fprintln(stderr, "orq mem:", err)
			return 1
		}
		fmt.Fprintf(stdout, "%s guardado (%d de %d líneas)\n", it.ID, n, memoria.Tope)
		if n > memoria.Tope {
			fmt.Fprintln(stdout, "Pasó el tope: corre `orq mem compactar`.")
		}
		return 0
	case "ver":
		fs := flag.NewFlagSet("mem ver", flag.ContinueOnError)
		cat := fs.String("cat", "", "filtra por categoría")
		if fs.Parse(args[1:]) != nil {
			return 1
		}
		items, err := memoria.Load(p.memPath())
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if len(items) == 0 {
			fmt.Fprintln(stdout, "memoria vacía")
			return 0
		}
		fmt.Fprint(stdout, memoria.Render(items, *cat))
		return 0
	case "compactar":
		// Con el candado tomado, nadie agrega líneas que el resumen borraría.
		soltar, err := orqdir.Candado(p.memPath())
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		defer soltar()
		items, err := memoria.Load(p.memPath())
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		res, err := memoria.Compact(items, memoria.Tope*2/3, escriba.Compactar, time.Now())
		if err != nil {
			fmt.Fprintln(stderr, "orq mem:", err)
			return 1
		}
		if len(res) == len(items) {
			fmt.Fprintf(stdout, "%d líneas: no hace falta compactar\n", len(items))
			return 0
		}
		bak := p.memPath() + ".antes"
		raw, _ := os.ReadFile(p.memPath())
		os.WriteFile(bak, raw, 0o644)
		if err := memoria.Save(p.memPath(), res); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintf(stdout, "memoria compactada: %d → %d líneas (copia previa: %s)\n", len(items), len(res), bak)
		return 0
	}
	fmt.Fprint(stderr, usage)
	return 1
}

// ---------- msg ----------

func runMsg(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	p, err := abrirProyecto()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if len(args) == 0 {
		args = []string{"leer"}
	}
	ses, err := canal.LoadSesiones(p.sesPath())
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	switch args[0] {
	case "send":
		fs := flag.NewFlagSet("msg send", flag.ContinueOnError)
		fs.SetOutput(stderr)
		para := fs.String("para", "", "atena|orquestador")
		tipo := fs.String("tipo", "", strings.Join(canal.Tipos, "|"))
		de := fs.String("de", "", "rol que escribe (por defecto, el de este panel)")
		ref := fs.String("ref", "", "id del mensaje al que respondes")
		clave := fs.String("clave", "", "clave de idempotencia")
		intento := fs.String("intento", "", "intento al que se refiere un cierre (S-NN/aK; aK si hay un solo slice)")
		if fs.Parse(args[1:]) != nil {
			return 1
		}
		if *de == "" {
			*de = os.Getenv("ORQ_ROL")
		}
		if *de == "" {
			*de = canal.RolDePane(ses, herdr.CurrentPane())
		}
		m := canal.Msg{TS: time.Now().UTC().Truncate(time.Second), De: *de, Para: *para, Tipo: *tipo, Texto: texto(fs.Args(), stdin), Ref: *ref, Clave: *clave}
		var e *slice.Estado
		var errSlice error
		if strings.HasPrefix(*intento, "S-") {
			e, errSlice = p.slices().Load(strings.SplitN(*intento, "/", 2)[0])
		} else {
			e, errSlice = p.slices().Load()
		}
		if errSlice != nil && m.Tipo == "cierre" {
			fmt.Fprintf(stderr, "orq msg: %v. Un cierre debe decir su intento completo: --intento S-NN/aK\n", errSlice)
			return 5
		}
		if e != nil {
			m.Attempt = e.Attempt()
			// Un cierre solo vale para el intento vigente, y lo decide orq, no el modelo.
			if m.Tipo == "cierre" {
				pedido := *intento
				if pedido != "" && !strings.HasPrefix(pedido, "S-") {
					pedido = fmt.Sprintf("%s/a%s", e.ID(), strings.TrimPrefix(pedido, "a"))
				}
				if pedido == "" {
					fmt.Fprintf(stderr, "orq msg: un cierre debe decir de qué intento es: --intento %s\n", e.Attempt())
					return 5
				}
				if pedido != e.Attempt() {
					fmt.Fprintf(stderr, "orq msg: cierre de %s, pero el intento vigente es %s: es un resultado viejo, no se entrega\n", pedido, e.Attempt())
					return 5
				}
			}
		}
		if err := canal.Append(p.canalPath(), &m); errors.Is(err, canal.ErrDuplicado) {
			fmt.Fprintf(stdout, "ya estaba guardado como %s (%s); no se repite\n", m.ID, m.Estado)
			return 0
		} else if err != nil {
			fmt.Fprintln(stderr, "orq msg:", err)
			return 1
		}
		dest, ok := ses[*para]
		if !ok {
			fmt.Fprintf(stdout, "guardado como %s, pendiente; %s aún no tiene sesión (orq up)\n", m.ID, *para)
			return 0
		}
		if err := herdr.Prompt(dest.Agente, m.Format()); err != nil {
			if errors.Is(err, herdr.ErrTimeout) {
				fmt.Fprintf(stderr, "%s guardado; herdr no respondió: puede haber llegado o no. Revisa y usa `orq msg reconciliar`, no lo reenvíes a ciegas\n", m.ID)
			} else {
				fmt.Fprintf(stderr, "%s guardado, pero no se pudo entregar a %s: %v\n", m.ID, dest.Agente, err)
			}
			return 4
		}
		canal.MarcarEntregado(p.canalPath(), m.ID, time.Now())
		fmt.Fprintf(stdout, "entregado a %s\n", dest.Agente)
		return 0
	case "reconciliar":
		ms, err := canal.Load(p.canalPath())
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		pend := canal.Pendientes(ms)
		if len(args) >= 3 && args[1] == "--reenviar" {
			for _, m := range pend {
				if m.ID != args[2] {
					continue
				}
				dest, ok := ses[m.Para]
				if !ok {
					fmt.Fprintf(stderr, "%s no tiene sesión\n", m.Para)
					return 1
				}
				if err := herdr.Prompt(dest.Agente, m.Format()); err != nil {
					fmt.Fprintln(stderr, err)
					return 4
				}
				canal.MarcarEntregado(p.canalPath(), m.ID, time.Now())
				fmt.Fprintf(stdout, "%s reenviado y entregado\n", m.ID)
				return 0
			}
			fmt.Fprintf(stderr, "%s no está pendiente\n", args[2])
			return 1
		}
		if len(pend) == 0 {
			fmt.Fprintln(stdout, "sin mensajes pendientes")
			return 0
		}
		for _, m := range pend {
			fmt.Fprintf(stdout, "%s %s→%s %s %s: %s\n", m.ID, m.De, m.Para, m.Tipo, m.Attempt, recortar(m.Texto, 80))
		}
		fmt.Fprintln(stdout, "si el destino no lo recibió: orq msg reconciliar --reenviar ID")
		return 3
	case "leer":
		fs := flag.NewFlagSet("msg leer", flag.ContinueOnError)
		n := fs.Int("n", 10, "últimos N")
		if fs.Parse(args[1:]) != nil {
			return 1
		}
		ms, err := canal.Load(p.canalPath())
		ms = canal.Vigentes(ms)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if len(ms) > *n {
			ms = ms[len(ms)-*n:]
		}
		for _, m := range ms {
			fmt.Fprintf(stdout, "%s %s→%s [%s] %s\n", m.TS.Local().Format("02-01 15:04"), m.De, m.Para, m.Tipo, m.Texto)
		}
		return 0
	}
	fmt.Fprint(stderr, usage)
	return 1
}

// ---------- up y relevo ----------

type arranque struct {
	kind, modelo, esfuerzo string
}

func flagsArranque(fs *flag.FlagSet) *arranque {
	a := &arranque{}
	fs.StringVar(&a.kind, "kind", "", "agente de herdr: claude|codex (por defecto, el del rol)")
	fs.StringVar(&a.modelo, "modelo", "", "modelo (por defecto, el del rol)")
	fs.StringVar(&a.esfuerzo, "esfuerzo", "", "esfuerzo (por defecto, el del rol)")
	return a
}

// Cada rol tiene su agente por defecto. El Orquestador solo reparte: no necesita el modelo más caro.
// El Planificador piensa el plan: Opus. Astra mejora el plan desde otro proveedor: gpt-6-astra.
var porRol = map[string]arranque{
	"orquestador":  {"claude", "sonnet", "medium"},
	"planificador": {"claude", "opus", "high"},
	"astra":        {"codex", "gpt-6-astra", "high"},
	"atena":        {"claude", "opus", "medium"},
}

func (a *arranque) completar(rol string) {
	d := porRol[rol]
	if a.kind == "" {
		a.kind = d.kind
	}
	if a.kind != d.kind && a.modelo == "" { // otro proveedor: su modelo por defecto
		a.modelo = map[string]string{"claude": "opus", "codex": "gpt-6.1-sol"}[a.kind]
	}
	if a.modelo == "" {
		a.modelo = d.modelo
	}
	if a.esfuerzo == "" {
		a.esfuerzo = d.esfuerzo
	}
}

// agentArgs traduce modelo y esfuerzo a cada CLI.
func agentArgs(kind, modelo, esfuerzo, name string) []string {
	if kind == "codex" {
		return []string{"-m", modelo, "-c", "model_reasoning_effort=" + esfuerzo}
	}
	return []string{"--model", modelo, "--effort", esfuerzo, "-n", name}
}

// etiqueta es el nombre tipado de la pestaña de cada rol: «<rol> · <proyecto>».
func etiqueta(rol string, p proyecto) string {
	return map[string]string{"orquestador": "orq", "planificador": "plan", "astra": "plan"}[rol] + " · " + orqdir.Slug(p.root)
}

const arranqueRol = `You are the %[1]s of orq in this repository. Load skill %[2]s and run "orq skill %[1]s".
Read first: .orq/relevo-%[1]s.md if it exists, then "orq mem ver".
Talk to other roles only via "orq msg send". Then wait for instructions.`

func skillDe(rol string) string {
	switch rol {
	case "atena":
		return "orq-atena"
	case "planificador", "astra":
		return "orq-planner"
	}
	return "orq-orchestrator"
}

// lanzar abre un panel con el rol, lo nombra y le da el arranque.
func lanzar(p proyecto, ses map[string]canal.Sesion, rol string, a *arranque) (canal.Sesion, error) {
	gen := ses[rol].Gen + 1
	name := rol + "-" + orqdir.Slug(p.root)
	if gen > 1 {
		name = fmt.Sprintf("%s-%d", name, gen)
	}
	a.completar(rol)
	env := map[string]string{"ORQ_ROL": rol}
	// Distribución fija: orq en su pestaña, plan en la suya; Astra al lado del Planificador.
	var tab, pane string
	var err error
	if pl, ok := ses["planificador"]; rol == "astra" && ok && pl.Pane != "" {
		tab = pl.Tab
		pane, err = herdr.SplitDe(pl.Pane, p.root, env)
	} else {
		tab, pane, err = herdr.Tab(etiqueta(rol, p), p.root, env)
	}
	if err != nil {
		return canal.Sesion{}, err
	}
	if err := herdr.Start(name, a.kind, pane, rolArgs(a.kind, a.modelo, a.esfuerzo, name, p.root)); err != nil {
		return canal.Sesion{}, fmt.Errorf("%v (el panel %s quedó abierto)", err, pane)
	}
	s := canal.Sesion{Agente: name, Pane: pane, Tab: tab, Desde: time.Now().UTC().Truncate(time.Second), Gen: gen}
	ses[rol] = s
	if err := canal.SaveSesiones(p.sesPath(), ses); err != nil {
		return s, err
	}
	return s, herdr.Prompt(name, fmt.Sprintf(arranqueRol, rol, skillDe(rol)))
}

func runUp(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("up", flag.ContinueOnError)
	fs.SetOutput(stderr)
	a := flagsArranque(fs)
	if fs.Parse(args) != nil {
		return 1
	}
	if !herdr.Inside() {
		fmt.Fprintln(stderr, "orq up: corre esto desde un panel de herdr (falta HERDR_ENV). Orca llegará después.")
		return 1
	}
	if reason, halted := state.ReadHalt(state.Dir()); halted {
		fmt.Fprintf(stderr, "orq up: HALT activo (%s). Libera espacio y `orq halt --clear`.\n", reason)
		return 2
	}
	p, err := abrirProyecto()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := p.excluir(); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	ses, err := canal.LoadSesiones(p.sesPath())
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if o, ok := ses["orquestador"]; ok {
		fmt.Fprintf(stderr, "ya hay orquestador (%s, panel %s). Para una sesión fresca: orq relevo orquestador\n", o.Agente, o.Pane)
		return 1
	}
	fname := "atena-" + orqdir.Slug(p.root)
	if err := herdr.Rename(herdr.CurrentPane(), fname); err != nil {
		fmt.Fprintf(stderr, "aviso: no se pudo nombrar este panel como %s (%v)\n", fname, err)
	}
	ses["atena"] = canal.Sesion{Agente: fname, Pane: herdr.CurrentPane(), Desde: time.Now().UTC().Truncate(time.Second), Gen: 1}
	o, err := lanzar(p, ses, "orquestador", a)
	if err != nil {
		fmt.Fprintln(stderr, "orq up:", err)
		return 1
	}
	fmt.Fprintf(stdout, "atena: %s (this pane)\norquestador: %s (tab %q, pane %s)\nlayout: %s\nTalk: orq msg send --para <rol> --tipo <tipo> \"...\"\n", fname, o.Agente, etiqueta("orquestador", p), o.Pane, layout)
	return 0
}

const lineasRelevo = 25

func runRelevo(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("relevo", flag.ContinueOnError)
	fs.SetOutput(stderr)
	a := flagsArranque(fs)
	soloArchivo := fs.Bool("solo-archivo", false, "escribe el relevo sin abrir sesión nueva")
	auto := fs.Bool("auto", false, "hook PreCompact: rol de esta sesión, solo archivo, silencioso si no hay rol")
	forzar := fs.Bool("forzar", false, "abre sesión aunque ya haya una nueva viva")
	if fs.Parse(args) != nil {
		return 1
	}
	p, err := abrirProyecto()
	if *auto {
		if err != nil || p.rolActual() == "" {
			return 0
		}
		*soloArchivo = true
	} else if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	rol := fs.Arg(0)
	if *auto {
		rol = p.rolActual()
	}
	if !slices.Contains(canal.Roles, rol) {
		fmt.Fprintln(stderr, "uso: orq relevo [--solo-archivo] atena|orquestador")
		return 1
	}
	ses, _ := canal.LoadSesiones(p.sesPath())
	// Una sola sesión por rol: si la anterior relevo ya abrió una sesión que sigue viva, no se abre otra.
	if prev, ok := ses[rol]; ok && !*soloArchivo && !*forzar && prev.Gen > 1 && time.Since(prev.Desde) < 15*time.Minute && herdr.Inside() {
		if _, err := herdr.Run("agent", "get", prev.Agente); err == nil {
			fmt.Fprintf(stderr, "orq relevo: %s ya tiene una sesión nueva viva (%s, hace %s). No se abre otra; usa --forzar si de verdad hace falta\n", rol, prev.Agente, time.Since(prev.Desde).Round(time.Second))
			return 1
		}
	}
	rv := p.datosRelevo(rol)
	var msgs strings.Builder
	ms, _ := canal.Load(p.canalPath())
	ms = canal.Vigentes(ms)
	for _, m := range ms[max(0, len(ms)-15):] {
		if m.De == rol || m.Para == rol {
			fmt.Fprintf(&msgs, "%s %s→%s [%s] %s\n", m.ID, m.De, m.Para, m.Tipo, m.Texto)
		}
	}
	est, _ := json.Marshal(rv)
	ahora, err := escriba.Relevo(rol, memoria.Render(p.memVigente(), ""), string(est), msgs.String(), lineasRelevo)
	if err != nil {
		ahora = ""
		fmt.Fprintf(stderr, "aviso: sin resumen del escriba (%v); el relevo lleva solo datos comprobados\n", err)
	}
	md := rv.Markdown(time.Now().Format("2006-01-02 15:04"), ahora)
	if err := orqdir.WriteAtomic(p.relevo(rol), []byte(md)); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	orqdir.WriteAtomic(strings.TrimSuffix(p.relevo(rol), ".md")+".json", est)
	fmt.Fprintf(stdout, "relevo escrito: %s (%d líneas)\n", p.relevo(rol), strings.Count(md, "\n"))
	if *soloArchivo {
		return 0
	}
	if !herdr.Inside() {
		fmt.Fprintln(stdout, "fuera de herdr: abre una sesión nueva y pídele que lea ese archivo.")
		return 0
	}
	viejo := ses[rol]
	nuevo, err := lanzar(p, ses, rol, a)
	if err != nil {
		fmt.Fprintln(stderr, "orq relevo:", err)
		return 1
	}
	fmt.Fprintf(stdout, "sesión nueva: %s (panel %s). Los mensajes ya van a ella.\n", nuevo.Agente, nuevo.Pane)
	if viejo.Pane != "" {
		fmt.Fprintf(stdout, "La sesión anterior (%s) ya puede cerrarse: herdr pane close %s\n", viejo.Agente, viejo.Pane)
	}
	return 0
}

// ---------- slice ----------

func runSlice(args []string, stdout, stderr io.Writer) int {
	p, err := abrirProyecto()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	r := p.slices()
	nn := ""
	for i := 0; i < len(args); i++ {
		if args[i] == "--slice" && i+1 < len(args) {
			nn = strings.TrimPrefix(args[i+1], "S-")
			args = append(args[:i:i], args[i+2:]...)
			break
		}
	}
	if len(args) == 0 {
		args = []string{"estado"}
	}
	// Pedir ayuda nunca ejecuta: un «verificar --help» corría el examen y podía gastar un intento.
	if slices.ContainsFunc(args, func(a string) bool { return a == "-h" || a == "--help" || a == "help" }) {
		fmt.Fprintln(stdout, "uso: orq slice [--slice NN] abrir|tomar|alcance|verificar|juntar|podar|examen|estado|recibo|rehuella ... · guía: orq skill orquestador")
		return 0
	}
	show := func(out string) {
		lines := strings.Split(strings.TrimSpace(out), "\n")
		if len(lines) > 30 {
			lines = append([]string{"…"}, lines[len(lines)-30:]...)
		}
		if strings.TrimSpace(out) != "" {
			fmt.Fprintln(stdout, strings.Join(lines, "\n"))
		}
	}
	switch args[0] {
	case "abrir":
		fs := flag.NewFlagSet("slice abrir", flag.ContinueOnError)
		fs.SetOutput(stderr)
		titulo := fs.String("titulo", "", "historia en pocas palabras")
		test := fs.String("test", "", "ruta del examen de aceptación, relativa a la raíz")
		cmd := fs.String("cmd", "", "comando que corre el examen")
		sinScouts := fs.String("sin-scouts", "", "motivo para saltar el contexto (cambio trivial)")
		bundle := fs.String("bundle", "", "config del runner y fixtures que tampoco se tocan, separados por coma")
		alcance := fs.String("alcance", "", "rutas que la historia justifica tocar, separadas por coma (alarma de desalineación)")
		sano := fs.String("sano", "", "comando que prueba que la base está sana antes de empezar")
		reporte := fs.String("reporte", "", "archivo que deja el runner (para contar pruebas)")
		formato := fs.String("formato", "", "tap|junit|gotest-json")
		omitir := fs.Bool("permite-omitir", false, "acepta pruebas omitidas")
		aislado := fs.Bool("aislado", false, "corre el examen en una copia limpia del commit")
		timeout := fs.Int("timeout", 0, "segundos máximos del examen (0 = 600)")
		estricto := fs.Bool("estricto", false, "--reporte/--formato obligatorios: sin contadores no hay verde (también limits.json conteo_estricto)")
		if len(args) < 2 || fs.Parse(args[2:]) != nil || *titulo == "" || *test == "" || *cmd == "" {
			fmt.Fprintln(stderr, "uso: orq slice abrir NN --titulo \"...\" --test ruta --cmd \"...\" [--bundle a,b] [--sano \"cmd\"] [--reporte f --formato tap|junit|gotest-json] [--permite-omitir] [--aislado] [--timeout s] [--sin-scouts \"motivo\"]")
			return 1
		}
		if *sinScouts == "" && !hayContexto(p, "S-"+args[1]) {
			fmt.Fprintf(stderr, "orq slice: C·Contexto antes de A·Análisis. Guarda los criterios de los scouts con `orq mem add --fuente scout --cat tecnico --slice S-%s ...`, o abre con --sin-scouts \"motivo\".\n", args[1])
			return 1
		}
		var extra []string
		for _, f := range strings.Split(*bundle, ",") {
			if f = strings.TrimSpace(f); f != "" {
				extra = append(extra, f)
			}
		}
		e, out, err := r.Abrir(args[1], *titulo, *test, *cmd, slice.Opciones{Alcance: partir(*alcance), Bundle: extra, Sano: *sano, Reporte: *reporte, Formato: *formato, PermiteOmitir: *omitir, Aislado: *aislado, Timeout: *timeout, Estricto: *estricto || estrictoPorLimites()}, time.Now())
		show(out)
		if err != nil {
			fmt.Fprintln(stderr, "orq slice:", err)
			return 1
		}
		fmt.Fprintf(stdout, "%s abierto en rojo sobre %s (%s). Worktree: %s\nPaquete protegido: %s\nAsigna el escritor: orq slice tomar <quien>\n", e.Attempt(), e.Base, e.SHARojo[:8], e.Worktree, strings.Join(e.Bundle, ", "))
		return 0
	case "tomar":
		fs := flag.NewFlagSet("slice tomar", flag.ContinueOnError)
		fs.SetOutput(stderr)
		relevo := fs.Bool("relevo", false, "reemplaza al escritor actual en el mismo worktree")
		if len(args) < 2 || fs.Parse(args[2:]) != nil {
			fmt.Fprintln(stderr, "uso: orq slice tomar QUIEN [--relevo]")
			return 1
		}
		e, err := r.Tomar(nn, args[1], *relevo)
		if err != nil {
			fmt.Fprintln(stderr, "orq slice:", err)
			return 1
		}
		fmt.Fprintf(stdout, "%s: escribe %s en %s\n", e.Attempt(), e.Escritor, e.Worktree)
		return 0
	case "alcance":
		fs := flag.NewFlagSet("slice alcance", flag.ContinueOnError)
		fs.SetOutput(stderr)
		aprobado := fs.String("aprobado", "", "quién lo aprobó y por qué (req o mem de Atena/Owner)")
		zona := fs.String("zona", "", "también la suma a la zona de ese worker ya lanzado")
		if len(args) < 2 || fs.Parse(args[2:]) != nil || *aprobado == "" {
			fmt.Fprintln(stderr, "uso: orq slice alcance RUTA[,RUTA] --aprobado \"mNN|cNN de Atena/Owner: motivo\" [--zona back]")
			return 1
		}
		if err := p.cambioDeContrato(*aprobado, r, nn); err != nil {
			fmt.Fprintln(stderr, "orq slice:", err)
			return 1
		}
		e, err := r.Ampliar(nn, partir(args[1]))
		if err == nil && *zona != "" {
			e, err = r.AmpliarZona(nn, *zona, partir(args[1]))
		}
		if err != nil {
			fmt.Fprintln(stderr, "orq slice:", err)
			return 1
		}
		memoria.Add(p.memPath(), memoria.Item{Imp: "alta", Cat: "decision", Fuente: "orquestador", Slice: e.ID(), Vence: e.ID(),
			Texto: recortar(fmt.Sprintf("%s alcance + %s (%s)", e.ID(), args[1], *aprobado), memoria.MaxTexto)}, time.Now())
		fmt.Fprintf(stdout, "%s: alcance %s\n", e.ID(), strings.Join(e.Alcance, ", "))
		return 0
	case "verificar":
		e, out, err := r.Verificar(nn, time.Now())
		show(out)
		if err != nil {
			fmt.Fprintln(stderr, "orq slice:", err)
			return 1
		}
		fmt.Fprintf(stdout, "%s verificado: el examen pasa intacto en %s.\n", e.Attempt(), e.Recibo.Head[:8])
		if e.Recibo.Contado {
			fmt.Fprintf(stdout, "  %d ejecutadas, %d omitidas, %d fallidas\n", e.Recibo.Ejecutados, e.Recibo.Omitidos, e.Recibo.Fallidos)
		} else {
			fmt.Fprintf(stdout, "  aviso: %s\n", e.Recibo.Nota)
		}
		fmt.Fprintln(stdout, "  recibo: orq slice recibo · falta la demo del Owner")
		if p.alinear(e.NN, stdout, stderr) == 3 {
			fmt.Fprintln(stdout, "  verde, pero desalineado: resuelve la pregunta con el Atena antes del cierre")
		}
		return 0
	case "rehuella":
		if len(args) < 2 || strings.TrimSpace(strings.Join(args[1:], " ")) == "" {
			fmt.Fprintln(stderr, "uso: orq slice rehuella \"por qué la huella estaba mal y quién lo aprobó\"")
			return 1
		}
		motivo := strings.Join(args[1:], " ")
		e, err := r.Rehuella(nn, motivo)
		if err != nil {
			fmt.Fprintln(stderr, "orq slice:", err)
			return 1
		}
		memoria.Add(p.memPath(), memoria.Item{Imp: "alta", Cat: "decision", Fuente: "orquestador", Slice: e.ID(), Vence: e.ID(),
			Texto: recortar(e.Attempt()+" huella reparada: "+motivo, memoria.MaxTexto)}, time.Now())
		fmt.Fprintf(stdout, "%s: huella tomada del rojo (%s), el examen es idéntico. Vuelve a verificar\n", e.Attempt(), e.SHARojo[:7])
		return 0
	case "reabrir":
		if len(args) < 2 || strings.TrimSpace(strings.Join(args[1:], " ")) == "" {
			fmt.Fprintln(stderr, "uso: orq slice reabrir \"qué se repara y quién lo pidió\"")
			return 1
		}
		motivo := strings.Join(args[1:], " ")
		e, err := r.Reabrir(nn, motivo)
		if err != nil {
			fmt.Fprintln(stderr, "orq slice:", err)
			return 1
		}
		memoria.Add(p.memPath(), memoria.Item{Imp: "alta", Cat: "decision", Fuente: "orquestador", Slice: e.ID(), Vence: e.ID(),
			Texto: recortar(e.Attempt()+" reabierto: "+motivo, memoria.MaxTexto)}, time.Now())
		fmt.Fprintf(stdout, "%s reabierto en el mismo worktree, con su trabajo. El recibo ya no vale: lanza la reparación y vuelve a verificar\n", e.Attempt())
		return 0
	case "reintentar":
		owner := false
		var resto []string
		for _, a := range args[1:] {
			if a == "--decision-owner" {
				owner = true
			} else {
				resto = append(resto, a)
			}
		}
		if len(resto) == 0 {
			fmt.Fprintln(stderr, "uso: orq slice reintentar \"qué cambia en este intento\" [--decision-owner]")
			return 1
		}
		args = append(args[:1], resto...)
		e, err := r.Reintentar(nn, strings.Join(resto, " "), owner)
		if err != nil {
			fmt.Fprintln(stderr, "orq slice:", err)
			return 1
		}
		memoria.Add(p.memPath(), memoria.Item{Imp: "alta", Cat: "decision", Fuente: "orquestador", Slice: e.ID(), Vence: e.ID(),
			Texto: recortar(e.Attempt()+": "+strings.Join(args[1:], " "), memoria.MaxTexto)}, time.Now())
		p.aprenderSolo(strings.Join(resto, " "), "reintento", e.Attempt())
		fmt.Fprintf(stdout, "%s abierto en el mismo worktree. Recibos anteriores ya no valen. Asigna escritor: orq slice tomar <quien>\n", e.Attempt())
		return 0
	case "cerrar":
		fs := flag.NewFlagSet("slice cerrar", flag.ContinueOnError)
		fs.SetOutput(stderr)
		ok := fs.Bool("aceptado", false, "el Owner aceptó la demo")
		motivo := fs.String("rechazado", "", "motivo del rechazo en una línea")
		if fs.Parse(args[1:]) != nil || *ok == (*motivo != "") {
			fmt.Fprintln(stderr, "uso: orq slice cerrar --aceptado | --rechazado \"motivo\"")
			return 1
		}
		reg, cuarentena, out, err := r.Cerrar(nn, *ok, *motivo, time.Now())
		show(out)
		if errors.Is(err, slice.ErrYaCerrado) {
			fmt.Fprintf(stdout, "S-%s ya estaba cerrado como %s; no se repite nada\n", reg.NN, reg.Result)
			return 0
		}
		if err != nil {
			fmt.Fprintln(stderr, "orq slice:", err)
			return 1
		}
		if len(cuarentena) > 0 {
			fmt.Fprintf(stdout, "en cuarentena (no se borró, revísalo): %s\n", strings.Join(cuarentena, ", "))
		}
		mem := memoria.Item{Imp: "media", Cat: "aprendizaje", Fuente: "orquestador", Slice: "S-" + reg.NN,
			Texto: recortar(fmt.Sprintf("S-%s %s: %s", reg.NN, reg.Result, reg.Titulo), memoria.MaxTexto)}
		if !*ok {
			mem.Imp, mem.Cat, mem.Fuente = "critica", "requisito", "owner"
			mem.Texto = recortar(fmt.Sprintf("Rechazo de S-%s: %s", reg.NN, *motivo), memoria.MaxTexto)
			p.aprenderSolo("El Owner rechazó: "+*motivo, "rechazo", "S-"+reg.NN)
		}
		memoria.Add(p.memPath(), mem, time.Now())
		canal.Archivar(p.canalPath(), "S-"+reg.NN, time.Now()) // append-only: lo pendiente de otros slices sigue
		os.Remove(p.planPath(reg.NN))
		// Higiene: los paneles de los workers de este slice ya no tienen trabajo.
		if ses, err := canal.LoadSesiones(p.sesPath()); err == nil {
			if c, _ := p.cerrarSesiones(ses, func(k string) bool { return strings.HasPrefix(k, "worker-") && strings.HasSuffix(k, "-s"+reg.NN) }); len(c) > 0 {
				fmt.Fprintf(stdout, "paneles cerrados: %s\n", strings.Join(c, ", "))
			}
		}
		if err := p.escribirFicha(); err != nil {
			fmt.Fprintf(stdout, "aviso: la ficha del proyecto no se actualizó (%v)\n", err)
		} else {
			fmt.Fprintf(stdout, "ficha del proyecto actualizada: %s\n", p.fichaPath())
		}
		// Higiene: lo que orq creó y ya no se usa se retira con el protocolo seguro.
		if res, err := p.gc(false); err != nil {
			fmt.Fprintf(stdout, "aviso: gc no corrió (%v)\n", err)
		} else if len(res) > 0 {
			mostrarGC(stdout, res)
		}
		fmt.Fprintf(stdout, "S-%s cerrado como %s. Mensajes de S-%s archivados; memoria actualizada.\n", reg.NN, reg.Result, reg.NN)
		if !*ok {
			fmt.Fprintf(stdout, "La rama %s se conserva para revisar.\n", "slice/"+reg.NN)
		}
		return 0
	case "lanzar":
		fs := flag.NewFlagSet("slice lanzar", flag.ContinueOnError)
		fs.SetOutput(stderr)
		agente := fs.String("agente", "claude", "claude (Sonnet) | codex")
		zona := fs.String("zona", "", "front, back, test…: suma el worker al equipo del slice")
		rutas := fs.String("rutas", "", "carpetas de esa zona, separadas por coma")
		alcance := fs.String("alcance", "solo lo necesario para que el examen pase", "qué sí y qué no")
		suite := fs.String("suite", "", "comando de la suite rápida")
		pres := fs.String("presupuesto", "45 min", "tiempo o tokens")
		dif := fs.String("dificultad", "", "baja|media|alta: elige modelo y esfuerzo (haiku/sonnet/opus; luna/sol)")
		tarea := fs.String("tarea", "", "ID del plan o 'siguiente': zona, rutas y dificultad salen de la tarea")
		if fs.Parse(args[1:]) != nil {
			return 1
		}
		var rs []string
		for _, x := range strings.Split(*rutas, ",") {
			if x = strings.TrimSpace(x); x != "" {
				rs = append(rs, x)
			}
		}
		nombre, err := p.lanzarWorker(nn, *agente, *zona, rs, *alcance, *suite, *pres, *dif, *tarea)
		if err != nil {
			fmt.Fprintln(stderr, "orq slice:", err)
			return 1
		}
		fmt.Fprintf(stdout, "%s trabajando en su worktree con el contrato. Mira el avance con `orq slice estado`.\n", nombre)
		return 0
	case "examen":
		fs := flag.NewFlagSet("slice examen", flag.ContinueOnError)
		fs.SetOutput(stderr)
		cmd := fs.String("cmd", "", "comando nuevo del examen si debe correr también los archivos sumados")
		aprobado := fs.String("aprobado", "", "req de Atena u Owner que aprueba el examen nuevo")
		if len(args) < 2 || fs.Parse(args[2:]) != nil || *aprobado == "" {
			fmt.Fprintln(stderr, "uso: orq slice examen [--slice NN] ARCHIVO[,ARCHIVO] --aprobado \"mNN|cNN de Atena/Owner\" [--cmd \"...\"]")
			return 1
		}
		if err := p.cambioDeContrato(*aprobado, r, nn); err != nil {
			fmt.Fprintln(stderr, "orq slice:", err)
			return 1
		}
		e, err := r.SumarExamen(nn, partir(args[1]), *cmd, *aprobado)
		if err != nil {
			fmt.Fprintln(stderr, "orq slice:", err)
			return 1
		}
		fmt.Fprintf(stdout, "%s: examen protegido %s (base %s)\n  cmd: %s\n", e.ID(), strings.Join(e.Bundle, ", "), e.SHAExamen[:8], e.Cmd)
		return 0
	case "podar":
		if len(args) < 2 {
			fmt.Fprintln(stderr, "uso: orq slice podar [--slice NN] WORKER [WORKER...]  (worker terminado y juntado: quita worktree, rama y panel del equipo)")
			return 1
		}
		_, informe, err := r.Podar(nn, args[1:])
		for _, l := range informe {
			fmt.Fprintln(stdout, "  "+l)
		}
		if err != nil {
			fmt.Fprintln(stderr, "orq slice:", err)
			return 1
		}
		return 0
	case "juntar":
		e, inf, err := r.Juntar(nn)
		for _, l := range inf {
			fmt.Fprintln(stdout, "  "+l)
		}
		if err != nil {
			fmt.Fprintln(stderr, "orq slice:", err)
			return 1
		}
		fmt.Fprintf(stdout, "%s: equipo juntado en %s. Siguiente: orq slice verificar --slice %s\n", e.Attempt(), e.Worktree, e.NN)
		p.alinear(e.NN, stdout, stderr)
		if pl, err := plan.Leer(p.planPath(e.NN)); err == nil {
			listas, _ := plan.Listas(pl)
			fmt.Fprintf(stdout, "plan: %d pendientes; marca lo terminado con `orq plan hecha ID`; listas: %s\n", pl.Pendientes(), ids(listas))
			// El Planificador vigila su plan: cada junta le pide revisarlo.
			if ses, _ := canal.LoadSesiones(p.sesPath()); ses["planificador"].Agente != "" && herdr.Inside() {
				herdr.Prompt(ses["planificador"].Agente, fmt.Sprintf("[orq] S-%s merged. Run `orq plan revisar --slice %s` and keep the plan minimal and current.", e.NN, e.NN))
			}
		}
		// Documentación barata en paralelo: Haiku actualiza la ficha sin frenar al equipo.
		if self, err := os.Executable(); err == nil && os.Getenv("ORQ_SIN_DOC") == "" {
			c := exec.Command(self, "doc")
			c.Dir = p.root
			if c.Start() == nil {
				go c.Wait()
				fmt.Fprintln(stdout, "la ficha del proyecto se actualiza en segundo plano (Haiku): .orq/PROYECTO.md")
			}
		}
		return 0
	case "contrato":
		fs := flag.NewFlagSet("slice contrato", flag.ContinueOnError)
		fs.SetOutput(stderr)
		worker := fs.String("worker", "", "nombre del worker (por defecto, el escritor)")
		alcance := fs.String("alcance", "solo lo necesario para que el examen pase", "qué sí y qué no")
		suite := fs.String("suite", "", "comando de la suite rápida")
		pres := fs.String("presupuesto", "45 min", "tiempo o tokens")
		if fs.Parse(args[1:]) != nil {
			return 1
		}
		c, err := p.contrato(nn, *worker, *alcance, *suite, *pres)
		if err != nil {
			fmt.Fprintln(stderr, "orq slice:", err)
			return 1
		}
		fmt.Fprint(stdout, c)
		return 0
	case "recibo":
		e, err := r.Load(nn)
		if err != nil || e == nil || e.Recibo == nil {
			fmt.Fprintln(stdout, "sin recibo vigente; el historial está en .orq/recibos.jsonl")
			return 0
		}
		raw, _ := json.MarshalIndent(e.Recibo, "", "  ")
		fmt.Fprintln(stdout, string(raw))
		return 0
	case "estado":
		ab, err := r.Abiertos()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if len(ab) == 0 {
			fmt.Fprintln(stdout, "sin slice abierto")
		}
		for _, e := range ab {
			if nn != "" && e.NN != nn {
				continue
			}
			fmt.Fprintf(stdout, "%s «%s» · %s\n  paquete: %s (%s)\n  base: %s@%s  worktree: %s (%s)\n  escritor: %s  fallos: %d/%d\n", e.Attempt(), e.Titulo, e.Fase, strings.Join(e.Bundle, ", "), e.Cmd, e.Base, e.SHARojo[:8], e.Worktree, e.Procedencia, valor(e.Escritor), e.Fallos, slice.MaxFallos)
			if e.Motivo != "" {
				fmt.Fprintf(stdout, "  bloqueado: %s\n", e.Motivo)
			}
			if e.Recibo != nil {
				fmt.Fprintf(stdout, "  recibo: %s exit %d en %s\n", e.Recibo.Attempt, e.Recibo.Exit, e.Recibo.Head[:8])
			}
			for _, w := range e.Equipo {
				j := "sin juntar"
				if w.Juntado != "" {
					j = "juntado " + w.Juntado[:8]
				}
				fmt.Fprintf(stdout, "  equipo: %-22s %-6s %-6s %s · %s\n", w.Nombre, w.Agente, w.Zona, strings.Join(w.Rutas, ","), j)
			}
		}
		if h, err := r.Huerfanos(); err == nil && len(h) > 0 {
			fmt.Fprintf(stdout, "worktrees sobrantes (%d): revísalos y quítalos con git worktree remove:\n  %s\n", len(h), strings.Join(h, "\n  "))
		}
		return 0
	}
	fmt.Fprint(stderr, usage)
	return 1
}

func valor(s string) string {
	if s == "" {
		return "(nadie)"
	}
	return s
}

func hayContexto(p proyecto, nn string) bool {
	items, _ := memoria.Load(p.memPath())
	for _, it := range items {
		if it.Slice == nn && it.Fuente == "scout" {
			return true
		}
	}
	return false
}

func recortar(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

func partir(s string) []string {
	var out []string
	for _, x := range strings.Split(s, ",") {
		if x = strings.TrimSpace(x); x != "" {
			out = append(out, x)
		}
	}
	return out
}

// rolArgs: un rol Codex (Astra) escribe rojos y planes en los repos hermanos del producto,
// así que además del sandbox de worker puede escribir en la carpeta que los contiene.
func rolArgs(kind, modelo, esfuerzo, name, root string) []string {
	args := workerArgs(kind, modelo, esfuerzo, name, root)
	if kind == "codex" {
		args = append(args, "--add-dir", filepath.Dir(root))
	}
	return args
}

// cambioDeContrato: el examen y el alcance solo cambian desde el checkout principal (no desde un
// worktree de worker, aunque falte el hook) y con una decisión registrada de Atena u Owner.
func (p proyecto) cambioDeContrato(aprobado string, r slice.Repo, nn string) error {
	if wd, _ := os.Getwd(); gate.IsLinkedWorktree(wd) || gate.IsLinkedWorktree(p.root) {
		return errors.New("desde un worktree de worker no se cambia el examen ni el alcance: lo pide el Orquestador")
	}
	e, err := r.Load(nn)
	if err != nil || e == nil {
		return errors.Join(err, errors.New("no hay slice abierto"))
	}
	return p.decisionDelSlice(aprobado, e.ID(), e.DesdeContrato())
}

// decisionRegistrada exige que --aprobado nombre un id existente: mNN de memoria con fuente
// owner|atena, o cNN del canal enviado por atena. El texto libre no aprueba nada.
func (p proyecto) decisionRegistrada(aprobado string) (string, error) {
	f := strings.Fields(strings.NewReplacer(",", " ", ":", " ", ";", " ").Replace(aprobado))
	if len(f) > 0 {
		id := f[0]
		if items, _ := memoria.Load(p.memPath()); strings.HasPrefix(id, "m") {
			for _, it := range items {
				if it.ID == id && (it.Fuente == "owner" || it.Fuente == "atena") {
					return id, nil
				}
			}
		}
		if ms, _ := canal.Load(p.canalPath()); strings.HasPrefix(id, "c") {
			for _, m := range ms {
				if m.ID == id && m.De == "atena" {
					return id, nil
				}
			}
		}
	}
	return "", fmt.Errorf("--aprobado %q no nombra una decisión registrada: empieza por el id (mNN de `orq mem` con fuente owner|atena, o cNN de un mensaje de atena)", aprobado)
}

// decisionDelSlice liga --aprobado al slice: la decisión registrada (mNN de owner|atena o cNN de
// atena) nombra el slice en su texto o en su campo slice/attempt, y es posterior a la apertura o
// al último cambio de examen o alcance. Una decisión vieja o de otra historia no aprueba nada.
func (p proyecto) decisionDelSlice(aprobado, id string, desde time.Time) error {
	ref, err := p.decisionRegistrada(aprobado)
	if err != nil {
		return err
	}
	nombra := regexp.MustCompile(`(^|[^0-9A-Za-z])` + regexp.QuoteMeta(id) + `([^0-9]|$)`)
	habla := func(campo, texto string, ts time.Time) error {
		if !nombra.MatchString(campo) && !nombra.MatchString(texto) {
			return fmt.Errorf("--aprobado %s no habla de %s: la decisión tiene que nombrar el slice (texto o --slice)", ref, id)
		}
		if ts.Before(desde.Truncate(time.Second)) {
			return fmt.Errorf("--aprobado %s es de %s, anterior a la apertura o al último cambio de contrato de %s (%s): pide una decisión nueva", ref, ts.Format(time.RFC3339), id, desde.Format(time.RFC3339))
		}
		return nil
	}
	if strings.HasPrefix(ref, "m") {
		items, _ := memoria.Load(p.memPath())
		for _, it := range items {
			if it.ID == ref {
				return habla(it.Slice, it.Texto, it.TS)
			}
		}
	}
	ms, _ := canal.Load(p.canalPath())
	for _, m := range ms {
		if m.ID == ref {
			return habla(m.Attempt, m.Texto, m.TS)
		}
	}
	return fmt.Errorf("--aprobado %s no se encontró", ref)
}

// estrictoPorLimites: limits.json puede volver obligatorio el conteo en todos los slices.
func estrictoPorLimites() bool {
	l, err := limits.Load(limits.DefaultPath())
	return err == nil && l.ConteoEstricto
}
