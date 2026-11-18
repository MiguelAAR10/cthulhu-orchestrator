# Brief A1 · command-policy checker: correct parsing (branch v2/gate, worktree ~/worktrees/cthulhu-gate)

Read /tmp/claude-1000/-home-miguel-projects-MiguelAR10-Builder-AI-Edge-Runners-experiments-cthulhu-orchestrator/97993ac3-28de-4b5f-887a-6d3b8a94f645/scratchpad/briefs/COMMON.md first. This is defensive work on our own tool: `orq gate` is a Claude Code PreToolUse hook that decides whether a tool call by a coding agent is allowed. Its command parsing is too naive, so its decisions are wrong in both directions. You are making the checker correct and adding regression tests.

## You own
harness/internal/gate/ (all files) · templates/claude-settings.json · the `hookSettings` const in harness/internal/cli/v03.go · `realEnv`, `bundleAbierto`, `runGate` in harness/internal/cli/cli.go · the gate rows of docs/REGLAS-HARNESS.md.

## Regression cases
`/tmp/claude-1000/-home-miguel-projects-MiguelAR10-Builder-AI-Edge-Runners-experiments-cthulhu-orchestrator/97993ac3-28de-4b5f-887a-6d3b8a94f645/scratchpad/probe.sh` holds the command table (run it as `bash /tmp/claude-1000/-home-miguel-projects-MiguelAR10-Builder-AI-Edge-Runners-experiments-cthulhu-orchestrator/97993ac3-28de-4b5f-887a-6d3b8a94f645/scratchpad/probe.sh <empty-temp-dir> $(pwd)`; it builds the binary and prints the decision per case). Expected decisions: every case in the sections before "falsos positivos" is BLOCK, except `python3 /tmp/reescribe.py` (ALLOW: a script file the checker cannot read; the verification step covers it) and `Edit package.json` (ALLOW: another activity handles runner files). Every case under "falsos positivos" is ALLOW.

## Result required
1. **A shell word parser replaces regex-on-raw-text.** `Normalizar` currently turns quote characters into spaces, so a command word written with embedded quotes is not recognised. Parse properly: split into segments (`;`, `&&`, `||`, `|`, `&`, newline, subshells), build words honouring single quotes, double quotes and backslash, and recurse into wrappers that run another command (`sh -c`, `bash -c`, `zsh -c`, `eval`, `xargs`, `find -exec`, `env`, `sudo`, `nohup`, `time`, `command`).
2. **Rules apply to the command word of a segment**, never to text inside arguments. Mentioning a git subcommand inside a grep pattern, a commit message or an echoed string is allowed.
3. **Fail closed on what cannot be resolved, inside a worktree only.** When the cwd is a linked worktree and a command word or a write target contains an unresolved expansion (`$var`, `${...}`, `$(...)`, backticks), deny and tell the agent to write the literal path. Outside a worktree behaviour is unchanged.
4. **Harness files are read-only for workers.** From a linked worktree, or with `agent_id` set, deny writes (Edit/Write/MultiEdit/NotebookEdit and shell write targets) to: the main checkout's `.orq/`, `state.Dir()`, `limits.DefaultPath()`, and any `.claude/settings.json` or `.claude/settings.local.json`. Add fields to `gate.Env` so tests inject these paths. Invoking the `orq` binary stays allowed.
5. **Workers do not start sub-agents.** `Agent`/`Task` from a linked worktree is denied even without `agent_id`.
6. **Write targets the parser must understand:** `cp -t DIR`, `mv -t DIR`, `install -t DIR`, `find ... -delete`, `git apply`, `patch`, `git stash pop|apply` (in a worktree with an open slice bundle these are denied: the target is unknowable), and inline interpreter code (`-c`, `-e`, `--eval`, `-p`, `-r`, stdin heredoc) that names a protected path.
7. **MCP tools.** The matcher in both hook templates gains `mcp__.*`. For an `mcp__*` tool called from a worktree, deny when any string value in `tool_input` resolves to a protected path.
8. **`TestDecisiones`**: a table-driven Go test holding every probe case with its expected decision, using injected Env (no real disk state).
9. Existing gate tests pass; `validacion/validar-mvp.sh` (no flags) passes with ORQ set to your built binary.

Keep it small and readable. Prefer one simple fail-closed rule to many patterns. Commit after item 2 passes, then again at the end.
