# orq2 pilot log · Real State (DANTE-OS)

Project: `~/projects/MiguelAR10-Builder/AI-Edge-Runners/products/real-estate/AI-Agent-RealState`, a throwaway idea the Owner allows breaking. All times are local (Lima, UTC−5). Sources:
- `.orq/diario.jsonl` and `.orq/canal.jsonl` in the project;
- `orq2 status` and `orq2 metricas`;
- the panes' cost lines;
- the Owner's messages for human minutes.

## How we work: two lanes, one owner each

| Lane | Where | Who | Rule |
|---|---|---|---|
| **Pilot** | Real State, through `~/bin/orq2` | Owner + Atena → Orchestrator → Planner/Astra → workers | One story at a time. The Owner acts at three points: tell the story, approve the card, decide on the page. |
| **Fixes** | Cthulhu, `~/worktrees/cthulhu-build` (v2/build) | Fable plans, Opus builds, the integrator (this session) runs `make ci` and reinstalls | Each pilot defect becomes a P-item with a red test first. Fixes never touch Real State. The pilot keeps running on the installed binary. |

**Cadence.**
- The integrator checks the pilot about every 15 min and appends to the timeline below.
- A fix batch reinstalls `~/bin/orq2` only after `make ci` passes, and never while a slice is verifying.
- Nothing is committed in Cthulhu while a `make ci` runs.

**Stop rule (unchanged).** Over 10 stories:
- median verified→decided ≤10 min;
- 0 undecided after 24 h.

Watched as well:
- card ready→approved ≤3 min;
- ≤1 exam correction per 10 stories.

## Schedule

| When | What | Who |
|---|---|---|
| 2026-11-19 21:10 | orq2 installed (24f2438); `make ci` at 21:26: 86/0, 14/0 | integrator |
| 21:13 | `orq2 init` + Python checks by hand; `orq2 check` 3/3 | integrator |
| 21:28 | S-01 starts | Owner + Atena |
| ~21:45 | Fix batch P1–P8 reviewed, `make ci`, reinstall | integrator |
| tonight | S-01: card → approval → worker → verify → decision | Owner at the card and the decision |
| next days | S-02 … S-10, one at a time | Owner |
| after S-10 | Apply the stop rule: continue, one page fix plus 5 stories, or back to 1.0.0 + CI | Owner |

## S-01 · First controlled outreach

Story (Atena, Owner-approved): the Owner picks 10–20 health prospects in `/admin/lanzamiento`, previews the exact message without sending, approves, and the send uses a Meta-approved template. **The real send is the Owner's act, outside the slice. No test sends a message; the box has no network.**

| Time | Event | Source |
|---|---|---|
| 21:20 | Atena reads the code alone: its scouts were blocked by the worker cap (7 of 4, foreign sessions) | Owner paste |
| 21:21 | Cap workaround: `max_workers` 12 | integrator |
| 21:28 | Atena → Orchestrator: story S-01 | canal c1 |
| 21:28 | Orchestrator pane opened (w4E:p2) | diario |
| 21:29 | Orchestrator: the 7 live workers belong to other projects | canal c2 |
| 21:29 | Scout: the code has no Meta template support (`kapso_channel.py:44-57`) | canal c4 |
| 21:30 | Atena: template send is in scope for S-01 | canal c5 |
| 21:30 | Planner opened (w4E:p3) with the story | diario, canal c6 |
| — | Card ready | pending |
| — | Card approved (`orq2 slice aprobar 01`) | pending |
| — | Worker launched | pending |
| — | Verified (receipt) | pending |
| — | Decided | pending |

## Pilot defects and frictions

| ID | Found | What | Status |
|---|---|---|---|
| P1 | 21:13 | `init` proposed no Python checks | fixed in caa3e01, not yet installed |
| P2 | 21:13 | `check --caja` did not prove uv deps offline; commands needed `--no-sync` + `PYTHONPATH=src` | fixed in 268a132; Real State config corrected by hand |
| P3 | 21:12 | `watch` >2 min over `~/worktrees` | fixed in 307ffb7 (timings in the fix report) |
| P4 | 21:14 | 3 inbound tests are environment-dependent and not a valid red | not code; story choice changed |
| P5 | 21:10 | `orq2 version` printed 1.0.0 | fixed in 9262893 |
| P6 | 21:16 | Foreign sessions counted as orq2 workers, which blocked Atena's scouts | fixed in 89177c6; workaround active |
| P7 | 21:33 | Fixed head-count cap of 4 (Owner: remove it) | fixed in 007d2cb: optional, RAM/CPU/load decide |
| P8 | 21:33 | Guide assumed Astra on Codex | fixed in 75bb0be |
| F1 | 21:29 | Atena relayed a worktree removal **with `--force`** to the Owner | rule: never `--force`. Owner told not to approve it. Check where the agent took it from. |
| F2 | 21:33 | Owner: policies are too rigid (Astra always, fixed scouts, fixed repairs); only the locks must be fixed | Fable designs criteria-based policies after this batch (backlog) |

## Metrics (from `orq2 metricas`, Real State)

| Story | Card ready→approved | Verified→decided | Exam corrections | Repairs | Owner minutes | Cost |
|---|---|---|---|---|---|---|
| S-01 | — | — | — | — | — | — |
