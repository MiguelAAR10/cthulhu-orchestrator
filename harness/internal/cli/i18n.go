package cli

import "strings"

// English command, subcommand, flag and value names. Internally orq keeps its original
// Spanish names; normalizeArgs maps the English forms onto them, so both work forever.

var cmdEN = map[string]string{
	"session": "sesion", "learn": "aprender", "align": "alinear", "metrics": "metricas",
	"context": "contexto", "phase": "fase", "handoff-note": "relevo",
}

var subEN = map[string]map[string]string{
	"slice": {"open": "abrir", "close": "cerrar", "launch": "lanzar", "merge": "juntar", "verify": "verificar",
		"retry": "reintentar", "take": "tomar", "exam": "examen", "scope": "alcance", "status": "estado",
		"receipt": "recibo", "reopen": "reabrir", "refingerprint": "rehuella", "prune": "podar", "contract": "contrato"},
	"plan":     {"load": "cargar", "show": "ver", "done": "hecha", "release": "soltar", "ready": "listas", "review": "revisar", "debated": "debatido"},
	"sesion":   {"show": "ver", "open": "abrir", "close": "cerrar"},
	"mem":      {"show": "ver", "compact": "compactar"},
	"msg":      {"read": "leer", "reconcile": "reconciliar"},
	"aprender": {"show": "ver", "confirm": "confirmar", "refute": "refutar", "promote": "promover"},
	"autonomy": {"show": "ver"},
	"qa":       {"show": "ver"},
}

var flagEN = map[string]map[string]string{
	"slice": {"approved": "aprobado", "accepted": "aceptado", "rejected": "rechazado", "strict": "estricto",
		"agent": "agente", "difficulty": "dificultad", "budget": "presupuesto", "task": "tarea", "zone": "zona",
		"paths": "rutas", "scope": "alcance", "title": "titulo", "healthy": "sano", "no-scouts": "sin-scouts",
		"report": "reporte", "format": "formato", "allow-skip": "permite-omitir", "isolated": "aislado",
		"handoff": "relevo"},
	"msg":      {"to": "para", "type": "tipo", "attempt": "intento", "key": "clave", "resend": "reenviar", "from": "de"},
	"mem":      {"source": "fuente", "evidence": "evidencia", "expires": "vence", "replaces": "reemplaza"},
	"aprender": {"source": "fuente", "evidence": "evidencia"},
	"relevo":   {"file-only": "solo-archivo", "force": "forzar"},
	"metricas": {"since": "desde"},
	"init":     {"mode": "modo"},
}

var valueEN = map[string]string{
	"low": "baja", "medium": "media", "high": "alta",
	"story": "historia", "question": "pregunta", "answer": "respuesta", "closing": "cierre",
	"planner": "planificador", "orchestrator": "orquestador", "verifier": "verificador",
	"improve": "mejora", "new": "nuevo", "status": "estado",
}

// flags whose values (and positional role arguments) are translated.
var valueFlags = map[string]bool{"dificultad": true, "tipo": true, "para": true, "modo": true, "imp": true}

func normalizeArgs(args []string) []string {
	if len(args) == 0 {
		return args
	}
	out := append([]string{}, args...)
	if c, ok := cmdEN[out[0]]; ok {
		out[0] = c
	}
	cmd := out[0]
	// The subcommand is the first non-flag word, skipping `--slice NN` / `--repo X`.
	if subs := subEN[cmd]; subs != nil {
		for i := 1; i < len(out); i++ {
			if out[i] == "--slice" || out[i] == "--repo" {
				i++
				continue
			}
			if strings.HasPrefix(out[i], "-") {
				break
			}
			if s, ok := subs[out[i]]; ok {
				out[i] = s
			}
			if (cmd == "sesion" && out[i] == "abrir") && i+1 < len(out) {
				out[i+1] = tradValor(out[i+1])
			}
			break
		}
	}
	if cmd == "relevo" || cmd == "skill" {
		for i := 1; i < len(out); i++ {
			if !strings.HasPrefix(out[i], "-") {
				out[i] = tradValor(out[i])
			}
		}
	}
	flags := flagEN[cmd]
	for i := 1; i < len(out); i++ {
		a := out[i]
		if !strings.HasPrefix(a, "--") {
			continue
		}
		name, val, eq := strings.Cut(a[2:], "=")
		if es, ok := flags[name]; ok {
			name = es
		}
		if valueFlags[name] {
			if eq {
				val = tradValor(val)
			} else if i+1 < len(out) {
				out[i+1] = tradValor(out[i+1])
			}
		}
		if eq {
			out[i] = "--" + name + "=" + val
		} else {
			out[i] = "--" + name
		}
	}
	return out
}

func tradValor(v string) string {
	if es, ok := valueEN[v]; ok {
		return es
	}
	return v
}
