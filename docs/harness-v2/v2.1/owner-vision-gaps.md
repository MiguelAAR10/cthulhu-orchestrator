# orq2 deficiency map against the Owner's product vision (2026-11-21)

Status: **project paused by the Owner** after S-02. This is the map to come back to, not a plan to execute.
The lean plan (`lean-plan.md`) fixes the harness brakes; this file records what orq2 does not do at all yet.

## The Owner's correction, in his words (summary)
"El criterio no es hacerlo, es hacerlo bien." He narrowed orq2 himself to "dame el componente y lo hago" and says
that was a mistake. Anyone can generate a frontend or a backend with AI; this is a high-level factory: functional
quality, polished aesthetics, and backend, infrastructure, services and architecture built on expert software
engineering principles. The user (or architect) must not be reduced to one user story: there must be dialogue,
participation, and a way for people to say "this far I can go, from here I do it alone, I understood you".

## Deficiencies (D14–D22)

| # | What the Owner wants | What orq2 has today | Gap |
|---|---|---|---|
| D14 | **An identity and a beautiful interface.** On start, a Cthulhu header (ASCII octopus, AI Edge Runners style); highlight what matters; show clearly what is happening. Today he sees "letters and boxes, nothing is happening". | Plain text output; HTML pages made by hand; a status page made by the filter today | No visual identity, no live status in the terminal, no highlighting of key items |
| D15 | **An onboarding conversation.** Ask: which models do you have, any limits (quota, days left), do you want Astra or Fable, how important is this project. Limit and pace the work from those answers. | Model per role/difficulty in config (`orq2 config ver`); no questions; quota unknown to orq2 | Setup never asks; budget and importance never shape the run |
| D16 | **A chosen autonomy level.** "Do I decide everything, or do you keep intervening?" — the human can say "at my own risk, do it all", or "this matters to me, I stay in". | Fixed gates (Owner-only commands as speed bumps); today the filter decided on delegation by hand | No explicit autonomy setting per project or per story; no record of who decided under which level |
| D17 | **A product document before features.** Product idea → feature → explain it → scouts → mockups → design and art → deliverables → then the development cycle (integration, good practices). Ideation and trade-offs, architecture and patterns, integrations and dependencies with other components, because they decide the product's value. | Atena translates a story into a card; Planner writes a red test; no product doc, no trade-off step, no architecture proposal | orq2 starts at the story; ideation, architecture and SDLC stages are missing |
| D18 | **Scouts that use what already exists.** Real State has design work (including a final 3D model version) that the S-02 mockup did not pull: no real design unification. | Atena's scouts read code; nothing indexes design assets, mockups or art | No design/asset inventory; mockups ignore existing art and the latest version |
| D19 | **Reference repositories first.** "Does a repo already solve this (e.g. a conversational agent)? How did they design it? Stars, author?" Explain it so the agent follows a proven pattern. Ask "do you have a repo that inspired you, or should I search?" | D3 recorded (gh search, skills.sh) but not built | No reference-search step with a short, sourced comparison |
| D20 | **Only the right skills, validated.** Detect the stack and suggest the minimum validated skills: TypeScript good practices; Angular skills only if Angular; for frontend, quality skills (e.g. Impeccable, UI skills, taste skills) to avoid slop. | `internal/perfil/catalogo.json` (11 entries; `Detect`, `Recomendadas`) suggests skills by signal | Catalogue is small, not validated per stack, not shown to the human as a choice, no frontend-quality set |
| D21 | **Design artefacts for product work.** Wireframes, process maps and interaction flows, above all for AI products with several flows. | One static mockup when Atena chooses to make one | No wireframe / flow-map stage; no rule for when it is required |
| D22 | **Notify the human.** Tell him when something needs him or something finished, without him watching panes. | Filter relays by hand; status page polls | No notification channel |

## What this changes in how we judge orq2
- Acceptance is not only "exam green": it includes design consistency with existing art (D18), reuse of proven
  patterns (D19), stack-appropriate practices (D20) and engineering principles in architecture (D17).
- The Owner's minutes are not the only metric: participation he chooses (D16) is value, not cost.
- Next step when the project resumes: decide with the Owner which of D14–D22 enter v2.1 next to the lean plan,
  and which wait for a v3 (product stages before the story).
