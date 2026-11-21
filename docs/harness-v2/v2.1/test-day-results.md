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
