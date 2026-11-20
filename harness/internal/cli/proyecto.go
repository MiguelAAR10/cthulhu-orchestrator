package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"orq/internal/plan"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"

	"orq/internal/app"
	"orq/internal/canal"
	"orq/internal/config"
	"orq/internal/gate"
	"orq/internal/identidad"
	"orq/internal/memoria"
	"orq/internal/orqdir"
	"orq/internal/slice"
	"orq/internal/state"
	"orq/internal/terminal"
)

type proyecto = app.Proyecto

func abrirProyecto() (proyecto, error) { return app.Abrir() }

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
		it, n, err := memoria.Add(p.MemPath(), memoria.Item{Imp: *imp, Cat: *cat, Fuente: *fuente, Slice: *sl, Tipo: *tipo, Evidencia: *ev, Vence: *vence, Reemplaza: *reemplaza, Texto: texto(fs.Args(), stdin)}, time.Now())
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
		items, err := memoria.Load(p.MemPath())
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
		soltar, err := orqdir.Candado(p.MemPath())
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		defer soltar()
		items, err := memoria.Load(p.MemPath())
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		res, err := memoria.Compact(items, memoria.Tope*2/3, p.Escriba().Compactar, time.Now())
		if err != nil {
			fmt.Fprintln(stderr, "orq mem:", err)
			return 1
		}
		if len(res) == len(items) {
			fmt.Fprintf(stdout, "%d líneas: no hace falta compactar\n", len(items))
			return 0
		}
		bak := p.MemPath() + ".antes"
		raw, _ := os.ReadFile(p.MemPath())
		os.WriteFile(bak, raw, 0o644)
		if err := memoria.Save(p.MemPath(), res); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintf(stdout, "memoria compactada: %d → %d líneas (copia previa: %s)\n", len(items), len(res), bak)
		return 0
	}
	fmt.Fprint(stderr, uso())
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
	ses, err := canal.LoadSesiones(p.SesPath())
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
			*de = canal.RolDePane(ses, p.Term.PanelActual())
		}
		m := canal.Msg{TS: time.Now().UTC().Truncate(time.Second), De: *de, Para: *para, Tipo: *tipo, Texto: texto(fs.Args(), stdin), Ref: *ref, Clave: *clave}
		var e *slice.Estado
		var errSlice error
		if strings.HasPrefix(*intento, "S-") {
			e, errSlice = p.Slices().Load(strings.SplitN(*intento, "/", 2)[0])
		} else {
			e, errSlice = p.Slices().Load()
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
		if err := canal.Append(p.CanalPath(), &m); errors.Is(err, canal.ErrDuplicado) {
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
		if err := p.Term.Avisar(context.Background(), dest.Agente, m.Format()); err != nil {
			if errors.Is(err, terminal.ErrDesconocido) {
				fmt.Fprintf(stderr, "%s guardado; el terminal no respondió: puede haber llegado o no. Revisa y usa `orq msg reconciliar`, no lo reenvíes a ciegas\n", m.ID)
			} else {
				fmt.Fprintf(stderr, "%s guardado, pero no se pudo entregar a %s: %v\n", m.ID, dest.Agente, err)
			}
			return 4
		}
		canal.MarcarEntregado(p.CanalPath(), m.ID, time.Now())
		fmt.Fprintf(stdout, "entregado a %s\n", dest.Agente)
		return 0
	case "reconciliar":
		ms, err := canal.Load(p.CanalPath())
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
				if err := p.Term.Avisar(context.Background(), dest.Agente, m.Format()); err != nil {
					fmt.Fprintln(stderr, err)
					return 4
				}
				canal.MarcarEntregado(p.CanalPath(), m.ID, time.Now())
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
			fmt.Fprintf(stdout, "%s %s→%s %s %s: %s\n", m.ID, m.De, m.Para, m.Tipo, m.Attempt, app.Recortar(m.Texto, 80))
		}
		fmt.Fprintln(stdout, "si el destino no lo recibió: orq msg reconciliar --reenviar ID")
		return 3
	case "leer":
		fs := flag.NewFlagSet("msg leer", flag.ContinueOnError)
		n := fs.Int("n", 10, "últimos N")
		if fs.Parse(args[1:]) != nil {
			return 1
		}
		ms, err := canal.Load(p.CanalPath())
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
	fmt.Fprint(stderr, uso())
	return 1
}

// ---------- up y relevo ----------

// flagsArranque: lo que estas banderas no fijan sale de la configuración (`orq config ver`).
func flagsArranque(fs *flag.FlagSet) *config.Agente {
	a := &config.Agente{}
	fs.StringVar(&a.Kind, "kind", "", "agente: claude|codex (por defecto, el del rol)")
	fs.StringVar(&a.Modelo, "modelo", "", "modelo (por defecto, el del rol)")
	fs.StringVar(&a.Esfuerzo, "esfuerzo", "", "esfuerzo (por defecto, el del rol)")
	return a
}

func runUp(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("up", flag.ContinueOnError)
	fs.SetOutput(stderr)
	a := flagsArranque(fs)
	if fs.Parse(args) != nil {
		return 1
	}
	p, err := abrirProyecto()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := p.Dentro(); err != nil {
		fmt.Fprintln(stderr, "orq up:", err)
		return 1
	}
	if reason, halted := state.ReadHalt(state.Dir()); halted {
		fmt.Fprintf(stderr, "orq up: HALT activo (%s). Libera espacio y `orq halt --clear`.\n", reason)
		return 2
	}
	atena, o, aviso, err := p.Subir(context.Background(), *a)
	if aviso != nil {
		fmt.Fprintln(stderr, "aviso:", aviso)
	}
	if err != nil {
		fmt.Fprintln(stderr, "orq up:", err)
		return 1
	}
	fmt.Fprintf(stdout, "atena: %s (this pane)\norquestador: %s (tab %q, pane %s)\nlayout: %s\nTalk: orq msg send --para <rol> --tipo <tipo> \"...\"\n", atena, o.Agente, p.Etiqueta("orquestador"), o.Pane, app.Layout)
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
		if err != nil || p.RolActual() == "" {
			return 0
		}
		*soloArchivo = true
	} else if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	rol := fs.Arg(0)
	if *auto {
		rol = p.RolActual()
	}
	if !slices.Contains(canal.Roles, rol) {
		fmt.Fprintln(stderr, "uso: orq relevo [--solo-archivo] atena|orquestador")
		return 1
	}
	ses, _ := canal.LoadSesiones(p.SesPath())
	// Una sola sesión por rol: si la anterior relevo ya abrió una sesión que sigue viva, no se abre otra.
	if prev, ok := ses[rol]; ok && !*soloArchivo && !*forzar && prev.Gen > 1 && time.Since(prev.Desde) < 15*time.Minute && p.Term.Dentro() {
		if p.SesionViva(context.Background(), prev) {
			fmt.Fprintf(stderr, "orq relevo: %s ya tiene una sesión nueva viva (%s, hace %s). No se abre otra; usa --forzar si de verdad hace falta\n", rol, prev.Agente, time.Since(prev.Desde).Round(time.Second))
			return 1
		}
	}
	rv := p.DatosRelevo(rol)
	var msgs strings.Builder
	ms, _ := canal.Load(p.CanalPath())
	for _, m := range ms[max(0, len(ms)-15):] {
		if m.De == rol || m.Para == rol {
			fmt.Fprintf(&msgs, "%s %s→%s [%s] %s\n", m.ID, m.De, m.Para, m.Tipo, m.Texto)
		}
	}
	est, _ := json.Marshal(rv)
	ahora, err := p.Escriba().Relevo(rol, memoria.Render(p.MemVigente(), ""), string(est), msgs.String(), lineasRelevo)
	if err != nil {
		ahora = ""
		fmt.Fprintf(stderr, "aviso: sin resumen del escriba (%v); el relevo lleva solo datos comprobados\n", err)
	}
	md := identidad.Texto(rv.Markdown(time.Now().Format("2006-01-02 15:04"), ahora))
	if err := orqdir.WriteAtomic(p.RelevoPath(rol), []byte(md)); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	orqdir.WriteAtomic(strings.TrimSuffix(p.RelevoPath(rol), ".md")+".json", est)
	fmt.Fprintf(stdout, "relevo escrito: %s (%d líneas)\n", p.RelevoPath(rol), strings.Count(md, "\n"))
	if *soloArchivo {
		return 0
	}
	if !p.Term.Dentro() {
		fmt.Fprintf(stdout, "fuera de %s: abre una sesión nueva y pídele que lea ese archivo.\n", p.Term.Nombre())
		return 0
	}
	viejo := ses[rol]
	nuevo, _, err := p.LanzarRol(context.Background(), ses, rol, *a)
	if err != nil {
		fmt.Fprintln(stderr, "orq relevo:", err)
		return 1
	}
	fmt.Fprintf(stdout, "sesión nueva: %s (panel %s). Los mensajes ya van a ella.\n", nuevo.Agente, nuevo.Pane)
	if viejo.Pane != "" {
		fmt.Fprintf(stdout, "La sesión anterior (%s, panel %s) ya puede cerrarse.\n", viejo.Agente, viejo.Pane)
	}
	return 0
}

// ---------- slice ----------

// subcomando is the first argument of `orq slice` that is not --slice NN.
func subcomando(args []string) string {
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--slice":
			i++
		case strings.HasPrefix(a, "--slice="):
		default:
			return a
		}
	}
	return ""
}

func runSlice(args []string, stdout, stderr io.Writer) int {
	if sub := subcomando(args); gate.SliceSoloOrquestador(sub) {
		// The gate matches orq by name; a copied or renamed binary, an alias or a script escapes
		// it. The binary itself refuses inside a worker, so none of them changes a slice.
		if wd, err := os.Getwd(); err != nil || gate.IsWorker(wd) {
			fmt.Fprintf(stderr, "orq: `orq slice %s` belongs to the orchestrator and does not run inside a worker clone or worktree. Send your result to the orchestrator.\n", sub)
			return 1
		}
	}
	p, err := abrirProyecto()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	r := p.Slices()
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
		fmt.Fprintln(stdout, "uso: orq slice [--slice NN] abrir|tarjeta|aprobar|tomar|alcance|verificar|juntar|podar|examen|ruta|estado|recibo|decidir|rehuella ... · guía: orq skill orquestador")
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
		fs.Bool("aislado", false, "sin efecto: la copia limpia del commit ya es el modo por defecto")
		sinConteo := fs.String("sin-conteo", "", "motivo para abrir sin --reporte/--formato (el runner no puede dejar un reporte)")
		enSitio := fs.String("en-sitio", "", "removed: the exam always runs in the box; giving it is an error")
		ruta := fs.String("ruta", "", "fix (un worker, sin equipo ni plan) | slice (por defecto)")
		timeout := fs.Int("timeout", 0, "segundos máximos del examen (0 = 600)")
		demo := fs.String("demo", "", "hasta 3 pasos para ver el resultado funcionando, separados por ;")
		tipo := fs.String("tipo", "", "tipo de tarea con que se eligen las skills del worker (por defecto, el stack)")
		historia := fs.String("historia", "", "historia de la que este slice es hijo (orq historia abrir)")
		if len(args) < 2 || fs.Parse(args[2:]) != nil || *titulo == "" || *test == "" || *cmd == "" {
			fmt.Fprintln(stderr, "uso: orq slice abrir NN --titulo \"...\" --test ruta --cmd \"...\" (--reporte f --formato tap|junit|gotest-json | --sin-conteo \"motivo\") [--bundle a,b] [--sano \"cmd\"] [--permite-omitir] [--ruta fix|slice] [--timeout s] [--demo \"paso; paso; paso\"] [--tipo T] [--historia H] [--sin-scouts \"motivo\"]")
			return 1
		}
		if *sinScouts == "" && !p.HayContexto("S-"+args[1]) {
			fmt.Fprintf(stderr, "orq slice: C·Contexto antes de A·Análisis. Guarda los criterios de los scouts con `orq mem add --fuente scout --cat tecnico --slice S-%s ...`, o abre con --sin-scouts \"motivo\".\n", args[1])
			return 1
		}
		var extra []string
		for _, f := range strings.Split(*bundle, ",") {
			if f = strings.TrimSpace(f); f != "" {
				extra = append(extra, f)
			}
		}
		e, out, err := r.Abrir(args[1], *titulo, *test, *cmd, slice.Opciones{Alcance: partir(*alcance), Bundle: extra, Sano: *sano, Reporte: *reporte, Formato: *formato, PermiteOmitir: *omitir, SinConteo: *sinConteo, EnSitio: *enSitio, Ruta: *ruta, Timeout: *timeout, Demo: pasos(*demo), Tipo: *tipo, Historia: *historia}, time.Now())
		show(out)
		if err != nil {
			fmt.Fprintln(stderr, "orq slice:", err)
			return 1
		}
		fmt.Fprintf(stdout, "%s abierto en rojo sobre %s (%s) · ruta %s. Worktree: %s\nPaquete protegido: %s\n", e.Attempt(), e.Base, slice.Corto(e.SHARojo), e.RutaEfectiva(), e.Worktree, strings.Join(e.Bundle, ", "))
		if len(e.Vigilados) > 0 {
			var rs []string
			for _, v := range e.Vigilados {
				rs = append(rs, v.Ruta)
			}
			fmt.Fprintf(stdout, "Runner vigilado: %s (si cambia, orq slice verificar pide --acepto-runner)\n", strings.Join(rs, ", "))
		}
		if e.SinConteo != "" {
			fmt.Fprintf(stdout, "Sin conteo: %s\n", e.SinConteo)
		}
		if e.EnSitio != "" {
			fmt.Fprintf(stdout, "Examen en sitio, sin copia aislada: %s\n", e.EnSitio)
		}
		fmt.Fprintln(stdout, "Asigna el escritor: orq slice tomar <quien>")
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
			fmt.Fprintln(stderr, "uso: orq slice alcance RUTA[,RUTA] --aprobado \"req ATN-... de Atena: motivo\" [--zona back]")
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
		memoria.Add(p.MemPath(), memoria.Item{Imp: "alta", Cat: "decision", Fuente: "orquestador", Slice: e.ID(),
			Texto: app.Recortar(fmt.Sprintf("%s alcance + %s (%s)", e.ID(), args[1], *aprobado), memoria.MaxTexto)}, time.Now())
		fmt.Fprintf(stdout, "%s: alcance %s\n", e.ID(), strings.Join(e.Alcance, ", "))
		return 0
	case "verificar":
		fs := flag.NewFlagSet("slice verificar", flag.ContinueOnError)
		fs.SetOutput(stderr)
		acepto := fs.String("acepto-runner", "", "motivo por el que se acepta que cambiaron los archivos del runner (queda en el recibo)")
		if fs.Parse(args[1:]) != nil {
			fmt.Fprintln(stderr, "uso: orq slice verificar [--slice NN] [--acepto-runner \"motivo\"]")
			return 1
		}
		e, out, err := r.Verificar(nn, time.Now(), slice.VerificarOpc{AceptoRunner: *acepto})
		show(out)
		var rc *slice.ErrRunnerCambio
		if errors.As(err, &rc) {
			fmt.Fprintln(stderr, "orq slice:", err)
			return 3
		}
		if err != nil {
			fmt.Fprintln(stderr, "orq slice:", err)
			return 1
		}
		fmt.Fprintf(stdout, "%s verificado: el examen pasa intacto en %s.\n", e.Attempt(), slice.Corto(e.Recibo.Head))
		if e.Recibo.Contado {
			fmt.Fprintf(stdout, "  %d ejecutadas, %d omitidas, %d fallidas\n", e.Recibo.Ejecutados, e.Recibo.Omitidos, e.Recibo.Fallidos)
		}
		if e.Recibo.Nota != "" {
			fmt.Fprintf(stdout, "  nota: %s\n", e.Recibo.Nota)
		}
		if len(e.Recibo.Checks) > 0 {
			var xs []string
			for _, c := range e.Recibo.Checks {
				xs = append(xs, c.Nombre+" "+c.Resultado)
			}
			fmt.Fprintf(stdout, "  checks: %s\n", strings.Join(xs, " · "))
		}
		fmt.Fprintln(stdout, "  recibo: orq slice recibo · falta la demo del Owner")
		if alinear(p, e.NN, stdout, stderr) == 3 {
			fmt.Fprintln(stdout, "  verde, pero desalineado: resuelve la pregunta con el Atena antes del cierre")
		}
		return 0
	case "ruta":
		if len(args) == 1 {
			e, err := r.Load(nn)
			if err != nil || e == nil {
				fmt.Fprintln(stderr, "orq slice: no hay slice abierto")
				return 1
			}
			fmt.Fprintf(stdout, "%s va por la ruta %s\n", e.ID(), e.RutaEfectiva())
			return 0
		}
		if len(args) < 3 || strings.TrimSpace(strings.Join(args[2:], " ")) == "" {
			fmt.Fprintln(stderr, "uso: orq slice ruta slice \"por qué el fix creció\"  (sube un fix a slice completo; no baja)")
			return 1
		}
		motivo := strings.Join(args[2:], " ")
		e, err := r.CambiarRuta(nn, args[1], motivo)
		if err != nil {
			fmt.Fprintln(stderr, "orq slice:", err)
			return 1
		}
		memoria.Add(p.MemPath(), memoria.Item{Imp: "alta", Cat: "decision", Fuente: "orquestador", Slice: e.ID(),
			Texto: app.Recortar(e.Attempt()+" sube de fix a slice: "+motivo, memoria.MaxTexto)}, time.Now())
		fmt.Fprintf(stdout, "%s sube a la ruta slice (%s). Sigue el flujo completo desde el Planificador: plan, Astra, orq plan cargar, equipo. El rojo y los commits se quedan\n", e.Attempt(), motivo)
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
		memoria.Add(p.MemPath(), memoria.Item{Imp: "alta", Cat: "decision", Fuente: "orquestador", Slice: e.ID(),
			Texto: app.Recortar(e.Attempt()+" huella reparada: "+motivo, memoria.MaxTexto)}, time.Now())
		fmt.Fprintf(stdout, "%s: huella tomada del rojo (%s), el examen es idéntico. Vuelve a verificar\n", e.Attempt(), slice.Corto(e.SHARojo))
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
		memoria.Add(p.MemPath(), memoria.Item{Imp: "alta", Cat: "decision", Fuente: "orquestador", Slice: e.ID(),
			Texto: app.Recortar(e.Attempt()+" reabierto: "+motivo, memoria.MaxTexto)}, time.Now())
		fmt.Fprintf(stdout, "%s reabierto en el mismo worktree, con su trabajo. El recibo ya no vale: lanza la reparación y vuelve a verificar\n", e.Attempt())
		return 0
	case "reintentar":
		owner, causa := false, ""
		var resto []string
		for i := 1; i < len(args); i++ {
			switch {
			case args[i] == "--decision-owner":
				owner = true
			case args[i] == "--causa" && i+1 < len(args):
				causa = args[i+1]
				i++
			default:
				resto = append(resto, args[i])
			}
		}
		if len(resto) == 0 {
			fmt.Fprintln(stderr, "uso: orq slice reintentar \"qué cambia en este intento\" --causa worker|examen [--decision-owner]")
			return 1
		}
		args = append(args[:1], resto...)
		e, err := r.Reintentar(nn, strings.Join(resto, " "), causa, owner)
		if err != nil {
			fmt.Fprintln(stderr, "orq slice:", err)
			return 1
		}
		memoria.Add(p.MemPath(), memoria.Item{Imp: "alta", Cat: "decision", Fuente: "orquestador", Slice: e.ID(),
			Texto: app.Recortar(e.Attempt()+": "+strings.Join(args[1:], " "), memoria.MaxTexto)}, time.Now())
		p.AprenderSolo(strings.Join(resto, " "), "reintento", e.Attempt())
		fmt.Fprintf(stdout, "%s abierto en el mismo worktree. Recibos anteriores ya no valen. Asigna escritor: orq slice tomar <quien>\n", e.Attempt())
		return 0
	case "cerrar":
		fs := flag.NewFlagSet("slice cerrar", flag.ContinueOnError)
		fs.SetOutput(stderr)
		ok := fs.Bool("aceptado", false, "el Owner aceptó la demo")
		motivo := fs.String("rechazado", "", "motivo del rechazo en una línea")
		aceptoChecks := fs.Bool("acepto-checks", false, "el Owner leyó el diff de los archivos que definen los checks y lo acepta")
		if fs.Parse(args[1:]) != nil || *ok == (*motivo != "") || *aceptoChecks && !*ok {
			fmt.Fprintln(stderr, "uso: orq slice cerrar --aceptado [--acepto-checks] | --rechazado \"motivo\"")
			return 1
		}
		reg, cuarentena, out, err := r.Cerrar(nn, *ok, *motivo, time.Now(), slice.CerrarOpc{AceptoChecks: *aceptoChecks})
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
			Texto: app.Recortar(fmt.Sprintf("S-%s %s: %s", reg.NN, reg.Result, reg.Titulo), memoria.MaxTexto)}
		if !*ok {
			mem.Imp, mem.Cat, mem.Fuente = "critica", "requisito", "owner"
			mem.Texto = app.Recortar(fmt.Sprintf("Rechazo de S-%s: %s", reg.NN, *motivo), memoria.MaxTexto)
			p.AprenderSolo("El Owner rechazó: "+*motivo, "rechazo", "S-"+reg.NN)
		}
		memoria.Add(p.MemPath(), mem, time.Now())
		os.Remove(p.CanalPath())
		os.Remove(p.PlanPath(reg.NN))
		// Higiene: los paneles de los workers de este slice ya no tienen trabajo.
		if ses, err := canal.LoadSesiones(p.SesPath()); err == nil {
			if c, _ := p.CerrarSesiones(context.Background(), ses, func(k string) bool { return strings.HasPrefix(k, "worker-") && strings.HasSuffix(k, "-s"+reg.NN) }); len(c) > 0 {
				fmt.Fprintf(stdout, "paneles cerrados: %s\n", strings.Join(c, ", "))
			}
		}
		if os.Getenv("ORQ_SIN_DOC") != "" {
			fmt.Fprintln(stdout, "project card not updated (ORQ_SIN_DOC set)")
		} else if err := p.EscribirFicha(); err != nil {
			fmt.Fprintf(stdout, "aviso: la ficha del proyecto no se actualizó (%v)\n", err)
		} else {
			fmt.Fprintf(stdout, "ficha del proyecto actualizada: %s\n", p.FichaPath())
		}
		fmt.Fprintf(stdout, "S-%s cerrado como %s. Canal limpio; memoria actualizada.\n", reg.NN, reg.Result)
		if !*ok {
			fmt.Fprintf(stdout, "La rama %s se conserva para revisar.\n", "slice/"+reg.NN)
		}
		return 0
	case "lanzar":
		fs := flag.NewFlagSet("slice lanzar", flag.ContinueOnError)
		fs.SetOutput(stderr)
		agente := fs.String("agente", "claude", "claude | codex")
		zona := fs.String("zona", "", "front, back, test…: suma el worker al equipo del slice")
		rutas := fs.String("rutas", "", "carpetas de esa zona, separadas por coma")
		alcance := fs.String("alcance", "solo lo necesario para que el examen pase", "qué sí y qué no")
		suite := fs.String("suite", "", "comando de la suite rápida")
		pres := fs.String("presupuesto", "45 min", "tiempo o tokens")
		dif := fs.String("dificultad", "", "baja|media|alta: elige modelo y esfuerzo según `orq config ver`")
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
		nombre, ya, err := p.LanzarWorker(context.Background(), app.PedidoWorker{Slice: nn, Agente: *agente, Zona: *zona, Rutas: rs,
			Alcance: *alcance, Suite: *suite, Presupuesto: *pres, Dificultad: *dif, Tarea: *tarea})
		if err != nil {
			fmt.Fprintln(stderr, "orq slice:", err)
			if errors.Is(err, terminal.ErrDesconocido) {
				return 4 // desconocido: mirar el panel y repetir el mismo comando; no lanza dos veces
			}
			return 1
		}
		if ya {
			fmt.Fprintf(stdout, "%s ya estaba trabajando con su contrato; no se lanzó otro.\n", nombre)
			return 0
		}
		fmt.Fprintf(stdout, "%s trabajando en su worktree con el contrato. Mira el avance con `orq slice estado`.\n", nombre)
		return 0
	case "examen":
		fs := flag.NewFlagSet("slice examen", flag.ContinueOnError)
		fs.SetOutput(stderr)
		cmd := fs.String("cmd", "", "comando nuevo del examen si debe correr también los archivos sumados")
		aprobado := fs.String("aprobado", "", "req de Atena u Owner que aprueba el examen nuevo")
		if len(args) < 2 || fs.Parse(args[2:]) != nil || *aprobado == "" {
			fmt.Fprintln(stderr, "uso: orq slice examen [--slice NN] ARCHIVO[,ARCHIVO] --aprobado \"req ATN-...\" [--cmd \"...\"]")
			return 1
		}
		e, err := r.SumarExamen(nn, partir(args[1]), *cmd, *aprobado)
		if err != nil {
			fmt.Fprintln(stderr, "orq slice:", err)
			return 1
		}
		fmt.Fprintf(stdout, "%s: examen protegido %s (base %s)\n  cmd: %s\n", e.ID(), strings.Join(e.Bundle, ", "), slice.Corto(e.SHAExamen), e.Cmd)
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
		alinear(p, e.NN, stdout, stderr)
		if pl, err := plan.Leer(p.PlanPath(e.NN)); err == nil {
			listas, _ := plan.Listas(pl)
			fmt.Fprintf(stdout, "plan: %d pendientes; marca lo terminado con `orq plan hecha ID`; listas: %s\n", pl.Pendientes(), ids(listas))
			// El Planificador vigila su plan: cada junta le pide revisarlo.
			if ses, _ := canal.LoadSesiones(p.SesPath()); ses["planificador"].Agente != "" && p.Term.Dentro() {
				p.Term.Avisar(context.Background(), ses["planificador"].Agente, fmt.Sprintf("[orq] S-%s merged. Run `orq plan revisar --slice %s` and keep the plan minimal and current.", e.NN, e.NN))
			}
		}
		// Documentación barata en paralelo: el escriba actualiza la ficha sin frenar al equipo.
		if self, err := os.Executable(); err == nil && os.Getenv("ORQ_SIN_DOC") == "" {
			c := exec.Command(self, "doc")
			c.Dir = p.Root
			if c.Start() == nil {
				go c.Wait()
				fmt.Fprintln(stdout, "la ficha del proyecto se actualiza en segundo plano (escriba): .orq/PROYECTO.md")
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
		c, err := p.Contrato(nn, *worker, *alcance, *suite, *pres)
		if err != nil {
			fmt.Fprintln(stderr, "orq slice:", err)
			return 1
		}
		fmt.Fprint(stdout, c)
		return 0
	case "decidir":
		return runDecidir(p, nn, args[1:], stdout, stderr)
	case "tarjeta":
		return runTarjeta(p, nn, args[1:], stdout, stderr)
	case "aprobar":
		return runAprobar(p, nn, args[1:], stdout, stderr)
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
			fmt.Fprintf(stdout, "%s «%s» · %s\n  paquete: %s (%s)\n  base: %s@%s  worktree: %s (%s)\n  escritor: %s  fallos: %d/%d\n", e.Attempt(), e.Titulo, e.Fase, strings.Join(e.Bundle, ", "), e.Cmd, e.Base, slice.Corto(e.SHARojo), e.Worktree, e.Procedencia, app.Valor(e.Escritor), e.Fallos, slice.MaxFallos)
			fmt.Fprintf(stdout, "  ruta: %s", e.RutaEfectiva())
			if e.Ascendido {
				fmt.Fprintf(stdout, " (subió de fix: %s)", e.RutaMotivo)
			}
			fmt.Fprintln(stdout)
			if e.SinConteo != "" {
				fmt.Fprintf(stdout, "  sin conteo: %s\n", e.SinConteo)
			}
			if e.EnSitio != "" {
				fmt.Fprintf(stdout, "  examen en sitio: %s\n", e.EnSitio)
			}
			if e.Motivo != "" {
				fmt.Fprintf(stdout, "  bloqueado: %s\n", e.Motivo)
			}
			if e.Recibo != nil {
				fmt.Fprintf(stdout, "  recibo: %s exit %d en %s\n", e.Recibo.Attempt, e.Recibo.Exit, slice.Corto(e.Recibo.Head))
			}
			for _, w := range e.Equipo {
				j := "sin juntar"
				if w.Juntado != "" {
					j = "juntado " + slice.Corto(w.Juntado)
				}
				fmt.Fprintf(stdout, "  equipo: %-22s %-6s %-6s %s · %s\n", w.Nombre, w.Agente, w.Zona, strings.Join(w.Rutas, ","), j)
			}
		}
		if h, err := r.Huerfanos(); err == nil && len(h) > 0 {
			fmt.Fprintf(stdout, "worktrees sobrantes (%d): revísalos y quítalos con git worktree remove:\n  %s\n", len(h), strings.Join(h, "\n  "))
		}
		return 0
	}
	fmt.Fprint(stderr, uso())
	return 1
}

// pasos parte una demo «a; b; c» en hasta 3 pasos.
func pasos(s string) []string {
	var out []string
	for _, x := range strings.Split(s, ";") {
		if x = strings.TrimSpace(x); x != "" && len(out) < 3 {
			out = append(out, x)
		}
	}
	return out
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
