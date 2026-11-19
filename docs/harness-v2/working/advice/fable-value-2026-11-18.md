# orq · outside advice · 2026-11-18

Read: `plan.md`, `step2-plan.md`, `informe-uso.md`, package sizes in `harness/`. Nothing was run or changed.

## 1. The value orq must deliver first

orq must cut the Owner's attention per accepted story: he sees how a story was understood before code exists, and decides on a finished result from one page.
Measure it as the median minutes from "verified" to "accepted or rejected" (target ≤ 10) and the count of verified slices with no decision after 24 h (target 0, today 75).

## 2. Keep / cut / freeze

The usage report says where the pain is. Workers were honest (0 false "done" in ~200 slices). The waste was exams corrected after opening (12 of 13 second attempts), results nobody closed (75), and big work leaving the cycle (M24). Tonight's work addressed none of those three.

**Keep (it already earns its cost)**
- Close by receipt (command, exit code, bundle hash), not by what the worker says.
- Clones plus bundle ingestion with the scope check on the diff. This protects what reaches `main` for any provider, with no hook.
- `config` (model per role and per difficulty, user file then project file), `perfil` (stack detection), the terminal driver, `make ci`.
- The gate as it stands today, as a mistake-stopper. It is 1,952 lines plus 1,087 of tests, the second largest package.

**The gate: freeze now.** It guards against a worker that deliberately escapes, which 200 slices never showed. Eight review rounds each found a new way out because reading command text cannot be made complete. No more review rounds, no new attack rows, no more mutation work unless a real incident happens in real use.

**The box: land tasks 1–3, then freeze tasks 4–7.**
- Tasks 1–3 (network, credential, `orq sandbox agente`) are nearly done; finish them green so the work is not lost.
- Tasks 4–7 wait. The box is insurance, not the value the Owner asked for.
- It has one real product use: a non-Claude agent has no hook, so the box is the only containment for it on repos where a mistake is expensive.
- Unfreeze when either happens: the Owner names the second provider for an expensive repo, or a worker damages something outside its clone.
- When it resumes: a budget of 2 review rounds, and the registry allowlist stays off until a task needs an install.

**Cut from the roadmap (not needed for the value)**
- The Owner passphrase (step 3). A plain command in his terminal is enough until a role is seen accepting on its own.
- `orq setup` full-screen page (U1), quality levels `rapido/normal/estricto` (Q1), held-out tests, mutation on worker diffs.
- Channel rework (C6), resident watcher (W1) as separate pieces, Orca adapter, translation pass (L1).
- `instinto`, model-compacted memory.

**Freeze until the stop rule in section 5 says continue**
- Recovery test (acceptance 7), portable knowledge and the skill catalog (step 8), second provider (step 7).

## 3. Smallest sequence of deliverables

One at a time, one Opus builder, one Opus review, at most 2 review rounds each, then escalate to the Owner.

**D1 · Decision page and timestamps** — Opus, about 1 day
- After: the Owner runs one command on a verified slice and gets one page: tests counted, files planned vs touched, exam changes highlighted, a three-step demo. He accepts or rejects with a reason from that page. `orq status` lists verified slices waiting for him, with their age.
- Also records three stamps (worker finished, verified, decided) and the cause of each new attempt (worker or exam). Today these cannot be measured.
- Exit test: on 5 real odontoflow slices he decides from the page alone; `orq metricas` prints the median verified→decision time from orq data.

**D2 · Correct code per project** — Sonnet for the wiring, Opus review, about 1 day
- After: `orq init` writes the project's lint, type and test commands into the project config (stack detection exists in `perfil`). Every receipt carries the three results. "Skipped" never counts as passed.
- Exit test: odontoflow backend and frontend produce 100 % complete receipts; a planted type error is rejected with the tool's message.

**D3 · Understanding card before code** — Opus, about 1 day
- After: before any worker starts, the Owner reads a card in ≤ 3 minutes: the story in three lines, what will not be done, the exam in plain words, zones touched, open questions. The full lane does not open without his approval. An exam change after approval is shown at close.
- Exit test: 10 real stories; approval median ≤ 3 min; exam corrections after opening ≤ 1 in 10 (today about 1 in 4 on backend).

**D4 · Big stories stay inside** — Opus, about 2 days
- After: a large story is split into child slices under one story. He sees one line per child, gets an alert only on drift (out of scope, exam change, worker did not start in 60 s, no progress), and can pause without destroying anything. A worker's close triggers merge and verify by orq itself.
- Exit test: one real large story with ≥ 3 children, every integration has a ledger row, a pause stops new launches, a worker that never starts is flagged within 60 s.

**D5 · Second provider** — Opus, about 1 day for the adapter; only after section 5 says continue
- After: the same story runs with a provider the Owner names. Box tasks 4–7 are built only if that provider must work on an expensive repo.
- Exit test: the same 5 stories pass with both providers; 0 workers shown as running that never ran.

## 4. Multi-model and per-project configuration

- Multi-model is already a config field: `agente` kind plus model per role and per worker difficulty. `config.Kinds` is `claude | codex` and the default for Astra is still Codex, against the Owner rule. Remove Codex from the defaults in D2.
- A new provider is one entry in `agente` (83 lines today): how to start its CLI, how to pass the model, how to confirm it started. No plugin system, no bus.
- What makes providers interchangeable is the contract, not the launcher: same clone, same exam, same receipt, same scope check on the bundle.
- Per-project configuration is one file, `<repo>/.orq/config.json`, merged over the user file (already built). Add one block: the three check commands. Nothing else until a second project asks for it.
- Reusable knowledge starts as one human-reviewed markdown file per project that every brief links to. No catalog, no automatic memory.

## 5. Stop rule

After the first 10 real stories run through D1–D3, read one number: median minutes from verified to the Owner's decision.
- ≤ 10 min and 0 slices undecided after 24 h: continue to D4 and D5.
- 10–20 min: one fix to the decision page, measure 5 more stories, then decide.
- Over 20 min, or the Owner says the page did not replace reading the diff: stop building the harness. Keep orq 1.0.0 plus CI and protected branches, and spend the time on products.

A second guard on building cost: any deliverable that passes 2 review rounds or about USD 15 stops and goes to the Owner. Tonight's gate would have stopped at round 2.

One thing this advice cannot settle: there is no baseline for Owner time today (the report could not measure it). D1 creates it, so the first 5 slices are the baseline, not proof.

## 6. Para el Owner

Miguel: tienes razón, nos desviamos. Esta noche se gastó casi todo en una cerca para que los agentes no hagan trampa, y en 200 tareas reales ninguno hizo trampa. Tu dolor es otro: no ves cómo entendieron la tarea antes de que escriban código, y revisar lo que entregan te toma demasiado (hay 75 tareas terminadas que nadie cerró). Propongo dejar la cerca como está, terminar lo que ya está casi listo de la caja y guardarla, y construir en este orden, una cosa por día: primero una página única para aceptar o rechazar un resultado en menos de diez minutos; segundo, que cada proyecto diga sus comandos de revisión de estilo, de tipos y de pruebas, y que el recibo los muestre siempre; tercero, una ficha corta que apruebas antes de que empiece el trabajo; cuarto, que las tareas grandes se partan y las puedas pausar. Usar otro modelo además de Claude ya es casi solo configuración, y lo sumamos después. La regla para decidir es un solo número tras diez tareas reales: si decides en diez minutos o menos, seguimos; si pasas de veinte, paramos y usamos lo que ya funciona.
