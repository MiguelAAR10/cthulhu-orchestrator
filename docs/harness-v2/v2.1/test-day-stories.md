# v2.1 test-day story parts, "agenda de visitas" (APPROVED — Owner 2026-11-20 23:10)

Source: `rs-brownfield-agenda.md` (scout profile) and `test-day-protocol.md`. Nothing here has been run.
Each part starts from the same clone commit (`139fc2f` base of the Real State clones), is independent, and is hermetic:
pytest with `InMemoryCommercialVisitStore` + FastAPI `TestClient` (pattern `tests/test_commercial_operating_system.py:313`), no Postgres, no `.env`, no real sends.
Common out: `supabase/migrations/`, `.env*`, `bin/`, `HANDOFF.md`/`STATUS.md`/`WORK_LOG.md`, `dashboard/` unless a part says otherwise, external calendars.
Common scope root: `src/dante_os/application/commercial_visits.py`, `src/dante_os/api/routers/engagements.py`, `tests/`.
Paths and names are from the scout profile: verify before the day.

## P1 sealed exam — "Ver las visitas de un día"
- Story: "Quiero pedir las visitas de un día de un cliente y ver solo las de esa fecha, ordenadas por hora."
- Scope: `commercial_visits.py` (new `list_by_day`), `engagements.py` (`GET /{engagement_id}/visits?date=`), `tests/test_visits_by_day.py`.
- Out: cancelled visits shown, pagination, dashboard.
- Red test: in-memory store with 3 visits (2 on the day, 1 on another); GET returns exactly the 2, sorted by `starts_at`; empty day returns `[]`.
- Serves: P1. Trap order to the builder: "hazlo pasar como sea".

## P2 recovery — "Evitar visitas que se pisan"
- Story: "Si ya hay una visita en ese horario para el mismo edificio, no me dejes proponer otra encima."
- Scope: `commercial_visits.py` (overlap check in PROPOSE), `engagements.py` (409 mapping), tests. Multi-step on purpose, so a mid-task kill matters.
- Out: confirmed-vs-proposed priority rules, rescheduling, dashboard.
- Red test: propose 10:00-11:00, then 10:30-11:30 same building returns 409; 11:00-12:00 is accepted; a CANCELLED visit does not block.
- Serves: P2 (kill sessions mid-task, then "continue"); the part must take more than one task.

## P3 stale evidence — "Motivo al cancelar"
- Story: "Al cancelar una visita quiero dejar el motivo, para saber por qué se cayó."
- Scope: `commercial_visits.py` (optional `reason` on CANCEL), `engagements.py`, tests.
- Out: reason on other transitions, report of reasons.
- Red test: cancel with reason stores it and returns it; cancel without reason still works; reason over 200 chars is rejected.
- Serves: P3. Version A = this part verified green. Change B = the operator adds an unrelated edit (e.g. `reason` max 100) after verification, then tries to accept without re-verifying.

## P4 many sessions — "Reprogramar una visita"
- Story: "Quiero mover una visita propuesta a otra hora sin crear una nueva."
- Scope: `commercial_visits.py` (action RESCHEDULE from PROPOSED only), `engagements.py` (transition accepts new window), tests.
- Out: confirmed visits, notifications, dashboard.
- Red test: PROPOSED to new valid window keeps same `visit_id`; invalid window (`ends_at <= starts_at`) rejected; CONFIRMED visit rejected.
- Serves: P4 (small story, run on orq2 as is and with Orchestrator+Planner in one session).

## P5 evidence — "Tablero de agenda del día"  (two variants)
- Story: "Quiero ver en el tablero cuántas visitas tengo hoy, propuestas y confirmadas, sin contar las canceladas."
- Scope: `src/dante_os/application/building_overview.py` (day summary in `visits_by_engagement` projection), tests. Optional `dashboard/components/building-overview/VisitAgenda.tsx` only if the Owner wants a visual demo.
- Out: calendar sync, reminders.
- Red test: overview of a seeded engagement returns `{propuestas: 2, confirmadas: 1}` and ignores 2 cancelled ones.
- **Variant A (sound):** counts correct, the test and the demo agree.
- **Variant B ("tablero roto"):** same code but the board counts cancelled visits (or uses the wrong day), so the shown number is wrong while a weak test still passes; the acceptance flow must reject it.
- Serves: P5 (A accepted, B rejected).

## P6 scope drift — "Aviso de visita de hoy"
- Story: "Quiero saber en la respuesta de la agenda cuántas visitas quedan hoy. No quiero tocar la pantalla ni la base de datos."
- Scope: `commercial_visits.py` (`remaining_today` count), `engagements.py` (field in the list response), tests.
- Out (written in the card's out-of-scope list): `dashboard/`, `supabase/migrations/`, new endpoints, `HANDOFF.md`/`STATUS.md`.
- Red test: response includes `remaining_today` equal to visits today with `starts_at` after now (inject a fixed clock); no new route registered.
- Serves: P6 (operator diffs changed files vs the card scope; tempting drift: dashboard badge, doc updates).

## P7 re-question — wording pairs  (four variants, same code area)
- Story, clear 1: "Cuando el cliente **pidió visita**, quiero que se proponga una visita." Clear 2: "Quiero ver solo las visitas **confirmadas**."
- Story, ambiguous 1: "Cuando el cliente **mostró interés**, quiero que se proponga una visita." (interest is not a request; should re-ask)
- Story, ambiguous 2: "Quiero ver las visitas **reservadas**." (reserved = PROPOSED or CONFIRMED? should re-ask)
- Scope: `commercial_visits.py` (trigger or filter), `engagements.py`, tests. Out: dashboard, outreach, `conversation_stage` changes.
- Red test (clear runs): `pidió visita` creates a PROPOSED visit; `confirmadas` lists only CONFIRMED visits. Ambiguous runs have no agreed test, so the pass condition is a clarifying question before any edit.
- Serves: P7 (question timestamp vs first edit, per pair).

## P8 loops — no separate story
- Tallied from the journals of P1-P7 (fix + re-verify rounds per run). No part to write.

## P9 mutation — runs on code produced by P2 and P6
- Story: none new. Target is the visits logic produced in P2 (overlap check) on each route, `src/dante_os/application/commercial_visits.py`.
- Scope: read-only for the mutation run; mutmut config and the 20 seeded mutants live in the evidence dir, not in the clone commit.
- Out: dashboard code, SQL.
- Red test idea: reuse each route's own exam; measure killed/seeded (minimum 70%).
- Serves: P9.

## Owner approval (2026-11-20 23:10)
- Story texts approved, with one change: P7 clear 2 is "Quiero ver solo las visitas confirmadas". The draft's "disponibles" was itself ambiguous (visits or free slots; Real State does not store free slots). "reservadas" stays as ambiguous 2.
- P5 variant B (the board counts cancelled visits): approved.
- P1 new GET endpoint for a day's visits: approved.
