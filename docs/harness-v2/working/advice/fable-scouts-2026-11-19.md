# orq · advice on scouts and the project skill · 2026-11-19

Read: `plan.md` (Route, Approach, checkpoints), my 11-18 advice, the two Codex scouts, the D3b scout, the Close sections of d1–d4, `guias/scout.md`, `memoria/memoria.go`. Nothing was run or changed. "UNVERIFIED" marks what I did not check.

## 1. orq today

**Real (tested, on fixtures and test apps).** Close by sealed receipt; clones with a scope check on the bundle; project checks in a loop and in the receipt (D2a); Playwright exams counted by test name (D2b); a card that blocks launch until approved (D3); one decision page with a queue and timestamps (D1); skill scores over `claude plugin eval` (D3b); child slices, pause, merge on close (D4). Built in one day for about USD 60, each inside the review budget except D2a.

**Unproven (zero real stories so far).** That the Owner decides from the page without reading the diff. That the card cuts exam corrections (12 of 13 second attempts were exam fixes). That D3b scores predict better stories (one case, USD 0.05, judge noise unmeasured). That D4 survives a real 3-child story. That WAIRA even starts in the offline box (`node_modules`, Chromium, Google font, Supabase fixture: all open in the pilot scout).

**Expect from the 10 stories.** Stories 1–3 will mostly surface box and exam friction, not product speed. The trial yields one honest number (verified → decision) and a list of real defects. It cannot prove orq is faster than 1.0.0: there is no baseline. Ten is a signal, not proof.

## 2. The thesis

**Where I agree.** The measured waste was misunderstanding, not dishonesty: exams corrected after opening, and repairs spent on environment facts nobody knew. Both are "the agent did not know the project". Facts gathered before the card is written attack exactly that. Scouting while the Owner is still defining the story costs him no waiting time.

**Where I disagree.** The gain is at the card and the exam, not in execution. A worker with a correct exam and `orq check` already converges. So scouts should feed the Planner, and the project skill should be short. A swarm is the wrong size: orq's own guide says ≤ 3 scouts per story, one question each, ≤ 10 lines.

**Value (estimate, UNVERIFIED until measured).** Fewer exam corrections, fewer infrastructure reds counted as test failures, shorter cards because open questions arrive answered.

**Cost.** Tokens are small (Sonnet/Haiku, read-only). The real costs are Owner attention if he must read scout output, context tokens every worker pays for a loaded skill, and upkeep when the file goes stale.

**Already exists — do not rebuild.**
- Scouts: Claude Code subagents (Explore, read-only) and `orq skill scout` + `guias/scout.md`.
- Fact store with evidence: `orq mem add --fuente scout --tipo hecho --evidencia path:line`; a fact without evidence is refused (`memoria.go:86`).
- Project sheet: `.orq/PROYECTO.md` by the scribe (`orq doc`). Route map: `docs/mapa.md`. Stack detection and skill catalog: `perfil`.
- Skill choice by score: D3b ranking at launch. Unused-skill report: `/skill-doctor`.
- A per-project skill in git: Claude Code loads `<repo>/.claude/skills/<name>/SKILL.md` natively, and worker clones carry it. Its evolution log is `git log` on that file.
- Stack practice: `vercel:nextjs`, `vercel:react-best-practices`, Context7 for Supabase docs. Execution: Superpowers (installed), Pocock-style `tdd`, `grill-with-docs`, `diagnosing-bugs`, `to-tickets`, `domain-modeling` (installed; authorship UNVERIFIED). pstack: not found on this machine, UNVERIFIED.

**Where it fails.**
1. Confident wrong facts. Today orq only checks that evidence is non-empty, not that `path:line` exists. A wrong "fact" in a skill misleads every later worker.
2. Staleness. A map nobody refreshes is worse than no map.
3. Four documents saying the same thing (`CLAUDE.md`, `PROYECTO.md`, `mapa.md`, the skill) and drifting apart.
4. "Experts that write stack skills" duplicates the packs above. Generic Next.js advice written by us will be worse than Vercel's.
5. Best-practice claims from the web cannot be checked by a machine; they become opinion with a link.
6. More skills is not better: each one costs context and can lower scores. Only D3b can say.
7. RAM: 7.3 GiB free and one pause already. Scouts must be subagents inside one session, not panes.

## 3. The logic I would use

One rule: **a scout answers one question that a card or an exam needs, and every claim is checkable.** No standing swarm.

| Step | Who / existing skill | Scouts (read-only, parallel) | Returns |
|---|---|---|---|
| 0. Project onboarding, once | Orchestrator; `perfil`, `orq init` | codebase map; dependencies and integrations; how it runs and is tested offline | facts with `path:line`; draft of the project skill |
| 1. Idea → story | Atena; `grill-with-docs`, `domain-modeling` | product scout: spec, mockups, existing screens (`docs/mapa.md`) | what exists, what is missing, terms |
| 2. Story → card + red exam | Planner; `orq-planner`, `to-tickets` | ≤ 3 per story: where the change goes, what depends on it, how it is tested today | facts cited on the card; answered open questions |
| 3. Greenfield or new library only | Planner; Context7, `vercel:*` | external practice scout | version-pinned doc links, marked hypothesis |
| 4. Build | Worker; Superpowers `test-driven-development`, `systematic-debugging`; `orq check` | none | commits, receipt |
| 5. Verify, decide | orq; decision page; `/code-review` only if the Owner asks | none | accept or reject with reason |
| 6. Learn, after acceptance | Orchestrator | none | ≤ 3 lines proposed for the project skill |

**Verification of claims, deterministic.**
- Code fact: `path:line` must exist at the scouted commit (`git cat-file -e <commit>:<path>` and line count), and the scout quotes the line; orq compares the quote. Fail → the fact is stored as hypothesis.
- Dependency fact: name and version must match the lockfile.
- "How it runs" fact: the command and its exit code, run in the box. This is the only proof that counts for offline claims.
- External fact: URL plus version; always hypothesis until a check, an exam or an eval backs it.
- Facts carry the commit hash. When a cited file changes in an accepted slice, the fact is flagged for re-check (the scope diff already lists touched files).

**The project skill.** One file, `<repo>/.claude/skills/<project>/SKILL.md`, at most about 60 lines: how to run and test offline, the 5–10 project rules a generic skill cannot know, traps already paid for, links to `docs/mapa.md` and the spec. Every line ends with its evidence. It links, it does not copy. `PROYECTO.md` stays the status sheet; `mapa.md` stays the route map.

**How it evolves.** Workers never edit it inside a story (it is outside their scope, so the page would flag it). After an accepted story the orchestrator proposes ≤ 3 lines from what cost a repair or an exam correction. The change is its own small commit with the reason; the Owner sees it on a page. A line that causes no difference in D3b (with vs without, 3 cases taken from real corrections) is deleted. Rules that must never be skipped leave the skill and become a check in the receipt (the Approach already says this).

**Choosing skills per step.** Fixed by role in the table above, then ranked by D3b score where a score exists. No search system: with fewer than 15 candidate skills a table is enough.

## 4. First step on WAIRA

It fits the route because it adds no harness code: "portable knowledge and skill catalog" stay frozen until the stop rule.

1. **Onboarding scouts, once, before story 1.** Three subagents in one session (Sonnet), one question each, taken from the pilot scout's blockers: (a) what the app needs to start offline — fonts, Supabase, port 3100, env; (b) which checks exist and which pass today in the box; (c) the code path for the landing pages of stories 1–7 and for the three forms of 8–10. Facts go in with `orq mem add --fuente scout`. The orchestrator checks every `path:line` by hand with one git command and runs every "how it runs" claim in the box.
2. **Write the WAIRA skill from the surviving facts** (≤ 60 lines), commit it in `waira-web`. The Owner reads it once: target ≤ 5 minutes.
3. **Run the 10 stories as planned.** The Planner cites scout facts on each card. After each accepted story, at most 3 lines are proposed for the skill.

**Measure (all but the last two already come from `orq metricas` and the journal).**
- Verified → decision median, and undecided after 24 h (the stop rule; unchanged).
- Exam corrections after opening: target ≤ 1 in 10.
- Repairs per slice, and how many were infrastructure reds.
- Card approval median: target ≤ 3 min.
- Scout facts: number written, share that failed the citation check, number later found wrong.
- Owner minutes spent reading scout or skill text: target ≤ 5 per story on average.

**Read the result honestly.** There is no control group. If stories 6–10 do not show fewer corrections and repairs than 1–5, or the Owner reads scout output for longer than he saves, keep the three onboarding scouts and drop the rest.

**Cut.** A standing swarm. Our own stack-expert skills (use Vercel's and Context7). A skill search engine. A separate evolution-log format (git is the log). Scouts during build. Automatic edits to the skill by workers. Product scouts on mockups until a story actually has mockups. Any new `orq` command for this before the stop rule passes; the first candidate afterwards is one small thing: orq verifying `path:line` and the quoted line itself.

## 5. Para el Owner

Miguel: tu idea apunta al problema correcto. Lo que nos hizo perder tiempo fue que el agente no conocía el proyecto.
Pero la ganancia está antes de escribir código: en la ficha y en la prueba. Ahí deben ayudar los exploradores.
No hace falta un enjambre. Tres exploradores por proyecto al inicio y hasta tres por historia bastan.
Casi todo ya existe: los exploradores, la memoria con evidencia, el mapa de pantallas y las guías de Vercel y Superpowers.
Lo nuestro sería una sola hoja corta por proyecto, guardada en git, donde cada línea dice de dónde salió.
Esa hoja crece poco: máximo tres líneas por historia aceptada, y tú ves cada cambio.
El riesgo es que un explorador afirme algo falso con seguridad. Por eso cada dato se comprueba contra el archivo real.
En WAIRA: tres exploradores antes de la primera historia, la hoja escrita, y luego las diez historias como estaba previsto.
No construimos nada nuevo en orq hasta ver el número de las diez historias.
Si de la historia seis a la diez no hay menos correcciones, nos quedamos solo con los exploradores del inicio.
