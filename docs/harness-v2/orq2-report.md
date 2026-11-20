# orq2 · v2 side by side with orq 1.0.0

Plan line: "Real tests of D1–D3 use v2 installed side by side as `~/bin/orq2`".

## What decides the identity

One package, `harness/internal/identidad` (layer 0), answers "which orq am I":

- **orq2** when `ORQ_NOMBRE=orq2`, or the base name of `os.Args[0]` or of `os.Executable()` is `orq2`
  (`os.Args[0]` first, so a symlink `~/bin/orq2 -> dist/orq` still counts as orq2).
- **orq** in every other case. `ORQ_NOMBRE` with any other value means orq.

For orq every function returns what the code returned before (same hook string, same paths, text
untouched, writers unwrapped), so a binary named orq behaves byte for byte as before.

| | orq (unchanged) | orq2 |
|---|---|---|
| hook command in `.claude/settings.json` | `$HOME/bin/orq gate\|contexto\|relevo --auto` | absolute path of the running binary (`ORQ_NOMBRE=orq2 <path>` if the file is not named orq2) |
| texts (CLI stdout/stderr, guides, repo skills, launch prompts, channel lines, `relevo-ROL.md`) | as written | `orq <sub>` → `orq2 <sub>` |
| user config, limits, instintos | `~/.config/orq/` | `~/.config/orq2/` |
| state (HALT, snapshots, notes) | `~/.local/state/orq` | `~/.local/state/orq2` |
| receipts' runner | `orq 1.0.0` | `orq2 1.0.0` |

`ORQ_CONFIG`, `ORQ_LIMITS`, `ORQ_STATE` still override the defaults for both.

The text rewrite (`identidad.Texto`) changes `orq` only as a command word: a whole word followed by a
space and a lowercase subcommand. Paths (`~/.config/orq/`, `.orq/`, `bin/orq`), skill names
(`orq-planner`), session labels and message tags (`orq · P`, `[orq · historia …]`) and error
prefixes (`orq:`) stay. It is applied at the chokepoints: `cli.Run` wraps stdout and stderr; init
rewrites the repo skills it installs; `app.Abrir` wraps the terminal driver so `Avisar` lines and
`Arrancar` args (launch prompts) are rewritten; `orq relevo` rewrites the hand-off file.

The gate treats a command named `orq2` as `orq` for every rule (`gate/shell.go` `nombre`), so the
orchestrator-only `slice` subcommands and the expansion rules hold for both names.

## init on a project orq 1.0.0 manages

`orq2 init` first checks, writing nothing, whether 1.0.0 manages the project:

- `.claude/settings.json` or `.claude/settings.local.json` contains `orq gate`, `orq contexto` or
  `orq relevo` (orq2's own hooks never match: they say `orq2 …`);
- `.orq/slices/*.json` or the v0.4 `.orq/slice.json` holds a slice of schema 0 (opened by 1.0.0),
  read with `slice.AbiertosOrq1`, which does not migrate.

If so it exits 1 with the reasons and the two ways out (close/unhook with 1.0.0, or `--convivir`),
before `abrirProyecto`, so the project is untouched (the test compares every file under `.orq` and
`.claude` before and after).

`orq2 init --convivir` writes nothing under `.claude` (no settings, no skills), does not overwrite an
existing `.orq/proyecto.json`, and prints the orq2 hooks block plus the skills it did not install.
It still adds the detected `checks` to `.orq/config.json` only when that block is missing (the same
rule as always: an existing block is the Owner's). Name orq ignores `--convivir` (unchanged).

## make install2

`make build2` builds `dist/orq2`; `make install2` copies it to `$(BIN)/orq2` through a temp file and
rename. It writes no other file: not `~/bin/orq`, not `~/.config/orq*`, not `~/.local/state/orq*`
(orq2 creates its own dirs on first use; there is no default `limits.json` copy, so orq2 runs with
the built-in limits until one is put at `~/.config/orq2/limits.json`). Not run here.

## Tests

- `internal/orq2_test.go` builds the real binary twice (named `orq` and `orq2`) and runs it with a
  temp HOME, `PATH=/usr/bin:/bin`, no `ORQ_STATE/ORQ_CONFIG/ORQ_LIMITS/ORQ_NOMBRE`:
  - `TestOrqSinCambios`: `orq init` output plus every file it writes under `.orq` and `.claude`
    equals `testdata/orq-init.golden`, taken from the pre-orq2 binary (commit 2d326a5) on the same
    fixture.
  - `TestOrq2Identidad`: hooks carry orq2's absolute path; init output, the installed orchestrator
    skill and `orq2 skill worker` say `orq2`; `halt --set` lands in `~/.local/state/orq2`; `status`
    shows `~/.config/orq2/limits.json`; nothing is created under `~/.config/orq` or
    `~/.local/state/orq`; `ORQ_NOMBRE=orq2` on a file named orq writes `ORQ_NOMBRE=orq2 <path>` hooks.
  - `TestOrq2NoPisaOrq1`: refuses on a fixture with 1.0.0 hooks and on one with an open 1.0.0 slice,
    changing nothing; `--convivir` succeeds and leaves `.claude` identical.
- `internal/identidad/identidad_test.go`: what the rewrite changes and keeps; defaults for both names.

## Residual risks

- Both versions share the project's `.orq/` (memory, channel, slices). The refusal covers open 1.0.0
  slices; with `--convivir`, running both on one repo at the same time is the operator's choice.
- An agent that types `orq …` by hand despite the texts runs 1.0.0. Hooks are absolute, so the gate,
  context and relevo always run orq2.
- The orq2 sandbox hides `~/.config` and `~/.local/state` as a whole, so 1.0.0's dirs stay hidden from
  workers; orq2's gate protects orq2's state paths, not 1.0.0's beyond that.

## Review fixes (orq2-review.md)

1. **Own skill names.** Under orq2, `identidad.Skills` renames `orq-{atena,orchestrator,planner,ruta,worker}` to `orq2-*`: `orq2 init` writes `.claude/skills/orq2-*` (name, directory and text), the role launch prompts (`skillDe`) load those names, and `orq2 skill ROL` guides point at them. A personal `orq-*` 1.0.0 skill can no longer shadow them. A binary named `orq` is untouched (golden test green).
2. **Bare `orq` refused.** Under orq2 the gate (`gate.orqPelado`) denies a command whose word is `orq` (any path, any subcommand) with a message saying to use `orq2 <subcommand>`; `orq2` and the `ORQ_NOMBRE=orq2 <path>/orq` form stay allowed. Under `orq` nothing changes.
3. **Own hooks not read as 1.0.0.** `hookOrq1` ignores a hit that follows `ORQ_NOMBRE=orq2` on its line; `orq2 init` twice on its own project succeeds (both hook forms tested).
4. **Texts.** The refusal quotes the real matched command (`$HOME/bin/orq gate`); `identidad.Texto` rewrites `~/.config/orq/` and `~/.local/state/orq/` to the orq2 dirs, so the usage line shows `~/.config/orq2/config.json`.

Tests (`internal/orq2_test.go`, temp HOME, real binaries) failed before each fix and pass after. Residual: the orq2 gate only sees commands typed through Bash; a script that calls `orq` internally is not inspected.
