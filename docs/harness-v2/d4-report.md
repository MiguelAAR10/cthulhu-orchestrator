# D4 report · Big stories stay inside

Branch `v2/d4`, worktree `~/worktrees/cthulhu-d3`, built on `dd40972`. Plan: `docs/harness-v2/d4-plan.md`. Spec: `plan.md` › "Route (Owner, 2026-11-18 late)" D4, and `working/advice/fable-value-2026-11-18.md` §3 D4.

What I did not do:
- Install anything or push.
- Write to `~/bin/orq`, `~/.config/orq` or `~/.local/state/orq`.
- Call a real `claude`, `codex` or `herdr`. Every test uses temp `ORQ_*` dirs, a temp `ORQ_CONFIG`, the fake terminal and an injectable clock (`Proyecto.Ahora`).
- Add dependencies.

`internal/gate` has one change: `historia abrir|pausar|reanudar`, `pausar` and `reanudar` join the orchestrator-only list, with four rows in `decisiones_test.go`. `app/workers.go` has one change, three lines after `PuedeLanzar`: `r.Pausado(e)`.

## Commits

| Task | Commit | Subject |
|---|---|---|
| plan | `aafda23` | docs: D4 plan — child slices, pause, drift alerts, merge on worker close |
| 1 | `857ea43` | slice: stories group child slices |
| 2 | `4ede12a` | app: pause refuses new launches and destroys nothing |
| 3 + 4 | `905bb03` | cli: orq historia, orq pausar/reanudar, status one line per story; drift alerts journaled once and shown first |
| 5 | `c370283` | app: closing a worker merges and verifies; one ledger row per integration; no progress alert only for registered workers |
| report | this commit | docs: D4 report |

## `make ci` on `c370283` (clean tree)

`make ci` exited 0.

- Candidate: `/tmp/orq-ci.h6jO2M/bin/orq · 1.0.0 revision=c3702836ee5fd7f38ccad992cdb56fe30caba51e modified=false`.
- `make lint` (staticcheck 2025.1.1 on `gate` and `slice`): clean. `gofmt -l harness`: nothing listed. `go vet ./...`: clean.
- `go test -race ./...`: 23 packages `ok`, 0 `FAIL`. `go test -list` counts 350 top-level tests; D3 had 297, and this branch also carries D3b's.
- `e2e-v05`: **86 pass, 0 fail**.
- `aislamiento`: **14 pass, 0 fail, 0 pending**.

## What the Owner can do now, item by item

**1 · A large story split into child slices; `orq status` shows one line per story.**
- To open a story: `orq historia abrir H --titulo T`. It is stored in `.orq/historias/H.json`. The id may use only `a-z0-9-`, up to 30 characters, and an id that already exists is refused.
- To open a child: `orq slice abrir NN --historia H …`. The child keeps its own card, exam, contract and clone; nothing about those changed. An unknown story is refused before anything is created, and the error says `orq historia abrir H`.
- The story is kept in `Estado.Historia` and also in the closing row of `slices.jsonl` (`Registro.Historia`), so closed children still show.
- `orq status` prints a section like this:
  ```
  historias:
    pagos «Pagos» · S-01 IMPLEMENTANDO · S-02 VERIFICADO · S-03 rechazado
  slices sin historia:
    S-04/a1 «t» IMPLEMENTANDO
  ```
  Children appear in slice order. A closed child shows its result.
- Evidence:
  - `slice.TestHistoriaConTresHijos`: three children, then one closed as rejected, so 2 open and 1 closed. It also checks that a repeated id, a bad id and an unknown story are refused.
  - `cli.TestStatusAgrupaHistoria`: three children through the CLI, and exactly one status line that names `S-01`, `S-02` and `S-03`.

**2 · Pause without destroying anything.**
- `orq historia pausar H` and `orq historia reanudar H` set a flag on the story.
- `orq pausar ["motivo"]` writes `.orq/pausa`; `orq reanudar` removes it. Each of the four writes a journal event: `tipo: pausa`, `resultado: pausada|reanudada`, `detalle: H|todo`.
- `LanzarWorker` checks the pause after the card check and before it reserves anything. A refused launch opens no pane and takes no writer. It does not touch the clone or the phase.
- `orq status` prints `PAUSA: motivo · …` near the top, and `PAUSADA` on the story's line.
- Evidence:
  - `app.TestPausaNiegaLanzarYConservaClones`. With the story paused, the launch is refused naming `orq historia reanudar h`, 0 panes are opened, the clone still exists, and the phase is still `IMPLEMENTANDO` with no writer. With the global pause, it is refused naming `orq reanudar`. After resuming, it launches.
  - `slice.TestPausaHistoriaYGlobal`: the journal holds `pausada:h reanudada:h pausada:todo reanudada:todo`.
  - `cli.TestPausarCLI`.
  - `cli.TestSliceDelOrquestadorSeNiegaEnUnWorker` now also covers `historia pausar|abrir`, `pausar` and `reanudar`.
  - Gate rows in `TestDecisiones…`.

**3 · Drift alerts only; journaled; first in `orq status`.**
- `app.Alertas(ctx)` checks every open slice. It journals each alert once (`tipo: alerta, paso: <tipo>`) and returns them all. `orq status` prints `ALERTAS (N):` as its very first lines, and prints nothing when there are none.

| Alert | Rule |
|---|---|
| `no_arranco` | 60 s (`PlazoArranque`) after the worker's last `lanzamiento/intento`, the journal has no `arranque` ok/ya_estaba after it, or no `contrato` delivered ok |
| `sin_progreso` | slice in IMPLEMENTANDO, the worker started and is still in `sesiones.json`, and its last activity is older than `alertas.sin_progreso_min` (config, default 30, must be > 0). Last activity is the latest of: the start, any journal event for that worker, and the clone HEAD's commit time |
| `fuera_de_alcance` | `Alinear` (reads clone tips) finds files outside the scope and zones |
| `examen_tras_aprobar` | one per `examen` journal event after the card approval |

- Evidence, each with the fake clock:
  - `app.TestWorkerQueNoArrancaSeMarcaEn60s`. The stub's `arrancar` returns "unknown" and the agent is absent. At **59 s: 0 alerts. At 60 s: 1 `no_arranco`** for `worker-claude-s01`. A second call leaves 1 journal event.
  - `app.TestSinAlertasEnCorridaLimpia`: normal launch, a commit in scope, at +5 min **0 alerts and 0 `alerta` events**.
  - `app.TestAlertaSinProgreso`: config 10 min. None at +9 min; `sin_progreso` at +11 min.
  - `app.TestAlertasFueraDeAlcanceYExamen`: `otro.txt` outside `v01.txt` is flagged, and so is the exam event; slice 01 next to it has none.
  - `app.TestWorkerCerradoNoAlertaSinProgreso`: a worker whose session was closed gets no `sin_progreso` an hour later.
  - `cli.TestStatusAlertasArriba`: line 1 is `ALERTAS (1):` and line 2 names `S-01 examen_tras_aprobar … req ATN-9`.

**4 · On a worker's close orq merges and verifies, with one ledger row per integration.**
- The trigger is `orq sesion cerrar worker-<x>-sNN`. Before the pane closes, `app.CerrarWorker` does the following:
  - When the slice is in IMPLEMENTANDO: `Juntar` (team slices), then `Verificar`. With a team, verification waits until the last member with a session closes; that member's row says `verificado: false`.
  - It writes one row: `tipo: integracion, paso: worker, actor, slice, intento, resultado: ok|fallo, datos.sha, detalle: reason`.
  - On failure, the command exits 1 with `… no se cierra: <reason>`, and the pane and session stay.
  - Repeating the command after success adds no row, because the slice is no longer IMPLEMENTANDO.
- `slice.Cerrar` writes `tipo: integracion, paso: base, datos.sha = Integrado` every time it integrates into the base. A failure writes `resultado: fallo` with the reason, and the close stops, as before.
- Evidence:
  - `app.TestCerrarWorkerVerificaYAnota`: VERIFICADO, 1 row ok, `sha == Recibo.Head`; a repeat makes no second row.
  - `app.TestCerrarWorkerFalloNoCierra`: 1 row `fallo` with its reason, and not VERIFICADO.
  - `slice.TestIntegracionBaseAnota`: exactly 1 `base` row, with `sha == reg.SHA`.
  - `cli.TestSesionCerrarWorkerFalloMantienePanel`: a failing exam gives exit 1 and the session is kept; after the fix, it is verified and closed.

Every test above failed before its code existed. The red for `alertas_test.go` was a build failure: I wrote `alertas.go` first and moved it aside to see the red. Its 59 s / 60 s assertions do separate the two cases.

## Design choices made from the code (no design changed)

- **"A worker closes" is `orq sesion cerrar`, not a command run by the worker.**
  - orq run inside a worker clone resolves its project to the clone (`orqdir.Root` reads `git-common-dir`), not to the main `.orq`.
  - `juntar` and `verificar` are orchestrator-only in the gate.
  - So the merge runs on the orchestrator's side, at the one step the orchestrator already takes when a worker reports.
  - It removes the manual `juntar` + `verificar` step; it does not remove the orchestrator's close call.
- **"Started" is read from orq's own journal:** the terminal confirmed the start (`arranque` ok) and the contract was delivered.
  - A worker's own `orq` calls write into its clone, so orq cannot see them; this is the same reason as the first point.
  - "Never started" therefore means the terminal never confirmed the start or the contract never landed. It does not mean the model never produced output.
- **"Blocks with the reason" means the close is refused.** The merge is not done and the pane stays. Exam failures keep `Verificar`'s own rule: BLOQUEADO after 3 failures.
- **The story is not sealed in the contract.** A hand-edited `historia` field in `.orq/slices/NN.json` can move a slice out of a paused story. Pause is a mistake-stopper, like the D3 approval.

## Not done, and why

- **The real exit test from the advice is the Owner's:** one real large story with ≥ 3 children through `~/bin/orq2`.
- **No closing of a story** (`orq historia cerrar`). It was not asked for, and a story with all children closed already shows their results.
- **Nothing runs alerts in the background.** They are computed whenever `orq status` runs, so a worker that never started is flagged on the first `status` after 60 s. A watcher loop belongs with the orchestrator skill or a later step.
- **The Planner/Atena skills do not yet mention stories or alerts.** Only the orchestrator guide (`guias/orquestador.md`) gained two lines.

## Open decisions for the Owner

1. Should a worker's own "done" (from inside its clone) trigger the merge, instead of the orchestrator's `orq sesion cerrar`? That needs orq in the clone to reach the main `.orq`. Today it does not, by design of the gate.
2. Is the default `alertas.sin_progreso_min` of 30 right?
3. Should `orq pausar` also stop `verificar`/`cerrar`, or only launches as now?

## Suggestions outside this row

- `orq metricas` could count alerts per type and story; the journal already has them.
- `orq slice estado` could print the story.

## Review fixes, round 1

Each fix has a test that was seen failing first.

1. **Merge-on-close only for a real session.** `CerrarWorker` now requires the name to be a live session and, on a team slice, a member, before it merges or verifies. A wrong name returns an error (CLI exits 1) and changes nothing: no verify, no ledger row. Test: `TestCerrarWorkerNombreSinSesionNoIntegra` (`worker-nadie-s01`). A repeated close of an already closed slice is still a no-op (the phase check runs first).
2. **`orq status` never hides an open slice.** A slice whose story does not exist is listed under `!! historia desconocida: <id>`. Test: `TestStatusNoOcultaSliceConHistoriaDesconocida`.
3. **One bad story file hides nothing.** `Historias` returns every readable story plus an error naming each bad file; status prints it as `!! historias: no se pudieron leer …` and still lists every slice. Test: `TestStatusUnaHistoriaMalaNoOcultaSlices`.
4. **Alert dedup per slice and kind.** The journal key is slice + kind + actor, without the detail, so out-of-scope is journaled once per slice; `orq status` shows the latest file list. `examen_tras_aprobar` keeps its detail in the key because each exam is its own event. Test: `TestFueraDeAlcanceSeAnotaUnaVezPorSlice`.

Not covered by a test: the team-member check (a wrong name on a team slice).
