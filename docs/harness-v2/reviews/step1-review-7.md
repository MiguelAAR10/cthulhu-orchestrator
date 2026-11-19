# Review of 1296336 (step 1, review 6 fix)

`go test ./internal/gate` passes. I tested against real git 2.43 and the gate (temporary test, since deleted).

- Literal spellings are denied: `--output=x`, `--output x`, quoted (`--"output"`, `'--output=…'`), escaped (`--out\put`), `--outp=` and forms after `--grep=--`. Git 2.43 rejects abbreviations for log, diff, show and rev-list anyway. `-o` means output on none of the allowlisted subcommands. `rev-list` and `reflog` do take `--output` (it wrote files in real git), and both are denied.
- Stopping at `--` is safe. In real git, no option that takes a value accepts `--` as that value (`--grep -- …`, `-S -- …` and `-U -- …` all fail or end the options), so `git diff -- --output=x` is a pathspec.
- `ls-tree` and `check-ignore` have no option that writes or runs a command (`--format` and `--stdin` only read).
- MUST-FIX 1: `saleArchivo` ignores dynamic words. The gate allows `O=--output; git log -1 --format='[x]%ny=1' $O /x/home/.gitconfig` (also `$(echo --output)`). In real bash this wrote a file in config format (`[x]\ny=1`) to an arbitrary path. Deny git when any argument is `dyn` and could be an option, or at least when any dynamic word appears before `--`.
- MUST-FIX 2: the `reflog` subcommand check has the same gap. The gate allows `S=expire; git reflog $S --expire=now --all` and `git reflog $(echo expire) …`, and in real git this emptied the reflog (1 → 0 entries). Deny when `p[0].dyn`.
- MUST-FIX 3: the reflog check is a denylist, and `git reflog drop` (Git ≥ 2.50) is not on it. The gate allows `git reflog drop HEAD@{1}`, and on newer git that deletes reflogs. Add `drop`. Better: allow only no positional, `show`, `list`, `exists`, or a ref name (no `expire`/`delete`/`drop`/`write`), and say in the docs which git version the list targets.
- Minor: denying `reflog exists` costs nothing but is not needed (it only reads). The test expectations for `expire` and `delete` match on the substring "git reflog", which is weak; they would pass on an unrelated reflog message.

VERDICT: fix — (1) deny dynamic words that can become `--output`; (2) deny a dynamic first positional of `git reflog`; (3) add `reflog drop` (and any future writer) with a test.

## Orchestrator classification (2026-11-18, evaluated commit 1296336)
- Blocks the gate's intended scope (fix now): MUST-FIX 1–3. They are in the command line the gate parses.
- Needs the step 2 box: nothing new. Already listed: scripts/interpreters, hooks in the clone's own `.git`.
- Can wait: `reflog exists` is read-only. The weak `expire`/`delete` test expectations are folded into the fix, because a test that does not prove its name is a defect.
