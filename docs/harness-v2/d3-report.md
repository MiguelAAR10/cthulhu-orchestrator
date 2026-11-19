# D3 report · Understanding card before code

Branch `v2/d3`, worktree `~/worktrees/cthulhu-d3`, built on `0f03881`. Plan: `docs/harness-v2/d3-plan.md`. Spec: `plan.md` › "Route (Owner, 2026-11-18 late)" D3, and `working/advice/fable-value-2026-11-18.md` §3 D3.

What I did not do:
- Install, push, or write to `~/bin/orq`, `~/.config/orq` or `~/.local/state/orq`.
- Call a real `claude` or `codex`. Every test uses temp `ORQ_*` dirs and a temp `ORQ_CONFIG`.
- Touch `perfil/web*`, the Playwright server code, `caja` or `slice/nucleo.go`.
- Add dependencies.

`internal/gate` has one change: `tarjeta` and `aprobar` join `sliceOrquestador`. `cli/decidir.go` has one added line, the call to `seccionTarjeta`.

## Commits

| Task | Commit | Subject |
|---|---|---|
| plan | `478f618` | docs: D3 plan — understanding card before code |
| 1 | `13e91e3` | slice: understanding card — five short parts parsed and validated |
| 2 | `487dec2` | slice: card frozen with the contract; launch refused until the Owner approves (fix skips) |
| 3 | `fb21681` | cli: orq slice tarjeta and aprobar; status lists cards waiting first |
| 4 | `7c81e61` | decidir: the page shows the approved card and flags exam or zone drift after approval |
| 5 | `ef2ca4e` | metricas: card ready to approved median and exam corrections per 10 slices |
| report | this commit | docs: D3 report |

## `make ci` on `ef2ca4e` (clean tree, exit 0)

- Candidate: `/tmp/orq-ci.yO7RvJ/bin/orq · 1.0.0 revision=ef2ca4efa269a3d1497b3080549897e7e531e40d modified=false`.
- `make lint` (staticcheck 2025.1.1): clean. `gofmt -l harness`: nothing listed. `go vet ./...`: clean.
- `go test -race -count=1 ./...`: every package ok.
  - 297 top-level tests pass, 0 fail, 2 skipped. 36 subtests pass.
  - The 2 skips are the usual ones: `TestPrepararDeps` and herdr's `TestLiveTabSplitClose`.
- `e2e-v05`: **86 pass, 0 fail**.
- `aislamiento`: **14 pass, 0 fail, 0 pending**.

## The card

`orq slice tarjeta NN --archivo card.md` reads a markdown file with five sections, in any order:

```markdown
## Historia     1–3 lines (the story now has its own field, no longer the title)
## Fuera        what will not be done
## Examen       the exam in plain words
## Zonas        paths the work touches, one per line, relative to the repo
## Preguntas    open questions ("ninguna" when there are none)
```

- **Short means:** every section other than Historia has 1–5 lines. No line is longer than 200 characters. The whole card is at most 400 words, about 3 minutes of reading.
- **Refused:** a missing, empty, unknown or repeated section, a zone that is absolute or contains `..`, and loose text outside a section. Each error names the section or the limit it broke.
- **Where it is stored:** the card lives in `Estado.Tarjeta` and also in `Contrato`. So it is sealed in `refs/orq/contrato/NN` along with the exam.
- **Before approval:** it can be rewritten. `orq slice tarjeta NN` with no `--archivo` prints it for the Owner.

## What the Owner can do now, item by item

**1 · A card exists before any worker starts, written by a command and checked by orq.**
- The command is `orq slice tarjeta NN --archivo F`. It runs after `abrir`, because the card belongs to an open slice.
- Evidence:
  - `slice.TestLeerTarjetaCincoPartes`.
  - `slice.TestLeerTarjetaRechaza`: 11 cases, each checking that the error names what is wrong.
  - `cli.TestTarjetaCLIPoneYMuestra`: a card with no Zonas exits 1 and names `Zonas`; a valid card is set, then printed back.

**2 · `orq slice aprobar NN`; the launch refuses an unapproved card unless the route is `fix`; `orq status` lists cards first.**
- Approval writes the journal event `tipo: tarjeta, resultado: aprobada` and seals the contract again. Running `aprobar` a second time prints "ya estaba aprobada" and adds no second stamp.
- `aprobar` refuses in three cases:
  - inside a worker: the gate's orchestrator-only list, plus the CLI's own refusal of that list;
  - in any pane orq opened (`ORQ_ROL` set): it says to run it in the Owner's terminal;
  - when the slice has no card.
- `app.LanzarWorker` calls `Estado.PuedeLanzar()` before it reserves anything. On the slice route, it says either "escríbela con orq slice tarjeta NN" or "aprueba con orq slice aprobar NN". The fix route passes without a card.
- `orq status` prints `tarjetas por aprobar (antes que nada):` just before `esperan al Owner`, oldest first, with both commands on each line.
- Evidence:
  - `app.TestLanzarSinTarjetaAprobadaSeNiega`: refused with no card and with an unapproved card, and no panel is created; it launches once the card is approved.
  - `app.TestLanzarFixSinTarjeta`.
  - `slice.TestTarjetaAprobadaSellaYCongela`: one stamp; the contract in git carries the approval.
  - `slice.TestSinTarjetaAprobadaNoSeLanza`, `cli.TestAprobarCLISella`, `cli.TestStatusTarjetasPrimero`.
  - `cli.TestSliceDelOrquestadorSeNiegaEnUnWorker` now also covers `aprobar` and `tarjeta`. I removed the gate change and saw it fail, then put it back.

**3 · The card is frozen with the contract; at close the page shows it and flags drift with `!!`.**
- Rewriting an approved card is refused ("congelada"). An approval written by hand into `.orq/slices/NN.json` fails `confiable` with `ErrEstadoForjado` naming `tarjeta`.
- Both `PonerTarjeta` and `AprobarTarjeta` check the loaded state against the contract before they seal again, so a forged state is never laundered into the contract.
- On the page:
  - The Historia section uses the card, with source `tarjeta del slice`.
  - A new "Tarjeta" section shows Fuera, Examen, Zonas and Preguntas, and the approval time.
  - `!! examen cambiado tras aprobar la tarjeta (…)` appears for every `examen` journal event after the approval.
  - `!! fuera de las zonas de la tarjeta: F` appears for every file changed from red to the verified commit outside the zones. Zones are matched by path segment, so `total.txt.bak` is outside `total.txt`. Exam files are left out of this check because their change has its own line.
  - A slice-route slice with no approved card gets `!! sin tarjeta aprobada`.
- Evidence: `slice.TestTarjetaForjadaNoEsConfiable`, `cli.TestDecidirMuestraTarjeta` (no `!!` without drift), `cli.TestDecidirTarjetaExamenYZonasTrasAprobar`, `cli.TestDecidirSinTarjetaAvisa`.

**4 · `orq metricas` has two new lines.**
- `tarjeta lista → aprobada: mediana N min (K aprobadas)`. The clock starts at the last `lista` before approval; the `desde` filter applies to the approval.
- `exámenes corregidos tras abrir: N en M slices cerrados (X por 10)`. N counts `examen` events; M counts slices closed in the window.
- Evidence:
  - `slice.TestMetricasTarjetaDesdeDiario`, on a fixture journal:
    - all data: median **4 min** over 3 approved cards (one card was rewritten, one never approved), 2 corrections in 4 closed slices, **5.0 per 10**;
    - with `desde` 10:30: median 6 over 2.
  - `cli.TestMetricasCLITarjeta` checks both printed lines.

Every test above was seen failing before its code existed. All of them run in `make ci`.

## Design choices made from the code (no design changed)

- **The card comes after `abrir`.** Every slice field lives in the state that `abrir` creates. The exam's red commit and the clone exist by then, but no worker has started.
- **"Exam changed after approval" reads the journal** (`examen` events after `Aprobada`), not a fingerprint comparison. `rehuella` rewrites the fingerprint without changing the exam, which would raise a false `!!`.
- **The card has no estimate or risks.** The brief lists five parts; the Design section's longer list (estimate, risks) was not asked for in D3.
- **Owner-only is a mistake-stopper, not identity.** The passphrase was cut. `aprobar` refuses inside worker clones and in any pane orq opened, but an agent that unsets `ORQ_ROL` in a plain shell can still run it.
- **A fix promoted to the slice route needs a card for its next launch.** The work already in its clone stays.
- **Small duplication.** `archivosDe` in `cli/tarjeta.go` repeats 5 lines of `decidir.go`'s exam loop, so that `decidir.go` only gains one line.

## Not done, and why

- **The D3 exit test from the advice is the Owner's:** 10 real stories, approval median ≤ 3 min, exam corrections ≤ 1 in 10. It needs `~/bin/orq2` on real projects.
- **The Planner skill (`orq-planner`) does not yet say to write the card.** The orchestrator guide and skill do. The Planner skill belongs to the planning flow, and I kept the change to the orchestrator's dispatch step.
- **The card leaves no record after close.** `Registro` in `slices.jsonl` does not keep it; only the journal stamps remain.

## Open decisions for the Owner

1. Should `aprobar` be blocked by something stronger than `ORQ_ROL`? The passphrase was cut; a PR approval on published repos is the other option in the Design.
2. Should a card change the slice's `alcance`, which drives `orq alinear`, or stay a separate list as now? Today the two can differ.
3. Should a fix promoted to the slice route skip the card for workers already running?

## Suggestions outside this row

- `orq slice estado` could print one line about the card: missing, waiting since …, or approved.
- Atena's skill could read `tarjetas por aprobar` from `orq status` to tell the Owner when a card is waiting.

## Review fixes, round 1

- **Launch trusts unsealed state:** `LanzarWorker` now runs the contract check (`Confiar`/`confiable`) before `PuedeLanzar`; test `TestLanzarNoConfiaEnAprobacionForjada` (forged `aprobada` → refused, no panel).
- **Route can be relabeled `fix`:** `Ruta` is now sealed in the contract (`CambiarRuta` checks and re-seals), launch refuses a rewritten route and the decision page adds a `!!` notice when state and contract differ; tests `TestLanzarNoConfiaEnRutaRebajadaAFix`, `TestSubirRutaSigueSiendoValida`. Slices opened before this commit as `fix` have no sealed route and will read as forged.
- **Zone `.` turns the zone check off:** `LeerTarjeta` rejects `.`, `./` (and `/`, already absolute) with "cubre todo el repo"; cases added to `TestLeerTarjetaRechaza`.
