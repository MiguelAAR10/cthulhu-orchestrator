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

## Premise change

Owner decision, Acceptance 6 in `plan.md`: v2 does not inherit orq 1.0.0 slices. Legacy adoption failed three reviews, so it was removed, not patched. Every behaviour below started as a test that I saw fail.

| Commit | What changed |
|---|---|
| c8aebfc | `Load` refuses a schema-0 state (no `esquema`) with `S-NN was opened by orq 1.0.0; close it with orq 1.0.0 before using this version` (`ErrOrq1`). So verify, merge (`juntar`) and close (accepted or rejected) all refuse it before they touch the state or refs. Tests: `TestSliceOrq1SeNiega` (all four, with and without `--slice`), and e2e case S-30 (verificar, juntar, cerrar --aceptado/--rechazado, no contract written). That case fails 5 checks on the old binary. **Removed:** `orq slice convertir` and `orq slice soltar` (CLI, usage, gate list), `Convertir`, `Soltar`, `enUsoSlice`, `baseConfiable`, `enLaBase`, `codigoConversion`, `candadoLibre`, `ErrEnSeco`, `ErrProcesosVivos`, `ProcedenciaLegado`, the `ratifico` field in state and contract, the config `base` field (added only for convertir), the unused `aislado` state field, linked-worktree handling (`esEnlazado` in close, prune and the slice tip), and the 1.0.0 integration worktree (`intPath`, `int_dir`). Also removed: `legado_test.go`'s 13 adoption tests and `convertir_test.go` (its usage test is now `uso_test.go`, which asserts that nothing mentions convertir/soltar). A v0.4 `slice.json` is still moved, and is then refused like any 1.0.0 state. |
| 0c46b38 | Gate, review 3 finding B: a worker runs git only against its own repository, meaning the nearest marked clone upward (`gate.RepoPropio`, branch from the marker). Denied: `-C`, `--git-dir`, `--work-tree`, `-c core.worktree`/`include.*`, `--config-env`, `GIT_DIR`/`GIT_WORK_TREE`/`GIT_COMMON_DIR`/`GIT_INDEX_FILE`/object-dir variables as prefix, after `env` or in `export`, `GIT_CONFIG_*`, and a `cd` outside the clone. A path that is not literal is denied (fail closed). Also denied: `git config core.worktree|include.*`, and writes to `.git/config` and `.git/commondir`. `update-ref` (and `--stdin`), `symbolic-ref` writes and `branch -f/-M/-C` are denied on any ref that is not `refs/heads/<own branch>` or under it. `branch -D` was already denied by the destructive rule. 41 new deny rows and 14 allow rows in `TestDecisiones`, all deny rows seen failing first. `TestRepoPropio`, and `TestWorkerGitContraElRepoPrincipal` on a real clone, which fails with the rule switched off. |

Line count delta (11699b0..0c46b38, `harness` + `validacion`): +464 / −863, net −399. The removal commit alone is −736 net; the gate rule adds +337.

Known limits of the gate rule: it cannot see inside a script file it does not read (`sh x.sh`, `python x.py`). A symlink created in the same command line (`ln -s <main> l && git -C l …`) is resolved only once it exists, so the next call is denied but that same line is not. The rule covers the ref commands named in finding B; `fetch <src> x:refs/heads/main` and `checkout -B` act only on the clone's own refs.

`make ci` (exit 0): gofmt clean, go vet clean, `go test -race` all packages ok (170 pass, 0 fail, 2 skip), e2e-v05 86 pass / 0 fail, aislamiento 14 pass / 0 fail / 0 pending.

## Review fixes 4

Fixes for `docs/harness-v2/reviews/step1-review-4.md` (verdict: fix), in commit ee1b552. The approach denies whole classes, and the strongest layer comes first. Every bypass in the review is now a row that I saw fail before the fix.

| Finding | What changed |
|---|---|
| 4 · copied or renamed binary (structural) | `orq slice` refuses every orchestrator-only subcommand when `gate.IsWorker(os.Getwd())` is true, or when the cwd cannot be read. The list is `gate.SliceSoloOrquestador`, shared with the gate. A copy, a rename, an alias or a script that runs the real binary from a worker clone is refused by the binary itself. The gate rule stays as a second layer. Test: `TestSliceDelOrquestadorSeNiegaEnUnWorker` (verificar, podar, cerrar, `--slice 01 tomar`, abrir and examen are refused from a marked clone; `estado` is not). The cli `TestMain` now runs from a temp dir outside any repository, because the suite itself may run from a linked worktree. |
| 3 · podar | `podar` is in the orchestrator-only slice list (gate and CLI). |
| 1 · nested `.git` | Git from a worker is denied when any `.git` (directory, gitfile or symlink) lies between the command's dir and the clone root (`gitEntre`, on disk). `--git-dir`, `GIT_DIR` and `GIT_COMMON_DIR` must resolve to exactly `<clone>/.git`, because any other directory can carry a `commondir` that names main. A worker cannot create, write, move or link a path with a `.git` component, except its clone's own `.git`, through Edit, Write, MCP or the shell (literal and resolved path). It also cannot copy a source named `.git` (`cp`, `rsync`, `install`). Real-clone rows: a gitfile `sub/.git` (`git -C sub commit`, `cd sub && git update-ref refs/heads/slice/01 HEAD`, a deeper `cd sub/a/b`), a symlink `enlace/.git`, a `hondo/.git` with a `commondir`, and `--git-dir`/`GIT_DIR` at each one. |
| 2 · `-c`, `git config`, aliases | For workers, `-c` is allowed only for `user.name`, `user.email` and `color.*`. `git config` reads anything but writes only those keys. `-e`/`edit` and `rename-section`/`remove-section` are denied as whole forms. Also denied: any `GIT_CONFIG*` variable, the `GIT_*` variables git runs as a command (`GIT_EXTERNAL_DIFF`, `GIT_SSH[_COMMAND]`, `GIT_PAGER`, `GIT_EDITOR`, `GIT_SEQUENCE_EDITOR`, `GIT_ASKPASS`, `GIT_PROXY_COMMAND`, `GIT_EXEC_PATH`, `GIT_TEMPLATE_DIR`) unless the value is empty, `cat`, `true` or `:`, plus `--config-env` and `--exec-path=<dir>`. A subcommand that is not a git builtin (from `git --list-cmds=main`, minus the ones that serve or run a command: `daemon`, `shell`, `instaweb`, `remote-ext`, `remote-fd`, `filter-branch`…) is denied, which covers aliases and `git-*` programs on PATH. Commands that git runs from the line are denied too: `rebase --exec/-x`, `submodule foreach`, `bisect run`, `difftool/mergetool --extcmd`. |

Rows: 43 new deny and 13 allow rows in `TestDecisiones`. Five existing rows now expect the config reason. The new real-clone rows are in `TestWorkerGitContraElRepoPrincipal`, which now reports every row instead of stopping at the first.

Still allowed on purpose: reading config, `git -c user.*/color.*`, `GIT_PAGER=cat`, `git rebase -i` (the editor is the worker's own session), `git submodule update`, and writes inside the clone's own `.git` other than the marker, `config` and `commondir` (for example `.git/info/exclude`).

`make ci` (exit 0): gofmt clean, go vet clean, `go test -race` all packages ok, e2e-v05 ok, aislamiento 14 pass / 0 fail / 0 pending.

## Review fixes 5 (git allowlist)

Fixes for `docs/harness-v2/reviews/step1-review-5.md` (verdict: fix), in commit 62b73a9. This is the second miss on the same class (git builtins that run a command), so the premise changed: **for workers, git is an allowlist**, not a list of builtins minus denials.

| Finding | What changed |
|---|---|
| 2a · builtin options that run a command | A worker runs only `status, add, commit, log, diff, show, branch, switch, checkout, restore, rm, mv, reset, stash, rev-parse, ls-files, merge-base, cat-file, blame, grep, describe, shortlog, tag -l, worktree list, version, help` (`gitPermitido`, `repo.go`). Anything else, including aliases and `git-*` programs, is denied with a message that names the list. That puts `fetch`, `pull`, `clone`, `ls-remote`, `fetch-pack` (`--upload-pack`/`-u`), `archive --exec`, `submodule`, `bisect`, `rebase`, `am`, `apply`, `config`, `update-ref` and `symbolic-ref` out. `tag` only lists (no arguments, or `-l`/`--list`), and `worktree` only runs `list`. Within the list, `grep -O`/`--open-files-in-pager` and `--ext-diff`/`--textconv` on any subcommand are denied. The global-option checks stay as they were (`-C`, `--git-dir`, `--work-tree`, `-c` only for `user.*`/`color.*`, `--config-env`, `--exec-path=`, `GIT_*` variables, nested `.git`). `PAGER`, `EDITOR` and `VISUAL` joined the variables that git runs as a command. |
| 2a · refs | The own-branch rules for `branch -f/-M/-C` stay. `checkout -B` and `switch -C` now follow the same rule: they may force-create or reset only `slice/NN` or a branch under it. `reset` moves only the branch that is checked out. |
| 2b · global git config | Worker writes to `$HOME/.gitconfig`, `~/.config/git/config`, `$XDG_CONFIG_HOME/git/config`, `~/.git-credentials` and `/etc/gitconfig` are protected in the same way as `.git/config`: through Edit, Write, MCP and the shell, and including a recursive delete of a directory that contains them. Reading them is still allowed. |

Review 6 follow-up: `--output`/`--output=` (and git's abbreviations, e.g. `--outp`) are denied on every allowlisted git subcommand with a message to use `> file`, which the gate checks; deny rows cover `$HOME/.gitconfig`, `.git/config`, the exam and the slice state file in both forms, each seen failing first; the worker allowlist also gains read-only `reflog` (show/list), `ls-tree`, `rev-list` and `check-ignore`, with allow rows.

Review 7 fix: for workers, any git word before `--` that is dynamic (variable, `$(...)`, backticks, or a glob that can start with `-`) is denied as one class rule (`dinamicaAntes`), and `git reflog` is an allowlist (none, `show`, `list`, `exists` or a ref name; checked against git 2.43, `drop` exists from 2.50); deny tests assert the exact reason string.

Removed: the 110-name builtin list, `git config` parsing (`gitConfig`) and the per-builtin denials (`rebase --exec`, `submodule foreach`, `bisect run`, `difftool/mergetool --extcmd`, `update-ref`, `symbolic-ref`). Non-test code: +44 / −104 lines.

Rows in `TestDecisiones`: 31 new deny rows and 22 new allow rows. I saw all 31 deny rows fail before the fix. The deny rows cover every probe in the review (`ls-remote`/`fetch`/`pull`/`fetch-pack --upload-pack`, `clone -u`, `archive --remote --exec`, `grep -O`/`--open-files-in-pager`, and the `printf … >> ~/.gitconfig` fsmonitor write), plus `diff/log/show --ext-diff/--textconv`, `tag`/`worktree` writes, `checkout -B`/`switch -C main`, and each global config path. The allow rows cover the legitimate flow the reviewer listed (`status`, `add -A && git -c user.name=… -c user.email=… commit`, `log`, `diff HEAD~1`, `branch -f slice/01 HEAD`, `git -C src log`) and every other command on the list. The tests set a temp `HOME` and `XDG_CONFIG_HOME`. 33 existing rows changed their expected reason to the allowlist reason. Seven of them were allowed before and are now denied on purpose: `git config` reads and `user.email` writes (use `git -c user.*=…` on commit instead), `git rebase -i`, `git submodule update`, and `update-ref`/`symbolic-ref` on the worker's own branch (use `branch -f`).

`make ci` (exit 0): gofmt clean, go vet clean, `go test -race` all packages ok, e2e-v05 ok, aislamiento ok.

## Known gaps until step 7

The gate reads only the command line it is given. These stay open until the worker box of roadmap step 7:

- **Scripts the gate does not read.** `sh x.sh`, `bash x.sh`, `make`, `npm run`, a `Makefile`/`package.json` target, a test runner, or a git hook in the clone. Whatever they run is invisible.
- **Interpreters.** `python x.py`, `node x.js`, `go run`, and also inline code (`python -c`, `node -e`). For inline code the gate checks only that it does not name a protected path. It does not stop the code from running git, `orq` or `os.symlink`.
- **Anything run outside the parsed command line.** Values read at run time (`$(cat f)`, files, stdin) are denied when they are the command word or a path, but not when they are data a program acts on. Programs that run other programs from their own config (an editor, a pager or a tool configured in a file the gate does not see) are also missed. So is a `.git` created by a program the gate does not model (`tar x`, `unzip`, `git init/clone` into a subdirectory). After creation, git is still denied there by `gitEntre`, but a program that talks to the main repository directly is not stopped.
- **The binary refusal relies on cwd.** A program that `chdir`s out of the clone before it execs `orq` (for example `python -c 'os.chdir("/main"); os.execv(...)'`) is not refused. That is the interpreter gap again.
