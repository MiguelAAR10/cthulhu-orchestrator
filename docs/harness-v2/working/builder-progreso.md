# builder · progreso (rama v2/build)

| id | commit | pruebas | no hecho |
|---|---|---|---|
| plan | e781629 | n/a (solo docs) | — |
| C1 | 62d1642 | gofmt, vet, go test -race ./... verde; validar-mvp 16/16; e2e-v05 74/74 | las skills instaladas (SKILL.md) ya no nombran modelos, remiten a `orq config ver`; no se plantillan porque se comparan byte a byte al instalar |
| C2 | b18fd1d | gofmt, vet, go test -race ./... verde (incl. arch_test, 6 pruebas de lanzamiento con terminal falso, Orca contra binario falso); validar-mvp 16/16; e2e-v05 74/74 | Orca sin probar en vivo: la app de Orca no corre en esta máquina (`orca status`: not_running); el adaptador sigue el esquema de `orca agent-context --json` y busca el handle term_… sin fijar el esquema de la respuesta. `Dentro()` de Orca pregunta `orca terminal show` (no hay variable de entorno documentada). `runMsg`, `runSlice` siguen en cli con su lógica (se mueven en C6/B5). Incluye la nota del Orchestrator: `entorno` (cuenta) por rol y por dificultad |
| C3 | 6f580b5 | gofmt, vet, go test -race ./... verde; validar-mvp 18/18 (2 nuevas: sin RAM bloquea y lo dice; status muestra capacidad); e2e-v05 74/74 | el conteo de workers vivos sigue siendo por worktree enlazado (cambia a clones en B5); no hay cgroups: el presupuesto por agente es una regla de admisión, no un límite que el kernel haga cumplir |

## Owner decisions
(ninguna todavía)
