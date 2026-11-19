# Step 1b · Binary identity, attack index, named-risk checks — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Every `make ci` suite proves it ran the candidate binary it just built; every known attack points at the test that reproduces it or at an accepted limit; `staticcheck` guards `gate`/`slice`, and a time-boxed mutation report covers the four critical decisions.

**Architecture:** No new subsystem. `orq version` reads `debug.ReadBuildInfo`. `make ci` builds the candidate once and passes its path to both suites, which source one guard (`validacion/candidato.sh`). The attack index is a Markdown list checked by a Go test in `harness/internal` (the existing test-only package of `arch_test.go`). Mutation is a small in-repo tool (`harness/tools/mutar`) using `go/ast` and `go test -overlay`, so no source file is ever rewritten.

**Tech Stack:** Go 1.25 standard library; bash; `staticcheck` 2025.1.1 as a dev tool in `~/go/bin`.

**Spec:** `docs/harness-v2/plan.md`, "Milestone direction (Owner, 2026-11-18 evening)" › "Step 1b, bounded", and "Design" › "Step 1b".

## Global Constraints
- Branch `v2/build`; never `make install`, never write `~/bin/orq`, `~/.config/orq`, `~/.local/state/orq`; no push, no `--force`, no `git branch -D`.
- Tests use temp `ORQ_STATE`/`ORQ_LIMITS`/`ORQ_WORKTREES` dirs and a temp `ORQ_CONFIG` file; nothing calls a real `claude` or `codex`.
- orq stays standard library only; `staticcheck` is a dev tool, not a module dependency.
- Each commit: `gofmt -l harness` empty, `go vet ./...`, `go test -race ./...`, `make lint` once it exists; `internal/arch_test.go` green.
- English code, comments and commits; commits end with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.

## Facts found before planning (they shape the tasks)
- Go stamps VCS info only when `.git` is a directory (`$GOROOT/src/cmd/go/internal/vcs/vcs.go:218`, `isDir: true`). In this linked worktree `.git` is a file, and `go build` embeds no `vcs.*` settings (checked: a build here has none; a build in a plain clone has `vcs.revision=587078e…`). So `make ci` builds the candidate in a throwaway local clone that carries the working tree's files (tracked changes, untracked non-ignored files, deletions). `orq version` still reads only `debug.ReadBuildInfo`.
- gremlins v0.5.0 scopes only by package path, `--coverpkg` or `--diff`; it cannot target functions. So mutation uses the in-repo tool (the row's fallback).
- `staticcheck ./internal/gate/... ./internal/slice/...` today: `slice/slice.go:273` ST1005 (message starts with the slice ID `S-NN`), `slice/slice_test.go:143,590,614` SA4006 (value never used). Nothing in `gate`.
- The gate's table tests have no subtests (no `t.Run`), so a "row" in the index is a literal fragment of the row's source text.

## Review Focus
1. A suite run with a stale `orq` earlier in PATH than the candidate → the guard stops the suite before its first case (Task 2 test).
2. A candidate whose `orq version` revision differs from `git rev-parse HEAD` (built from another commit) → refused (Task 2 test).
3. A binary with no VCS info → prints `revision=unknown modified=unknown`, never an empty or invented revision (Task 1 test).
4. An index line with neither a test reference nor an accepted limit, or naming a test or row that no longer exists → `TestAttackIndex` fails (Task 3 test).
5. A mutant that hangs (loop condition inverted) → counted as killed by timeout, and the run never exceeds the 10-minute budget (Task 5 test + run).

---

### Task 1: `orq version` prints revision and dirty flag

**Files:** Modify `harness/internal/cli/cli.go` (`case "version"`); Create `harness/internal/cli/version.go`, `harness/internal/cli/version_test.go`.

**Interfaces:** Produces `func versionLine(bi *debug.BuildInfo, ok bool) string` → `"1.0.0 revision=<vcs.revision> modified=<vcs.modified>"`; each missing value is `unknown`.

- [ ] **Step 1: failing tests** `TestVersionLine`: settings `vcs.revision=abc`, `vcs.modified=true` → `"1.0.0 revision=abc modified=true"`; `ok=false` → `"1.0.0 revision=unknown modified=unknown"`; settings without vcs keys → same unknowns. `TestVersionCommand`: `Run([]string{"version"}, …)` exit 0, stdout `"1.0.0 revision=unknown modified=unknown\n"` (test binaries carry no VCS info).
- [ ] **Step 2:** run `go test ./internal/cli -run 'TestVersion'` → FAIL (undefined `versionLine`).
- [ ] **Step 3:** implement; `case "version"` prints `versionLine(debug.ReadBuildInfo())`.
- [ ] **Step 4:** tests pass; full quality set.
- [ ] **Step 5:** commit `cli: orq version prints the VCS revision and the modified flag`.

### Task 2: `make ci` builds the candidate once; each suite proves it runs it

**Files:** Create `validacion/candidato.sh`; Modify `validacion/e2e-v05.sh`, `validacion/aislamiento.sh`, `Makefile` (`ci`); Create `harness/internal/candidato_test.go`.

**Interfaces:** `candidato.sh` defines `candidato PATH` (bash function): prints `candidate: PATH` and its `orq version`; returns 1 with a message containing `not the candidate` when `ORQ_CANDIDATO` is unset, when PATH ≠ `$ORQ_CANDIDATO`, or when the version line lacks `revision=$(git -C <repo of candidato.sh> rev-parse HEAD)`. e2e calls `candidato "$(command -v orq)" || exit 1` right after its PATH setup; aislamiento calls `candidato "$ORQ" || exit 1` where `ORQ` defaults to `$ORQ_CANDIDATO` (its own self-build is removed: with no VCS stamp in a worktree it could never pass the guard).

`make ci`: first builds the candidate — `git clone -q --shared` of the repo into `$tmp/src`, copy `git ls-files -co --exclude-standard` files over it, remove `git ls-files -d` files, `go build -o $tmp/bin/orq ./cmd/orq` there — then prints `candidate: $tmp/bin/orq` and `orq version`, exports `ORQ_CANDIDATO=$tmp/bin/orq`, and runs the existing steps unchanged.

- [ ] **Step 1: failing tests** in `harness/internal/candidato_test.go` (package `internal_test`), with fake `orq` shell scripts in `t.TempDir()`; `head` = `git rev-parse HEAD`:
  - `TestCandidatoGuard` (sources `validacion/candidato.sh` with bash): candidate path matches and prints `1.0.0 revision=<head> modified=false` → exit 0; other path → exit 1 + `not the candidate`; right path, revision `deadbeef` → exit 1; `ORQ_CANDIDATO` unset → exit 1.
  - `TestE2EStopsOnAStaleOrqFirstInPath`: `PATH=<stub>:<cand>:/usr/bin:/bin`, stub `orq` prints `1.0.0` (like the installed one), `ORQ_CANDIDATO=<cand>/orq`; `bash validacion/e2e-v05.sh <tmp>` → exit ≠ 0, output contains `not the candidate`, does not contain `== S-01`.
  - `TestAislamientoStopsOnAnotherOrq`: `ORQ=<stub>/orq ORQ_CANDIDATO=<cand>/orq bash validacion/aislamiento.sh` → exit ≠ 0, `not the candidate`, no `Parte 1`/first case output.
- [ ] **Step 2:** `go test ./internal -run 'Candidato|Stops'` → FAIL (no `candidato.sh`; e2e runs its cases).
- [ ] **Step 3:** implement as above.
- [ ] **Step 4:** tests pass; `make ci` prints the candidate path and `1.0.0 revision=<HEAD> modified=<…>`, and each suite prints its `candidate:` line.
- [ ] **Step 5:** commit `ci: build the candidate once; every suite checks it runs that binary`.

### Task 3: attack index with a reference checker

**Files:** Create `docs/harness-v2/attacks.md`, `harness/internal/attacks_test.go`; Modify `harness/internal/gate/decisiones_test.go` (regression rows for held attacks that had no row).

**Index format** (one line per attack, from `reviews/step1-review*.md` and `working/inspector/informe*.md`):
`- **ID** source · attack → `path:TestName` or `path:TestName/row fragment` …` or `→ limit until step N (accepted: <review or report section>)`. Paths are repo-relative. For `.go` files the checker needs `func TestName(` in the file and, when given, the row fragment as a literal substring of the file; for `.sh` files the part after `:` is a case label that must appear in the file.

**Interfaces:** `func checkIndex(root, text string) []string` (problems, one per bad line/ref).

- [ ] **Step 1: failing tests** `TestCheckIndexFindsBadLines`: sample text with (a) a line with no `→`, (b) `→` with neither a ref nor `limit until step`, (c) a limit without `(accepted:`, (d) a ref to a missing file, (e) a missing `func`, (f) a missing row fragment, (g) a missing `.sh` label → 7 problems; a good line → 0. `TestAttackIndex`: `checkIndex` on the real `docs/harness-v2/attacks.md` → no problems, and the file has ≥ 1 entry.
- [ ] **Step 2:** run → FAIL (undefined / missing file).
- [ ] **Step 3:** write `checkIndex`; write `attacks.md`. Held attacks found by a review without a reproducing row get one in `TestDecisiones` (already denied, so they pass at once; they are regression rows): `--"output"`, `--out\put`, `rev-list --output`, `reflog --output`, `git -C /x/repo update-ref refs/orq/contrato/01 HEAD`, `git update-ref refs/orq/contrato/01 HEAD`, `echo x > /x/repo/.git/refs/orq/contrato/01`. Attacks still open (interpreters, scripts, programs outside the parsed line, hooks in the clone, `ln -s` in the same line, dynamic `git commit` option values, plain-text authority) are limits with the section that accepted them.
- [ ] **Step 4:** tests pass; quality set.
- [ ] **Step 5:** commit `docs: attack index; a test checks every referenced test exists`.

### Task 4: `make lint` with pinned staticcheck

**Files:** Modify `Makefile` (`lint`, `ci: lint`), `.github/workflows/ci.yml` (install step), `harness/internal/slice/slice.go:273`, `harness/internal/slice/slice_test.go:143,590,614`.

- `STATICCHECK_VERSION := 2025.1.1`, `STATICCHECK ?= $(HOME)/go/bin/staticcheck`. `lint` fails with `install: go install honnef.co/go/tools/cmd/staticcheck@2025.1.1` unless `$(STATICCHECK) -version` starts with `staticcheck 2025.1.1 `; then runs `staticcheck ./internal/gate/... ./internal/slice/...` in `harness`.
- [ ] **Step 1:** run `make lint` → FAIL with the four findings above (the failing check).
- [ ] **Step 2:** fix the SA4006 values (discard with `_` or assert them); ST1005 on `slice.go:273` is a false positive (the message starts with the ID `S-NN`): `//lint:ignore ST1005 the message starts with the slice ID (S-NN)`.
- [ ] **Step 3:** `make lint` exit 0; `STATICCHECK=/bin/false make lint` → exit 1 with the install line.
- [ ] **Step 4:** commit `ci: make lint runs pinned staticcheck on gate and slice`.

### Task 5: `make mutation` (report only)

**Files:** Create `harness/tools/mutar/main.go`, `harness/tools/mutar/main_test.go`; Modify `Makefile` (`mutation`, not in `ci`).

**Interfaces:**
- `type mutant struct{ File string; Line int; Off int; Old, New string }`
- `func sitios(src []byte, fn string) ([]mutant, error)` — inside the function (or method) named `fn`: `==`↔`!=`, `<`↔`>=`, `>`↔`<=`, `&&`↔`||`, drop unary `!`, `true`↔`false`.
- `func aplicar(src []byte, m mutant) []byte` — byte splice at `Off`.
- `main`: args `-t 10m` (whole budget) and targets `path/file.go:Func`; for each mutant writes the mutated file to a temp dir and runs `go test -count=1 -failfast -timeout 90s -overlay <json> ./<pkg dir>`; 4 at a time. Result per mutant: `KILLED`, `LIVED`, `TIMEOUT` (budget or test timeout; counted as killed), `INVALID` (build failed). Prints `LIVED file:line old→new` lines, then totals; exit 1 if the budget ran out before every mutant ran.
- Targets: `internal/gate/gate.go:IsWorker`, `internal/gate/repo.go:gitAjeno`, `internal/slice/slice.go:huellaDe`, `internal/slice/nucleo.go:examen`, `internal/slice/nucleo.go:Verificar` (fingerprint), `internal/slice/clones.go:ingerir`, `internal/slice/caja.go:esAncestro` (bundle descendant).
- [ ] **Step 1: failing tests** `TestSitios`: for `func f(a, b int, c bool) bool { return a == b && !c || true }` → exactly `==→!=`, `&&→||`, `||→&&`, `!→""`, `true→false` with right offsets; sites in another function are not listed; unknown function → error. `TestAplicar`: splicing `==`→`!=` gives source that `go/parser` parses and contains `a != b`.
- [ ] **Step 2:** run → FAIL.
- [ ] **Step 3:** implement; `make mutation` sets temp `ORQ_*` like `ci` and runs `go run ./tools/mutar -t 10m <targets>`.
- [ ] **Step 4:** run `make mutation`; record wall time and survivors. For each survivor that shows a real missing test, add the test (seen failing against the mutant via the overlay) and rerun; justify the rest in one line each in the report.
- [ ] **Step 5:** commit `tools: mutar, a scoped mutation report for the four critical decisions` (+ one commit per killed survivor test).

### Task 6: report

- [ ] `make ci` from a clean tree; read its output (counts per suite, candidate path, `orq version`).
- [ ] Write `docs/harness-v2/step1b-report.md`: commits; per deliverable the risk it catches and its evidence; `make ci` counts and the tested binary's `orq version`; mutation survivors and justifications; not done and why; Owner decisions (worktree VCS stamping and `make install`; staticcheck does not flag `_ = err` or unchecked calls — that is `errcheck`, not added).
- [ ] Commit `docs: step 1b report`.
