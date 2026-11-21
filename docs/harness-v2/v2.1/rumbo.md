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
