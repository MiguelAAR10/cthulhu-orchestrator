package cli

import (
	"strings"

	"orq/internal/identidad"
)

// tarjetaPlanner is the orq2 rule for the card: the Owner's decision document, not the plan.
const tarjetaPlanner = `## Card (the Owner's decision document)
After consensus, write ` + "`card.md`" + ` and send its path to the Orchestrator. The Owner approves the slice from it; the technical plan already lives in ` + "`.orq/plan-sNN.json`" + `. Write every section except ` + "`## Zonas`" + ` in plain Spanish a non-programmer reads in 2 minutes. Stay within the card caps under orq2 rules (` + "`orq slice tarjeta --topes`" + ` prints them); a card past them is refused. Keep the section names exactly, orq parses them: ` + "`## Historia`, `## Fuera`, `## Examen`, `## Zonas`, `## Preguntas`" + `.
- ` + "`## Historia`" + `: who, and what they will see and do, within the Historia cap; the ` + "`Valor:`" + ` line counts as one of them: ` + "`Valor: <product metric> hoy <value> → meta <value>`" + `.
- ` + "`## Fuera`" + `: what this slice does not do, in plain words.
- ` + "`## Examen`" + `: one line per test group saying what behaviour is proven ("si hay un precio en el mensaje, no se envía"); no status codes or internals; the test file path once.
- ` + "`## Zonas`" + `: one repo path per line (folder or file), relative to the repo root, never absolute or ` + "`..`" + `, never the root itself; orq checks every changed file against these paths. Nothing else goes there: what changes for the Owner goes in ` + "`## Historia` and `## Examen`" + `. Task ids (T1…), function and env var names and technical words stay in the plan.
- ` + "`## Preguntas`" + `: only what the Owner must answer, each with your recommendation ("Recomiendo: …"). Human gates as a checklist: ` + "`- [ ] lo haces tú: …`" + `.
A word only a programmer understands (HTTP 422, dedup key, fake, JUnit, mock) does not go on the card (Zonas paths excepted): say what it means for the user, or leave it in the plan.

`

// tarjetaOrq is how orq's orchestrator skill defines the card; orq2 points to the planner's rule.
const (
	tarjetaOrq  = "(`## Historia` ≤3 lines, `## Fuera`, `## Examen`, `## Zonas`, `## Preguntas`)"
	tarjetaOrq2 = "(plain Spanish for the Owner, within the card caps under orq2 rules, sections `## Historia`, `## Fuera`, `## Examen`, `## Zonas`, `## Preguntas`; rules in the `orq-planner` skill, section Card; send back a card with technical words before setting it)"
)

// skillOrq2 adapts an orq skill's text for orq2; for orq it returns s untouched.
func skillOrq2(nombre, s string) string {
	if !identidad.EsOrq2() {
		return s
	}
	switch nombre {
	case "orq-planner":
		return strings.Replace(s, "## Astra\n", tarjetaPlanner+"## Astra\n", 1)
	case "orq-orchestrator":
		return strings.Replace(s, tarjetaOrq, tarjetaOrq2, 1)
	}
	return s
}
