# Step 1 review (range ceb1560^..HEAD; harness/, validacion/, Makefile)

`cd harness && go test ./...`: every package ok. Bypasses below were run with a temporary test
(`internal/gate/zz_probe_test.go`, since deleted) on a real main repo and marked clone with the real `IsWorker`/`RepoPropio`; each allowed command was then run with bash in the clone.

## Must fix

1. **A gitfile in a subdirectory points git at the main repo.** `repo.go:62` (`gitAjeno` only checks that `c.dir` is inside the clone) and `repo.go:170` (`RepoPropio` skips nested `.git`).
   Bypass: `mkdir -p sub && printf 'gitdir: <main>/.git\n' > sub/.git` is allowed. So is `git -C sub commit --allow-empty -m pwned`, and the main repo's `main` then had the commit `pwned`.
   `cd sub && git update-ref refs/heads/slice/01 HEAD` is also allowed: it counts as the worker's "own branch", but it moves main's ref.
   Fix: deny git when any `.git` (dir, file or symlink) lies between `c.dir` and `raiz`. Also protect writes to any path whose last element is `.git` inside a clone.
2. **`-c alias.*` and `git config alias.*` run unparsed shell.** `shell.go:619` and `repo.go:78`.
   Bypass: `git -c alias.x='!git --git-dir=<main>/.git update-ref refs/heads/hacked2 HEAD' x` was allowed and created `refs/heads/hacked2` in main.
   Same in two calls: `git config alias.y '!…'`, then `git y` created `hacked3`.
   `git -c alias.v='!orq slice verificar' v` is allowed too, which is also an item-1 bypass: a worker runs an orchestrator-only command.
   `-c core.fsmonitor=…`, `core.pager`, `core.sshCommand`, `core.hooksPath`, `diff.*.textconv`, `filter.*` and `sequence.editor` (all allowed, all run commands) are the same class.
   Fix: for workers, deny `-c` and `git config` for any key except a short allowlist (`user.*`, `color.*`, …), and deny running an alias that is not a builtin.
   These are not "scripts the gate doesn't read". The command text is in the line the gate parses, so they need a fix now.

## Should fix

3. **`orq slice podar` is not orchestrator-only.** `reglas.go:163`.
   Bypass: `orq slice podar w-front` from a clone is allowed. It rewrites the slice state (`Equipo`) and removes another worker's worktree, branch and panel once that worker is merged.
   Fix: add `"podar"` to the `slice` list.
4. **A copied or renamed `orq` binary escapes `soloOrquestador`, because it matches by basename.**
   Bypass: `cp …/orq ./o && ./o slice verificar` is allowed (shown with a stand-in binary).
   Same class as the script gap. Acceptable until step 7 only if the report lists it.
   One-line fix: none in the parser; the real fix is step 7 (box), or `orq slice` refusing to run when the caller is a worker (check `IsWorker(cwd)` in the CLI).

## The two gaps the report lists

- Scripts the gate does not read (`sh x.sh`, `python x.py`): acceptable until step 7, because no shell parser can close them.
- A symlink created in the same command line: acceptable, but low value next to finding 1. A *previous* call can plant `sub/.git` (a file or a symlink to `<main>/.git`), and the next call is still allowed. So "resolved once it exists" does not cover `.git`.

## Checked, no defect

`mv .git`, `read`/`for` setting `GIT_DIR`, `$G`-style git, and `git push <main>` are all denied.
`TestGateDeniesInClone`, `TestGateDeniesInRepoNestedInClone`, `TestWorkerGitContraElRepoPrincipal`, `TestSliceOrq1SeNiega` and e2e S-30 each deny for the reason their name gives.

VERDICT: fix — (1) nested `.git`/gitfile reaches main; (2) git `-c`/`config` alias and command-running keys; (3) add `podar` to orchestrator-only.
