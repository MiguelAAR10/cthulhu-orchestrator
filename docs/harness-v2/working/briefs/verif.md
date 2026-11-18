# Brief A2 · honest verification and lanes in code (branch v2/verif, worktree ~/worktrees/cthulhu-verif)

Read /tmp/claude-1000/-home-miguel-projects-MiguelAR10-Builder-AI-Edge-Runners-experiments-cthulhu-orchestrator/97993ac3-28de-4b5f-887a-6d3b8a94f645/scratchpad/briefs/COMMON.md first.

## Why
The exam file is fingerprinted, but the verdict can still be gamed or be weaker than the README says. The lane router skill (harness/internal/cli/orqskills/orq-ruta/SKILL.md) exists only as a prompt.

## You own
harness/internal/slice/ (all files) · harness/internal/cli/proyecto.go · harness/internal/cli/plan.go · harness/internal/cli/metricas.go · the usage text for `slice`, `plan`, `metricas` lines in harness/internal/cli/cli.go (touch only those lines) · validacion/e2e-v05.sh · the orq-ruta, orq-orchestrator and orq-planner skill files and guias/ when a flag you add must be mentioned.

## Result required
1. **One verdict.** A single function decides pass/fail from (exit ok, Conteo, Omitir) and both `Verificar` and `integrar` (slice.go) call it. Today `integrar` ignores skipped tests.
2. **Counting is the default.** `orq slice abrir` refuses without `--reporte` + `--formato` unless `--sin-conteo "motivo"` is given; the reason is stored in the state and repeated in every receipt's note.
3. **Isolated run is the default.** `Verificar` runs the exam on `copiaAislada` of HEAD, with ignored dependencies provided the same way `prepararDeps` does for worktrees. Opt out per slice with `--en-sitio "motivo"` at open (stored, shown in the receipt). Keep `--aislado` accepted as a no-op for compatibility.
4. **Watched runner files.** At open, derive from `--cmd` the runner files that decide what the command means and exist tracked in the repo: npm/pnpm/yarn/npx/bun → package.json; make → Makefile; pytest/python -m pytest → pyproject.toml, pytest.ini, setup.cfg, tox.ini, conftest.py at root; vitest/jest/playwright → their root config files; `sh X`/`bash X` → X. Store them as `vigilados` with their blob hash at the red commit. At `Verificar`, if a watched file differs from the red commit, the run does not become VERIFICADO: it returns a clear message listing the changed files and exit code 3, unless called with `--acepto-runner "motivo"`, which is recorded in the receipt. It does not count as a failed repair.
5. **Lanes in code.** `orq slice abrir --ruta fix|slice` (default slice), stored in the state and in the closing record. In lane fix: `Sumar` (team zones) and `orq plan cargar` refuse with a message pointing to the upgrade; `orq slice ruta slice "motivo"` upgrades an open fix to slice and records the reason. `orq metricas` reports accepted count, first-attempt rate and mean hours per lane, and how many fixes were upgraded. Update the orq-ruta skill so its commands use `--ruta fix` and the upgrade command.
6. Tests for each of 1-5 in slice_test.go / proyecto_test.go; validacion/e2e-v05.sh updated and passing against your built binary (put it first in PATH; the script calls `orq`).
7. Old state files without the new fields keep loading (a slice opened by orq 1.0.0 must still verify and close).
