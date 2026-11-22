# orq2 direction after the pause (synthesis, 2026-11-21)

Sources: one-pass reviews by Fable and Astra (`~/cthulhu-evidence/2026-11-21/rumbo/fable.md`, `astra.md`) of the
Owner's vision D14–D29 (`owner-vision-gaps.md`). Status: paused for token budget; this is the restart map.

## Core value (both agree)
Each request gets only the ceremony its size and risk deserve; the Owner chooses how far he participates; every
result comes with proof; what one request teaches (environment, index, rules) makes the next one cheaper.

## Agreed architecture
- **Build the new layer as Claude Code skills, not Go.** Conversation, router and product stages are text and
  criteria; the Go harness keeps what text cannot guarantee: sealed exam, zones, receipts, journal.
- **Experience (D14, D15, D16, D27, D29):** a `/cthulhu` entry with the ASCII octopus; a script reports configured
  and active models, machine capacity (`internal/sysinfo`), and quota when the status line provides it (Claude Code's
  status line receives model and, when available, usage and reset; otherwise "unknown" and ask the Owner). The Owner
  sets importance, time, finish level, autonomy and frontend criteria, per project and per request.
- **Router first.** Reuse the existing route guide (`harness/internal/cli/orqskills/orq-ruta/SKILL.md`: fix needs
  five conditions; uncertain → one question → slice). Add routes: question (0 new sessions), explore/decide only,
  design change, new product, and **small-but-risky** (payments, data, irreversible) which gets a separate reviewer
  regardless of file count. Routes only go up, never down by themselves; going up shows the cost first.
- **Product stages only on the product route:** one page each — product sheet, two options with trade-offs,
  reference repos (`gh search repos`, skills.sh: author, stars, maintenance, licence, tests, reusable pattern),
  inventory of existing art (which version the Owner approved), module boundaries and data ownership, flows or
  wireframes only when there are new steps or a visual choice, the minimum validated skills per stack. The Owner can
  stop at any stage with a decision; that counts as success.
- **Living context (D13, D23, D24):** one row per capability/component (entry files, test that proves it, service it
  calls, approved design link, last story, revision checked) plus a per-folder environment profile with one line per
  test class (unit, integration, e2e, multi-service: command, prerequisites, success signal). Preflight runs it on
  the base revision; at close the changed files mark their rows for review. Graphify on demand for large work only.
  Not built: vector DB, separate graph DB, per-file model-written tags, full regeneration per turn.
- **Learning loop (D26):** reuse `orq2 aprender` (lessons with confidence) and `skill eval`. A miss caught by the
  reviewer, exam or Owner becomes a candidate lesson with evidence; it becomes a rule only with Owner approval;
  prefer turning it into an automatic check; cap rules per role; prune rules that prevent nothing.
- **Metrics (D28):** add to the journal: proposed and effective route, reason, model, duration, who decided, who
  relayed. Close shows phase times, attempts by cause, interventions, and cost when measured.

## Where they differ, and the call
| Topic | Fable | Astra | Call |
|---|---|---|---|
| Short fix | 1 worker, no Atena/card/red test | agreed example + red test + receipt, Haiku | **Astra:** keep example + red test (it is the proof), drop card/plan/debate/demo |
| Agents for small routes | subagents inside the session | separate sessions for independence | **subagents for question/fix; separate reviewer only on risky routes** |
| First step | build `/cthulhu` skill | rehearse the router on paper with 4 requests | **Astra's order:** paper rehearsal → welcome skill → one real index row → product rehearsal, then harness work |
| Lean plan items 2 (re-seal) and 5 (session lease) | postpone | keep before unattended runs | **postpone until the Owner wants unattended runs** (both compatible) |
| Token estimates | 0.2–2 M per step | 1–45 thousand per step | Astra's are too low for any agent session; Fable's are closer. Measure on step 1 and recalibrate |

## Restart order (cheapest first; stop after any step)
1. Paper rehearsal of the router on 4 requests (a question, a label fix, a payment change, a new product). Haiku.
2. `/cthulhu` welcome skill: octopus, models/quota/capacity report, 5 choices saved to config. Sonnet.
3. One real index row + its environment/test profile; a fresh agent finds file, design and command alone. Haiku.
4. Product rehearsal on one pending Real State decision: two patterns from reference repos, the approved art,
   one validated skill; the Owner decides before anything is built. Sonnet + Haiku.
5. Lean plan 1, 3 (trimmed), 4, 6: honest attempts, preflight, approve chains dispatch, worker runs all checks. Sonnet.
6. One real fix end to end with metrics; compare with S-02.

## Reference: Google Cloud "Choose a design pattern for your agentic AI system" (read 2026-11-21)
Mapping to orq2, and what we take:
| Pattern | In orq2 today | Take |
|---|---|---|
| Single agent (start here) | question and fix routes | confirms: small requests stay single-agent, no team |
| Coordinator / hierarchical decomposition | Atena → Orchestrator → Planner → workers | keep only on feature and product routes (cost and latency grow with each level) |
| Parallel | workers by independent zone | keep; the gather step is the merge + sealed exam |
| Review and critique | independent reviewer, sealed exam | keep; cap revision loops |
| Loop / iterative refinement | repair attempts (max 3) | every loop needs an explicit exit (count, time, quality); add to the router a cap per route |
| Swarm (all-to-all debate) | the Astra/Fable debate rounds | avoid by default: most costly, risk of not converging; one-pass reviews instead |
| Human in the loop | card approval, close | becomes the autonomy setting (D16): which checkpoints pause |
Router criteria to add (from the guide's requirement questions): task open-ended or fixed steps; latency vs
thoroughness; inference budget; human involvement for high-stakes or subjective decisions. Context engineering terms
for D23/D24: isolate context per agent, persist across steps, compress. Security note for D1: per-agent identity and
least privilege. Not adopted: ADK and Google Cloud services (orq2 runs on Claude Code).

## Expert agents as tools (Owner question, 2026-11-21) — D30
Yes, with a rule: **skill = knowledge loaded into the agent that builds** (cheap, shares context: "how to build
with Angular/TypeScript/Supabase"); **agent-as-tool = a separate expert with its own short context, called for an
independent judgment or for research that would bloat the caller's context.**
- Start with at most 4, each a Claude Code subagent file with model, tools and a checklist:
  security/auth (any route, middleware, token, RLS), data/migrations (schema, constraints, races), frontend
  taste/accessibility (screens, mobile, consistency with approved art), reference-repo scout (search, compare, Haiku).
- Called by the router from signals (files or zones touched, route, risk), never on every request.
- Each returns a short verdict with evidence; only a High blocks. Its checklist grows from misses (D26).
- Kept only if it catches something real within N stories; otherwise removed (avoid agent sprawl and token cost).
Evidence it pays: in S-02 the missing session check on `/api/properties` was found by chance by the Orchestrator;
a security expert triggered by "new API route" makes that routine.

## Reference: "software factory that doesn't produce AI slop" (Antigravity talk, read 2026-11-21) — D31
Confirms: factory lines as markdown subagents invoked by the main agent from an ambiguous request (= router + D30);
isolated context windows in parallel to avoid compaction; the human as floor manager who can stop, steer or
message a worker; a `/learn` step that refines workers after each run (= D26); a final report of how each
subagent and skill performed (= proof page + `orq2 skill eval`).
New for orq2:
- **Project policy file** read by every agent: support targets (browser/runtime versions), quality bar, finish level
  (= D27 made concrete, one short file).
- **Read-only tools for reviewers and auditors** by definition, not by instruction (fixes E16).
- **Maintenance line ("trailing edge")**: remove dead code, polyfills and deprecated APIs, report dependency and
  code freshness from git history (feeds the index revision dates of D24). Less code = less for agents to read and
  review. Run on demand; scheduled runs only when tokens allow (recurring cost).
Not adopted: Antigravity/Gemini tooling.

## Reference: Zach Lloyd (Warp), two talks on self-improving software factories (read 2026-11-21) — D32
Definition worth keeping: **a code-defined, closed-loop system for continuously measuring and improving coding
agents.** Confirms: spec-driven work plus user-level verification evidence to cut review burden (= card + sealed
exam + screenshots); models and harnesses as swappable choices (orq2 already sets agent and model per role);
self-improvement proposed by agents as a change to the factory definition (= D26, with Owner approval).
New for orq2, in order of value for a solo Owner with a tight token budget:
1. **Factory as code:** one versioned folder that defines the factory — roles and their model, routes, skills per
   stack, expert agents (D30), policies (D31), checks. Today it is spread over `.orq/config.json`, global skills and
   the Go defaults. Gains: rollback, compare two configurations, agents propose a diff to it.
2. **Record every run** (conversation pointer, artifacts, screenshots, receipts, route, model, duration) somewhere
   durable, not in git-excluded `.orq/`. It is the raw material for scorers and benchmarks (closes the proof gap).
3. **Scorers (online evals):** after each story a cheap judge (Haiku) scores the run on named failure modes —
   redundant tests, out-of-scope files, unverified claims, environment loops. Turns the hand-made ledger
   (`orq2-defects.md`) into automatic detection; patterns that repeat feed D26.
4. **Benchmarks on our own stories:** S-01 and S-02 become replayable tasks to compare models/configurations on
   cost vs quality (e.g. are Sonnet workers enough for medium tasks?). Valuable but spends tokens: only when spare.
5. **Metrics:** cycle time, human interactions per story, cost per story and share spent on verification.
Not adopted now: cloud sandboxes, team-wide live view, Slack/Linear intake, AI-sovereignty infra — the speaker
himself says this is overkill below ~10 engineers.

## Reference: Lauren Tan (Poteto) with Matt Pocock, live, 2026-10-02 (read 2026-11-21) — D33
Confirms: trust is climbed with verification plus environment, not granted; "context, not control"; the engineer's
new job is the kitchen (skills, tools, constraints); domain expertise and precise words are the bottleneck (a
compressed word like "tautological tests" carries intent; grilling draws those words out).
New or sharper for orq2:
1. **A verification skill per project, with hands and eyes:** the agent runs the app and uses it like a user
   (clicks, traces, screenshots), and the deterministic parts live in a small script/CLI inside the skill so every
   agent reuses it instead of "rebuilding the world each time" — exactly the S-02 environment loops (D23).
2. **Every repeated agent mistake becomes a constraint in the environment** (lint rule, type, directory
   convention, registry) so it is impossible, not remembered. Conventional structure ("one feature, one directory")
   doubles as the index of D24.
3. **Mine past transcripts for interventions:** each time the Owner (or the filter) had to correct an agent is a
   candidate skill or lint rule. Cheap first step for D26: one Haiku pass over today's sessions.
4. **Autonomy by door type:** two-way doors (revertible, verifiable) can merge on autopilot and be reviewed by
   sampling after landing; one-way doors (data loss, payments, irreversible) keep a human gate. This is how D16's
   autonomy level should be decided, per change, not only per project.
5. **A buffer before fixing:** scanners append findings to a queue; a coordinator (or the Owner) looks for patterns
   before spawning fixes, so the root cause is fixed once (same idea as D32 scorers + ledger).
6. **Skills shrink to workflow:** with current models, drop implementation detail from skills; keep steps.
Not now: outer-loop triggers from Slack/X/email and 24/7 self-merging — need earned trust and spare tokens.
