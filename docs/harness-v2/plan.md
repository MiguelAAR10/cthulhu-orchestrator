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
| A1 · 2 | Gate: tokenizer, harness paths protected, no helpers from worktrees, MCP | worker-gate, Opus (security-critical parsing) | `internal/gate`, hook templates, gate funcs in `cli.go` | none · parallel | `TestEvasiones`, validar-mvp part A; verifier ≠ implementer | `~/worktrees/cthulhu-gate` | in progress |
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

## Checkpoint
- Phase: D for A1 (attempt 1 of 2; one earlier start was cut by a safety filter before any change and does not count). A2 verified. A3: analysis closed, spike B0 running, build rows pending spike results.
- Integration branch: `harness-v2` (local only), tip ecb3859. Integration owner: orchestrator pane `w3X:p1`.
- Versions: working `harness-v2`; installed `~/bin/orq` 1.0.0 untouched, in use by live workers in another project; rollback = `main` 1f2d0b0.
- Open panes: worker-gate `w3X:p2` (worktree `~/worktrees/cthulhu-gate`, branch `v2/gate`), worker-spike `w3X:p6` (scratch only). Closed: planner, Astra, worker-verif.
- Spend: not measured.
- Pending Owner decision: dedicated credential for sandboxed agents.
- Next action: integrate A1 when delivered; turn spike results into B rows and launch them.
