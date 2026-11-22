# orq2 v2.0 fixes · component plan

Owner-approved 2026-11-20 ("apruebo todo esto de hallazgos"). Scale: **Medium, fast path**. T, C and A are done:
the findings were produced by Astra and Fable and then checked against the code by an independent Opus.
Start at **D (Development)**. Do not re-plan. Do not run Astra, Fable or Codex: the Owner forbade new sessions with them.

## Translation (fixed)

**Story.** As the Owner, I use orq2 v2.0 on a real story (one story at a time, I decide each one) and nothing is
lost silently, writes do not clobber each other, approve/close survive a crash, the exam tests every folder the
story touches, memory holds no unproven "confirmado", and an approved story never sits idle.

**Out of scope (v2.1):** resident dispatcher, autopilot, seq/cause events, project id in agent names, fsync and torn-line
guards in canal/WriteAtomic, capacity reservation, stronger approval identity (H6, H9, H16, H18–H20, H22, H23, H31, H34).
Not done at all: H15, H28, H32.

**Acceptance (Owner tests it through the pilot story "agenda de visitas" in Real State after install):**
each group below merged into `harness-v2` with its red test, an independent Opus review with no High findings, and
`make ci` green on the integrated revision. Then clone proof, `make install2` (Owner told first), story run.

## Sources (read before briefing a worker)
- Verified findings, one row per finding with `path:line`: `docs/harness-v2/research/hallazgos-verificados.md` (H1–H34)
- Owner page: `docs/harness-v2/owner/orq2-hallazgos.html`
- Reports: `docs/harness-v2/research/astra-arquitectura.md` (§4 failure table), `fable-arquitectura.md`, `astra-evaluacion.md`
- Integration review in flight: `docs/harness-v2/reviews/review-v2-integration.md`

## Activity table

Two lanes, at most 2 builders at once. Each lane is one Herdr pane and one worktree. Groups within a lane go in
order; each group is one or a few commits, red test first, then a fresh Opus reviewer pane checks it before merge.

| ID | Group | Findings | Lane | Model | Base | Depends on | Verifier |
|---|---|---|---|---|---|---|---|
| R2 | Integration review fixes (H1 Zonas as paths, M1 no literal card sizes, M2 orq byte-identity + orq2 all violations, M3 SondaUvDe panic, L1, L3) | review-v2-integration.md | (being built now by the filter session, branch `v2/review2-fixes`, worktree `~/worktrees/cthulhu-r2`) | Opus | 309addf | — | fresh Opus review → `make ci` → clone proof → **install2 (tell Owner first)** |
| G0 | Fast tests: `t.Parallel` in `internal/slice` where no env/cwd mutation; `make rapido` (gofmt, vet, `go test` changed pkgs, no -race) | H33 | A | Opus | harness-v2 tip | — | `internal/slice` wall time before/after; no test lost (count equal) |
| G1 | Nothing lost silently: close never deletes the channel (archive per story, keep ids monotonic); `msg send` from a worker clone reaches the project `.orq`; save errors are returned (plan hecha/soltar/debatido, CerrarWorker on unreadable state, memoria.Add on close); one bad memory line does not hide all memory | H2, H17, H27, H7, H13 | A | Opus | after G0 merged | G0 | red tests per item |
| G2 | Writes that do not clobber: lock + version check on `sesiones.json` and plan saves; two members closing at once still get exactly one verification; contract sent exactly once per task+generation; `MarcarEntregado` takes the channel lock | H3, H4, H5, H8 (+ unmentioned MarcarEntregado) | A | Opus | after G1 merged | G1 | concurrency tests (`-race`) |
| G3 | Recoverable approve/close: durable intent + resume; "aceptado" logged only once the commit is on main; a crash at each boundary converges to one result (use existing `ORQ_FALLA_EN`) | H10, H11, H12, H29 | A | Opus | after G2 merged | G2 | crash-injection tests at each boundary |
| G4 | Exam covers every touched folder: per-folder checks derived from scope/diff (Python + `dashboard/` pnpm with its own `node_modules`), re-run on the merge commit; a story touching a folder with no declared check does not open | H14, K8 | B | Opus | harness-v2 tip | — | Python green + dashboard red candidate must not pass |
| G5 | Memory with evidence: merge/adapt `v2/memoria` (VerificarCita, Reemplazar); `confirmado` computed by orq2 from verifiable evidence, never self-declared; decisions stored only after the event; `mem add --reemplaza` honoured by every view | H24, H25, H26 | B | Opus | after G4 merged | G4 | replay m12/m13: m12 is intention or superseded, never confirmado |
| G6 | Nothing approved sits idle: `orq2 pendientes` (read-only: who owes what since when) and `orq2 despachar --una-vez` (launch ready tasks of approved slices, idempotent); panel shows "aprobada, falta lanzar" | H1, H30 | B | Opus | after G5 merged | G5 | on a copy of the S-01 `.orq`: prints "aprobada hace N h, T1 lista, sin worker"; two `despachar` runs → one worker |
| G7 | Self-update (Owner request 2026-11-20): `orq2 update` installs the harness-v2 tip of this repo only if that exact SHA has a gate record (independent review no High + `make ci` exit 0 for that SHA; small gate record file written by the integrator); refuses while any slice is verifying or closing; keeps the current binary as rollback; atomic install to `~/bin/orq2` only; prints old→new revision. `--ver` only reports whether a newer gated version exists; `--volver` restores the rollback. Owner-only from a TTY; never under identity `orq` | Owner request | first free lane (A after G0 merged), else B after G6 | Opus | harness-v2 tip at launch | — (no collision with G1/G4 beyond cli wiring) | red tests: ungated SHA refused, slice verifying refused, non-TTY refused, orq identity has no `update`, `--volver` restores byte-identical, `--ver` read-only |

Rules for every group: orq under identity `orq` stays byte-identical to 1.0.0 (no golden changes); stdlib only;
English commits ending with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`; no rebase, no `--force`,
no `-D`; each builder merges `harness-v2` into its lane before delivery and resolves there; the orchestrator merges
with `--no-ff`. Builders never run `make ci` or install; only the orchestrator does, never two `make ci` at once,
and only with ≥8 GB RAM free. Tests use isolated `ORQ_STATE/ORQ_LIMITS/ORQ_WORKTREES/ORQ_CONFIG` in a `mktemp -d`.
Real State (`~/projects/MiguelAR10-Builder/AI-Edge-Runners/products/real-estate/AI-Agent-RealState`) is never touched
by builders; S-01 is paused as is.

## Checkpoint

Final v2.0 state, 2026-11-20 23:45. Earlier checkpoint history is in Git (`git log -p -- docs/harness-v2/v2-fixes/plan.md`).

```json
{
  "phase": "PAUSED 2026-11-21 by the Owner. v2.0 closed; test day and S-02 done; no v2.1 work started",
  "integration_branch": "harness-v2",
  "integration_owner": "orchestrator pane w47:p3B; filter session w47:p9 relays the Owner and commits Owner docs only on docs/owner-pages",
  "rules": "never commit on harness-v2 while make ci runs (moves HEAD, e2e refuses the candidate); one make ci at a time; builders run targeted tests -p 1",
  "installed": "~/bin/orq2 = 2.0.0-pilot revision=795560fc4590630357254f444e2a7b555844e5fa modified=false (Owner GO 23:40, clean-clone build, make install2). ~/bin/orq untouched (1.0.0)",
  "rollback": "~/.local/share/orq2-rollback/orq2-f9a936b (previous), orq2-299d02d, orq2-24f2438. Identity follows the file name: restore by copying to ~/bin/orq2 (run under its rollback name it acts as orq 1.0.0)",
  "v2_0_contents": {
    "integrated": "R2, G0 (rapido), G1, G2 (locks), G3 (recoverable approve/close, Owner-only --revoca-rechazo), G4 (per-folder checks), G6 (dispatcher), G7 (orq2 update gate record)",
    "parked": "G5 option A (v2/g5-memoria b6a26ab): 2 High in final review, backlog K10; harness-v2 keeps pre-G5 memory",
    "high_findings_by_review": "R2 0, G0 0, G4 0, G7 1, G1 0, G2 1, G6 0, G3 1, G5 7 (parked)"
  },
  "gate": {
    "make_ci_795560f": "PASS exit 0, 7m50s, 0 FAIL, e2e-v05 86/0, aislamiento 14/0/0; log ~/.local/share/orq2-gate-logs/make-ci-795560f.log",
    "gate_record": "git note refs/notes/orq2-gate on 795560f (also 8aa4f8b, f9a936b)",
    "clone_proof_795560f": "PASS 22:58: version modified=false, doctor only 3 dead RS sessions, check --caja 0, over-cap card lists 3 violations"
  },
  "known_limits": "orq2 update and --revoca-rechazo are speed bumps, not locks: a script pty + env -u ORQ_ROL passes soloOwner (K4, K10)",
  "real_state": {
    "real_repo": "untouched; S-01 paused IMPLEMENTANDO (10/11 red). Its .orq/config.json has no carpetas.dashboard, so orq2 795560f refuses S-01 (scope has dashboard/*) until the Owner decides that config",
    "test_clones": "~/worktrees/rs-prueba-orq2 is the agenda product line: main f7d1416 = S-01 (overlap, faa29db) + S-02 weekly calendar (closed accepted 16:14:33, a3 verified at 3dbfb8e). .orq/config.json: test-day worker model overrides removed (baja haiku, media sonnet, alta opus), dashboard checks via node (no pnpm). ~/worktrees/rs-prueba-simple kept as test-day evidence (1bc2d8a). Real RS repo untouched"
  },
  "test_day": {
    "date": "2026-11-21",
    "result": "blind score: orq2 2 clear wins (M1 re-question, M5 stale evidence), plain 1 (M4), 3 ties (M2 corrected, M3, M6) -> continue with real stories",
    "docs": "docs/harness-v2/v2.1/test-day-results.md, test-day-score.md",
    "evidence": "~/cthulhu-evidence/2026-11-21/ (blind pack and sealed map alongside)"
  },
  "remote": "harness-v2 pushed to origin by the orchestrator at the filter's request (2026-11-21); main untouched",
  "worktrees": "~/worktrees/cthulhu-docs (filter), the RS test clones rs-prueba-orq2, rs-prueba-simple",
  "panes": "w47:p9 (filter), w47:p3B (orchestrator); all project sessions closed; resource sampler stopped",
  "branches_not_merged": "v2/g5-memoria (K10), v2/panel (K11), v2/tui (K12), v2/locks (K13), v2/memoria (K14); each has a backlog line",
  "v2_1_notes": "orq2 update installs the tip while docs commits move past the gated SHA; G7 Lows; G2 L1/L3/L5 and relevo; G6 L2/L6; G4 L2/L3 and e2e per folder; G3 L-3/L-4 and --revoca-rechazo has no gate raw-text rule; K10 G5 resume plan",
  "attempts": "G3 review-fix 2 of 2 used; G5 option A final round used (parked)",
  "owner_last_update": "2026-11-21: still PAUSED; Owner definition of the next version docs/harness-v2/v2.1/definicion-siguiente-version.md and D30-D33 in rumbo.md (docs/owner-pages 38d54c4, merged)",
  "next_action": "definicion-siguiente-version.md §6: one Sonnet session, E1 verification skill in the RS test clone, measure tokens and minutes",
  "s02": {
    "stages": "~/cthulhu-evidence/2026-11-21/agenda/etapas.csv",
    "stalls": "agenda/estancamientos.log",
    "ram": "agenda/recursos.csv (190 samples 13:15-16:15; avail min 8.8 GB, max 13.4 GB)",
    "backlog": "T4 docs; Medium 1 atomic hours save; 4 Low and 2 Nits from the S-02 reviews"
  },
  "defects": "docs/harness-v2/v2.1/orq2-defects.md (E01-E27, Owner ledger)"
}
```
