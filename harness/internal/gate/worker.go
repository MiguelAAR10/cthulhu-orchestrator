package gate

import (
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"orq/internal/lang"
)

// rolesDeSesion: paneles que orq abre en el checkout principal y no son workers.
// Cualquier otro ORQ_ROL no vacío (w1, tester, repairer...) es un worker, esté donde esté su cwd.
var rolesDeSesion = map[string]bool{"": true, "owner": true, "atena": true, "orquestador": true, "orchestrator": true,
	"planificador": true, "planner": true, "astra": true}

// esWorker: subagente, cwd en un worktree enlazado, ORQ_WORKER_ID del agente o un rol de worker.
// ORQ_WORKER_ID y ORQ_ROL salen del entorno del agente (el que orq le dio al lanzarlo), no del comando.
func esWorker(p Payload, env Env) bool {
	return p.AgentID != "" || env.IsWorktree(p.Cwd) || env.WorkerID != "" || !rolesDeSesion[env.Rol]
}

// orqWorker: lo único que un worker pide a orq. Clave vacía = sin subcomando; "*" = cualquiera.
var orqWorker = map[string]map[string]bool{
	"slice":    set("", "verificar", "verify", "estado", "status", "contrato", "contract", "recibo", "receipt", "hecho", "fact", "hipotesis", "hypothesis"),
	"msg":      set("send", "leer", "read"),
	"mem":      set("", "add", "ver", "show"),
	"worker":   set("beat", "done", "list"),
	"stop":     set("raise", "list"),
	"qa":       set("finding", "ver", "show"),
	"aprender": set("", "ver", "show", "add", "confirmar", "confirm", "refutar", "refute"),
	"learn":    set("", "ver", "show", "add", "confirmar", "confirm", "refutar", "refute"),
	"plan":     set("", "ver", "show", "listas", "ready"),
	"skill":    set("*"), "--skill": set("*"), "contexto": set("*"), "context": set("*"), "fase": set("*"), "phase": set("*"),
	"status": set("*"), "board": set("*"), "version": set("*"), "help": set("*"), "-h": set("*"), "--help": set("*"),
	"doc": set("*"), "gate": set("*"),
}

func set(xs ...string) map[string]bool {
	m := map[string]bool{}
	for _, x := range xs {
		m[x] = true
	}
	return m
}

// OrqPermitidoAWorker decide si un worker puede correr `orq <args>`. Lista blanca: lo que no está, no pasa.
// Lo usan el gate (Bash de Claude) y el shim de orq que reciben los workers sin hook (Codex).
func OrqPermitidoAWorker(args []string) bool {
	if len(args) == 0 {
		return true
	}
	cmd := args[0]
	sub := ""
	for i := 1; i < len(args); i++ {
		a := args[i]
		if a == "--slice" || a == "--repo" {
			i++
			continue
		}
		if strings.HasPrefix(a, "-") {
			if cmd == "relevo" && a == "--auto" {
				return true // hook PreCompact: escribe el relevo del propio rol
			}
			continue
		}
		sub = a
		break
	}
	subs, ok := orqWorker[cmd]
	if !ok || !(subs["*"] || subs[sub]) {
		return false
	}
	for i := 1; i < len(args); i++ {
		k, v, conIgual := strings.Cut(normalFlag(args[i]), "=")
		if !conIgual && i+1 < len(args) {
			v = args[i+1]
		}
		switch k {
		case "--fuente", "--source":
			if v == "owner" || v == "atena" {
				return false
			}
		case "--de", "--from": // el remitente de un worker es su propio rol, nunca otro
			return false
		case "--tipo", "--type":
			if cmd == "msg" && !tiposDeWorker[v] {
				return false
			}
		}
	}
	return true
}

// tiposDeWorker: lo único que un worker manda por el canal. cierre, historia y plan son del
// Orquestador, de Atena o del Planificador.
var tiposDeWorker = set("pregunta", "question", "respuesta", "answer", "estado", "status")

// normalFlag: Go acepta -de igual que --de; la lista blanca compara en una sola forma.
func normalFlag(a string) string {
	if len(a) > 2 && a[0] == '-' && a[1] != '-' {
		return "-" + a
	}
	return a
}

var (
	segOrqRe = regexp.MustCompile(`\|\||&&|[;|&\n()` + "`" + `]`)
	// trampas que un worker no necesita: cambiar su identidad, alias, copiar orq, leer el token.
	trampaWorkerRe = []*regexp.Regexp{
		regexp.MustCompile(`\bORQ_\w*=`),
		regexp.MustCompile(`\bunset\b[^;&|]*\bORQ_`),
		regexp.MustCompile(`\benv\b[^;&|]*\s(-\w*[iu]\b|--ignore-environment|--unset)`),
		regexp.MustCompile(`(^|[\s;&|(])alias\b`),
		regexp.MustCompile(`\bgit\b[^;&|]*(\s-c\s*alias\.|\sconfig\b[^;&|]*\balias\.)`),
		regexp.MustCompile(`(^|[\s;&|(])(ln|cp|install|mv|rsync|tee|cat\s*>)\b[^;&|]*(\borq\b|\$\()`),
		regexp.MustCompile(`owner\.token|\.config/orq|XDG_CONFIG_HOME|/environ\b`),
		// globs sobre la config o una carpeta oculta del home: ~/.config/*/own?r.tok*, ~/.c?nfig
		regexp.MustCompile(`\.config\S*[*?\[]|(~|\$HOME|\$\{HOME\}|/home/[^/\s]+)/\.[^\s/]*[*?\[]`),
		// ${!ORQ@}, ${!ORQ*}, ${ORQ...}: enumerar o expandir la identidad
		regexp.MustCompile(`[!{]ORQ`),
		// buscar tokens por patrón: *.token, own?r.tok*, find -name '*token*'
		regexp.MustCompile(`(?i)\S*[*?\[]\S*tok|\btok\S*[*?\[]`),
		// unset/export que nombran ORQ de forma indirecta: $(printf "ORQ_%s=..."), $(compgen | grep ORQ)
		regexp.MustCompile(`\b(unset|export|declare|typeset|readonly)\b[^;&]*ORQ`),
	}
)

// workerOrq: para un worker, cada invocación de orq (por nombre o ruta) debe estar en la lista blanca,
// sin expansiones de shell en sus argumentos. Las comillas y barras invertidas se quitan sin dejar
// espacio, como hace el shell: sl”ice es slice.
func workerOrq(cmd string) (Decision, bool) {
	c := strings.NewReplacer(`'`, "", `"`, "", `\`, "").Replace(cmd)
	no := func() (Decision, bool) {
		return deny("%s", lang.T("orq: a worker only runs read-only or worker orq commands (slice status|verify|receipt|contract, msg send|read, mem add, worker beat|done, stop raise|list, qa finding). It does not change its identity (ORQ_*), alias or copy orq, or read the Owner's token.",
			"orq: un worker solo corre comandos de orq de lectura o de worker (slice estado|verificar|recibo|contrato, msg send|leer, mem add, worker beat|done, stop raise|list, qa finding). No cambia su identidad (ORQ_*), ni hace alias o copias de orq, ni lee el token del Owner.")), true
	}
	for _, re := range trampaWorkerRe {
		if re.MatchString(c) {
			return no()
		}
	}
	if d := dirConfigOrq(); d != "" && strings.Contains(c, d) {
		return no()
	}
	if escapaDelArbol(c) || interpreteEscapa(c) {
		return deny("%s", lang.T("orq: a worker does not detach from its process tree (setsid, nohup, disown, systemd-run, at, tmux new, herdr): orq recognises workers by their ancestors.",
			"orq: un worker no se separa de su árbol de procesos (setsid, nohup, disown, systemd-run, at, tmux new, herdr): orq reconoce a los workers por sus ancestros.")), true
	}
	for _, seg := range segOrqRe.Split(c, -1) {
		f := strings.Fields(seg)
		for i, tok := range f {
			if path.Base(tok) != "orq" {
				continue
			}
			if strings.Contains(seg, "$") || !OrqPermitidoAWorker(f[i+1:]) {
				return no()
			}
			break
		}
	}
	return Decision{}, false
}

// sueltaArbol: comandos que lanzan un proceso fuera del árbol del worker (otro padre, otra sesión).
// script/unbuffer/expect/socat abren un pseudo-terminal nuevo: orq acepta una terminal de persona
// como prueba del Owner, así que un worker tampoco se fabrica una.
var sueltaArbol = set("setsid", "nohup", "disown", "systemd-run", "at", "batch", "daemonize", "start-stop-daemon", "screen", "herdr", "orca", "orca-ide",
	"script", "unbuffer", "expect", "socat")

var (
	// interpCodigoRe: un intérprete con código en línea (-c/-e/-r/--eval) o leído de stdin/heredoc.
	interpCodigoRe = regexp.MustCompile(`(^|[\s;&|(/])(python[0-9.]*|perl|node|nodejs|ruby|php|deno|bun|lua|tclsh)\b[^;&|]*(\s-\w*[cer]\b|\s--eval\b|\s-\s|\s-$|<<|\seval\b)`)
	// codigoEscapaRe: lo que ese código no necesita en un worker: separarse del árbol, fabricar una
	// terminal o llamar a orq por debajo del gate.
	codigoEscapaRe = regexp.MustCompile(`setsid|\bfork|daemon|start_new_session|detached|\bpty\b|openpty|forkpty|nohup|disown|` +
		`(os\.system|subprocess|popen|Popen|child_process|exec\w*|spawn\w*|system|\x60|qx)[^\n]*\borq\b|\borq\b[^\n]*(subprocess|system|spawn|exec)`)
)

// interpreteEscapa: python/perl/node/ruby... con código en línea que hace fork/setsid/daemon,
// abre un pty o lanza orq. Defensa en profundidad: el control que vale es el ancla en orq.
func interpreteEscapa(c string) bool {
	return interpCodigoRe.MatchString(c) && codigoEscapaRe.MatchString(c)
}

// escapaDelArbol mira el comando de cada segmento (tras VAR=valor y sudo/env/exec).
func escapaDelArbol(c string) bool {
	for _, seg := range segOrqRe.Split(c, -1) {
		f := strings.Fields(seg)
		for len(f) > 0 && (strings.Contains(f[0], "=") || f[0] == "exec" || f[0] == "command" || f[0] == "builtin" || f[0] == "sudo" || f[0] == "env") {
			f = f[1:]
		}
		if len(f) == 0 {
			continue
		}
		cmd := path.Base(f[0])
		if sueltaArbol[cmd] {
			return true
		}
		if cmd == "tmux" {
			for _, a := range f[1:] {
				if strings.HasPrefix(a, "-") {
					continue
				}
				for _, x := range []string{"new", "split", "run", "respawn"} {
					if strings.HasPrefix(a, x) {
						return true
					}
				}
				break
			}
		}
	}
	return false
}

// dirConfigOrq: la carpeta de config de orq (XDG_CONFIG_HOME/orq o ~/.config/orq), con token y sessions/.
func dirConfigOrq() string {
	cfg, err := os.UserConfigDir()
	if err != nil || cfg == "" {
		return ""
	}
	return filepath.Join(cfg, "orq")
}

// workerLee: Read/Grep/Glob (y Write/Edit) de un worker no tocan la config de orq ni el entorno de
// otros procesos: ni leer el token ni plantar un ancla en sessions/.
func workerLee(p Payload) (Decision, bool) {
	switch p.ToolName {
	case "Read", "Grep", "Glob", "LS", "NotebookRead", "Write", "Edit", "MultiEdit", "NotebookEdit":
	default:
		return Decision{}, false
	}
	home, _ := os.UserHomeDir()
	cfg, _ := os.UserConfigDir()
	var rutas []string
	for _, k := range []string{"file_path", "path", "notebook_path"} {
		if v, _ := p.ToolInput[k].(string); v != "" {
			rutas = append(rutas, v)
		}
	}
	if p.ToolName == "Glob" {
		if v, _ := p.ToolInput["pattern"].(string); v != "" {
			rutas = append(rutas, v)
		}
	}
	for _, r := range rutas {
		if strings.HasPrefix(r, "~") && home != "" {
			r = home + r[1:]
		}
		if !filepath.IsAbs(r) && p.Cwd != "" {
			r = filepath.Join(p.Cwd, r)
		}
		r = filepath.Clean(r)
		malo := strings.Contains(r, "/.config") || strings.Contains(r, "owner.token") ||
			(cfg != "" && (r == cfg || strings.HasPrefix(r, cfg+"/"))) ||
			r == "/proc" || (strings.HasPrefix(r, "/proc/") && (strings.Contains(r, "environ") || strings.ContainsAny(r, "*?[")))
		if malo {
			return deny("%s", lang.T("orq: a worker does not read or write orq's config directory or another process's environment.",
				"orq: un worker no lee ni escribe la carpeta de config de orq ni el entorno de otro proceso.")), true
		}
	}
	return Decision{}, false
}
