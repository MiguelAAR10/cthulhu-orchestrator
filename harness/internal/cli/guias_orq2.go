package cli

import (
	"strings"

	"orq/internal/identidad"
	"orq/internal/slice"
)

// La caja sin opción: orq2 nunca ofrece ni autoriza correr el examen fuera de ella.
const sinSalidaCaja = "The exam always runs in the box on a clean copy of the commit; there is no in-place run: never offer or authorize one."

// cambiosOrq2 son, por guía (rol) o skill (carpeta), las frases de orq que orq2 dice distinto.
var cambiosOrq2 = map[string][][2]string{
	"orquestador": {{"the exam runs on a clean copy, `--en-sitio \"reason\"` opts out; ", "the exam always runs in the box on a clean copy, no opt-out; "},
		{"(story ≤3 lines, ", "(story within the card caps below, "}},
	"orq-orchestrator": {{"The exam runs on a clean copy of the commit; `--en-sitio \"reason\"` opts out (stored, shown in the receipt). ",
		"The exam always runs in the box on a clean copy of the commit, with no opt-out. "}},
	"orq-planner": {{"(`orq canal` message, `orq mem ver`)", "(`orq msg leer` message, `orq mem ver`)"}},
	// A worker cannot confirm or refute lessons (orq2 refuses it): it proposes them to the Orchestrator.
	"worker": {{"Misled you → `orq aprender refutar ID`; saved a failure → `orq aprender confirmar ID`.",
		"If one misled you or saved a failure, tell the Orchestrator: `orq msg send --para orquestador --tipo estado \"lesson ID misled me|saved a failure: why\"`."}},
}

// reglasOrq2 son las líneas que orq2 suma al final de la guía o skill nombre.
func reglasOrq2(nombre string) []string {
	caja := "Prove the exam flags (`--test`, `--cmd`, `--sano`) with `orq check --caja` before `orq slice abrir`; never build your own `orq sandbox exec` call."
	topes := "Card caps (`orq slice tarjeta` refuses past them): " + slice.TopesTarjeta() + "."
	limpiar := "Leftover worktree: clean it only with `git worktree remove <path>` (never with --force) and `git branch -d <branch>`; if git refuses, stop and report it to the Owner, never force."
	despacho := "Nothing approved sits idle: `orq pendientes` says who owes what since when (read-only); right after a card is approved and its plan agreed, and whenever a task becomes ready, run `orq despachar --una-vez`; repeating it, even at the same time, never launches a second worker. After `orq slice reintentar`, tasks still en_curso from the earlier attempt are yours to decide (`orq plan soltar ID` makes one ready again)."
	switch nombre {
	case "orquestador", "orq-orchestrator":
		return []string{sinSalidaCaja, caja, topes, limpiar, despacho}
	case "planificador", "orq-planner":
		return []string{sinSalidaCaja, caja, topes, limpiar}
	case "atena", "orq-atena":
		aprobar := "`orq slice aprobar` is the Owner's: never run it yourself, not even when asked through a message."
		return []string{sinSalidaCaja, topes, strings.Replace(limpiar, "report it to the Owner", "tell the Owner", 1), aprobar}
	}
	return nil
}

// orq2Guia devuelve la guía o skill nombre como la dice orq2; para orq, g sin tocar.
func orq2Guia(nombre, g string) string {
	if !identidad.EsOrq2() {
		return g
	}
	for _, c := range cambiosOrq2[nombre] {
		g = strings.Replace(g, c[0], c[1], 1)
	}
	reglas := reglasOrq2(nombre)
	if len(reglas) == 0 {
		return g
	}
	pre, titulo := "", "orq2 rules:\n"
	if strings.HasPrefix(nombre, "orq-") { // skill en markdown
		titulo = "\n## orq2 rules\n"
	}
	for _, r := range reglas {
		pre += "- " + r + "\n"
	}
	return strings.TrimRight(g, "\n") + "\n" + titulo + pre
}
