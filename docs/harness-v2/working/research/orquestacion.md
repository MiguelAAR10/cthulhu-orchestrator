# Orchestrating multiple coding agents: what the evidence says (for orq)
Legend: [V] verified from a page I fetched or a search snippet naming the source; [S] secondhand/unverified; [I] my inference.
Honest limit: web search was US-only/standard mode; several items are from search snippets, not full-paper reads. Marked below.

## (a) Hermes Agent (Nous Research)
- [V] Open-source agent harness (not the Hermes model line). Docs: https://hermes-agent.nousresearch.com/docs
- [V] "Closed learning loop": agent-curated memory with periodic nudges, autonomous skill creation, skills that self-improve in use, FTS5 cross-session recall + LLM summarization, Honcho user modeling (same URL).
- [V] Skills = procedural memory, open agentskills.io standard (portable between harnesses), Skills Hub for sharing.
- [V] Spawns "isolated subagents for parallel workstreams"; `execute_code` programmatic tool calling collapses multi-step pipelines into one inference call. Runs on local/Docker/SSH/Daytona/Modal/Vercel Sandbox backends; 20+ chat gateways; cron; MCP.
- [V] Docs page gives NO limits on subagent depth/concurrency, and no planner/verifier/merge model. [S] "~40% faster research" is a single community benchmark, unverified; one critic warns self-mutating skills drift (via search summary of third-party reviews).
- [I] Distinctive = long-lived personal assistant with memory that compounds. It is NOT a delivery harness: no acceptance test, no merge gate, no receipt. Closest relevant idea for orq: skills that are captured from experience (orq has static role skills only).

## (b) Other systems (planning / isolation / verification / merging)
- Anthropic multi-agent research system (lead Opus 4 + Sonnet 4 subagents): https://www.anthropic.com/engineering/multi-agent-research-system
  [V] +90.2% vs single Opus 4 on their internal RESEARCH eval; ~15x tokens of chat; token use explained 80% of BrowseComp variance; failure modes: over-spawning (50 subagents for simple queries), duplicate work from vague delegation, errors compounding. [V] The post itself says coding has fewer parallelizable subtasks and agents coordinate poorly in real time.
- Cognition, "Don't Build Multi-Agents": https://cognition.com/blog/dont-build-multi-agents
  [V] Principles: share full traces; actions carry implicit decisions that conflict across parallel agents. Recommends single-threaded linear agent (+ history-compressing model for long tasks). [V] NO measurements in the article, opinion from experience.
- Agentless (no agent loop: localize -> repair -> validate with tests): https://arxiv.org/abs/2407.01489
  [V] v1 27.33% at $0.34; v2 32.00% at $0.70 on SWE-bench Lite, best open-source at the time; also found SWE-bench Lite flaws (4.3% leaked patches, 10% missing info). Authors' own evaluation.
- Single-loop scaffolds: OpenHands (Docker sandbox, action/observation loop), SWE-agent, mini-SWE-agent (~100 lines). [S] Codesota table (https://codesota.com/agentic/openhands-vs-swe-agent) with Opus 4.5: OpenHands 77.6, mini-SWE-agent 76.8, SWE-agent 72.0 SWE-bench Verified. [I] Scaffold complexity buys little over a lean loop with a strong model.
- Aider architect/editor (planner model + editor model): [V] exists; [S] the one score I saw (31.4%) was a blog, unverified. SWE-Edit paper (https://arxiv.org/html/2604.26102) notes it does not fix context pollution; not tested at repo scale.
- Anthropic "Building effective agents": https://www.anthropic.com/research/building-effective-agents
  [V] Names orchestrator-workers (fits multi-file coding) and evaluator-optimizer (needs clear criteria); advises ground truth from the environment each step. Post notes tooling has changed since Dec 2024.
- [gap] I did not get primary reads for MetaGPT, ChatDev, or Devin internals; not claimed.

## (c) Measured results
- Google/MIT scaling study, 180 configs, 5 architectures, 3 LLM families: https://arxiv.org/abs/2512.08296 , summary https://research.google/blog/towards-a-science-of-scaling-agent-systems-when-and-why-agent-systems-work/
  [V] Parallelizable (Finance-Agent): centralized +80.9%. Sequential (PlanCraft): every multi-agent variant -39% to -70%. Error amplification: independent 17.2x, centralized (orchestrator validates) 4.4x. Coordination overhead grows with tool count (coding agent with 16+ tools cited). [S] "45% single-agent baseline" threshold: not confirmed in the paper text I saw.
- MAST, "Why Do Multi-Agent LLM Systems Fail?": https://arxiv.org/abs/2503.13657
  [V] 14 failure modes in 3 groups: spec/design, inter-agent misalignment, verification/termination. [V] Better role specs and orchestration helped only partly. [S] 86.7% failure for HyperAgent on SWE-bench Lite: figure text garbled, verify before citing.
- Self-correction: https://arxiv.org/abs/2310.01798
  [V] LLMs do not reliably self-correct without external feedback, sometimes degrade; debate/critique methods underperform equal-sample baselines on reasoning tasks (reasoning, not coding).
- Reward hacking: https://metr.org/blog/2025-06-05-recent-reward-hacking
  [V] METR saw o3 hack scoring code in 1-2% of attempts on HCAST/RE-Bench and models modifying tests/scoring. [V] On SWE-bench Verified PRs METR found no test special-casing (agents were not shown tests) https://metr.org/blog/2025-08-12-research-update-towards-reconciling-slowdown-with-time-horizons
  [I] Hidden/protected tests matter when the agent CAN see the test, which is orq's case.
- No primary source found for "multi-agent 72.2% vs 65% single on SWE-bench Verified" (dev.to post); treated as unverified.

## Comparison with orq
Supported by evidence:
- Deterministic gate + untouched test + "0 tests run is not green": matches ground-truth-from-environment, reward-hacking data, and the finding that centralized validation cuts error amplification 17.2x -> 4.4x.
- Few workers, isolated, central orchestrator, no worker-spawned helpers: matches over-spawning and independent-agent findings.
- Test-first red acceptance: Agentless/validation and Anthropic ground-truth advice agree.
Heavier than evidence supports:
- Full Atena + Orchestrator + Planner + Astra + workers + verifier for small work: [I] each handoff loses context (Cognition) and adds coordination cost; evidence on cross-family adversarial critique for coding plans is absent [gap]; self-critique evidence is negative, but a DIFFERENT model with test feedback is untested here. The `fix` lane is the right direction.
- Parallel 1-4 workers on one slice: evidence says parallel pays only for decomposable work; sequential degrades up to -70%. [I] Default to 1 worker; parallelize only on disjoint zones with a stated reason.
- TCADD documentation/context phases on every slice: [I] fine for slices, overhead for `answer`/`fix`.
Missing vs evidence:
- Handoff carries full trace/decisions, not just a summary (Cognition): [I] a decisions file per slice that every worker reads.
- Measured cost/outcome per lane (tokens, retries, pass rate): Anthropic shows tokens explain most variance; plan says spend "not measured".
- Independent verification with a different model/hidden variants (plan lists hidden variants out of scope).
- Learning loop from receipts (Hermes idea): failures become reusable checks/skills.

## What orq should adopt (ranked)
1. Measure spend and outcomes per lane/role (tokens, wall time, attempts, pass/BLOCKED). Why: only way to know whether Astra/Planner/parallelism pay; Anthropic found tokens drive results. Effort S-M. Changes: fills "Spend: not measured"; extends `orq metricas`.
2. Single-worker default; parallel only with declared disjoint zones. Why: sequential/shared-context work degrades with multi-agent. Effort S. Changes: slice planning default (max 4 stays a ceiling).
3. Decisions/trace file handed to every worker and verifier (not a summary). Why: Cognition's context principle, MAST inter-agent misalignment. Effort S. Changes: handoff template and Documentation phase output.
4. Skip Atena->Orchestrator->Planner->Astra chain unless scale is large; Astra only for risky/large slices. Why: no evidence it pays on small work; cost per handoff. Effort S. Changes: lane router rules (A0).
5. Hidden/held-out test variants for verification. Why: reward-hacking evidence when agents see tests. Effort M. Changes: moves item from out-of-scope; verifier runs variants the worker never saw.
6. Lessons loop: on BLOCKED/rework, append a short failure note that planner/skills read next time (Hermes-style, human-approved, not self-mutating). Effort M. Changes: adds to receipts/docs; avoids drift by gating.

## What orq should NOT adopt
- Hermes-style autonomous self-editing skills: behavior drift, and orq's point is invariance.
- Persistent user-modeling/memory (Honcho): not a delivery problem.
- Many-subagent swarms / agent-to-agent chat: over-spawning and 17.2x error amplification evidence.
- Agent self-critique loops without test feedback: no gain without external signal.
- Giant frameworks (MetaGPT/ChatDev style role play): no verified advantage; mini-SWE-agent matches heavier scaffolds within ~1 point (secondhand).
- Chat-gateway / 20-platform reach: unrelated to correctness.
