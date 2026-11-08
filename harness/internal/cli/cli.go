// Package cli enruta los subcomandos de orq.
package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"orq/internal/slice"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"orq/internal/gate"
	"orq/internal/limits"
	"orq/internal/state"
	"orq/internal/sysinfo"
	"orq/internal/watch"
	"orq/internal/worktrees"
)

const Version = "1.0.0"

const usage = `orq ` + Version + ` · harness de orquestador-2

  orq gate            hook PreToolUse: lee el JSON por stdin; exit 2 = bloquea
  orq watch           mide scratch y disco; escribe HALT si corresponde
  orq gc [--dry-run]  retira worktrees de orq sin perder nada (nunca --force); lo dudoso a cuarentena
  orq status          muestra límites, medición, HALT y workers
  orq halt            muestra HALT
  orq halt --clear    quita HALT (solo tras liberar espacio)
  orq halt --set MSG  pone HALT a mano

  orq init [--modo=mejora|nuevo] detecta modo y stack, instala hooks y lista skills
  orq profile [set type=greenfield|brownfield|legacy criticality=trivial|low|medium|high stage=development|production stacks=a,b]
                                 multi-signal detection (code, git age/commits/authors, tests, CI, manifests);
                                 Owner-forced values persist in .orq/profile.json and win; contradictions warn
  orq skills [plan --phase X --role R | install [ID...] | vet ID [--file F] | fijar | verificar]
                                 curated gallery (skills.catalog.json + .orq/skills.catalog.json): minimal set per
                                 phase×role by stakes; install pins hashes in .orq/skills.lock.json and refuses
                                 the never-list; vet records validated|rejected in .orq/skills.gallery.json
  orq skill ROL                  guía del rol (atena|orquestador|scout|worker|verificador) y sus skills de fase×rol
  orq fase                       T·Traducción, C·Contexto, A·Análisis, D1·Desarrollo, D2·Documentación
  orq contexto                   hook SessionStart: rol, fase, relevo y memoria
  orq doc                        ficha viva del proyecto (.orq/PROYECTO.md) con un modelo barato
  orq doctor                     worktrees, ramas, sesiones y memoria sobrantes
  orq up                         desde el panel del Atena (herdr): crea la sesión del Orquestador
  orq msg send --para ROL --tipo T "texto"   historia|pregunta|respuesta|estado|cierre
  orq msg send ... [--intento S-NN/aK] [--ref ID] [--clave K]   un cierre exige el intento vigente
  orq msg reconciliar [--reenviar ID]   mensajes sin entrega confirmada
  orq msg leer [-n 10]
  orq mem add --imp I --cat C [--fuente F] [--slice S-NN] "texto ≤200"
  orq mem ver [--cat C]          la memoria etiquetada, lo crítico primero
  orq mem compactar              Haiku resume lo no crítico; se valida antes de guardar
  orq relevo [--solo-archivo] ROL   escribe .orq/relevo-ROL.md y abre una sesión fresca
  orq slice abrir NN --titulo T --test RUTA --cmd "CMD" [--bundle a,b] [--sano "CMD"] [--alcance rutas] [--sin-scouts "motivo"]
         [--reporte F --formato tap|junit|gotest-json] [--permite-omitir] [--aislado] [--timeout S]
                                 descubre, exige base sana y rojo por aserción; crea el worktree
  orq slice tomar QUIEN [--relevo]   un solo escritor por slice
  orq slice reabrir "motivo"   un slice verificado vuelve a construirse para una reparación, con su trabajo
  orq slice rehuella "motivo"   BLOQUEADO por huella mal tomada: si el examen es idéntico al del rojo, la toma del commit
  orq slice alcance RUTA[,RUTA] --aprobado "req…"   amplía el alcance con aprobación de Atena
  orq slice verificar            corre el examen intacto, cuenta pruebas y deja un recibo
                                 (2 reparaciones o sin progreso → BLOQUEADO)
  orq slice lanzar [--slice NN] [--agente A] [--tarea ID|siguiente] [--dificultad baja|media|alta]
                                 [--zona front --rutas web,ui] [--alcance "..."]
                                 abre el worker en su panel (runner: herdr|tmux|process) con su contrato. Con --zona se suma
                                 al EQUIPO del slice (front, back, test… a la vez, hasta 4), cada uno en su
                                 rama y worktree; se hablan con orq msg send --para worker-<zona>-sNN
  orq slice juntar [--slice NN]  integra lo commiteado de cada worker del equipo; rechaza lo que salga
                                 de su zona o toque el examen; luego orq slice verificar
  orq slice recibo               el recibo vigente
  orq slice contrato [--worker N] [--alcance "..."] [--suite "cmd"] [--presupuesto "..."]
                                 el contrato del worker con los datos reales del slice
  orq sesion ver | abrir planificador|astra | cerrar NOMBRE
                                 paneles con nombre: "orq · P", "plan · P" (planificador | astra),
                                 "S-NN · P" (workers). Al cerrar un slice se cierran sus paneles
  orq plan [--slice NN] cargar ARCHIVO.json | ver | hecha ID | debatido
                                 grafo de tareas del slice: {id, zona, rutas, depende_de, dificultad, que}.
                                 Valida ids, dependencias, ciclos y alcance; una tarea está lista cuando
                                 sus dependencias están hechas y nadie en curso toca sus rutas
  orq aprender [--global] ver|add "lección"|confirmar ID|refutar ID|promover ID
                                 lecciones con confianza; se aprenden solas (reintentos, rechazos,
                                 desvíos) y las ≥0.7 entran al arrancar cada sesión (máx. 6)
  orq alinear [--slice NN]       ¿lo cambiado sigue sirviendo a la historia? exit 3 = pregunta al Atena
  orq slice reintentar "qué cambia" [--decision-owner]   intento nuevo desde BLOQUEADO (máx. 3)
  orq slice cerrar --aceptado | --rechazado "motivo"   integra aparte, revalida, publica y limpia;
                                 si se interrumpe, repetirlo continúa donde quedó
  orq slice estado               fase, intento, escritor, recibo y sobrantes
                                 Con varios slices abiertos (hasta 4), cada comando lleva --slice NN
  orq metricas [--desde AAAA-MM-DD] [--json]   aceptados, intentos, sin progreso, cuarentena
  orq config agents [show | detect | set ROL AGENTE:MODELO]
                                 agentes y modelos por rol desde datos (orq.agents.json sobre
                                 ~/.config/orq/agents.json). detect busca claude, codex, cursor-agent,
                                 gemini, opencode y aider en el PATH y escribe una propuesta; el Owner gana.
                                 El adversario es de otra familia que el planificador (lo impone orq)
  orq worker beat|done|list      estado vivo del worker (.orq/workers/<id>/state.json, ORQ_WORKER_ID)
  orq stop raise|list|resolve|cancel   pedidos al Owner (credential|api_key|decision|confirmation|access,
                                 blocking|soon|fyi); solo atena/owner resuelven; nunca un secreto en texto
  orq autonomy [ver|guided|free --scope ...]   autonomía; free lo aprueba el Owner
  orq board [--json|--html [--out F]]   tablero del Owner: stoppers, slices, workers, velocidad
  orq handoff [--out F]          HANDOFF.md para que otra persona retome el proyecto
  orq sync push|pull [--theirs]  memoria en la rama orq/state (orq init --sync=local|commit|branch)
  orq qa run S-NN                pirámide de pruebas (unit → integration → e2e; mutation/adversarial solo si
                                 criticidad ≥ high) y recibo QA. Solo el Orquestador. Cerrar aceptado lo exige
                                 desde criticidad medium (qa.require_receipt_from)
  orq qa finding add S-NN --requirement R --commit SHA --repro CMD --observed O --expected E --impact I
                                 confirmado solo si orq re-ejecuta el repro y falla · orq qa ver S-NN
  orq evidence add S-NN --kind target|before|after --file PNG [--note "..."] [--pulir]
                                 docs/slices/S-NN/: PNG, diff % after vs target y README ≤40 líneas
  orq version
`

// Run devuelve el código de salida. En `gate`, cualquier pánico termina en 2 (bloquear).
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) (code int) {
	slice.Version = "orq " + Version
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 1
	}
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(stderr, "orq: fallo interno (%v); por seguridad se bloquea.\n", r)
			code = 2
		}
	}()
	switch args[0] {
	case "gate":
		return runGate(stdin, stderr)
	case "watch":
		return runWatch(stdout, stderr)
	case "gc":
		return runGC(args[1:], stdout, stderr)
	case "status":
		return runStatus(stdout, stderr)
	case "halt":
		return runHalt(args[1:], stdout, stderr)
	case "init":
		return runInit(args[1:], stdout, stderr)
	case "profile":
		return runProfile(args[1:], stdout, stderr)
	case "skills":
		return runSkills(args[1:], stdout, stderr)
	case "fase":
		return runFase(stdout, stderr)
	case "contexto":
		return runContexto(stdout, stderr)
	case "doc":
		return runDoc(stdout, stderr)
	case "doctor":
		return runDoctor(stdout, stderr)
	case "up":
		return runUp(args[1:], stdout, stderr)
	case "msg":
		return runMsg(args[1:], stdin, stdout, stderr)
	case "mem":
		return runMem(args[1:], stdin, stdout, stderr)
	case "relevo":
		return runRelevo(args[1:], stdout, stderr)
	case "slice":
		return runSlice(args[1:], stdout, stderr)
	case "sesion":
		return runSesion(args[1:], stdout, stderr)
	case "plan":
		return runPlan(args[1:], stdout, stderr)
	case "aprender":
		return runAprender(args[1:], stdout, stderr)
	case "alinear":
		return runAlinear(args[1:], stdout, stderr)
	case "worker":
		return runWorker(args[1:], stdout, stderr)
	case "stop":
		return runStop(args[1:], stdout, stderr)
	case "autonomy":
		return runAutonomy(args[1:], stdout, stderr)
	case "board":
		return runBoard(args[1:], stdout, stderr)
	case "revive":
		return runRevive(args[1:], stdout, stderr)
	case "handoff":
		return runHandoff(args[1:], stdout, stderr)
	case "sync":
		return runSync(args[1:], stdout, stderr)
	case "metricas":
		return runMetricas(args[1:], stdout, stderr)
	case "skill", "--skill":
		return runSkill(args[1:], stdout, stderr)
	case "config":
		return runConfig(args[1:], stdout, stderr)
	case "qa":
		return runQA(args[1:], stdout, stderr)
	case "evidence":
		return runEvidence(args[1:], stdout, stderr)
	case "version":
		fmt.Fprintln(stdout, Version)
		return 0
	default:
		fmt.Fprint(stderr, usage)
		return 1
	}
}

func realEnv() gate.Env {
	return gate.Env{
		DiskFreeGB: sysinfo.FreeGB,
		Now:        time.Now,
		StateDir:   state.Dir(),
		IsWorktree: gate.IsLinkedWorktree,
		Bundle:     bundleAbierto,
		Worktrees:  worktreesDe,
		Rol:        os.Getenv("ORQ_ROL"),
	}
}

// bundleAbierto lee el paquete del examen del slice abierto del repo de cwd.
func bundleAbierto(cwd string) []string {
	out, err := exec.Command("git", "-C", cwd, "rev-parse", "--path-format=absolute", "--git-common-dir").Output()
	if err != nil {
		return nil
	}
	root := filepath.Dir(strings.TrimSpace(string(out)))
	ab, _ := slice.Repo{Root: root, OrqDir: filepath.Join(root, ".orq")}.Abiertos()
	var bundle []string
	for _, e := range ab {
		if cwd == e.Worktree || strings.HasPrefix(cwd, e.Worktree+"/") || len(ab) == 1 {
			bundle = append(bundle, e.Bundle...)
		}
	}
	return bundle
}

func runGate(stdin io.Reader, stderr io.Writer) int {
	var p gate.Payload
	if err := json.NewDecoder(stdin).Decode(&p); err != nil {
		fmt.Fprintf(stderr, "orq: payload ilegible (%v); por seguridad se bloquea.\n", err)
		return 2
	}
	l, err := limits.Load(limits.DefaultPath())
	if err != nil {
		// Sin límites válidos solo se bloquean lanzamientos; el resto de herramientas sigue.
		if p.ToolName == "Agent" || p.ToolName == "Task" || p.ToolName == "Bash" {
			fmt.Fprintf(stderr, "orq: %v. Crea el archivo (ver templates/limits.json) para poder lanzar workers.\n", err)
			return 2
		}
		// Aun sin límites, el examen y el paquete del slice siguen protegidos.
		if d := gate.Evaluate(p, limits.Limits{Protected: []string{"tests/aceptacion/"}}, realEnv()); !d.Allow {
			fmt.Fprintln(stderr, d.Reason)
			return 2
		}
		return 0
	}
	env := realEnv()
	env.Workers = func() (int, error) {
		procs, err := sysinfo.ListProcs()
		return len(sysinfo.Workers(procs, l.WorkerProcessName, gate.IsLinkedWorktree)), err
	}
	d := gate.Evaluate(p, l, env)
	if d.Allow {
		return 0
	}
	fmt.Fprintln(stderr, d.Reason)
	return 2
}

func runWatch(stdout, stderr io.Writer) int {
	// El vigía de workers corre aunque falten los límites de disco.
	if p, err := abrirProyecto(); err == nil {
		p.vigilarWorkers(stdout)
	}
	l, err := limits.Load(limits.DefaultPath())
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	r, err := watch.Run(l, state.Dir(), time.Now())
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "scratch %.1f GB (cuota %.0f GB)\n", r.Snapshot.ScratchGB, *l.Scratch.QuotaGB)
	for p, gb := range r.Snapshot.PerPathGB {
		fmt.Fprintf(stdout, "  %-50s %.1f GB\n", p, gb)
	}
	for p, gb := range r.Snapshot.DiskFreeGB {
		fmt.Fprintf(stdout, "libre %-6s %.0f GB (mínimo %.0f, crítico %.0f)\n", p, gb, l.MinFreeGB[p], *l.Scratch.CriticalFreeGB)
	}
	for _, a := range r.Avisos {
		fmt.Fprintf(stdout, "AVISO %s\n", a)
	}
	// Dentro de un proyecto con .orq, watch también retira lo seguro y cuenta worktrees.
	if p, err := abrirProyecto(); err == nil {
		if res, err := p.gc(false); err != nil {
			fmt.Fprintf(stdout, "gc: %v\n", err)
		} else if len(res) > 0 {
			mostrarGC(stdout, res)
		}
		n, _ := worktrees.Contar(p.dir)
		fmt.Fprintf(stdout, "worktrees de orq: %d (tope max_worktrees %d)\n", n, l.TopeWorktrees())
	}
	if r.Halt != "" {
		fmt.Fprintf(stdout, "HALT escrito: %s\n", r.Halt)
		return 3
	}
	return 0
}

func runStatus(stdout, stderr io.Writer) int {
	l, err := limits.Load(limits.DefaultPath())
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "límites: %s\n", limits.DefaultPath())
	fmt.Fprintf(stdout, "  max_workers=%d  quota_scratch=%.0f GB  protegidos=%s\n", *l.MaxWorkers, *l.Scratch.QuotaGB, strings.Join(l.Protected, ", "))
	for p, min := range l.MinFreeGB {
		free, _ := sysinfo.FreeGB(p)
		fmt.Fprintf(stdout, "  disco %-6s libre %.0f GB (mínimo %.0f)\n", p, free, min)
	}
	if snap, err := state.LoadSnapshot(state.Dir()); err == nil {
		fmt.Fprintf(stdout, "medición: hace %.1f h, scratch %.1f GB\n", time.Since(snap.TS).Hours(), snap.ScratchGB)
	} else {
		fmt.Fprintln(stdout, "medición: ninguna (ejecuta `orq watch`)")
	}
	if reason, ok := state.ReadHalt(state.Dir()); ok {
		fmt.Fprintf(stdout, "HALT: %s\n", reason)
	} else {
		fmt.Fprintln(stdout, "HALT: no")
	}
	procs, err := sysinfo.ListProcs()
	if err != nil {
		fmt.Fprintf(stdout, "workers: no se pudo leer /proc (%v)\n", err)
		return 0
	}
	ws := sysinfo.Workers(procs, l.WorkerProcessName, gate.IsLinkedWorktree)
	fmt.Fprintf(stdout, "workers (agentes en worktrees): %d de %d\n", len(ws), *l.MaxWorkers)
	for _, w := range ws {
		fmt.Fprintf(stdout, "  %-7d %-7s %s\n", w.PID, w.Comm, w.Cwd)
	}
	return 0
}

func runHalt(args []string, stdout, stderr io.Writer) int {
	dir := state.Dir()
	switch {
	case len(args) == 0:
		if reason, ok := state.ReadHalt(dir); ok {
			fmt.Fprintf(stdout, "HALT: %s\n", reason)
			return 3
		}
		fmt.Fprintln(stdout, "HALT: no")
		return 0
	case args[0] == "--clear":
		reason, ok := state.ReadHalt(dir)
		if err := state.ClearHalt(dir); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if ok {
			fmt.Fprintf(stdout, "HALT quitado (era: %s)\n", reason)
		}
		return 0
	case args[0] == "--set" && len(args) > 1:
		if err := state.WriteHalt(dir, strings.Join(args[1:], " "), time.Now()); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	}
	fmt.Fprint(stderr, usage)
	return 1
}
