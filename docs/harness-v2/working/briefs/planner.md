# Brief A3-plan · design worker isolation for orq (read-only; you write ONE file)

You are the general planner. Read-only on the repo at /home/miguel/projects/MiguelAR10-Builder/AI-Edge-Runners/experiments/cthulhu-orchestrator (branch harness-v2). Write your plan only to /tmp/claude-1000/-home-miguel-projects-MiguelAR10-Builder-AI-Edge-Runners-experiments-cthulhu-orchestrator/97993ac3-28de-4b5f-887a-6d3b8a94f645/scratchpad/plan-aislamiento.md. No code changes.
Read README.md, docs/REGLAS-HARNESS.md, harness/internal/slice/slice.go, harness/internal/gate/gate.go, harness/internal/cli/v05.go (lanzarWorker, workerArgs), harness/internal/cli/proyecto.go, harness/internal/herdr/herdr.go, harness/internal/sysinfo/sysinfo.go, harness/internal/orqdir/orqdir.go.

## Problem (demonstrated, not hypothetical)
A worker runs as the same Unix user as the harness, in a linked git worktree. It can write `<main>/.orq/slices/NN.json`, `~/.config/orq/limits.json`, `~/.local/state/orq/HALT` and the shared `.git`. Demo: /tmp/claude-1000/-home-miguel-projects-MiguelAR10-Builder-AI-Edge-Runners-experiments-cthulhu-orchestrator/97993ac3-28de-4b5f-887a-6d3b8a94f645/scratchpad/forja.sh — the worker commits code that fails the exam, rewrites the slice state (cmd "true", recomputed fingerprint, fake receipt), and `orq slice cerrar --aceptado` merges it into main as accepted. The gate is a regex denylist and cannot be the guarantee.

## Direction to evaluate (challenge it; propose better if you find it)
- Each worker gets a local clone (`git clone --local`) instead of a linked worktree; juntar/verificar `git fetch` from the clone, so the worker has no write path to the main repo, its refs or .orq.
- The worker process runs inside bubblewrap (bwrap 0.9.0 is installed and works on this WSL2): filesystem read-only, read-write only on its clone and a private tmp, network off unless the slice asks for it.
- Slice state moves out of the repo to a directory outside every sandbox.
- Workers are counted from orq's own registry, not from "a process named claude in a linked worktree".

## Constraints
- Go stdlib only. Linux/WSL2 only is fine.
- Workers are Claude Code and Codex CLIs started by herdr (`herdr agent start NAME --kind claude|codex --pane ID -- args`, see herdr.go). Work out how a herdr-started agent ends up inside bwrap (wrapper executable on PATH, `herdr pane run`, or another way) and what Claude Code needs writable (its config dir, credentials, session files) to still run. State what you could verify by reading and what must be tested.
- Slices opened by orq 1.0.0 are in flight in another project with the installed binary. The new binary must close them: specify the migration and the compatibility window.
- Two other activities run in parallel and own these files: A1 gate (internal/gate, cli.go gate functions), A2 verification (internal/slice verdict/open flags, proyecto.go, plan.go, metricas.go). Your activities land after both are merged; say which functions you will rewrite.
- Dependencies (node_modules, .venv) are today hard-linked or symlinked into worktrees by prepararDeps; say how they reach a clone inside a sandbox.

## Deliver in /tmp/claude-1000/-home-miguel-projects-MiguelAR10-Builder-AI-Edge-Runners-experiments-cthulhu-orchestrator/97993ac3-28de-4b5f-887a-6d3b8a94f645/scratchpad/plan-aislamiento.md, 160 lines or fewer
1. Decision: the design in 10 lines, and what you rejected with one line of why each.
2. Threat model after the change: what a worker still can do, honestly.
3. Activity table: ID · result and exit test · files · depends on · parallel/serial · size (S/M/L) · suggested model. Activities sized for one worker context each, with disjoint files where parallel.
4. Hypotheses that must be validated before building (each: how to test it in under 10 minutes without touching the real repo or ~/bin/orq).
5. Migration and rollback.
When the file is written, reply in 8 lines or fewer with the path and the three riskiest hypotheses.
