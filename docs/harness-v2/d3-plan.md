# D3 · Understanding card before code — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Before any worker starts on a full-lane slice, the Owner reads and approves a short card; the card is frozen with the contract, shown at close with `!!` on drift, and measured in `orq metricas`.

**Architecture:** The card is a new field `Estado.Tarjeta` that is also part of `Contrato`, so it is sealed in `refs/orq/contrato/NN` like the exam. `orq slice tarjeta` parses a five-section markdown file and seals it; `orq slice aprobar` stamps approval in the journal and seals again; `app.LanzarWorker` refuses a slice-route slice without an approved card. The decision page compares exam events and touched files with the approved card. Metrics read the new `tarjeta` journal events.

**Tech Stack:** Go standard library only.

**Spec:** `docs/harness-v2/plan.md` › "Route (Owner, 2026-11-18 late)" D3; `docs/harness-v2/working/advice/fable-value-2026-11-18.md` §3 D3; the D3 brief from the orchestrator (five Owner abilities + exit tests).

## Global Constraints

- Standard library only; no new dependencies; imports point down (`internal/arch_test.go` green).
- `internal/gate`: only add `aprobar` and `tarjeta` to `sliceOrquestador`.
- Do not touch `perfil/web*`, Playwright server code, `caja`. Keep edits to `slice/nucleo.go` and `cli/decidir.go` small (decidir.go: one line calling a new section builder).
- Tests use temp `ORQ_*` dirs; nothing calls a real `claude`/`codex`.
- Every commit: `gofmt -l harness` empty, `go vet ./...`, `go test -race ./...`, `make lint`.
- CLI text in Spanish like the rest of the CLI; code identifiers follow the package's Spanish names; commits in English.

## Decisions (from the code)

- **Card file format** (`--archivo card.md`): sections `## Historia`, `## Fuera`, `## Examen`, `## Zonas`, `## Preguntas` (case-insensitive). Before the first section only blank lines and one `# title` line are allowed. A leading `- ` or `* ` on a line is dropped.
- **Short:** Historia 1–3 lines; every other section 1–5 lines; each line ≤ 200 characters; whole card ≤ 400 words (≈ 3 min). Zonas are relative paths without `..`. Missing, empty, unknown or repeated section → error naming it.
- **Who writes:** planner/orchestrator via `orq slice tarjeta NN --archivo F` after `abrir` (the card needs the slice). Rewriting is allowed until approval; after approval it is refused (frozen).
- **Approval:** `orq slice aprobar NN`. Refused inside a worker (gate list + CLI check already applied to that list) and refused when `ORQ_ROL` is set (every pane orq opens has it), so it runs from the Owner's own terminal. This is a mistake-stopper, not identity.
- **Journal:** `tipo: tarjeta`, `resultado: lista | aprobada`, `slice`, `intento`, `ruta`. A repeated `aprobar` is a no-op with no second stamp.
- **Launch:** `LanzarWorker` refuses unless `e.RutaEfectiva() == fix` or the card is approved. A fix promoted to slice needs a card for its next launch.
- **Page:** the Historia section uses the card's Historia when there is a card (source `tarjeta del slice`), else the D1 behaviour. New section "Tarjeta" after Historia: Fuera, Examen, Zonas, Preguntas, approval time; `!!` for every `examen` journal event after approval; `!!` for every file changed red→verified outside the card's Zonas; `!!` when a slice-route slice has no approved card; fix route without card: one plain line.
- **Metrics:** `TarjetaMinMediana` over approved cards (last `lista` before the `aprobada`, filtered by `desde` on the approval), `TarjetasAprobadas`; `CorreccionesExamen` = `examen` events since `desde`; `CorreccionesPor10` = `10 × CorreccionesExamen / (Aceptados + Rechazados)` (0 when none closed).
- **Status:** `tarjetas por aprobar` printed before `esperan al Owner`, oldest `Lista` first.

## Review Focus

1. A card with the five headings but a 6-line story or a 900-word section → refused with the section named (test in Task 1).
2. Rewriting an approved card (to sneak a new exam or zones past the Owner) → refused (Task 2).
3. Hand-editing `.orq/slices/NN.json` to set `Aprobada` → `confiable` refuses at close because the card is in the contract (Task 2 test: forged state differs on `tarjeta`).
4. `aprobar` run twice → one journal stamp (Task 2).
5. Zone `src/` vs file `src2/x.go` → outside (prefix by path segment, reuse `dentro`) (Task 4).

---

### Task 1: Card parsing and validation

**Files:**
- Create: `harness/internal/slice/tarjeta.go`, `harness/internal/slice/tarjeta_test.go`

**Interfaces:**
- Produces:
  - `type Tarjeta struct { Historia, Fuera, Examen, Zonas, Preguntas []string; Lista, Aprobada time.Time }` (json `historia, fuera, examen, zonas, preguntas, lista, aprobada,omitempty`)
  - `func LeerTarjeta(texto string) (Tarjeta, error)`
  - `func (t *Tarjeta) EstaAprobada() bool` (`!t.Aprobada.IsZero()`)

- [ ] Step 1: tests `TestLeerTarjetaCincoPartes` (valid card → each field's lines, `- ` stripped, `# title` ignored), `TestLeerTarjetaRechaza` table: missing `Zonas` → error contains `Zonas`; empty `Preguntas` → `Preguntas`; Historia 4 lines → `Historia` and `3`; a 201-char line → `200`; 401 words → `400`; unknown `## Riesgos` → `Riesgos`; repeated section; zone `../x` and `/abs` → `Zonas`; text before first section → error.
- [ ] Step 2: run `go test ./internal/slice -run TestLeerTarjeta` → build failure.
- [ ] Step 3: implement.
- [ ] Step 4: pass. Commit `slice: understanding card — five short parts parsed and validated`.

### Task 2: Set, approve, freeze; launch refused without approval

**Files:**
- Modify: `harness/internal/slice/slice.go` (field `Tarjeta *Tarjeta \`json:"tarjeta,omitempty"\`` in `Estado`), `harness/internal/slice/contrato.go` (same field in `Contrato`, copied in `contratoDe`), `harness/internal/slice/tarjeta.go`
- Modify: `harness/internal/app/workers.go` (one check after Load), `harness/internal/app/app_test.go` (helper approves a card for slice 01)
- Modify: `harness/internal/gate/reglas.go` (add `aprobar`, `tarjeta`)
- Test: `harness/internal/slice/tarjeta_test.go`, `harness/internal/app/app_test.go`

**Interfaces:**
- `func (r Repo) PonerTarjeta(nn string, t Tarjeta, now time.Time) (*Estado, error)` — Lista = now (UTC, s), seals contract, journals `lista`; refused when approved.
- `func (r Repo) AprobarTarjeta(nn string, now time.Time) (e *Estado, ya bool, err error)` — refused without card; `ya=true` and no stamp when already approved.
- `func (e *Estado) PuedeLanzar() error` — nil for fix route or approved card; else error naming `orq slice tarjeta NN --archivo` / `orq slice aprobar NN`.

- [ ] Step 1: tests
  - `TestTarjetaAprobadaSellaYCongela`: open slice, `PonerTarjeta` → journal has `tarjeta/lista`; `AprobarTarjeta` → `tarjeta/aprobada` stamp with slice NN; second `AprobarTarjeta` → `ya`, still 1 stamp; `PonerTarjeta` after approval → error contains `congelada`; the git contract (`leerBlob(refContrato)`) carries `Tarjeta.Aprobada`.
  - `TestTarjetaForjadaNoEsConfiable`: write `Aprobada` into the state file by hand → `r.confiable(e)` errors with `ErrEstadoForjado` naming `tarjeta`.
  - `TestAprobarSinTarjeta`: error contains `orq slice tarjeta`.
  - app `TestLanzarSinTarjetaAprobadaSeNiega`: slice 02 (slice route) with no card → error contains `aprobar`, no `crear` op; card set but not approved → refused; approved → launched. `TestLanzarFixSinTarjeta`: slice opened with `Ruta: fix` launches with no card.
  - gate: `SliceSoloOrquestador("aprobar")` and `("tarjeta")` true (add to the existing gate test table if there is one, else in `tarjeta_test.go` of cli).
- [ ] Step 2: see them fail.
- [ ] Step 3: implement; update `proyectoDePrueba` to set and approve a card for slice 01 so the existing launch tests keep their meaning.
- [ ] Step 4: pass, full `go test -race ./...`. Commit `slice: card frozen with the contract; launch refused until the Owner approves (fix skips)`.

### Task 3: CLI `orq slice tarjeta`, `orq slice aprobar`, status

**Files:**
- Create: `harness/internal/cli/tarjeta.go`, `harness/internal/cli/tarjeta_test.go`
- Modify: `harness/internal/cli/proyecto.go` (two `case` lines + help list), `harness/internal/cli/cli.go` (usage lines; `tarjetasPorAprobar` call before `esperanOwner`), `harness/internal/cli/guias/orquestador.md` (one line: card → Owner approves → lanzar)

**Interfaces:**
- `func runTarjeta(p proyecto, nn string, args []string, stdout, stderr io.Writer) int` — `[NN] --archivo F` sets; `[NN]` alone prints the card.
- `func runAprobar(p proyecto, nn string, args []string, stdout, stderr io.Writer) int`
- `func tarjetasPorAprobar(p proyecto, now time.Time, stdout io.Writer)`

- [ ] Step 1: tests `TestTarjetaCLIPoneYMuestra` (exit 0, prints `tarjeta lista` and `orq slice aprobar 01`; bad file exit 1 with the section named), `TestAprobarCLISella` (exit 0, journal stamp; with `ORQ_ROL=orquestador` exit 1 with `terminal del Owner`), `TestStatusTarjetasPrimero` (`orq status` output: `tarjetas por aprobar` index < `esperan al Owner` index, line has `orq slice aprobar 01`).
- [ ] Step 2: fail. Step 3: implement. Step 4: pass. Commit `cli: orq slice tarjeta and aprobar; status lists cards waiting first`.

### Task 4: Decision page shows the card and drift

**Files:**
- Modify: `harness/internal/slice/pagina.go` (fields `ExamenTrasAprobar []diario.Evento`, `FueraDeTarjeta []string`), `harness/internal/app/pagina.go` (story from the card), `harness/internal/cli/decidir.go` (one line: `out = append(out, seccionTarjeta(pd))` after Historia)
- Modify: `harness/internal/cli/tarjeta.go` (`func seccionTarjeta(pd *app.PaginaDecision) seccion`)
- Test: `harness/internal/cli/decidir_test.go` or `tarjeta_test.go`

- [ ] Step 1: tests `TestDecidirMuestraTarjeta` (approved card, verified slice: section `== Tarjeta (tarjeta del slice · aprobada …)`, Fuera/Examen/Zonas/Preguntas lines, Historia section shows the card's lines with source `tarjeta del slice`, no `!!` in the Tarjeta section) and `TestDecidirTarjetaExamenCambiadoTrasAprobar` (approve, then `SumarExamen`, verify → `!! examen cambiado tras aprobar la tarjeta`; a file outside Zonas → `!! fuera de las zonas de la tarjeta: x`; `src2/` not inside zone `src`).
- [ ] Step 2: fail. Step 3: implement. Step 4: pass. Commit `decidir: the page shows the approved card and flags exam or zone drift after approval`.

### Task 5: Metrics

**Files:**
- Modify: `harness/internal/slice/metricas.go`, `harness/internal/cli/metricas.go`
- Test: `harness/internal/slice/tarjeta_test.go` (fixture journal), `harness/internal/cli/tarjeta_test.go`

- [ ] Step 1: `TestMetricasTarjetaDesdeDiario`: fixture journal — S-01 lista 10:00, lista 10:20 (rewritten), aprobada 10:24 (→ 4 min); S-02 lista 11:00 aprobada 11:10 (10); S-03 lista 12:00 aprobada 12:02 (2); S-04 lista only (not counted); 2 `examen` events; `slices.jsonl` with 4 closed → median **4 min**, 3 aprobadas, 2 corrections, **5.0 per 10**. `desde` after S-01's approval → median 6 (2 aprobadas). CLI `TestMetricasCLITarjeta` prints `tarjeta lista → aprobada: mediana 4 min (3 aprobadas)` and `exámenes corregidos tras abrir: 2 en 4 slices cerrados (5.0 por 10)`.
- [ ] Step 2: fail. Step 3: implement. Step 4: pass. Commit `metricas: card ready to approved median and exam corrections per 10 slices`.

### Task 6: Verify and report

- [ ] `make ci` from the repo root; read the counts.
- [ ] Write `docs/harness-v2/d3-report.md` (commits, counts, each exit item with its test, not done, open decisions). Commit.
