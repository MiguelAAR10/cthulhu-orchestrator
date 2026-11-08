---
name: orq-board
description: Use when the Owner asks "how are we doing?", wants status, progress, a dashboard, what is blocked, who is working on what, or what needs them. For Atena (and the Owner). Runs `orq board` and explains it in business words. Read-only: it never changes state and never invents a number.
---

# Board · orq

You answer "how are we doing?" from files, not from memory. `orq board` reads `.orq/` and prints one screen. You translate it into plain business language for the Owner.

## Steps
1. Run `orq board --json`. It is the source of truth (schema `orq.board/v1`).
2. Explain the result in this order, the same order the board uses:
   1. **Stoppers first.** These are the things only the Owner can unblock. For each one, say what is needed and where to put it (for example, "a Stripe test key, in the env var `STRIPE_TEST_KEY`"). Say which work waits on it and that everything else keeps going.
   2. **Overall progress**: `overall.pct` and "`measured` of `total` slices measured".
   3. **One line per slice**: the product title, the status word (BLOCKED, AT RISK, IN PROGRESS, NOT PLANNED, DONE), the % done, and what is still missing (`missing`, in product words).
   4. **Workers**: only mention the ones whose verdict is not `ok` (stuck, dead, timeout).
   5. **Velocity** and rework, only if the Owner asks or something looks off.
3. If the Owner wants something to look at or share, run `orq board --html`. It writes `.orq/board.html`, a single file that opens offline. Use `--out PATH` to write it somewhere else.

## Rules
- **Never invent a number.** If a field is `null`, say "unknown" and why (for example, "no plan yet"). Never estimate it.
- No jargon. Say "slice" only if the Owner already uses the word. Say "tests passing" instead of "receipt". Never use phase names like IMPLEMENTANDO unless asked.
- Read-only. You never resolve, edit, or launch anything from this skill. To act on a stopper, use the `orq-stopper` skill (`orq stop resolve`, which only Atena or the Owner may run).
- Never print or repeat a secret. Stoppers say where a secret is, never what it is.
- Keep it short: about five lines for a normal update. Add detail only when asked.
