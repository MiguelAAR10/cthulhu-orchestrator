# Cthulhu Harness v2 — Recap of every finding so far

Source: working sessions with the Owner (Nov 2026). Base commit: `1f2d0b0` of `cthulhu-orchestrator`.
Status legend: **[V]** verified in code or real data · **[I]** inferred.

## 1. What the product is for (Owner's vision)

- **Problem.** When many coding agents work in parallel, the team loses focus on the product's goal. We want speed without babysitting each agent.
- **Promise.** "Agents write the code. A program decides when it's done." Zero comprehension debt, zero drift from product value.
- **Not waterfall.** Specs change: developers find mistakes, the product asks for new things, things fail. Plans are living documents, never giant specs.
- **Method TCADD.** Translation → Context → Analysis → Development → Documentation.
  - T: interview the Owner (Matt Pocock style grilling) → user story in plain product language; classify the work (greenfield/brownfield, stage: in development/in production, business criticality).
  - C: scouts validate the story against the real repo and data. A minimal PRD in English, plain value language, with sub-documents = vertical slices.
  - A: technical implementation plan using engineering skills (pstack, Superpowers): modularity, patterns, no hardcoding; in brownfield, understand what exists and weigh trade-offs.
  - D: tasks small enough that one agent finishes each in a fresh session; parallel workers by dependency; TDD, adversarial tests, mutation tests where risk justifies; real product checks (Playwright). Rigor proportional to size, risk and business criticality.
  - D: minimal, atomic, solid memory so no role depends on its own session; visual evidence (before/after screenshots vs mockups) when the project has a UI.
- **Roles.** Atena (product filter, the Owner's only interface, defends the project even against the Owner's rushed reactions) · Main orchestrator ("sacred": opens sessions, never executes) · Planner · Adversary · Scouts · Workers · Independent verifier.
- **Platform-free.** Claude Code, Codex, Cursor, xAI, DeepSeek… configured from what models/subscriptions the user has. Cheap models for cheap work (docs → Haiku-class).
- **Skills.** A curated, validated gallery; a JSON with the `npx` install commands per project type and stack (e.g. TypeScript best practices, Python best practices); installed inside the project; loaded per phase and role, not all at once. Matt Pocock skills are verbose and token-hungry: use them at the front (T, product understanding), not everywhere.
- **Must survive death.** The project can be closed and revived from its memory, per role.
- **Hygiene.** Worktrees are removed after merge; sessions are closed; disk is protected.
- **International.** Everything in English.
- **Process.** Lean, flexible, reversible, agile, value-first.

## 2. Incident that motivated the hygiene work [V]

- `~/worktrees` reached 344 directories / 81 GB; WSL locked up repeatedly.
- Safe manual cleanup removed 251 items and freed 43 GB (measured with `df`), with bundles and verified evidence backups. Protocol and scripts: `~/worktree-cleanup-audit/` (`limpiar.py`, `nogit.py`).
- Lessons: ignored-but-unique evidence (`evidence/`, `.orq/`, `.m0/`) is deleted silently by `git worktree remove`; integration branches existed only locally; nested worktrees, broken worktrees without `.git`, root-owned files from Docker; nobody closes what orq did not create.

## 3. Our own audits (4 read-only agents) [V unless noted]

1. **Orchestrator bottleneck.** In `ventia-metricas`, the orchestrator is in 479 of 482 channel messages (99%). 16–18 hops per slice; mechanical round-trips ("load the plan and tell me"); ready tasks waiting for a human-like dispatch; duplicated messages.
2. **No worker supervision outside herdr.** A worker signals "done" only as free text in its pane; no result file, exit code or heartbeat. Creation is automatic, closing is manual; BLOCKED slices live forever; failed removals only go to `cuarentena.jsonl`; nothing orq did not create is closed.
3. **Memory overflows.** Cap 60 lines; real projects have 322 and 138. Auto "scope +" lines never expire; `Compact` cannot run when critical+confirmed exceed the cap; instincts rarely reach the 0.7 threshold; 358 un-pruned files in one `.orq`.
4. **Greenfield/brownfield is almost cosmetic.** 5+ code files ⇒ "improve" mode; only the recommended skill changes; role guides are identical.
5. **Skills.** Four collections installed: Superpowers, Matt Pocock, gstack, pstack (Lauren Tan / @poteto, `cursor/plugins`). About 10 conflict with orq's rules (agents spawning agents: `swarm`, `arena`, `dispatching-parallel-agents`, `poteto-mode`; shipping: `ship`, `land-and-deploy`, `canary`; `worktree-cleanup` uses `rm -rf`). Gaps: worker supervision/closing, red-acceptance-test authoring.
6. **Platform coupling.** 29 herdr call sites in 6 packages; the gate reads Claude Code hook JSON; `claude|codex` hardcoded; CLI and guides in Spanish.

## 4. External review (reproduced failures) — H-01…H-06

Confirmed by us in code: H-01, H-02, H-04. The rest are consistent with the code.

| ID | Failure | Where |
|---|---|---|
| H-01 | A worker can change the acceptance command (`slice examen` / `slice alcance` are not orchestrator-only; `--aprobado` is free text) ⇒ false VERIFIED | `gate.go:332`, `cli/proyecto.go:804` |
| H-02 | `orq slice lanzar` bypasses HALT, disk and quota | `gate.go:49`, `cli/v05.go:218` |
| H-03 | Verify and integrate apply different rules (skipped tests); the integrated SHA has no receipt of its own | `slice.go:537`, `slice.go:975` |
| H-04 | Closing one slice deletes the project-wide channel; verify and plan lose concurrent writes | `cli/proyecto.go:755`, `slice.go:559`, `plan.go:66` |
| H-05 | Reloading a plan keeps a task "done" even if its contract changed; `debatido: true` is a bare boolean | `plan.go:267` |
| H-06 | The hook may be missing in a worker's worktree; Codex never passes through `orq gate`; "worker does not delegate" relies on `agent_id` | `cli/v03.go:91`, `cli/v05.go:356`, `gate.go:90` |

Also: `presupuesto` is only text in the prompt; the scribe calls Claude without timeout; `instinto` raises confidence by repetition (recurrence ≠ correctness).

## 5. Independent critical score: 3.9 / 10 (realistic target ≈ 8)

Vision 8 · Engineering 6 · Alignment 4 · Roles/comms 4 · Memory/revive 4 · Lean 4 · Guarantees 3 · International 3 · Platform independence 2 · Resources 2.
Verdict: **partially aligned** — the vision asks for program-enforced guarantees, model independence and clean revival; the code offers bypassable guarantees, depends on herdr and Claude hooks, and accumulates state instead of recovering it.

Key critiques of the vision itself:
- A "sacred" orchestrator surrounded by LLM monitors is still a funnel; monitors must be code that emits events.
- Model independence cannot rest on hooks; the real guarantee must live in git + CLI (merge, verify, receipts) and OS permissions.
- Mutation/adversarial testing everywhere contradicts proportional rigor; trigger it by declared criticality.
- "Zero comprehension debt" needs a measurable proxy (one-page product receipt + approved demo + Owner can answer 3 questions about the slice).
- Too many roles for an MVP; Planner + Adversary can be one bounded adversarial pass.

## 6. Owner's latest design decisions

- **Sub-orchestrators per front**, lean: only when ≥2 independent fronts; depth 1; one-page brief down, one-page receipt up; the main orchestrator only validates product value and the integrated result.
- **Inside each front**: scouts answer concrete questions about the code the plan touches (evidence report) → planner ⇄ adversary (other model family, ≤2 rounds) → workers with prompts written by the planner (who has the context) → independent verifier.
- **`orq config agents`**: per-role agent/model selection, auto-detected from installed CLIs, Owner overrides always win; adversary must be a different family than the planner (enforced by code).
- **Project-type routing** ("if/for"): project type + stack + criticality ⇒ skills, flow and processes. Curated gallery with validated skills, added to the repo over time; skill analyzers to vet them.
- **Independent tester**: cannot change the tests or the product; the repairer is separate; evidence is not only "tests green" but real behavior.
- **Visual documentation**: Playwright screenshots, before/after vs mockup/target, minimal and organized; written by cheap models in business language.

## 7. Ideas from the Owner's reference videos (to evaluate, not adopt blindly)

- **pstack (Lauren Tan):** "the best spec is the code"; `how`/`why`/`teach` to learn a brownfield codebase; `architect` + multi-model `arena` + `interrogate` (judge and attack the diff); `create-verification` builds end-to-end proof environments; `no-comments` (agents trust comments over code); `deslop` diff cleanup; principles as small on-demand skills with an index (not one giant system prompt); `attack-the-premise` when fixes loop; reject impossible states.
- **Eric Tech "software factory" (Superboard):** builder → QA → reviewer agents with a kanban; QA uses the test pyramid (unit → integration/component → e2e); `ponytail`-style ladder ("should this code exist at all? reuse? stdlib?"); Context7 before using libraries; humanizer for plain language; UI refinement loop (diagnose → critique → fix → polish → re-diagnose); guard hooks for secrets.
- **Addy Osmani agent-skills:** six-stage SDLC (define, plan, build, verify, review, ship); `build auto` (approve plan once, then autonomous TDD tasks that stop on failure); anti-rationalization tables (the excuses an agent gives to skip a step, with rebuttals); doubt-driven development (fresh-context reviewer whose only job is to doubt a claim); reviewer personas; **ceremony must match the stakes** (button color ≠ PRD).
- **Common lesson:** do not install everything; fewer, sharper skills; skills encode *how we work*; prune as models improve.
