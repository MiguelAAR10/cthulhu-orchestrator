# Step 1 · Finish B5 + local CI — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** B5 (worker clones, bundles, sandboxed exam, contract in git) reaches a tested state: every suite green, the gate sees clones, and a worker cannot run orchestrator/Owner-only slice commands.

**Architecture:** No new subsystem. The gate learns the clone marker that B5 already writes; the orchestrator-only command list grows; `cerrar` honours the existing `ORQ_SIN_DOC`; the seven failing slice tests are fixed by deciding each behaviour below; `make ci` runs every suite.

**Tech Stack:** Go 1.25, standard library only; bash validation scripts; bubblewrap 0.9.

**Spec:** `docs/harness-v2/plan.md`, section "Design (Owner approved 2026-11-18)", roadmap row 1; Acceptance 2 and 6.

## Global Constraints
- Branch `v2/build` in `~/worktrees/cthulhu-build`; one commit per task; never push, never `--force`, never `make install`, never touch `~/bin/orq`, `~/.config/orq`, `~/.local/state/orq`.
- Tests use temp dirs: `ORQ_STATE`, `ORQ_LIMITS`, `ORQ_WORKTREES` dirs, `ORQ_CONFIG` a **file** path.
- Nothing in tests or scripts may call a real `claude` (`e2e-v05.sh` shims it; Go tests set `ORQ_SIN_DOC=1`).
- New code, comments, messages and commit messages in English; existing Spanish names stay.
- Standard library only; `internal/arch_test.go` (imports only point down) stays green.

## Review Focus
1. A worker inside a clone using a relative path to the exam (`../../tests/aceptacion/x.sh`) or `cd <main> && orq slice examen …` → denied (Task 1, Task 2 tests).
2. A directory that has `.git` as a folder but no marker (the Owner's normal repo) → treated as main checkout, not as worker (Task 1 test).
3. A corrupt or empty `orq-clon` marker → treated as worker (fail closed), never as main (Task 1 test).
4. `orq slice cerrar` with no `claude` binary on PATH and `ORQ_SIN_DOC` unset → closes and prints the warning, exit 0 (Task 3 test).
5. A slice opened by orq 1.0.0 (schema 0, linked worktree, no contract ref) → still verifies and closes with the new binary (Task 4 tests).

---

### Task 1: The gate recognizes worker clones

**Files:**
- Modify: `harness/internal/gate/gate.go` (add `IsWorker`), `harness/internal/cli/cli.go:175` (`realEnv` uses it)
- Modify: `harness/internal/orqdir/orqdir.go` (single home of the marker name) and `harness/internal/slice/clones.go:24` (use it)
- Test: `harness/internal/gate/worker_test.go` (new)

**Interfaces:**
- Produces: `orqdir.MarcaClon = "orq-clon"`; `gate.IsWorker(dir string) bool` — true when `IsLinkedWorktree(dir)` or the nearest `.git` directory upward contains `orqdir.MarcaClon` (any content, even unreadable JSON). `slice.MarcaClon` becomes an alias of `orqdir.MarcaClon` or is removed with its uses updated.

- [ ] **Step 1: Write the failing tests** in `worker_test.go`, on real temp dirs (`t.TempDir()`, `git init`):
  - `TestIsWorkerClonWithMarker`: `.git/orq-clon` with valid JSON → `IsWorker(sub/dir)` is true.
  - `TestIsWorkerCorruptMarker`: marker content `"{"` → true.
  - `TestIsWorkerPlainRepo`: `.git` folder, no marker → false.
  - `TestIsWorkerLinkedWorktree`: `git worktree add` → true.
  - `TestGateDeniesInClone`: with `Env.IsWorktree = IsWorker` and `Cwd` inside a marked clone, these payloads return `Allow == false`: Edit of the bundle file, `Bash` `echo x > tests/aceptacion/s01.sh`, tool `Agent`, Write to `<main>/.orq/slices/01.json`, `Bash` `orq slice cerrar --aceptado`.
- [ ] **Step 2: Run** `cd harness && go test ./internal/gate -run 'IsWorker|GateDeniesInClone' -v` → FAIL (`IsWorker` undefined).
- [ ] **Step 3: Implement** `IsWorker`, move the marker name to `orqdir`, set `IsWorktree: gate.IsWorker` in `realEnv`.
- [ ] **Step 4: Run** `go test ./internal/gate ./internal/slice ./internal/cli` → the new tests PASS; no new failures.
- [ ] **Step 5: Commit** `gate: treat orq worker clones as workers (marker in .git)`.

### Task 2: Workers cannot run orchestrator/Owner-only slice commands

**Files:**
- Modify: `harness/internal/gate/reglas.go:161` (`soloOrquestador`)
- Test: `harness/internal/gate/decisiones_test.go` (add rows to `TestDecisiones`)

**Interfaces:**
- Consumes: `gate.IsWorker` (Task 1).
- Produces: `soloOrquestador` denies, for `slice`: `abrir, cerrar, reintentar, tomar, lanzar, juntar, examen, alcance, rehuella, reabrir, ruta, verificar, convertir`. `halt --set` stays allowed (pausing destroys nothing).

- [ ] **Step 1: Add failing rows** to `TestDecisiones`, worker cwd, each expecting BLOCK with the existing worker message: `orq slice examen tests/s01.sh --aprobado "req ATN-1"`, `cd /main && orq slice examen tests/s01.sh --aprobado x`, `orq slice alcance src/ --aprobado x`, `orq slice rehuella`, `orq slice reabrir 01`, `orq slice verificar`, `orq slice ruta slice "x"`. Same commands from the main checkout with no `AgentID` → ALLOW. `orq halt --set "x"` from a worker → ALLOW.
- [ ] **Step 2: Run** `go test ./internal/gate -run TestDecisiones -v` → the new rows FAIL.
- [ ] **Step 3: Extend the list** in `soloOrquestador`.
- [ ] **Step 4: Run** `go test ./internal/gate` → PASS.
- [ ] **Step 5: Commit** `gate: exam, scope, rehash, reopen, lane and verify are orchestrator-only`.

### Task 3: `orq slice cerrar` honours `ORQ_SIN_DOC`

**Files:**
- Modify: `harness/internal/cli/proyecto.go:671` (the `EscribirFicha` call in `cerrar`)
- Test: `harness/internal/cli/` existing close test file (add one test)

**Interfaces:**
- Produces: when `ORQ_SIN_DOC` is non-empty, `cerrar` skips `EscribirFicha` and prints `project card not updated (ORQ_SIN_DOC set)`; otherwise unchanged (a failing `claude` still yields the warning and exit 0).

- [ ] **Step 1: Write `TestCerrarSinDocNoLlamaAlEscriba`**: PATH holds a fake `claude` that writes a sentinel file; with `ORQ_SIN_DOC=1`, close an accepted slice → exit 0 and the sentinel does not exist. Second case, `ORQ_SIN_DOC` unset and no `claude` on PATH → exit 0 and output contains the warning.
- [ ] **Step 2: Run** it → FAIL (sentinel exists).
- [ ] **Step 3: Implement** the guard.
- [ ] **Step 4: Run** `go test ./internal/cli` → PASS.
- [ ] **Step 5: Commit** `cli: slice cerrar skips the scribe when ORQ_SIN_DOC is set`.

### Task 4: The seven failing slice tests, by decided behaviour

**Files:**
- Modify: `harness/internal/slice/slice_test.go`, and the slice code each decision touches (`contrato.go`, `cierre.go`, `clones.go`, `nucleo.go`)
- Modify: `harness/internal/cli/cli.go:57,60` (usage text that still offers `--en-sitio`)

**Interfaces:**
- Produces: `orq slice convertir NN` (orchestrator-only, Task 2 list) for 1.0.0 states; the error at `contrato.go:108` names a command that now exists.

Decisions (each test is rewritten to assert the decided behaviour, never deleted):
1. `TestVerificarAisladoPorDefectoYOptOut` — `--en-sitio` stays removed: the test asserts the refusal message and exit code; usage text no longer offers it.
2. `TestEstadoViejoVerificaEnElWorktree`, `TestEstadoViejoSigueVerificandoYCerrando` — Acceptance 6 holds: a schema-0 state with no contract ref is adopted: verify writes the contract from the state after checking the exam hash against `SHARojo`, records `Procedencia: legado`, and continues; verification of a legacy slice runs on a clean copy of its commit (not in the worktree). If the state's processes are alive (writer lock held), it refuses and says to stop them. `orq slice convertir NN` does the same explicitly.
3. `TestVerdeSinPruebasEjecutadasFalla` — the behaviour (0 tests run ≠ green) is kept; the fixture stops writing `.orq/slices/01.json` by hand and goes through `Abrir`, so the contract matches.
4. `TestRehuella` — keep the rule "rehash only from BLOQUEADO"; the test drives the slice to BLOQUEADO first and also asserts the refusal from IMPLEMENTANDO.
5. `TestPrepararDeps` — dependencies reach the clone: `node_modules` (and `.venv`) of the main checkout appear in the clone as a read-only link; the test asserts presence and that writing through it fails.
6. `TestAlinearDetectaDesvio` — drift detection works on clones: `alinear` diffs the clone's branch tip against the red commit; the test asserts the out-of-scope warning.

- [ ] **Step 1: Rewrite the seven tests** to the decisions above; run `go test ./internal/slice -v` → they FAIL for the decided reason (not for fixture errors).
- [ ] **Step 2: Implement** decisions 2, 5, 6 (2 and 3 are already behaviour), and the usage text fix.
- [ ] **Step 3: Run** `go test -race ./...` → PASS, every package.
- [ ] **Step 4: Commit** one commit per decision group: `slice: adopt 1.0.0 states (convertir)`, `slice: deps and drift on clones`, `test: slice fixtures through the API`.

### Task 5: Validation scripts green and `make ci`

**Files:**
- Modify: `validacion/e2e-v05.sh` (the 3 `--en-sitio` cases assert refusal; the 2 gate cases run in a real clone; the 2 metrics cases match the clone model), `validacion/aislamiento.sh` (part 2 asserts the refusal reason text)
- Modify: `Makefile` (target `ci`)
- Create: `.github/workflows/ci.yml` (not pushed)

**Interfaces:**
- Produces: `make ci` = `gofmt -l harness` empty, `go vet ./...`, `go test -race ./...`, `validacion/e2e-v05.sh`, `validacion/aislamiento.sh --todo` (skips with a clear `SKIP: bwrap missing` and non-zero exit unless `ORQ_CI_ALLOW_SKIP=1`). The workflow runs `make ci` on ubuntu-latest after `apt-get install -y bubblewrap`.

- [ ] **Step 1: Add a new e2e case** "worker in a clone runs `orq slice examen --aprobado` → gate exit 2" and run `bash validacion/e2e-v05.sh` → it FAILS before Tasks 1–2 are merged in this branch, PASS after; record the counts.
- [ ] **Step 2: Fix the other e2e cases** per the list above; run → `0 fallan`.
- [ ] **Step 3: Run** `bash validacion/aislamiento.sh --todo` → all pass, 0 pending.
- [ ] **Step 4: Add** `make ci` and the workflow; run `make ci` → exit 0.
- [ ] **Step 5: Commit** `ci: make ci runs every suite; e2e covers clones`.

### Task 6: Report

- [ ] Write `docs/harness-v2/step1-report.md` on `v2/build`: commit SHAs per task, exact suite counts from `make ci`, each of the seven tests with the decision applied, anything not done and why. Commit `docs: step 1 report`.
