# D3b scout: does `claude plugin eval` cover skill evals?
Sources: `claude plugin eval --help` (CC 2.1.328, local), https://code.claude.com/docs/en/plugin-evals.md, skills.md, commands.md. No eval was run, nothing installed. Early-access gate ("plugin eval is currently in early access/unavailable" troubleshooting entries): availability on orq's account is UNVERIFIED.

**Verdict: PARTIAL. It covers the K-runs / throwaway-sandbox / grader-model / numeric-score core of D3b. orq still builds the loop around it (store, choose, edit-and-keep).**

## What it gives
- Case format: `evals/<case>/prompt.md` (frontmatter: runs=3, max_turns=10, timeout_seconds=300, model, allowed_tools, tags, env EVAL_*; body = prompt) + `graders/*.md`, or one `case.yaml` (schema_version "1.1"; context.scaffold_script/history_file/add_dirs). `claude plugin eval init [--bare]` authors them.
- Graders (6, no custom code): `regex`, `tool_used` (e.g. Skill fired), `tool_order`, `file_exists` (free); `llm` (rubric, 2-of-3 judge votes, default judge haiku, `--judge-model`), `baseline` (judge vs reference transcript). Per-grader `weight`.
- Score: run = weighted fraction of graders passed; case = mean over runs; suite = mean over cases. YES, numeric 0..1 per case.
- With/without arm (no-plugin baseline) gives `Δ` per case. `tool_used: Skill` is an indicator, not scored in the two-arm mode.
- Output: `results/<ts>/aggregate-result.json` (`--json [path]`, schemaVersion 1): `aggregates.overallScore/casesPassed/casesTotal/meanDelta`, `cases[].name`, `cases[].aggregates.score/delta`, `cases[].arms.with[].error/aborted`, `costUsd`, `durationSeconds`, `claudeVersion`, `partial`. Plus self-contained `report.html`. Exit 1 if any case < `--threshold` (default 1.0); exit 2 on cost ceiling.
- Sandbox: each run is a `claude -p` child with temp HOME, empty cwd, only the target plugin loaded (no user/project settings, CLAUDE.md, MCP, other skills). Only read-only tools unless `--allow-tools`; Bash needs the OS sandbox (bubblewrap+socat on Linux, WSL2 ok) or the run is refused. Eval dir hidden from the agent. Fixtures only via `--scaffold` (author's bash, runs as you). Not a security boundary.
- Cost: real model calls. Roughly cases x runs x 2 arms agent runs + 3 judge calls per llm/baseline grader per run. Doc example: 1 case = 6 runs ≈ $0.41. `--max-cost-usd`, `-j 1..8`, `--ablation none` halves it, `--runs`, `--model` pins. `costUsd` is a list-price estimate.
- Not a model-free check: cannot run offline.

## Standalone skill (not a plugin)?
Target must be a plugin: path with plugin.json / .claude-plugin/plugin.json, an installed plugin, or a skills-dir plugin (`name@skills-dir`, `claude plugin init` scaffolds ~/.claude/skills/<name>/). A bare SKILL.md without a manifest: UNVERIFIED, likely needs a minimal plugin wrapper (one manifest per skill or per skill set). The without-arm removes the whole plugin, so a multi-skill plugin gives a plugin-level Δ, not per-skill; wrap one skill per plugin for per-skill scores.
Alternative: skill-creator plugin (`evals/evals.json` inside the skill dir, in-session loop, incompatible format). Not scouted further.

## /skill-doctor
NOT an eval. It reports per-skill context token cost and invocation counts and flags never-used skills (needs CC >= 2.1.252 and feature-flag fetching). Useful only as a "drop unused skills" input for D3b, no scoring.

## What orq must still build
1. Per-skill eval suites (cases + rubrics) and one plugin wrapper per skill, in the form the CLI accepts; per task type, cases that several candidate skills share (cases are tied to the plugin directory, so the same prompt set must be copied per candidate: UNVERIFIED how to point one suite at another plugin, `plugins:` field in prompt.md may allow it).
2. Run driver: call `claude plugin eval <skill-plugin> --json <path> --ablation none --max-cost-usd N --trust-plugin`, parse `cases[].aggregates.score`, handle `partial` and exit codes.
3. Score store: persist (skill hash, task type, score, runs, cost, claudeVersion, date) in orq's registry; the tool only writes timestamped result dirs.
4. Selector: pick the best-scoring skill per task type/stack and cap the set. The tool does not compare skills against each other, only plugin vs none.
5. Improvement loop: orq or a worker edits the skill, reruns, keeps the edit only if the score rises. Noise: 3 runs x llm judge is noisy; needs a margin or more runs, and a model/judge pin.
6. Fixtures: repo-shaped tasks need `--scaffold` (author-supplied bash) and granted Write/Edit/Bash with the sandbox; read-only prompt/reply tasks are cheap.
