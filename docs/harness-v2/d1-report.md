# D1 report · Decision page and timestamps

Branch `v2/build`, worktree `~/worktrees/cthulhu-build`. Plan: `docs/harness-v2/d1-plan.md`. Spec: `plan.md` › "Route (Owner, 2026-11-18 late)", D1, and `working/advice/fable-value-2026-11-18.md` §3 D1.

What I did not do: install anything, push, or write to `~/bin/orq`, `~/.config/orq` or `~/.local/state/orq`. Every test used temp `ORQ_*` dirs and a temp `ORQ_CONFIG`. Nothing called a real `claude` or `codex`. I did not touch `internal/gate` or `internal/sandbox`. I added no new packages and no new dependencies.

## Commits

| Task | Commit | Subject |
|---|---|---|
| plan | `22fbc44` | docs: D1 plan — decision page and timestamps |
| 1 | `719c100` | slice: journal stamps for finished, verified, decided, exam change and retry cause |
| 2 | `e495267` | metricas: median minutes verified to decided and undecided after 24 h, from the journal |
| 3 | `e845939` | slice: orq slice decidir, one page to accept or reject a verified slice |
| 4 | `85651d7` | status: list verified slices waiting for the Owner with their age |
| report | this commit | docs: D1 report |

## `make ci` on `85651d7` (clean tree, exit 0)

- Candidate: `/tmp/orq-ci.3u66LU/bin/orq · 1.0.0 revision=85651d72840691dd5776af6b6ec01ff09642a04e modified=false`.
- `make lint` (staticcheck 2025.1.1): clean. `gofmt -l harness`: nothing listed. `go vet ./...`: clean.
- `go test -race ./...`: all 22 packages ok.
  - 205 top-level tests pass, 0 fail, 2 skipped. 23 subtests pass.
  - The 2 skips are the same as in step 1b: `TestPrepararDeps` and herdr `TestLiveTabSplitClose`.
  - That is 23 more top-level tests than step 1b's 182: the 23 tests below.
- `e2e-v05`: **86 pass, 0 fail**. `aislamiento`: **14 pass, 0 fail, 0 pending**.

## Exit test, item by item

**1 · `orq slice decidir [NN] [--html RUTA]` prints one page for a verified slice.** I picked the name `decidir` because `orq slice --help` lists Spanish verbs. `--slice NN` also works.

Each section header names where its data comes from:

| Section | Source | What it shows |
|---|---|---|
| Historia | the latest `historia` in the channel, or the slice title | up to 3 lines |
| Pruebas | the receipt, with the runner's count | how many tests ran, were skipped and failed, and the result; `!!` when nothing was counted |
| Archivos | the scope from `abrir` and the team's zones, compared with the git diff from the red commit to the tip | `!!` on every file outside the scope; says so when no scope was declared |
| Examen | the journal plus the slice state | `!!` when the exam was added or corrected after opening, with its files and who approved it; `!!` when the receipt says a runner change was accepted |
| Recibo | `refs/orq/recibo/NN`: the sealed receipt, checked against the state as at close | command, exit code, commit, full output |
| Cómo verlo funcionar | `orq slice abrir --demo` | numbered steps, or `sin demo registrada` |
| Decide | — | `orq slice cerrar --slice NN --aceptado` and `--rechazado "motivo"`, with the slice number filled in |

- `--html` writes the same sections as one self-contained file: no links, no scripts, escaped by `html/template`, with a dark mode.
- Evidence, all in `internal/cli/decidir_test.go`, on throwaway repos:
  - `TestDecidirMuestraCadaCampoConSuFuente` checks every header with its source and every field. It also checks that a file outside the scope gets `!!` and the file inside it does not.
  - `TestDecidirResaltaExamenCambiado`: an exam added after opening shows `!! examen cambiado tras abrir … tests/extra.sh · aprobado: req ATN-7`.
  - `TestDecidirSinDemoNiAlcance`, `TestDecidirHTML` (escaping, self-contained, story taken from the channel and cut to 3 lines), `TestDecidirHTMLResalta` and `TestDecidirSoloVerificado` (an unverified slice: exit 1, and the message names `IMPLEMENTANDO`).
  - Seen red: all six failed before the command existed.

**2 · `orq status` lists verified slices waiting for the Owner, oldest first, with their age.**
- The new section comes after HALT. Each line reads `S-NN/aK «título» hace 2h05m · orq slice decidir NN`. With none waiting it prints `esperan al Owner: ninguno`.
- Age is counted from the receipt's time.
- Evidence: `TestStatusListaVerificados` (cli) and `TestEsperandoOrdenaPorAntiguedad` (slice). Both were seen red.

**3 · The journal has three stamps per slice and a cause for every new attempt. `orq metricas` prints the median and the undecided count.** All of these go to `.orq/diario.jsonl`, stamped with the operation's own time:

| What | How it is recorded |
|---|---|
| worker finished | `verificacion.datos.terminado`: the commit time of the tip that was verified |
| verified | `verificacion` with `resultado: paso` |
| decided | new event `tipo: decision`, `resultado aceptado\|rechazado`, `detalle` = the reason |
| exam changed | new event `tipo: examen` (from `orq slice examen`), with `detalle` = who approved it |
| new attempt | new event `tipo: intento` with `datos.causa`; `orq slice reintentar` now requires `--causa worker\|examen` |

- An accepted slice is stamped only when it leaves VERIFICADO, so resuming a close that was cut off does not stamp a second decision.
- `orq metricas` adds three lines, all read from the journal: `verificado → decisión: mediana N min (K decididos)`, `sin decidir tras 24 h: N` and `intentos nuevos por causa: worker N · examen M`.
- Evidence:
  - `TestSellosDelCiclo`, `TestRechazoSellaMotivo`, `TestCierreRetomadoNoSellaDosVeces`, `TestReintentoAnotaCausa` and `TestSumarExamenAnota`. Their first red was a build error, so I also forced `anotarDecision` to do nothing: the three decision tests then failed, and passed again once restored.
  - `TestMetricasDecisionDesdeDiario` uses a fixture journal with 6 slices. It covers one re-verified after `reabrir`, one whose first verification failed, one more than 24 h old and one less than 24 h old. Result: median **8 min** over 4 decisions, **1** undecided after 24 h, causes 1 worker and 2 exam. The `desde` filter is checked too.
  - `TestMetricasCLIImprimeMediana` checks the printed lines.

## Design choices made from the code (no design was changed)

- **The demo field did not exist.** I added `Estado.Demo`, set with `orq slice abrir --demo "paso; paso; paso"`; only the first 3 steps are kept.
- **The story has no dedicated slice field.** The page uses the latest `historia` message in the channel. Without one, it falls back to the title, and the header says which source it used. `cerrar` clears the channel, so two slices open at once from different stories would show the same (latest) story.
- **"Worker finished"** is the commit time of the tip that was verified. For a team slice, that tip is the orchestrator's merge from `juntar`, not each worker's last commit. A true "worker closed" event arrives with D4 (orq merges on a worker's close).
- **The metric clock** starts at the last verification that passed. A decision stops it and counts. `reabrir`, or a close with no decision stamp (data from before D1), stops it without counting.
- **`decidir` is not on the gate's orchestrator-only list,** because the gate is frozen. It only reads, and fetches the clone tip into `refs/orq/…`, as `orq alinear` already does.

## Not done

- The real 5-slice check with `~/bin/orq2` is the Owner's.
- I measured nothing on real data: the journal has had these stamps only since this build.
- Slices verified with orq 1.0.0 have no `decision` events, so they never enter the median.

## Open decisions for the Owner

1. `--causa` values are Spanish, `worker|examen`, like the rest of the CLI; the spec wrote `worker` or `exam`. Say if you want the English value.
2. Should the story become its own slice field, set at `abrir`, instead of being read from the channel? I would decide this in D3, where the understanding card needs it anyway.
3. Is a team slice's "worker finished" good enough as the merge time until D4?

## Suggestions outside this row

- The help text of `orq slice cerrar --aceptado` says "el Owner aceptó la demo". It could point at `orq slice decidir` first.
- `orq slice verificar` still prints "falta la demo del Owner". It could print `orq slice decidir NN` instead.

## Review fixes

Fixes for `reviews/d1-review.md`, each repro a test seen failing first:

1. `metricas` keeps the first `paso` while the slice stays VERIFICADO (`TestMetricasReverificarNoReiniciaElReloj`).
2. The clock stops on any `De=VERIFICADO` transition except to INTEGRANDO, so `SinDecidir24h` agrees with `orq status` (`TestMetricasBloqueoDetieneElReloj`).
3. The page's "Archivos" diffs red→`rec.Head`; if the clone tip differs, `!!` names both commits (`TestPaginaArchivosSonDelCommitVerificado`).
4. The story uses the `historia` whose attempt is this slice's; without a link and with more than one slice open the page raises `!! story may belong to another slice` (`TestDecidirHistoriaVinculadaYAviso`, `...SinVinculoConVariosAbiertosAvisa`).
5. The `verificacion` stamp is `now` plus the time the run took (`TestVerificacionSelloDespuesDeLaCorrida`); `TestSellosDelCiclo` now allows that drift.
6. The page computes `Fuera` for `rec.Head` itself with the same rule `Alinear` uses (`Estado.fueraDeAlcance`), so a file the verified commit added out of scope stays flagged after a later clone commit deletes it (`TestPaginaFueraEsDelCommitVerificado`).
6. Hints in `slice/slice.go` and `app/relevo.go` include `--causa worker|examen`.

`gofmt -l harness` empty, `go vet ./...`, `go test -race ./...` and `make ci` (exit 0) pass. `internal/gate` and `internal/sandbox` untouched.

## Close (orchestrator, 2026-11-19)
Integrated as 2d4f5b8 after 2 review rounds (budget reached; `reviews/d1-review.md`, `reviews/d1-review-2.md`). Last fix checked by the orchestrator: `TestPaginaFueraEsDelCommitVerificado` fails on the pre-fix `pagina.go`/`equipo.go` and passes on 2d4f5b8. `make ci` on 2d4f5b8 with the candidate binary (`modified=false`): exit 0, e2e 86/0, aislamiento 14/0.
Known limit: the text notice built by `Alinear` ("N file(s) outside…") still counts on the clone tip, not on `rec.Head`; the page's own `!!` list is computed on `rec.Head`.
Owner check still pending: decide 5 real slices from the page alone with `~/bin/orq2`.
Defaults taken (Owner may change): retry cause value `examen`; story becomes its own field in D3; team "worker finished" = merge time until D4.
