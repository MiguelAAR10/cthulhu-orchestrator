# Brief A1 · gate hardening (branch v2/gate, worktree ~/worktrees/cthulhu-gate)

Read /tmp/claude-1000/-home-miguel-projects-MiguelAR10-Builder-AI-Edge-Runners-experiments-cthulhu-orchestrator/97993ac3-28de-4b5f-887a-6d3b8a94f645/scratchpad/briefs/COMMON.md first.

## Why
An audit fed 18 evasion commands to `orq gate` from a worker worktree and all 18 were allowed; 4 harmless commands were blocked.
Reproduce: `bash /tmp/claude-1000/-home-miguel-projects-MiguelAR10-Builder-AI-Edge-Runners-experiments-cthulhu-orchestrator/97993ac3-28de-4b5f-887a-6d3b8a94f645/scratchpad/probe.sh <an-empty-temp-dir> $(pwd)` from the worktree root (it builds the binary and prints BLOCK/ALLOW per command; read it first).

## You own
harness/internal/gate/ (all files) · templates/claude-settings.json · the `hookSettings` const in harness/internal/cli/v03.go · `realEnv`, `bundleAbierto`, `runGate` in harness/internal/cli/cli.go · the gate rows of docs/REGLAS-HARNESS.md.

## Result required
1. **Quote evasion closed.** `Normalizar` (gate.go) replaces quotes and backslashes with a space, so `git pu""sh`, `r""m -rf .git`, `orq sl""ice cerrar`, `> tests/acep""tacion/x` pass. Replace the regex-on-raw-text approach with a small shell tokenizer: split into segments (`;`, `&&`, `||`, `|`, `&`, newline, subshells), honour single and double quotes and backslash while building words, and recurse into `sh -c`, `bash -c`, `zsh -c`, `eval`, `xargs`, `find -exec/-delete`, `env`, `sudo`, `nohup`, `time`, `command`.
2. **Rules match the command word of a segment**, not text inside arguments. `grep -rn "git push" docs/`, `git commit -m "docs: explica git push --force"`, `echo "no uses rm en worktrees" >> notas.md` and `rm -rf node_modules/.cache/worktrees-plugin` must be allowed.
3. **Unresolvable targets fail closed inside a worktree.** When the cwd is a linked worktree and a write target or command word contains an expansion the gate cannot resolve (`$var`, `${...}`, `$(...)`, backticks), deny with a message telling the worker to write the literal path. Outside a worktree nothing changes.
4. **Harness state is protected from workers.** From a linked worktree or from a subagent (`agent_id` set), deny writes (Edit/Write/MultiEdit/NotebookEdit and shell write targets) to: the main checkout's `.orq/`, the orq state dir (`state.Dir()`), the limits file (`limits.DefaultPath()`), and any `.claude/settings.json` or `.claude/settings.local.json`. Add what you need to `gate.Env` so tests inject these paths. Running `orq ...` itself stays allowed (it is the legitimate writer).
5. **A worker spawns no helpers.** `Agent`/`Task` from a linked worktree is denied even without `agent_id`. The README claims this; today only `agent_id` payloads are denied.
6. **More write vectors**, all resolved against the protected list: `cp -t DIR`, `mv -t DIR`, `install -t`, `git apply`, `patch`, `git stash pop/apply` (deny inside a worktree when a slice bundle exists: the target cannot be known), heredoc or stdin fed interpreters (`python3 - <<EOF`, `node --eval`, `node -p`, `ruby -e`, `perl -e`, `php -r`) that name a protected path.
7. **MCP tools.** Matcher in both hook templates gains `mcp__.*`. For an `mcp__*` tool called from a worktree, deny when any string value in `tool_input` resolves to a protected path.
8. `TestEvasiones`: a table-driven test with every command in probe.sh and its expected decision. The only probe allowed to stay ALLOW is `python3 /tmp/reescribe.py` (a script file the gate cannot read); assert it and say in a comment that verification and the sandbox catch it.
9. Existing gate tests keep passing; validacion/validar-mvp.sh part A (no --real) still passes when run with ORQ pointing at your built binary.

Keep it small: the gate is the first barrier with clear early messages; a sandbox (another activity) will be the real one. Prefer one simple fail-closed rule over ten clever regexes.
