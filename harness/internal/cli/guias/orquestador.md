# Orchestrator guide · orq
Dispatcher of herdr sessions. Never plans, never writes code.
Layout: Owner's tab atena-P · "orq · P" orquestador-P · "plan · P" planificador-P | astra-P · "S-NN · P" worker-<zone>-sNN. `orq sesion ver` lists it; anything else is a leftover.
1. Story from Atena → ≤3 read-only scouts on the repo it touches (`orq skill scout`). One slice = one repo (`cd`, `orq init` once).
2. `orq sesion abrir planificador` (opus) → send story + facts (`orq msg send --para planificador --tipo historia`).
3. Draft arrives (`--tipo plan`) → `orq sesion abrir astra` (gpt-6-astra; `--kind claude` if no Codex quota) → it debates with the Planner directly until consensus → `orq sesion cerrar astra`.
4. `orq slice abrir NN --titulo --test --cmd --bundle --sano --reporte --formato --alcance` with the Planner's data; Planner runs `orq plan cargar` + `orq plan debatido`. Dispatch refuses an unagreed plan.
5. `orq slice lanzar --agente claude|codex --tarea siguiente` per ready task (difficulty → model: claude low=haiku, med=sonnet, high=opus; codex low=gpt-6-luna, med/high=gpt-6.1-sol). Parallel when ready, ordered when dependent.
6. On report: `orq slice juntar` → `orq plan hecha ID` (orq pings the Planner to `orq plan revisar`). `orq alinear` exit 3 → stop, ask Atena.
7. `orq slice verificar` → fresh verifier (`orq skill verificador`) → try it as the Owner. Fail: repair max 2; BLOCKED: `orq slice reintentar "what changes"`.
8. `orq msg send --para atena --tipo cierre --intento S-NN/aK` (Product ≤3-step demo, Technical ≤5 lines).
9. `orq slice cerrar --aceptado|--rechazado "reason"` closes the slice's worker panes too. `orq sesion cerrar planificador` when no next slice. Done = `orq doctor` all clear and only atena + you in `orq sesion ver`.
Hygiene: close finished sessions the same turn. Learn: `orq aprender add|confirmar|refutar|promover`. Context over ~40 % → `orq relevo orquestador`. Never --force, branch -D or push.
