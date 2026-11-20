# Pilot fixes report

## v2/review-fixes: review P1-P8 findings, P11, agent-first step 4 and 1 (from 75bb0be)

Every change has a red test first, then the smallest fix, one commit each. orq 1.0.0 is unchanged: goldens and
version_test pass, every guide change is an orq2-only overlay applied when the guide is rendered
(`cli/guias_orq2.go`), and the `.md` files are untouched.

| Commit | What |
|---|---|
| fce0967 F1 | `DirSizeBytes` Lstats the root: a symlinked root is not followed, same as the serial walk |
| 7058c9e F2 | orq2: with pyproject.toml but no uv.lock, package.json decides; the uv.lock reason only shows when there is no Node |
| 95cd50b F3 | uv flat layout (no `src/`): ruff checks `.`, not only `tests/` |
| 9dd15eb F4 | single-quoted TOML strings name a tool |
| 5bbffcd F5 | uv probe skips the values of uv's value flags (`--with`, `--directory`, `-p`, …) and stops at `--` |
| 801c483 F6 | `check --caja`: `--no-sync` is checked and the probe runs on every `uv run` segment; a tool also named in a plain segment keeps its host lookup |
| d448bbe F7 | orq2 `validate` rejects `max_workers < 0`; `SinTope` is now `math.MinInt` (out of band), so orq's `-1` caps at 0 again, as in 1.0.0 |
| f7cddc8 P11 | orq2 Orchestrator, Planner and Atena guides and skills: no `--en-sitio` opt-out; exam flags proved with `orq2 check --caja`, never an own `sandbox exec`; card caps printed from the `slice/tarjeta.go` constants (`slice.TopesTarjeta`); worktree cleanup only with `git worktree remove` (no --force) and `git branch -d`, report to the Owner if git refuses. Steps 2-3 of the agent-first plan are folded in here: the Planner skill says `orq2 msg leer` (no `orq canal`), and Atena is told `slice aprobar` is the Owner's |
| b5b5c93 step 4 | orq2 `LeerTarjeta` returns every cap violation in one pass (`errors.Join`); orq still returns the first one, so accepted cards and orq messages do not change. `orq2 slice tarjeta --topes` prints the caps |
| 7965e1d step 1 | `cli/comandos.go` command table; `TestTablaIgualAlDespacho` checks it against `Run`'s source; `TestGuiasYSkillsSoloNombranLoQueExiste` (orq2 render). Before P11 it failed on exactly `orq-orchestrator/SKILL.md:25`, `guias/orquestador.md:8` and `orq-planner/SKILL.md:12` |

Not done: agent-first step 5 (refusals name the next command) was not started because of the 00:15 cut-off.
Step 6 was skipped as asked.

**Merge note (P9 on v2/build):** P9's Planner text says the card may be 25 lines. The validator caps at
`## Historia` ≤3 lines, ≤5 lines per other section, ≤200 characters per line and ≤400 words
(`internal/slice/tarjeta.go:30-35`). P11 does not touch P9's sentence and writes no literal caps: the guides print
`slice.TopesTarjeta()`. Reconcile P9's sentence with that output when merging.

Known environment failure: `cli.TestStatusMuestraMaquinaYCapacidad` reads this machine's live processes. With 14
claude sessions running, CPU becomes the limit and the test fails. It fails the same way on a `git archive 75bb0be`
copy, so it does not come from these changes.

Suite: SUITE_RESULT
