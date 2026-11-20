ISSUES
Review of 24f2438..75bb0be (P1-P8), fresh Opus. Repros: Go tests in a `git archive 75bb0be` copy (scratchpad rv/harness, files `rv_test.go`).
Priority 1 (gate/box) HOLDS: no finding weakens the gate or the box. Findings are P3 semantics, P1 layouts, P2 diagnostics.

Guarantees checked, no bug found
- P6 `gate.EsWorkerPropio` (gate.go:272): marker Lstat is `!IsNotExist`, so a permission error still counts (good). It only sets the
  head count; `IsWorker` (security question) is unchanged. Clones from an orq built from this tree also count, so it can over-count, never under-count.
  Every orq2 worker cwd is a `crearClon` (report closes (b)). `Workers` already dropped an empty cwd before the change.
- P7 gate.go:156: nil max_workers and nil Capacidad -> deny. Measuring errors -> deny (CapacidadPara returns err). `config` rejects
  ram/cpu_por_agente <= 0, so `Calcular` never hits Inf/NaN, and PorCPU always limits SinTope. orq still needs max_workers (limits.go:83).
- P2 `SondarUv` runs only via `correrEn` -> `enCaja`. With no box it returns "sin caja no corre nada" (caja.go:70) and never falls back to the host.
  `principal()` trusts the clone marker's Root, but at worst that is another repo's HEAD copy plus its .venv RO, still inside the box.
- orq 1.0.0: `uso()`/`version()` return `usage`/`Version` unchanged; `Checks(root)` = `ChecksPara(root,false)`; P8 replace is orq2 only.

F1 MEDIUM - P3 changes the scratch number when a scratch path is a symlink (plan: "keep the number identical")
  sysinfo.go:124 `os.ReadDir(root)` follows a symlinked root. The old `filepath.WalkDir` Lstats the root and does not descend.
  Repro `TestRV_SymlinkRoot`: link -> dir with a 1000 B file: serial=0, parallel=1000 (FAIL). If `~/worktrees` or any limits.json path is a
  symlink, quota/HALT now measure the target. That is the safe direction, but it is a silent semantic change.
  Fix: `if fi, err := os.Lstat(root); err != nil || !fi.IsDir() { return serial WalkDir }` before ReadDir.

F2 MEDIUM - P1: under orq2 a pyproject.toml now hides the Node checks
  checks.go:33: `python && existe(pyproject.toml)` returns before package.json. With pyproject.toml but no uv.lock, all three checks are omitted.
  Repro `TestRV_NodeLostUnderOrq2`: pyproject + package.json{lint,test}: orq -> `npm run lint`/`npm test`; orq2 -> 3x "Python sin uv.lock".
  Fix: take the Python branch only when uv.lock exists. Otherwise fall through to package.json, and use the uv.lock motive only when no package.json exists.

F3 MEDIUM - P1 flat layout: lint skips the package code
  checks.go:96-104: with no `src/` and an existing `tests/`, D="tests", so the package (`mypkg/`) is never linted or format-checked.
  Repro `TestRV_FlatLayout`: `ruff check tests && ... ruff format --check tests` (mypy correctly uses `.`).
  Follows the plan's wording, but it is wrong on a real flat layout. Fix: D="src tests" only when src exists; otherwise ".".

F4 LOW - P1 misses single-quoted TOML deps
  checks.go:90 matches only `[tool.x` or `"x`. `dev = ['ruff','mypy','pytest']` is valid TOML and gives 3x "no nombra" (repro `TestRV_SingleQuote`).
  Fail-safe (omitted with a reason). Fix: also match `'x`.

F5 MEDIUM - P2 probe gives a false "corre" for uv flags that take a value
  caja.go:251-253 skips only `-`-prefixed words, so the flag's value becomes the "tool". Repro `TestRV_Sonda`:
  `uv run --with foo pytest` -> probe `uv run --with foo --version`; `uv run --directory sub --no-sync pytest` -> `uv run --directory sub --version`.
  uv takes `--version` for itself and exits 0, so `check --caja` says "corre" with uv's version. `HerramientasUv` then drops "foo"/"sub" instead of pytest.
  This is diagnostic only: the real check still runs in the box and fails closed. Fix: know uv's value flags
  (--with, --directory, --project, --python, --group, --extra, --package, --env-file, -p), or stop at `--`.

F6 LOW - P2 checks the whole command for --no-sync but probes only the first segment
  app/check.go:147 `strings.Contains(cc.Cmd,"--no-sync")`. In `uv run --offline --no-sync ruff check . && uv run ruff format --check .`
  the second segment syncs (no network, so it fails in the exam), yet `check --caja` says "corre".
  `HerramientasUv` also removes a host `pytest` used in a later plain segment (`uv run --no-sync pytest && pytest`), hiding its host lookup.
  Fix: apply the static check and the probe to each `uv run` segment, and remove only the tools of the uv segments.

F7 LOW - P7: max_workers <= -1 now reads as "sin tope"
  limits.go:103 / recursos.go:40: an explicit `max_workers: -1` equals `SinTope`. Repro `TestRV_MaxWorkersMinus1`: Tope=15, Limita=CPU,
  Resumen "max_workers sin tope". In 1.0.0 status showed capacity 0 (limits by max_workers), so the orq status line changes for that input.
  The gate still denies (gate.go:159 `n >= -1`), so the gate stays safe, but status and gate disagree.
  Fix: reject max_workers < 0 in `validate`, or keep SinTope out of band (a bool, not -1).

Suite on the copy: `go test ./...` green except my F1 repro and 2 `internal/candidato_test.go` tests that call `git rev-parse` (the archive copy is not a repo, so exit 128; this comes from the copy, not the code).
Not verified: bwrap-backed run of the probe (no box run in this review); P3 cold-cache timing; Real State proof (taken from the report).
