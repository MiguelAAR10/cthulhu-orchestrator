# Step 1 report · Finish B5 + local CI

Branch `v2/build`, worktree `~/worktrees/cthulhu-build`. Plan: `docs/harness-v2/step1-plan.md`.
Nothing was installed, pushed or written outside this worktree. All suites ran with temp `ORQ_STATE`, `ORQ_LIMITS`, `ORQ_WORKTREES` and `ORQ_CONFIG`, and nothing called a real `claude` or `codex`.

## Commits

| Task | Commit | Subject |
|---|---|---|
| 1 | `e583e78` | gate: treat orq worker clones as workers (marker in .git) |
| 2 | `9ad9c91` | gate: exam, scope, rehash, reopen, lane and verify are orchestrator-only |
| 3 | `975b53c` | cli: slice cerrar skips the scribe when ORQ_SIN_DOC is set |
| 4 | `a18361d` | slice: adopt 1.0.0 states (convertir) |
| 4 | `93b23ff` | slice: deps and drift on clones |
| 4 | `1b78e62` | test: slice fixtures through the API |
| 5 | `3e4fff7` | ci: make ci runs every suite; e2e covers clones |
| review | `b1105d1` | fix: review findings on clones (marker, legacy tip, capacity) |
| 6 | this commit | docs: step 1 report |

## `make ci` (run after `b1105d1`, exit 0)

- `gofmt -l harness`: no files listed.
- `go vet ./...`: clean.
- `go test -race ./...`: 21 packages ok. Top-level tests: 165 pass, 0 fail, 2 skipped. Subtests: 23 pass.
  - `TestPrepararDeps` is skipped on purpose; see decision 5.
  - `herdr` `TestLiveTabSplitClose` is a pre-existing live-terminal test.
- `validacion/e2e-v05.sh`: **77 pass, 0 fail**.
  - Was 69/5 before Task 5.
  - With the pre-Task-1 binary (`57762a6`) the new script gives 73/4. The 4 failures are all gate cases inside a clone.
- `validacion/aislamiento.sh` (run without `--todo`): **14 pass, 0 fail, 0 pending**.
  - Part 2 now checks that the refusal comes from the git contract.
  - Without `bwrap` the script prints `SKIP: bwrap missing` and exits 1. It exits 0 only with `ORQ_CI_ALLOW_SKIP=1`.
- `.github/workflows/ci.yml` runs `make ci` on ubuntu-latest after installing bubblewrap. It is committed, not pushed, and has never run on GitHub.

## The seven slice tests

| # | Test | Decision applied | Result |
|---|---|---|---|
| 1 | `TestVerificarAisladoPorDefectoYOptOut` → renamed `TestVerificarSiempreAisladoYEnSitioRechazado` | `--en-sitio` stays removed. The test checks the refusal text and that the refused open leaves nothing behind (no open slice, no branch). The CLI exit code is checked in `TestSliceAbrirEnSitioRechazado`. The usage text no longer offers the flag. | pass (the code already behaved this way) |
| 2a | `TestEstadoViejoSigueVerificandoYCerrando` | A schema-0 state (linked worktree, no contract ref) is adopted on verify: the exam hash is checked against the red commit, the contract is written to git, and the provenance becomes `legado`. The slice then verifies and closes, and the linked worktree is removed. | pass |
| 2b | `TestEstadoViejoVerificaEnElWorktree` → renamed `TestEstadoViejoVerificaEnCopiaLimpia` | A legacy slice is verified on a clean copy in the box, never in the worktree. A file that git ignores in the worktree does not make it pass, and the receipt says `aislado`. | pass |
| 2c | new `TestConvertirEstadoViejo` | Adoption refuses while another process holds the slice lock, and refuses on an exam-hash mismatch (writing no contract). `orq slice convertir NN` adopts explicitly; converting a second time is refused. | pass |
| 3 | `TestVerdeSinPruebasEjecutadasFalla` | The fixture goes through `Abrir`: the runner itself reports `1..0` when green, so 0 tests run is still not green. | pass (the code already behaved this way) |
| 4 | `TestRehuella` | A hash can only be retaken from BLOQUEADO. The test checks the refusal from IMPLEMENTANDO, then blocks the slice with a wrong hash in both state and contract, retakes the hash and verifies. | pass (the code already behaved this way) |
| 5 | `TestPrepararDeps` | **Not done. The test is rewritten to the decision and skipped.** See below. | skip |
| 6 | `TestAlinearDetectaDesvio` | `alinear` reads each clone's tip: the commits are brought into `refs/orq/sNN/<name>` and the branch does not move. The test was unchanged; the code changed. | pass |

New CLI command: `orq slice convertir NN`. It is orchestrator-only (Task 2 list). The error for a missing contract now names this command.

## Not done, and why

**Decision 5 — dependencies in clones as a read-only link.** I stopped this one under the Global Constraint instead of designing something else.
- Until B6, the worker runs outside the box with the Owner's uid. Any link from a clone into the main checkout's `node_modules` or `.venv` is therefore writable by the worker. A worker could plant code that trusted sessions in the main checkout then run. That is worse than having no link.
- A private copy is not an option either: P5 says "no per-worker dependency copies".
- No unprivileged mechanism makes a same-uid link read-only outside a sandbox.
- What still works: the exam already gets the dependencies read-only inside the box (`TestAisladoRecibeLasDependenciasIgnoradas`).
- To unblock: do B6 (worker inside the box with `--ro-bind` of the deps), then remove the `t.Skip`.

## Final review: open questions for the Owner

A fresh reviewer read the whole branch. It found 0 Critical, 5 Important and 4 Minor issues.
- Fixed in `b1105d1`, each with a test seen failing first:
  - A worker could move or rewrite its own `.git/orq-clon` marker and so stop being treated as a worker.
  - Verifying a legacy slice ran git inside the worker's worktree (its filters ran as the Owner), and its tip was not checked to descend from the red commit.
  - Workers working in clones were not counted against `max_workers`.
- The two below are decisions for the Owner.

1. **Adopting legacy slices: the plan and the spec disagree.**
   - The plan's decision 2 says verify adopts a 1.0.0 state by itself, and I built it that way.
   - The spec (plan.md, Design: "legacy slices are converted with their old processes stopped … a contract without provenance needs ratification") asks for a human ratification step.
   - The current checks are: exam hash against the red commit, the slice lock not held by another process, and the tip descends from the red commit.
   - These checks are not enough against a 1.0.0 worker that rewrites the state's `cmd` together with a matching hash. Such a worker runs unsandboxed with the shared refs, and its "processes stopped" is only proved by the slice lock, not by checking the processes.
   - Proposal if the Owner wants the spec's version: verify refuses with `ErrSinContrato`; `orq slice convertir NN --ratifico "who"` shows cmd, bundle and red commit, refuses while a writer lease is held, and stores the ratification in the contract.
2. **Until B6, the contract in git is only as strong as the gate.** From a clone, a worker can still run `git -C <main> update-ref …` or write into `<main>/.git/refs`. I did not add more shell parsing to the gate, because the spec stops growing the shell gate. The fix is B6. Until then, `refs/orq/contrato/NN` protects against a forged state file, not against a worker that targets git directly.

Also still open until B6: a worker can still damage its marker with an interpreter script. If it does, the gate's bundle and main-checkout lookups fall back to the clone.

## Deferred minors

- `orq slice convertir` prints `SHARojo[:7]`, which panics on a forged SHA shorter than 7 characters.
- `alinear` stops at the first clone that fails to bundle, and it fetches without the slice lock.
- `make ci` sets `ORQ_SIN_DOC=1` for `go test`, so no Go test runs the real scribe path.
- An adopted state keeps `Esquema` 0, and `Procedencia` is not part of the contract, so it can be forged.
- Closing a legacy slice removes its worktree with `git worktree remove` run from main. That command still inspects the worker's worktree (pre-existing 1.0.0 path).

## Rulings made during execution

Summary first, then the full ledger lines with the cost if each is wrong.
- T1: the bundle and main root come from the clone marker in the real gate env.
- T2: `verificar` is orchestrator-only; two old gate assertions were flipped.
- T4: "processes alive" is read as "slice flock held"; the writer lease is not checked.
- T4: a legacy tip is the shared branch.
- T4: `Esquema` stays 0 after adoption.
- T4: two tests renamed, because their names described removed behaviour.
- T4: the conversion message has no `--ratifico`.
- T4: decision 5 stopped.
- T5: `aislamiento.sh` runs without `--todo`.
- T5: the e2e gate cases already ran in a real clone.
- T5: the workflow relaxes the AppArmor user-namespace restriction (with `|| true`).
- Final review: items 1 and 2 above.

### Ledger (verbatim)

- Task 1: Ruling: plan only sets IsWorktree=IsWorker in realEnv, but in a clone bundleAbierto/CheckoutPrincipal resolve to the clone itself (no .orq), so the bundle and the main .orq were unprotected in the real gate — wired Bundle and MainRoot to slice.LeerMarca in cli (test TestRealEnvEnUnClon) — if wrong: gate trusts marker content a worker can rewrite; Verificar still checks the exam hash.
- Task 2: Ruling: two existing gate tests asserted a worker may run `orq slice verificar`; the plan lists verificar as orchestrator-only — rewrote both assertions to deny (worker) / allow (main) — if wrong: workers that self-verify are blocked and must ask the orchestrator.
- Task 4: Ruling: 'processes alive (writer lock held)' read as: another process holds the slice flock (non-blocking try fails) — the Escritor lease is not checked, else every 1.0.0 slice with a writer could never verify — if wrong: a live 1.0.0 worker that is between commands could still commit after adoption; the exam hash and protected-path checks still apply on each verify.
- Task 4: Ruling: legacy linked worktree tip = the shared branch refs/heads/slice/NN; dirty check via gitDueno status in the worktree (warning-level, as for clones) — if wrong: git config of the main repo applies to that status run.
- Task 4: Ruling: Esquema of an adopted state is left at 0; provenance 'legado' + the contract ref mark adoption — if wrong: code that later branches on Esquema must also look at Procedencia.
- Task 4: Ruling: renamed TestVerificarAisladoPorDefectoYOptOut → TestVerificarSiempreAisladoYEnSitioRechazado and TestEstadoViejoVerificaEnElWorktree → TestEstadoViejoVerificaEnCopiaLimpia (names asserted the removed behaviour); added TestConvertirEstadoViejo and CLI tests in internal/cli/convertir_test.go — if wrong: only names.
- Task 4: Ruling: ErrSinContrato message now says `orq slice convertir NN` (no --ratifico): the plan's interface has no ratification flag; the hash check against SHARojo replaces it — if wrong: a forged legacy state whose hash still matches the red commit's exam is adopted without a human naming who reviewed the exam.
- Task 4: Ruling: decision 5 stopped per the Global Constraint — a read-only link from a clone into the main checkout cannot be made for a worker running unsandboxed with the owner's uid (B6 not built); a writable link would let a worker plant code in main's node_modules, a copy breaks P5. TestPrepararDeps rewritten to the decision and t.Skip'd, reason in step1-report.md — if wrong: clones keep no deps until B6; the exam still gets them read-only in the box (TestAisladoRecibeLasDependenciasIgnoradas).
- Task 4: Ruling: TestAlinearDetectaDesvio already asserted decision 6; it is unchanged, the code changed — if wrong: none.
- Task 5: Ruling: make ci runs aislamiento.sh without --todo (stricter than the plan's --todo: a pending part 2 fails CI, as spec row B5 asks 'green without --todo') — if wrong: none, both pass today.
- Task 5: Ruling: the 2 'gate cases' already ran in a real clone (W10 is the clone made by abrir); they failed with the pre-Task-1 binary (57762a6: 73 pass / 4 fail incl. the 2 new cases) and pass now (77/0) — left as is.
- Task 5: Ruling: workflow also relaxes apparmor userns restriction (ubuntu-24.04 runners block unprivileged bwrap otherwise) — if wrong: one no-op step (|| true).
- Final: Ruling: I2 auto-adoption trusts the state (spec plan.md:80 asks ratification; step1-plan decision 2 says verify adopts) — kept the plan's behaviour plus the new ancestry check; the plan/spec conflict goes to the Owner in the report — cost if wrong: an unsandboxed 1.0.0 worker that forges cmd+hash in the state gets a weaker exam adopted without a human.
- Final: Ruling: I3 workers can still write main's refs (git -C main update-ref, writes into main/.git) — not fixed: the spec stops growing the shell gate and B6 (worker in the box) is the fix; the contract model depends on B6, said in the report — cost if wrong: until B6 a worker can delete or rewrite refs/orq/contrato/NN.
- Final: Ruling: I1 second half (a corrupt marker makes Bundle/MainRoot fall back to the clone) — not fixed separately: the marker is now protected from the worker's tools; remaining paths are interpreter scripts, open until B6 — cost if wrong: a script-corrupted marker opens main/.orq to tool writes.

## Review fixes

Fixes for `docs/harness-v2/reviews/step1-review.md` (verdict: corregir). Each fix started as a test that I saw fail.

| Commit | Findings | What changed |
|---|---|---|
| 7bbfd50 | ALTA 1, BAJA 5 | `gate.IsWorker` and `slice.LeerMarca` keep walking up past a `.git` that has no marker. A directory is a worker if any ancestor has `.git/orq-clon` or a `.git` file. An empty or relative cwd counts as a worker. `IsWorker`, `LeerMarca` and `CheckoutPrincipal` stop when `Dir(d) == d`. Tests: `TestGateDeniesInRepoNestedInClone` (`git init sub` and `git clone . sub` inside a clone: exam Edit, `orq slice examen --aprobado` from the clone and from main, and a write to main `.orq` are all denied), `TestIsWorkerCwdVacioORelativo`, `TestCheckoutPrincipalCwdRelativoNoSeCuelga`, `TestLeerMarca*`. |
| 374d3cf | ALTA 2, ALTA 3, MEDIA 4, tests | `verificar` never adopts. It returns `ErrSinContrato` naming `orq slice convertir NN --ratifico "<who and why>"`. Only `Convertir(nn, ratifico, out)` adopts, and it requires a non-empty ratification, which it stores in the contract (`ratifico`). `sha_rojo` and `sha_examen` must be 40-hex, equal to `rev-parse --verify <x>^{commit}`, and ancestors of `refs/heads/<Base>` in main. Before sealing it prints cmd, bundle, the red/exam commits and the ratification. It refuses while the slice lock is held, while the `Escritor` lease is held, or while a process has its cwd inside the worktree (`EnUso`, i.e. `sysinfo.Usan`). SHAs read from the state are shortened with `slice.Corto`, which cannot panic. `convertir` stays orchestrator-only in the gate (the case with `--ratifico` is tested). The usage text says the ratification is recorded but not authenticated until roadmap step 3. `TestEstadoViejoNoCorreGitEnElWorktree` now requires a receipt. Before this fix it never reached the exam, because `.gitattributes` is protected; the test now approves it with `Ampliar`. New tests: `TestConvertirRechazaExamenForjadoCoherente`, `TestConvertirExigeSHACompletoEnLaBase`, `TestConvertirSeNiegaConEscritorOProcesoDentro`. |

`validacion/e2e-v05.sh`: no case relied on automatic adoption, so the script is unchanged.

Consequences to know:
- No command releases the `Escritor` lease (only blocking, reopening and rehuella clear it). A 1.0.0 slice that still names a writer cannot be converted until something clears the lease.
- `sha_examen` is the slice-branch head when an exam was added mid-slice, so it is not on `Base`. Those 1.0.0 slices are refused and are closed as rejected.

`make ci` (exit 0): gofmt clean, go vet clean, `go test -race` 21 packages ok, e2e-v05 77 pass / 0 fail, aislamiento 14 pass / 0 fail / 0 pending.

## Review fixes 2

Fixes for `docs/harness-v2/reviews/step1-review-2.md` (verdict: corregir, item 3 and H4). Each fix started as a test that I saw fail.

| Commit | Item | What changed |
|---|---|---|
| 1fb3718 | 1 · trusted base | `Convertir` never takes `Base` from the state. `baseConfiable`: the project config's `base` (new optional C1 field, read from `<root>/.orq/config.json` only), else `main` if it exists, else `master`; a `slice/*` branch, the slice's own `Rama` or a missing branch is refused. A state whose `base` differs is refused, naming both. Tests: `TestConvertirBaseNoSaleDelEstado` (the reviewer's bypass with `base`=`slice/01`, and a `git branch x <commit>` alias made from the worktree), `TestBaseConfiable`. |
| 1684567 | 2 · ratification bound to what was shown | `orq slice convertir NN` without `--ratifico` is a dry run: it prints cmd, bundle, red commit, exam commit, trusted base and a 12-hex `code` (sha256 of those values), seals nothing, exits 1 (`ErrEnSeco`). Sealing needs `--ratifico "<who and why>" --codigo <code>`; the code is recomputed at seal time and a mismatch (wrong code, or the state changed after the dry run) is refused. Test: `TestConvertirEnSecoYCodigo`; CLI usage cases in `convertir_test.go`. |
| 8a2f47e | 4 · H4 residue | The in-use check (`enUsoSlice`) asks `EnUso` about the worktree `git worktree list --porcelain` reports for `refs/heads/<Rama>`, plus the state's path (a clone is not a git worktree). Test: `TestConvertirEnUsoMiraElWorktreeDeGit` (forged `worktree` field; before the fix it converted). |
| 28cc527 | 3 · stale lease | `orq slice soltar NN` (orchestrator-only in the gate) clears `Escritor` only when no process has its cwd in the slice's worktree (same check as item 4), and journals it (`diario`: `paso: soltar`, actor = the old writer). Tests: `TestSoltarLiberaEscritorViejo` (refused with a process inside, released and journaled without, then converts), gate case in `decisiones_test.go`. |

Still true: the ratification is recorded, not authenticated, until roadmap step 3; an exam commit added mid-slice is not on the base, so such 1.0.0 slices are refused.

`make ci` (exit 0): gofmt clean, go vet clean, `go test -race` all packages ok, e2e-v05 77 pass / 0 fail, aislamiento 14 pass / 0 fail / 0 pending.
