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

## Technical plan
Owner of this section: the builder (branch `v2/build`). Order C1 → C2 → C3 → B5 → B6 → Q1 → U1 → C6 → W1 → C4 → C5 → A4, one commit per activity, each green.

### Target layout and dependency direction
`cli` (flags, usage, printing) → `app` (use cases) → domain → adapters. Imports only point down; `internal/arch_test.go` parses every import and fails on an upward one.

| Layer | Packages | Holds |
|---|---|---|
| cli | `internal/cli` | Argument parsing and output. No git, no multiplexer, no model table. |
| app | `internal/app` (new) | `Proyecto`, launch of roles and workers, messaging, handoff and contract text, capacity, metrics. Receives its ports in a struct; tests pass fakes. |
| domain | `slice`, `plan`, `canal`, `memoria`, `instinto`, `perfil`, `gate`, `config` (new), `diario` (new), `watch`, `escriba` | State machines and rules. Pure on injected clock, git and sandbox where they decide acceptance. |
| adapters and base | `terminal` (+`herdr`, `orca`, `fake`), `agente`, `sandbox`, `gitx`, `sysinfo`, `orqdir`, `limits`, `state` | Everything that execs a process or reads the OS, and the two file readers the sandbox itself needs. |

What moves:
- `cli/proyecto.go` → `proyecto`, `abrirProyecto`, paths, `excluir` to `app/proyecto.go`; `porRol`, `arranque.completar` to `config`; `agentArgs`, `rolArgs` to `agente`; `lanzar`, the body of `runUp` to `app/roles.go` (idempotent open in `app/sesiones.go`); the body of `runMsg` to `app/mensajes.go` (C6). `runSlice`, `runMem` keep only flags and printing.
- `cli/v05.go` → `datosRelevo`, `Relevo.Markdown`, `contrato` to `app/relevo.go`; `lanzarWorker`, `registrarLlamador`, `workerArgs` to `app/workers.go` and `agente`; `memVigente` to `app/proyecto.go`.
- `cli/v03.go` → `fase`, `escribirFicha`, the checks of `runDoctor` to `app/estado.go`; `runInit`, `runSkills` stay in cli (they only install files and print).
- `cli/sesion.go` → `cerrarSesiones`, `panelDelSlice` to `app/sesiones.go`. `plan.Modelo` is deleted (C1). `internal/herdr` becomes `internal/terminal/herdr` (C2).

### Ports
| Port | Interface | Adapters |
|---|---|---|
| Terminal driver | `terminal.Driver`: `Dentro`, `PanelActual`, `CrearPanel`, `Arrancar`, `Avisar`, `Estado`, `Renombrar`, `Cerrar` | `terminal/herdr` (today), `terminal/orca` (C2, on the `orca` CLI), `terminal/fake` (tests) |
| Agent runner | `agente.Runner`: argv and env for an interactive agent of a kind, model, effort; `Preguntar` for one headless answer | `agente` for claude and codex; a simulated binary in tests |
| Sandbox | `slice.Caja`: `Command(ctx, sandbox.Spec, argv...)` | `internal/sandbox` (bubblewrap); no "off" adapter exists |
| Clock | `func() time.Time` passed in (`now` parameters stay) | `time.Now`; fixed times in tests |
| Git | `gitx.Git`: `Run(ctx, gitDir, args...)` with a timeout and a config that ignores user and system files | `gitx` on the `git` binary; always `--git-dir` of the main repo, never a worker repo as cwd |

### Rules held everywhere
1. Every external call (git, bwrap, herdr, orca, agent, exam) takes a `context` with a timeout; a timeout is reported as "unknown", never as success or as a plain failure.
2. Every state transition is written to the journal (`.orq/diario.jsonl`, append under lock, `diario` package) before the state file changes, with slice, attempt, actor, from, to and an operation id. Repeating a command with the same effect is a no-op that says so.
3. Lock order, always: `abrir` → `slice-NN` → `integracion` → file locks (`canal`, `memoria`, `diario`). No lock is taken while holding a later one; `slice` asserts it in tests.
4. No ignored error on a path that decides acceptance (verify, ingest, merge, integrate, publish, close): a `go/ast` test over those functions fails when the error of a git, sandbox, journal or save call is discarded.
5. `go test -race ./... -count=1`, `gofmt -l`, `go vet` clean on every commit. Standard library only.
6. A launch is two-phase: intent journaled → driver call → result journaled. An intent without a result is "unknown" and is resolved by `Estado` before anything is retried.

### Activities
| ID | What lands | Exit test | Files |
|---|---|---|---|
| C1 | `config`: defaults = today; per role and per worker difficulty an optional `entorno` (account variables such as `CLAUDE_CONFIG_DIR`, `CODEX_HOME`; paths only, a secret-looking name is an error) passed to the pane and to the headless escriba call; user file (`ORQ_CONFIG` or `~/.config/orq/config.json`) then `<root>/.orq/config.json`, merged per field, origin kept; unknown field is an error; `orq config ver`; `porRol` and `plan.Modelo` deleted; guides print models from config | `go test ./internal/config`; `TestDefaultsIgualesAHoy` pins the old tables; `grep` test: no model name outside `config` | `internal/config/*`, `cli/config.go`, `cli/proyecto.go`, `cli/v05.go`, `cli/skill.go`, `guias/*.md`, `plan/plan.go`, `escriba` |
| C2 | `terminal.Driver`, herdr behind it, Orca adapter, fake; `app` package with role and worker launch; timed-out start = unknown, checked with `Estado` before retry | `go test ./internal/terminal/... ./internal/app`: fake driver proves no second launch after a timeout; Orca adapter tested against a stub `orca` binary; arch test | `internal/terminal/**`, `internal/app/*`, `internal/agente/*`, `cli/*` |
| C3 | `sysinfo.Memoria/CPUs/Carga`; `app.Capacidad` = min(max_workers, RAM, CPU) with per-agent budget and reserve from config; gate and `slice lanzar` deny with the limiting factor; `orq status` shows RAM, CPU, disk, capacity | unit tests with fake `/proc` files; gate test denies on low RAM and says why | `sysinfo`, `config`, `gate`, `app/capacidad.go`, `cli/cli.go` |
| B5 | Independent clones (`--no-hardlinks`), bundle in, `refs/orq/sNN/*`; merge by `merge-tree` + `commit-tree`; exam, `--sano`, integration re-run inside the sandbox on `git archive`; protected paths checked on the ingested diff; contract kept in `refs/orq/contrato/NN`, compared on verify and close; CERRANDO and `integrado` re-checked; `orq slice convertir` for 1.0.0 slices | `validacion/aislamiento.sh` green without `--todo`, part 2 asserting the refusal reason; `e2e-v05.sh`; slice tests | `slice/*`, `gitx`, `gate` (worker detection, contract read), `cli`, `validacion/*` |
| B6 | PATH shim → `orq caja agente`; private agent config dir seeded by orq; credential only from `caja.credencial.<agente>`, else refusal naming the key; deps policy P5; skills allowlist P6 | `go test ./internal/app` with a simulated agent binary inside the real box; refusal test | `app/caja.go`, `cli/sandbox.go`, `config`, `agente` |
| Q1 | Quality profile in verification: levels `rapido`/`normal`/`estricto` per lane (`calidad` in config, command per language and gate); gates run in the box on the clean copy without network: no new suppressions or weakened strictness config, secrets on the diff (gitleaks or built-in patterns), typecheck/vet, new dependency in lockfile and trusted store, fail-first proof of added tests, mutation report. Receipt gains a `calidad` block; a missing tool is `omitido`; a failed hard gate is not VERIFICADO | `go test ./internal/calidad ./internal/slice`: each gate pass/fail/omitted on toy repos; a hard failure blocks like an exam failure | `internal/calidad/*` (new), `slice`, `config` |
| U1 | `orq setup`: full-screen page (termios via `syscall`, always restored) editing user or project config: terminal, agent/model/effort per role, worker model per difficulty, quality level per lane, max workers, RAM budget; shows sandbox self-test and free RAM read-only; line mode when not a tty or under 80x24; `--no-interactivo` prints the effective JSON; writes atomically through `config` validation and prints the diff | `go test ./internal/tui ./internal/cli`: 80x24 render within width for Unicode and ASCII art, key handling on a fake reader, line mode over a pipe | `internal/tui/*` (new), `cli/setup.go`, `config` (write path) |
| C6 | Identity from the box spool, `--de` ignored inside; inbox per role as source of truth, one-line doorbell; `datos` on `cierre`/`estado`; derived key; closed attempt rejects every type; task/message/artifact on disk | `go test ./internal/canal ./internal/app`: spoofed sender ignored, doorbell carries no body, stale attempt rejected | `canal/*`, `app/mensajes.go`, `cli` |
| W1 | `orq vigia`: resident watcher. Observes the driver's panes and the journal; detects stalled or dead workers and launches left unknown; pauses new launches when budget or RAM limits are passed; nudges through the channel doorbell; escalates to the Orchestrator and the Owner. It never accepts, merges or launches | `go test ./internal/app`: fake driver and journal fixtures for each detection; a test proves it calls no accept, merge or launch path | `app/vigia.go`, `cli/vigia.go`, `diario` |
| C4 | Journal-based metrics per slice, lane, role: wall time, attempts, verifications, sessions, tokens (best effort) | `go test ./internal/app` on a journal fixture; `orq metricas` shows the table | `diario`, `app/metricas.go`, `cli/metricas.go` |
| C5 | One worker by default; a second zone needs `--paralelo "reason"`; Astra and scouts gated by `--escala grande` or `--riesgo`; `.orq/decisiones/NN.md` in every contract; skills and guides rewritten | tests for the three gates; guides tests | `slice`, `app`, `guias/*`, `orqskills/*`, `skills/*` |
| A4 | GitHub Actions: gofmt, vet, `go test -race`, staticcheck if on PATH, `validacion/*`; README and REGLAS state only what is enforced and what a worker can still do | workflow file valid; scripts pass locally | `.github/workflows/ci.yml`, `README.md`, `docs/REGLAS-HARNESS.md` |

Order notes: Q1 follows B6 because it runs inside the sandboxed verification. U1 follows Q1, not C3, because it edits the quality level and the box credential key, which exist only after Q1 and B6. Later rows for Q1, listed and not built: lint, SAST with base comparison, dependency audits (need network, so outside the box), held-out tests (P8). B5 waits for `git merge harness-v2` (sandbox package), which lands before it by order.

## Checkpoint
- Phase: D. Integrated and verified on the integrated revision (tip 7b37f02 plus this commit): A1 gate, A2 verification and lanes, B1+B2 sandbox, C1 config, C2 terminal driver with account per role, C3 machine awareness. Evidence at 7b37f02: `go test -race ./...` green; e2e-v05 74/74; validar-mvp 18/18; aislamiento.sh 13 pass, 1 pending (forged-state close, owned by B5); `orq config ver` and `orq status` show configuration origin and RAM/CPU capacity.
- Builder: one Fable session, pane `w3X:pE`, worktree `~/worktrees/cthulhu-build`, branch `v2/build`, now on B5. Remaining in its order: B5, B6, C6, W1 vigia, C4, C5, Q1, U1, A4.
- Agile design (routing by signals, context map, UI evidence, comprehension report, watcher, loop ledger) written by the architect (Fable) and under attack by Astra (Codex); both files live in the orchestrator's scratch dir until consolidated here. Its activities reach the builder as an addendum after the critique.
- Proposals P1–P10 and the watcher approved by the Owner on 2026-11-13.
- Owner decision 2026-11-13: the repository is English (comments, identifiers, CLI messages, commands, docs, commits); only what a role says to the Owner stays Spanish. New work is English from now; activity L1 (builder, before A4) translates the rest, keeping the Spanish command names and JSON keys as accepted aliases.
- Integration branch: `harness-v2` (local only). Integration owner: orchestrator pane `w3X:p1`. Installed `~/bin/orq` 1.0.0 untouched; rollback = `main` 1f2d0b0.
- Review owed: independent review of B5/B6 (high risk) before acceptance.
- Pending Owner decision: dedicated credential for launching a real agent inside the sandbox.
- Spend: not measured by orq; the builder pane showed about USD 7.5 at its first ten minutes.
- Next action: consolidate Astra's critique with the architect, send the addendum to the builder, keep integrating at activity boundaries.
