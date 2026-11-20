# orq2 pilot fixes — plan (Real State, Python/uv)

Planner: Fable. Builder: Opus. Paths are relative to `harness/`. Order = value for the pilot.
Invariants for every item: `orq` 1.0.0 output stays byte-identical (`internal/testdata/orq-init.golden`,
`cli/version_test.go`); no box-less mode; the box never sees `$HOME`, network or the main checkout's sources.

## Evidence gathered (run by the planner in a real box, `orq2 sandbox exec`, clean `git archive HEAD` copy)
Mounts as `correrEn` does: copy RW, main `.venv` RO + symlink, `~/.local/bin` RO.
- `uv run --offline python -c "import dante_os"` → **fails**: uv tries to rebuild the editable project, `hatchling` not in cache.
- `uv run --offline --no-sync ruff --version` → **ok** (`ruff 0.15.20`).
- `uv run --offline --no-sync python -c "import dante_os"` → **ModuleNotFoundError**: `.venv/.../_editable_impl_dante_os.pth`
  points at the main checkout's `src`, which the box does not mount (good: it must not test the main checkout).
- `PYTHONPATH=src uv run --offline --no-sync python -c "import dante_os"` → **ok**, imports from the copy.
- UNVERIFIED: full `pytest` / `mypy src` runs in the box with these commands (only import and `--version` were run).

So the env question is already half solved: `slice/caja.go:99-116` (`depsEn`) binds `<root>/.venv` RO at its real
path plus a symlink in the copy. What breaks is the *command*: the hand-written checks in Real State's
`.orq/config.json` (`uv run --offline …`, no `--no-sync`, no `PYTHONPATH`) cannot pass in the box.

## P1 (high) — `init` proposes no Python checks
Root cause: `internal/perfil/checks.go:21-33` knows only `go.mod` and `package.json`; anything else falls to
"stack sin comandos conocidos" (`:31`).
Smallest fix: one branch between Go and Node, when `pyproject.toml` AND `uv.lock` exist. No TOML parser (stdlib):
read `pyproject.toml` as text. A tool counts as present if the text contains `[tool.<name>` or `"<name>` (dep line).
- prefix `P` = `PYTHONPATH=src ` only if `src/` is a directory; dirs `D` = those of `src tests` that exist, else `.`.
- lint (ruff): `P uv run --offline --no-sync ruff check D && P uv run --offline --no-sync ruff format --check D`
- types (mypy): `P uv run --offline --no-sync mypy <src or .>`
- tests (pytest): `P uv run --offline --no-sync pytest -q -p no:cacheprovider`
- a tool not found → `Omitir: "pyproject.toml no nombra <tool>"`. Never invent.
`sandbox.palabras` (`sandbox/toolchain.go:38-50`) already skips `X=y` and `-flags`; `uv` stays the first tool.
orq 1.0.0: the branch changes what `orq init` prints on a Python repo. Gate it: `app` passes "is orq2"
(`identidad.Nombre()`) into `perfil.Checks`; `orq` keeps today's omission. UNVERIFIED: whether `arch_test.go`
lets `perfil` import `identidad` — pass a bool from `app` to avoid the question.
Tests first (`perfil/checks_test.go`, `t.TempDir()`): (1) pyproject+uv.lock+src+tests with ruff/mypy/pytest →
exact three commands; (2) no `src/` → no `PYTHONPATH`, dirs `.`; (3) pyproject without mypy → types omitted with
motive; (4) pyproject without `uv.lock` → today's omission; (5) orq (not orq2) → today's omission; golden unchanged.
Scope: **uv only now.** requirements.txt/poetry get the existing omission with a clearer motive
("Python sin uv.lock: escribe el comando en .orq/config.json"). Add them when a real project needs them.
Risk: text matching can false-positive on a comment naming ruff; the check then fails loudly in the box (fail-closed).

## P2 (medium) — `check --caja` does not prove Python deps resolve in the box
Root cause: `cli/check.go:53-109` only runs `sandbox.Sondear` (`toolchain.go:228-268`: `command -v` + `--version`)
with the tool's RO mounts — no copy, no `.venv`, no project command. `uv` answers `--version`, so it says "corre".
Decision: **bind the main checkout's `.venv` read-only (already done by `depsEn`), never sync in the box.** Same
trust model as `node_modules`: the main checkout is trusted, the copy is not. A cache-complete offline sync is
rejected: needs `~/.cache/uv` in the box, writes a venv per run, and already failed (grpcio, mypy, hatchling).
Consequence to document: a slice that adds a dependency needs `uv sync` in the main checkout first; until then the
check fails with ImportError (fail-closed, correct).
Smallest fix, in `check --caja` only, for a check whose command contains `uv run`:
1. static: command lacks `--no-sync` → "no corre: uv sincroniza en la caja y no hay red; usa `uv run --offline --no-sync`".
2. static: `<root>/.venv` is not a directory → "no corre: falta .venv; corre `uv sync` en el checkout principal".
3. real probe: `copiaDe(HEAD)` + `correrEn(dir, probe, 60s)` where probe = the check's own prefix up to and
   including the tool word, plus ` --version` (e.g. `PYTHONPATH=src uv run --offline --no-sync pytest --version`).
   Exit ≠ 0 → "no corre" with the first output line. This proves: uv visible, `.venv` bound, tool installed in it.
   Expose it as one exported method on `slice.Repo` (it owns `copiaDe`/`correrEn`); `app.ChecksCaja` calls it.
Not proven (say so in the output line): that every project import resolves — only the real check proves that.
Tests first: `slice` test with a fake `Caja` (`Repo.Caja`) asserting the probe argv and the RO list contains
`.venv`; `cli` test for the two static messages with a temp repo, temp HOME and a fake `uv` script on a temp PATH.
A bwrap-backed test follows the existing skip pattern in `sandbox/sandbox_test.go` (UNVERIFIED: its skip helper name).
Must not change: `depsEn`, `entornoExamen`, `Spec` — no new env vars or mounts in the box.
Known limit, not in scope: `dashboard/` (pnpm, `node_modules` in a subfolder) — `depsEn` only binds root
`node_modules`. Record in `backlog-next.md`, do not fix now.
Pilot action (not code): rewrite Real State's `.orq/config.json` with the P1 commands; re-run `orq2 check --caja`.

## P3 (low) — `watch` > 2 min
Root cause: `watch/watch.go:22-30` calls `sysinfo.DirSizeBytes` (`sysinfo/sysinfo.go:93-116`) on each
`scratch.paths` entry; `templates/limits.json:6` lists `~/worktrees` (133 entries today, many `node_modules`),
walked serially with one `Info()` (stat) per file.
Smallest fix: keep the number identical, make the walk parallel — one bounded pool (8) over the top-level children
of the path, the hardlink `seen` map behind a mutex. No change to quota semantics, limits.json or output.
UNVERIFIED: that stat latency (not directory reads) is the cost; builder runs `time orq2 watch` before and after
and records both. If still > 20 s, stop and report — do not exclude `node_modules` or cache sizes without the Owner
(that changes what the quota means).
Tests first: `sysinfo` test — tree with nested dirs and a hardlink across two top-level children → same total as
today's serial result, run with `-race`. Existing `watch_test.go` unchanged.

## P5 (cosmetic) — `orq2 version` prints `1.0.0`
Root cause: `cli/cli.go:26` single `const Version`, used by `cli/version.go:19` and the usage header (`cli.go:28`).
Smallest fix: `const VersionOrq2 = "2.0.0-pilot"`; `versionLine` and the usage header pick by `identidad.Nombre()`.
Leave `slice.Version` (`cli.go:120`) alone — UNVERIFIED whether it enters seals/receipts (`slice/sellos_test.go`);
changing it could invalidate open orq2 slices.
Tests first: with `t.Setenv("ORQ_NOMBRE","orq2")`, `Run version` → `2.0.0-pilot revision=unknown modified=unknown`;
the two existing tests in `cli/version_test.go` stay untouched and green.

## P6 (HIGH, blocks the pilot — do first) — orq2 counts foreign agents as its workers
Root cause: `app/capacidad.go:22-26` counts with `sysinfo.Workers(procs, names, gate.IsWorker)`
(`sysinfo/sysinfo.go:56-72`), and `gate.IsWorker` (`gate/gate.go:247-264`) is true for **any** linked worktree
(`.git` is a file — the orq 1.0.0 rule) on the machine. `cli/cli.go:261-264` feeds that count to the gate
(`gate/gate.go:152-158`). Seen: 7 of 4 (VentIA under `~/.herdr/worktrees`, airiflow, the planner in `cthulhu-build`).
`IsWorker` is also the gate's security question ("is this session a worker, so restrict it") and must stay
fail-closed and unchanged.
Smallest fix: a second, narrower predicate used only for counting under orq2 — `gate.EsWorkerPropio(dir)`: true only
if some `.git` directory upward holds `orqdir.MarcaClon` (a clone orq2 itself created). No "linked worktree"
branch, no fail-closed "relative dir" branch. `MedirReal` picks it when `identidad.Nombre()==Orq2`; `orq` keeps
`gate.IsWorker`. Preferred over an env marker: no `/proc/<pid>/environ` reads, survives relaunches, nothing to forge
that matters (a worker hiding itself only raises the count ceiling; RAM/CPU still bound it).
RAM/CPU: `recursos.Calcular` reads `sysinfo.LeerMaquina` (whole machine) — untouched, so foreign sessions still
reduce capacity through free RAM/load, just not through the head count.
UNVERIFIED: (a) that orq 1.0.0 never writes `MarcaClon` (if it does, also require the marker/clone to belong to
orq2's state — check `slice/clones.go` and `orqdir`); (b) that every orq2 worker cwd is inside a marked clone
(`slice/equipo.go:116-130` `crearClon`), including a slice without a team (`equipo.go:197`).
Tests first (`gate` + `app`, `t.TempDir()`): procs with cwd in (1) a linked worktree, (2) a plain repo, (3) a marked
clone, (4) a repo nested in a marked clone → orq2 counts 2 (3 and 4); with `ORQ_NOMBRE` unset the count is today's
(1, 3, 4). Gate test: 5 foreign worktree agents + 0 own, `max_workers=4` → orq2 allows; orq denies as today.
Existing `IsWorker` tests untouched.
Risk: under-counting if an orq2 worker runs outside a marked clone → (b) must be closed before merge.
After the fix: restore `max_workers` in `~/.config/orq2/limits.json` (workaround set it to 12) — Owner's call.

## Builder brief (Opus)
0. P6 first (it blocks the pilot): add the narrow counting predicate for orq2 only; close its two UNVERIFIED points.
1. Work in `~/worktrees/cthulhu-build`, branch `v2/build`. Go 1.25, stdlib only. One commit per item, test red first.
2. Order: P6 → P1 → P2 → P5 → P3. Stop after each: `go test ./... -race` green, `orq-init.golden` and `version_test` untouched.
3. P1: add the uv branch in `perfil/checks.go` gated to orq2 (bool from `app`), commands exactly as above.
4. P2: add the two static refusals and the copy-based probe to `check --caja`; do not touch `depsEn`/`Spec`/env.
5. P5: `VersionOrq2` for the `version` command and usage header only.
6. P3: measure, parallelise `DirSizeBytes`, measure again; report both timings; no semantic change.
7. Tests: temp HOME (`t.Setenv("HOME", t.TempDir())`), no network, no real `claude`, fake tools as scripts on a temp PATH.
8. Final proof on Real State (read-only project, do not commit there): `orq2 init` proposes the three commands;
   `orq2 check --caja` says "corre" for all three; then run each check once through the box and paste the exit codes.
   This closes the UNVERIFIED line about full pytest/mypy in the box. If pytest fails there for env reasons
   (the 3 inbound tests), report it — do not weaken the command.
9. Report in `docs/harness-v2/pilot-fixes-report.md`: per item red→green evidence, timings, anything still unverified.
