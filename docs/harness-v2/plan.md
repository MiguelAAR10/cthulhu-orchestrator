# harness-v2 · plan

## Story
As the Owner, I want `orq` to enforce by program what the README promises, and to take the lightest path for small work, so that nothing reaches `main` without a real receipt and a brief fix does not pay the full protocol.

## Acceptance (Owner checks)
1. The forged-state demo ends rejected: a worker that rewrites slice state cannot get failing code merged.
2. The 18 audited gate evasions are blocked (one documented exception caught by verification); the 4 false positives pass.
3. A slice cannot be opened without counted tests unless a reason is given; verification runs on a clean copy of the commit.
4. `orq slice abrir --ruta fix` runs a fix with one worker and no plan; `orq metricas` shows time per lane.
5. `make test` and both scripts in `validacion/` pass in CI.
6. v2 does not inherit 1.0.0 slices (Owner, 2026-11-18, after three failed reviews of legacy adoption): it refuses a schema-0 slice with a message saying to close it with orq 1.0.0 first. Open 1.0.0 slices are drained with 1.0.0 before v2 is installed.

7. Recovery: a run interrupted mid-slice is resumed by a new session with no access to the previous conversation. It recovers the goal and current slice revision, the relevant decisions and open questions, the real commits, pending changes and processes, the evidence still valid for that state, and the next safe action. It checks memory against Git and the live run, and resolves or flags an ambiguous attempt before repeating anything.
8. Pilot: one real slice end to end on a real project (see Direction), with a recorded harness commit, at least one accepted correct result and one rejected known failure, each with a precise reason.

Out of scope: publishing, pushing, installing over `~/bin/orq` (Owner decides when), Orca adapter, hidden test variants.

Scale: large. Evidence behind this plan: audit of 2026-11-12 (gate probe, forged-state demo).

## Direction (Owner, 2026-11-13)
Goal of Cthulhu: build software with agents without losing product intent, context, decisions or control when requirements change or sessions stop. Independent verification is one part of that goal. `orq verify` as a separate product is a hypothesis to validate later, not a decision.
- **TCADD stays, proportional.** Translation (the user outcome and why it has value), Context (what exists, constraints, wrong assumptions), Analysis (one solution, an executable plan), Development (implement, check, integrate), Documentation (decisions and evidence, updated during the work, not at the end). Story, slice and plan point at one goal and do not restate it: the slice is a checkable product result, the plan is how it is built. A question is answered directly; a small fix takes one agent and targeted checks; dependencies, uncertainty or risk justify scouts, review and several workers.
- **Acceptance tests are protected within an attempt.** A legitimate requirement change opens a new revision of the agreement and says which work and evidence remain valid.
- **Roles.** Atena clarifies, reports state and argues decisions with evidence; it says when the Owner is right and explains when there is a confusion. The Owner can always pause or cancel. The Planner keeps the plan and proposes changes on new facts. The Orchestrator coordinates sessions, dependencies, limits and results with a small context: full logs and evidence live in files; its summary says what happened, what proves it, what is blocked, the next action. Permission and close rules are enforced by code. Worker briefs are runnable by a fresh session: goal, relevant context, constraints, dependencies, checks, expected result. Every extra role names the problem it solves.
- **Verification means:** a green check proves which checks passed; each check is tied to the behaviour the user expects. A skipped test or a runner change can be legitimate: the system tells an authorized change from one that voids the evidence. An infrastructure failure is reported as such, never as a product defect. Isolation states which resources a worker can reach and what is protected; the promise grows only after its tests run.
- **Order.** (1) B5 to a tested state, or an exact list of what is missing and what blocks testing; no open-ended chain of improvements. (2) Recovery test (acceptance 7) before any more instinct or automatic memory work. (3) Pilot (acceptance 8): exact harness commit and pilot goal first; minimum agents, one real worker, parallel only for independent tasks; product request → small slice and plan → real agent on an available provider → checks relevant to the change with fail-first evidence where it applies → interrupt and resume → independent verification bound to the evaluated code and contract revision → one accept and one reject with reasons. A requirement change during the pilot shows how the plan is updated and which evidence is rerun. (4) Decide with results. Builder activities after B5 (B6 aside, needed by the pilot) wait for the pilot result.
- **Measure.** Building cost and operating cost are separate. The pilot records result, total time, human intervention time, attempts, blocks, observable cost, problems found, wrong rejections, install and recovery effort; then compares with one agent + repo instructions + existing checks + CI (including Claude Code `TaskCompleted`/`Stop` hooks and protected-branch required checks). One run shows a flow works; a sample of comparable tasks decides habitual use.
- **Claims.** Report demonstrated capabilities, their limits and what is pending, separating source, interpretation and hypothesis. Not claimed: being the only tool that verifies, an unforgeable close, percentages of completion.
- **Credential.** The mechanism compatible with the existing subscription is `claude setup-token` (one-year OAuth token, model requests only, read from `CLAUDE_CODE_OAUTH_TOKEN`; not read in `--bare`). The Owner generates it and places it where orq reads it (`caja.credencial.claude`); it never goes through chat. No new subscription assumed.
- English for new work; renaming the executable is a separate decision with compatibility. No new round chasing a numeric score.

## Design (Owner approved 2026-11-18)
Source: external Fable review in four rounds and a usage scout over ~200 real slices (`working/inspector/`). This section replaces the builder order in "Technical plan › Activities"; rows there not named below are frozen.

**Product.** Cthulhu is the AI Edge Runners harness for carrying user stories with agents from several providers across the team's projects. Developers spend their judgement on details; a program enforces base correctness (lint, types, tests per stack). Real usage: 72–91 % first-attempt rate, 0 caught false "done" in ~200 slices; the pain is seeing how a story was understood before code exists, and accepting results in minutes. 12 of 13 second attempts were the Planner's exam being corrected; 75 verified slices were never closed; large work (VentIA M24) left the slice cycle.

**Owner vision (his words, summarized).** (1) Correct code by stack, configurable. (2) The heavy part is inspecting results; the filter reports during work, can pause the orchestrator, and can spin independent work in parallel. (3) Several LLM providers, not only Claude. (4) Project knowledge that a newcomer or another project can reuse, with recommended skills from our base and external finders. (5) Rails against unwanted features and slop; lean; story-driven.

**Approaches considered.** Pub/sub bus with a shared hive memory (rejected: N× token cost, duplicate actions, races on merge, memory drift; research in `working/research/orquestacion.md`). Orchestrator as thin coordinator plus an in-orq reactor and checkpoints for the human (chosen). Replace orq with CI + protected branches (kept as the pilot's baseline, not as replacement).

**Orchestrator: keeps** one registry story → exam → receipt → acceptance across agents and providers; confirms each worker really started (within 60 s); splits large work into child slices; exposes state as data for the filter; integrates serially. **Stops** being the mandatory step for mechanical work (orq merges and verifies itself on a worker's close), counting exam corrections as worker attempts, leaving slices in VERIFICADO, growing the shell gate, needing manual limit edits, and Planner/Astra consensus without a cap.

**Filter = Atena (no new role).** As a model it reports, asks, recommends and may pause (pausing destroys nothing). Only the human approves the story card, accepts or rejects, authorizes an exam change after approval, and loosens limits; those four need a passphrase typed in the Owner's own terminal, outside agent panes; orq stores only its hash and journals the signed event (limit: holds while no agent can read that terminal; a PR approval with the Owner's account can stand in on published repos). An exam correction is recorded and shown highlighted at close, not signed each time.

**Checkpoints.** Before code: understanding card (story in three lines, what will not be done, the exam in plain words with its file, zones touched, estimate, risks, open questions), ≤ 3 min. During: one line per slice, alerts only on drift (out of scope, exam change requested, worker did not start, no progress), ≤ 1 min. Before close: evidence pack (receipt with tests, lint and types; planned vs touched; exam changes highlighted; three-step demo; diff of critical zones only), ≤ 10 min.

**Excellent means** (measured by the Owner with `orq metricas` once step 2 lands; thresholds are starting points, tuned after the first measurement): review time from verified to decision, median ≤ 10 min · verified slices without decision after 24 h = 0 · stories with an approved card before the first worker commit = 100 % on the full lane · exams corrected after opening ≤ 5 % (to be tuned) · first attempt ≥ 85 % · receipts with counted tests, lint and types = 100 %, "skipped" never counts as pass · workers launched that never ran = 0, the same 5 stories pass with two providers · new repo to first accepted story ≤ 1 day · files touched outside the declared scope and new suppressions = 0 · integrations to base without a registry row = 0 · manual `limits.json` edits ≤ 1 per month · cost recorded for 100 % of slices.

**Roadmap** (one at a time; the next starts when the previous exit test is green on the integrated revision):
| # | Step | Exit test | Vision |
|---|---|---|---|
| 1 | Finish B5 + local CI (`make ci`, workflow file): gate recognizes clones; workers cannot run Owner/orchestrator-only slice commands from a clone; `cerrar` honours `ORQ_SIN_DOC`; the 7 failing slice tests fixed or rewritten for the decided behaviour | `go test -race ./...`, `e2e-v05.sh`, `aislamiento.sh --todo` green; new test: a worker in a clone tries `orq slice examen --aprobado` and is denied | 5 |
| 2a | Measure: stamps for worker-finished, verified, accepted; cause of every attempt (worker vs exam correction); cost when known; `orq metricas` | reproduces the usage report's figures from orq data without reading the channel | 2 |
| 2b | Remove the mechanical step: auto merge+verify on a worker's close, idempotent; start confirmation in 60 s; alert on verified-but-unclosed | a repeated close yields one receipt; a worker that never starts is flagged within 60 s | 2, 3 |
| 3 | Understanding card + human authority (passphrase); exam reviewed before opening; its correction recorded apart | 10 real stories: exam correction ≤ 10 %, approval median ≤ 3 min; a role without the passphrase cannot accept | 2, 5 |
| 4 | Recovery test (acceptance 7) | as defined in Acceptance | 5 |
| 5 | Correctness by stack: `orq init` records the project's lint, types and test commands in the contract; the receipt carries that block; no new suppressions | odontoflow backend and frontend: 100 % complete receipts; a planted suppression fails | 1, 4 |
| 6 | Measured pilot with one large story split into child slices | the "Excellent" criteria on 10 comparable tasks vs one agent + CI. **Decide here: continue, cut or stop** | 2 |
| 7 | Several providers for real: worker inside the box (B6) and a second provider | the same 5 stories pass with two providers; 0 "done" without running | 3 |
| 8 | Portable knowledge: project-knowledge doc reviewed once by a human; catalog open to external sources, pinned and verified, installed only with approval | a new repo reaches its first accepted story in ≤ 1 day; one project's rules apply to another of the same stack | 4 |
Until step 7, Codex workers run with no hook and no box: the pilot says so and uses repos where a mistake is cheap.

**Owner rules (2026-11-18):** Codex is never used, for any role (workers, reviews, Astra); the "second provider" of step 7 is to be named by the Owner. Fable plans and reviews only when the Owner asks; Opus builds; Sonnet/Haiku for mechanical work and scouts. One session at a time: a worker finishes and closes, a fresh Opus reviewer checks it, fixes if needed, the orchestrator integrates and runs `make ci`, then the next step.

**Loop rules (Owner, 2026-11-18, Lauren Tan's pstack criteria):** trust before parallelism — one worker at a time until a step is reviewed and green; a second worker only for an independent step with disjoint files, and only after the current one is approved. Strongest layer first: make the bad pattern impossible in code, then lint/types/CI, then tests, then prompts. Encode every repeated review finding as a permanent check (attack catalog row, lint rule, test). Prove it works on the real artifact (`make ci` on the integrated revision, by the orchestrator). After two failed fixes on the same item, attack the premise instead of a third fix, and escalate to the Owner.

**Step 1b (added 2026-11-18):** `staticcheck` (pinned) in `make ci`; one attack catalog test (`TestAtaques`) with a row per known attack (audit, reviews, exam trap), all blocked; mutation testing of orq's critical packages (`gate`, `slice`, `sandbox`) with `gremlins` (pinned), report first with a baseline, then the score may not drop; surviving mutants there are killed or justified in writing. Dev tools live in `~/go/bin`; orq stays standard library only. Mutation on the worker's diff for built projects: step 5, as a receipt report.

**Reorder (Owner, 2026-11-18):** the worker box (B6) moves to right after step 1b, as step 2, because four reviews each found a new way out of the command-text gate, and the box is also what lets workers share dependencies read-only (disk/RAM scout, `docs/harness-v2/working/research/disk-memory-2026-11-18.md`). Until the box lands, the gate stops mistakes, not deliberate escapes. B6 needs the Owner to run `claude setup-token` and place the token where orq reads it; never through chat. Measurement (2a) and later steps shift by one.

## Milestone direction (Owner, 2026-11-18 evening)

Next relevant progress: "this real agent completed this task inside the intended boundary, this result was accepted, this other one was rejected, here is the evidence".

**Close step 1.** Each review is pinned to a commit. Findings are classified as (a) blocks the gate's intended scope → fix now; (b) needs the step 2 box → documented limit; (c) can wait. A fix gets a review focused on that finding and its effects, not a new general audit. Containment is not called solved before step 2 proves it.

**Step 1b, bounded.** (1) `orq version` prints the build's VCS revision and dirty flag, and `make ci` asserts every suite runs the candidate binary it just built (today candidate and installed both print `1.0.0`). (2) The attack catalog is an index (`docs/harness-v2/attacks.md`): one line per attack found so far → the test that reproduces it. (3) Lint and mutation only where they catch a named risk before step 2: `staticcheck` (pinned) for ignored errors and dead branches in `gate`/`slice`; mutation on the few critical decisions (`IsWorker`, worker git allowlist, exam fingerprint check, bundle descendant check) with a hard time limit, report only. No open-ended quality campaign.

**Step 2 exit tests (box), positive and negative, with decoy files, no real secrets:**
1. The whole agent process tree and its tools run inside the boundary (bwrap namespaces; checked from inside and outside).
2. The agent can edit and test code in its clone.
3. It cannot modify the authorized contract, the protected state, or the controller's integration branch — decoys outside the clone must be unreadable or unwritable, and the ref the controller treats as authorized lives where the worker has no write path.
4. The bundle is received as a code proposal and checked against the expected base and scope.
5. A valid result advances; an invalid one is rejected with a reproducible reason.
6. Repeating critical operations or losing a reply causes neither a duplicate merge nor a blind relaunch.

**Technical trial at the end of step 2** (not product validation, not recovery, not parallelism): a throwaway repo, one real worker, one bounded functional change: launch → implement → return → independent verify → result; plus one known variant that must be rejected. Record time, attempts, Owner interventions and observable cost from this first run.

**Credential, prepared before asking the Owner,** tested with a dummy value: which account and config; where it is stored locally; how it reaches Claude Code inside the box; how it is kept out of logs, reports and commits; which network the agent needs and what stays blocked. Then a minimal procedure for the Owner to authenticate and store it from his own terminal. Respect existing account configs. A model credential never grants authority to close or merge in orq.

**Reporting.** Each report states: concrete result available; evaluated commit; binary path and revision; checks passed, failed, skipped, pending; remaining blocker; next test. Future capabilities are written "will be tested in step N", never "tested". The Owner passphrase (step 3) must say which decision it represents and how it binds to the change; a phrase in chat never replaces Owner identity nor turns pending evidence into approved evidence.

**Models:** Opus builds and reviews, Sonnet for small mechanical work and scouts, Fable plans only on request, never Codex. Reviews keep author/reviewer separation, clear criteria and executable reproductions.

## Route (Owner, 2026-11-18 late) — supersedes the order of "Design", "Reorder" and "Milestone direction"

Decision after the Fable value advice (`working/advice/fable-value-2026-11-18.md`). First value: less Owner attention per accepted story, measured as median minutes verified → decision (target ≤ 10) and verified slices undecided after 24 h (target 0; 75 today).

- **Frozen:** the gate as it is (mistake-stopper; no more review rounds or attack rows without a real incident). The box after Task 1 (`180ee54`, network + proxy); Tasks 2–7 resume only when the Owner names a second provider for an expensive repo or a worker damages something outside its clone, with a 2-round review budget.
- **Cut:** Owner passphrase (a plain command in his terminal is enough), quality levels, mutation on worker diffs, `instinto`, model-compacted memory, U1, L1, Orca, C6/W1 as separate pieces.
- **Frozen until the stop rule says continue:** recovery test, portable knowledge and skill catalog, second provider.
- **Order, one at a time (Opus builds, Opus reviews; Sonnet for wiring):** D1 decision page + timestamps → D2 correct code per project (lint/types/tests in `orq init`, receipt carries the three; remove Codex from config defaults) → D3 understanding card before code → D4 big stories as child slices, pause, drift alerts, orq merges on worker close → D5 second provider (Owner names it; never Codex).
- **Budget per deliverable:** at most 2 review rounds or about USD 15; then it goes back to the Owner.
- **Stop rule:** after 10 real stories through D1–D3: median ≤ 10 min and 0 undecided after 24 h → continue to D4/D5; 10–20 min → one fix, 5 more stories; > 20 min or the page did not replace reading the diff → stop building the harness, keep orq 1.0.0 + CI + protected branches.
- **Install for real tests:** v2 side by side as `~/bin/orq2`, only in projects the Owner chooses; `~/bin/orq` (1.0.0) untouched for live workers. The final rename stays a separate decision.

**Approach (Owner, 2026-11-19, from a talk on running ~50 agents): three mechanisms, mapped onto the route.**
1. *Feedback loop* — the worker checks its own work before saying done: the project's checks (D2) run in a loop inside the worker session (`orq check`), plus, for web stacks, driving the app (Playwright) guided by a per-project feature map (`.orq/mapa.md`: feature → route → how to reach it). Goes into D2.
2. *Skill evals* — a skill is kept or changed only by score: an eval set per skill (tasks + rubric), K runs in throwaway clones, a grader model scores, an edit is kept only if the score rises; skills per role are chosen from scores and stack, a small set, not everything installed. New deliverable D3b after D3. First check whether Claude Code's own plugin/skill eval covers it before building anything.
3. *Guardrails at merge, not at the prompt* — rules the agent cannot skip are deterministic checks in the receipt (lint bans per project, architecture boundaries like `internal/arch_test.go`, banned patterns); a failing check blocks the close. Goes into D2. This is why the command-text gate stays frozen.
Parallel work grows only with measured trust (D1 metric) and RAM caps (disk/memory scout).

**Frozen:** pub/sub bus and hive memory, second orchestrator, U1 `orq setup`, L1 translation, Orca adapter work, C6 and W1 as separate pieces (their useful parts are in 2b), full Q1 quality profile, mutation and held-out tests (until the pilot shows wrong acceptances), model-compacted memory and injected "instincts". Not claimed: catching cheaters as the core value; being the only verifier.

## Activities
| ID · criterion | Result and exit | Agent, model | Files | Depends · mode | Test and verifier | Worktree | State · commit |
|---|---|---|---|---|---|---|---|
| A0 · 4 | Lane router skill | orchestrator session | `orqskills/orq-ruta`, role skills, guides | none | `go test ./...` | no | integrated · 7f42430 |
| A1 · 2 | Gate: tokenizer, harness paths protected, no helpers from worktrees, MCP | worker-gate, Opus (security-critical parsing) | `internal/gate`, hook templates, gate funcs in `cli.go` | none · parallel | `TestEvasiones`, validar-mvp part A; verifier ≠ implementer | removed after merge | verified on integrated revision · b950633 (go test green, probe 27 BLOCK / 8 ALLOW as expected, validar-mvp 16/16, e2e-v05 74/74) |
| A2 · 3,4,6 | One verdict, counting and isolated run by default, watched runner files, lanes in code | worker-verif, Sonnet | `internal/slice`, `proyecto.go`, `plan.go`, `metricas.go`, e2e | none · parallel | unit tests, e2e-v05; verifier ≠ implementer | removed after merge | verified on integrated revision · ecb3859 (go test green, e2e-v05 74/74) |
| A3 · 1,6 | Worker isolation: clones, sandbox, state outside the repo, worker registry, migration | planner Opus → Astra → workers (rows added after analysis) | to be set by the plan | A1, A2 integrated · serial | forged-state e2e, live-slice migration test; independent review (high risk) | yes | analysis |
| A4 · 5 | CI, README and rulebook say only what is enforced | Haiku/Sonnet | `.github/workflows`, `README.md`, `docs/REGLAS-HARNESS.md` | A1–A3 | CI green | no | pending |

## Decisions from analysis (A3)
Planner plan and Astra critique consolidated here; Astra verdict: GO with a cut.
- `.orq` stays in the main checkout and is absent or read-only inside every sandbox; no state relocation in the first release.
- Worker repos are independent clones without shared or hard-linked objects; work returns as a git bundle that orq verifies and fetches. orq never runs git with its cwd in a worker repo.
- The exam, `--sano` and the integration re-run always execute inside the sandbox; no "sandbox off" switch.
- Closing never trusts a forgeable state: a state in CERRANDO or with an `integrado` SHA is re-checked against the trusted contract before publishing; legacy slices are converted with their old processes stopped and re-verified, and a contract without provenance needs ratification.
- Protected paths are enforced on the diff at ingestion, not only by the hook.
- Network: agent reaches only its provider API; exam has no egress. Live launch of a real sandboxed agent needs a dedicated credential from the Owner (not created by agents); until then it is tested with simulated workers.
- Later: state relocation, persistent worker registry, automatic worker channel spool, second provider, overlays.

## Proposals from research (not in scope until the Owner approves)
Sources consolidated from three scouts on 2026-11-13 (A2A spec v1.0, Claude Code agent teams docs, Google/MIT scaling study arXiv 2512.08296, MAST arXiv 2503.13657, Anthropic multi-agent write-up, Cognition, METR, Hermes Agent docs, pnpm/uv/git docs).
| # | Proposal | Value | Effort |
|---|---|---|---|
| P1 | Channel: sender identity derived by orq from the sandbox, not `--de`; pull inbox with the pane keystroke as a doorbell only; `cierre`/`estado` carry structured data the harness reads; auto idempotency key; messages to a closed attempt rejected. Borrow the A2A task/message/artifact model; do not speak A2A on the wire. | Removes impersonation and "may have arrived"; results become machine-checkable | M |
| P2 | Measure tokens, wall time, attempts and outcome per lane and role in `orq metricas` | The only way to know whether Planner, Astra and parallel workers pay | S–M |
| P3 | One worker by default; parallel only with declared disjoint zones and a reason | Evidence: multi-agent degrades sequential work, helps only decomposable work | S |
| P4 | Astra and scouts only for large or risky slices | Fewer handoffs on medium work | S |
| P5 | Dependency policy per ecosystem: read-only bind by default; when a task changes dependencies, private copy plus offline frozen install from a trusted store, re-installed by orq for the exam | No per-worker dependency copies; no egress | M |
| P6 | Per-role skill allowlist installed read-only in the sandbox (worker: TDD + debugging; verifier: verification; Atena: interview), without skills that own worktrees, merging or PRs | Less context, no second scheduler fighting orq | S–M |
| P7 | Decisions file per slice read by every worker and verifier | Less context loss between sessions | S |
| P8 | Held-out exam variants the worker never sees | Evidence of agents gaming visible tests | M |
| P9 | Quality profile per language in three levels (rapido, normal, estricto): no new suppressions, secrets on the diff, typecheck, new-dependency check, fail-first proof for added tests, mutation testing on changed files as a report; results in the receipt, a missing tool is never a pass | Evidence that tests detect bugs when nobody reads the code | M–L |
| P10 | `orq setup`: interactive terminal page in the AI EDGE RUNNERS look to choose driver, models per role, quality level, worker cap and RAM budget | Configuration without editing JSON | M |
P9 and P10 approved by the Owner on 2026-11-13 (activities Q1 and U1 of the builder). Practitioner sources: mattpocock/skills, unclebob/swarm-forge and clj-mutate, Lauren Tan's pstack, Trail of Bits on mutation testing.
Rejected: A2A over HTTP, agent cards, shared or hard-linked clones, reflinks (ext4 has none), partial clones, self-editing skills, agent swarms.

## Technical plan
Owner of this section: the builder (branch `v2/build`). Order C1 → C2 → C3 → B5 → B6 → Q1 → U1 → C6 → W1 → C4 → C5 → A4, one commit per activity, each green.

### Target layout and dependency direction
`cli` (flags, usage, printing) → `app` (use cases) → domain → adapters. Imports only point down; `internal/arch_test.go` parses every import and fails on an upward one.

| Layer | Packages | Holds |
|---|---|---|
| cli | `internal/cli` | Argument parsing and output. No git, no multiplexer, no model table. |
| app | `internal/app` (new) | `Proyecto`, launch of roles and workers, messaging, handoff and contract text, capacity, metrics. Receives its ports in a struct; tests pass fakes. |
| domain | `slice`, `plan`, `canal`, `memoria`, `instinto`, `perfil`, `gate`, `config` (new), `diario` (new), `watch`, `escriba` | State machines and rules. Pure on injected clock, git and sandbox where they decide acceptance. |
| adapters and base | `terminal` (+`herdr`, `orca`, `fake`), `agente`, `sandbox`, `gitx`, `sysinfo`, `orqdir`, `limits`, `state` | Everything that execs a process or reads the OS, and the two file readers the sandbox itself needs. |

What moves:
- `cli/proyecto.go` → `proyecto`, `abrirProyecto`, paths, `excluir` to `app/proyecto.go`; `porRol`, `arranque.completar` to `config`; `agentArgs`, `rolArgs` to `agente`; `lanzar`, the body of `runUp` to `app/roles.go` (idempotent open in `app/sesiones.go`); the body of `runMsg` to `app/mensajes.go` (C6). `runSlice`, `runMem` keep only flags and printing.
- `cli/v05.go` → `datosRelevo`, `Relevo.Markdown`, `contrato` to `app/relevo.go`; `lanzarWorker`, `registrarLlamador`, `workerArgs` to `app/workers.go` and `agente`; `memVigente` to `app/proyecto.go`.
- `cli/v03.go` → `fase`, `escribirFicha`, the checks of `runDoctor` to `app/estado.go`; `runInit`, `runSkills` stay in cli (they only install files and print).
- `cli/sesion.go` → `cerrarSesiones`, `panelDelSlice` to `app/sesiones.go`. `plan.Modelo` is deleted (C1). `internal/herdr` becomes `internal/terminal/herdr` (C2).

### Ports
| Port | Interface | Adapters |
|---|---|---|
| Terminal driver | `terminal.Driver`: `Dentro`, `PanelActual`, `CrearPanel`, `Arrancar`, `Avisar`, `Estado`, `Renombrar`, `Cerrar` | `terminal/herdr` (today), `terminal/orca` (C2, on the `orca` CLI), `terminal/fake` (tests) |
| Agent runner | `agente.Runner`: argv and env for an interactive agent of a kind, model, effort; `Preguntar` for one headless answer | `agente` for claude and codex; a simulated binary in tests |
| Sandbox | `slice.Caja`: `Command(ctx, sandbox.Spec, argv...)` | `internal/sandbox` (bubblewrap); no "off" adapter exists |
| Clock | `func() time.Time` passed in (`now` parameters stay) | `time.Now`; fixed times in tests |
| Git | `gitx.Git`: `Run(ctx, gitDir, args...)` with a timeout and a config that ignores user and system files | `gitx` on the `git` binary; always `--git-dir` of the main repo, never a worker repo as cwd |

### Rules held everywhere
1. Every external call (git, bwrap, herdr, orca, agent, exam) takes a `context` with a timeout; a timeout is reported as "unknown", never as success or as a plain failure.
2. Every state transition is written to the journal (`.orq/diario.jsonl`, append under lock, `diario` package) before the state file changes, with slice, attempt, actor, from, to and an operation id. Repeating a command with the same effect is a no-op that says so.
3. Lock order, always: `abrir` → `slice-NN` → `integracion` → file locks (`canal`, `memoria`, `diario`). No lock is taken while holding a later one; `slice` asserts it in tests.
4. No ignored error on a path that decides acceptance (verify, ingest, merge, integrate, publish, close): a `go/ast` test over those functions fails when the error of a git, sandbox, journal or save call is discarded.
5. `go test -race ./... -count=1`, `gofmt -l`, `go vet` clean on every commit. Standard library only.
6. A launch is two-phase: intent journaled → driver call → result journaled. An intent without a result is "unknown" and is resolved by `Estado` before anything is retried.

### Activities
| ID | What lands | Exit test | Files |
|---|---|---|---|
| C1 | `config`: defaults = today; per role and per worker difficulty an optional `entorno` (account variables such as `CLAUDE_CONFIG_DIR`, `CODEX_HOME`; paths only, a secret-looking name is an error) passed to the pane and to the headless escriba call; user file (`ORQ_CONFIG` or `~/.config/orq/config.json`) then `<root>/.orq/config.json`, merged per field, origin kept; unknown field is an error; `orq config ver`; `porRol` and `plan.Modelo` deleted; guides print models from config | `go test ./internal/config`; `TestDefaultsIgualesAHoy` pins the old tables; `grep` test: no model name outside `config` | `internal/config/*`, `cli/config.go`, `cli/proyecto.go`, `cli/v05.go`, `cli/skill.go`, `guias/*.md`, `plan/plan.go`, `escriba` |
| C2 | `terminal.Driver`, herdr behind it, Orca adapter, fake; `app` package with role and worker launch; timed-out start = unknown, checked with `Estado` before retry | `go test ./internal/terminal/... ./internal/app`: fake driver proves no second launch after a timeout; Orca adapter tested against a stub `orca` binary; arch test | `internal/terminal/**`, `internal/app/*`, `internal/agente/*`, `cli/*` |
| C3 | `sysinfo.Memoria/CPUs/Carga`; `app.Capacidad` = min(max_workers, RAM, CPU) with per-agent budget and reserve from config; gate and `slice lanzar` deny with the limiting factor; `orq status` shows RAM, CPU, disk, capacity | unit tests with fake `/proc` files; gate test denies on low RAM and says why | `sysinfo`, `config`, `gate`, `app/capacidad.go`, `cli/cli.go` |
| B5 | Independent clones (`--no-hardlinks`), bundle in, `refs/orq/sNN/*`; merge by `merge-tree` + `commit-tree`; exam, `--sano`, integration re-run inside the sandbox on `git archive`; protected paths checked on the ingested diff; contract kept in `refs/orq/contrato/NN`, compared on verify and close; CERRANDO and `integrado` re-checked; 1.0.0 slices refused (acceptance 6) | `validacion/aislamiento.sh` green without `--todo`, part 2 asserting the refusal reason; `e2e-v05.sh`; slice tests | `slice/*`, `gitx`, `gate` (worker detection, contract read), `cli`, `validacion/*` |
| B6 | PATH shim → `orq caja agente`; private agent config dir seeded by orq; credential only from `caja.credencial.<agente>`, else refusal naming the key; deps policy P5; skills allowlist P6 | `go test ./internal/app` with a simulated agent binary inside the real box; refusal test | `app/caja.go`, `cli/sandbox.go`, `config`, `agente` |
| Q1 | Quality profile in verification: levels `rapido`/`normal`/`estricto` per lane (`calidad` in config, command per language and gate); gates run in the box on the clean copy without network: no new suppressions or weakened strictness config, secrets on the diff (gitleaks or built-in patterns), typecheck/vet, new dependency in lockfile and trusted store, fail-first proof of added tests, mutation report. Receipt gains a `calidad` block; a missing tool is `omitido`; a failed hard gate is not VERIFICADO | `go test ./internal/calidad ./internal/slice`: each gate pass/fail/omitted on toy repos; a hard failure blocks like an exam failure | `internal/calidad/*` (new), `slice`, `config` |
| U1 | `orq setup`: full-screen page (termios via `syscall`, always restored) editing user or project config: terminal, agent/model/effort per role, worker model per difficulty, quality level per lane, max workers, RAM budget; shows sandbox self-test and free RAM read-only; line mode when not a tty or under 80x24; `--no-interactivo` prints the effective JSON; writes atomically through `config` validation and prints the diff | `go test ./internal/tui ./internal/cli`: 80x24 render within width for Unicode and ASCII art, key handling on a fake reader, line mode over a pipe | `internal/tui/*` (new), `cli/setup.go`, `config` (write path) |
| C6 | Identity from the box spool, `--de` ignored inside; inbox per role as source of truth, one-line doorbell; `datos` on `cierre`/`estado`; derived key; closed attempt rejects every type; task/message/artifact on disk | `go test ./internal/canal ./internal/app`: spoofed sender ignored, doorbell carries no body, stale attempt rejected | `canal/*`, `app/mensajes.go`, `cli` |
| W1 | `orq vigia`: resident watcher. Observes the driver's panes and the journal; detects stalled or dead workers and launches left unknown; pauses new launches when budget or RAM limits are passed; nudges through the channel doorbell; escalates to the Orchestrator and the Owner. It never accepts, merges or launches | `go test ./internal/app`: fake driver and journal fixtures for each detection; a test proves it calls no accept, merge or launch path | `app/vigia.go`, `cli/vigia.go`, `diario` |
| C4 | Journal-based metrics per slice, lane, role: wall time, attempts, verifications, sessions, tokens (best effort) | `go test ./internal/app` on a journal fixture; `orq metricas` shows the table | `diario`, `app/metricas.go`, `cli/metricas.go` |
| C5 | One worker by default; a second zone needs `--paralelo "reason"`; Astra and scouts gated by `--escala grande` or `--riesgo`; `.orq/decisiones/NN.md` in every contract; skills and guides rewritten | tests for the three gates; guides tests | `slice`, `app`, `guias/*`, `orqskills/*`, `skills/*` |
| A4 | GitHub Actions: gofmt, vet, `go test -race`, staticcheck if on PATH, `validacion/*`; README and REGLAS state only what is enforced and what a worker can still do | workflow file valid; scripts pass locally | `.github/workflows/ci.yml`, `README.md`, `docs/REGLAS-HARNESS.md` |

Order notes: Q1 follows B6 because it runs inside the sandboxed verification. U1 follows Q1, not C3, because it edits the quality level and the box credential key, which exist only after Q1 and B6. Later rows for Q1, listed and not built: lint, SAST with base comparison, dependency audits (need network, so outside the box), held-out tests (P8). B5 waits for `git merge harness-v2` (sandbox package), which lands before it by order.

## Checkpoint
- Phase: D. Integrated and verified on the integrated revision (tip 7b37f02 plus this commit): A1 gate, A2 verification and lanes, B1+B2 sandbox, C1 config, C2 terminal driver with account per role, C3 machine awareness. Evidence at 7b37f02: `go test -race ./...` green; e2e-v05 74/74; validar-mvp 18/18; aislamiento.sh 13 pass, 1 pending (forged-state close, owned by B5); `orq config ver` and `orq status` show configuration origin and RAM/CPU capacity.
- **PAUSED 2026-11-13 by the Owner.** All harness-v2 panes closed. Fable 7-day quota was at 99% at pause time (builder USD 22.2, architect USD 4.8).
- Builder: stopped mid-B5. Worktree `~/worktrees/cthulhu-build`, branch `v2/build`, WIP commit c592a25 (`go build` and `go vet` pass; `go test -race ./...` run by the orchestrator: every package green except `internal/slice`, 7 failures — two test behaviour B5 removed on purpose (`--en-sitio`, verifying in the worktree), three reject 1.0.0-shaped or test-written states as "state does not match the contract" (`sin_conteo`, `cmd`/`huella`: acceptance 6 is broken until fixed or re-decided), one expects a dependency link in the slice worktree, one drift warning missing; classification is the orchestrator's reading of the messages, not yet confirmed by the builder; new `slice/caja.go`, `cierre.go`, `clones.go`, `contrato.go`; `slice.go`/`verif.go` rewired onto clones, bundles and the sandboxed exam). Remaining in order: finish B5, B6, C6, W1 vigia, C4, C5, Q1, U1, L1, A4. Resume session: `cd ~/worktrees/cthulhu-build && claude-alt --resume 47b39bbc-83d6-4ef2-8ace-3ef8e1e8503f`.
- Agile design: v1 `working/diseno-agil.md` (architect, Fable), critique `working/astra/critica-agil.md` (Astra, GO with cuts). The v2 revision (`diseno-agil-v2.md`, ≤130 lines, answering Astra's HIGH findings) was NOT written: the architect was interrupted. Resume session: `claude-alt --resume 3d613b47-619e-46db-8778-d9f4f878c8b5` from the repo root, and ask it to write `docs/harness-v2/working/diseno-agil-v2.md`.
- Working notes (briefs, research, Astra critiques, spike, isolation plan, probes) copied from the orchestrator's scratch dir to `docs/harness-v2/working/`; briefs still name the old scratch paths, read them as `docs/harness-v2/working/…`.
- Proposals P1–P10 and the watcher approved by the Owner on 2026-11-13.
- Owner decision 2026-11-13: the repository is English (comments, identifiers, CLI messages, commands, docs, commits); only what a role says to the Owner stays Spanish. New work is English from now; activity L1 (builder, before A4) translates the rest, keeping the Spanish command names and JSON keys as accepted aliases.
- Integration branch: `harness-v2` (local only). Integration owner: orchestrator pane `w3X:p1`. Installed `~/bin/orq` 1.0.0 untouched; rollback = `main` 1f2d0b0.
- Review owed: independent review of B5/B6 (high risk) before acceptance.
- Pending Owner decision: dedicated credential for launching a real agent inside the sandbox.
- Spend: not measured by orq; the builder pane showed about USD 7.5 at its first ten minutes.
- 2026-11-18 17:45: roadmap step 1 integrated on `harness-v2` (merge ceb1560). Orchestrator ran `make ci` on the integrated revision: exit 0 — gofmt, vet clean; go test -race 165 pass, 0 fail, 2 skip (TestPrepararDeps skipped by decision until B6; one live herdr test); e2e-v05 77/0; aislamiento 14/0/0. Builder report `docs/harness-v2/step1-report.md`. Open: Owner decisions on legacy-slice adoption (automatic vs ratified) and on B6 timing (until B6 the git contract stops forged state files, not a worker writing main's refs); independent review of B5 still owed. Builder pane closed.
- 2026-11-18: Owner approved the Design section and asked to build it step by step. Now: roadmap step 1, plan `docs/harness-v2/step1-plan.md`; builder = Opus 5.5 session on the main Claude account, herdr pane `w47:pS` (Tasks 1–3 done by Fable before the Owner moved building to Opus: top models plan, they do not build), worktree `~/worktrees/cthulhu-build` at 57762a6; orchestrator pane `w47:p9`.
- Previous next action (superseded by the roadmap): (1) builder runs `go test -race ./...` on c592a25 and finishes B5 until `validacion/aislamiento.sh --todo` part 2 is green with the refusal reason; (2) architect writes diseno-agil-v2.md, orchestrator sends it to the builder as an addendum; (3) independent review of B5/B6; (4) orchestrator merges `v2/build` into `harness-v2` at each activity boundary and reruns gofmt, vet, race tests, e2e-v05, validar-mvp, aislamiento.
- Resume orchestrator: `claude-alt --resume 97993ac3-28de-4b5f-887a-6d3b8a94f645` from the repo root, or a fresh session reading this checkpoint.
- 2026-11-18 20:55: **step 1 closed** on integrated commit fdf8149 after 8 reviews (findings classified in `docs/harness-v2/reviews/`). `make ci` by the orchestrator on that revision: exit 0 (race 21 pkgs ok, e2e 86/0, aislamiento 14/0/0; 2 skips declared). Remaining limits listed in step1-report.md "Close". Containment not claimed: will be tested in step 2. Next: step 1b (bounded), Opus builder.
- 2026-11-18 22:45: **step 1b closed** on 0730e9f (`make ci` exit 0, candidate revision checked by each suite). Review: candidate check holds; branch-moving forms reclassified to step 2 (R1b-1). Step 2 plan committed with the Owner's decisions (account claude-alt; registries and skill sources allowed via proxy). Next: step 2 builder (Opus), task by task.
- 2026-11-18 23:00: Owner chose the value route (D1–D5), froze the gate and the box after Task 1, and v2 installs as `~/bin/orq2` for real tests. Next: D1 decision page (Opus), starting tomorrow.
- 2026-11-19 08:50: **D1 closed** on 2d4f5b8 (decision page, `orq status`, `orq metricas`, journal stamps). 2 review rounds, about USD 6.6 total (build 4.27, reviews 0.95 + 0.60, fixes 0.51 + 0.26). `make ci` exit 0. Pending: Owner's 5-slice check with `~/bin/orq2`. Next: D2 (project checks, `orq check` loop, merge-time guardrails, Playwright + feature map for web, Codex out of defaults).
- 2026-11-19 10:05: **D2a closed** on 7be03d7 (checks per project in `orq init`, `orq check` loop, checks + guardrails in the receipt and the page, toolchain in the box, Codex out of defaults). 3 review rounds, about USD 15 total (build 4.66, fixes 4.31 + 2.30 + 0.33, reviews 0.90 + 1.44 + 0.97). Next: D2b (Playwright + feature map for web stacks).
- 2026-11-19 11:40: **PAUSED by the Owner (out of RAM; he needs the machine).** State to resume from:
  - harness-v2 at fbe85e6: D1, D2a closed; D2b built + 2 review rounds; D3 built and merged (8881c9b), review 1 = fix. `make ci` exit 0 on 8881c9b.
  - v2/build 547be6a (worktree ~/worktrees/cthulhu-build): D2b review-2 fix committed by the builder (exam tests by name + `--forbid-only` through package scripts), NOT yet integrated or `make ci`-verified; the builder was stopped before writing its report.
  - v2/d3 c97002e (worktree ~/worktrees/cthulhu-d3): D3 review fixes WIP (launch checks the sealed contract, route sealed, zone `.` refused), stopped mid-work, NOT verified.
  - Next on resume: finish/verify D3 fixes, integrate both branches, `make ci`, rerun the D2b review-2 repro myself, close D2b and D3; then D3b (wire `claude plugin eval`, see scout notes) and D4.
- 2026-11-19 12:45: resumed after the RAM pause. **D2b closed** on d80cc88 (Playwright in the box, feature map `docs/mapa.md`, exam tests by name, `--forbid-only`). 2 review rounds + fix, about USD 17. D3 fixes resumed in `~/worktrees/cthulhu-d3`.
