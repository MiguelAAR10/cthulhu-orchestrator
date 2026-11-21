# Real State brownfield profile for the "agenda de visitas" story

Scout: Sonnet, read-only. Repo: `products/real-estate/AI-Agent-RealState` (RS), `main` at `139fc2f` ("test(S-01): red scenario"), 2026-11-20.
Tags: FACT = read in the repo, HYPOTHESIS = inferred, UNKNOWN = not verifiable read-only.

## Headline

A visits vertical **already exists** (backend + migration + Next proxies + UI), keyed by engagement, not by lead. The "agenda" story is mostly an extension/UX story on top of it, not greenfield. Its production migration is not applied (human gate).

## 1. Stack per folder

- FACT root: Python >=3.12, package `dante-os` (hatchling), `pyproject.toml:1-48`. FastAPI 0.128, pydantic, langgraph, anthropic, google-genai, psycopg, httpx. Manager `uv`; `uv.lock` is tracked (`git ls-files uv.lock`) although `.gitignore` lists `uv.lock` (ignored-but-tracked). Deps live in root `.venv/` (has `pytest`, `ruff`, `mypy`).
- FACT `src/dante_os/`: layers `api/routers`, `application`, `domain`, `infrastructure` (supabase_* adapters), `channel` (Kapso), `broker`, `health_scraper`, `seller`.
- FACT `dashboard/`: Next 16.2.10, React 19.2.4, Tailwind 4, TS 5, vitest 4, eslint 9, pnpm (`pnpm-lock.yaml`, `.npmrc`, `pnpm-workspace.yaml`); `dashboard/node_modules` present. Own `Dockerfile`, `deploy.sh`.
- FACT `supabase/migrations/` (25 SQL files, latest `20260830090000_commercial_operating_system.sql`) and a stale mirror `src/dante_os/db/00x_*.sql`.
- FACT other: `brochure/` (static HTML/PDF), `bin/` (scripts), `deploy/cloud-run`, `docs/`, `phases/`, `prompts/`, `.audit/` (agent evidence, ~30 dirs), `.opencode/`, `.agents/`, `.claude/`.
- FACT no `.github/`, no Makefile: no CI file in the repo.

## 2. Checks and the orq2 gap

- FACT Python (AGENTS.md:48-50,122-124; HANDOFF gates): `pytest`, `ruff check`, `ruff format --check`, `mypy src`. `pyproject.toml [tool.pytest]` `testpaths=tests`, `asyncio_mode=auto`; ruff line-length 100; mypy `strict=true`.
- FACT dashboard (`dashboard/package.json` scripts): `lint`=`eslint`, `test`=`vitest run`, `build`=`next build`; typecheck has no script (HANDOFF cites `tsc`; HYPOTHESIS: `pnpm exec tsc --noEmit`, `tsc` is in `node_modules/.bin`). Vitest covers only `dashboard/lib/*.test.ts` (node, no component/DOM tests, per memoria m7).
- FACT current `.orq/config.json` has only root `checks`: `lint` (ruff check + format --check on `src tests`), `tests` (`pytest -q -p no:cacheprovider`), `types` (`mypy src`), all `PYTHONPATH=src uv run --offline --no-sync ...`. **No `carpetas` key.**
- GAP (FACT): nothing checks `dashboard/`. S-01 scope already lists `dashboard/*` files (T5), so G4 per-folder checks would need a `carpetas.dashboard` entry with: `pnpm test` (vitest), `pnpm lint`, a tsc command (HYPOTHESIS), optionally `pnpm build` (slow, writes `.next`). Commands should run with cwd `dashboard/`.
- FACT S-01's own `sano` cmd excludes two tests: `--ignore=tests/test_s01_controlled_outreach.py --ignore=tests/test_send_test_inbound.py` (`.orq/slices/01.json`). Why `test_send_test_inbound.py` is excluded: UNKNOWN.
- FACT HANDOFF.md:28-29: last full green was 662 hermetic / 750 with disposable PostgreSQL; known preexisting mypy error in `tests/test_inventory_persistence_sql.py:77`, so `mypy src` (not tests) is the right scope. The orq config already uses `mypy src`.
- FACT SQL tests (`test_engagement_cutover_sql.py:28-32`, `test_inventory_persistence_sql.py:19-22`) are skipped unless `DANTE_TEST_DB_URL` points to a disposable DB. Under orq2 they skip silently, so the migration/RPC behavior of visits is NOT verified by the default gate (HANDOFF mentions a docker container `dante-cos01-pg` on 55434; current state UNKNOWN).

## 3. What exists for visits/agenda

All FACT unless marked.

- Domain/service: `src/dante_os/application/commercial_visits.py` (230 lines). Actions PROPOSE/CONFIRM/CANCEL/COMPLETE (`:12-16`); states PROPOSED->CONFIRMED->COMPLETED, CANCELLED from PROPOSED|CONFIRMED (`:112-114`). Fields: `starts_at`, `ends_at`, `timezone`, `idempotency_key`, actors, `calendar_provider="NOT_CONFIGURED"`, `calendar_event_id` (`:36-53`). Validates `ends_at > starts_at`, idempotent replay conflict, tenant/engagement guard. In-memory store included.
- Adapter: `src/dante_os/infrastructure/supabase_commercial_visits.py` (125 lines), wired in `api/dependencies.py:236` (`get_commercial_visit_service`).
- API: `api/routers/engagements.py:340` `POST /{engagement_id}/visits`, `:372` `POST /{engagement_id}/visits/{visit_id}/transition`; event `visit_proposed` at `:232`.
- DB: `supabase/migrations/20260830090000_commercial_operating_system.sql`: table `commercial_visits` (`:69`), tenant trigger (`:113`), RPCs `propose_commercial_visit` (`:138`) / `transition_commercial_visit` (`:193`), RLS enabled (`:256`). VERSIONED ONLY, production not applied (HANDOFF.md:7,47; STATUS.md:823-844).
- Read model: `application/building_overview.py:178,389,481-505` projects `visits_by_engagement` into the overview.
- Dashboard: proxies `dashboard/app/api/engagements/[engagementId]/visits/route.ts` and `.../visits/[visitId]/transition/route.ts`; UI `dashboard/components/building-overview/VisitAgenda.tsx` (184 lines) used by `BuildingOverviewView.tsx`; types in `dashboard/lib/building-overview.ts` (`OverviewVisit`).
- Tests: `tests/test_commercial_operating_system.py` (8 tests: `:211` lifecycle/idempotency/invalid transition, `:251` invalid window, `:274` cross-engagement, `:313` API fail-closed, `:389` end-to-end safe vertical, `:468` migration additive); visit mentions also in `test_engagement_cutover.py`, `test_engagement_cutover_sql.py`, `test_message_quality.py`, `test_s01_controlled_outreach.py`. No dashboard test for `VisitAgenda`.
- Docs: HANDOFF.md:16,37 (`commercial_visits` is internal truth; external calendars are an optional projection, `NOT_CONFIGURED` for now, never authority), STATUS.md:823-844, WORK_LOG.md:2343-2352. `.audit/commercial-operating-system-01/scouts/b-domain-pipeline-calendar.md` is the prior domain/calendar scout (read it before planning).
- Gaps (FACT/HYPOTHESIS): no external calendar sync (FACT, by decision); no availability/slot or conflict detection between visits (HYPOTHESIS: `commercial_visits.py` only validates the window, no overlap check seen); no reminder/notification path (HYPOTHESIS, not seen); agenda keyed by `engagementId`, while the S-01 outcome view is keyed by lead `dedup_key` (memoria m9, `.orq/memoria.jsonl`).
- Note: memoria m9 says `commercial_visits` is "unreferenced in dashboard code"; that is true of the table name only. The dashboard reaches visits through the engagement proxies and `VisitAgenda`.

Adjacent (would be touched): `engagements.py` control/next-action, human-control flag (IA blocked under human control), `conversation_stage`/`enrollment_status` on `health_prospects`, `LaunchBoard` S-01 "visit_requested" outcome (S-01 uses `conversation_stage interested | enrollment visited` as proxy because the visits table is not in prod), `building_overview` projection, `seller` runtime and `channel/kapso_inbound` (inbound replies that ask for a visit).

## 4. Risks for an agent here

- Secrets (names only): real `.env` (root) holds `SUPABASE_SERVICE_ROLE_KEY`, `KAPSO_API_KEY`, `KAPSO_WEBHOOK_SECRET`, `GEMINI_API_KEY`, `PERPLEXITY_API_KEY`, `GOOGLE_MAPS_API_KEY`, `EXA_API_KEY`, plus `.env.example` also lists `ANTHROPIC_API_KEY`, `ADMIN_API_TOKEN`, `DASHBOARD_PASSWORD`, `SESSION_SECRET`, `CLOUD_TASKS_SHARED_SECRET`. `.env`, `.env.*` and `dashboard/.env.local` are gitignored. `bin/secret-scan` exists. Never read or copy `.env` into a worktree prompt.
- External services: Supabase (real project URL in `.env`), Kapso/WhatsApp (real sends), Google Cloud Tasks/Run, Gemini/Vertex, Perplexity, Overpass. Tests mock HTTP, but `bin/send-test-inbound`, `bin/kapso-preflight`, `bin/cloud-runtime-smoke`, `bin/expose-api` can reach real services: do not run them.
- Human gates (FACT, HANDOFF.md:47-52): production migration apply, Google login of the dashboard, real WhatsApp confirmation, template approval; deployed state UNKNOWN (gcloud account lacks access).
- Dirty main checkout (FACT `git status`): `M .gitignore`, untracked `.agents/`, several `.audit/*` dirs. Work in the worktree (`/home/miguel/worktrees/ai-agent-realstate-s01`, branch `slice/01`), not in main.
- Slow/flaky/side effects: `pytest` writes `.pytest_cache`/`.coverage` unless `-p no:cacheprovider` (already in config); `next build` writes `.next` and is slow; SQL tests need a Docker Postgres. Flaky tests: UNKNOWN.
- Generated/noise: `.audit/` PNGs and reports, `docs/graph/data/*.json` (generated by `docs/graph/scripts/build_graph.py`), `dashboard/tsconfig.tsbuildinfo`, `.mypy_cache`, `.ruff_cache`, `.coverage`.
- Data files: `brochure/Edificio-Rosales-brochure.pdf` and photos (large binaries, partly gitignored). No CSV/parquet tracked.
- Language: code in English, docs/status/handoff in Spanish. UI strings Spanish (`/admin/lanzamiento`, `LANZAR`).
- Convention (AGENTS.md, HANDOFF.md): each activity closes with HANDOFF.md/STATUS.md/WORK_LOG.md updates and `.audit/<activity>/` evidence; S-01 T6 already edits HANDOFF/STATUS, so an agenda story editing them will conflict with S-01 if run in parallel.

## 5. Paused story S-01 as seen in `.orq`

- FACT `.orq/slices/01.json`: `fase: IMPLEMENTANDO`, `intento: 1`, title "Outreach controlado 10-20 prospectos salud con preview exacto", branch `slice/01`, worktree `/home/miguel/worktrees/ai-agent-realstate-s01`, red SHA `139fc2f`, opened 2026-11-20T02:44:37Z; card approved 2026-11-20T04:49:47Z (`diario.jsonl` tail). Red exam: 10 failed / 1 passed in `tests/test_s01_controlled_outreach.py`.
- FACT plan `.orq/plan-s01.json` / `planes/01.json`: T1..T6 (back contract, Kapso template, use case + repo, admin API, LaunchBoard front, docs). Task states were all `pendiente` in the plan JSONs I read; I did not see worker progress or commits on `slice/01` (not checked). `.orq/sesiones.json` lists atena (w4E:p1), orquestador (w4E:p2), planificador (w4E:p3), astra (w4E:p5 in log) from 2026-11-20T02:28-02:36Z. Whether those panes are alive: UNKNOWN. `proyecto.json`: modo `mejora`, 282 source files.
- FACT scope includes dashboard: `dashboard/components/LaunchBoard.tsx`, `dashboard/app/admin/lanzamiento/page.tsx`, `dashboard/app/api/launch/route.ts`, `dashboard/app/api/launch-queue/route.ts`, `dashboard/lib/launch-types.ts`, `dashboard/lib/launch-selection.ts`, `dashboard/lib/launch-selection.test.ts`; backend: `domain/models.py`, `api/config.py`, `application/interfaces.py`, `kapso_channel.py`, `outreach_use_case.py`, `supabase_repo.py`, `routers/admin.py`, `dependencies.py`, `supervision.py`, plus `HANDOFF.md`, `STATUS.md`.
- Overlap with an agenda story: `dependencies.py`, `admin.py`/`engagements.py` area, `building_overview` untouched by S-01, HANDOFF/STATUS shared. The S-01 card calls "pidio visita" a proxy because visits are not in prod: an agenda story that goes live would change that definition.
- FACT `vigilados` is `pyproject.toml` only (blob a7c1657), so dependency changes during a story are detected.
- The S-01 `huella`/`sha_rojo` sit on `main`'s HEAD, so any new story branch must base on `139fc2f` or the red commit.

## Planning checklist for the agenda story

1. Decide scope against the existing vertical: extend `commercial_visits` (overlap/availability, list/filter by day, link to lead) vs. new UI only. Read `.audit/commercial-operating-system-01/scouts/b-domain-pipeline-calendar.md` first.
2. Add `carpetas.dashboard` to `.orq/config.json` (vitest, eslint, tsc) before any dashboard change, or UI work is unchecked.
3. Decide whether the visits migration (human-gated) is applied; without it, the agenda cannot be demonstrated against real data, only the in-memory/fake store and the SQL tests.
4. Make the exam hermetic: pytest with `InMemoryCommercialVisitStore` and a TestClient (pattern in `tests/test_commercial_operating_system.py:313`), not `DANTE_TEST_DB_URL`.
5. Serialize against S-01 (shared HANDOFF/STATUS, `dependencies.py`, `slice/01` worktree).
