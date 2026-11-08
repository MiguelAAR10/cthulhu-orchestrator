package gate

import (
	"path"
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
		k, v, conIgual := strings.Cut(args[i], "=")
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
			if cmd == "msg" && (v == "cierre" || v == "closing") {
				return false
			}
		}
	}
	return true
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
