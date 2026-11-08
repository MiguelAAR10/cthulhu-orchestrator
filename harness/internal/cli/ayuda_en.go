package cli

import (
	"flag"
	"fmt"
	"strings"

	"orq/internal/lang"
)

// usoEN: the English help text of each flag. Flags keep their Spanish usage in the source;
// `orq <cmd> -h` prints the English text and the English flag name unless ORQ_LANG=es.
var usoEN = map[string]string{
	"--reporte/--formato obligatorios: sin contadores no hay verde (también limits.json conteo_estricto)": "--report/--format required: no counts, no green (also limits.json conteo_estricto)",
	"ID del plan o 'siguiente': zona, rutas y dificultad salen de la tarea":                               "plan task ID or 'next': zone, paths and difficulty come from the task",
	"S-NN: deja de valer al cerrarse ese slice":                                                           "S-NN: stops applying when that slice closes",
	"abre sesión aunque ya haya una nueva viva":                                                           "open a session even if a fresh one is alive",
	"acepta pruebas omitidas":                                                                             "accept skipped tests",
	"agente de orq.agents.json (por defecto, el de roles.worker por riesgo)":                              "agent from orq.agents.json (default: roles.worker by risk)",
	"agente de orq.agents.json (por defecto, el del rol)":                                                 "agent from orq.agents.json (default: the role's)",
	"archivo que deja el runner (para contar pruebas)":                                                    "file the test runner writes (to count tests)",
	"baja|media|alta: elige modelo y esfuerzo (haiku/sonnet/opus; luna/sol)":                              "low|medium|high: picks model and effort (haiku/sonnet/opus; luna/sol)",
	"carpetas de esa zona, separadas por coma":                                                            "folders of that zone, comma separated",
	"clave de idempotencia":                                                                               "idempotency key",
	"comando de la suite rápida":                                                                          "fast suite command",
	"comando nuevo del examen si debe correr también los archivos sumados":                                "new exam command if it must also run the added files",
	"comando que corre el examen":                                                                         "command that runs the exam",
	"comando que prueba que la base está sana antes de empezar":                                           "command that proves the base is healthy before starting",
	"comando que reproduce el fallo (debe fallar)":                                                        "command that reproduces the failure (must fail)",
	"commit probado": "commit tested",
	"config del runner y fixtures que tampoco se tocan, separados por coma":       "runner config and fixtures that are not touched either, comma separated",
	"corre el examen en una copia limpia del commit":                              "run the exam in a clean copy of the commit",
	"el Owner aceptó la demo":                                                     "the Owner accepted the demo",
	"escribe el relevo sin abrir sesión nueva":                                    "write the handoff note without opening a new session",
	"esfuerzo (por defecto, el del rol)":                                          "effort (default: the role's)",
	"filtra por categoría":                                                        "filter by category",
	"front, back, test…: suma el worker al equipo del slice":                      "front, back, test…: adds the worker to the slice team",
	"historia en pocas palabras":                                                  "the story in a few words",
	"hook PreCompact: rol de esta sesión, solo archivo, silencioso si no hay rol": "PreCompact hook: this session's role, file only, silent without a role",
	"id del mensaje al que respondes":                                             "id of the message you answer",
	"id que este hecho deja obsoleto":                                             "id this fact makes obsolete",
	"impacto para el usuario":                                                     "impact on the user",
	"intento al que se refiere un cierre (S-NN/aK; aK si hay un solo slice)":      "attempt a closing refers to (S-NN/aK; aK if there is one slice)",
	"lo esperado":                                                                         "expected",
	"lo observado":                                                                        "observed",
	"modelo (por defecto, el del rol)":                                                    "model (default: the role's)",
	"motivo del rechazo en una línea":                                                     "reason for rejecting, one line",
	"motivo para saltar el contexto (cambio trivial)":                                     "reason to skip context (trivial change)",
	"nombre del worker (por defecto, el escritor)":                                        "worker name (default: the writer)",
	"nota en lenguaje de negocio":                                                         "note in business language",
	"pule el README con el modelo de docs (opcional, con tiempo máximo)":                  "polish the README with the docs model (optional, time-boxed)",
	"quién la aprendió":                                                                   "who learned it",
	"quién lo aprobó y por qué (req o mem de Atena/Owner)":                                "who approved it and why (Atena/Owner req or mem)",
	"qué sí y qué no":                                                                     "what is in and what is out",
	"recibo, commit o S-NN/aK que la respalda":                                            "receipt, commit or S-NN/aK that backs it",
	"recibo, commit, archivo:línea o id de mensaje":                                       "receipt, commit, file:line or message id",
	"reemplaza al escritor actual en el mismo worktree":                                   "replace the current writer in the same worktree",
	"req de Atena u Owner que aprueba el examen nuevo":                                    "Atena or Owner req that approves the new exam",
	"requisito o línea de la historia":                                                    "requirement or story line",
	"rol que escribe (por defecto, el de este panel)":                                     "role that writes (default: this pane's)",
	"ruta del examen de aceptación, relativa a la raíz":                                   "acceptance exam path, relative to the root",
	"rutas que la historia justifica tocar, separadas por coma (alarma de desalineación)": "paths the story justifies touching, comma separated (misalignment alarm)",
	"segundos máximos del examen (0 = 600)":                                               "exam time limit in seconds (0 = 600)",
	"solo muestra qué haría":                                                              "only show what it would do",
	"también la suma a la zona de ese worker ya lanzado":                                  "also add it to the zone of that already launched worker",
	"tiempo o tokens":                                                                     "time or tokens",
	"últimos N":                                                                           "last N",
}

// valorEN: Spanish accepted values shown in a hint, with the English word orq also accepts.
func valorEN(v string) string {
	for en, es := range valueEN {
		if es == v {
			return en
		}
	}
	return v
}

// usoEnIngles traduce un texto de ayuda: la tabla, o una lista de valores a|b|c.
func usoEnIngles(u string) string {
	if en, ok := usoEN[u]; ok {
		return en
	}
	if strings.Contains(u, "|") && !strings.ContainsAny(u, " ") {
		vs := strings.Split(u, "|")
		for i, v := range vs {
			vs[i] = valorEN(v)
		}
		return strings.Join(vs, "|")
	}
	return u
}

// nuevoFS: flag.NewFlagSet con ayuda en inglés por defecto (nombre de flag inglés incluido).
func nuevoFS(nombre string, h flag.ErrorHandling) *flag.FlagSet {
	fs := flag.NewFlagSet(nombre, h)
	fs.Usage = func() {
		out := fs.Output()
		if lang.ES() {
			fmt.Fprintf(out, "Uso de %s:\n", nombre)
			fs.PrintDefaults()
			return
		}
		cmd, _, _ := strings.Cut(nombre, " ")
		if c, ok := cmdEN[cmd]; ok {
			cmd = c
		}
		nombres := map[string]string{}
		for en, es := range flagEN[cmd] {
			nombres[es] = en
		}
		fmt.Fprintf(out, "Usage of orq %s:\n", nombre)
		fs.VisitAll(func(f *flag.Flag) {
			n := f.Name
			if en, ok := nombres[n]; ok {
				n = en
			}
			tipo, uso := flag.UnquoteUsage(f)
			uso = usoEnIngles(uso)
			linea := "  --" + n
			if tipo != "" {
				linea += " " + tipo
			}
			if f.DefValue != "" && f.DefValue != "false" && f.DefValue != "0" {
				uso += fmt.Sprintf(" (default %q)", f.DefValue)
			}
			fmt.Fprintf(out, "%s\n    \t%s\n", linea, uso)
		})
	}
	return fs
}
