# orq2 · backlog for the next version

Status: **Later**. Nothing here enters scope until the stop rule passes. The stop rule: 10 real stories through D1–D3 with `~/bin/orq2` in Real State, median verified→decided ≤10 min and 0 undecided after 24 h (`plan.md`). Each item says where it came from. The Owner orders the list once the pilot gives a number.

## Owner ideas (2026-11-19)

| ID | Idea | What exists today | Missing |
|---|---|---|---|
| N1 | Interactive panel to see everything: agents, slices, checklist with values, progress, how agents are distributed | `orq2 status`, `orq2 metricas --json`, `slice decidir --html`; data in `.orq/` and the journal | A live view over the same data |
| N2 | Choose models per role with visible criteria (cost, quality, speed), editable in a UI | `orq2 config ver`: agent/model/effort per role and per worker difficulty, in user and project config layers | UI and criteria |
| N3 | Several accounts: order, block, see them (e.g. `claude-alt`) | Per-role `entorno` (`CLAUDE_CONFIG_DIR`, `CODEX_HOME`; paths only, never secrets) | Ordering, blocking, UI |
| N4 | Allow/block a provider or model per role, plus a spend cap per account (e.g. no Codex on Astra on a $20 plan) | Claude only; Codex out until D5 | Allow/block lists, caps |
| N5 | Preflight at start: call each model/account, report remaining quota, drop what cannot finish | `check --caja` (tools), the gate (RAM, disk, workers) | Model and quota probe |
| N6 | Mascot per model in the UI | — | Visual only, last |
| N7 | Terminal choice (herdr / Orca) in the UI | `terminal = herdr \| orca` in config | UI |
| N8 | Skill discovery: when a detected stack signal has no catalog pack (TypeScript-only, FastAPI, Supabase, LangGraph, Go), a scout runs `find-skills`, the Owner approves, D3b scores, and only skills that raise the score stay | Catalog `harness/internal/perfil/catalogo.json` (11 packs); `orq2 skills instalar\|fijar\|verificar`; D3b `skill comparar` | Discovery step and gating |

## From Fable (2026-11-19, `working/advice/fable-scouts-2026-11-19.md`)

| ID | Item | Gate |
|---|---|---|
| F1 | Living project skill (`.claude/skills/<project>/SKILL.md`, ≤60 lines, each line with evidence; ≤3 lines proposed per accepted story). Done by hand in the pilot first, with no harness code | Pilot |
| F2 | orq verifies `path:line` plus the quoted line in scout facts (`orq mem add --fuente scout`). Facts that fail are stored as hypotheses | First candidate after the stop rule |
| F3 | Flag scout facts for re-check when a cited file changes in an accepted slice | After F2 |

## Known limits (from the D1–D4 closes)

| ID | Limit | Reopen when |
|---|---|---|
| K1 | Playwright browsers are bound only with `checks.e2e`, and a browser launch error counts as red | A pilot story hits it |
| K2 | The box mounts all of `~/.local/lib` read-only | A narrower mount is needed |
| K3 | A deleted test is a `!!` warning, not a block | A deletion slips through a decision |
| K4 | "Owner only" approval rests on `ORQ_ROL`: it stops mistakes, not malice | Stronger identity is needed |
| K5 | Scripts and interpreters can hide commands from the gate (frozen box, step 2) | Damage outside a clone |
| K6 | D5: second provider | The Owner names it (never Codex without his explicit word) |
| K7 | `make ci` stalled once in `go test -race` with no child process (2026-11-19 21:00, under low free RAM) | If it reproduces |
