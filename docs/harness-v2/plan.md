# harness-v2 · plan

## Story
As the Owner, I want `orq` to enforce by program what the README promises, and to take the lightest path for small work, so that nothing reaches `main` without a real receipt and a brief fix does not pay the full protocol.

## Acceptance (Owner checks)
1. The forged-state demo ends rejected: a worker that rewrites slice state cannot get failing code merged.
2. The 18 audited gate evasions are blocked (one documented exception caught by verification); the 4 false positives pass.
3. A slice cannot be opened without counted tests unless a reason is given; verification runs on a clean copy of the commit.
4. `orq slice abrir --ruta fix` runs a fix with one worker and no plan; `orq metricas` shows time per lane.
5. `make test` and both scripts in `validacion/` pass in CI.
6. Slices opened with orq 1.0.0 still verify and close with the new binary.

Out of scope: publishing, pushing, installing over `~/bin/orq` (Owner decides when), Orca adapter, hidden test variants.

Scale: large. Evidence behind this plan: audit of 2026-11-12 (gate probe, forged-state demo).

## Activities
| ID · criterion | Result and exit | Agent, model | Files | Depends · mode | Test and verifier | Worktree | State · commit |
|---|---|---|---|---|---|---|---|
| A0 · 4 | Lane router skill | orchestrator session | `orqskills/orq-ruta`, role skills, guides | none | `go test ./...` | no | integrated · 7f42430 |
| A1 · 2 | Gate: tokenizer, harness paths protected, no helpers from worktrees, MCP | worker-gate, Opus (security-critical parsing) | `internal/gate`, hook templates, gate funcs in `cli.go` | none · parallel | `TestEvasiones`, validar-mvp part A; verifier ≠ implementer | removed after merge | verified on integrated revision · b950633 (go test green, probe 27 BLOCK / 8 ALLOW as expected, validar-mvp 16/16, e2e-v05 74/74) |
| A2 · 3,4,6 | One verdict, counting and isolated run by default, watched runner files, lanes in code | worker-verif, Sonnet | `internal/slice`, `proyecto.go`, `plan.go`, `metricas.go`, e2e | none · parallel | unit tests, e2e-v05; verifier ≠ implementer | removed after merge | verified on integrated revision · ecb3859 (go test green, e2e-v05 74/74) |
| A3 · 1,6 | Worker isolation: clones, sandbox, state outside the repo, worker registry, migration | planner Opus → Astra → workers (rows added after analysis) | to be set by the plan | A1, A2 integrated · serial | forged-state e2e, live-slice migration test; independent review (high risk) | yes | analysis |
| A4 · 5 | CI, README and rulebook say only what is enforced | Haiku/Sonnet | `.github/workflows`, `README.md`, `docs/REGLAS-HARNESS.md` | A1–A3 | CI green | no | pending |

## Decisions from analysis (A3)
Planner plan and Astra critique consolidated here; Astra verdict: GO with a cut.
- `.orq` stays in the main checkout and is absent or read-only inside every sandbox; no state relocation in the first release.
- Worker repos are independent clones without shared or hard-linked objects; work returns as a git bundle that orq verifies and fetches. orq never runs git with its cwd in a worker repo.
- The exam, `--sano` and the integration re-run always execute inside the sandbox; no "sandbox off" switch.
- Closing never trusts a forgeable state: a state in CERRANDO or with an `integrado` SHA is re-checked against the trusted contract before publishing; legacy slices are converted with their old processes stopped and re-verified, and a contract without provenance needs ratification.
- Protected paths are enforced on the diff at ingestion, not only by the hook.
- Network: agent reaches only its provider API; exam has no egress. Live launch of a real sandboxed agent needs a dedicated credential from the Owner (not created by agents); until then it is tested with simulated workers.
- Later: state relocation, persistent worker registry, automatic worker channel spool, second provider, overlays.

## Proposals from research (not in scope until the Owner approves)
Sources consolidated from three scouts on 2026-11-13 (A2A spec v1.0, Claude Code agent teams docs, Google/MIT scaling study arXiv 2512.08296, MAST arXiv 2503.13657, Anthropic multi-agent write-up, Cognition, METR, Hermes Agent docs, pnpm/uv/git docs).
| # | Proposal | Value | Effort |
|---|---|---|---|
| P1 | Channel: sender identity derived by orq from the sandbox, not `--de`; pull inbox with the pane keystroke as a doorbell only; `cierre`/`estado` carry structured data the harness reads; auto idempotency key; messages to a closed attempt rejected. Borrow the A2A task/message/artifact model; do not speak A2A on the wire. | Removes impersonation and "may have arrived"; results become machine-checkable | M |
| P2 | Measure tokens, wall time, attempts and outcome per lane and role in `orq metricas` | The only way to know whether Planner, Astra and parallel workers pay | S–M |
| P3 | One worker by default; parallel only with declared disjoint zones and a reason | Evidence: multi-agent degrades sequential work, helps only decomposable work | S |
| P4 | Astra and scouts only for large or risky slices | Fewer handoffs on medium work | S |
| P5 | Dependency policy per ecosystem: read-only bind by default; when a task changes dependencies, private copy plus offline frozen install from a trusted store, re-installed by orq for the exam | No per-worker dependency copies; no egress | M |
| P6 | Per-role skill allowlist installed read-only in the sandbox (worker: TDD + debugging; verifier: verification; Atena: interview), without skills that own worktrees, merging or PRs | Less context, no second scheduler fighting orq | S–M |
| P7 | Decisions file per slice read by every worker and verifier | Less context loss between sessions | S |
| P8 | Held-out exam variants the worker never sees | Evidence of agents gaming visible tests | M |
| P9 | Quality profile per language in three levels (rapido, normal, estricto): no new suppressions, secrets on the diff, typecheck, new-dependency check, fail-first proof for added tests, mutation testing on changed files as a report; results in the receipt, a missing tool is never a pass | Evidence that tests detect bugs when nobody reads the code | M–L |
| P10 | `orq setup`: interactive terminal page in the AI EDGE RUNNERS look to choose driver, models per role, quality level, worker cap and RAM budget | Configuration without editing JSON | M |
P9 and P10 approved by the Owner on 2026-11-13 (activities Q1 and U1 of the builder). Practitioner sources: mattpocock/skills, unclebob/swarm-forge and clj-mutate, Lauren Tan's pstack, Trail of Bits on mutation testing.
Rejected: A2A over HTTP, agent cards, shared or hard-linked clones, reflinks (ext4 has none), partial clones, self-editing skills, agent swarms.

## Checkpoint
- Phase: D. A1, A2 and B1+B2 integrated and verified on the integrated revision (tip 11426f3: go test green; `orq sandbox selftest` 11/11; `validacion/aislamiento.sh --todo` 13 pass, 1 pending = the forged-state close, owned by B5).
- Proposals P1–P8 approved by the Owner on 2026-11-13 and assigned to the builder.
- Builder: one Fable session, pane `w3X:pE`, worktree `~/worktrees/cthulhu-build`, branch `v2/build`. Order: technical plan, C1 config, C2 terminal driver (herdr + Orca), C3 machine awareness, B5 isolation, B6 launch in the box, C6 channel, C4 metrics, C5 lean defaults, A4 CI and docs. It records progress in the orchestrator's scratch progress file and must `git merge harness-v2` before B5.
- Integration branch: `harness-v2` (local only). Integration owner: orchestrator pane `w3X:p1`. Installed `~/bin/orq` 1.0.0 untouched; rollback = `main` 1f2d0b0.
- Review owed: independent review of B5/B6 (high risk) before acceptance.
- Pending Owner decision: dedicated credential for launching a real agent inside the sandbox; until then real launch is refused and tests use a simulated agent.
- Spend: not measured.
- Next action: review builder commits as they land; merge `v2/build` at activity boundaries.
