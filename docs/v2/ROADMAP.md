# Cthulhu Harness v2 — Lean roadmap

Principles: lean, reversible (feature flags / adapters), value-first, one vertical slice at a time, every reproduced failure becomes a permanent regression test. Ceremony matches the stakes.

## Phase 1 — Make the guarantees true (trust before autonomy)

| # | Slice | Observable result | Acceptance | Value metric |
|---|---|---|---|---|
| 1 | **Inviolable contract** (H-01, H-03) | The acceptance command and test bundle are pinned by hash and can only change through a recorded Owner decision; any change invalidates receipts; verify and integrate use one evaluator; the integrated SHA gets its own receipt | The false-VERIFIED reproduction ends in FAIL; the skipped-test-after-merge case is rejected | False VERIFIED = 0 |
| 2 | **Clean lifecycle** (H-02 + `orq gc`) | One admission function for every launch path (HALT, disk, quota, workers, **max worktrees**); merge closes worktree + session after verified backup of unique ignored files | After 10 e2e slices: 0 orphan worktrees, no launch under HALT | Open worktrees / GB after close |
| 3 | **Agent & runner adapters** (H-06) + `orq config agents` | `orq.agents.json` declares available CLIs/models and per-role choices; same slice runs with Claude Code and Codex on herdr, tmux or a plain process | e2e passes with 2 different agents and 2 runners | Agents supported with green e2e |
| 4 | **Bounded memory + `orq revive`** (H-04, H-05) | Append-only channel queried by slice; locked read-modify-write; plan tasks keep "done" only if their contract hash is unchanged; per-role `STATE.md` ≤ 60 lines | Kill everything, revive, next slice completes without help | Minutes to resume |
| 5 | **Event-driven status** | Workers write `.orq/workers/<id>/state.json` (heartbeat, result, exit code); a code watcher flags dead/stuck/timeout; Atena reads status from files | < 30% of messages go through the orchestrator | Owner minutes per slice |

## Phase 2 — Speed and alignment

| # | Slice | Observable result |
|---|---|---|
| 6 | **Sub-orchestrators per front** | Only when ≥2 independent fronts; depth 1; brief/receipt of one page; main orchestrator validates product value and the integrated result |
| 7 | **Project-type routing + skill gallery** | `orq profile` detects type (greenfield/brownfield/legacy), stage, stack (TS, Python…), criticality ⇒ flow + skills per phase/role from `skills.catalog.json` (with `npx` install commands, pinned hashes, `adapt` notes, `never` list) |
| 8 | **Independent QA loop** | Tester in fresh session cannot edit tests or product; repairer is separate; test pyramid; mutation/adversarial only above a criticality threshold; Playwright evidence |
| 9 | **Visual + business documentation** | `docs/slices/S-NN/` with one-page product receipt, before/after screenshots vs target, written by a cheap model |
| 10 | **English CLI + quickstart** | English commands with Spanish aliases; 5-minute quickstart; public CI |

## Not now

Global mutation testing · a separate Astra session for trivial slices · LLM-based monitors · renaming/branding · Cursor/xAI/DeepSeek specifics beyond what the adapter allows · translating all guides at once.

## How we build it

TCADD on itself: red test from the reproduction → smallest fix → full suite (85 Go tests + 44 e2e checks) stays green → receipt. 1 worker + 1 verifier until slices 1–2 are closed. Work on branch `harness/v2` of `~/projects/cthulhu-harness`; nothing is pushed without the Owner's approval; `~/orquestador-2` stays untouched until v2 replaces it.
