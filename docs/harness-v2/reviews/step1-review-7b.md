# Review of 6237819 + 22b5683 (step 1, review 7 fix)

`go test ./internal/gate` passes. I tested against real git 2.43 and the gate in a scratch clone at 22b5683 (temporary test, since deleted).

- MUST-FIX 1, PARTLY fixed. `dinamicaAntes` (repo.go:285) now denies `$O`, `${O}`, `$(...)`, backticks, `$'…'`, `{$O,}`, `-1$O` and `*` before `--`. But the 22b5683 exception lets a dynamic word through whenever the previous literal word is in `valorGit` (repo.go:293, 302). That set is global, while `-m` and `-F` are flags in log/show/diff (`-m` = merges, `-F` = fixed-strings), and it never checks whether that word is itself a value. All of these are ALLOWED by the gate, and in real bash each wrote a 135-byte file at an arbitrary path: `O=--output=/p; git log -1 -m $O`, `git log -1 -F $O`, `git show -m $O`, `git log -1 --grep -m $O`. Also `git log -1 --format "$O"`: git 2.43 rejects `--format` with a separate value, but it first truncated an existing file to 0 bytes (`keep` → 0). `git log -1 -m *` is allowed too, so a file in the cwd named `--output=…` becomes the option.
- MUST-FIX 2, FIXED. `S=expire; git reflog "$S"`, `` `echo expire` `` and `$(echo expire)` are denied by `dinamicaAntes` (repo.go:135), which runs before the reflog check.
- MUST-FIX 3, FIXED. `reflogLeer` (repo.go:325) is an allowlist: `drop`, `write`, `expire`, `delete` and `-q expire --all` are denied; `reflog`, `show`, `list`, `exists HEAD`, `HEAD` and `refs/heads/x` are allowed. The exact-reason test helpers fix the weak substring checks. `expire.x` passes as a "ref name"; git treats it as a ref, so that is harmless.
- Legitimate flow (item 3), WORKS except one form. Allowed: `git commit -m "$msg"`, `git commit -m "$(cat <<'EOF' … EOF)"`, `git add -A`, `git log --format="$f"`, `git diff -- "$F"`, `git status`, `git switch -c slice/01`, `git checkout slice/01`. Denied, and a false positive: `git commit -am "$m"`. This is a common clustered form, and `valorGit` (repo.go:302) holds only the bare `-m`, so `-am` is not recognised as taking the next word as its value.
- NEW (parsed command line, reproduced): the same valorGit hole defeats the own-branch decision. `X=-f; git branch -m "$X" slice/02` is ALLOWED (the literal `git branch -m -f slice/02` is denied by the `fuerza` check, repo.go:144, because `t` sees `$X`, not `-f`). In real git it ran `branch -m -f slice/02`, which replaced the other worker's `slice/02` (f6df5f7 → 2b94fdc) and removed `slice/01`. So this changes the branch decision as well as `--output`.
- Fix direction: treat the next word as a value only for an option that takes a value in that subcommand (commit `-m/-F/--file/--message/--author`; log/show/diff `-S/-G/--grep/--author`, not `--format` with a space). Also require that the option word is not itself a value. Allow the clustered `-am`/`-aF` as attached value options. Add the rows above as tests.

VERDICT: fix

## Orchestrator classification (evaluated 6237819 + 22b5683, integrated 9df1eaf)
- Blocks current scope: the global `valorGit` exception (option values, `-am`, `git branch -m "$X"`). Second miss on the dynamic-word item, so the premise changes: dynamic words before `--` are denied for every worker git subcommand except values of `git commit` message/author/file options, consumed left to right.
- Needs the box: nothing new.
- Can wait: nothing.
