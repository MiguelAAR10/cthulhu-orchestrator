# D4 Big stories stay inside: implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** a large story is split into child slices under one story. The Owner sees one status line per story, can pause it (or everything) without destroying anything, and gets alerts only on drift. Closing a worker makes orq merge and verify on its own, and writes a ledger row.

**Architecture:** stories are small JSON files beside the slice states (`.orq/historias/H.json`). A slice points at its story (`Estado.Historia`). Pause is a flag on the story plus a global file `.orq/pausa`. `LanzarWorker` checks both with one added call. Alerts are computed from the journal, the clones and the slice states by `app.Alertas`. Each alert is journaled once (`tipo: alerta`), and `orq status` prints them first. "A worker closes" is `orq sesion cerrar worker-…-sNN`. Before the pane closes, orq runs the existing `Juntar` (team slices) and `Verificar`, and writes one `tipo: integracion` row. A failure keeps the pane open and prints the reason. The integration into the base inside `Cerrar` also writes one ledger row.

**Tech Stack:** Go standard library; existing packages `slice`, `app`, `cli`, `diario`, `config`, fake terminal.

**Spec:** `docs/harness-v2/plan.md` › "Route (Owner, 2026-11-18 late)" D4; `docs/harness-v2/working/advice/fable-value-2026-11-18.md` §3 D4; the D4 brief (Owner items 1–4 and exit tests).

## Global Constraints

- Standard library only; no new dependencies. `internal/arch_test.go` green (imports point down: `cli → app → slice → diario`).
- `internal/gate`: only new orchestrator-only commands added to its list (`historia`, `pausar`, `reanudar`).
- `app/workers.go`: one added check, nothing else (a parallel fixer edits the skill ranking there).
- Tests use temp `ORQ_STATE/ORQ_LIMITS/ORQ_WORKTREES/ORQ_CONFIG` and the fake terminal; nothing calls `claude`, `codex` or `herdr`.
- CLI words in Spanish like the rest of orq (`historia`, `pausar`, `reanudar`); code, commits and docs in English.
- Start deadline: **60 s** (`app.PlazoArranque`). No-progress deadline: config `alertas.sin_progreso_min`, default **30**.

## Review Focus

1. A slice opened with `--historia` that does not exist: refused, the error names the story and says `orq historia abrir`. Covered in Task 1.
2. Pausing a story must not touch the clones, branches or state of its children. Task 2 asserts the worktree dir and phase are unchanged after a refused launch.
3. Alerts must not repeat in the journal every time `orq status` runs. Task 4 calls `Alertas` twice and asserts one `alerta` event.
4. A worker launch that came back "unknown" (the terminal timed out) is the "never started" case. It must be flagged after 60 s, not before. Task 4 tests 59 s → none and 60 s → one.
5. A failed merge or verification on a worker's close must not close the pane, and must not report success. Task 5 asserts exit 1, the pane still listed, and a ledger row with `resultado: fallo` and the reason.

---

### Task 1: Stories and child slices

**Files:**
- Create: `harness/internal/slice/historia.go`, `harness/internal/slice/historia_test.go`
- Modify: `harness/internal/slice/slice.go` (`Estado.Historia`, `Registro.Historia`, `Opciones.Historia`), `harness/internal/slice/nucleo.go` (`Abrir` validates and stores it), `harness/internal/slice/cierre.go` (the closing `Registro` carries it)

**Interfaces — Produces:**
- `type Historia struct { ID, Titulo string; Abierta time.Time; Pausada bool }` (json `id, titulo, abierta, pausada`)
- `func (r Repo) AbrirHistoria(id, titulo string, now time.Time) (*Historia, error)`. The id matches `^[a-z0-9-]{1,30}$`. A repeated id is refused.
- `func (r Repo) LeerHistoria(id string) (*Historia, error)`. Unknown → error containing `orq historia abrir`.
- `func (r Repo) Historias() ([]*Historia, error)`, sorted by id.
- `func (r Repo) Hijos(id string) (abiertos []*Estado, cerrados []Registro, err error)`

- [ ] Test `TestHistoriaConTresHijos`: `AbrirHistoria("pagos","Pagos",now)`. Open slices 01, 02, 03 with `Opciones{Historia:"pagos"}` (`MaxSlices: 4`). Then `Hijos("pagos")` returns 3 open slices with `Historia == "pagos"`. Close 03 as rejected: `Hijos` returns 2 open and 1 closed `Registro{NN:"03", Historia:"pagos"}`. A repeated `AbrirHistoria("pagos")` is an error. `Abrir` with `Historia:"nada"` is an error that contains `orq historia abrir`.
- [ ] Run it, see it fail, implement, see it pass, commit `slice: stories group child slices`.

### Task 2: Pause, per story and global

**Files:**
- Modify: `harness/internal/slice/historia.go` (+ test), `harness/internal/app/workers.go` (one call after `PuedeLanzar`), `harness/internal/app/app_test.go`

**Interfaces — Produces:**
- `func (r Repo) PausarHistoria(id string, pausada bool, now time.Time) error`. Writes the flag and a journal event `tipo: pausa, resultado: pausada|reanudada, detalle: id`.
- `func (r Repo) PausarTodo(pausada bool, motivo string, now time.Time) error`. Writes or removes `.orq/pausa`, with the same journal event and `detalle: "todo"`.
- `func (r Repo) Pausado(e *Estado) error`. nil, or an error that names the pause and the command that lifts it (`orq reanudar` / `orq historia reanudar H`).

- [ ] Test `TestPausaNiegaLanzarYConservaClones` (app). Story `h`, slice 02 under it, card approved. `PausarHistoria("h",true)`, then `LanzarWorker`: the error contains `orq historia reanudar h`, `term.Ops("crear")` is empty, the slice worktree still exists, and the phase is still `IMPLEMENTANDO`. After `PausarHistoria("h",false)`, `PausarTodo(true,"almuerzo")` refuses slice 01 (no story) with `orq reanudar`. After `PausarTodo(false)`, it launches.
- [ ] Fail, implement, pass, commit `app: pause refuses new launches and destroys nothing`.

### Task 3: CLI — `orq historia`, `orq pausar|reanudar`, `--historia`, status grouped

**Files:**
- Create: `harness/internal/cli/historia.go`, `harness/internal/cli/historia_test.go`
- Modify: `harness/internal/cli/cli.go` (routing, usage, status section), `harness/internal/cli/proyecto.go` (`slice abrir --historia`), `harness/internal/gate/reglas.go` (orchestrator-only: `historia abrir|pausar|reanudar`, `pausar`, `reanudar`)

**Interfaces — Consumes:** Task 1 and Task 2. **Produces:** `func runHistoria(args []string, stdout, stderr io.Writer) int`, `func runPausa(pausar bool, args []string, stdout, stderr io.Writer) int`, `func slicesPorHistoria(p proyecto, stdout io.Writer)`.

Status prints, after HALT:
```
historias:
  pagos «Pagos» · S-01 IMPLEMENTANDO · S-02 IMPLEMENTANDO · S-03 rechazado
  pagos-2 «…» PAUSADA · …
slices sueltos:
  S-04/a1 «t» IMPLEMENTANDO
```
When the global pause is on, `PAUSA: motivo · orq reanudar` comes first. Inside a worker clone the CLI refuses these commands, like `runSlice` does.

- [ ] Test `TestStatusAgrupaHistoria`. Run `historia abrir pagos --titulo Pagos`, then `slice abrir 01|02|03 … --historia pagos`. Status has exactly one line containing `S-01`, and that line also contains `pagos «Pagos»`, `S-02` and `S-03`.
- [ ] Test `TestPausarCLI`. `historia pausar pagos` → status line contains `PAUSADA`. `pausar "almuerzo"` → status contains `PAUSA: almuerzo`. `reanudar` → no `PAUSA:`.
- [ ] Gate test row: `orq historia pausar x` and `orq pausar` are orchestrator-only (`TestSliceDelOrquestadorSeNiegaEnUnWorker`-style check in an existing gate test table).
- [ ] Fail, implement, pass, commit `cli: orq historia, orq pausar/reanudar; status shows one line per story`.

### Task 4: Drift alerts

**Files:**
- Create: `harness/internal/app/alertas.go`, `harness/internal/app/alertas_test.go`
- Modify: `harness/internal/config/config.go` (`Alertas{SinProgresoMin int}` json `alertas.sin_progreso_min`, default 30, must be > 0), `harness/internal/cli/cli.go` (alerts printed first in `orq status`)

**Interfaces — Produces:**
- `const PlazoArranque = 60 * time.Second`
- `type Alerta struct { Slice, Tipo, Actor, Detalle string }`. Tipo is `fuera_de_alcance | examen_tras_aprobar | no_arranco | sin_progreso`.
- `func (p Proyecto) Alertas(ctx context.Context) ([]Alerta, error)`. It journals each new alert once (`tipo: alerta`, `paso: Tipo`, `actor`, `slice`, `detalle`) and returns all current alerts.

Rules, for each open slice and its current attempt:
- **no_arranco:** the last `lanzamiento/intento` of a worker is at T. No later `arranque` with `ok|ya_estaba` plus a later `contrato*` with `ok` exists, and `now − T ≥ 60 s`.
- **sin_progreso:** the phase is IMPLEMENTANDO and the worker started. Last activity is the latest of the start event, the worker's journal events and the clone HEAD's commit time. Alert when `now − last ≥ sin_progreso_min`.
- **fuera_de_alcance:** `Alinear(nn).Fuera` is not empty. One alert per slice, with the files in `detalle`.
- **examen_tras_aprobar:** one alert per `examen` journal event of the slice after `Tarjeta.Aprobada`.

- [ ] Test `TestWorkerQueNoArrancaSeMarcaEn60s`. Fake clock at t0. The fake makes `arrancar` return `terminal.ErrDesconocido` and `EstadoFijo` returns Ausente for the worker. At t0+59 s: 0 alerts. At t0+60 s: one `no_arranco` naming `worker-claude-s01`. A second call: still one `alerta` event in the journal.
- [ ] Test `TestSinAlertasEnCorridaLimpia`. A normal launch, then a commit inside scope in the clone. `Alertas` at t0+5 min → empty, and no `alerta` event.
- [ ] Test `TestAlertaSinProgreso`. Config `{"alertas":{"sin_progreso_min":10}}`. A launch at t0, `Alertas` at t0+11 min → `sin_progreso`.
- [ ] Test `TestAlertasFueraDeAlcanceYExamen`. The slice has `Alcance: v01.txt`. A commit of `otro.txt` in the clone → `fuera_de_alcance` with `otro.txt`. An `examen` event after the approval → `examen_tras_aprobar`.
- [ ] Test `TestStatusAlertasArriba` (cli). An exam event journaled after approval. The first line of `orq status` starts with `ALERTAS` and the next line names `S-01`.
- [ ] Fail, implement, pass, commit `app: drift alerts journaled once and shown first in orq status`.

### Task 5: Merge and verify on a worker's close; ledger rows

**Files:**
- Create: `harness/internal/app/integrar.go`, `harness/internal/app/integrar_test.go`
- Modify: `harness/internal/cli/sesion.go` (`cerrar` of a `worker-*-sNN` calls it first), `harness/internal/slice/cierre.go` (ledger row around `integrar`), `harness/internal/slice/historia_test.go` or `sellos_test.go`

**Interfaces — Produces:**
- `func (p Proyecto) CerrarWorker(ctx context.Context, nombre string) (string, error)`. It returns the human report. The slice comes from the name suffix `-sNN`; a closed or missing slice means close only. With phase IMPLEMENTANDO: run `Juntar` if the slice has a team. Then run `Verificar`, unless another team member still has a session. Write one journal row `tipo: integracion, paso: worker, actor: nombre, slice, intento, resultado: ok|fallo, datos.sha, detalle: reason`. On failure, return an error with the reason; the caller keeps the pane.
- In `Cerrar`: one row `tipo: integracion, paso: base, resultado: ok|fallo, datos.sha = e.Integrado, detalle` each time `integrar` runs.

- [ ] Test `TestCerrarWorkerVerificaYAnota` (app). Launch the worker, commit `v01.txt` = `ok` in the clone. `CerrarWorker` → nil, the slice is VERIFICADO, and one `integracion` row with `resultado ok` and `datos.sha == Recibo.Head`.
- [ ] Test `TestCerrarWorkerFalloNoCierra` (app). No fix committed. `CerrarWorker` → an error with the exam failure. One row with `resultado fallo` and a non-empty `detalle`. The slice is not VERIFICADO.
- [ ] Test `TestIntegracionBaseAnota` (slice). The full accepted cycle: exactly one `integracion/base` row with `datos.sha == reg.SHA`.
- [ ] Test `TestSesionCerrarWorkerFalloMantienePanel` (cli, fake driver via `app.NuevoDriver`). A failing exam → exit 1, and the session is still in `sesiones.json`.
- [ ] Fail, implement, pass, commit `app: closing a worker merges and verifies; one ledger row per integration`.

### Task 6: Report

- [ ] `make ci`, read the counts. Write `docs/harness-v2/d4-report.md`, commit `docs: D4 report`.
