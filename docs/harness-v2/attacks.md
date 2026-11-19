# Attack index

One line per attack found so far in `reviews/step1-review*.md` and `working/inspector/informe*.md` (plus the final review of step 1, in `step1-report.md`). Each line points at the test that reproduces it, or says it is a limit and which review or report section accepted it. `harness/internal/attacks_test.go:TestAttackIndex` checks that every referenced file, test function, row fragment and shell case label exists.

Reading a reference: `file:TestName` is a Go test; `file:TestName/fragment` is a row of a table test (the gate tables have no subtests, so the fragment is a literal piece of the row's source, usually its command); `validacion/*.sh:label` is a case of a shell suite run by `make ci`.

Limits: until the worker box of step 2, the gate stops mistakes, not deliberate escapes (plan.md, "Reorder"; step1-report.md, "Close"). Authority typed as text waits for the passphrase of step 3.

## Worker clones and the gate

- **I1-1** informe.md §3 #1 · the gate does not see clones: exam edit, shell write, Agent, write into `.orq` and `cerrar` all pass → `harness/internal/gate/worker_test.go:TestGateDeniesInClone/edit of the bundle file`, `harness/internal/gate/worker_test.go:TestGateDeniesInClone/shell write to the exam`, `harness/internal/gate/worker_test.go:TestGateDeniesInClone/subagent launch`, `harness/internal/gate/worker_test.go:TestGateDeniesInClone/close the slice`, `harness/internal/cli/gate_clon_test.go:TestRealEnvEnUnClon`, `validacion/e2e-v05.sh:gate: un worker no cierra`
- **R1-1** step1-review.md §1 · `git init sub` / `git clone . sub`, then `cd sub`: `IsWorker` stops at the first unmarked `.git` → `harness/internal/gate/worker_test.go:TestGateDeniesInRepoNestedInClone`, `harness/internal/slice/marca_test.go:TestLeerMarcaAtraviesaRepoAnidado`
- **R1-5** step1-review.md §5 · empty or relative `cwd` makes `IsWorker`/`LeerMarca` loop and the hook fails open → `harness/internal/gate/worker_test.go:TestIsWorkerCwdVacioORelativo`, `harness/internal/gate/worker_test.go:TestGateCwdVacioORelativoNoSeCuelga`, `harness/internal/gate/worker_test.go:TestCheckoutPrincipalCwdRelativoNoSeCuelga`, `harness/internal/slice/marca_test.go:TestLeerMarcaCwdVacioORelativo`
- **RF-1** step1-report.md "Final review" · a worker moves, truncates or rewrites its own `.git/orq-clon` marker → `harness/internal/gate/worker_test.go:TestGateDeniesInClone/move the clone marker away`, `harness/internal/gate/worker_test.go:TestGateDeniesInClone/truncate the clone marker`, `harness/internal/gate/worker_test.go:TestGateDeniesInClone/write the clone marker`, `harness/internal/gate/worker_test.go:TestGateDeniesInClone/move the clone's .git`
- **RF-1b** step1-report.md "Final review" · the marker damaged by an interpreter script → limit until step 2 (accepted: step1-report.md "Final review", last paragraph)
- **RF-2** step1-report.md "Final review" · workers in clones were not counted against `max_workers` → `harness/internal/app/capacidad_clon_test.go:TestCapacidadCuentaWorkersEnClones`

## Orchestrator-only slice commands

- **I1-2** informe.md §3 #2 · the worker rewrites the exam and approves it with `orq slice examen … --aprobado` → `harness/internal/gate/decisiones_test.go:TestDecisiones/orq slice examen tests/s01.sh --aprobado`, `harness/internal/gate/decisiones_test.go:TestDecisiones/orq slice alcance src/ --aprobado x`, `harness/internal/gate/decisiones_test.go:TestDecisiones/orq slice rehuella`, `harness/internal/gate/decisiones_test.go:TestDecisiones/orq slice reabrir 01`, `validacion/e2e-v05.sh:gate: a worker in a clone cannot approve an exam`, `validacion/e2e-v05.sh:gate: nor by cd to the main checkout`
- **I1-2b** informe.md §3 #2 · the exam bundle changed after the red commit → `harness/internal/slice/slice_test.go:TestFixtureTocadoEsTrampaYBloquea`, `harness/internal/slice/slice_test.go:TestRehuellaNoBendiceTrampa`, `validacion/e2e-v05.sh:la trampa en el fixture se detecta`
- **R4-3** step1-review-4.md should-fix 3 · `orq slice podar w-front` from a clone → `harness/internal/gate/decisiones_test.go:TestDecisiones/orq slice podar w-front`
- **R4-4** step1-review-4.md should-fix 4 · a copied or renamed `orq` (`cp orq ./o && ./o slice verificar`) escapes the name match → `harness/internal/cli/gate_clon_test.go:TestSliceDelOrquestadorSeNiegaEnUnWorker`
- **RF-6** step1-report.md "Known gaps" · a program `chdir`s out of the clone before it runs `orq`, so the CLI refusal does not fire → limit until step 2 (accepted: step1-report.md "Close", known limits)

## Worker git against the main repository

- **R3-1B** step1-review-3.md item 1 B · `git update-ref refs/heads/main <fake>` from the worker → `harness/internal/gate/decisiones_test.go:TestDecisiones/git update-ref refs/heads/main abc`, `harness/internal/gate/decisiones_test.go:TestDecisiones/git update-ref -d refs/heads/main`, `harness/internal/gate/worker_test.go:TestWorkerGitContraElRepoPrincipal`
- **I1-3** informe.md §3 #3 · the worker rewrites `refs/orq/contrato/NN` or `slice/NN` in the main repo through git or by writing its `.git` → `harness/internal/gate/decisiones_test.go:TestDecisiones/git -C /x/repo update-ref refs/orq/contrato/01 HEAD`, `harness/internal/gate/decisiones_test.go:TestDecisiones/git update-ref refs/orq/contrato/01 HEAD`, `harness/internal/gate/decisiones_test.go:TestDecisiones/echo x > /x/repo/.git/refs/orq/contrato/01`, `validacion/aislamiento.sh:escribir <main>/.git/refs/heads/main`
- **I1-3b** step1-report.md "Final review" #2 · the same ref write done by a script or interpreter the gate does not read → limit until step 2 (accepted: step1-report.md "Final review" #2 and "Close")
- **R4-1** step1-review-4.md must-fix 1 · a gitfile `sub/.git` (or symlink, or commondir) pointing at the main `.git` → `harness/internal/gate/worker_test.go:TestWorkerGitContraElRepoPrincipal/git -C sub commit --allow-empty -m pwned`, `harness/internal/gate/worker_test.go:TestWorkerGitContraElRepoPrincipal/cd sub && git update-ref refs/heads/slice/01 HEAD`, `harness/internal/gate/worker_test.go:TestWorkerGitContraElRepoPrincipal/git -C enlace log`, `harness/internal/gate/decisiones_test.go:TestDecisiones/ln -s /x/repo/.git sub/.git`
- **R4-6** step1-review-4.md "two gaps" · a symlink created in the same command line (`ln -s <main> l && git -C l log`) → limit until step 2 (accepted: step1-report.md "Premise change")
- **R4-2** step1-review-4.md must-fix 2 · `git -c alias.x='!…'`, `git config alias.y`, `core.fsmonitor`, pager, sshCommand, hooksPath, textconv, filters → `harness/internal/gate/decisiones_test.go:TestDecisiones/git -c alias.x=`, `harness/internal/gate/decisiones_test.go:TestDecisiones/git config alias.y`, `harness/internal/gate/decisiones_test.go:TestDecisiones/git -c core.fsmonitor=/tmp/x status`
- **R5-1a** step1-review-5.md item 2 · `ls-remote`/`fetch`/`pull`/`fetch-pack --upload-pack`, `clone -u`, `archive --remote --exec`, `grep -O` run a command → `harness/internal/gate/decisiones_test.go:TestDecisiones/git ls-remote --upload-pack=`, `harness/internal/gate/decisiones_test.go:TestDecisiones/git fetch --upload-pack=`, `harness/internal/gate/decisiones_test.go:TestDecisiones/git clone -u /tmp/x . /tmp/c`, `harness/internal/gate/decisiones_test.go:TestDecisiones/git archive --remote=. --exec=/tmp/x HEAD`, `harness/internal/gate/decisiones_test.go:TestDecisiones/git grep -O/tmp/x foo`
- **R5-1b** step1-review-5.md item 2 · `printf '[core] fsmonitor=…' >> ~/.gitconfig`, then `git status` runs it → `harness/internal/gate/decisiones_test.go:TestDecisiones/>> ~/.gitconfig`, `harness/internal/gate/decisiones_test.go:TestDecisiones/echo x > /etc/gitconfig`
- **R6-1** step1-review-6.md must-fix 1 · `git log --output=<path>` writes any file (`~/.gitconfig`, `.git/config`, the exam, the state) → `harness/internal/gate/decisiones_test.go:TestDecisiones/--output=$HOME/.gitconfig`, `harness/internal/gate/decisiones_test.go:TestDecisiones/git diff --output .git/config`, `harness/internal/gate/decisiones_test.go:TestDecisiones/--output=tests/aceptacion/a_test.go`, `harness/internal/gate/decisiones_test.go:TestDecisiones/git diff --output /x/state/s.json`
- **R7-0** step1-review-7.md bullet 1 · quoted, escaped and other subcommands' spellings of `--output` (held in review 7) → `harness/internal/gate/decisiones_test.go:TestDecisiones/git log --"output"=x`, `harness/internal/gate/decisiones_test.go:TestDecisiones/git rev-list --output=x HEAD`, `harness/internal/gate/decisiones_test.go:TestDecisiones/git reflog --output=x`
- **R7-1** step1-review-7.md must-fix 1 · `O=--output; git log $O /path` turns a dynamic word into an option → `harness/internal/gate/decisiones_test.go:TestDecisiones/O=--output; git log $O x`, `harness/internal/gate/decisiones_test.go:TestDecisiones/git log $(echo --output) x`
- **R7-2** step1-review-7.md must-fix 2 · `S=expire; git reflog $S --expire=now --all` empties the reflog → `harness/internal/gate/decisiones_test.go:TestDecisiones/S=expire; git reflog $S --expire=now --all`
- **R7-3** step1-review-7.md must-fix 3 · `git reflog drop HEAD@{1}` deletes reflog entries → `harness/internal/gate/decisiones_test.go:TestDecisiones/git reflog drop HEAD@{1}`
- **R7b-1** step1-review-7b.md MUST-FIX 1 · `git log -1 -m $O`, `-F $O`, `--format "$O"`, `log -1 -m *` write a path through the global option-value exception → `harness/internal/gate/decisiones_test.go:TestDecisiones/O=--output=/p; git log -1 -m $O`, `harness/internal/gate/decisiones_test.go:TestDecisiones/git log -1 -F $O`, `harness/internal/gate/decisiones_test.go:TestDecisiones/git log -1 -m *`
- **R7b-2** step1-review-7b.md new · `X=-f; git branch -m "$X" slice/02` replaces another worker's branch → `harness/internal/gate/decisiones_test.go:TestDecisiones/X=-f; git branch -m "$X" slice/02`
- **R7b-3** step1-review-7b.md fix direction · `git commit -m -m $O` → `harness/internal/gate/decisiones_test.go:TestDecisiones/git commit -m -m $O`
- **R8-1** step1-review-8.md · a dynamic word becomes a `git commit` option (`-tm "$O"`, `-Sm`, `--template`, `--trailer`, `--cleanup`, `--fixup`) → limit until step 2 (accepted: review 8 orchestrator classification; step1-report.md "Close")
- **RF-5** step1-report.md "Known gaps" · hooks in the clone's own `.git` run on `git commit` → limit until step 2 (accepted: step1-report.md "Close"; reviews 6 and 7, "needs the step 2 box")

## Writes the gate does not read

- **R4-5** step1-review-4.md "two gaps" · scripts the gate does not read (`sh x.sh`, `make`, `npm run`, test runners) → limit until step 2 (accepted: step1-report.md "Known gaps" and "Close")
- **I1-5** informe.md §3 #5 · `awk 'BEGIN{print > …}'`, `python3 -c` with a built path, `curl -o`, `| tar xf -` write the exam → limit until step 2 (accepted: step1-report.md "Known gaps": interpreters and programs outside the parsed line)
- **I1-5a** informe.md §3 #5 · the literal forms of the same writes (`python3 -c "open('tests/aceptacion/…')"`, `dd of=`) → `harness/internal/gate/gate_test.go:TestRutasRealesYTrucosDeShell`
- **I1-5b** informe.md §2 · the exam's report is written by the same process that runs the worker's code; `--sin-conteo` leaves only the exit code → limit until step 2 (accepted: step1-report.md "Known gaps": anything run outside the parsed command line); the verdict rule itself → `harness/internal/slice/slice_test.go:TestVeredictoUnico`
- **RF-4** step1-report.md "Not done" · a link from a clone to main's `node_modules`/`.venv` would be writable by the worker → limit until step 2 (accepted: step1-report.md "Not done, and why", decision 5)

## Forged state and legacy slices

- **I1-F** informe.md §6 · a forged state file (cmd `true`, matching hash, fake receipt) then `cerrar --aceptado` → `validacion/aislamiento.sh:forja: main no avanza y el cierre se niega por el contrato en git`
- **R1-2** step1-review.md §2 · a forged schema-0 state with a deleted contract ref and a matching hash gets adopted → `harness/internal/slice/legado_test.go:TestSliceOrq1SeNiega`, `validacion/e2e-v05.sh:1.0.0 slice: $sub refuses`, `validacion/e2e-v05.sh:1.0.0 slice: no contract written`
- **R1-3** step1-review.md §3 A–B · `sha_examen`/`sha_rojo` point at a worker commit or a moving ref → `harness/internal/slice/legado_test.go:TestSliceOrq1SeNiega`
- **R1-3C** step1-review.md §3 C · a `sha_rojo` shorter than 7 characters panics on `[:7]` → `harness/internal/slice/marca_test.go:TestCortoNoEntraEnPanico`
- **R1-4** step1-review.md §4 · adoption runs while the 1.0.0 worker is alive → `harness/internal/slice/legado_test.go:TestSliceOrq1SeNiega`
- **R2-1** step1-review-2.md item 3 · `Base` read from the forged state → `harness/internal/slice/legado_test.go:TestSliceOrq1SeNiega`
- **R2-2** step1-review-2.md H4 · a live worker clears `escritor` and points `worktree` elsewhere to pass the in-use checks → `harness/internal/slice/legado_test.go:TestSliceOrq1SeNiega`
- **R3-1A** step1-review-3.md item 1 A · `config.json` `base`=`x` plus a forged state → `harness/internal/slice/legado_test.go:TestSliceOrq1SeNiega`
- **R3-4C** step1-review-3.md item 4 C · `git checkout --detach` plus a forged `worktree` bypasses the use check → `harness/internal/slice/legado_test.go:TestSliceOrq1SeNiega`
- **RF-3** step1-report.md "Final review" · verifying a legacy slice ran git in the worker's worktree → `harness/internal/slice/legado_test.go:TestSliceOrq1SeNiega`, `validacion/e2e-v05.sh:un archivo ignorado del worker no hace pasar el examen (copia aislada)`

## Authority typed as text

- **RA-1** informe-2.md part 2 §5, informe-4.md §3 · any role can type `--aprobado`, `--decision-owner`, `--acepto-runner`, `--owner` → limit until step 3 (accepted: plan.md "Design", human authority with a passphrase in step 3; step1-report.md "Review fixes")
- **R3-2** step1-review-3.md item 2 · the dry-run `code` is a sha256 with no secret, so a worker can compute it → limit until step 3 (accepted: step1-report.md "Review fixes 2")
