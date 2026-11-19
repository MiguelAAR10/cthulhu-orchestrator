# Review of 8bbaa55..a6bf038 (worker git allowlist, global git config)

`cd harness && go test ./...`: all packages ok. Temporary probe test `internal/gate/zz_probe_test.go` deleted afterwards; the tree is clean.

## Must fix
1. **`--output=<file>` on `log`/`diff`/`show` is an unchecked write to any path.** It is a diff option that git honors on every allowlisted diff-producing subcommand. With `--format` the worker controls every byte (`%n`, `%x09`). The gate allows all of these:
   - `git log -1 --format='[core]%n%x09fsmonitor = "git --git-dir=<main>/.git branch hackO HEAD; true"' --output=$HOME/.gitconfig`
   - `git log -1 --format=… --output=.git/config` (the clone's own config, protected since review 4)
   - `git diff --output=tests/aceptacion/a_test.go` (the exam), `--output=<main>/.git/config`, `--output=<state>/s.json` (slice state)
   - the separate-argument form `git diff --output x.patch` too.
   Demonstrated with real git in a scratch clone: the first command wrote the global config, and the next plain `git status` in the clone created branch `hackO` in the main repo. This undoes fix 2b and the `.git/config` protection, and it reaches the exam and the state directly. Fix: treat `--output`/`--output=` as a write path (run it through `cubre`/`puntoGit` like a `>` redirect), or deny it outright. A worker can use `> file` instead, which the gate already checks. Add deny rows for both forms.

## Checked and held
- `-c` beyond `user.*`/`color.*` is denied (`core.pager`, `user.signingkey`). `PAGER`/`EDITOR`/`GIT_*` assignments are denied. `grep -O`, `--ext-diff`, `--textconv`, `tag`/`worktree` writes are denied. Ref forcing on `main` (`branch -f/--force/-fq`, `checkout -B`, `switch -C`) is denied. Path forms that reach the exam (`checkout <ref> -- tests/…`, `restore --source`, `rm`, `mv`, `stash pop`, `show > tests/…`) are denied.
- `cat-file --filters` and attribute drivers (diff/filter) need config keys the worker can no longer set, apart from the `--output` hole above.
- Hooks in the clone's `.git/hooks` stay writable and `git commit` runs them. The report lists this under known gaps until step 7, so I'm not reporting it.
- Minor, not must-fix: `git help -w/-i` and `MANPAGER=… git help` start a browser, info or man, which read their own environment and config. `LESSOPEN` with `git -p` is the same. All of these are helper or pager paths outside the parsed line.

## Legitimate flow
`status`, `add`, `commit` (including `-am` and `--amend`), `log | head`, `diff --stat`, branch work on `slice/01`, `go test ./...` and `cd src && go test` all pass. These are now denied: `merge main` (catching up with main), `reflog` (recovery after a bad `reset`), `revert`, `cherry-pick`, `ls-tree`, `rev-list`, `check-ignore`, `clean -n`. Adding the read-only ones (`reflog`, `ls-tree`, `rev-list`, `check-ignore`) is cheap. That is the orchestrator's call, not a blocker.

VERDICT: fix — (1) treat `--output[=]` on `log`/`diff`/`show` (and any allowlisted subcommand that takes diff options) as a protected-path write, or deny it, with deny rows for `$HOME/.gitconfig`, `.git/config`, the exam and both option forms.
