package gate

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"orq/internal/lang"
)

func negadoOrq() Decision {
	return deny("%s", lang.T("orq: a worker does not write to .orq or orq's state folder; it reports with `orq worker beat|done`, `orq mem add`, `orq stop raise` or `orq msg send`.",
		"orq: un worker no escribe en .orq ni en la carpeta de estado de orq; informa con `orq worker beat|done`, `orq mem add`, `orq stop raise` u `orq msg send`."))
}

func dirsEstado(env Env) []string {
	var out []string
	add := func(d string) {
		if d != "" {
			out = append(out, filepath.Clean(d), rutaReal(filepath.Clean(d), 0))
		}
	}
	add(env.StateDir)
	if x := os.Getenv("XDG_STATE_HOME"); x != "" {
		add(filepath.Join(x, "orq"))
	}
	if h, err := os.UserHomeDir(); err == nil {
		add(filepath.Join(h, ".local", "state", "orq"))
	}
	return out
}

// tocaOrq: la ruta real cae en una carpeta .orq (salvo el propio worktree, que puede vivir bajo
// una) o en el estado de orq. Un segmento sin expandir también cuenta: $M/.orq/x.
func tocaOrq(ruta, cwd string, env Env) bool {
	for _, d := range dirsEstado(env) {
		if dentroDe(ruta, d) {
			return true
		}
	}
	resto := ruta
	if raiz := raizWorktree(cwd); raiz != "" {
		for _, r := range []string{raiz, rutaReal(raiz, 0)} {
			if dentroDe(ruta, r) {
				resto = strings.TrimPrefix(ruta, r)
				break
			}
		}
	}
	return strings.Contains(resto+"/", "/.orq/")
}

// estadoEnTextoRe: destinos con variables que el gate no expande pero dicen a dónde van.
var estadoEnTextoRe = regexp.MustCompile(`\.local/state/orq(/|\s|$)|\$\{?(ORQ_STATE|XDG_STATE_HOME)\b`)

var (
	// findBorraRe: find que borra o ejecuta, sobre una ruta que nombra .orq o el estado.
	findBorraRe = regexp.MustCompile(`\bfind\b[^;&|]*(\.orq|\.local/state/orq)[^;&|]*\s-(delete|exec|execdir|ok)\b`)
	// inlineOrqRe: código en línea de un intérprete, o awk -i inplace, que nombra .orq o el estado.
	inlineOrqRe = regexp.MustCompile(`(^|[\s;&|(/])(python[0-9.]*|perl|node|nodejs|ruby|php|deno|bun|lua|awk|gawk)\b[^;&|]*(\s-\w*[cei]\b|\s--eval\b|<<)[^\n]*(\.orq\b|\.local/state/orq)`)
)

// workerEscribeOrq: Write/Edit/MultiEdit/NotebookEdit y Bash, con la misma regla de ruta real.
func workerEscribeOrq(p Payload, env Env) (Decision, bool) {
	switch p.ToolName {
	case "Write", "Edit", "MultiEdit", "NotebookEdit":
		for _, k := range []string{"file_path", "notebook_path"} {
			if v, _ := p.ToolInput[k].(string); v != "" && (tocaOrq(limpia(p.Cwd, v), p.Cwd, env) || tocaOrq(resolver(p.Cwd, v), p.Cwd, env)) {
				return negadoOrq(), true
			}
		}
	case "Bash":
		cmd, _ := p.ToolInput["command"].(string)
		c := strings.NewReplacer(`'`, "", `"`, "", `\`, "").Replace(cmd)
		for _, t := range shellWriteTargets(c, p.Cwd) {
			if tocaOrq(t, p.Cwd, env) {
				return negadoOrq(), true
			}
		}
		for _, seg := range segRe.Split(c, -1) {
			for _, m := range redirRe.FindAllStringSubmatch(seg, -1) {
				if estadoEnTextoRe.MatchString(m[1]) {
					return negadoOrq(), true
				}
			}
			f := stripPrefixes(strings.Fields(seg))
			if len(f) > 1 && escribeArgs[f[0]] && estadoEnTextoRe.MatchString(seg) {
				return negadoOrq(), true
			}
		}
		if findBorraRe.MatchString(c) || inlineOrqRe.MatchString(c) {
			return negadoOrq(), true
		}
		for _, d := range dirsEstado(env) {
			if strings.Contains(c, d) && (findBorraRe.MatchString(strings.ReplaceAll(c, d, ".orq")) || inlineOrqRe.MatchString(strings.ReplaceAll(c, d, ".orq"))) {
				return negadoOrq(), true
			}
		}
	}
	return Decision{}, false
}

var escribeArgs = set("tee", "rm", "mv", "truncate", "unlink", "shred", "touch", "chmod", "ln", "cp", "install", "rsync", "dd", "sed", "perl")
