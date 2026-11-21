# orq2 defect ledger (v2.1 input)

One row per defect seen while orq2 runs real stories. Append, never rewrite. Owner rule (2026-11-21):
"es clave ir anotando los errores de orq2". Evidence lives in `~/cthulhu-evidence/<date>/` (outside git).

Severity: **A** = would stop an autonomous run or let a wrong change in · **B** = costs Owner or agent time ·
**C** = friction. Cost = minutes lost or interventions, as measured.

| ID | Date · story | Defect | Seen as | Cost | Sev | Fix direction | Links |
|---|---|---|---|---|---|---|---|
| E01 | 11-20 test day · S-01 | Prompt typed into a pane but not submitted | worker idle with the prompt in its input box (twice); again 11-21 15:49 as `[Pasted text #1]` | ~41 min + 1.5 min | A | session launch confirms submission and retries; never treat "typed" as "delivered" | K-fix 1 |
| E02 | 11-20 · S-01; 11-21 · S-02 | No heartbeat or lease: session registry keeps dead panes | `orq2 sesion ver` lists closed panes as live; a question to Atena went to dead pane w47:p49 | 2 interventions; 9 min | A | lease per session, expired = dead; send fails loudly to a dead pane | K-fix 2 |
| E03 | 11-21 · S-02 | Dead registry entries cannot be removed or re-registered | `orq2 sesion cerrar` refuses names that `sesion ver` lists; operator edited `.orq/sesiones.json` by hand | ~2 min, manual edit | B | `orq2 sesion cerrar --muerta` and `sesion registrar`, both checked against Herdr | E02 |
| E04 | 11-20 · S-01 | Agents do not find the right command | agent missed `slice reabrir`, almost rejected a good story | 1 intervention | B | `orq2 ayuda "<intención>"` and a generated command dictionary (D5, D9) | K-fix 3 |
| E05 | 11-20, 11-21 | Trust dialog on every new clone | worker waits on "Do you trust this folder?" (4 times) | <0.5 min each, needs a watcher | B | orq2 pre-accepts trust for clones it creates | K-fix 4, D13 |
| E06 | 11-20 · S-01; 11-21 · S-02 | Sealed-exam box has no network: tools cannot start | 3 network tests deselected; pnpm/corepack tried to download itself and failed dashboard checks | 1 repair attempt spent, ~5 min | A | declared network policy per check; shared environment profile | D13 |
| E07 | 11-21 · S-02 | Environment failures consume repair attempts | box failure counted as repair 1/2 with no code defect | 1 attempt | A | receipt says "environment failed"; counter untouched | D13 |
| E08 | 11-21 · S-02 | No way to test the box without spending an attempt | `check --caja` probes only root uv tools, not `dashboard/` | operator had to rebuild the box by hand | B | preflight of every declared check on the base revision, not counted | D13 |
| E09 | 11-20 · S-01; 11-21 · S-02 | Worker clones come incomplete | empty `.venv`, no mypy; langgraph missing; worker asked to install | ~0.5 min + risk of installs | B | clones come ready: deps present or mounted, env profile copied | K-fix 6, D13 |
| E10 | 11-21 · S-02 | `slice aprobar` sends no event | orq2 Orchestrator did not notice the approval | ~1 min, 1 nudge | A | approve/close emit a channel event that wakes the Orchestrator | v2.1 journal bus |
| E11 | 11-21 · S-02 | Gate hook false positive on message text | Orchestrator → planner message blocked twice: "orq2 update es del Owner" | ~1 min | B | gate matches commands, not words inside message bodies | |
| E12 | 11-21 · S-02 | Astra step skipped silently | plan marked `debatido` with no Astra session | quality risk | A | `debatido` only with an Astra receipt for that plan revision | |
| E13 | 11-21 · S-02 | Planner's red test violates orq2's own suppression guard | `# type: ignore[arg-type]` in the exam file; caught only at verify | blocked S-02 at 3/3 | A | run the guard (and lint) on the exam before sealing it | |
| E14 | 11-21 · S-02 | A planner defect counted against the worker | the exam-file failure consumed the worker's repair counter | 1 attempt | B | blame by file owner: exam files → planner, environment → orchestrator | E07 |
| E15 | 11-21 · S-02 | Worker contract does not require the full suite | fix worker ran only the exam; an older worker test contradicted the new rule | blocked S-02 at 3/3 | A | contract: run every declared check before delivery | |
| E16 | 11-21 · S-02 | Read-only reviewer changed checkout state | independent reviewer ran `git checkout --detach` in the main checkout, then returned | risk, no damage | B | reviewer runs in its own clone or with a read-only mount | |
| E17 | 11-21 · S-02 | Test-day model overrides stayed after the test | all workers on Opus, including a low-difficulty migration | token waste | B | per-difficulty defaults restored automatically; status shows effective model per session | |
| E18 | 11-21 · S-02 | Agent memory records "fuente owner" for answers relayed by the filter | `orq2 mem add --fuente owner` for each Atena answer | trust risk (K10) | B | source = who typed it; relayed answers say "filtro por delegación" | G5/K10 |
| E19 | 11-20 | Stale global HALT left from a test | HALT 'x' from the day before | confusion | C | tests never write the real HALT; HALT shows its age and origin | |
| E20 | 11-21 · S-02 | API stream cut mid-task, no automatic resume | worker and planner "The response stopped arriving"; operator typed "continúa" | 2 nudges | B | Orchestrator detects a stopped session and resumes it once | E02 |

Not orq2 (operator tooling): the test-day watcher matched agents of another workspace and woke 79 min late (11-21 13:30–14:49).
