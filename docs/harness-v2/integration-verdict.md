# orq2 integration verdict (Fable, judge)

2026-11-19. Read-only judgement from `git log`/`diff --stat`/`status` on the six lanes plus the closing and
agent-first plans. No code was read line by line; no test was run by me.

## 0. Two facts that change the order

1. **`harness-v2` does not contain P1–P8.** It is `24f2438` plus four docs commits (`5ff5c72`). P1–P8 live only
   on `75bb0be`, which is not an ancestor of `harness-v2`. The installed binary is therefore pre-P1.
2. **`v2/locks` was cut from `5ff5c72`, so it lacks P1–P8** (no P7 `limits`, no `check --caja`). Anything built
   there now is built on the wrong base. It has 0 commits and a clean tree, so nothing is lost by pausing.

Half of what is called "in the lanes" is uncommitted: P12 (build), P11 (fix), the `panel` CLI wiring, and two
plan docs staged in `v2/memoria`. Uncommitted work is the disorder risk, not the number of lanes.

## 1. Verdict per lane

| Lane | Verdict | Why |
|---|---|---|
| `v2/build` P9 (`277252d`) | **KEEP** | Committed, 105 lines, tested. Carries P1–P8 into `harness-v2`. Merge first |
| `v2/build` P12 (dirty) | **KEEP if committed green by T+15, else PARK** | Pilot value is real (worker cap, RAM warning) but it edits `limits.go`, which F7 also changes |
| `v2/review-fixes` F1–F7 | **KEEP** | Seven reviewed defects, one commit each, 286 lines. Highest value per line in the window |
| `v2/review-fixes` P11 (dirty) | **KEEP** | Guide text only; removes what the pilot actually tripped on (`--en-sitio`, caps, `check --caja`) |
| agent-first steps 4, 1, 5 | **KEEP step 4; PARK 1 and 5 unless green by T+15** | Step 4 (all cap violations at once) is small and was hit. Step 1 (command table) touches every command |
| `v2/panel` | **PARK** | Read-only and isolated (new package, layer 1), but 3,474 lines would triple the review scope. First merge of the next window |
| `v2/locks` | **PARK, restart on the new base** | Wrong base, just started, touches `gate` and `config`: the riskiest lane to rush |
| `v2/memoria` | **STOP** | No code exists. `mem reemplazar` goes to the backlog; E5 is already fixed by hand |
| `v2/tui` | **STOP (do not start)** | Raw termios through `syscall`; the panel gives the Owner the same view at a fraction of the risk |

## 2. Does "not before the stop rule" still hold?

It was written assuming the Owner types `aprobar`. He now says agents run every command, so:

- **Lifted for locks.** The process rule "only the Owner types `aprobar`" (E11) is void when agents type
  everything. Owner confirmation via TTY is now the one real safety gap. Next window, on the correct base.
- **Lifted for the read-only panel.** An Owner who runs nothing needs one place to look. It writes nothing.
- **Still holds for the TUI, citation checks in code, journal events and new `metricas` output.** None of them
  is made necessary by agents running commands; the 3-citation spot-check by hand stays.

## 3. Merge order into `harness-v2`

Rule: **each builder merges `harness-v2` into its own branch, resolves there, runs `go test ./...`, then the
integrator merges with `git merge --no-ff`.** No rebase, no `--force`, no resolving on `harness-v2`.

1. `v2/build` at `277252d` (P9 + P1–P8). No conflict expected: `harness-v2` added docs only.
2. `v2/review-fixes` F1–F7 (`d448bbe`). No conflict with P9 (different files).
3. `v2/review-fixes` P11, then step 4, after it has merged `harness-v2`.
4. `v2/build` P12, after it has merged `harness-v2`. Only if it met T+15.

Expected conflicts and the ruling for each:

| Where | Between | Resolution |
|---|---|---|
| `cli/skill.go`, guide text | P9 (landed) vs P11 (dirty) | P11 keeps P9's sentence untouched and adds around it |
| Card caps in guides | P9 `tarjeta_guia.go`, P11 `guias_orq2.go`, step 4 | **No literal number in any guide.** Caps print from the `slice/tarjeta.go` constants (step 4). Where texts disagree, the constants win |
| `--en-sitio`, `check --caja` | P11 vs agent-first steps 2–3 | Same change twice. **P11 owns it; steps 2–3 are dropped as commits.** Step 1's guide test proves it later |
| `limits/limits.go` | F7 (landed) vs P12 (dirty) | F7 stands: negative `max_workers` is rejected, `SinTope` stays out of band. P12 expresses "no cap" through that sentinel |
| `cli/cli.go` | P12 `doctor --ram`, panel wiring, step 1 table | Additive: keep both sides. Whoever lands last adds its row |
| `backlog-next.md` | build (dirty) vs `harness-v2` | Keep both sets of lines |
| `closing-plan.md`, `agent-first-plan.md` | staged copies in `v2/memoria` | Unstage them; they already exist on `harness-v2` and would collide add/add |

## 4. The single gate before reinstall

All three, in this order, on `harness-v2` after the last merge. Any red: no install, the binary stays `24f2438`.

1. **One fresh Opus review**, scope `git diff 75bb0be..harness-v2 -- harness/` only: P9, F1–F7 checked one by
   one against `review-p1-p8.md`, P11, step 4, and P12 if it landed. P1–P8 are not re-reviewed.
   Must confirm that `orq` output is byte-identical (no golden changed).
2. **`make ci` green** from the main checkout (gofmt, vet, `go test -race`, e2e-v05, aislamiento).
3. **Real State clone proof** with the candidate binary in a throwaway clone, never the live `.orq`:
   `orq2 version` says `2.0.0-pilot`; `orq2 doctor` clean; `orq2 check --caja` passes; a card over the caps
   lists every violation in one run.

Then `make install`, and relaunch the roles between slices, not during one.

## 5. What to tell each builder now

- **build:** "Merge harness-v2 into v2/build once F1–F7 land, express no-cap through F7's SinTope, commit P12 green by T+15 or commit it as `wip:` and stop; commit your two pilot-fixes docs either way."
- **fix:** "Commit P11 now keeping P9's sentence and no literal caps, then step 4 only; steps 2–3 are absorbed by P11; steps 1 and 5 only if green by T+15, else leave them unstarted."
- **panel:** "Commit the CLI wiring and panel-plan.md on v2/panel, get `go test ./...` green, then stop; you are parked and merge first next window with your own review."
- **locks:** "Stop before writing code: your base lacks P1–P8; wait for the reinstall, merge harness-v2 into v2/locks, then start."
- **mem:** "Stop: run `git restore --staged` on the two plan docs, write nothing; the lane is closed and `mem reemplazar` moves to backlog-next.md."
- **tui:** not launched; nothing to send.

## 6. Clean-up rule

- **Merged branch:** `git worktree remove <path>` then `git branch -d <branch>`. Both refuse on dirty or
  unmerged work; a refusal means stop and look, never `--force` or `-D`.
- **Parked branch:** tree clean (a `wip:` commit is allowed), and one line in `backlog-next.md`:
  branch, last commit, what is left, the condition to resume. No line, no park.
- **Stopped lane (`v2/memoria`):** unstage, `git worktree remove`, `git branch -d` once step 1 of the merge
  order has landed (it points at `75bb0be`, which is then merged).
- **End-of-window check:** `git worktree list` and `git branch --no-merged harness-v2` show exactly the parked
  set (`v2/panel`, `v2/locks`, and `v2/build` only if P12 was parked); `git status` is clean in each.

## 7. Window budget (~45 min)

T+0 send the messages. T+15 commit cut-off. T+15–25 merges. T+25–40 review, `make ci`, clone proof.
T+40 reinstall or keep the old binary. Whatever is not committed green at T+15 is parked, without exception.
