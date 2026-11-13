# orq · agile design: route by signals, prove by program
Architect, 2026-11-13, on `harness-v2`. Marks: **[F]** fact with source `[n]` (list at the end) · **[L]** my own run today · **[I]** inference, not measured. orq has measured nothing yet (plan: "Spend: not measured"), so every number below is a starting default that lives in `config` and is editable in `orq setup`. CLI words are Spanish, like the code.

In short: two things are fixed for any work that changes files, a check declared before the work that orq re-runs on a clean copy and receipts, and the Owner's accept. Everything else (interview, scouts, plan, Astra, team, verification depth, report size) is a step that a program turns on or off from declared signals, shows with its reason, and lets the Owner change in one command.

## 1. TCADD: an order, not a pipeline
**Decision.** T→C→A→D→D stays as the order steps run in and as vocabulary. No step is mandatory except proof and accept. The full walk is the longest route, taken only when the signals ask for it.

**Router.** `orq ruta decidir` (new package `internal/ruta`: a pure function over a rule table in config).
- Declared by Atena as flags: `--tipo pregunta|sonda|docs|fix|ui|incidente|refactor|deps|feature` · `--criterio "<given/when/then>"` or none · `--causa conocida|desconocida` · `--riesgo datos,dinero,auth,esquema,cuentas,irreversible` · `--archivos N` (estimate) · `--repos N` · `--confianza alta|baja` (does the Owner already know what "right" looks like).
- Derived by the program: `modo` nuevo|mejora and `web` (`perfil.Detect`) · freshness of the map and facts (§2) · reopens and BLOCKED on these paths (closing records) · facts of the diff at ingestion (risk globs, UI globs, manifest touched, files against the estimate) · capacity (C3).
- Rules: `tipo` picks a lane preset; modifiers then only raise steps; a missing signal counts as its heavier value and the output names the one question that would lower it. Output: lane, level of each step, the rule that set it, and `más barato: <lane> si <condition>`. The route travels in the data of the `historia` message (C6) and `orq slice abrir --historia cN` copies it into the slice state; a `historia` without a route is refused.

| Lane (`tipo`) | Questions, max | Scouts | Plan | Astra | Workers | Check orq re-runs | Quality (P9) | Fresh verifier | Report |
|---|---|---|---|---|---|---|---|---|---|
| respuesta | 0 | 0 | no | no | 0 | none, nothing changes | none | no | none |
| sonda (spike, no criterion) | 2: the question, the timebox | 1 | no | no | 1 | none; must leave `hallazgo.md` with evidence; branch `sonda/NN` is never integrated | secrets | no | short |
| docs | 1 | 0 | no | no | 1, cheap model | diff stays inside doc globs; docs command if the repo has one | rapido | no | short |
| fix | 1 | 0 (1 if cause unknown) | red test only | no | 1 | red → green | rapido | no | short |
| ui (polish, no logic change) | 1 | 0 | red Playwright spec only | no | 1 | red → green plus screenshots (§3) | rapido | no, the Owner looks | short, before and after |
| incidente | 1, then act | 1, to reproduce | red reproduction only | no | 1, model one tier up | red → green; orq first offers a revert of the last closed slice on those paths | rapido now; debt: re-verify at normal within 24 h | if a risk flag | short now, full with the debt |
| refactor (no behaviour change) | 2 | 1: which tests cover it | if more than 3 files | no | 1 | existing suite green at base and at head, executed count not lower, test files untouched | normal + mutation report | if more than 10 files | full, structure before and after |
| deps | 2 | 1: changelog, breaking changes | if a major version | no | 1 | existing suite before and after; lockfile consistent; orq fetches outside the box (P5) | normal + audit outside the box as a report | no | short |
| slice (feature) | 5 | up to 3, from gaps (§2) | plan.json + red test | no | 1; zones only with `--paralelo "reason"` | red → green | normal | yes | full |
| slice + riesgo or grande | 8 | up to 3 per repo | plan.json + red test | yes | 1 to 4 by zones | red → green (+ held-out tests when P8 exists) | estricto | yes, other model family | full |

Modifiers (they only raise): any `riesgo` flag → never the fix, ui or docs lane; adds Astra, estricto, verifier · `grande` = more than 15 files, more than one zone or repo → plan, scouts, Astra · `repos>1` → one slice per repo, the contract slice first · `causa desconocida` → one scout that reproduces, worker one model tier up · `confianza baja` → mockup first when UI, otherwise a sonda first · `modo nuevo` → §2 · UI paths in the diff → §3 · two reopens on the same paths → Atena proposes a sonda before another fix (pstack principle 4 [13]).

**Override and reverse.**
- `orq ruta fijar S-NN <paso>=<nivel> | carril=<lane> "motivo"`: one command, any step, up or down; kept in the state, the journal, the receipt and the report.
- Down mid-flight is new (today a route only goes up). It applies to steps not yet started; the plan, tests, commits and receipts already made stay; extra workers are merged with `juntar` and pruned with the existing `Podar`. Going below a floor that a risk flag set needs `--owner` and is printed on the receipt.
- Up: verification depth rises by itself when the ingested diff shows a risk glob, a manifest or a UI path. Ceremony (plan, Astra, team) is never added silently: launches pause and Atena asks one line, as the fix → slice upgrade does today.
- Not switchable by any route or by the Owner: the box, the clean copy, counted green, the secrets gate, the no-new-suppressions gate.

| Step | What running it costs [I] | What skipping it costs | Where the miss shows |
|---|---|---|---|
| Interview | Owner minutes | the wrong thing gets built | the demo, the latest and dearest place |
| Scouts | up to 3 cheap read-only calls | a plan on a false assumption about the code | `alinear` exit 3 or BLOCKED: one lost attempt |
| Plan | one Opus session | the worker splits the work alone and scope grows | `alinear`, zone rejections |
| Astra | one session of another family, 2 rounds | a plan flaw reaches code; no evidence either way for coding plans [I] | BLOCKED or rework; C4 decides if it stays |
| Team | panes, RAM, merges | wall time only; sequential work got 39–70 % worse with more agents [F 1] | nowhere |
| Fresh verifier | one session | code that satisfies the test and not the story | the Owner at the demo, or nobody |
| Quality level | seconds to minutes | see the P9 table | a later defect |
| Report | seconds, at most one cheap call | the Owner cannot explain what was built | nowhere: comprehension debt [F 9] |

## 2. Context: bought once by a program, topped up by scouts only where there is a gap
Cache, all under `.orq/`:
- `mapa.json`, built by a program with no model (`orq mapa`): directories with counts, languages, entry points, run/test/check commands (package.json scripts, Makefile, go.mod), where tests live, churn and co-change from 90 days of `git log`, imports (Go through `go/parser`; TS, JS and Python by regex, marked approximate). Rebuilt at every `slice abrir` and `slice cerrar`; it takes seconds, so it is never stale. `orq contexto` injects only the part for the slice's `alcance` and its neighbours, within 1.5k tokens [I; aider ships a ranked repo map with a 1k-token default, F 7].
- Memory facts (they exist, with `path:line` evidence) gain the commit they were seen at. Stale is decided by program: `git log <sha>..HEAD -- <path>` not empty → the fact is shown as `revisar`, never as truth.
- The decisions file per slice (P7), and the project card (`orq doc`, exists) kept to 40 lines for the Owner and Atena. Agents receive the map and the facts, not prose [F: LLM-written context files lowered success and raised cost by more than 20 %, 6; I that this carries over to the card].

**Brownfield (`modo mejora`).** `orq init` builds the map and has the Owner confirm three commands: run, test, check. Per slice, `orq scouts preguntas S-NN` prints at most 3 questions generated from the gaps: (a) where the change goes and which pattern to copy, (b) what depends on that area and which tests cover it, (c) how it runs and is tested, asked only when the map has no command or the fact is `revisar`. No gap, no scout.
**Greenfield (`modo nuevo`).** There is no code to read, so scouts look outward: (a) the stack and an end-to-end test with the fewest pieces, checked against current docs, (b) the one decision that would be costly to reverse, (c) code the Owner already has that can be reused. orq forces the first slice to be the walking skeleton: one real behaviour end to end plus the runner, the counted report and the check commands, because every later receipt depends on them. The interview gets 3 more questions (value, user, out of scope). The mode flips to mejora when `Detect` counts 5 source files (exists).
Several repos: one map per repo, one slice per repo.

## 3. Verification in proportion
**Decision.** Depth is the lane's column in §1 plus what the diff raises; P9 stays as the mechanism. Three additions:
1. The check gets a kind, stored on the receipt: `rojo` (today), `existente` (refactor, deps), `alcance` (docs), `ninguno` (sonda, which never integrates).
2. The quality level comes from the route, is raised by the diff (a manifest change turns on the new-dependency gate even in rapido; an auth, money or migration glob means estricto) and is lowered only with `orq ruta fijar … --owner`.
3. UI evidence, below.

**Frontend.** `ui` is true when the profile has `web` and the ingested diff touches `ui.rutas` (tsx, jsx, vue, svelte, html, css; configurable). It is read from the diff, so not declaring it does not avoid it.
- Where: inside the box, with no egress. [L] Playwright's Chromium headless shell (build 1243) ran under `bwrap --unshare-all` with a tmpfs `$HOME`: it loaded a page from a server on the box's own loopback and wrote the PNG; an outside URL was unreachable. Why inside: the exam runs worker code, and the plan allows no exam outside the box. It needs `~/.cache/ms-playwright` read-only in `sandbox.ro`, the app server and the browser in the same box, and Chromium's own sandbox off, which the Playwright docs accept for trusted end-to-end tests [F 8]; here bwrap is the boundary. Browsers are installed outside the box by `orq init`, never by a worker. Not run yet: a whole `npx playwright test` in the box [I that it works; V1's exit test settles it].
- Evidence: the screenshots are taken by the protected spec. `slice abrir --evidencia <dir>`; orq empties that directory, runs the spec on the clean copy at HEAD ("después") and at the red commit ("antes", failures expected), copies the PNGs to `.orq/evidencia/S-NN/aK/` and writes each name and sha256 in the receipt.
- Pass: exit 0; at least one test counted and none failed; at least one PNG created in this run that decodes and is not a single flat colour. Waits are state assertions only (already in the worker guide).
- Not a gate: pixel comparison. Rendering varies with OS, hardware and headless mode [F 8]. Before and after are shown to the Owner, who judges the look.
- A web project without Playwright, or UI that is not web: verification fails with "UI sin evidencia"; `--sin-ui "reason"` is stored and listed under "not verified". No egress means the spec stubs outside services, and the report says so.

## 4. The human loop
**Atena guides from the router's output.**
- Questions: only those the router lists as missing signals, one per message, up to the lane's maximum. Past the maximum she states her assumption and goes on. This is judgement in the skill; the enforced part is that no story is sent without a route.
- She pushes back once per topic when: (1) nobody can say who gains what → she offers a sonda or dropping it; (2) the router prints a cheaper lane → she offers the trim ("without X this is a fix"); (3) the work carries a risk flag the Owner did not mention → she names it and what it adds; (4) the request contradicts a critical memory line → she shows the line and asks which wins; (5) it is the third rework on the same paths → stop and probe. The Owner's second yes is final and saved as a decision. Never on look or wording: she mocks it up.

**Comprehension report.** The program writes the facts, a cheap model writes at most three short things, the program checks them.
- When: `orq slice verificar` calls `orq informe S-NN` when it passes; the Owner reads it at the demo, before accepting. `orq slice cerrar --aceptado` refuses if the report is missing or its embedded commit, receipt hash or facts hash do not match; closing re-stamps the integrated SHA without a model. `orq informe proyecto` is rebuilt at every close.
- Sources: `informe.json`, built by program from the receipt (check kind, counts, quality block, screenshot hashes), `git diff --stat` red..head, plan.json, the route and its reasons, the decisions file, journal times and attempts, and the map before and after (new modules, dependencies, import edges). "Not verified" is computed, not written: every `omitido` gate, every stored waiver (`--sin-conteo`, `--en-sitio`, `--acepto-runner`, `--sin-ui`, a lowered level), stubs, the absence of held-out tests.
- Model part (`escriba`, one headless call): "what you can do now" in 3 lines, one line per module, and up to 7 steps of a walk-through, returned as JSON that may only name node ids present in `informe.json`. Anything else is dropped and the report prints "texto descartado: N".
- Rendering: one embedded template (`html/template`, inline CSS and SVG, PNGs as data URIs, no script, no network), the site's monochrome look (tema-terminal.md), light and dark. Diagrams are drawn by the program: a change map (modules as boxes in layers by import depth; new = filled, changed = heavy border, neighbours = thin; 12 boxes at most), the walk-through as a left-to-right strip, and screens before and after. `informe.json` and its sha256 are embedded for audit.
- Sections: what you can do now · change map · walk-through · screens · verified · not verified · route, time, attempts · decisions and debt.
- Proportional: fix, docs, ui and sonda get one screen and no model call; a slice gets the full report and one call; the project report calls a model only when modules changed. Cost [I]: seconds of program time, at most one cheap call, no session.
- Why: [F 9, read through a search summary] in a randomized trial the group that delegated to AI scored about 50 % on a comprehension quiz against 65–67 % by hand, and those who generated and then studied the result scored 86 %.

## 5. Supervision and reliability
**`orq vigia`**: a Go loop with no model, one per project (lock), started by `orq up`, tick 30 s. Every orq command reads its heartbeat file; when stale, a warning is printed and `slice lanzar` refuses until it is back.
It observes: pane state of each registered session (`terminal.Driver.Estado`) · per worker, the last gate call (each tool call already passes the gate, which will stamp a file), last commit and last message · the same failing signature repeating (`FirmaFallo` exists) · wall time, attempts and tokens (C4) against the lane budget · RAM, CPU, disk (C3, `watch`) · the loop ledger below.

| Alone (reversible, journaled) | Escalates and never does |
|---|---|
| ring a worker silent for 10 min, twice per attempt at most | kill or close a session with uncommitted work |
| pause launches (RAM, disk, budget, or its own doubt) and lift the pause when the cause clears | pick which worker to stop under RAM pressure |
| mark a session `desconocido`, resolved with `Estado` before any retry | retry, merge, verify, accept, reject, change the route |
| close panes of closed slices whose copy is clean | extend a budget (the Owner, with a reason) |
| mark an attempt `excedido` at its budget | answer a question |

Escalation: `alerta` to the Orchestrator → no action in 10 min → Atena → no action → `.orq/ALERTA`, first line of `orq status`, pane title. One alert per loop id.

**Every loop closes.** Anything that opens something goes through `diario.Abrir(kind, id, owner, deadline, parent)`: slice, attempt, worker, session, question, pause, doorbell, sonda, debt (incident re-verify, deferred report), route proposal. The deadline comes from the lane budget; nobody types it. There are four endings: done, rejected, cancelled, expired-and-escalated. `orq cabos` lists what is open; `slice cerrar` refuses while children are open; `orq doctor` is clear only when nothing is open except items the Owner owns. Limit, stated plainly: orq guarantees no loop is forgotten or ends silently, not that a human answers.

| Failure mode (evidence) | Control |
|---|---|
| Agent games a test it can see: o3 hacked scoring in 1–2 % of attempts [F 3] | protected exam, protected paths checked on the ingested diff, clean copy; held-out tests later (P8) |
| Green with nothing run; a missing tool read as a pass | 0 tests is not green; `omitido` is never `paso`; both printed under "not verified" |
| Errors amplify between independent agents: 17.2× against 4.4× with central validation [F 1] | only orq merges and verifies; workers cannot spawn; one worker by default |
| Over-spawning and duplicate work [F 4] | route caps: 3 scouts, 4 workers, capacity by RAM (C3) |
| Context lost at handoffs [F 5]; inter-agent misalignment [F 2] | fewer handoffs through lanes; decisions file; structured closing data (P1) |
| "Done" taken on the agent's word; no reliable self-correction without outside feedback [F 10] | only the receipt counts; verifier of another family on risk; report built from sources |
| Never ends, or ends early (verification and termination group of MAST [F 2]) | loop ledger, lane budgets, vigia |
| Invented packages: at least 5.2 % of samples from commercial models [F 11] | new-dependency gate against lockfile and trusted store (Q1, P5); no egress |
| Forged state, impersonation (shown here with forja.sh) | box, contract outside the worker's reach, identity from the box (B5, C6) |
| The harness costs more than it saves [I; one anecdote: 1 h against 30 min, F 14] | time per lane (C4) and the kill rule in §6 |

## 6. Why different
Superpowers [12], Pocock's skills and SwarmForge's role packs [15] keep the process in prompts, which an agent can skip; pstack [13] moves checks into lint and types and leaves merge and acceptance to the session; Claude Code agent teams [16] give mailboxes, a shared task list and hook vetoes, with no acceptance test, no receipt and no resume of a teammate. orq bets the other way round: proof is a program invariant (a check declared first, re-run in a box on a clean copy, counted, receipted, reported from those same facts) and process is a route chosen by rules, so small work pays little and risky work pays in full [I]. That is worth building only if it beats one good agent with good skills, so the deciding measurement (M1, on C4) is: the same 10 stories, 5 fix-size and 5 slice-size, run with orq and with a single bare Claude Code session using Superpowers, plus orq's own record over its next 20 accepted slices. Per lane: escaped defects (accepted work reopened or fixed within 14 days), wall time and Owner wait until acceptance, false "done" claims caught. The bet is wrong for a lane when orq has no fewer escapes and takes more than 1.5× the time; that lane then shrinks to one agent, the check and the receipt. The same rule prunes steps: Astra, the fresh verifier and scouts each stay only if, over 10 comparable slices with and without, they lower BLOCKED, rework or escapes.

## Activities for the builder (by value per effort; they slot after the builder's own row named in "depends on")
| ID | What lands | Exit test | Files | Depends on | Effort |
|---|---|---|---|---|---|
| R1 | `internal/ruta`: signals → lane, step levels, reasons, cheaper-lane line; presets and thresholds in config; `orq ruta decidir\|ver\|fijar` up and down; route in the `historia` data and the slice state. Absorbs C5's `--escala` and `--riesgo` gates | table test: every row and modifier of §1 gives the stated levels and reason; a missing signal gives the heavier value and its question; `fijar` down keeps plan and commits; below a risk floor without `--owner` is refused | `internal/ruta/*`, `cli/ruta.go`, `config`, `slice` (state, `CambiarRuta`) | C1; C6 for the message data (flags until then) | M |
| H3 | Role skills and guides rewritten around the route: question maxima, the five push-back triggers; TCADD described as the longest route | guides tests; no rule the program decides is left in prose (grep test) | `orqskills/*`, `guias/*`, `README.md` | R1 | S |
| R3a | Check kinds `alcance` (docs) and `ninguno` (sonda: `hallazgo.md` required, never integrated, timebox as budget) | slice tests: a docs slice with a code file is rejected; a sonda cannot integrate; the receipt shows the kind | `slice/*`, `cli` | R1 | S |
| V1 | UI evidence: `ui` from the diff, browsers read-only in the box, `--evidencia`, before and after runs, PNG checks, receipt `evidencia[]`, `--sin-ui` | `validacion/ui.sh`: toy web repo, a real `playwright test` in the box without network passes and leaves a PNG; a blank PNG fails; no Playwright fails with the reason | `slice/verif.go`, sandbox spec, `config`, `validacion/` | B5, Q1 | M |
| H1 | `orq informe` v1, program only: `informe.json`, the template, sections verified, not verified, screens, route; embedded hashes; `cerrar` refuses without a matching report; project page (slices, debts) | golden HTML from a fixture; tampered facts or a stale commit make `cerrar` refuse; parser test: no script, no external URL; the Owner approves the template on one real slice | `internal/informe/*` (new), `slice`, `cli/informe.go` | renders without Q1, fuller with it | M |
| X1 | `orq mapa` and stale facts: rebuilt at open and close, injected by `orq contexto` filtered by scope; memory items carry a sha and show `revisar` | unit tests on toy Go, TS and Python repos; injected context within budget; a fact on a changed file is marked | `internal/mapa/*` (new), `memoria`, `cli/v03.go` | none | M |
| G1 | Loop ledger on the journal: `Abrir`/`Cerrar` with owner, deadline, parent; `orq cabos`; `cerrar` refuses with open children; doctor reads it | each opening command leaves an open entry and its closing command closes it; AST test: no pane, worker or question is created outside `diario.Abrir` | `diario`, `app`, `slice`, `cli` | the journal (technical plan, rule 2) | M |
| G2 | `orq vigia`: tick loop, gate heartbeat stamp, silence and repeated-failure detection, doorbell, pause and resume, `desconocido`, `excedido`, the `alerta` ladder; no launch when it is dead | `go test ./internal/vigia` with fake driver and clock covers each row of the §5 table; it never closes a dirty copy; with vigia killed, `lanzar` refuses | `internal/vigia/*` (new), `gate`, `app`, `canal` | C2, C3, C6, G1 | M |
| R2 | Raises from the ingested diff (risk, UI and manifest globs; files over the estimate) and the pause-and-ask for ceremony | slice tests: a migration file in a fix raises to estricto and pauses launches; the journal has the reason | `ruta`, `slice`, `config` | R1, B5 | S |
| R3b | Check kind `existente` (refactor, deps) with base and head counts and untouched tests; incident preset with the revert offer and the re-verify debt | slice tests: fewer tests executed at head fails; an edited test fails; an incident close leaves a debt that `cabos` shows | `slice/*`, `ruta` | R1, G1 | M |
| X2 | `orq scouts preguntas` from gaps; the greenfield first-slice preset; `init` confirming the three commands | fresh map and facts → zero questions; `modo nuevo` without a runner → the route forces the skeleton | `ruta`, `mapa`, `cli`, `guias/scout.md` | X1, R1 | S |
| H2 | Report v2: the validated model call; change map and walk-through drawn as SVG by program; project map | model text naming an unknown id is dropped and counted; layout goldens for 3, 8 and 20 modules; the report renders when the call fails | `informe`, `escriba`, `mapa` | H1, X1 | M–L |
| M1 | The §6 protocol and `orq metricas` per lane with escapes and Owner wait | fixture test; the paired runs recorded in `docs/harness-v2/medicion.md` | `app/metricas.go`, `docs/` | C4 | S |

## Not now
- Held-out tests (P8): wait until metrics or a receipt show a gamed exam.
- Pixel-diff as a gate: noisy across environments [F 8]; before and after go to the Owner's eyes.
- A model-built code map, embeddings or a search index: the program map is free and cannot invent.
- A vigia that reasons with a model, or that kills workers: supervision has to be cheaper and more predictable than what it watches.
- Routes that lower themselves, or a router that learns weights from metrics: first 20 measured slices; lowering stays a human command.
- One slice across several repos: one repo per slice keeps the receipt meaningful.
- A comprehension quiz for the Owner, a dashboard server, reports committed to git: the HTML file is enough; screenshots would bloat history.
- Mutation floors and SAST as hard gates: report first (Q1), floor after data.
- Playwright evidence for React Native or terminal UI: no verified tool; it is listed under "not verified".

## Sources
[1] arxiv.org/abs/2512.08296 · [2] arxiv.org/abs/2503.13657 (14 modes, 3 groups, 1,600+ traces) · [3] metr.org/blog/2025-06-05-recent-reward-hacking · [4] anthropic.com/engineering/multi-agent-research-system · [5] cognition.com/blog/dont-build-multi-agents · [6] arxiv.org/abs/2602.11988 · [7] aider.chat/docs/repomap.html · [8] playwright.dev/docs/docker and playwright.dev/docs/test-snapshots · [9] anthropic.com/research/AI-assistance-coding-skills · [10] arxiv.org/abs/2310.01798 · [11] arxiv.org/abs/2406.10279 · [12] github.com/obra/superpowers · [13] github.com/cursor/plugins/tree/main/pstack (principles: docs/guide/08-principles.md) · [14] theneuron.ai/explainer-articles/pstack-explained-lauren-tans-system-for-trustworthy-ai-agents · [15] github.com/unclebob/swarm-forge · [16] code.claude.com/docs/en/agent-teams. Read first-hand today: 2 (abstract), 7, 8; 9 through a search summary; the rest as recorded in the scouts' files under `research/`. [L] run: scratch script, bwrap 0.9.0, no repo file touched.
