# orq2 v2.1 lean plan (synthesis, 2026-11-21)

Sources: one-pass reviews by Fable and Astra (`~/cthulhu-evidence/2026-11-21/lean/fable.md`, `astra.md`), two
Haiku scout fact sheets (`lean/hechos.md`), defect ledger `orq2-defects.md` (E01–E27), Owner decisions D1–D13.
Owner ask: lean, fast, always proof of how it was built; 5 days of token budget.

## Where both reviewers agree (adopted)
- The gates are right; what slows orq2 is that gates punish failures that are not code (environment, exam
  paperwork, dead sessions). Fix what counts and chain the steps; remove almost nothing that protects value.
- Keep: Atena questions (won M1), red test, sealed exam + receipt (won M5), independent review, Owner close.
- Remove the routine plan debate (Astra) for small and medium work: today it is a mark set without the debate
  (E12). Large or risky design only, with a receipt for that plan version.
- Card approval wakes the Orchestrator and chains open + dispatch (E10).
- `init` detects checks per folder and writes the shared environment profile (D13); a preflight runs every declared
  check in the box on the base revision without counting; clones come ready (trust, deps, profile).
- Failures go to whoever can fix them: environment → orchestrator, exam file → planner, product → worker counter.
- One retry path re-seals an approved exam change and adds the named files to the zone, also from BLOQUEADO.
- Every closed story leaves one proof page **in git** (`docs/historias/S-NN/`), generated at close from data orq2
  already has: what was asked (card + who answered), what was built (diff stat), that it works (receipt, checks per
  folder, exam intact since red), how it looks (2 screenshots from an automatic demo in its own clone), second
  opinion (review verdict + open debt), what it cost (phase times, attempts by cause, model per session, Owner minutes).
- Not in v2.1: new message bus, new panel, exact token accounting, credential isolation beyond today (D1), skills
  catalogue (D5), living C4 docs (D9), mutation on every story (D11).

## Where they differ, and the call
| Topic | Fable | Astra | Call |
|---|---|---|---|
| Network in the exam box | yes/no per check | allow only declared services | **yes/no per check now**; allowlist later (fits 5 days) |
| Card approval | Owner command, chained | approval in the conversation, bound to card version and to who relayed it | **keep the command, record who decided and who relayed** (fixes E18 too) |
| Review findings | only High reopens by default | depth by risk | **only High reopens by default**; Owner can raise per story (S-02's Medium fix cost 26 min) |
| Small frictions (E11, E17–E19, E21) | one Haiku package | leave open | **include, Haiku, in parallel** (cheap) |
| Last half day | — | one short real story end to end | **adopt** |

## The plan (7 changes + 1 cheap package, ordered by minutes recovered in S-02)
| # | Change | Removes | Model | Size | Proof test |
|---|---|---|---|---|---|
| 1 | Honest attempts: receipt `causa codigo/entorno/examen`, only code counts; suppression guard + lint on the exam before sealing | E07, E13, E14 | Sonnet | ½ d | missing tool → counter unchanged; `type: ignore` in exam → refused at seal |
| 2 | One-step re-seal in `reintentar --causa examen`, from BLOQUEADO, with zone files and recorded approval | E23–E26 | **Opus** (trust piece) | 1 d | replay S-02 a2 on a toy repo: one command, 0 extra attempts; unapproved exam change still "trampa" |
| 3 | Environment profile + preflight + ready clones | E05, E06, E08, E09 | Sonnet | 1–1½ d | RS clone: preflight green incl. dashboard, no hand `--deselect`, no trust dialog |
| 4 | Approve = open + dispatch + event; `debatido` only for large work with a receipt | E10, E12, E22 | Sonnet | ½ d | after `aprobar`, first worker runs with no other command |
| 5 | Confirmed delivery + session lease + `sesion cerrar --muerta` + one automatic resume | E01–E03, E20 | Sonnet | ¾–1 d | kill a pane → shown dead, send fails loudly; pasted-unsent prompt is submitted |
| 6 | Worker contract runs every declared check; `juntar` refuses otherwise | E15 | Haiku | 1 h | stale old test → refused at `juntar` |
| 7 | `orq2 demo` in its own clone + proof page in git at close + `recibo` on closed stories | E27, proof gaps | Sonnet (template Haiku) | 1 d | close a story → page with 6 sections and 2 screenshots, no human step |
| 8 | Friction package | E11, E17, E18, E19, E21 | Haiku | ½ d | one test per ledger row |
Last half day: one short real Real State story end to end on the new build.

## How we know it got lean (next real story vs S-02)
1. Story → first worker, minus documented external waits: S-02 ≈25 min → target ≤15.
2. Extra attempts with no code defect: S-02 3 (1 environment, 2 paperwork) → target 0.
3. Manual interventions outside orq2: S-02 ≈10 kinds → target ≤2. And the proof page exists with no human step.
