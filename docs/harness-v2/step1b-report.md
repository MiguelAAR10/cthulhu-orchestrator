# Step 1b report · Binary identity, attack index, named-risk checks

Branch `v2/build`, worktree `~/worktrees/cthulhu-build`. Plan: `docs/harness-v2/step1b-plan.md`. Spec: plan.md, "Milestone direction" › "Step 1b, bounded".
I did not install anything, push, or write `~/bin/orq`, `~/.config/orq` or `~/.local/state/orq`. Every suite used temp `ORQ_*` dirs and a temp `ORQ_CONFIG` file. Nothing called a real `claude` or `codex`. The only thing written outside the worktree is the dev tool `~/go/bin/staticcheck`, installed with `go install`. I installed gremlins and then removed it, because it was not used.

## Commits

| Deliverable | Commit | Subject |
|---|---|---|
| plan | `8dd716d` | docs: step 1b plan |
| 1 | `7cd5f25` | cli: orq version prints the VCS revision and the modified flag |
| 1 | `4dd597c` | ci: build the candidate once; every suite checks it runs that binary |
| 2 | `821f1ea` | docs: attack index; a test checks every referenced test exists |
| 3a | `a601c1e` | ci: make lint runs pinned staticcheck on gate and slice |
| 3b | `2939abd` | tools: mutar, a scoped mutation report for the four critical decisions |
| 3b | `231801b` | slice: tests for the exam rewriting its bundle and a committed report |
| 3b | `27ffcfd` | gate: grouped and = forms of force-create cannot move another branch |
| report | this commit | docs: step 1b report |

## `make ci` on `27ffcfd` (clean tree, exit 0)

- Binary tested: `/tmp/orq-ci.xEXcvV/bin/orq · 1.0.0 revision=27ffcfd0fa65ac2701181987bb7b34a25f023048 modified=false`. It is printed once after the build, and again by each suite's guard.
- `make lint` (staticcheck 2025.1.1 on `internal/gate/...` and `internal/slice/...`): clean.
- `gofmt -l harness`: nothing listed. `go vet ./...`: clean.
- `go test -race ./...`: 22 packages ok. Top-level tests: 182 pass, 0 fail, 2 skipped. Subtests: 23 pass.
  - The 2 skips are the same as in step 1: `TestPrepararDeps` (needs the box) and herdr `TestLiveTabSplitClose`.
- `e2e-v05`: **86 pass, 0 fail**. `aislamiento`: **14 pass, 0 fail, 0 pending**.

## 1 · Binary identity

**Risk it catches.** A suite could test an installed or stale `orq` instead of the build under review. Before this step, the candidate and `~/bin/orq` both printed `1.0.0`, so nothing could tell them apart.

**What changed.**
- `orq version` prints `1.0.0 revision=<vcs.revision> modified=<vcs.modified>`, read from `debug.ReadBuildInfo`. A value the build did not record prints `unknown`, never a guess (`internal/cli/version.go`).
- `make ci` builds the candidate once, prints its path and `orq version`, and exports `ORQ_CANDIDATO`.
- Both suites source `validacion/candidato.sh` before their first case:
  - e2e checks `command -v orq`.
  - aislamiento checks `$ORQ`.
  - Each one stops unless that path equals `ORQ_CANDIDATO` and the version line has `revision=$(git rev-parse HEAD)`.

**Toolchain finding.** Go stamps VCS info only when `.git` is a directory (`$GOROOT/src/cmd/go/internal/vcs/vcs.go:218`, `isDir: true`). In a linked worktree, like this one, `.git` is a file, so a plain `go build` prints `revision=unknown`.
- `make ci` therefore builds in a throwaway `git clone --shared` that carries the working tree's files: tracked changes, untracked non-ignored files, and deletions.
- `orq version` still reads only `ReadBuildInfo`.
- Checked: on a dirty tree the run printed `modified=true`; on the clean commit, `modified=false`.

**Evidence.**
- `TestVersionLine` and `TestVersionCommand` (red: undefined `versionLine`).
- `TestCandidatoGuard` covers four cases: the candidate passes; another path, another revision, or no `ORQ_CANDIDATO` each fail with `not the candidate`.
- `TestE2EStopsOnAStaleOrqFirstInPath`: a stub `orq` that prints `1.0.0` comes before the candidate in PATH.
  - Before the guard, the suite ran with the stub and ended 29 pass / 57 fail.
  - Now it stops at the guard, before `== S-01`.
- `TestAislamientoStopsOnAnotherOrq`: same check through `$ORQ`, stopping before `== Parte 1`.

**Ruling.** `make ci` puts the candidate first in PATH itself, so a stub in the caller's PATH can never come first inside `make ci`. The test therefore plants the stub inside each suite's own PATH, which is where the guard runs. `set -e` in `make ci` turns a guard failure into a failed run. No test runs `make ci` itself with a stub, because that would nest `go test -race` inside `go test`.

**Ruling.** `aislamiento.sh` no longer builds its own orq when `ORQ` is unset. `ORQ` now defaults to `ORQ_CANDIDATO`. A binary it built itself in a worktree would have no revision and could never pass the guard.

## 2 · Attack index

**Risk it catches.** A known attack whose test is deleted or renamed would go unnoticed. So would an attack that never had a test, or a limit that nobody accepted.

**What changed.**
- `docs/harness-v2/attacks.md` has 49 lines: 46 from the reviews, the inspector reports and step 1's final review, plus 3 found by mutation (M-1..M-3).
- Each line points at a test or shell case (`file:TestName`, `file:TestName/row fragment`, `validacion/*.sh:label`), or says `limit until step N (accepted: …)`.
- The gate's table tests have no subtests, so a "row" is a literal fragment of the row's source.

**Evidence.**
- `harness/internal/attacks_test.go:TestAttackIndex` fails on:
  - a line with neither a reference nor a limit;
  - a limit without `(accepted:`;
  - a missing file, test function, row fragment or shell label.
- `TestCheckIndexFindsBadLines` checks that 7 kinds of bad line give 7 problems and that good lines give 0.
- Seen red: first with no `attacks.md`, then on one wrong shell label.
- 7 regression rows went into `TestDecisiones`. They cover attacks that reviews had found already blocked but that no row reproduced: quoted and escaped `--output`, `rev-list`/`reflog --output`, and the `refs/orq/contrato` writes. They passed at once, as expected.

**Limits listed.**
- Until step 2:
  - scripts, interpreters and programs outside the parsed command line;
  - hooks in the clone;
  - a symlink made in the same command line;
  - dynamic `git commit` option values (review 8);
  - a marker damaged by a script;
  - writable dependency links;
  - a `chdir` before `orq`.
- Until step 3: authority typed as text, and the computable dry-run code.

## 3a · staticcheck

**Risk it catches.** Values that are assigned and never used, and dead branches, in `gate` and `slice`.

**What changed.**
- `make lint` checks `$(HOME)/go/bin/staticcheck -version` against `2025.1.1`. If it does not match, lint fails with `go install honnef.co/go/tools/cmd/staticcheck@2025.1.1`.
- It runs on `./internal/gate/... ./internal/slice/...`. `ci` depends on `lint`.
- The workflow installs the same version.

**Findings, seen red first.**
- `slice_test.go:143,590,614`, SA4006 (value never used). Fixed. The dead `_ = e` at the end of `TestEquipoFrontBackTestEnUnSlice` went with it.
- `slice.go:273`, ST1005. Suppressed with `//lint:ignore ST1005 the message starts with the slice ID (S-NN)`. This is the only suppression.
- `gate`: no findings.

**Limit.** staticcheck does not flag errors that are discarded (`_ = f()`) or never checked; that is `errcheck`, which is not added. One more `_ = e` remains at `slice_test.go:269`, which staticcheck does not flag.

## 3b · Mutation (report only)

**Why not gremlins.** gremlins v0.5.0 can be narrowed only by package, `--coverpkg` or `--diff`, not by function. So, as the row allows, I wrote an in-repo mutator.
- `harness/tools/mutar` uses `go/ast` and `go test -overlay`, so no source file is ever rewritten.
- It flips `==`/`!=`, the relational operators (negation and boundary) and `&&`/`||`, drops a unary `!`, and swaps `true`/`false`.
- It runs each package's tests once without mutants first.
- It is about 190 lines, not 40, because the baseline run, the time budget and the result classes need room.

**`make mutation`** is not part of `ci`. It has a 10-minute budget and runs 6 mutants at a time, on these targets:
- `IsWorker`
- `gitAjeno` (the worker git allowlist)
- the exam fingerprint: `huellaDe` (0 sites), `examen`, `Verificar`
- the bundle descendant check: `ingerir`, `esAncestro`

**Runs.**
- First: 175 mutants, 145 killed, 27 lived, 3 invalid, 6m15s.
- Final, on `27ffcfd`: **176 mutants, 153 killed, 0 timeout, 20 lived, 3 invalid, 0 not run, 2m55s**.

**Killed during this step.** Each fix is a test seen failing against its mutant.
- `nucleo.go:101` `&&→||`: nothing tested an exam run that rewrites its bundle in the copy. Added `TestExamenModificadoMientrasCorreBloquea`.
- `nucleo.go:93` `!=→==`: nothing tested that a report the worker committed is deleted before the exam runs. Added `TestReporteCommiteadoPorElWorkerNoCuenta`.
- `repo.go:122,144,146×2,152`: added rows for bare `git`, `branch -f` with no branch, and grouped `-qf`/`-qM`/`-qC`.
  - These rows exposed a **real gate bypass**. `git checkout -qB main`, `git switch -qC main`, `git checkout -qBmain` and `git switch --force-create=main` were allowed, while the ungrouped forms were denied.
  - Fixed in `valorDe` (it now reads a value after a letter inside grouped short options) and in `fuerza` (it now sees `--force-create=`).
  - Impact was low: in a clone the refs are the worker's own. The bypass is in the index as M-1.

**Remaining survivors** (stopped at the orchestrator's time box), one line each:
- `repo.go:154 &&→||` (`(checkout||switch) && fuerza`): equivalent. Earlier cases catch `branch`, and for other commands the inner `valorDe -B/-C` finds nothing, so the result is the same.
- `nucleo.go:88 false→true` ×2: equivalent. These are return values on the error path, and the caller returns on `err` first.
- `nucleo.go:98 false→true` ×2: equivalent, same reason.
- `clones.go:184 ||→&&` ×2 and `>→>=`: the bundle file sanity check (not regular, or larger than `MaxBundle`). Reproducing it needs hostile code inside the box that writes into the out dir. This is step 2 box-test material.
- `clones.go:202 !=→==` (`in.Nuevo`): outside the descendant decision. It only marks that a new tip was fetched.
- `caja.go:56 false→true`, `caja.go:59 false→true`: equivalent. These are error-path returns, and every caller checks `err` first.
- `nucleo.go:296 ||→&&` (`err != nil || e == nil` after load): outside the fingerprint decision. A nil state comes with an error in practice.
- `nucleo.go:299 !=→==` (phase guard: verify only in Implementando/Verificado): outside the decision, but **a real missing test**. Verifying a BLOQUEADO slice should be refused. Suggested for step 2.
- `nucleo.go:358 true→false` (`rec.Contado`): outside the decision, but **a real missing test**. No test asserts that the receipt says it counted tests. Suggested for step 2a (receipts with counted tests = 100 %).
- `nucleo.go:359 !=→==` (verdict note appended): outside the decision. It only changes the receipt note text.
- `nucleo.go:386 ||→&&` ×3 (the slice changed while the exam ran → the receipt does not count): outside the decision. A concurrency path with no test. Suggested for step 2b (idempotent close).
- `nucleo.go:416 &&→||` ×2 (no-progress detection): outside the decision. Partly covered by `TestSinProgresoBloquea…`. The negated combinations are not pinned.
- 3 INVALID (`repo.go:78`, `repo.go:154`, `nucleo.go:299`): the mutant does not compile, so it is not counted.

Survivors in `IsWorker`: 0. In the descendant comparison itself (`clones.go:197 !ok`, `esAncestro` `code == 0`/`code > 1`): 0. In the fingerprint comparisons (`nucleo.go:101`, `:327`): 0.

## Not done, and why

- I did not kill the remaining survivors. The orchestrator's time box stopped the work; the two real missing tests are listed above as suggestions.
- `errcheck` was not added. It is outside the row (staticcheck only, pinned).
- No test runs `make ci` end to end with a stub; see ruling 1.

## Open decisions for the Owner

1. **`make install` from a linked worktree** produces `revision=unknown`, for the same Go reason. Installing from the main checkout (a real `.git` directory) stamps it correctly. Options:
   - install only from the main checkout;
   - make `install` use the same clone build as `ci`.
   - Not changed: `install` is outside this row, and the rule says never to run it.
2. **The gate fix in 3b** (`valorDe`/`fuerza`) was a bypass found by mutation, fixed with its rows seen red first. It touches gate code that the row did not name. It needs a review focused on that finding.
3. The two real missing tests outside the named decisions (phase guard, `rec.Contado`) are waiting for a decision on which step owns them.

## Close (orchestrator, 2026-11-18)
Integrated as 0730e9f. `make ci` by the orchestrator: exit 0; candidate `revision=0730e9f… modified=false` checked by every suite; lint clean; race 22 packages ok; e2e 86/0; aislamiento 14/0/0. Focused review `reviews/step1b-review.md`: candidate check holds; the force-create item is reclassified to step 2 (attack R1b-1, step 2 Task 6 test).
