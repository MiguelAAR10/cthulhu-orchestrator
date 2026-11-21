# v2.1 Owner decisions and principles (2026-11-20)

Source: Owner messages to the filter, 2026-11-20 evening. These are approved v2.1 requirements, not v2.0 scope.

## D1. Project identity and credential isolation
Evidence (filter, read-only, 2026-11-20): one Linux user runs every agent. `gh` has 3 accounts logged in and one
global active account (the employer one); `gcloud` has 6 configurations and one global active one; global git email
is the employer one, only NEO has an `includeIf`; SSH keys are already split per account.
Risk: an agent in project X pushes, opens a PR or runs gcloud with project Y's identity (hard to reverse).
- Now (outside orq2, needs Owner go per change): per-folder profile: `GH_CONFIG_DIR`, `CLOUDSDK_ACTIVE_CONFIG_NAME`,
  git `includeIf gitdir:` identity, SSH host alias. Stops confusion, not a hostile agent.
- v2.1 (orq2): `orq2 init` records the project id plus its profile (GitHub account, gcloud config, git identity,
  visible secrets, allowed external commands). Every session starts with that profile inside the existing bwrap box
  and sees only that project's credentials. Every journal line carries proyecto + sesion + rol.
- Later, only if measured necessary: one Linux user per project (also the real fix for G5's identity root cause).
- Test: inside project X, `gh api user`, `git config user.email` and `gcloud config get project` return X's values,
  and project Y's credential dirs are not readable.

## D2. Agents are non-deterministic and suggestible
Coding agents answer toward what the conversation steers them to; without an external base they can confirm our
own bias. Design rule: decisions rest on evidence outside the conversation (tests, measurements, code, proven
external references), never on an agent agreeing. Reviews are independent and adversarial; planners get the code
and the facts, not our conclusion.

## D3. Reference first: do not reinvent the wheel
Before designing a component, a prior-art scout looks for proven solutions and the plan states what is reused.
- GitHub: repos that solve the same problem, ranked by evidence of real use (stars as a weak signal; better:
  used in production by a known product, active maintenance, tests, license). Read how they structure it and their
  algorithms; record what transfers to orq2 and what does not, and why.
- Skills: search existing agent skills before writing one (e.g. Vercel's skills ecosystem / `npx skills`
  registry, pstack, Matt Pocock's skills, Anthropic skills). [U: exact CLI and registry to be verified]
- Output: a short "prior art" table in the component plan: candidate, evidence of use, what we take, what we skip.
- Goal: fewer things built from scratch, fewer bugs, faster delivery.

## D4. Standard engineering criteria for every group
Every fix or feature states: problem with evidence, root cause, hypothesis, design (what stays, what is removed and
why), impact on every consumer, tests that prove it, and what it does not solve. Applied first to G5 (option A).

## Owner vision, 2026-11-20 22:40: the fears orq2 must answer
The Owner's main fears are not autonomy. They are:
F1. Endless refactoring cycles and endless verification loops.
F2. The agent builds what was not asked or understands what was not meant.
F3. One badly chosen word (or a hurried answer from someone who did not read the agent's whole reply) flips the
    direction of the whole project, because the agent does not re-ask.
F4. Our code becomes the agent's source of truth; if it is full of mistakes, the agent learns our mistakes
    (Lauren Tan: "the best spec is code" cuts both ways).
F5. Speed lost to over-engineering, rebuilding what already exists, and heavy ceremony on small fixes.

## D5. Skills as suggestions chosen by project and stage
Skills are not all loaded. orq2 detects the stack and the stage and suggests a short list per role; agents use
only those. Partly exists: `harness/internal/perfil` (Detect, catalogo.json by signal and role, fingerprints,
Recomendadas). Extend: signals for TypeScript, LangGraph/LangChain, Google ADK, FastAPI, data modeling; roles
scout and reviewer; a reviewer skill and a scout skill of our own. New skills enter only through D3 + Owner go.

## D6. Scale chosen per task
A short fix does not run the full protocol. orq2 classifies the task (short / medium / large, as in
orquestador-1) with written criteria, and the ceremony follows the stake.

## D7. Scope fences against loops (F1, F2)
Every card has an out-of-scope list; the reviewer compares the diff with the card and flags anything not asked.
Attempts are capped (2 per defect); a round that does not improve the acceptance criterion stops and escalates.
Refactors only when a card asks for them.

## D8. External baseline for truth (F4)
Patterns come from reference projects and validated skills (D3), not only from our code. The plan names the
reference pattern and the reviewer checks the code against it.

## D9. Minimal living documents
One-page product map (PRD-lite), architecture (C4 diagrams), exceptions, guide projects and code patterns.
Updated at each slice close, short, and shown as local HTML in a structured folder. Code is self-documenting;
docs never duplicate it. External research lives beside the project with the same name, outside the codebase.

## D10. Re-question and rails (F3)
A requirements guardian (filter / Atena) checks each Owner message against the product map before it changes
direction, and re-asks when a word would change scope. Any new session reads the product map first. Owner gets a
notification when a direction change is proposed.

## D11. Verification that pushes the code to its limit
When we verify, we do it hard: mutation tests and adversarial tests on the critical paths, not only simple
happy-path tests. One hard verification loop, not many shallow ones.

## D12. Map capabilities before building
Know which capabilities already exist (code, skills, references, people) before deciding what to build.
