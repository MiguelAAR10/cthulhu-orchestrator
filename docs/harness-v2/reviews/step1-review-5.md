# Step 1 review 5 (range e4a74ee..b6080eb, fixes for review 4)

`cd harness && go test ./...`: every package ok. Probes ran in a temporary `internal/gate/zz_probe_test.go` (now deleted). It used a real main repo and a marked clone on `slice/01` with the real `IsWorker`/`RepoPropio`, and ran each allowed command with bash in the clone.

## Review 4 findings

1. **Nested .git / gitfile: fixed.** `repo.go:109` (`gitEntre`, `repo.go:231`) denies git when a `.git` lies between `c.dir` and the root. `reglas.go:287` (`puntoGit`) denies creating, writing, moving or linking any other `.git`, and `reglas.go:391` covers `cp/rsync/install` of a `.git`. Review 4's bypasses are now denied.
2. **`-c` / `git config` / alias: partly fixed.** `repo.go:115` allowlists `-c`, `repo.go:194` allowlists `git config` writes, `repo.go:135` denies anything that is not a builtin, and `gitEnvCmd` covers the env vars. Every vector review 4 named is now denied. Two parts of the same class are still open:
   - **Command-running options on builtins.** `fetch`, `ls-remote`, `clone` and `pull` are in `gitBuiltin` (`repo.go:33`), and nothing checks `--upload-pack`/`-u`. Probe: `git ls-remote --upload-pack='git --git-dir=<main>/.git update-ref refs/heads/hackUP HEAD; git-upload-pack' .` was allowed and created `refs/heads/hackUP` in main. `git fetch --upload-pack=… .` created `hackF`. `git clone -u …`, `git archive --remote=. --exec=…` and `git grep -O<cmd>`/`--open-files-in-pager` were also allowed. They are the same kind of thing as the `rebase --exec` and `bisect run` cases this fix already denies. (`push --receive-pack` is denied, because push is denied.)
   - **Global git config written directly.** `git config --global core.pager` and `GIT_CONFIG_GLOBAL` are denied, but a plain write to `~/.gitconfig` is not. Probe: `printf '[core]\n\tfsmonitor = "git --git-dir=<main>/.git update-ref refs/heads/hackH HEAD; true"\n' >> ~/.gitconfig` was allowed. A later, allowed `git status` then created `refs/heads/hackH` in main. The write is in the parsed line, so this is not the "config file the gate does not see" gap.
3. **`podar` orchestrator-only: fixed.** `reglas.go:144` adds `podar`, and `reglas.go:170` checks the set. Tests cover `orq slice podar` and `orq --slice 01 slice podar`.
4. **Copied/renamed `orq`: fixed** (better than listing it as a gap). `cli/proyecto.go:419`: `orq slice <orchestrator subcommand>` refuses when cwd is a worker. Covered by `cli/gate_clon_test.go:49`. The chdir escape is listed in "Known gaps", so it is accepted.

## Legitimate worker flow

These were all allowed and ran correctly: `git status`, `git add -A && git -c user.name=… -c user.email=… commit`, `git log`, `git diff HEAD~1`, `git branch -f slice/01 HEAD`, `git -C src log`, `go version`. Writes to `.gitignore` and `.git/info/exclude` are allowed. No regression found.

## Must fix

- Deny `--upload-pack`/`-u` (fetch, pull, ls-remote, clone, fetch-pack), `--exec` (archive) and `-O`/`--open-files-in-pager` (grep) for workers.
- Protect `$HOME/.gitconfig`, `$XDG_CONFIG_HOME/git/config` and `~/.config/git/config` from worker writes, in the same way as `.git/config`.

VERDICT: fix — (2a) builtin options that run a command (`--upload-pack`, `-u`, `--exec`, `-O`) reach main; (2b) a direct write to the global git config (`~/.gitconfig` core.fsmonitor/pager) reaches main.
