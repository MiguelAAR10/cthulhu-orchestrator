# orq2 update

`orq2 update` replaces `~/bin/orq2` with a build of the `harness-v2` tip of this repo. It is the
Owner's command, run in the Owner's own terminal. It does not exist under `orq` (1.0.0).

| Command | What it does |
|---|---|
| `orq2 update` | installs the tip if its exact SHA has a gate record, after showing it and asking for the short SHA |
| `orq2 update --ver` | read-only: installed revision, tip, whether the tip has a record |
| `orq2 update --volver` | restores `~/.local/share/orq2-rollback/orq2-anterior`; the binary it replaces is kept, so a second `--volver` undoes the first |
| `orq2 update --registrar SHA --revisor QUIEN --veredicto sin-high --ci-exit 0 --ci-log ARCHIVO` | the integrator records a gate that passed |

## Gate record

A git note on `refs/notes/orq2-gate` of this repo, attached to one exact commit:

```json
{
  "esquema": 1,
  "sha": "<40 hex>",
  "revision": {"veredicto": "sin-high", "revisor": "<who reviewed>"},
  "ci": {"comando": "make ci", "exit": 0, "log": "<absolute path of the make ci log>"},
  "fecha": "<RFC 3339>"
}
```

- It is a note, not a file in the tree, because committing a record would create a new tip
  that has no record. A note belongs to one commit, so a record for a parent never counts for
  its child.
- History: `git log -p refs/notes/orq2-gate`. To correct a record:
  `git notes --ref=orq2-gate remove SHA`, then register again.
- The CI log must exist and contain the full SHA, both when it is registered and when it is
  installed. The `make ci` candidate line prints it. Keep the log somewhere durable, not in a
  temp dir that gets cleaned.
- `--registrar` only records a pass, and only from the Owner or a session with
  `ORQ_ROL=orquestador`. Never a worker, never another role, never inside the box.

## What update refuses

- An ungated tip, a record that names another SHA, or a record whose CI log is missing or does
  not name the SHA.
- Any slice that is verifying or closing in any project it can see. It counts as busy:
  - a slice in INTEGRANDO or CERRANDO;
  - a held `.orq/slice-*.lock` or `integracion.lock`;
  - a running `orq*` process doing `slice verificar|cerrar|juntar` or `sesion cerrar`;
  - a slice state file it cannot read.

  It finds projects through `ORQ_ROOT`, the current folder, the worker clone markers under
  `~/worktrees` (and under `ORQ_WORKTREES`, if set), and the folder of each running `orq*`
  process. There is no global project registry. A project that none of these reaches, for
  example a crash left a slice in CERRANDO with no clone and no process left, is not seen.
- Any `ORQ_UPDATE_*` variable. The real binary has no overrides. Repo, branch, `~/bin`, the
  rollback dir and `go` are pinned, and the home comes from the account database, not `$HOME`.
  git and go run with a fixed PATH. The build gets a fixed environment, with no GOROOT,
  GOFLAGS or go env file from the caller.
- A caller that is not the Owner at a terminal: `ORQ_ROL` set, a worker folder, inside the box,
  or stdin that is not a tty.

## Rollback

- Before installing, the current binary becomes `orq2-anterior` only if it is a clean build of
  a gated SHA, or if there is no rollback yet.
- Otherwise it is kept beside the rollback as `orq2-sin-puerta-<rev>`. A half-failed update or a
  manual install therefore never replaces a good rollback.
- Nothing else in the folder is touched, for example the integrator's `orq2-24f2438`.
- Every install and restore is a temp file in the same folder followed by a rename.

## Threat model (limits)

Every Owner check (tty, no `ORQ_ROL`, not a worker folder, the typed short SHA) runs as the same
Unix user as the agents. A determined agent can fake each one: a pty from `script`, a cleared
env, a `cd`. These checks are speed bumps against mistakes, not a security boundary.

The PreToolUse gate deny is a speed bump too. It denies a command line that may run
`orq2 update` to a subagent (`agent_id`), a worker (worktree or clone) or a role panel
(`ORQ_ROL`). It checks the raw text and the words as bash sees them, so quotes, variables and a
name built at run time count. A script file (`sh x.sh`) or an interpreter can still hide it.

The real protections are these:

- the binary takes no overrides: repo, branch, `~/bin`, the rollback dir and go are pinned, and
  any `ORQ_UPDATE_*` is refused;
- the gate record must exist for the exact SHA, with a CI log that names it, and the Owner reads
  it and types the short SHA before anything is installed. Anyone who can write the repo's refs
  can write a note with plain git, so that reading is part of the check.

Pinned does not mean tamper-proof. When go is `~/.local/bin/go`, it is as writable by the same
user as `~/bin` itself.

The gate deny exists only under identity orq2, because orq stays byte-identical to 1.0.0.
Projects hooked with orq (`$HOME/bin/orq gate`) are not covered: a worker there is stopped only
by the speed bumps above.

Ungated copies (`orq2-sin-puerta-*`) are capped at the newest 2.
