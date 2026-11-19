# D2a · Correct code per project and merge-time guardrails — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:executing-plans (inline, one builder). Steps use checkbox (`- [ ]`) syntax.

**Goal:** every project declares its lint, types, tests and extra guardrail commands once (`orq init` detects them); the worker runs them with `orq check`; `verificar` runs them in the sandboxed copy, the receipt carries each result, a failing one blocks the close, and the decision page shows them. Codex leaves the config defaults.

**Architecture:** the block lives in `config` (`Config.Checks`, merged like every other field). `perfil` detects it from files on disk; `cli init` writes it into `<repo>/.orq/config.json`. `slice` gets the ordered list on `Repo.Checks` (set by `app.Proyecto.Slices()` from the loaded config) and one shared runner `CorrerChecks` used both by `verificar` (inside the existing sandbox via `correrEn`) and by `orq check` (plain `sh -c` in the clone).

**Tech Stack:** Go standard library only.

**Spec:** `docs/harness-v2/plan.md` › "Route (Owner, 2026-11-18 late)" (D2) and "Approach" mechanisms 1 and 3; `docs/harness-v2/working/advice/fable-value-2026-11-18.md` §3 D2 and §4.

## Global Constraints

- Standard library only; `internal/arch_test.go` stays green (`slice` and `perfil` may import `config`: same layer 1).
- Do not touch `internal/gate` or `internal/sandbox`; checks in `verificar` run through the existing `Repo.correrEn`.
- "Skipped" never counts as passed: a skipped check is recorded as `omitido` with its reason and flagged `!!` on the page.
- Only commands that exist: never invent a script. A missing one is written as `{"omitir": "<reason>"}`.
- Codex is never used, for any role (Owner rule, 2026-11-18): defaults have no `codex`; a `codex` kind in a user file is refused naming that rule.
- Tests: temp `ORQ_STATE/ORQ_LIMITS/ORQ_WORKTREES`, temp `ORQ_CONFIG`; no real `claude`/`codex`; no network.
- Each commit: `gofmt -l harness` empty, `go vet ./...`, `go test -race ./...`, `make lint`.

## Config shape (decided here)

```json
"checks": {
  "lint":  {"cmd": "! gofmt -l . | grep ."},
  "types": {"cmd": "go vet ./..."},
  "tests": {"cmd": "go test ./..."},
  "sin-suppress": {"cmd": "! git grep -n 'nolint' -- '*.go'"}
}
```
- Keys `lint`, `types`, `tests` are required when the block exists; any other key (`^[a-z][a-z0-9-]*$`) is an extra guardrail.
- Each entry has exactly one of `cmd` or `omitir`.
- Order: lint, types, tests, then extras by name.
- No block (project never ran the new `orq init`): verification runs no checks; the receipt has none and the page says `!! sin checks`.
- `gofmt -l` exits 0 even with unformatted files, so the Go lint command is `! gofmt -l . | grep .` (fails and prints the files).

## Review Focus

1. A gofmt-only fix after a failing lint check changes only whitespace: the no-progress rule (`git diff -w`) must not block the slice → the failure signature includes the check output (Task 3 test `TestCheckArregloDeFormatoNoEsSinProgreso`).
2. A project with no `checks` block (every slice opened before D2a, the e2e suites) must still verify as before (Task 3 `TestVerificarSinChecksComoAntes`, plus the unchanged suites).
3. `orq init` run twice, or on a `.orq/config.json` the Owner already edited, must not overwrite the Owner's block or drop other keys (Task 2 `TestInitNoPisaChecksDelOwner`).
4. A `package.json` with npm's default `"test": "echo \"Error: no test specified\" && exit 1"` is not a real test script (Task 2 `TestChecksTSSinScripts`).
5. `orq check` from inside a worker clone reads the main checkout's `.orq/config.json` (the clone has no `.orq`) (Task 4 `TestCheckEnClonLeeConfigDelProyecto`).

---

### Task 1: Config — `checks` block and no Codex

**Files:** Modify `harness/internal/config/config.go`, `harness/internal/app/workers.go`, `harness/internal/app/roles.go` (only where a kind is accepted). Test: `harness/internal/config/config_test.go`.

**Interfaces — Produces:**
- `type Check struct { Nombre string \`json:"-"\`; Cmd string \`json:"cmd,omitempty"\`; Omitir string \`json:"omitir,omitempty"\` }`
- `Config.Checks map[string]Check \`json:"checks,omitempty"\``
- `func (c Config) ChecksEnOrden() []Check` (Nombre filled; lint, types, tests, then extras sorted)
- `func ValidarKind(kind string) error` — nil for `claude`; for `codex` an error containing `regla del Owner`; otherwise `usa claude`.
- `Kinds = []string{"claude"}`.

- [ ] Write failing tests:
  - `TestDefectoSinCodex`: `json.Marshal(Defecto())` does not contain `codex`/`gpt-`; `Defecto().Roles["astra"].Kind == "claude"`.
  - `TestCodexEnArchivoSeRechaza`: user file `{"roles":{"astra":{"agente":"codex","modelo":"x"}}}` → error containing `regla del Owner`; same for `{"workers":{"alta":{"codex":{"modelo":"x","esfuerzo":"high"}}}}`.
  - `TestChecksDelProyecto`: project file with lint/types/tests + extra `zz` and `aa` → `ChecksEnOrden()` names `[lint types tests aa zz]`, `Cmd` and `Omitir` kept.
  - `TestChecksInvalidos`: missing `tests` → error `checks.tests`; both `cmd` and `omitir` → error; neither → error; name `Bad Name` → error.
- [ ] Run `go test ./internal/config/` — FAIL.
- [ ] Implement; update existing tests that assert codex defaults to the new rule; `app/workers.go` and the role kind path call `config.ValidarKind`.
- [ ] `go test ./...` green; commit `config: checks block per project; Codex out of the defaults and refused`.

### Task 2: Detection and `orq init`

**Files:** Create `harness/internal/perfil/checks.go`; modify `harness/internal/cli/v03.go` (`runInit`). Tests: `harness/internal/perfil/perfil_test.go`, `harness/internal/cli/check_test.go` (new).

**Interfaces — Produces:** `func perfil.Checks(root string) map[string]config.Check`.
Rules: `go.mod` at root → lint `! gofmt -l . | grep .`, types `go vet ./...`, tests `go test ./...`. Else `package.json` at root → lint `npm run lint` if `scripts.lint`; types `npm run typecheck` if `scripts.typecheck`, else `npx --no-install tsc --noEmit` if `tsconfig.json` exists and `typescript` is a (dev)dependency; tests `npm test` if `scripts.test` and it is not npm's default stub. Anything not found → `Omitir` with the reason (e.g. `package.json no tiene script lint`). No stack → all three omitted with `stack sin comandos conocidos: escribe el comando en .orq/config.json`.

- [ ] Failing tests:
  - `TestChecksGo`: dir with `go.mod` → exactly the three Go commands above.
  - `TestChecksTS`: `package.json` scripts `lint`,`typecheck`,`test` (shell stubs) → `npm run lint`, `npm run typecheck`, `npm test`.
  - `TestChecksTSSinScripts`: scripts `{"test": "echo \"Error: no test specified\" && exit 1"}`, `tsconfig.json`, devDependency `typescript` → lint omitted (reason contains `lint`), types `npx --no-install tsc --noEmit`, tests omitted.
  - `TestInitEscribeChecks` (cli, throwaway git repo with `go.mod`): `orq init` → `.orq/config.json` loads with `config.Cargar` and has the three Go checks; stdout lists them.
  - `TestInitNoPisaChecksDelOwner`: existing `.orq/config.json` with `{"terminal":"herdr","checks":{...owner's...}}` → after `orq init` the file is byte-identical; stdout says the block was kept.
- [ ] Run — FAIL. Implement: `runInit` adds `checks` only when absent, keeping other top-level keys (`map[string]json.RawMessage`), writes with `orqdir.WriteAtomic`, prints one line per check.
- [ ] Green; commit `init: detect lint, types and tests per stack into .orq/config.json`.

### Task 3: Checks in `verificar`, in the receipt, blocking the close

**Files:** Create `harness/internal/slice/checks.go`; modify `harness/internal/slice/nucleo.go` (`Verificar`), `harness/internal/slice/slice.go` (`Recibo`, `Repo`), `harness/internal/app/proyecto.go` (`Slices`). Test: `harness/internal/slice/checks_test.go`.

**Interfaces — Produces:**
- `Repo.Checks []config.Check`
- `type ResultadoCheck struct { Nombre, Cmd string; Exit int; Resultado string /* paso|fallo|omitido */; Motivo, Salida string }` (json `nombre, cmd, exit, resultado, motivo, salida`), `Recibo.Checks []ResultadoCheck \`json:"checks,omitempty"\``
- `type Correr func(cmd string) (out string, code int, err error)`
- `func CorrerChecks(cs []config.Check, correr Correr) ([]ResultadoCheck, error)` — `Salida` = first 10 lines of output.
- `func FalloDeChecks(rs []ResultadoCheck) string` — "" when none failed; else `check <nombre> falla (exit N): <first lines>` for each failing one, joined by `; `.

Verificar: after the exam, runs `r.Checks` in one fresh `copiaDe(head)` via `correrEn` (limit `e.limite()`), stores the results in the receipt, and a failing check makes the result `fallo` with `motivo` = `FalloDeChecks`. The failure signature is `firmaDe(out + check outputs)`. `app.Proyecto.Slices()` sets `Checks: p.Cfg.ChecksEnOrden()`.

- [ ] Failing tests (toy repo from `repoDeJuguete`; checks are shell stubs inside the repo):
  - `TestCheckFallidoBloqueaElCierre`: types `sh tipos.sh` printing `error TS2322: Type 'string' is not assignable to type 'number'` and exit 2 → `Verificar` error contains `check types falla (exit 2)` and `TS2322`; state not VERIFICADO; `Cerrar(.., true, ..)` fails.
  - `TestCheckGuardiaExtraBloquea`: extra `sin-suppress` `! grep -rn eslint-disable src` with a planted `// eslint-disable` → error contains `check sin-suppress` and the grep line.
  - `TestReciboLlevaCadaCheck`: lint pass, types pass, tests `Omitir: "sin script test"`, extra pass → VERIFICADO; `Recibo.Checks` has 4 entries in order with `cmd`, `exit 0`, `paso`, and tests `omitido` with motivo; the sealed receipt (`reciboConfiable`) carries them too.
  - `TestCheckArregloDeFormatoNoEsSinProgreso`: lint `! grep -l '  $' *.txt` fails on trailing spaces; second commit removes only whitespace → verified, not BLOQUEADO.
  - `TestVerificarSinChecksComoAntes`: no checks → VERIFICADO, `Recibo.Checks` empty.
- [ ] Run — FAIL. Implement. Green (whole slice suite). Commit `verificar: run the project's checks in the box; a failing check blocks the close`.

### Task 4: `orq check` and the worker contract

**Files:** Create `harness/internal/cli/check.go`; modify `harness/internal/cli/cli.go` (dispatch, usage), `harness/internal/cli/guias/worker.md`. Test: `harness/internal/cli/check_test.go`.

**Interfaces — Consumes:** `slice.CorrerChecks`, `slice.FalloDeChecks`, `config.Cargar`, `slice.LeerMarca`.
`orq check`: dir = git toplevel of cwd; config root = clone marker's `Root` when in a clone, else dir. Runs each with `sh -c` in dir (timeout `slice.TimeoutDefecto`). Prints `NAME  paso|fallo|omitido  · cmd` per check, the tool's first lines under a failing one, then `N pasan · M fallan · K omitidos (omitido no cuenta como pasado)`. Exit 0 iff no failure and at least one check exists; no block → exit 1 `sin checks: corre orq init`.

- [ ] Failing tests:
  - `TestCheckImprimeCadaResultado`: repo with lint pass, types fail (`echo 'a.go:3: undefined: x'; exit 1`), tests omitted → exit 1; output contains `lint  paso`, `types  fallo`, `a.go:3: undefined: x`, `tests  omitido: …`, `1 pasan · 1 fallan · 1 omitidos`.
  - `TestCheckSinBloque`: exit 1, `orq init`.
  - `TestCheckEnClonLeeConfigDelProyecto`: open a slice (as `sliceVerdeCon` does), main repo `.orq/config.json` has checks, run `orq check` with cwd = the clone → uses them, runs in the clone (a check `test -f solo-en-clon` passes only there).
  - `TestGuiaWorkerPideOrqCheck`: `guias/worker.md` contains `orq check` and says to loop until it passes before REQUEST VERIFICATION.
- [ ] Run — FAIL; implement; green; commit `check: orq check runs the project's checks in the clone; worker guide loops on it`.

### Task 5: The decision page and `verificar` output show the checks

**Files:** Modify `harness/internal/cli/decidir.go`, `harness/internal/cli/proyecto.go` (verificar success lines). Test: `harness/internal/cli/decidir_test.go`.

Section `Checks`, source `recibo · checks de .orq/config.json`: `paso` → normal `lint paso · exit 0 · <cmd>`; `omitido` → alerta `tests omitido: <motivo> (no cuenta como pasado)`; `fallo` → alerta with exit and first output line; none → alerta `sin checks en el recibo: corre orq init y vuelve a verificar`. Same in `--html` (shared `seccionesDecision`).

- [ ] Failing tests:
  - `TestDecidirMuestraLosChecks`: project `.orq/config.json` with lint/types pass and tests omitted, `sliceVerdeCon` → page contains `Checks`, `lint paso · exit 0`, `types paso`, `!! tests omitido: sin script test (no cuenta como pasado)`; `--html` contains `tests omitido`.
  - `TestDecidirSinChecksAvisa`: no block → `!! sin checks en el recibo`.
  - `TestVerificarCLIRechazaConMensajeDeLaHerramienta`: types stub fails with `error TS2322` → `orq slice verificar` exit 1, stderr contains `check types falla` and `TS2322`; `orq slice cerrar --aceptado` exit ≠ 0.
- [ ] Run — FAIL; implement; green; commit `decidir: the page shows each check with its command, exit and result`.

### Task 6: Verify and report

- [ ] `make ci`, read the output: candidate revision, gofmt, vet, race tests count, e2e and aislamiento counts.
- [ ] Write `docs/harness-v2/d2a-report.md` (commits, `make ci` counts, exit test item → evidence, not done, open decisions) and commit it.
