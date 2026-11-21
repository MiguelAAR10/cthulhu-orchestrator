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

```json
{
  "phase": "D",
  "integration_branch": "harness-v2",
  "integration_owner": "orchestrator pane w47:p3B (since 2026-11-20); filter session w47:p9 relays Owner",
  "harness_v2_tip": "97d4a72 — G0, G4, G7, G1, G2, G6 merged",
  "installed": "~/bin/orq2 = see install2 / installed_version; rollback = rollback_binary",
  "in_flight": {
    "R2": "integrated 5f25823. Independent Opus review of 309addf..bdf40db: VERDICT no High, no Medium; Lows for v2.1 notes: (a) aviso_ram_gb 0 now invalidates limits.json under orq2 and blocks all Bash (fails closed, release note) (b) card preamble gives one error per line under orq2 (c) lines in duplicate/unknown sections not checked in same pass (d) literal caps in cli.go:93 usage are orq 1.0.0 text, keep. Reviewer pane closed, review worktree removed. Next: make ci on 5f25823, clone proof, tell filter before install2",
    "clone_proof_5f25823": "PASS (throwaway RS clone + copied .orq/.venv in session scratchpad, isolated ORQ_*): version 2.0.0-pilot rev 5f25823 modified=false; doctor lists only 3 RS sessions that really no longer exist (S-01 paused; correct report, no harness defect); check --caja exit 0 (lint/types/tests run in box; first run failed only because the copied .venv kept shebangs to the live path, rewritten in the copy); over-cap card: orq2 lists all 3 violations in one run, orq name returns only the first (1.0.0)",
    "make_ci_on_97d4a72": "INVALID (not a code failure): e2e refused because HEAD moved during the run (orchestrator committed a checkpoint mid-ci). Rule: never commit on harness-v2 while make ci runs",
    "make_ci_on_69e251c": "INVALID again: filter committed docs dc06403/a28ff1a mid-run (25 pkgs ok, 0 FAIL); asked filter not to commit on harness-v2",
    "make_ci_on_8aa4f8b": "PASS exit 0, 15m51s (G2 + G6 code); gate record written (git note refs/notes/orq2-gate)",
    "make_ci_on_795560f": "PASS exit 0, 7m50s (G2+G6+G3 + hermetic status test); gate record written",
    "install_candidate": "795560f (G2+G6+G3, gate record written); clone proof PASS 22:58 (version modified=false, doctor 3 dead RS sessions only, check --caja 0, 3 card violations); waiting Owner GO via filter",
    "docs_owner_pages": "merged 2485ee1 (D5-D12, debate response, test-day protocol draft 70ae851); filter commits Owner docs only on docs/owner-pages",
    "test_day_prep": "Owner 22:55: (A) no install of 8aa4f8b; one install after G3+G5. (B) OQ1-OQ10 answered (RS clones ~/worktrees/rs-prueba-orq2 and rs-prueba-simple, date 2026-11-21, 60 min/test, mutmut 20 mutants, kill >=70%, Opus builder + Opus reviewer both routes, blind Opus scorer). Sonnet w47 testday writing answers into the protocol + story parts draft (needs Owner approval) on docs/owner-pages. RS scout profile done fcd3800 (key: dashboard/ has no checks; SQL tests skip without DANTE_TEST_DB_URL so agenda exam must be hermetic; S-01 paused IMPLEMENTANDO 10/11 red). Owner 23:00 dashboard checks approved; 23:10 stories APPROVED (P7 confirmadas). Clones created 22:38 at RS 139fc2f: ~/worktrees/rs-prueba-orq2 and rs-prueba-simple (no remote, .venv+node_modules copied, fresh .orq/config.json with carpetas.dashboard; dashboard lint/tsc/vitest exit 0)",
    "make_ci_on_f9a936b": "PASS exit 0, 9m36s (lint, gofmt, vet, race 24 pkgs ok, internal/slice 169s vs 302s before G0; e2e-v05 + aislamiento 14/0/0); log ~/.local/share/orq2-gate-logs/make-ci-f9a936b.log",
    "gate_record_f9a936b": "written: git note refs/notes/orq2-gate on f9a936b (orq2 update --registrar, ORQ_ROL=orquestador)",
    "clone_proof_f9a936b": "PASS: version 2.0.0-pilot rev f9a936b modified=false; doctor only the 3 real dead RS sessions; check --caja exit 0; over-cap card lists all 3 violations",
    "install2_f9a936b": "DONE (Owner GO); next install will include G2, G6, G3 (+G5 if unblocked)",
    "review_policy": "Owner 2026-11-20: one fresh Opus review per group; second review only for G2 and G3 or when the first finds a High. Reviewer panes closed after each review",
    "gate_logs": "~/.local/share/orq2-gate-logs/ (durable, for orq2 update --registrar)",
    "make_ci_on_5f25823": "PASS exit 0, 14m54s: lint, gofmt, vet, go test -race 24 pkgs ok (internal/slice 302s), e2e-v05 and aislamiento 14/0/0 (orchestrator-run; modified=true only from untracked docs copied by ci)",
    "install2": "Owner GO 2026-11-20 (via filter) for 5f25823 code; built from the docs-only commit on top of it so modified=false; result recorded below after install",
    "installed_version": "~/bin/orq2 = 2.0.0-pilot revision=f9a936b16e4bec493af303f21cea9f46d00e2f46 modified=false (installed 2026-11-20 ~21:40 after Owner GO, built -trimpath from a clean clone at f9a936b); ~/bin/orq untouched (1.0.0)",
    "rollback_binary": "~/.local/share/orq2-rollback/orq2-299d02d (previous install) and orq2-24f2438 (older); identity follows the file name, so a copy reports 1.0.0 until restored as ~/bin/orq2. Restore: cp -p <copy> ~/bin/.orq2.new && mv -f ~/bin/.orq2.new ~/bin/orq2",
    "make_ci_on_309addf": "PASS exit 0, e2e + aislamiento green, 19m19s (filter-run, relayed 2026-11-20); internal/slice 319s under contention",
    "309addf_installable": "NO: fresh Opus review found 1 High (H1 Zonas) + 3 Medium, all in R2 scope"
  },
  "lanes": {
    "A": {"group": "G3", "state": "INTEGRATED f7042db (reviews: r1 1 High fixed, r2 0 High + M-1 fixed; last delta diff-checked by orchestrator; adds Owner-only --revoca-rechazo). Lane A now: v2/test-hermetico (Haiku) makes TestStatusMuestraMaquinaYCapacidad independent of live host agents, then make ci G2+G6+G3. v2.1: --revoca-rechazo has no gate raw-text rule (soloOwner bypassable via script pty, same speed bump as G7); G3 L-3/L-4 open", "attempt": 1, "pane": "w47:p3W (agent g3-recuperable, Opus 5.5)", "worktree": "~/worktrees/cthulhu-g3", "branch": "v2/g3-recuperable", "base": "d88b615", "note": "high risk: two reviews"},
    "G2": "integrated d88b615; reviews: 1 High fixed, 2 no High (2 Medium fixed, orchestrator diff check). Pane closed, worktree removed, branch deleted",
    "G6": "integrated 97d4a72; review no High (2 Medium fixed); concurrent despachar goroutine test after G2 (one worker, one contract). Pane closed, worktree removed, branch deleted",
    "lint_fix": {"state": "in progress", "pane": "w47:p3M (agent lint-st1005, Haiku)", "worktree": "~/worktrees/cthulhu-lint", "branch": "v2/lint-st1005", "why": "make ci on f451d5b failed at lint in 8 s: ST1005 slice.go:298 from G1 99954c8 (S-NN error string); fix = lint:ignore directive, text unchanged; log ~/.local/share/orq2-gate-logs/make-ci-f451d5b.log"},
    "G7": "integrated 6a26b85; reviews: ddf2508 High fixed, 6163067 no High, N1 fix bb71cf7 by orchestrator diff check. Pane closed, worktree removed, branch deleted. Gate record must be written with orq2 update --registrar for each gated SHA; ci log kept at a durable path",
    "G0": "integrated 8683242 (orchestrator diff check: only t.Parallel lines + make rapido; 115 tests before/after; 3 runs + -race slice PASS; ~20-35% faster under load). Pane closed, worktree removed, branch deleted. make ci pending: batched with G4/G1 merge",
    "B": {"group": "G4", "state": "depsEn gated to orq2 (19b30cc, 4 pkgs green -p 1); reviewer re-checking delta 68770f7..19b30cc", "owner_note": "after G4 is installed, RS S-01 will be refused until Owner adds a carpetas.dashboard block to RS .orq/config.json (builder proposed: pnpm run lint / pnpm exec tsc --noEmit / pnpm run test, build optional)", "attempt": 1, "pane": "w47:p3D (agent g4-examen, Opus 5.5)", "worktree": "~/worktrees/cthulhu-g4", "branch": "v2/g4-examen", "base": "309addf", "started": "2026-11-20"}
  },
  "G1": "integrated c0add30; reviews 98b34b4 no High (M1, M2 fixed), delta no High (N1, N2, N4 fixed), guide fix by orchestrator diff check. Pane closed, worktree removed, branch deleted",
  "briefs": "session scratchpad brief-g0.md / brief-g4.md (also re-derivable from this plan's rows)",
  "queued": {},
  "owner_held": {"RS carpetas.dashboard config": "proposal held; Owner approves when next RS story (agenda de visitas) starts"},
  "cleanup_2026-11-20": "removed clean merged worktrees + branches: v2/review2-fixes 26a0b91, v2/build 5d440fb, v2/review-fixes cb65b9e, v2/g0-rapido d5adfdd. Rule (Owner): close each pane as soon as its agent is done and its worktree clean/removed; free -m before every launch",
  "parked": ["v2/panel", "v2/locks", "v2/memoria (input to G5)", "v2/tui (branch only)"],
  "attempts": {"G5_review_fix": "2 of 2 used; round 3 authorized by Owner (option A), final", "G2_review_fix": "1 of 2 done", "G0": 1, "G4": "1 + review-fix 1", "G1": "1 + gating round", "G7": "1 + review-fix 1 of 2", "R2_review": 1},
  "owner_last_update": "2026-11-20 evening: approved all findings, asked for speed",
  "next_action": "review G1 then G4 sequentially with ONE fresh Opus reviewer (machine loaded). Poll A and B every ~10 min; on delivery: diff check, fresh Opus reviewer pane, merge --no-ff into harness-v2, then make ci (one at a time, >=8 GB free); wait for R2 delivery and make ci result from filter"
}
```
