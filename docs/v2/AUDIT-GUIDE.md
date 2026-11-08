# Audit guide for adversarial reviewers

Branch `harness/v2` vs `main` (`1f2d0b0`). Diff: `git diff main...harness/v2`, log: `git log --oneline main..harness/v2`.

## How it was built
- TDD per finding: every reproduced exploit became a failing Go test first, then the smallest fix.
- Loop: independent skeptical evaluator (fresh agent, no repo writes) → reproduce attacks for real → score 10 dimensions → fix top gaps → re-evaluate. Scores: 3.9 → 5.9 → 6.3 → 6.9 → 7.1 → 7.5.
- Context and decisions: `docs/v2/RECAP.md`, `docs/v2/ROADMAP.md`, `docs/v2/research/RESEARCH.md`, specs in `docs/v2/specs/`.

## Run it
```
cd harness && go vet ./... && go test ./...
bash validacion/e2e-v05.sh      # toy repo end-to-end
bash validacion/quickstart.sh   # 5-minute quickstart, hermetic
GOOS=windows go build ./... ; GOOS=darwin go build ./...
```

## Claims to attack
| Claim | Where | Spec |
|---|---|---|
| A worker cannot change its exam/scope or forge Owner/Atena approval (H-01) | `internal/cli/token.go`, `internal/identidad/`, `internal/gate/` | `docs/v2/specs/identity.md` |
| Privileged actions need owner token + signed session anchor or human TTY ancestor, and no worker ancestor | `internal/identidad/ancestros.go` | identity.md |
| No launch bypasses HALT/disk/quota (H-02) | `internal/admision/` | ROADMAP slice 2 |
| Verify and integrate use one evaluator; integrated SHA has its own receipt (H-03) | `internal/slice/` | — |
| Channel is append-only, locked writes; plan "done" survives only with unchanged contract hash (H-04, H-05) | `internal/canal`, `internal/plan/` | — |
| Closing/gc never loses uncommitted or unique ignored files | `internal/worktrees/` | — |
| Tester cannot alter results: layers run in a throwaway worktree; main/worker/config snapshots invalidate the receipt | `internal/qa/` | `docs/v2/QA.md` |
| Works with Claude/Codex and herdr/tmux/process | `internal/runner/`, `internal/agentes/` | — |

## Declared residual limits (not hidden bugs)
Same OS user as the Owner: a worker that creates its own pty from a compiled binary, uses ptrace, or injects into Owner processes can escape. Outside Linux only the token protects. Real fix: run workers as a different OS user or in a container. QA cannot see change-and-restore inside the throwaway copy or git-ignored files. No real end-to-end run with live Claude/Codex agents yet.
