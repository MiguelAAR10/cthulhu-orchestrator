# Astra · attack the isolation plan for orq

You review, you do not build. Your only output file: /tmp/claude-1000/-home-miguel-projects-MiguelAR10-Builder-AI-Edge-Runners-experiments-cthulhu-orchestrator/97993ac3-28de-4b5f-887a-6d3b8a94f645/scratchpad/astra/critica.md (60 lines or fewer). Read-only everywhere else.
Read: /tmp/claude-1000/-home-miguel-projects-MiguelAR10-Builder-AI-Edge-Runners-experiments-cthulhu-orchestrator/97993ac3-28de-4b5f-887a-6d3b8a94f645/scratchpad/plan-aislamiento.md (the plan), /home/miguel/projects/MiguelAR10-Builder/AI-Edge-Runners/experiments/cthulhu-orchestrator/docs/harness-v2/plan.md (story and acceptance), /tmp/claude-1000/-home-miguel-projects-MiguelAR10-Builder-AI-Edge-Runners-experiments-cthulhu-orchestrator/97993ac3-28de-4b5f-887a-6d3b8a94f645/scratchpad/forja.sh (the demonstrated attack), and the code it names under /home/miguel/projects/MiguelAR10-Builder/AI-Edge-Runners/experiments/cthulhu-orchestrator/harness/internal.

Goal: more value, sooner. No nitpicks, no extra engineering.
1. **Smallest cut that makes acceptance criterion 1 true** (a worker that rewrites slice state cannot get failing code merged) and keeps criterion 6 (slices opened by orq 1.0.0 still close). Name which activities B0-B7 are in a first release, which move to "later", and what the first release honestly still leaves open.
2. **What would break the plan**, with evidence (file:line or a command you ran in a temp dir): wrong assumptions about git, bwrap, herdr, Claude Code or Codex. Do not touch ~/bin/orq, the real repo, real orq state, or any herdr pane.
3. **Owner decisions hidden in the plan** (credentials such as long-lived tokens or API keys, WSL interop, network policy): list each with the cheapest safe default so work can proceed without waiting.
4. Wrong sizing, false dependencies, or parallel activities that collide on files.
End with a verdict line: GO as is | GO with this cut | NO-GO and why. Then reply in 6 lines or fewer with the path.
