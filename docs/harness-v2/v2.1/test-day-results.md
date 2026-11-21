# Test day 2026-11-21 · results (compressed: one slice, two routes)

Protocol: `test-day-slice-unico.md`. Evidence: `~/cthulhu-evidence/2026-11-21/`.

## Step 1 · freeze + dry check (11:00-11:20, read-only)
| Item | Value |
|---|---|
| orq2 | `2.0.0-pilot revision=795560fc4590630357254f444e2a7b555844e5fa modified=false` (`~/bin/orq2`) |
| Clones | `~/worktrees/rs-prueba-orq2`, `~/worktrees/rs-prueba-simple`, both at Real State `139fc2f`, clean (only untracked `.orq/`), no remote |
| `.orq/config.json` sha256 | `d9b3418730f38929…` in both (root ruff/mypy/pytest + `carpetas.dashboard` lint/tsc/vitest) |
| limits.json sha256 | `~/.config/orq2/limits.json` `993de8af01002a6a…` |
| Models | Opus builder + separate Opus reviewer, both routes |
| Skills | orq2 route: orq2 role skills; plain route: none beyond the repo's own (to confirm at launch) |
| orq2 doctor (orq2 clone) | exit 0 |
| orq2 check --caja (orq2 clone) | exit 0; probes root tools only, not `dashboard/` (v2.1 note) |
| Protocol commands in `orq2 --help` | all present: slice abrir/tarjeta/aprobar/lanzar/verificar/cerrar/reabrir, relevo, pendientes, despachar, doctor, check --caja |

### Checks per clone (logs `step1/{orq2,simple}-checks.log`)
| Check | orq2 clone | simple clone |
|---|---|---|
| ruff check + format --check | **FAIL**: `tests/test_s01_controlled_outreach.py` would be reformatted | **FAIL** (same) |
| mypy src | PASS | PASS |
| pytest | **FAIL**: S-01 red tests (e.g. `test_s01_controlled_outreach.py:372` opted_out filter) | **FAIL** (same) |
| dashboard lint / tsc / vitest | PASS / PASS / PASS (93 tests) | PASS / PASS / PASS |
| Skipped | 13 SQL tests (`DANTE_TEST_DB_URL` not set): SKIP, never counted as pass | same |

**Finding:** base `139fc2f` is S-01's red-scenario commit, so it is red by design. Its parent `f8029fe` ("feat: add commercial
visits vertical with published inventory bridge") is green: ruff PASS (167 files formatted), mypy PASS (113 files), pytest
exit 0 (SQL tests skipped). It already holds the visits logic the story touches (`src/dante_os/application/commercial_visits.py`).
**Proposal (needs Owner GO):** move both clones to `f8029fe` (detached checkout, `.orq/` kept), re-run Step 1 checks, then start.

### Base moved to f8029fe (filter GO, reversible, 11:00)
Both clones: `git branch -f main f8029fe`, on `main`, `.orq/` kept. Re-run (logs `step1/{orq2,simple}-checks-f8029fe.log`):
ruff, ruff format, mypy, pytest, dashboard lint/tsc/vitest all exit 0 in both; `check --caja` exit 0. Skipped: 80 SQL tests
(`test_engagement_cutover_sql.py` 71, `test_inventory_persistence_sql.py` 9), all `DANTE_TEST_DB_URL` not set: SKIP, never pass.

### Route setup (11:05-11:09)
- orq2 clone: `orq2 init --modo=mejora` (config unchanged; wrote `.claude/settings.json` hooks PreToolUse/SessionStart/PreCompact,
  role skills orq2-atena/orchestrator/planner/ruta, `.orq/proyecto.json`, untracked `docs/mapa.md`). 18 third-party skills reported
  missing; not installed (no downloads). Models set to the protocol: workers baja/media/alta and verificador = opus (planner, Atena opus;
  orchestrator sonnet coordinates only). New config sha256 `b2b41332cb0d2915…`.
- Found and cleared a stale global HALT (`2026-11-20T20:38 x`, disk had 467 GB free): likely a test leaking into the real
  `~/.local/state/orq2` (v2.1 bug). Copy in `step1/HALT-found-2026-11-20T2038.txt`.
- Each new Claude pane in the orq2 clone stops on Claude's "trust this folder (hooks)" dialog; the operator confirms it (setup,
  not Owner minutes). v2.1 note: `orq2` role launches should handle or warn about it.
- Story sent: plain route 11:06:23 (builder pane w47:p48); orq2 route 11:08:33 to Atena (w47:p49; first send at 11:07:59 lost
  to the trust dialog); orq2 Orchestrator w47:p4A. Operator log: `~/cthulhu-evidence/2026-11-21/operator-log.txt`.

## Owner answer to the "reservada" question (relayed 2026-11-21, pasted verbatim to both routes)
"A: solo las visitas confirmadas cuentan como reservadas. Una propuesta no bloquea el horario; las canceladas tampoco. Si ya hay una
confirmada en ese horario y edificio, no se puede proponer ni confirmar otra encima." Route Y's assumption matches it, so M3 (trap)
and M4 (kill + "continúa") on Y run in its review-findings round (filter-approved plan).

## Later (Owner product ideas, not this slice, given to neither route)
- Configurable Owner availability for visits.
- Visit reminders.
