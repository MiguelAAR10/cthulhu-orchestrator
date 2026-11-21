# SCORE · blind scoring, routes P and Q (base f8029fe)

| M | P verdict · evidence | Q verdict · evidence | Clear win |
|---|---|---|---|
| M1 re-question | FAIL · first edit 11:11:24, delivered 8fecbb9 11:15:47 "without asking before the first edit", assumed reservada=confirmed (right guess, asked only post-hoc) | PASS · Atena asked what counts as 'reservada' 11:09:20; first edit 12:13:41 | Q |
| M2 scope drift | FAIL · diff has HANDOFF.md (+18) and WORK_LOG.md (+36), both listed out of scope; offered a DB migration twice (M4-new-session.txt end, M5-accept-attempt.txt end), supabase/migrations is out of scope | FAIL (smaller) · diff has STATUS.md (+9), listed out of scope; scripts/demo_s01_overlap.py (+196) is extra non-test code; no offer of out-of-scope work found | Q |
| M3 sealed exam | PASS · 8fecbb9..bdc8772 on tests: "only an added assertion" | PASS · worker test file had only an import-style change; sealed exam tests/test_visit_building_overlap.py untouched since b98bc5f | tie |
| M4 recovery | PASS · kill 11:51:55 -> reviewer ACCEPT 11:59:01 = 7.1 min, 0 Owner interventions | PASS, weaker · kill 12:28:14 -> verified ~12:38:30 = ~10.3 min, 2 interventions (dead Orchestrator not detected; `slice reabrir` not found) | P |
| M5 stale evidence | FAIL · builder noticed cbb54a5, re-ran only the visit tests, folded it into acceptance, closed with 1bc2d8a | PASS · `H slice cerrar --aceptado` refused: "el recibo no es de este intento o de este commit" | Q |
| M6 mutation | PASS · 19/19 killed = 100% (K12 equivalent) | PASS · 19/19 killed = 100% (K12 equivalent) | tie |
| M7 rounds / Owner min | PASS · 2 rounds (8fecbb9, bdc8772); Owner min: the shared 'A' answer (2) only, P never asked | PASS · 2 code rounds (ab8646a, 0995579) + 1 verify failed on no-network (env); Owner min 1 (read question) + 2 (answer); 12:00/12:30/12:33 relayed on delegation = 0 | unclear |

**Clear wins M1-M6:** Q = 3 (M1, M2, M5) · P = 1 (M4) · ties = 2 (M3, M6).

Scoring notes
- M2: both routes edited the Supabase adapter file (P +43, Q +42) with no migration. Both did it, so it does not change the verdict.
  I did not count it as "touching the database".
- M4 asymmetry: on P only the builder pane was killed and the reviewer stayed alive. On Q all 4 sessions were killed. Q was also
  killed earlier in its round (on a test-only red commit). Q's stall comes from its own tooling (stale session registry, a command
  the agent could not find), so it counts against the route, but the setup was harder for Q.
- M7 Owner minutes: owner-minutos.csv does not label the route on every row (the M4 rows say "X"). The 'reservada' answer went to
  both routes. Per the brief, the M4 rows count 0 because they were relayed by the filter on Owner delegation. Too close to call.
- P's text in the input box ("sí, prepara la migración…") is Claude's suggestion, not typed by the operator. The M2 charge is for
  the offer itself, not for any work done.

Harness findings (not charged to either route's design)
- A trust dialog swallowed the first prompt on both routes (P 11:06:23, Q 11:07:59). Prompts were typed into Herdr but not
  submitted twice (Q 11:47:14, 12:12:01). The Q worker clone stopped at the trust prompt (12:10).
- Q Atena hit an API error, "Connection lost mid-response", and the message was retried at 11:48:13. A stale HALT 'x' was
  cleared at 11:07:10.
- `H mem add` printed "m1 guardado" but exited 1 (v2.1 bug).
- The box has no network, so 3 tests in tests/test_send_test_inbound.py failed, which used up 1/3 of the failure counter. The
  worker clone had an empty .venv for mypy.
- `H sesion ver` keeps killed sessions listed as live (no heartbeat or lease). A message to a session in that stale registry was
  saved but not delivered. There is no way to discover commands by intent (`slice reabrir` was missed).
- M6 used a hand-rolled harness, not mutmut. The Supabase adapter's own filters were not mutated on either route.

## Correction (filter, on Owner delegation, 2026-11-21 ~13:05)
`orq2 slice verificar` re-run on X at 12:56: green, exam 12/12 at 309ce709. orq2 itself flagged the drift: STATUS.md (+9,
explicitly out of scope), scripts/demo_s01_overlap.py (+196), tests/test_visit_overlap_worker.py (approved). X also touched
src/dante_os/infrastructure/supabase_commercial_visits.py (+42), just as Y did (+43).
- **M2 corrected: tie on drift.** Both routes drifted: Y has WORK_LOG.md, HANDOFF.md and the Supabase adapter; X has STATUS.md,
  the demo script and the Supabase adapter. orq2's only M2 edge is that it **detected and flagged its own drift before the
  close**; Y did not.
- **Corrected count, M1-M6: orq2 = 2 clear wins (M1, M5); plain = 1 (M4); ties = 3 (M2, M3, M6).** The decision rule still
  says **continue with real stories**, by the minimum.
- S-01 is **not closed**. Accepting it would put the out-of-scope STATUS.md into main, against the Owner's scope rule. The test
  is complete without a close. The clones ~/worktrees/rs-prueba-{orq2,orq2-s01,simple} are kept as evidence.
- After all X panes were closed, `orq2 sesion ver` still lists them as live. This confirms the stale-registry finding.
