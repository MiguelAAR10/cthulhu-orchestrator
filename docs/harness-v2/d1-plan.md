# D1 · Decision page and timestamps · Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:executing-plans (inline, one builder). Steps use checkbox (`- [ ]`) syntax.

**Goal:** the Owner decides on a verified slice from one page, sees which slices wait for him, and orq measures verified → decided.

**Architecture:** the slice package stamps the journal (`.orq/diario.jsonl`) at verify, exam change, retry and decision, and assembles the page data from what it already holds (state, receipt, `Alinear`, journal). `app` adds the story from the channel. `cli` renders the page as text and, with `--html`, as one self-contained file. `Metricas` reads the journal.

**Tech Stack:** Go standard library only (`html/template` for the HTML file).

**Spec:** `docs/harness-v2/plan.md` › "Route (Owner, 2026-11-18 late)", D1; `docs/harness-v2/working/advice/fable-value-2026-11-18.md` §3 D1.

## Global Constraints

- Standard library only; no new packages; `internal/gate` and `internal/sandbox` untouched (frozen).
- Imports point down (`internal/arch_test.go`): cli → app → slice/diario/canal.
- Tests use temp `ORQ_STATE/ORQ_LIMITS/ORQ_WORKTREES` and a temp `ORQ_CONFIG`; nothing calls a real `claude` or `codex`.
- CLI output stays in Spanish like the rest of orq; code identifiers follow the package's existing language.
- Every commit: `gofmt -l harness` empty, `go vet ./...`, `go test -race ./...`, `make lint`.

## Design decisions taken from the code

- **Command name:** `orq slice decidir [NN] [--html RUTA]` (`orq slice --help` lists Spanish verbs; `--slice NN` keeps working). Not added to the gate's orchestrator-only list: the gate is frozen, and the command only reads state and fetches the clone tip into `refs/orq/…`, as `orq alinear` already does.
- **Stamps** (one journal line each, `TS` = the operation's `now`):
  - worker finished: `verificacion` event gains `datos.terminado` = committer time of the tip handed to verification (the worker's last commit; after `juntar`, the merge).
  - verified: the existing `verificacion` event with `resultado: paso`.
  - decided: new `tipo: decision`, `resultado: aceptado|rechazado`, `detalle: motivo`. Written once: on accept only when the slice leaves `VERIFICADO` (a resumed close does not stamp again); on every reject.
  - exam changed: new `tipo: examen` in `SumarExamen`, `detalle: aprobado`, `datos.archivos`, `datos.base`.
  - new attempt: new `tipo: intento` in `Reintentar`, `intento` = the new one, `detalle: cambio`, `datos.causa: worker|examen`. `orq slice reintentar` requires `--causa worker|examen`.
- **Demo field:** none exists. `Estado.Demo []string`, set by `orq slice abrir --demo "paso; paso; paso"` (split on `;`, at most 3 kept).
- **Story:** the latest `historia` message in the channel, first 3 non-empty lines; otherwise the slice title. The page names which.
- **Exam changed:** highlighted when `Estado.SHAExamen != ""` (state) and/or there are `examen` journal events since `Abierto`; a receipt note saying the runner changed and was accepted is highlighted too.
- **Metric:** per slice, the latest `verificacion paso` starts the clock; a `decision` stops it (counted); `VERIFICADO → IMPLEMENTANDO` (reabrir) or `CERRADO` without a decision stops it uncounted. A clock still running and older than 24 h at `now` counts as undecided. Median of the counted durations, in minutes.

## Review Focus

1. A slice not in `VERIFICADO` → `decidir` refuses with its phase, exit 1 (Task 3 test).
2. A repeated `cerrar --aceptado` after an interrupted integration does not stamp a second decision (Task 1 test).
3. A slice re-verified after `reabrir` measures from the last verification, not the first (Task 2 test).
4. HTML escapes story and file names (a `<script>` in the title appears escaped) (Task 3 test).
5. No scope declared → the page says nothing can be flagged instead of listing every file as outside (Task 3 test).

---

### Task 1: stamps in the journal and `--causa` on retry

**Files:** Modify `harness/internal/slice/nucleo.go` (Verificar, Reintentar, SumarExamen), `harness/internal/slice/cierre.go` (Cerrar), `harness/internal/cli/proyecto.go` (reintentar), `validacion/e2e-v05.sh:47`, `harness/internal/cli/guias/orquestador.md`, `harness/internal/cli/orqskills/orq-orchestrator/SKILL.md`. Test: `harness/internal/slice/sellos_test.go`, `harness/internal/slice/slice_test.go` (callers).

**Interfaces — Produces:** `func (r Repo) Reintentar(nn, cambio, causa string, decisionOwner bool) (*Estado, error)`; `const CausaWorker = "worker"`, `CausaExamen = "examen"`; journal `tipo` values `decision`, `examen`, `intento`.

- [ ] Test `TestSellosDelCiclo`: open, commit green, `Verificar(t1)`, `Cerrar(accept, t2)`; journal has one `verificacion` with `resultado paso`, `TS == t1`, `datos.terminado` parseable RFC3339; exactly one `decision` with `resultado aceptado`, `TS == t2`.
- [ ] Test `TestRechazoSellaMotivo`: reject with `"no es lo pedido"` → one `decision`, `resultado rechazado`, `detalle "no es lo pedido"`.
- [ ] Test `TestCierreRetomadoNoSellaDosVeces`: `ORQ_FALLA_EN=integrar` pattern from `TestCierreInterrumpidoSeRetoma` (subprocess) or a state left in `INTEGRANDO`; after the second `Cerrar`, still one `decision`.
- [ ] Test `TestReintentoAnotaCausa`: `Reintentar("", "x", "", false)` and `"otra"` fail naming `worker|examen`; `Reintentar("", "x", CausaExamen, false)` → one `intento` event with `intento 2`, `datos.causa "examen"`.
- [ ] Test `TestSumarExamenAnota`: add `tests/extra.sh` via `SumarExamen(..., "req ATN-1")` → one `examen` event, `detalle "req ATN-1"`.
- [ ] Run, see them fail. Implement. Update the callers in `slice_test.go`, the CLI flag (`--causa`, required, usage line), e2e line 47 (`--causa worker`), guide and skill lines. Green. Commit `slice: journal stamps for finished, verified, decided, exam change and retry cause`.

### Task 2: `orq metricas` — median verified → decided and undecided after 24 h

**Files:** Modify `harness/internal/slice/metricas.go`, `harness/internal/cli/metricas.go`. Test: `harness/internal/slice/sellos_test.go`, `harness/internal/cli/decidir_test.go`.

**Interfaces — Produces:** `func (r Repo) Metricas(desde, now time.Time) Metricas`; new fields `DecisionMinMediana float64`, `Decididos int`, `SinDecidir24h int`, `IntentosPorCausa map[string]int`.

- [ ] Test `TestMetricasDecisionDesdeDiario` on a fixture journal written with `diario.Anotar`: slices 01 (verified 10:00, decided 10:06), 02 (10:00 → 10:20), 03 (10:00 → 10:10), 04 (verified 10:00, reabrir 10:30, verified 11:00, decided 11:04), 05 verified 2 days before `now`, 06 verified 1 h before `now`; `intento` events with causa worker ×1, examen ×2. Assert `Decididos 4`, `DecisionMinMediana 8` (median of 4, 6, 10, 20), `SinDecidir24h 1`, `IntentosPorCausa {worker:1, examen:2}`; with `desde` after 01's decision, `Decididos 3`.
- [ ] Test `TestMetricasCLIImprimeMediana` (cli): same journal in a `repoCLI` project → `orq metricas` prints `verificado → decisión: mediana 8 min (4 decididos)` and `sin decidir tras 24 h: 1`.
- [ ] Fail, implement, update the two callers of `Metricas`, green, commit `metricas: median minutes verified to decided and undecided after 24 h, from the journal`.

### Task 3: `orq slice decidir` page, text and `--html`

**Files:** Create `harness/internal/slice/pagina.go`, `harness/internal/app/pagina.go`, `harness/internal/cli/decidir.go`. Modify `harness/internal/slice/slice.go` (Estado.Demo, Opciones.Demo), `nucleo.go` (Abrir stores Demo), `harness/internal/cli/proyecto.go` (`--demo`, `decidir` case, help line), `harness/internal/cli/cli.go` (usage). Test: `harness/internal/cli/decidir_test.go`.

**Interfaces — Produces:**
- `type Pagina struct { Estado *Estado; Alineacion *Alineacion; Examen []diario.Evento }`; `func (r Repo) Pagina(nn string) (*Pagina, error)` — error unless `VERIFICADO`.
- `type PaginaDecision struct { *slice.Pagina; Historia []string; FuenteHistoria string }`; `func (p Proyecto) PaginaDecision(nn string) (*PaginaDecision, error)`.
- cli: `func seccionesDecision(pd *app.PaginaDecision, now time.Time) []seccion`, `type seccion struct{ Titulo, Fuente string; Lineas []linea }`, `type linea struct{ Texto string; Alerta bool }`; text and HTML render the same sections. Alert lines start with `!!` in text and carry `class="alerta"` in HTML.

- [ ] Test `TestDecidirMuestraCadaCampoConSuFuente` (cli, `sliceVerde`-style with `--alcance total.txt --demo "abre total.txt; mira 170; listo"` and an extra out-of-scope commit `notas.txt`): output contains `S-01/a1`, `Historia` with source `título del slice`, `1 ejecutadas, 0 omitidas, 0 fallidas`, `recibo`, `cmd: sh run.sh · exit 0`, `planeado: total.txt`, `!! notas.txt fuera del alcance`, `examen: sin cambios desde que se abrió`, `1. abre total.txt`, `orq slice cerrar --slice 01 --aceptado`, `orq slice cerrar --slice 01 --rechazado "motivo"`. Each section header names its source file or command.
- [ ] Test `TestDecidirResaltaExamenCambiado`: before verifying, commit `tests/extra.sh` in the clone and run `orq slice examen tests/extra.sh --aprobado "req ATN-7"` → page has a line starting `!! examen cambiado tras abrir` containing `tests/extra.sh` and `req ATN-7`.
- [ ] Test `TestDecidirSinDemoNiAlcance`: plain `sliceVerde` → `sin demo registrada` and `sin alcance declarado: nada que marcar fuera`.
- [ ] Test `TestDecidirHTML`: title `<script>x</script>` via a historia message in the channel; `--html out.html` writes one file with `<!doctype html>`, `class="alerta"` when applicable, `&lt;script&gt;`, no `<link` or `src=`; stdout says the path. Source of the story says `canal: historia`.
- [ ] Test `TestDecidirSoloVerificado`: an open, unverified slice → exit 1, stderr names `IMPLEMENTANDO`.
- [ ] Fail, implement, green, commit `slice: orq slice decidir, one page to accept or reject a verified slice`.

### Task 4: `orq status` lists verified slices waiting for the Owner

**Files:** Modify `harness/internal/slice/pagina.go` (`func (r Repo) Esperando() ([]*Estado, error)`: `VERIFICADO` slices, oldest receipt first), `harness/internal/cli/cli.go` (runStatus). Test: `harness/internal/slice/sellos_test.go`, `harness/internal/cli/decidir_test.go`.

- [ ] Test `TestEsperandoOrdenaPorAntiguedad`: two verified slices with receipts at t0+1h and t0 → order `02, 01` when 02 is older.
- [ ] Test `TestStatusListaVerificados` (cli): after `sliceVerde`, `orq status` prints `esperan al Owner (verificados, el más antiguo primero):` and a line with `S-01/a1`, `hace`, and `orq slice decidir 01`; with none open: `esperan al Owner: ninguno`.
- [ ] Fail, implement (section printed after HALT, before the machine block, only inside a project), green, commit `status: list verified slices waiting for the Owner with their age`.

### Task 5: verification and report

- [ ] `make ci`, read the counts. Write `docs/harness-v2/d1-report.md` (commits, counts, each exit-test item with evidence, not done, open decisions). Commit `docs: D1 report`.
