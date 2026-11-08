# Independent QA and visual evidence (slices 8–9)

**Why.** The worker that built a slice is the worst judge of it. QA runs in a separate, fresh session that cannot change what it tests, and a bug only counts when orq can reproduce it.

## Roles
- **tester** (`ORQ_ROL=tester`, fresh session, `roles.tester` in `orq.agents.json`): read-only. The gate denies Edit/Write/MultiEdit/NotebookEdit and write-ish Bash (redirects other than `/dev/null`, `rm/mv/cp/tee/touch/sed -i`, `mkdir`, `git add/commit/checkout/...`, package installs, inline interpreters), plus every orchestrator-only command. It may run tests and `orq qa finding add`.
- **repairer** (`ORQ_ROL=repairer`, separate worker): may edit product code, never the hash-pinned exam bundle (H-01). The gate protects the bundle for this role even outside a linked worktree.

## Loop
1. Orchestrator: `orq qa run S-NN`. Runs the pyramid declared in `.orq/config.json` in order, inside the slice worktree:
   `{"qa":{"layers":[{"name":"unit","cmd":"go test ./..."},{"name":"integration","cmd":"..."},{"name":"e2e","cmd":"npx playwright test"},{"name":"mutation","cmd":"...","min_criticality":"high"}],"require_receipt_from":"medium"}}`
   Writes `.orq/qa/S-NN/receipt.json` with head, criticality and per-layer exit/passed/failed. Playwright is just a command; orq never launches a browser itself.
   It also records HEAD and `git status --porcelain` of the worktree before and after the layers: if anything changed outside the allowed reports (`out.tap` plus `qa.reports` globs), the receipt is `invalid` and does not count.
2. Tester: `orq qa finding add S-NN --requirement --commit --repro --observed --expected --impact`. Missing fields are rejected. orq re-runs `--repro`: **confirmed only if it fails**. A repro that passes is stored but never counts.
3. Repairer fixes; orchestrator re-runs `orq qa run`.

## Proportional rigor
Criticality is read tolerantly from `config.json` (`criticality` or `project.criticality`; low|medium|high, synonyms accepted; default **medium**).
- Layers with `min_criticality: high` (mutation, adversarial) run only at high; otherwise they are recorded as skipped.
- `orq slice cerrar --aceptado` requires a green QA receipt, and no confirmed finding newer than it, when criticality ≥ `require_receipt_from` (default medium). Low-criticality projects close without QA.

## Anti-rationalization
`guias/worker.md` and `guias/verificador.md` carry a 5-row excuse → rebuttal table (flaky, "green so it works", "adjust the test", "out of scope", "follow-up").

## Visual evidence
`orq evidence add S-NN --kind target|before|after --file PNG [--note "..."] [--pulir]`
- Stores `docs/slices/S-NN/{target,before,after}.png` and `evidence.json`.
- Pixel diff % after vs target (stdlib `image/png`, exact RGBA comparison, non-overlapping area counts as different, 2 decimals). Deterministic.
- Regenerates `docs/slices/S-NN/README.md` (≤40 lines): title, one plain-language outcome paragraph (from the note), acceptance (QA receipt), diff %, image links, receipts.
- `--pulir` asks the cheapest `docs` model to polish it, with the escriba timeout. If it fails, times out, drops the heading or exceeds 40 lines, the deterministic README stays.
