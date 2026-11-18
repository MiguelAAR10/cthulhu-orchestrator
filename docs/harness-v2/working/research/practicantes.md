# Practitioners: what orq should copy (scout r4, 2026-11-13)
Read first: orq README, plan.md Story/Acceptance/Proposals (P1-P8). Nothing modified.
Method note: WebFetch returns small-model summaries, not raw text. "V" = read on the primary page; "S" = secondary summary only; "I" = my inference.

## 1. Matt Pocock — mattpocock/skills (V)
Source: https://github.com/mattpocock/skills (27 skills, MIT, Markdown only)
- Grill first: `grill-me`/`grilling` interview the user until every branch of the design is resolved; README: use "every time you want to make a change". `grill-with-docs` also updates glossary + ADRs.
- Spec -> tickets: `to-spec` (conversation -> spec in the tracker, no interview), `to-tickets` splits into "tracer-bullet" tickets with blocking links (vertical slices).
- `implement` = build via `/tdd`, end with `/code-review` before commit. `implement-spec` runs a whole spec on one integration branch with parallel subagents.
- `tdd`: red-green-refactor in vertical slices. README: "a failing test first, then fixes the test".
- `code-review`: diff since a fixed point, two axes (Standards, Spec) in separate parallel subagents so neither biases the other.
- `handoff`: compacts a session into a doc for another agent. `retro`: suggests environment fixes after a session.
- Structural rule: "A user-invoked skill may invoke model-invoked skills, but never another user-invoked one."
Evidence: none beyond the repo itself; popularity claims (trending Sept 2026) are from aggregators (S): https://mcp.directory/blog/matt-pocock-skills-the-npm-moment-explained-2026
Enforceable by program: almost nothing. All skills are prompts. Not verified: whether AI Hero courses add checks; I did not open them (catalog: https://www.aihero.dev/skills-catalog).
Fit: orq already owns story, red test and ordering. Useful are the review split and the ticket shape.

## 2. Robert C. Martin (Uncle Bob) — mostly SECONDARY, state plainly
I could NOT reach his own posts/X thread. Everything below is third-party except the two repos.
- Claims (S): stopped reading agent-written code; replaces it with "extreme constraints": unit tests, gherkin tests, QA procedures, quality metrics, mutation testing, coverage (Digg summary https://digg.com/ai/xjqbsn4t ; https://startupfortune.com/uncle-bob-martin-says-he-no-longer-reads-ai-generated-code-and-the-developer-world-is-split/ ). He still draws module boundaries himself (S: https://www.besthub.dev/articles/uncle-bob-s-ai-coding-pipeline-5-agent-gauntlet-architecture-governance-9cdac721a063 ).
- "Gauntlet" (S, besthub summary, no primary link): Specifier (human reqs -> Gherkin + QA docs) -> Coder (unit tests + impl, red->green) -> Cleaner (CRAP analysis, split complex functions, DRY) -> Hardener (mutation testing, zero survivors) -> QA agent (UI-driven e2e). One `make gauntlet`; CRAP >8 returns work to the agent (human standard <=4); any dependency violation is a breach. Thresholds come from that summary only; unverified.
- SwarmForge (V): https://github.com/unclebob/swarm-forge . Roles in `swarmforge.conf` with a prompt each; packs: two-pack coder->cleaner, four-pack specifier->coder->refactorer->architect, six-pack adds hardening+QA. Each role in its own git worktree + tmux; handoffs via scripts and a `handoffd` daemon; `.swarmforge/` state "must not" be edited by agents except via helper commands; shared rules (engineering/workflow/handoffs) cannot be replaced by packs. README rule: "Do not pin prompt prose with automated tests. Test observable runtime behavior instead." The README does not state mutation or acceptance requirements.
- clj-mutate (V): https://github.com/unclebob/clj-mutate . Clojure/speclj mutation tester + Claude Code skill. Baseline must pass first (exit 2 otherwise); exit 3 = survivors or uncovered in-scope mutants; skips lines LCOV says are uncovered; snapshots in `.metrics/mutate/`; differential reruns; warns above 100 sites per file; `--update-manifest` marks everything killed without running tests and is "human override only". Workflow: one file at a time, no survivors before moving on. No CI gate described.
- Course: O'Reilly live course covers acceptance, unit, mutation testing, code quality analysis (S: https://www.oreilly.com/live-events/ai-agents-for-clean-code-with-uncle-bob-martin/0642572376765/ ). Cleancoders episode page lists "Testing and BDD", "Coverage and Mutation testing" only, no thresholds (V: https://cleancoders.com/episode/agentic-discipline-4 ).
Enforceable: mutation exit codes, coverage, CRAP/complexity, dependency rules, separate roles. Evidence: no measured study found, only his statements. Gap: "clj-mutate" is Clojure-only; for Go I did not verify a tool.

## 3. Lauren Tan (poteto) — pstack, Cursor plugins (V on repo summary, S on talk)
Sources: https://github.com/cursor/plugins/tree/main/pstack ; guide https://github.com/cursor/plugins/blob/main/pstack/docs/guide/README.md ; principles https://github.com/cursor/plugins/blob/main/pstack/docs/guide/08-principles.md ; talk summary https://redreamality.com/blog/lauren-tan-poteto-trust-before-parallel/ ; https://www.theneuron.ai/explainer-articles/pstack-explained-lauren-tans-system-for-trustworthy-ai-agents/
- Thesis (S): trust before parallelism. Strongest layer first: make the bad pattern impossible in the codebase; then static analysis (lint/compiler/CI: when a mistake recurs, add a rule); then rules/skills ("agents can skip it"); then human review; verification skills give evidence of correctness only.
- Principle 24 "Encode Lessons in Structure": advice repeated twice becomes a lint, check or script. 10 "Build the Lever": repeated manual work becomes a rerunnable tool. 19 "Sequence Work into Verifiable Units". 17 "Prove It Works": check the real artifact, not a proxy. 4 "Attack the Premise" after 2+ failed fixes. 12/13 boundary validation, type-system discipline.
- Verification skill per app: a CLI that launches the app, captures traces, flips flags, plus a "Feature Map" kept fresh by `/maintain-verification-skill`.
- Internal examples (S, no public docs): banned code comments so agents cannot paper over bugs; import-graph boundary rule.
- Evidence: self-reported ~2,000-2,462 PRs/month; no independent measurement. One third-party test: baseline ~30 min vs pstack ~1 h, pstack caught 3 false claims by the agent (theneuron, single anecdote).
- Not verified: no source ties her work to "compiler-style verification" beyond the lint/type layering above; I could not open the SKILL.md files, hooks or any rules file. The pstack guide page says nothing about hooks.
Enforceable: lint/type/import-graph rules, per-unit checks, idempotency checks, scripted verification. Prompt-only: most of the 24 principles.

## 4. Others
- Simon Willison, Agentic Engineering Patterns (S via search, pages not opened): https://simonwillison.net/guides/agentic-engineering-patterns/red-green-tdd/ ; .../first-run-the-tests/ ; .../agentic-manual-testing . Practice: confirm the new test fails before implementing (a test that passes at once proves nothing); start sessions by running the existing suite; passing tests do not prove it works, so exercise the feature (curl, Playwright), then turn any bug found into a permanent test. Evidence: his experience only.
- Trail of Bits, mutation testing for agents (V): https://blog.trailofbits.com/2026/04/01/mutation-testing-for-the-agentic-era/ . mewt/MuTON; agents configure campaigns and triage survivors; warns agents "may encode bugs as expected behavior" when writing tests from mutants. Cost: 5 min x 1,000 mutants ~83 h; equivalent mutants are noise. Claimed "~80% of insight for ~1% of manual work" is their estimate, not measured.

## What orq should adopt (ranked)
1. Fail-first proof for every new test a worker adds, not just the acceptance test (Willison, Pocock). Why: stops tests that pass before the change. S. Verification (orq reruns at the red commit). I: orq already does this for the exam only.
2. Mutation check on the worker's diff, scoped to changed files, run by orq in the sandbox, survivors = not done (Martin, Trail of Bits). Why: only evidence that tests detect bugs when nobody reads the code; ties to P8 (held-out variants). M-L. Verification. Caveat: need a Go mutation tool; not verified; budget runtime (83 h example).
3. Recurring review finding -> a mechanical rule (lint, import-graph, banned pattern) in the repo's check command (Tan #24). Why: rules in prompts get skipped. S-M. Verification + worker contract (orq lists the check command).
4. Review in two independent passes, Standards vs Spec, read-only, separate sessions (Pocock `code-review`). Why: cheaper than Astra on every slice; fits P4. S. Review.
5. Tickets as vertical "tracer-bullet" slices with blocking links, each ending in a check (Pocock `to-tickets`, Tan #19). Why: smaller worker tasks, matches P3. S. Plan.
6. Grill the Owner before the story is approved (Pocock `grill-me`), output into the existing story.md. Why: misalignment is the cheapest bug. S. Story. orq's interviewer skill already covers this; compare, do not add.
7. Complexity/CRAP and dependency-boundary ceilings as gates (Martin Gauntlet, S). Why: programmatic, no model. M. Verification. Thresholds unverified; set from orq's own data.
8. Verification skill per project: scripted launch + feature map for the demo (Tan). Why: checks real artifact. M-L. Verification/demo. Defer.
9. Protect state files: agents use helper commands only (SwarmForge `.swarmforge/` rule). Already covered by harness-v2 sandbox; no action beyond P1.

## What orq should NOT adopt
- Parallel swarms/competing-design arenas (`/arena`, `/swarm`) as default: conflicts with P3 and the plan's rejected swarms.
- Skills that own worktrees, merging or PRs (Pocock `implement-spec`, `pr`): second scheduler against orq (matches P6).
- Number of PRs/month as a success metric: self-reported, Tan herself says it was never a goal.
- Role prompts as the enforcement layer (SwarmForge roles, Pocock skills): prompts are skippable; orq's edge is the program.
- Equating "stopped reading code" with safe: Martin's claim is unmeasured; keep the human demo.
- Mutation-driven test generation by the worker: Trail of Bits warns it encodes bugs as expected behavior.
- A comment ban or lint set copied from Cursor/xAI: internal, undocumented (V: no public docs).

## Could not verify
Uncle Bob primary posts and the Gauntlet thresholds; Pocock's AI Hero courses; pstack SKILL.md contents and any hooks; Willison's pages beyond search summaries; a Go mutation tool.
