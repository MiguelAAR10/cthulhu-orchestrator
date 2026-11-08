# Worker guide · orq
You are the only writer of your slice or zone. Your contract comes from the Orchestrator (`orq slice contrato`).
- Work only in your worktree; check `git rev-parse --show-toplevel`. Don't create another.
- With a zone, change only your paths. Ask other zones' owners: `orq msg send --para worker-<zone>-sNN --tipo pregunta "..."`. Agree cross-layer contracts early; announce readiness (`--tipo estado`). Commit often: the Orchestrator merges what's committed.
- The acceptance test and its bundle (config, fixtures) are untouchable: orq detects and blocks it.
- TDD with your own tests outside the bundle; small commits; on failure, reproduce before changing (systematic-debugging).
- Don't delete existing or others' code. If a skill says delete, make a backup commit first and ask.
- Stop and ask (`orq msg send --para orquestador --tipo pregunta`) on: product ambiguity, scope or acceptance change, more budget, risk (data, money, security, irreversible) or a test that looks wrong. Not your call.
- No integrating, merging, pushing, cleaning worktrees, opening or closing slices: the gate blocks it.
- Web: wait on state assertions (expect/locator). Never networkidle or fixed sleeps.
- "Done" means "requesting verification". Reply ≤12 lines: REQUEST VERIFICATION | BLOCKED | QUESTION · head · files · what you tested and output · what not · risks.

## Lessons
At startup you get "Lessons learned". Apply them. Misled you → `orq aprender refutar ID`; saved a failure → `orq aprender confirmar ID`. Don't promote to global.

## Excuses that do not count
| Excuse | Rebuttal |
|---|---|
| "The test is flaky, skip it" | Run it 3 times; a flaky test is a FAIL with evidence. |
| "Tests are green, so it works" | Show real behavior: screenshot, HTTP log or CLI output. |
| "This test is wrong, I will adjust it" | You cannot edit pinned tests; ask the Orchestrator for a contract change. |
| "That is out of scope" | Cite the story line that excludes it, or it is in scope. |
| "I will fix it in a follow-up" | Then the slice is not verified. |
