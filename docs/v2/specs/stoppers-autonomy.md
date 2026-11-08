# Spec — `orq stop` and autonomy modes

Roadmap: Owner requirements 2026-11-08, slices 4 and 5. Related: H-01 (contract is Owner-only).

## What the Owner gets

When an agent needs something that only a human can give (a key, a login, a choice), it raises a **stopper** with a short plain sentence. Example: "I need a Stripe test API key to check payments. Put it in env var `STRIPE_TEST_KEY`." Only the tasks that depend on it stop. All other work continues. The board lists stoppers first. The Owner answers by doing the thing and resolving the stopper. Secrets never pass through orq.

Two autonomy modes let the Owner choose between being asked often (`guided`) and pre-approving a scope and budget (`free`).

## Commands

```
orq stop raise --type credential|api_key|decision|confirmation|access \
               --severity blocking|soon|fyi --slice S-NN [--tasks T1,T2] \
               --text "plain sentence for the Owner" [--options "a|b"] [--where ENV_NAME]
orq stop list [--open] [--slice S-NN] [--json]
orq stop resolve ST-NNNN [--provided-at "env:STRIPE_TEST_KEY"|"file:~/.config/x (outside repo)"] \
               [--choice "a"] [--note "..."]
orq stop cancel ST-NNNN --note "..."      # raiser no longer needs it
```

- `raise` checks that every task in `--tasks` exists in `.orq/planes/NN.json`. When the severity is `blocking`, it marks those tasks as held: `plan.Listas` treats a task with an open blocking stopper as not ready, and gives the reason `stop ST-NNNN`. A `soon` or `fyi` stopper holds nothing.
- `decision` requires `--options`. `resolve` on a decision requires `--choice`, and also writes a memory item (`cat=decision`, `tipo=decision`, `fuente=owner`, `evidencia=ST-NNNN`).
- Only the roles `atena` and `owner` may run `resolve`. Workers and the orchestrator may run `raise` and `cancel`. The CLI enforces this the same way it enforces role checks for contract commands under H-01.
- Re-raising with the same `(slice, type, text)` while one is still open returns the existing id. This is idempotent, like `canal.Clave`.

## Secrets rule

`.orq` never stores a secret value. At `raise` and `resolve`, every free-text field is rejected (exit 2, message "looks like a secret; say where it is, not what it is") if it matches either of these:
- a high-entropy token of 20 or more characters (Shannon entropy > 3.5 bits/char), or
- a known prefix (`sk-`, `ghp_`, `xox`, `AKIA`, `AIza`, `-----BEGIN`).

`--provided-at` accepts only `env:NAME` (where NAME matches `[A-Z_][A-Z0-9_]*`), `file:PATH` (PATH must be outside the repo root, or matched by `.gitignore`), or `vault:REF`. Resolving records `provided: yes` and the location. It never records the value. `resolve` can check that `env:NAME` is set in the resolver's environment, and prints `set`/`not set`, but never the value.

## Data contract — `.orq/stops.jsonl` (append-only, folded by id)

```json
{"id":"ST-0003","ts":"RFC3339","event":"raised|resolved|cancelled",
 "type":"api_key","severity":"blocking","slice":"S-04","tasks":["T2"],
 "by":"w-s04-t2","text":"...","options":["a","b"],
 "provided":"yes|no","provided_at":"env:STRIPE_TEST_KEY","choice":"","note":""}
```

The fold gives `state` = open | resolved | cancelled, taken from the last event. Writes use the same lock as other `.orq` read-modify-write operations (slice 4).

## Autonomy — `.orq/config.json`

```json
{"autonomy":{"mode":"guided|free",
  "scope":["harness/internal/plan/**","docs/**"],
  "budget":{"minutes_per_task":45,"max_workers":4,"max_attempts":3},
  "approved_by":"owner","approved_at":"RFC3339"}}
```

- **guided** (default): every `decision` and `confirmation` stopper reaches the Owner, whatever its severity.
- **free**: inside `scope` and `budget`, the orchestrator or planner resolves `decision`/`confirmation` stoppers of severity `soon`/`fyi` itself, using `--note "auto: <reason>"`. Each one is logged to memory as a decision with `fuente=orquestador`. Atena summarizes them in the next status.
- Only the Owner can switch to `free`. The switch is set through Atena and records `approved_at`. A scope or budget change is treated as a new approval.

**Always reaches the Owner, in any mode:**
1. Any `blocking` stopper, and any `credential`, `api_key`, or `access` stopper.
2. Product acceptance: a demo of a VERIFICADO slice.
3. Anything irreversible: data deletion or migration in a shared or production environment, push to a remote, publishing, spending money, or sending messages to real users.
4. Any change to the acceptance contract or exam (command, bundle, scope), per H-01.
5. Anything outside `scope`, or any overrun of `budget`. A task over `minutes_per_task` automatically raises `confirmation/soon`.

## Acceptance tests

1. **Given** plan S-04 with T1 and T2 independent, **when** a worker raises `api_key blocking --tasks T2`, **then** `orq plan listas` returns T1 only, and gives T2's reason as `stop ST-0001`.
2. **Given** `--text "use sk-live-abc123def456ghi789jkl"`, **when** `raise` runs, **then** it exits 2 and `stops.jsonl` does not change.
3. **Given** an open stopper, **when** a worker runs `resolve`, **then** it is refused. **When** Atena runs `resolve --provided-at env:STRIPE_TEST_KEY`, **then** the folded state shows `provided: yes`, the location is recorded, and T2 becomes ready.
4. **Given** mode `free` and a `decision soon` stopper inside scope, **when** the orchestrator resolves it, **then** the resolve is accepted and a memory decision item appears. **Given** the same stopper with severity `blocking`, **then** the orchestrator's resolve is refused.
5. **Given** mode `free`, **when** a worker attempts `orq slice examen`, **then** it is still refused (H-01 holds).
6. **Given** the same stopper raised twice, **then** a single id is returned.

## Out of scope

Storing or forwarding secrets, notifications (email or chat), automatic secret rotation, multi-owner approval, and spend tracking in money.
