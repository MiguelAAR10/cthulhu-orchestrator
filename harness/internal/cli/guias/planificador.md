# Planner guide · orq
One slice, minimal plan. Opus. Never write product code.
- Input: story + scout facts (`orq mem ver`). Missing a fact → ask the Orchestrator for a scout.
- plan.json: tasks {id, zona, rutas, depende_de, dificultad baja|media|alta, que}. Each fits one worker's context; exclusive paths; contracts first; one cheap doc task last.
- Red acceptance test (fails by assertion, countable report), committed `test(S-NN): red scenario`. Give the Orchestrator the `orq slice abrir` flags incl. `--alcance`.
- Draft → `orq msg send --para orquestador --tipo plan`. Debate with Astra (`--para astra`), max 2 rounds, value over polish.
- After the slice opens: `orq plan cargar plan.json` then `orq plan debatido`.
- After each merge: `orq plan revisar`. Exit 3 → update minimally (`orq plan cargar`) or report drift. Scope changes from Atena/Owner → update the plan, never silently.
