# Identity: who may act as Owner, Atena or Orchestrator

## Problem

`ORQ_ROL` and `ORQ_WORKER_ID` are environment variables. Any process can set or clear them,
so a worker could run `ORQ_WORKER_ID= ORQ_ROL=owner orq mem add --source owner ...` and forge an
Owner decision, or change the exam, the autonomy mode or a stopper.

## Design

**One secret, outside the repo, that orq never hands to a worker.**

- `orq init` (and `orq token`) create `~/.config/orq/owner.token` (`$XDG_CONFIG_HOME/orq/owner.token`):
  32 random bytes in hex, folder 0700, file 0600. A token readable by group or others is ignored.
  It is per OS user, not per project. It is never inside the repo or a worktree.
- `ORQ_ROL` names the role being claimed. `ORQ_TOKEN` proves it. A privileged action needs both:
  the claimed role and an `ORQ_TOKEN` equal to the file (constant-time compare).
- The Owner gets the token at their own terminal: `export ORQ_TOKEN="$(orq token)"`, before starting
  Atena. `orq token` refuses inside a worker (`ORQ_WORKER_ID`, `ORQ_SHIM`, a worker role or a linked worktree).
- `orq up` / `orq relevo` pass `ORQ_TOKEN` only to the Orchestrator and Atena panes, and only when the
  caller already presented a valid token. Planner and Astra panes get `ORQ_TOKEN=""`.
- Workers launched by orq get `ORQ_TOKEN=""`, `ORQ_WORKER_ID` and `ORQ_ROL=<worker name>`, and a `PATH`
  that starts with `.orq/workers/<id>/bin`, where an `orq` shim unsets `ORQ_TOKEN`, fixes
  `ORQ_WORKER_ID` and sets `ORQ_SHIM=1`.

### Privileged actions (need the token)

| Action | Role |
|---|---|
| `mem add --source owner` | owner or atena |
| `mem add --source atena`, `msg send --from atena` | atena |
| `slice exam`, `slice scope` | owner, atena or orchestrator (plus a registered decision) |
| `autonomy guided|free` | owner or atena |
| `stop resolve`, `stop cancel` | as `stops.CanResolve` says |
| `slice close` | anyone but a worker |

Without a valid token the role is reported as `"<role> without ORQ_TOKEN"` and every check above fails.

### Layers against a worker

0. **Ancestry (CLI, structural, Linux).** Before honouring `ORQ_ROL`/`ORQ_TOKEN`, `orq` walks its parent
   chain (`/proc/<pid>/stat` → ppid, up to init) and reads each ancestor's `/proc/<pid>/environ`. That file
   is the environment the process received at `exec`; a child cannot rewrite its ancestors'. If any
   ancestor has a non-empty `ORQ_WORKER_ID`, `ORQ_SHIM=1` or a worker `ORQ_ROL` (anything outside the
   session roles), the caller is that worker: the allowlist of layer 3 applies, `orq token` refuses and every
   privileged check sees a worker, even with `ORQ_ROL=owner` and a valid (stolen) `ORQ_TOKEN`. The same
   happens when the resolved cwd is inside a worktree orq registered for a slice, zone or scout
   (`.orq/worktrees.jsonl`). This defeats `unset "${!ORQ@}"; export ORQ_ROL=owner ORQ_TOKEN=...; orq ...`,
   whatever way the binary is reached. All runners put `ORQ_WORKER_ID` in the agent's initial
   environment: herdr (`--env` on `tab create`/`pane split`), tmux (`-e` on `new-window`/`split-window`),
   process (`cmd.Env`; a real test spawns a worker that runs the exploit and is refused), and Codex/Claude
   inherit it from the pane. The `/proc` reader is injectable (`identidad.Tabla`); an unreadable
   ancestor stops the walk. **Not Linux:** there is no `/proc`, the walk is skipped and only the
   current environment, the worktree check, the shim and the gate remain.
1. **Token (CLI).** Without the secret, no privileged action happens, whatever the environment says.
2. **Gate (Claude workers, PreToolUse).** A worker is anyone with `ORQ_WORKER_ID`, a role outside
   `{owner, atena, orquestador, orchestrator, planificador, planner, astra}`, a subagent, or a cwd in a
   linked worktree. The gate reads those values from the agent's own environment, which a Bash command
   cannot change. For a worker, orq is an **allowlist**: `slice status|verify|receipt|contract|fact|hypothesis`,
   `msg send|read` (no `--from`, no closing type), `mem add|show` (no owner/atena source),
   `worker beat|done|list`, `stop raise|list`, `qa finding|show`, `learn`, `plan show|ready`, `skill`,
   `context`, `phase`, `status`, `board`, `doc`, `gate`, `version`, `help`, `handoff-note --auto`.
   Quotes and backslashes are removed as the shell does (`sl''ice` is `slice`). Denied outright:
   `$` expansions in an orq call, flags before the subcommand, `alias`, `git -c alias.`/`git config alias.`,
   copying or linking orq, any `ORQ_*=` assignment, `unset ORQ_*`, `env -u|-i`, and any mention of
   the token path (`owner.token`, `.config/orq`, `XDG_CONFIG_HOME`) or `/proc/*/environ`, globs over
   `.config` or a hidden home folder (`~/.config/*/own?r.tok*`), `${!ORQ@}`/`{ORQ`, `ORQ` named inside
   `unset`/`export` (`export $(printf "ORQ_%s=..." ...)`), and commands that leave the process tree:
   `setsid`, `nohup`, `disown`, `systemd-run`, `at`/`batch`, `screen`, `tmux new|split|run|respawn`,
   `herdr`, `orca`. Single-dash flags are normalised (`-tipo cierre` is `--tipo cierre`); a worker may only
   send `pregunta|respuesta|estado` (question|answer|status). Read/Grep/Glob of the config folder or
   `/proc/*/environ` are denied too (the hook matcher includes Read|Grep|Glob).
3. **Shim + CLI allowlist (Codex and other agents without a hook).** With `ORQ_WORKER_ID` or `ORQ_SHIM`
   set, a worker ancestor (layer 0) or a cwd in a slice worktree, `orq` itself refuses anything outside the
   same allowlist (exit 2). `msg send` also refuses `cierre|historia|plan` from any effective worker and
   signs the message as the worker.
4. **QA tree check.** `orq qa run` records HEAD and `git status --porcelain` (tracked and untracked,
   not ignored; content hashed) before and after the layers. Any change outside the allowed reports
   (`out.tap` plus `qa.reports`) makes the receipt `invalid`, and close requires `pass`.

## Honest limits

- **Escaping the process tree.** Layer 0 trusts that the caller descends from the worker. A same-user
  process that gets started by something outside the tree (`systemd-run --user`, `at`, a cron entry,
  `setsid` + double fork reparented to init or to a subreaper without the marker, a tmux/herdr server
  that was not started by the worker, an editor or language server already running as the Owner) has no
  marked ancestor. The gate denies the obvious launchers, but only for Claude workers and only for
  commands it can parse; a script file can still call them. Reparenting to a marked subreaper is fine.
- **Ancestor environ is read only when readable.** `/proc/<pid>/environ` of the same user is readable;
  with `hidepid` or a different user it is not, and the walk stops there (fail open). Linux only.
- **Ignored files in the QA check.** Files under `.gitignore` are not in the tree snapshot; a tester
  could change ignored build output or caches without invalidating the receipt.
- **Same OS user.** A worker runs as the Owner's OS user, so it *can* read `~/.config/orq/owner.token`
  with a tool the gate does not parse (a script file, an editor's read tool, a compiled program), or
  read another process's environment in `/proc/<pid>/environ`. The gate blocks the obvious paths; it is not
  a sandbox. The real fix is OS-level isolation: run workers as a separate user (or in a container) that
  cannot read the Owner's config folder or processes.
- **Codex without a hook.** The shim only guards calls that go through `PATH`. A Codex worker that calls the
  real binary by absolute path after unsetting `ORQ_WORKER_ID` is still caught by layer 0 on Linux;
  elsewhere only layer 1 (the token) stops it.
- **The Owner's own panes.** Anything started from a shell that exported `ORQ_TOKEN` inherits it, unless
  orq launched it (orq clears it for workers, planner and Astra).
- **Rotation.** Delete the file and run `orq token` to rotate; re-export `ORQ_TOKEN` in the Owner's panes.
