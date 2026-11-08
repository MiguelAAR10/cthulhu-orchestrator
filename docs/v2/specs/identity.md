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
| `mem add --source owner` | owner or atena (never the default: without `--source`, a worker signs `worker`, a session role signs itself, owner and atena must say it) |
| `mem add --source filtro` | owner or atena |
| `mem add --source atena`, `msg send --from atena` | atena |
| `slice exam`, `slice scope` | owner, atena or orchestrator (plus a registered decision) |
| `autonomy guided|free` | owner or atena |
| `stop resolve`, `stop cancel` | as `stops.CanResolve` says |
| `slice close` | anyone but a worker |

Without a valid token the role is reported as `"<role> without a valid ORQ_TOKEN (...)"` and every check
above fails. `orq token` (once the token exists) and `orq session register` need the same trusted session.

### Fail-closed session check (the token is not enough)

Since the token can be stolen by a same-user process, every privileged action (the table above,
`orq token` after first creation, `orq session register`, handing the token to Orchestrator/Atena panes)
also requires that the **calling process be trusted** (`identidad.Confiable`, Linux `/proc`):

1. no ancestor carries a worker marker (layer 0 below), **and**
2. the process or one of its ancestors is a **registered session anchor**, matched by pid **and**
   process start time (`/proc/<pid>/stat` field 22), so a recycled pid never matches, **or**
3. the process or one of its ancestors has a **user terminal** as controlling TTY (`tty_nr` of a
   pseudo-terminal `/dev/pts/*`, majors 136-143, or a virtual console `/dev/ttyN`, major 4). System
   consoles (`hvc`, `ttyS`; WSL's init has one) do not count.

Otherwise the action is refused (*"no registered privileged session among this process's ancestors and no
user terminal"*). A worker child that double-forks, calls `setsid`, clears its environment and presents a
stolen token is reparented to init or a subreaper with no terminal and no anchor, so it is refused. The real
test (`TestHuerfanoSetsidConTokenRobadoRechazado`) launches a worker through the process runner whose
Python does exactly that and checks that `mem add --source owner`, `autonomy`, `token` and
`session register` all fail and nothing is written.

**Anchors** live in `~/.config/orq/sessions/<pid>-<start>.json` (0700 folder, next to the token, out of the
repo and the worktrees). This deviates from the first proposal (`.orq/sessions/`) on purpose: inside the
project a worker could plant an anchor for its own orphan. **Each anchor is signed**: the file carries
`mac = HMAC-SHA256(owner token, "pid|start|role")`, and `Anclas()` ignores any anchor whose MAC is missing
or wrong (fail closed; `TestAnclaPlantadaSinHMAC`). Writing a JSON file into `sessions/` is therefore not
enough: the forger also needs the token, and with the token rotated every old anchor stops counting.
`Anclar` refuses when there is no token. For workers the gate also denies `Read/Grep/Glob/LS` **and**
`Write/Edit/MultiEdit/NotebookEdit` on the orq config dir (`$XDG_CONFIG_HOME/orq`, `~/.config/orq`,
`sessions/` included). The comparison uses the **real path** (symlinks resolved up to the deepest existing
parent, dangling links followed), so `lnk -> ~/.config/orq` inside a worktree opens nothing; Bash arguments
that resolve there (`cat lnk/*`, `cd lnk`, `tar cf - lnk`, globs expanded against the disk) are denied too, as
is any Bash command naming that dir. `Grep` with `path` on `/`, the home folder or an ancestor of the config
dir is denied. Searching for the token by pattern is denied: `Glob` (pattern or path), `Grep --glob` and
`find -name|-iname|-path|-regex`, `rg -g`, `fd`, `locate` matching `*tok*` or `owner.*`, or any wildcard segment
that matches `owner.token` (`*.token`, `own?r.tok*`, `**/*t?k*`, braces expanded); from `/` or home one literal
is enough (`**/o*`). `find / -name 'owner.*'` and `**/*.tok*` with path `/` are the test cases. The worktree's
own absolute prefix is ignored, so a repo called `stock` still works; anything else with `tok` in the pattern
or path is denied even when innocent (`**/tokenizer.go`): read it by exact path. Dead anchors (pid gone or start time changed) are pruned on every registration. Anchors are created by:

- **first token creation** (`orq init` or `orq token` when no token exists): whoever creates the token is
  the Owner by definition; the parent process is anchored with role `owner`;
- **`orq session register [--pid N] [--role owner|atena|orchestrator]`**, run by a trusted session with the
  token; the default pid is the calling shell. It refuses workers, worktrees, and a target pid that is (or
  descends from) a worker. Use it from the Owner's terminal to anchor a headless process (CI runner,
  headless orchestrator);
- **`orq up` / `orq relevo`**, when the caller is already trusted: they anchor the calling shell.

The validation scripts have no TTY in CI: they run `orq token >/dev/null` first in a fresh
`XDG_CONFIG_HOME` (bootstrap anchors the script's shell `$$`), then `orq session register --pid $$`.
Both scripts pass when launched with `setsid -f ... </dev/null` (no terminal anywhere in their ancestry).
Unit tests anchor the test process in `TestMain`.

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
   `msg send|read` (no `--from`, no closing type), `mem add|show` (source `worker` or `scout` only; without `--source` it defaults from the effective role),
   `worker beat|done|list`, `stop raise|list`, `qa finding|show`, `learn`, `plan show|ready`, `skill`,
   `context`, `phase`, `status`, `board`, `doc`, `gate`, `version`, `help`, `handoff-note --auto`.
   Quotes and backslashes are removed as the shell does (`sl''ice` is `slice`). Denied outright:
   `$` expansions in an orq call, flags before the subcommand, `alias`, `git -c alias.`/`git config alias.`,
   copying or linking orq, any `ORQ_*=` assignment, `unset ORQ_*`, `env -u|-i`, and any mention of
   the token path (`owner.token`, `.config/orq`, `XDG_CONFIG_HOME`) or `/proc/*/environ`, globs over
   `.config` or a hidden home folder (`~/.config/*/own?r.tok*`), `${!ORQ@}`/`{ORQ`, `ORQ` named inside
   `unset`/`export` (`export $(printf "ORQ_%s=..." ...)`), and commands that leave the process tree:
   `setsid`, `nohup`, `disown`, `systemd-run`, `at`/`batch`, `screen`, `tmux new|split|run|respawn`,
   `herdr`, `orca`, and commands that fabricate a terminal (`script`, `unbuffer`, `expect`, `socat`).
   Inline interpreter code (`python*|perl|node|ruby|php|deno|bun|lua -c/-e/--eval`, `python -`, heredocs)
   is denied when it mentions `setsid`, `fork`, `daemon`, `start_new_session`, `detached`, `pty`/`openpty`,
   `nohup`/`disown`, or runs `orq` through `os.system`/`subprocess`/`exec*`/`spawn*`/`system`/backticks
   (string tricks like `'or' 'q'` evade it; the session check is the real control). Single-dash flags are normalised (`-tipo cierre` is `--tipo cierre`); a worker may only
   send `pregunta|respuesta|estado` (question|answer|status). Read/Grep/Glob of the config folder or
   `/proc/*/environ` are denied too (the hook matcher includes Read|Grep|Glob|LS; projects initialised
   earlier must add `LS` to their `.claude/settings.json`). Recursive `rm` (`-r`, `-R`, `-rf`, `-fr`,
   `--recursive`) is denied on `/`, `~`, `$HOME`, the worktree root itself and anything outside it (`..`
   included, `cd` followed), and fails closed on what cannot be checked (`$VAR`, `$(...)`, braces, `xargs`).
   `find <inside> -exec rm -r {}` and cleanup inside the worktree pass.
3. **Shim + CLI allowlist (Codex and other agents without a hook).** With `ORQ_WORKER_ID` or `ORQ_SHIM`
   set, a worker ancestor (layer 0) or a cwd in a slice worktree, `orq` itself refuses anything outside the
   same allowlist (exit 2). `msg send` also refuses `cierre|historia|plan` from any effective worker and
   signs the message as the worker.
4. **QA isolation.** `orq qa run` runs the layers in a throwaway `git worktree add --detach` at the worker's
   HEAD and watches the main checkout, the worker worktree and `.orq/config.json` (see `docs/v2/QA.md`).

## Honest limits

- **The gate's rm and path checks read the command text.** `find / -delete`, `python -c "shutil.rmtree(...)"`
  and a path assembled at run time (`"lnk/"+"owner.to"+"ken"`, a script file) are not seen.
- **Escaping the process tree, after the session check.** An orphan (double fork + `setsid`, `at`,
  `systemd-run --user`, cron) loses its worker ancestor *and* any terminal, so it is now refused. What is
  still accepted is a process that ends up under a **user terminal or an anchor it did not come from**:
  - a worker that **creates a new pseudo-terminal** and runs orq inside it (`pty.fork()` from a script
    file, `script`, `expect`): the new session's TTY is a pts, so rule 3 passes. The gate blocks the obvious
    commands for Claude workers, not a script file or a compiled program. Rule 3 cannot be dropped without
    breaking the Owner's plain terminal: orq cannot tell a pts the Owner opened from one a worker opened;
  - injecting into a process that already runs under the Owner's terminal or an anchor: a tmux/herdr
    server started by the Owner (`send-keys` to an Owner pane), an editor or language server, `ptrace` of
    an Owner process (same user, if `ptrace_scope` allows it);
  - a forged anchor now needs the token to sign it, so it reduces to the residual below: a process of the
    same OS user that reads the token with a tool the gate does not parse can sign anchors too.

  Remaining residual, in short: **same OS user** (token and `sessions/` are readable/writable by any
  process of that user that the gate does not see), **a new pty** (rule 3), and **ptrace/injection** into an
  Owner process. **Non-Linux** keeps only the token (see below).
- **Bootstrap.** If the token file is missing, whoever creates it is trusted and anchored. An orphan that
  deletes the token and recreates it is anchored too. Deleting the token is the Owner's signal to rotate;
  check `~/.config/orq/sessions` after rotating.
- **Not Linux.** Without `/proc` (macOS, Windows) there is no ancestry, no start time and no TTY walk:
  `Confiable` returns ok and only the token, the environment, the worktree check, the shim and the gate
  remain. `orq session register` refuses there. orq builds for Windows and macOS (`GOOS=windows|darwin
  go build ./...`); on Windows locks are O_EXCL sibling files (30 s stale timeout) and killing an agent
  kills only its root process.
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
