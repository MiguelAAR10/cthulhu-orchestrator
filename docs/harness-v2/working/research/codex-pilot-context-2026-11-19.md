# Where to run the first 10 orq2 stories | 2026-11-19, Bogotá
**Recommendation (inference).** Start in **WAIRA Web** after one clean, offline browser preflight. Its approved search-acquisition backlog gives the Owner real product work, its checks cover the web path, and its local checkout has zero legacy slices and no observed live workers. If the preflight stalls, Cthulhu is the Go fallback once its installer work clears.

## Facts | legacy inventory at 18:44–18:47 COT
Paths below are relative to `~/projects/MiguelAR10-Builder/AI-Edge-Runners/`. Open means `IMPLEMENTANDO` here. Age is time since opening for open slices and time since the earliest surviving green receipt for verified slices. Ages are rounded to the hour.
| Repo path / stack | Open, oldest | Verified awaiting a decision, oldest | Live work |
|---|---:|---:|---|
| `products/airiflow/odontoflow-backend` / Python, FastAPI, PostgreSQL (`repos.yaml:154`, `pyproject.toml:36–39`) | 4 · 17d 22h (67) | 43 · 22d 17h (03) | None observed across 10 registered Git worktrees |
| `products/airiflow/odontoflow-frontend` / Node, Next.js, TypeScript (`repos.yaml:161`, `package.json:11–13,30`) | 2 · 17d 21h (58) | 32 · 22d 16h (04) | None observed across 3 registered Git worktrees |
| `products/airiflow/airiflow-erp` / Python FastAPI + Node Next.js (`README.md:79`, checkout discovered under Builder) | 0 | 1 · 15d 22h (g1/N1) | Yes, three working Herdr panes on linked ERP worktrees |
- Command evidence. Parsed each `.orq/slices/*.json`: 82 unresolved schema-0 records, 6 implementing and 76 verified. Oldest verification timestamps from `03.json`, `04.json`, `g1.json` and their green receipts were Oct 28 06:50Z, Oct 28 07:18Z and Nov 4 01:46Z. Oldest verified slices by opening time were 02, 04 and g1 (22d19h, 22d19h and 16d18h).
- The three repos have `recibos.jsonl` and lack `diario.jsonl`. Backend and ERP `slices.jsonl` have two prior rejections and one prior acceptance respectively, with zero overlap with current slice IDs. Global `state.json:2–4,9–12` is a Nov 8 disk snapshot.
- Other `.orq` footprints have 0 open / 0 verified and no observed workers in those checkouts. These are `products/airiflow/{airiflow-Brain,airiflow-planning,odontoflow-sim,odontoflow-voice}` (docs, docs, Vite/Node, Python) and `products/odontoflow/odontoflow-planning` (docs). Evidence from Builder `.orq` scan and `repos.yaml:172,179,190,195`.
- `herdr pane list --workspace w49`: `p1V` index, `p1W` wiring and `p1X` reviewer were working. `git rev-parse --git-common-dir` tied all three to ERP. Process CWDs corroborate Claude PIDs 636369, 775472 and 795643. No standalone `orq` process was observed.

## Candidate ranking | inferences from the local evidence
1. **WAIRA Web** (`products/waira/repos/waira-web`, `repos.yaml:258`) · **0 to drain, low local interference.** Clean main at `6dda7e5`, no linked worktrees or local workers observed. Spec 020's next delivery advances search acquisition (`specs/020-seo-geo/spec.md:3–7,43–46`). Next.js has lint, types, Vitest and Playwright wired on paper (`package.json:5–12`, `playwright.config.ts:3–20`). Local reproducibility is the main risk.
2. **Cthulhu** (`experiments/cthulhu-orchestrator`) · **0 to drain, medium overlap.** Go builds and verifies with formatting, vet and tests. Its module is under `harness/`, so root auto-detection needs explicit commands (`harness/go.mod:1–3`, `harness/internal/perfil/checks.go:22–29`, `Makefile:38–51`). The open Owner check for a real multi-slice story is useful, but testing the harness on itself narrows the product evidence. Herdr `w47:p2E` installer lane was blocked (`docs/harness-v2/d4-report.md:142`).
3. **Airiflow ERP** (`products/airiflow/airiflow-erp`) · **1 to drain, high interference now.** Owner finance, agenda, and workflow work have clear value (`TASKS.md:9–25`). The existing browser tooling lacks a discoverable Playwright config, the frontend lacks a lint script, and the monorepo needs explicit root checks. The README records failing full-suite baselines (`README.md:79–91`, `apps/frontend/package.json:5–16`, Builder config scan). The older split repos would separately require 47 backend and 34 frontend decisions.

## WAIRA Web | 10 proposed one-slice stories and their red exams
Items 1–7 are successive slices of the pending programmatic landing pages (`specs/020-seo-geo/tasks.md:14–17`, `spec.md:13–24,43–46`). Items 8–10 are scoped to one existing form each.
1. Publish the first birthday-venue page for Cayma. **Red:** With three matching listings seeded, GET `/arequipa/locales-para-cumpleanos/cayma` returns 200 and only those profiles.
2. Hide pages with too little supply. **Red:** The same route with two matching listings returns 404 while a three-listing control returns 200.
3. Count only matching public listings. **Red:** Two valid venues plus a hidden venue, a caterer and a venue from another district still leave the page below threshold.
4. Give each landing page useful copy. **Red:** Two seeded districts produce distinct visible headings and introductions naming the district and event category.
5. Keep landing URLs canonical. **Red:** A filtered landing has exactly one absolute canonical to its clean URL and a `noindex, follow` directive.
6. Make eligible landing pages discoverable. **Red:** The sitemap includes the canonical URL for three listings exactly once and omits the two-listing fixture.
7. Describe the landing to search engines. **Red:** Its JSON-LD lists the three visible profiles and the correct breadcrumb trail, with no phone field.
8. Associate the claim consent error with its checkbox. **Red:** Submit an otherwise valid fixture without consent and assert that the checkbox exposes the displayed error as its accessible description. [`src/app/negocio/[id]/reclamar/FormularioReclamo.tsx:43–58`]
9. Show the optional request note's 1,000-character limit. **Red:** At 320px, the visible hint is associated with the textarea and typing stops at the stated limit. [`src/app/(sesion)/solicitud/nueva/FormularioSolicitud.tsx:40–52`]
10. Keep the reservation radio in sync with the hours input. **Red:** Focus and edit hours after selecting a package, then assert “Por horas” is checked and the selected mode submits as hours. [`src/app/(sesion)/reservar/[negocioId]/FormularioReservar.tsx:29–49`]

## Blockers and unknowns before story 1
- **Install and drain.** `ls ~/bin/orq2` and `command -v orq2` found no binary. Use the reviewed side-by-side build once the installer is ready. WAIRA needs zero drains. Before switching any legacy checkout, finish or close its open slices and decide its verified slices using 1.0.0 (`docs/harness-v2/plan.md:12,103–106`).
- **Prove offline WAIRA startup.** Node 24.12 is present, but `node_modules` and `docs/mapa.md` are absent. Prepare lockfile dependencies, a matching Chromium build, a route map and a counted JUnit exam. Playwright runs a production build on port 3100. Checks use isolated loopback, while WAIRA CI starts Supabase separately and `next/font/google` loads Poppins. Prove a portable local font and synthetic catalog/DB fixture in the same verification environment before counting a story (`playwright.config.ts:3–20`, `.github/workflows/ci.yml:95–113`, `src/app/layout.tsx:2–12`, orq `d2b-report.md:45,70,228`).
- **Baseline and authority.** Run lint, types, Vitest and the intended browser check in the clean box. Current passes, browser revision match and DB-dependent suite behavior are unverified. The last documented full browser run had an auth mail-rate failure (`specs/020-seo-geo/tasks.md:12`). Confirm WAIRA priority and the approved local branch with the team, then get the Owner's card approval before each worker launch. Production, real payments and outreach have separate approvals (`LANZAMIENTO.md:11–22,64–68`, orq `plan.md:233`).
- **RAM.** `free -h` showed 7.3 GiB available and 2.1 GiB swap used. Two live Next servers used ~4.4 GiB combined RSS (`ps` PIDs 815973 and 172182), including ERP's. orq2 work already paused once for RAM (`plan.md:227`). Start with one worker and one browser, and agree on a quiet run window with the people using those servers.
- **Pilot decision.** After 10 real stories through D1–D3, compare median verified-to-decision time with the 10-minute target and check for zero undecided after 24h (`plan.md:98–105`). This scout checked local files and liveness. The trial build, exams and checks remain unrun.

## Para el Owner
Empezaría en WAIRA Web. Tiene trabajo pendiente de producto y cero slices viejos por cerrar.
Antes de la primera historia, instala orq2 al lado de orq y prueba un recorrido con datos de prueba dentro de la caja, sin internet.
Corre una historia a la vez y coordina el horario con quienes están usando los servidores.
Si WAIRA no arranca en la caja, usa Cthulhu cuando quede libre su instalador. Deja Airiflow para cuando cierren el trabajo activo.
