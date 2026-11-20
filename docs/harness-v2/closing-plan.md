# orq2 closing plan

Fable, 2026-11-19 23:25 (Lima). Planning only. Built the way orq2 builds products: value metric → scouts → minimal plan → red first → build → independent check → Owner decides.

**Value metric of orq2 itself:** Owner minutes per accepted story. Proxy the harness can measure: median verified→decided ≤10 min over 10 stories (`plan.md:105`). Today: 0 stories decided.

**Evidence.** 4 read-only Sonnet scouts; I re-read 2+ citations from each (all matched, one line number corrected). Paths: `RS/` = Real State `.orq/`; `H/` = `~/worktrees/cthulhu-build/harness/internal/` at 75bb0be. Times are local; the logs are UTC. **[U]** = unverified.

## 1. Where S-01 stands

- Last event: 21:46:05, card pinned, Orchestrator asks the Owner to run `orq2 slice aprobar 01` (`RS/canal.jsonl:51`, `RS/diario.jsonl:14`). Nothing written since.
- No approval, no worker, no receipt. Slice clone clean at `139fc2f` (red test). Plan agreed. State on disk is consistent.
- The sessions that died at ~23:05 lost conversation only: all story state is in `.orq/` and git.
- Story to card took 17.5 min, of which ~6.5 min was box friction (E3, E4).

## 2. Error inventory, ranked by value to the pilot

| # | Error | Evidence | Sev | Call |
|---|---|---|---|---|
| E1 | The card is unreadable for the Owner, so the first Owner gate stalled (>95 min, target ≤3) | `RS/tarjeta-s01.md` ("422", "dedup_keys", "T1..T6") | High | **Fix now** = P9, in build |
| E2 | Fixes P1–P8 are built but not installed; the pilot runs the old binary (24f2438) | `pilot-log.md` defects table; `pilot-fixes-report.md` | High | **Fix now**: review → `make ci` → reinstall, while no slice is verifying |
| E3 | Exam box mounts only root `node_modules`: S-01's frozen exam is pytest only, its front zone (T5) has no boxed check | `RS/canal.jsonl:37,41`; K8 | High for S-01 | **Fix now by hand** (vitest outside the box, shown on the decision page). Code fix = P10 only if S-02 also touches `dashboard/` |
| E4 | Orchestrator offered `--en-sitio` (removed) and Atena authorized it | `RS/canal.jsonl:27,29,31`; `H/slice/nucleo.go:199` | Medium | **Fix now**, guide text only (P11). Where the agent learned the flag: **[U]**, not in the v2/build guides |
| E5 | Memory holds a false fact: "S-01 opened --en-sitio" | `RS/memoria.jsonl:12` vs `RS/canal.jsonl:31` | Medium | **Fix now by hand**: mark m12 superseded. Proves F2 is needed (after stop rule) |
| E6 | Planner proved exam flags with its own sandbox call, not the real open path: 1 failed open | `RS/canal.jsonl:33,35` | Medium | **Fix now**, guide (P11): prove with `orq2 check --caja` |
| E7 | Card size caps are learned from the rejection | `RS/canal.jsonl:47`; `H/slice/tarjeta.go:30-35` | Low | **Fix now** inside P9/P11 (caps written in the Planner guide). Whether P9 already covers it: **[U]** |
| E8 | Reboot recovery is untested for `relevo`/`contexto`; pane ids in `sesiones.json` predate the herdr restart (23:09) | `RS/sesiones.json`; no `relevo-*.md`; no test found | Medium | **Run it now by hand and time it.** P-item only if it fails |
| E9 | Kill inside `slice abrir` (between branch creation and state save) leaves a branch with no state; retry refuses | scout reading of `H/slice/nucleo.go` **[U]**, no test | Medium, rare | After stop rule, unless hit |
| E10 | `metricas` cannot measure Owner minutes, cost, scouts or facts reused | `H/cli/metricas.go:69` ("n/d") | Medium | **By hand** in the pilot-log table. Code after stop rule |
| E11 | "Owner only" on merge and on `--decision-owner` is not enforced in code; card approval rests on `ORQ_ROL` | `H/cli/proyecto.go:674`, `:647`; `H/cli/tarjeta.go:93`; K4 | Medium | After stop rule. Until then a process rule: only the Owner types `aprobar` and says "accepted" |
| E12 | Phase says IMPLEMENTANDO before the card is approved | `RS/diario.jsonl:13-14` | Low | Not a bug (label = slice open). Rename later |
| E13 | Astra is in the journal but not in `sesiones.json` | `RS/diario.jsonl:9-12` | Low | Probably closed after consensus **[U]**. Not a bug until shown |
| E14 | Two wordings of the stop rule's middle branch | `pilot-log.md:83` vs `plan.md:105` | Low | **Fix now**, docs: `plan.md:105` wins |
| E15 | F1: `--force` worktree removal relayed to the Owner | `RS/canal.jsonl:3,5` | High if run | Rule stands (never `--force`). Source of the advice **[U]** → P11 guide line |
| E16 | 2 `/tmp/orq-exec-*` dirs exist now; may be leaks from killed exams or live runs | `ls` at 23:23 **[U]** | Low | Integrator checks age; not a bug until shown |

K9 (uv probe answered by a host tool) stays "seen only with a symlinked `.venv`": not a bug for the pilot.

## 3. Locks vs policies

**Locks: fixed, never on the card, never relaxed by an agent.**

| Lock | Enforced at | Known hole (do not widen) |
|---|---|---|
| Frozen exam | fingerprint in git, checked at verify and close (`H/slice/nucleo.go:335-350`, `H/slice/cierre.go:305`) | Legit change only via `slice examen`, counted as an exam correction |
| Box without network | `--unshare-all` (`H/sandbox/sandbox.go:278`) | `--red` on `sandbox exec` not traced **[U]** |
| Gate | PreToolUse hook (`H/gate/gate.go:80-110`) | Matches by command name (K5) |
| Card approval | `PuedeLanzar` (`H/slice/tarjeta.go:157-165`); card frozen once approved | `ORQ_ROL` is an env var (K4); the `fix` route has no card |
| Owner merge | Convention only (E11) | No identity check |

**Policies: decided per story by a criterion, written before approval.**

| Policy | Today | Criterion | Exists already? |
|---|---|---|---|
| Scouts (how many) | ≥1 scout fact or `--sin-scouts "motivo"` (`H/cli/proyecto.go:463,482`) | One scout per open question that memory + skill do not answer; 0 is valid | Yes |
| Astra | Always on the `slice` route (`H/app/workers.go:92`) | Keep always for S-01…S-10: it cost 1.5 min in S-01 (21:36:44→21:38:10). Later: skip when 1 zone, ≤2 tasks, no money/outbound/migration/auth | No |
| Workers per slice | `MaxEquipo = 4` (`H/slice/slice.go:482`) | Number of independent zones in the plan; RAM/CPU/load cap it (P7) | Cap fixed |
| Repairs | `MaxIntentos = 3` (`H/slice/slice.go:57`) | Stop at the 2nd attempt with the same failure; more only by Owner word | `--decision-owner` |
| Model per task | Table by difficulty (`H/config/config.go:192-194`) | Planner sets difficulty per task; one tier up after a worker-caused failure | Yes |
| Exam timeout | 600 s (`H/slice/reporte.go:137`) | Measured green-base time ×3 | `--timeout` |
| Card size caps | 3/5 lines, 200 chars (`H/slice/tarjeta.go:30-35`) | Keep fixed: they protect the Owner's 2 minutes | n/a |

**Where it is recorded.** Before the stop rule: one plain line in the card's `## Preguntas`, e.g. "Proceso: 2 scouts (2 dudas nuevas), Astra sí, 1 worker, hasta 2 reparaciones". The Owner approves it with the card and the card freezes it (`H/slice/tarjeta.go:115`). No parser change: an unknown section is rejected (`H/slice/tarjeta.go:53`). After the stop rule: a `## Proceso` section plus contract fields, so it is tamper-evident.

## 4. Closing milestones

| # | Milestone | Done when | Today | What moves it |
|---|---|---|---|---|
| M0 | Pilot back up on the fixed binary | P1–P9 reviewed, `make ci` green, `~/bin/orq2` reinstalled, roles relaunched | 70 % | Review result; integrator |
| M1 | S-01 decided | Clear card approved, worker, receipt, Owner decision, product metric stated | 35 % | Owner at the card |
| M2 | Every story fills the metrics row | 11 columns of `pilot-log.md` filled for S-01, by hand where orq2 has no data | 20 % | Integrator, per story |
| M3 | Harness stable | 3 stories in a row with 0 new high P-items | 0 of 3 | Fewer defects per story: S-01 gave 8 P + 6 E |
| M4 | Stop rule | 10 stories decided; median ≤10 min; 0 undecided >24 h; card→approved ≤3 min | 0 of 10 | Owner time. **This is ~70 % of the distance.** |

**Honest total: ~12 %.** The harness is built (D1–D4 closed); what is missing is use. Each decided story is worth ~7 points; fixes alone cannot pass ~30 %. At one story per day, M4 lands about 2026-11-30; at two per day, about 2026-11-25. Both dates are **[U]** until S-01 gives a real duration.

Risk to the number itself: card→approved counts Owner absence. Rule: Atena pins the card only when the Owner is at the terminal. Re-pinning the clear S-01 card restarts that clock (`H/slice/metricas.go:167-173` uses the last `lista`).

## 5. Per-story loop and self-improvement

1. **Value.** Atena names one product metric with today's value. No metric, the story does not count.
2. **Context.** Scouts read `memoria.jsonl` + the project skill first, then ask only what is missing. Facts carry `path:line`; hypotheses are stored apart.
3. **Card.** Plain Spanish, ≤25 lines, with the "Proceso:" line. Owner approves.
4. **Red exam → build → boxed verification → decision page → Owner decides.**
5. **After the decision (integrator, ≤10 min):** spot-check 3 scout citations; correct or supersede false memory; add ≤3 evidenced lines to `.claude/skills/real-estate/SKILL.md`; log new P-items; fill the metrics row.

| Number to watch | S-01 | Target by S-05 | By S-10 |
|---|---|---|---|
| Scouts launched | ≥1, not recorded (4 scout facts) | ≤2 | ≤1 |
| Scouting minutes | not recorded | half of S-02 | ≤3 |
| Memory facts reused (cited in the card or plan) | 0 | ≥3 | ≥5 |
| False memory facts found in the spot-check | 1 (m12) | 0 | 0 |
| New high P-items | 8 | ≤1 | 0 for 3 in a row |
| Story → card ready | 17.5 min | ≤10 | ≤8 |
| Card ready → approved | >95 min (open) | ≤3 | ≤3 |
| Verified → decided | none | ≤10 | median ≤10 |
| Owner minutes (his own note) and cost (pane cost lines) | none | recorded 100 % | recorded 100 % |

Scouts, scouting minutes and facts reused are counted by hand from Atena's brief until the stop rule; the harness has no events for them.

## 6. Not before the stop rule

- Panel/UI, model picker, accounts, quota preflight, mascots, skill discovery (N1–N8).
- F2/F3 citation checking in code; scout, cost, or Owner-minute events in the journal; new `metricas` output.
- A `## Proceso` card section, contract policy fields, or configurable `MaxEquipo`/`MaxIntentos`/Astra.
- Stronger Owner identity (K4, E11), crash-injection tests (E9), second provider (K6), phase rename (E12).
- Any network access for the box, and any return of `--en-sitio`. If a check cannot run in the box, it is run by hand and shown as such.

Allowed now: P-items a story actually hit, each with a red test, one commit, fresh Opus review, `make ci`, reinstall between slices.

## 7. Next 48 h

| # | Action | Who |
|---|---|---|
| 1 | Finish the fresh review of P1–P8 and of P9; findings back as P-items | Opus reviewer |
| 2 | `make ci` on v2/build, reinstall `~/bin/orq2`; confirm `orq2 version` says 2.0.0-pilot | integrator |
| 3 | Recovery drill in Real State: `orq2 doctor`, then `orq2 relevo` per role. Time it; note whether `contexto` finds the role | integrator |
| 4 | Mark m12 superseded in memory; fix `pilot-log.md:83` to match `plan.md:105` | integrator |
| 5 | Re-pin the S-01 card in plain Spanish with the "Proceso:" line, only when the Owner is present | Planner via Orchestrator |
| 6 | Read the card (2 min), answer the "pidió visita" question, run `orq2 slice aprobar 01` | **Owner** |
| 7 | Build S-01; run vitest by hand outside the box and put the result on the decision page | workers; integrator |
| 8 | Decide S-01 on the page; note your own minutes | **Owner** |
| 9 | Fill the S-01 metrics row; 3-citation spot-check; first ≤3 lines of the project skill | integrator |
| 10 | Brief P11 (guides: card caps, no `--en-sitio`, prove flags with `check --caja`, never `--force`); P10 only if S-02 touches `dashboard/` | Fable plans, Opus builds |
| 11 | S-02 scouts start from memory + skill; count scouts and minutes | Atena's scouts |
| 12 | Choose S-02 and S-03 with a product metric each; say whether Astra-always stands for the 10 stories | **Owner** |
