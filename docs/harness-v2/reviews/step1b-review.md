# Review: 27ffcfd (gate force-create) + 4dd597c (candidate check)

Method: a temporary test in harness/internal/gate (now deleted) sent 49 commands through the real `Evaluate` (own branch slice/01). Each command it allowed was then run with git 2.43 in a scratch repo, checking whether `main` moved.

1. 27ffcfd: **fails**. The forms named in the commit are now denied: -qB, -Bmain, -qBmain, -qC, -fC, --force-create[=], -qf, -qM, -qC. Work on slice/01 still passes: -qB slice/01, -qC slice/01, --force-create=slice/01, branch -f slice/01, commit. The gate still allows these, and each one moved `main` in real git:
   - Abbreviated long options (git accepts any unique prefix). `git switch --force-c main` (also `--force-cr=main`) and `git branch --forc main HEAD` are allowed. The rule matches only exact strings: repo.go:144 (`hay`/`conPrefijo` on "--force", "--force-create") and reglas.go:66 (`valorDe` with `largo+"="`).
   - A repeated flag. `git checkout -B slice/01 -B main` and `git switch -C slice/01 -C main` are allowed. `valorDe` (reglas.go:67) returns the first value, but git uses the last one.
   - `git branch --copy --force slice/01 main` is allowed and overwrites main. repo.go:147 checks only p[0] when the flags are long.
   - `git branch -m main x` / `--move` (not a force flag) is allowed and renames main away. repo.go:146 checks only -M/-C.
   - `git checkout main && git reset --soft HEAD~3` (also mixed `reset HEAD~3`, or `commit`) is allowed and moves main. `reset --hard` is denied, but only as destructive. Checking out another branch is never restricted.
   Fix: treat long options as prefixes, check every value of -B/-C/--force-create, check every positional for -m/--move and --copy/--force, and decide whether `checkout/switch <other branch>` should be allowed at all.

2. 4dd597c + validacion/candidato.sh: **holds**. With a stub `orq` (printing `revision=HEAD`) earlier in PATH, `candidato "$(command -v orq)"` fails, because the path differs (candidato.sh:9). `hash -p stub orq` changes what `command -v` prints, so it is also caught when it happens before the check. A wrong `ORQ`/`ORQ_CANDIDATO`, or a binary built from another revision, fails too (aislamiento.sh:29-31, candidato.sh:13-16). The check runs only once. After it, e2e-v05.sh does not change PATH or the hash, and aislamiento.sh:116-117 puts only a copy of the checked binary on PATH. Residual (no exploit in `make ci`): if e2e-v05 is given a relative BASE, line 22 puts a relative PATH entry first, and it resolves against a new cwd after `cd tienda`.

VERDICT: fix

## Orchestrator classification (evaluated 27ffcfd and 4dd597c, integrated 0730e9f)
- Blocks current scope: none for item 2 (holds).
- Needs the step 2 box: item 1. Every bypass moves the `main` branch *of the repository the worker runs in*. In a linked worktree that branch is shared with the main repo, but from step 2 workers run only in `--no-hardlinks` clones without a remote inside the box, where `main` is the clone's own ref and the main repo is not mounted. What protects integration is that `ingerir` takes only `refs/heads/slice/NN` from the bundle. That becomes a step 2 exit-test row (Task 6), instead of a third round of gate parsing for this class.
- Can wait: relative BASE in e2e-v05 (no exploit in `make ci`).
