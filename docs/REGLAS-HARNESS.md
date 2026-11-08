# Qué hace el harness `orq` y qué número controla cada cosa

Para el Owner. Los números viven en `~/.config/orq/limits.json`; cambiarlos no requiere tocar código. Si el archivo falta o tiene un campo mal escrito, `orq` bloquea lanzamientos hasta que se arregle: prefiere parar a adivinar.

| Qué protege | Campo | Valor inicial | Por qué |
|---|---|---|---|
| Disco de Linux (`/`) | `min_free_gb["/"]` | 60 GB | Margen para un worktree con dependencias |
| Disco de Windows (`/mnt/c`) | `min_free_gb["/mnt/c"]` | 40 GB | Es el disco físico; el incidente de octubre fue cuando llegó a 0 |
| Workers simultáneos | `max_workers` | 4 | Hasta 4 workers en paralelo (Claude y Codex); el Orquestador usa solo los necesarios |
| Directorios desechables | `scratch.paths` | `orq/scratch`, `worktrees` (agrega los tuyos en tu limits.json local) | Lo desechable que crece sin control |
| Cuota de scratch | `scratch.quota_gb` | 200 GB hoy; bajarla tras limpiar `orq/` | Pasada la cuota, `orq watch` escribe HALT y nada se lanza |
| Disco crítico | `scratch.critical_free_gb` | 30 GB | Por debajo, HALT inmediato |
| Antigüedad de la medición | `state_max_age_hours` | 24 h | Sin medición fresca no se lanza |
| Worktrees de orq por proyecto | `max_worktrees` | 12 (si falta, 12) | Con 344 worktrees y 81 GB WSL se trabó; pasado el tope no se lanza hasta correr `orq gc` |
| Archivos que el worker no toca | `protected` | `tests/aceptacion/`, `SLICES.md` | El test es la spec; el ledger es la memoria |

## Qué cuenta como worker

Un proceso `claude` o `codex` de primer nivel cuyo directorio es un worktree enlazado de git. Tus sesiones abiertas en el checkout principal o en tu carpeta personal no cuentan. Un worker tampoco puede lanzar otro: él mismo ya ocupa el cupo.

## Qué cuenta como lanzamiento

Crear un worker, un panel o una copia de trabajo desde herdr, desde Orca (`orca` y `orca-ide`), con `codex` o con `claude -p`. Solo esos comandos pasan por las comprobaciones de disco, HALT, medición y tope de workers. Leer o listar, como `orca status` o `herdr agent list`, pasa sin trabas. Desde el 23-oct-2026 el guardia reconoce Orca; antes solo veía herdr y la línea de comandos.

## Una sola puerta de admisión

El gate (hook) y `orq slice lanzar` usan la misma función (`internal/admision`): límites válidos, sin HALT, disco sobre `min_free_gb`, medición de `orq watch` fresca, scratch bajo la cuota, workers bajo `max_workers` y worktrees registrados bajo `max_worktrees`. `orq slice lanzar` además reserva el cupo bajo candado (`reservas.json` en el estado de orq) hasta que el worker arranca o falla; una reserva sin liberar vence a los 10 minutos. Si la admisión rechaza, no se llama a herdr.

## Registro de worktrees y `orq gc`

Cada worktree que orq crea (slice, zona, integración) queda en `.orq/worktrees.jsonl` con `{path, repo, owner_role, slice, kind, created, ttl}` (ttl por defecto 72h). `orq gc [--dry-run]` revisa solo esas entradas y retira un worktree únicamente si: no es de un slice abierto, ningún proceso trabaja adentro, `git status --porcelain --untracked-files=all` sale vacío y no hubo actividad en el ttl (el puntero `.git` del worktree no cuenta como actividad). Si su HEAD no está en una rama local o remota, primero se guarda en `.orq/bundles/*.bundle` verificado. Los ignorados únicos (no dependencias regenerables) se respaldan en `.orq/backups/<nombre>.tgz` con manifiesto y sha256 comprobado releyendo; si alguno parece secreto (`.env`, claves, tokens) el worktree se conserva y se avisa. Luego `git worktree remove` **sin --force**; si git se niega, o falta el `.git`, va a `.orq/cuarentena.jsonl` con el motivo. Del registro solo se podan las rutas que ya no existen. `orq gc` corre solo al final de `orq slice cerrar` y dentro de `orq watch`, que además avisa (sin HALT) cuando el disco baja de `min_free_gb` y muestra cuántos worktrees hay.

## Runner y agentes (datos, no código)

- **Runner**: todo panel, arranque y mensaje pasa por `internal/runner` (`herdr`, `tmux` o `process`). Se elige con `ORQ_RUNNER`, luego `"runner"` en `orq.agents.json`; por defecto herdr si estás dentro o está instalado, si no `process` (proceso hijo headless con log; cada prompt es una corrida nueva).
- **Agentes**: `orq.agents.json` del proyecto pisa a `~/.config/orq/agents.json`, que pisa a la propuesta de `orq config agents detect` (`.orq/agents.propuesta.json`), que pisa a los defaults integrados (claude y codex de siempre, más cursor-agent, gemini, opencode y aider). Plantilla: `templates/orq.agents.json`. `orq config agents set ROL AGENTE:MODELO` escribe en el archivo del proyecto: lo del Owner siempre gana.
- **Reglas en código**: el adversario (Astra) es de otra familia que el planificador (se rechaza al cargar, al `set` y al abrir Astra); el verificador siempre arranca en sesión nueva; docs/escriba usa el modelo más barato configurado de su agente.

## H-06: gate presente o post-only

- **Agente con hooks (Claude)**: `orq slice abrir` deja `.claude/settings.json` con `orq gate` en el worktree del slice (lo copia del proyecto si falta) y `orq slice lanzar` lo exige en el worktree del worker: sin hook, no lanza. El registro de sesión queda con `gate: hook`.
- **Agente sin hooks (Codex y otros)**: se registra `gate: post-only`. Su protección es verificar/juntar/cerrar (huella del examen, zonas) y el sistema operativo: al lanzar, los archivos del examen quedan de solo lectura (`chmod a-w`) en el worktree del worker; solo `orq slice cerrar` les devuelve la escritura. No impide un `chmod` deliberado del agente: eso lo atrapa la huella en Verificar.
- **Escriba**: cada llamada tiene tiempo máximo (`ORQ_ESCRIBA_TIMEOUT` en segundos, 120 por defecto). Si falla o vence, el cierre, el relevo y la ficha siguen con un aviso.

## Qué pasa en cada caso

- **Lanzar con disco bajo, HALT, medición vieja, scratch sobre cuota o demasiados worktrees** (gate o `orq slice lanzar`): bloqueado, con la causa y el comando para resolverla.
- **El worker edita `tests/aceptacion/`** con Edit, Write, redirección, `tee`, `sed -i`, `cp`, `mv`, `rm` o `git checkout`/`restore`: bloqueado. Correr el test, aunque sea con `2>&1`, está permitido.
- **El worker escribe el test por una vía que el gate no reconoce**, por ejemplo un script de Python: lo atrapa el paso Verificar, porque el `git diff` del test debe salir vacío.
- **El worker cambia el contrato de aceptación** con `orq slice examen` o `orq slice alcance`: bloqueado en el gate y también en el propio comando (desde un worktree enlazado falla aunque falte el hook). Solo el Orquestador lo cambia, con `--aprobado` empezando por el id de una decisión registrada (`mNN` de memoria con fuente owner/atena, o `cNN` de un mensaje de atena); cambiar el examen anula el recibo previo y obliga a verificar de nuevo La decisión tiene que nombrar el slice (`S-NN` en el texto, o su campo slice/attempt) y ser posterior a la apertura o al último cambio de contrato: un id viejo o de otra historia se rechaza.
- **Recibos y contrato**: un solo evaluador (`slice.Evaluar`) juzga la verificación del worker y la revalidación tras integrar (exit 0, huella intacta, >0 ejecutadas, 0 fallidas, omitidas solo con `--permite-omitir`). El SHA integrado deja su propio recibo (`tipo: integracion` en `recibos.jsonl`) y sin él no se publica. Reintentar, reabrir, rehuella, sumar examen y ampliar alcance anulan todos los recibos. Modo estricto (`orq slice abrir --estricto` o `"conteo_estricto": true` en limits.json): `--reporte/--formato` obligatorios, sin contadores no hay verde. Por defecto sigue laxo por compatibilidad (avisa "sin contadores").
- **Estado compartido (H-04/H-05)**: el canal `canal.jsonl` es append-only; cerrar S-NN solo archiva sus mensajes ya entregados (y los entregados sin slice), nunca lo pendiente de otro slice. Verificar guarda la revisión del contrato antes de correr el examen (sin candado) y, al terminar, relee bajo candado: si el contrato cambió descarta el resultado (no verificado); si no, actualiza el estado actual. Reservar, marcar hecha, soltar, debatir y cargar el plan pasan por un candado de archivo. Al recargar el plan, una tarea hecha cuyo contrato (id, zona, rutas, dependencias, dificultad, qué) cambió vuelve a pendiente; `debatido` queda ligado a la revisión del plan y cualquier cambio de contrato lo borra.
- **Memoria con tope**: las líneas automáticas del slice (alcance +, huella, reabrir, reintentar) vencen con el slice (`vence: S-NN`). `orq contexto` inyecta como mucho 25 líneas: lo crítico, luego el slice activo, luego el resto.
- **`orq` no está o falla:** el hook tiene `onFailure: block` y bloquea en vez de dejar pasar.

## Visibilidad (slice 5): estado de workers, stoppers, autonomía, tablero, handoff y memoria

- **Estado del worker**: `.orq/workers/<id>/state.json` (`orq.worker/v1`), escrito por tmp + rename. Solo lo escriben el worker (`orq worker beat|done`, id por `ORQ_WORKER_ID`) y el vigía. `head` y `commits` los calcula orq con `git rev-list --count base..HEAD`; lo que informe el worker se pisa. `tests` sale del recibo contado del slice, o queda `null`; `context_pct` es `null` salvo que el agente lo diga. `--phase blocked` exige un stopper abierto. `orq worker done` marca la tarea `hecha` solo si `plan.Hecha` lo acepta y entonces agrega `done` a `.orq/tasks.jsonl` (`orq plan hecha` también; `orq plan soltar` agrega `reopened`). Pendiente: `orq slice lanzar` todavía no llama a `workers.Launched` (lo cablea el refactor del Runner).
- **Vigía de workers** (en `orq watch`, corre aunque falten los límites): `exited` (phase=exited), `dead` (pid propio inexistente en este host y phase ∉ {exited, asks_verification}), `stuck` (running, latido > `watch.stale_min` (10) **y** head igual al de la pasada anterior), `timeout` (>1.0× `budget_min` avisa; >1.5× además levanta `confirmation/soon` una sola vez), `ok`. Cada cambio de veredicto va una vez a `.orq/watch.jsonl` y un mensaje `de: vigia` al orquestador. Nunca mata procesos ni borra worktrees.
- **Stoppers** (`orq stop raise|list|resolve|cancel`, `.orq/stops.jsonl` solo de agregar, plegado por id, bajo candado): tipos credential|api_key|decision|confirmation|access, severidad blocking|soon|fyi. Un `blocking` retiene solo sus `--tasks` (`plan.Listas` da `stop ST-NNNN`); `soon`/`fyi` no retienen. Mismo (slice, tipo, texto) abierto → mismo id. Texto que parece secreto (prefijos `sk-`, `ghp_`, `xox`, `AKIA`, `AIza`, `-----BEGIN` o token ≥20 con entropía >3.5) → exit 2 sin escribir. `--provided-at` solo `env:NOMBRE`, `file:RUTA` fuera del repo o ignorada, `vault:REF`; nunca se guarda el valor. Resuelven atena y owner (sin rol = Owner en su terminal; con `ORQ_WORKER_ID` = worker). Resolver una decisión exige `--choice` y guarda memoria `cat=decision`.
- **Autonomía** (`orq autonomy [ver|guided|free --scope ...]`, en `.orq/config.json`, solo owner/atena la cambian): `guided` por defecto. En `free`, orquestador o planificador resuelven decision/confirmation `soon`/`fyi` con `--note "auto: motivo"` (memoria `fuente=orquestador|planificador`). Siempre llegan al Owner: todo `blocking` y todo credential/api_key/access; la aceptación de producto; lo irreversible; cambios del examen o contrato (H-01: un worker nunca usa `orq slice examen|alcance`, en ningún modo); lo que salga del alcance o del presupuesto.
- **Tablero** (`orq board [--json|--html [--out F]]`, solo lectura): stoppers abiertos primero (blocking, soon, fyi; lo más viejo primero), % global ponderado, tarjetas rojo/ámbar/azul/gris/verde con palabra de estado, % por peso (baja 1, media 2, alta 3), lo que falta (máx. 3 y `+N more`), quién, retrabajo; tira de workers y velocidad (eventos `done` de 24 h ÷ horas, mínimo 1 h; con menos de 2 eventos, `unknown`). Lo que no se puede calcular es `null`/`unknown`, nunca 0. HTML: un archivo con CSS en línea, sin scripts ni `http`.
- **Handoff** (`orq handoff [--out F]`): `HANDOFF.md` generado, 6 secciones, ≤120 líneas, sin rutas absolutas y con todo lo que parezca secreto tapado.
- **Memoria** (`orq init --sync|--memoria=local|commit|branch`, guardado en `config.json`): `local` ignora `.orq/`; `commit` lo versiona salvo workers, candados, board.html y salidas (y deja de escribir `/.orq/` en `.git/info/exclude`); `branch` usa `orq sync push|pull [--theirs]` con el worktree aparte `.orq-sync` en la rama `orq/state`: nunca toca el HEAD, el índice ni las ramas del usuario, nunca `--force` ni rebase; si diverge se detiene ("diverged").

## QA independiente y evidencia visual (slices 8–9) · ver docs/v2/QA.md

- **tester** (`ORQ_ROL=tester`, sesión fresca): el gate le niega Edit/Write/MultiEdit/NotebookEdit y todo Bash que escriba (redirecciones salvo `/dev/null`, `rm/mv/cp/tee/touch/sed -i`, `mkdir`, `git add/commit/checkout…`, instalaciones, intérpretes en línea) y los comandos del Orquestador. Prueba y registra hallazgos.
- **repairer** (`ORQ_ROL=repairer`, worker aparte): edita producto; el paquete del examen fijado por huella (H-01) le está protegido aunque no esté en un worktree enlazado.
- **`orq qa run S-NN`** (solo Orquestador; el gate y la CLI lo niegan a tester, repairer y workers): corre en orden las capas de `qa.layers` en `.orq/config.json` dentro del worktree del slice; recibo en `.orq/qa/S-NN/receipt.json` con conteos por capa. Las capas con `min_criticality: high` (mutación, adversarial) solo corren si la criticidad es high.
- **Criticidad**: `criticality` o `project.criticality` en `.orq/config.json` (low|medium|high y sinónimos), lectura tolerante, **medium** por defecto.
- **`orq qa finding add`** exige requirement, commit, repro, observed, expected, impact. orq re-ejecuta el repro: **confirmado solo si falla**; uno que pasa queda registrado y no cuenta.
- **Cierre**: `orq slice cerrar --aceptado` exige recibo QA verde y ningún hallazgo confirmado posterior cuando criticidad ≥ `qa.require_receipt_from` (medium).
- **Evidencia**: `orq evidence add S-NN --kind target|before|after --file PNG [--note] [--pulir]` guarda en `docs/slices/S-NN/`, calcula el % de píxeles distintos after vs target (stdlib, determinista) y regenera `README.md` ≤40 líneas. El pulido con el modelo `docs` es opcional y con tiempo máximo; si falla, queda el determinista.

## Skills (slice 7): perfil, enrutado fase×rol, galería con verificación y fijado

Detalle en inglés: `docs/v2/SKILLS.md`.

- **Perfil** (`orq profile`): detecta con señales: archivos de código, edad y commits de git, autores, tests, CI y manifiestos (package.json+tsconfig ⇒ typescript, pyproject/requirements ⇒ python, go.mod ⇒ go). CI con Dockerfile/deploy o con tags da la pista `production`. Tipo: greenfield (<5 archivos y <20 commits), legacy (≥3 años y le faltan tests o CI), brownfield el resto. `orq profile set type=… criticality=trivial|low|medium|high stage=… stacks=a,b` guarda en `.orq/profile.json`. El Owner gana siempre; orq avisa cuando la evidencia lo contradice.
- **Catálogo**: `skills.catalog.json` embebido, más `.orq/skills.catalog.json` del proyecto (el mismo id reemplaza). Cada entrada lleva id, fuente, install (`npx …` o `copy:NOMBRE`), ref y sha256 fijados, fases (T C A D V DOC), roles, project_types, stacks, min_criticality, token_tier, adapt y never.
- **Enrutado** (`orq skills plan --phase X --role R`): solo lo que encaja con tipo, stacks, fase, rol y criticidad efectiva (producción sube un nivel). Trivial ⇒ 0 o 1 skill; crítica en producción ⇒ todas las que encajan. `orq skill ROL` y `orq contexto` muestran solo el conjunto fase×rol: worker D, scout C, planificador/astra A, verificador V; atena y orquestador siguen la fase del proyecto. Matt Pocock va solo al frente (grilling en T desde medium).
- **Instalar** (`orq skills install [ID…]`): instala solo la selección, en `.claude/skills/` y `.agents/skills/`, y fija la huella en `.orq/skills.lock.json`. Rechaza la never-list, lo rechazado en la galería, lo que no pasa `vet` y lo que difiere del sha256 del catálogo o del lock.
- **Vet** (`orq skills vet ID [--file F]`): exige frontmatter con name/description y ≤20000 bytes. Rechaza `git push`, `--force`, `rm -rf`, `reset --hard`/`branch -D`, lanzar agentes y `curl|sh`; una línea que lo prohíbe no cuenta. El veredicto validated|rejected va con su motivo a `.orq/skills.gallery.json`.

## Comandos

```
orq status          límites, disco, medición, HALT y la lista de workers vivos
orq watch           mide y escribe HALT si corresponde
orq halt --clear    quitar HALT, solo después de liberar espacio
```
