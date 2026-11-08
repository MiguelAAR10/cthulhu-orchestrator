# Skills & agentic-coding research for Cthulhu Harness v2

Scope: inputs for roadmap slices 7–9. Tags: [URL] = primary source read on 2026-11-08 · [RECAP] = our own `docs/v2/RECAP.md` · [inferred] = our design judgment.

## 1. Sources

### Matt Pocock — `mattpocock/skills`
Philosophy
- Most failures are misunderstanding, so the agent interviews you before it builds ("grilling") [https://github.com/mattpocock/skills].
- A shared glossary of domain terms (GLOSSARY.md, ADRs) makes agents shorter and more consistent [https://github.com/mattpocock/skills].
- Tight feedback loops (types, browser, red-green-refactor) and deep modules hold back entropy [https://github.com/mattpocock/skills].

Mechanisms for us: `grill-me` / `grill-with-docs` for phase T; `domain-modeling` + `codebase-design` (deep-module vocabulary) for A; `to-spec`/`to-tickets` as the T→C handoff; `handoff` for revive; `writing-for-agents` for authoring our own role guides.
Do NOT copy: grilling "before every change" [https://github.com/mattpocock/skills]. It costs a lot of tokens and contradicts ceremony-by-stakes, so use it only at the front of large work [RECAP]. `implement`/`implement-spec` overlap with our workers [inferred]. Don't install it both as a plugin and through npx, because that duplicates every skill [https://github.com/mattpocock/skills].

### pstack (Lauren Tan) — `cursor/plugins/pstack`
Philosophy
- "If you want to go fast, go deep first"; the goal is less code of higher quality [https://github.com/cursor/plugins/tree/main/pstack].
- The best spec is the code, so there are no separate planning skills [same].
- Mix models for their strengths. Principles are small on-demand skills behind an index [same; RECAP].

Mechanisms: `how`/`why`/`blast-radius` for brownfield scouts; `architect` (callers, types and module shape before crossing boundaries); `interrogate` (several models try to break a diff, and only proven findings are kept), which suits our adversary/verifier; `create-verification-skill` + `maintain-verification-skill` (a project-local proof of behavior, with drift repair), which is the backbone of slice 8; `no-comments`, `show-me-your-work` (decision TSV) for receipts.
Do NOT copy: `swarm`, `arena`, `poteto-mode` (agents spawning agents bypasses our admission function, H-02) [RECAP]. `make-bot-ui` is off-mission [inferred]. "No planning skills" conflicts with our living plan, so keep the plan and keep it short [inferred].

### Superpowers (Jesse Vincent) — `obra/superpowers`
Philosophy
- TDD first, systematic process over guesswork, simplicity [https://github.com/obra/superpowers].
- Verify before you claim success [same].
- Skills are mandatory: the agent checks for a relevant skill before every task [same].

Mechanisms: `writing-plans` (tasks with file paths and verification steps, which fit the "one fresh session per task" rule); `test-driven-development`; `verification-before-completion` (evidence before assertions); `systematic-debugging` for the repairer; `requesting-`/`receiving-code-review` (critical issues block).
Do NOT copy: `dispatching-parallel-agents` and `subagent-driven-development` (the orchestrator, not a worker, spawns) [RECAP]. `finishing-a-development-branch` merge/discard is the job of `orq` code, not the LLM [inferred]. `using-superpowers` (mandatory skill check on every turn) burns tokens in cheap roles [inferred].

### Addy Osmani — `addyosmani/agent-skills`
Philosophy
- Agents default to the shortest path and skip specs, tests and security [https://github.com/addyosmani/agent-skills].
- A skill is a process with checkpoints, exit criteria and verification, not prose; "seems right" is never evidence [same].
- Six-stage SDLC: define, plan, build, verify, review, ship [same].

Mechanisms: **anti-rationalization tables** (each excuse paired with a rebuttal) in every skill; **doubt-driven-development** (CLAIM → EXTRACT → DOUBT → RECONCILE → STOP, in a fresh context, escalating to another model only with user authorization) [same]; `/build auto` (one plan approval, then per-task test+commit, pausing on failure or risk) [same]; `source-driven-development` / `context-engineering`; `browser-testing-with-devtools`.
Do NOT copy: `/ship`, `shipping-and-launch`, `ci-cd-and-automation` as autonomous actions, because nothing ships without the Owner [ROADMAP]. The pack has no light path for low-stakes changes [https://github.com/addyosmani/agent-skills], so we must add one.

### Eric Tech "software factory" (video, no primary repo found)
- Builder → QA → reviewer on a kanban; QA follows the test pyramid; a "should this code exist?" ladder; Context7 before libraries; a UI loop (diagnose → critique → fix → polish → re-diagnose); secret-guard hooks [RECAP].
Do NOT copy: the kanban as a second source of truth (use `.orq` state files) [inferred].

### Andrej Karpathy
Philosophy
- Software 3.0: prompts are programs, and LLM apps are "partial autonomy" products with an **autonomy slider** (Tab → agent mode, Tesla L1–L4) [https://www.latent.space/p/s3].
- **Make verification easy and fast, keep the AI on a tight leash for generation**; small increments, because the AI generates faster than humans can verify [https://www.latent.space/p/s3].
- **Context engineering**: filling the context window with just the right information for the next step [https://x.com/karpathy/status/1937902205765607626]. "Vibe coding" is fine for throwaway weekend projects only [https://x.com/karpathy/status/1886192184808149383].

Mechanisms: criticality as the autonomy slider; diff size limits per task; a fast gen-verify loop (receipt + demo); a minimal context per role.
Do NOT copy: vibe-coding mode for anything above "trivial" [inferred].

## 2. Synthesis: TCADD × role × ceremony

| Phase | Role | Best mechanism | Trivial | Small | Large | Production-critical |
|---|---|---|---|---|---|---|
| T | Atena | `grill-me` (Pocock); pstack `in-your-own-words` | 1-line restatement | `in-your-own-words` | `grill-with-docs` + glossary | grill + Owner signs the 3-question receipt |
| C | Scouts (cheap model) | pstack `how`/`why`/`blast-radius`; Context7 | skip | `blast-radius` | evidence report per plan question | + data/prod checks |
| A | Planner ⇄ Adversary | Superpowers `writing-plans`; pstack `architect`; Pocock `codebase-design` | none | 1-task plan | plan + 1 adversary round | 2 rounds, different model family (code-enforced) |
| D-dev | Workers | Superpowers TDD; Addy anti-rationalization; Karpathy small diffs | direct edit + test | TDD | TDD + red acceptance test | + mutation/adversarial tests |
| Verify | Tester / Repairer / Verifier | pstack `create-verification-skill`, `interrogate`; Addy doubt-driven; Superpowers `verification-before-completion` | tests green | pinned acceptance cmd | QA loop + Playwright | QA loop + interrogate + doubt pass |
| D-doc | Documenter (cheap) | pstack `technical-writing`/`unslop`; Addy `documentation-and-adrs` | commit msg | receipt | receipt + screenshots | + ADR + before/after vs target |

The ceremony columns are [inferred] from Addy's high-stakes guidance [https://github.com/addyosmani/agent-skills] and Karpathy's slider [https://www.latent.space/p/s3].

## 3. Proposals for slices 7–9

### 3.1 `skills.catalog.json` (slice 7)
```json
{ "version": 1, "skills": [
  { "id": "pocock.grill-with-docs",
    "source": "github.com/mattpocock/skills",
    "install": "npx skills@latest add mattpocock/skills --skill grill-with-docs",
    "pin": "<commit-sha>", "sha256": "<hash of SKILL.md tree>",
    "phases": ["T"], "roles": ["atena"],
    "project_types": ["greenfield","brownfield"], "stacks": ["*"],
    "criticality_min": "large", "token_tier": "high",
    "adapt": "Cap at 10 questions; write the story to .orq/story.md in English.",
    "never": null },
  { "id": "pstack.create-verification-skill",
    "source": "github.com/cursor/plugins/pstack",
    "install": "claude plugin install pstack@cursor-plugins  # no npx path published; vendor the file",
    "pin": "<commit-sha>", "sha256": "<hash>",
    "phases": ["verify"], "roles": ["tester"],
    "project_types": ["brownfield","greenfield"], "stacks": ["typescript","python"],
    "criticality_min": "small", "token_tier": "medium",
    "adapt": "Generated skill lives in repo; tester may run it, not edit it.",
    "never": null },
  { "id": "pstack.swarm",
    "source": "github.com/cursor/plugins/pstack",
    "install": null, "pin": null, "sha256": null,
    "phases": [], "roles": [], "project_types": [], "stacks": [],
    "criticality_min": null, "token_tier": "high", "adapt": null,
    "never": "Agent spawns agents; bypasses orq admission (H-02)." }
]}
```
The Addy and Pocock npx syntax comes from their READMEs [https://github.com/addyosmani/agent-skills][https://github.com/mattpocock/skills]. Pin and hash: verify the hash at install time, and `orq` refuses a skill whose hash does not match [inferred].

### 3.2 Routing rules (`orq profile` → flow + skills) [inferred]
Rules are evaluated top-down and the first match wins:
1. `criticality=trivial` → flow `direct`: worker + tests; no grill, no adversary; doc = commit message.
2. `type=brownfield ∧ any stack` → prepend scouts with `how`, `blast-radius`; planner gets `architect`.
3. `type=greenfield ∧ criticality≥large` → T with `grill-with-docs` + `domain-modeling`; A with `codebase-design`.
4. `stack=typescript` → add `pstack.typescript-best-practices` to workers. `stack=python` → the catalog's Python best-practices entry.
5. `has_ui` → verify adds Playwright evidence; D-doc adds before/after screenshots.
6. `criticality=production-critical` → adversary from another model family (2 rounds) + `interrogate` + doubt-driven pass + mutation tests on changed modules; Owner approval before merge.
7. `stage=in-production` → raise criticality by one level (floor "small").

Each role loads only the skills matched for its phase; it never loads the whole collection [RECAP].

### 3.3 Independent QA loop (slice 8) [inferred unless cited]
- **Tester** (fresh session, read-only worktree, enforced by OS permissions and the gate, not the prompt): runs the pinned acceptance command, the test pyramid (unit → integration → e2e) [RECAP] and the project verification skill [https://github.com/cursor/plugins/tree/main/pstack]. Output: `qa/report.json` (pass/fail per check, reproduction steps, evidence paths).
- **Repairer** (separate session): gets only the failing report. It may edit product code but not the tests or the acceptance bundle, because they are hash-pinned (slice 1). It uses `systematic-debugging` [https://github.com/obra/superpowers]. After 2 failed loops it runs `attack-the-premise` and escalates to the planner [RECAP].
- **Doubt pass** (production-critical only): a fresh-context reviewer extracts each claim in the receipt and tries to refute it [https://github.com/addyosmani/agent-skills].
- **Anti-rationalization table** embedded in the tester and repairer guides:

| Excuse | Rebuttal |
|---|---|
| "Test is flaky, skip it" | Run it 3×; a flaky test is a FAIL with evidence |
| "Tests green, so it works" | Show real behavior: screenshot, HTTP log or CLI output |
| "This test is wrong, I'll adjust it" | You can't edit tests; file a contract-change request to the Owner |
| "Out of scope" | Cite the story line, or else it's in scope |
| "Will fix in follow-up" | Then the slice is not VERIFIED |

### 3.4 Visual evidence layout (slice 9) [inferred]
```
docs/slices/S-07/
  RECEIPT.md          # ≤1 page, business language: value, what changed, 3 Owner questions, verdict
  evidence/
    01-login.target.png   # mockup or approved reference
    01-login.before.png   # main before merge
    01-login.after.png    # integrated SHA
    index.json            # {step, url, viewport, sha, diff_pct}
```
Captures come from Playwright at fixed viewports. `diff_pct` is computed by code, and RECEIPT.md is written by a cheap model from `index.json` plus the QA report.

## 4. Vetting skills and keeping token cost low

Vetting
- **Static analysis before admission**: grep for `rm -rf`, `git push`, `deploy`, sub-agent spawning, network calls and secret reads. Any hit goes to the `never` list unless an adapt note neutralizes it [RECAP lists `worktree-cleanup` `rm -rf`].
- **Evals**: treat skills as code. Superpowers `writing-skills` tests a skill against pressure scenarios before deployment [https://github.com/obra/superpowers]. Our version is a tiny fixture repo per skill, run before and after the skill is added, with the receipt metric compared [inferred].
- **Pin** to a commit SHA plus a content hash; avoid auto-update (Claude Code leaves auto-update off for third-party marketplaces by default [https://github.com/cursor/plugins/tree/main/pstack]); bump deliberately.
- **Prune**: log the skills loaded per slice. A skill unused in the last N slices, or one that shows no measured gain, is removed. Prune again as models improve [RECAP].

Token cost [inferred]
- Haiku-class models handle scouts' summaries, D-doc, receipts and screenshot captions. The best model goes to the planner and adversary only.
- Pocock grilling runs only in T and only for large or critical work. Its output is distilled into a ≤1-page story, and later roles never see the transcript.
- Each role gets a skill index (name + one line). The full SKILL.md is loaded on demand, following pstack's index-of-principles style [RECAP].
- Each role's context is limited to the story, its plan slice and its STATE.md (Karpathy's context engineering [https://x.com/karpathy/status/1937902205765607626]).
