# D2a report · Correct code per project and merge-time guardrails

Branch `v2/build`, worktree `~/worktrees/cthulhu-build`. Plan: `docs/harness-v2/d2a-plan.md`. Spec: `plan.md` › "Route (Owner, 2026-11-18 late)" D2 and "Approach" mechanisms 1 and 3; `working/advice/fable-value-2026-11-18.md` §3 D2 and §4. Playwright and the feature map are D2b and are not here.

What I did not do:
- Install, push, or write to `~/bin/orq`, `~/.config/orq` or `~/.local/state/orq`.
- Call a real `claude` or `codex`. Every test used temp `ORQ_*` dirs and a temp `ORQ_CONFIG`.
- Touch `internal/gate` or `internal/sandbox`. The checks run through the existing `Repo.correrEn`.
- Add dependencies.

## Commits

| Task | Commit | Subject |
|---|---|---|
| plan | `74eb76a` | docs: D2a plan — checks per project and merge-time guardrails |
| 1 | `c1bb8f6` | config: checks block per project; Codex out of the defaults and refused |
| 2 | `e9fdaec` | init: detect lint, types and tests per stack into .orq/config.json |
| 3 | `6c501df` | verificar: run the project's checks in the box; a failing check blocks the close |
| 4 | `37dfa73` | check: orq check runs the project's checks in the clone; worker guide loops on it |
| 5 | `36fec2f` | decidir: the page shows each check with its command, exit and result |
| self-review fix | `8e9e93f` | verificar: a check the box cannot run (exit 127) is labelled infrastructure |
| report | this commit | docs: D2a report |

## `make ci` on `8e9e93f` (clean tree, exit 0)

- Candidate: `/tmp/orq-ci.n72kXp/bin/orq · 1.0.0 revision=8e9e93fb72d08a5cd16e95c41de04c82b8cf6d8f modified=false`.
- `make lint` (staticcheck 2025.1.1): clean.
- `gofmt -l harness`: nothing listed.
- `go vet ./...`: clean.
- `go test -race ./...`: every package ok.
  - 239 top-level tests pass, 0 fail, 2 skipped. 23 subtests pass.
  - The 2 skips are the same as before: `TestPrepararDeps` and herdr's `TestLiveTabSplitClose`.
- `e2e-v05`: **86 pass, 0 fail**.
- `aislamiento`: **14 pass, 0 fail, 0 pending**.

## The block

It lives in `<repo>/.orq/config.json` and merges like every other config field.

```json
"checks": {
  "lint":  {"cmd": "! gofmt -l . | grep ."},
  "types": {"cmd": "go vet ./..."},
  "tests": {"cmd": "go test ./..."},
  "sin-suppress": {"cmd": "! git grep -n nolint -- '*.go'"}
}
```

- When the block exists, `lint`, `types` and `tests` are required.
- Any other name (`^[a-z][a-z0-9-]*$`) is an extra guardrail.
- Each entry has exactly one of `cmd` or `omitir` (the reason it is skipped).
- Checks run in this order: lint, types, tests, then the extras by name.
- `gofmt -l` exits 0 even when it lists files. So the Go lint command is `! gofmt -l . | grep .`, which fails and prints the files.

## What the Owner can do now, item by item

**1 · `orq init` writes the `checks` block (`perfil.Checks`).**
- Go: when `go.mod` is at the root, it writes `gofmt`, `go vet ./...` and `go test ./...`.
- `package.json`: it writes only the scripts that exist:
  - `npm run lint`;
  - `npm run typecheck`, or else `npx --no-install tsc --noEmit` when `tsconfig.json` exists and `typescript` is a dependency;
  - `npm test`, unless the script is npm's default `no test specified` stub.
- Anything missing is written as `omitir` with its reason. It never invents a script.
- An existing block is the Owner's and is left byte-identical. Other keys in the file are kept.
- Evidence:
  - perfil: `TestChecksGo`, `TestChecksTS` (the scripts are shell stubs), `TestChecksTSSinScripts`, `TestChecksSinStack`.
  - cli: `TestInitEscribeChecks`, `TestInitNoPisaChecksDelOwner`, `TestInitSumaChecksSinPerderOtrasClaves`.

**2 · `orq check` runs the checks in the current clone.**
- It prints one line per check: `NAME  paso`, `NAME  fallo (exit N)` with the tool's first 10 lines, or `NAME  omitido: reason`.
- It ends with `N pasan · M fallan · K omitidos (omitido no cuenta como pasado)`.
- It exits 0 only when no check fails. With no block it exits 1 and says `sin checks … corre orq init`.
- From a worker clone, it reads the main checkout's `.orq/config.json` (through the clone marker) and runs the commands in the clone.
- The worker guide (`guias/worker.md`) now says: run `orq check`, fix and rerun until it passes, before REQUEST VERIFICATION.
- Evidence: `TestCheckImprimeCadaResultado`, `TestCheckSinBloque`, `TestCheckEnClonLeeConfigDelProyecto`, `TestGuiaWorkerPideOrqCheck`.

**3 · `verificar` runs the same checks in the box, and the receipt carries them.**
- After the exam, the checks run on a fresh clean copy of the same commit, inside the existing sandbox.
- `Recibo.Checks` stores each check's `nombre`, `cmd`, `exit`, `resultado` (`paso`, `fallo` or `omitido`), `motivo` and `salida`. The receipt sealed in git carries the same list.
- A failing check fails the verification with `check NAME falla (exit N): <tool's lines>`, so the slice never reaches VERIFICADO and `cerrar --aceptado` refuses.
- A skipped check does not block, and it never counts as passed: the page flags it `!!`.
- The failure signature includes the checks' output. Without that, a whitespace-only gofmt fix would have been called "no progress" and blocked the slice.
- The page has a new "Checks" section, source `recibo · checks de .orq/config.json`, in text and in `--html`. A receipt without checks gets `!! sin checks en el recibo`. `orq slice verificar` prints `checks: lint paso · types paso · tests omitido`.
- Evidence:
  - slice: `TestCheckFallidoBloqueaElCierre` (a planted TS2322 type error; the tool's message is in the error; the close fails), `TestCheckGuardiaExtraBloquea` (a planted `eslint-disable` caught by an extra guardrail), `TestReciboLlevaCadaCheck` (4 checks in the state receipt and the sealed one), `TestCheckArregloDeFormatoNoEsSinProgreso`, `TestVerificarSinChecksComoAntes`, `TestCheckSinHerramientaEsInfraestructura`.
  - app: `TestSlicesLlevaLosChecksDelProyecto`.
  - cli: `TestDecidirMuestraLosChecks`, `TestDecidirSinChecksAvisa`, `TestVerificarCLIRechazaConMensajeDeLaHerramienta`, `TestVerificarCLIImprimeLosChecks`.
- `TestVerificarCLIRechazaConMensajeDeLaHerramienta` passed on its first run, because Task 3 had already built that behaviour. The slice tests it rests on were seen red.

**4 · Codex is out of the config defaults.**
- Astra now defaults to `claude/opus/high`. There are no `codex` worker entries, and `Kinds` is `claude`.
- A `codex` kind in a role, or a `codex` key under `workers`, is refused with: `Codex no se usa en ningún rol ni worker (regla del Owner, 2026-11-18); usa claude`.
- `orq slice lanzar --agente codex` and `orq sesion abrir … --kind codex` are refused the same way. Without this, they would have launched codex with an empty model.
- Evidence: `TestDefectoSinCodex` (the marshalled defaults contain neither `codex` nor `gpt-`), `TestCodexEnArchivoSeRechaza`, `TestCodexNoSeLanza` (seen red by reverting the app change), `TestChecksDelProyecto`, `TestChecksInvalidos`.
- I updated the four existing config tests that pinned the Codex defaults.

All of the exit tests above run inside `make ci`, on throwaway repos, with no network.

## Rulings (Owner may change)

1. **The checks come from the config as it is when you verify; they are not frozen when the slice opens.** `.orq/` is control space that a worker cannot write, so editing the checks stays the Owner's act. The cost: if the Owner changes the checks mid-slice, the verification uses the new ones.
2. **Every check runs, even when the exam failed.** The receipt then shows everything at once. When both the exam and a check fail, the reason names both.
3. **Exit 127 still blocks the close, labelled `[infraestructura: …sandbox.ro…]`.** The Direction says an infrastructure failure is reported as such, never as a product defect.
4. **I kept Codex's argv code in `agente` and the cross-kind branch of `ParaRol`.** Nothing reaches them now. D5 (second provider) either reuses them or removes them.
5. **Only npm.** The TS rules write `npm run …`/`npm test`; I added no pnpm/yarn detection by lockfile. The Owner edits the block if a project uses another tool.

## Not done, and why

- **The odontoflow receipts (the exit test from the advice) are the Owner's to run**, with `~/bin/orq2` on the real repos.
- **Real toolchains inside the box.** On this machine `go` (`~/.local/bin`) and `node`/`npm` (`~/.nvm`) live under `$HOME`, and the box cannot see them. So the close tests use shell stubs, not a real `go vet` or `tsc`.
  - On a real project, the Owner must add those toolchains to `sandbox.ro` in `limits.json`. Until then, each such check fails, labelled infrastructure.
  - A Go project with module dependencies also needs its module cache visible read-only: the box has no network.
- **Checks are not re-run when the merge is re-validated in `cerrar`.** Only `verificar` runs them; the row names `verificar`.
- **Mutation on the worker's diff and "no new suppressions"** (old step 5) are not built in. The Owner can declare a suppression ban as an extra guardrail, as the tests do with `eslint-disable`.

## Open decisions for the Owner

1. Should the checks be frozen into the slice contract at `abrir`, like the exam, so a mid-slice edit cannot change what a slice is judged by?
2. Should a check that cannot run (exit 127) block the close, as it does now, or count as `omitido` with the infrastructure reason?
3. Who sets `sandbox.ro` for the toolchains: `orq init` from what it detects, or the Owner by hand?

## Suggestions outside this row

- Once D5 names the second provider, drop the unreachable Codex launch code in `agente` and `app.argsRol`.
- `orq doctor` could check that every check command is visible inside the box, so the Owner learns about it before the first close.

## Review fixes (round 1 of 2, `reviews/d2a-review.md`)

Each fix has a test seen failing first. `internal/gate` is untouched. `make ci` exit 0 on `b41367a` (unit suites, 86 attack checks, 14 forge scenarios).

| # | Finding | Fix | Commit | Test |
|---|---|---|---|---|
| 1 | Go lint passed when gofmt never ran | `perfil` writes `out=$(gofmt -l . 2>&1); c=$?; printf …; if [ $c -ne 0 ]; then exit $c; fi; test -z "$out"`. A missing gofmt exits 127, which reads as infrastructure. An unparseable file exits 2. An unformatted file exits 1 | `2e6db9e` | `perfil.TestLintGoNoPasaSinGofmt`: clean, missing gofmt, `func {`, unformatted |
| 2 | Worker could rewrite what a check runs | The receipt gets `definiciones_checks` and the note "check definitions changed: <files>". Files that count: `package.json` (`scripts` only, so dependency-only changes do not count), `tsconfig*.json`, `eslint.config.*`, `.eslintrc*`, `biome.json*`, `.golangci.*`, `pyproject.toml` (`[tool.*]` only), `setup.cfg`, `ruff.toml`, `.prettierrc*`, plus `checks.definiciones.archivos`. The page flags them `!!` with the diff (up to 80 lines). `cerrar --aceptado` refuses without `--acepto-checks` | `48b358c` | `slice.TestDefinicionDeCheckCambiadaPideAlOwner` (review repro), `TestDefinicionesSoloDependenciasNoCuenta`, `cli.TestDecidirDefinicionesDeChecks` |
| 3 | Deleted tests not caught | The receipt gets `tests_borrados` and the note "tests deleted: …". It covers every test file deleted or renamed between red and the verified commit. The page flags each one `!!` under Archivos | `48b358c` | `slice.TestTestsBorradosEnElRecibo`, `cli.TestDecidirTestsBorrados` |
| 4 | Suppressions not caught by default | A built-in check `supresiones` is on whenever a `checks` block exists, and `orq init` writes one for every stack. It runs host-side on `git diff -U0` from red to the verified commit and looks only at added lines. It fails with `file:line: text`. `checks.supresiones.patrones` extends the list and `checks.supresiones.omitir` turns it off. `orq check` also runs it in the worker clone, including uncommitted and untracked files | `48b358c` | `slice.TestSupresionesGuardiaPorDefecto`, `TestSupresionesApagadaOExtendida`, `cli.TestCheckEnClonLeeConfigDelProyecto`, `config.TestSupresionesYDefiniciones` |
| 5 | Toolchains under `$HOME` invisible in the box | `sandbox.Herramientas` resolves each tool a check names on the host `PATH`. When the tool is under `$HOME`, verificar binds read-only both the `PATH` directory and the symlink target's directory. If either is a `bin/`, it binds that directory's parent (the toolchain root) instead. It never binds `$HOME` itself, `~/.claude*`, `~/.config`, `~/.ssh`, `~/.gnupg`, `~/.local/state` or the orq state dir, and never a directory that contains one of them, so `~/.local/bin` is bound but `~/.local` is not. `orq check --caja` reports per check whether the box can run it and what it binds, and exits 1 if one cannot run | `b41367a` | `sandbox.TestHerramientasUnderHome` (stub toolchain under a temp HOME), `slice.TestCheckVeLaToolchainDelHome` (real box, failed at exit 127 before), `cli.TestCheckCajaInforma` |
| 6 | Page labelled exit 127 `fallo` | `!! <check> infraestructura · exit 127 · <cmd> · la caja no ve el comando` | `37591e5` | `cli.TestDecidirExit127EsInfraestructura` |

Notes:
- On this machine, `orq check --caja` on a fresh Go module shows lint, types and tests running in the box, binding `~/.local/bin` and `~/.local/lib/go-1.25.8`. No one has yet run a real `go test` verification end to end through `orq slice verificar`. The integration test uses a stub tool in the real box.
- `checks.definiciones` and `checks.supresiones` are reserved names in the `checks` block. `patrones` is accepted only on `supresiones` and `archivos` only on `definiciones`.
- Not done: the `--acepto-checks` acceptance is not written to the diario as its own event.

## Review fixes, round 2 (`reviews/d2a-review-2.md`)

Each fix has a test seen failing first. Every commit passed `gofmt -l harness` (empty), `go vet ./...`, `go test -race ./...` and `make ci` (exit 0).

| # | Defect | Fix | Commit | Test (failed before) |
|---|---|---|---|---|
| 1 | The box skipped the middle symlink (`~/.local/lib/go -> go-1.25.8`), so `go` was not found (exit 127) | `sandbox.saltos` follows the PATH hit component by component, the way the kernel does, and records every symlink on the way, for files and for directories. The box binds each link's directory, the real binary's directory, and the toolchain root when that directory is a `bin/` (GOROOT). The deny list is the same, and it applies to every hop. Top-level system links (`/bin -> usr/bin`) are skipped because the box already recreates them | `debc544` | `sandbox.TestHerramientasSiguenCadaEnlace`: a temp HOME with this machine's layout, run in a real box (`command -v go && go version`). It was exit 127 before. A hop into `~/.config` stays out of the box, with the reason |
| 2 | `orq check --caja` printed a static "corre en la caja" | For each tool a check names, `sandbox.Sondear` runs `command -v <tool>` and then `--version` (or `version`) in a real box with the same binding as verificar. It reports `<tool> → <path in the box> · <first line>` and the probe's exit when it is not 0. Exit 126/127, or no path, is reported as "no corre en la caja: la sonda de X sale N" and exits 1 | `0198680` | `cli.TestCheckCajaInforma`: a tool behind the two-hop chain, and `rota`, whose interpreter the box never sees. The old code said `rota` "corre" |
| 3 | The exam got no toolchain binding | `correrEn` binds the toolchain of every tool its command names, so the exam, `--sano` and the checks share one binding | `6adbd1e` | `slice.TestExamenVeSuToolchain`: an exam that is a tool behind the two-hop chain. It was "fakego: not found" before |
| 4 | Exit 127 used up a repair and led to BLOQUEADO "sin progreso" | When the exam or a check exits 127, verificar still fails and still writes its receipt. It does not change `Fallos`, `HeadFallo` or the failure signature. The error says "infraestructura (…, exit 127): no cuenta como reparación" | `7379b9f` | `slice.TestInfraestructuraNoGastaReparacion` (check and exam): 4 runs on the same head stay IMPLEMENTANDO with 0 failures and leave 4 receipts |

### Real end-to-end run (item 5)

This ran in a throwaway Go module under the session scratchpad. `ORQ_STATE`, `ORQ_LIMITS` (missing file, so no `sandbox.ro`), `ORQ_WORKTREES` and `ORQ_CONFIG` were temp. The binary was built into the scratchpad; `~/bin/orq` was not touched, and nothing under `~/.config/orq` or `~/.local/state/orq` changed. Steps: `orq init`, then a red exam `accept/sum_test.go` (`Sum(2, 3) = 0, want 5`), then `orq slice abrir 01 … --cmd "go test ./accept/"` (red, in the box), then a green commit in the worker clone, then `orq slice verificar` → **VERIFICADO** (exit 0).

`orq check --caja` before the slice:

```
lint  corre en la caja: gofmt → /home/miguel/.local/bin/gofmt · flag provided but not defined: -version (exit 2) · monta /home/miguel/.local/bin, /home/miguel/.local/lib
types  corre en la caja: go → /home/miguel/.local/bin/go · go version go1.25.8 linux/amd64 · monta /home/miguel/.local/bin, /home/miguel/.local/lib
tests  corre en la caja: go → /home/miguel/.local/bin/go · go version go1.25.8 linux/amd64 · monta /home/miguel/.local/bin, /home/miguel/.local/lib
```

Receipt excerpt (`.orq/recibos.jsonl`):

```json
{"attempt": "S-01/a1", "head": "de88bbe9b90f768eb6a61d3d24c0b0a0fda119a8", "cmd": "go test ./accept/",
 "exit": 0, "resultado": "paso", "salida": "ok  \texample.com/m/accept\t0.002s",
 "checks": [
  {"nombre": "lint", "cmd": "out=$(gofmt -l . 2>&1); c=$?; printf '%s\\n' \"$out\"; if [ $c -ne 0 ]; then exit $c; fi; test -z \"$out\"", "exit": 0, "resultado": "paso"},
  {"nombre": "types", "cmd": "go vet ./...", "exit": 0, "resultado": "paso"},
  {"nombre": "tests", "cmd": "go test ./...", "exit": 0, "resultado": "paso", "salida": "?   \texample.com/m\t[no test files]\nok  \texample.com/m/accept\t0.003s"},
  {"nombre": "supresiones", "cmd": "orq: marcas de supresión en las líneas que suma el slice", "exit": 0, "resultado": "paso"}]}
```

Notes:
- `gofmt` has no version flag. The probe counts any exit other than 126/127 as "it ran", and shows that exit.
- The box now binds `~/.local/lib` whole, because the `go` link lives there. Under the deny list this is allowed, but the box sees every toolchain in `~/.local/lib`, not only Go.
- Still open: `abrir` accepts a red caused by exit 127, a "red" that does not come from an assertion. Item 4 means it cannot burn repairs later, but `abrir` could refuse it up front.

## Review fixes, round 3 (`reviews/d2a-review-3.md`)

1. `orq slice abrir` refuses a red whose exam exits 126/127 ("falta una herramienta"), also with `--sin-conteo`. Test: `slice.TestAbrirRechazaRojoDeHerramientaFaltante`. The round-2 exit-127 test now uses an exam that is red at open and 127 later.
2. `verificar` refuses when the worker's head equals `SHARojo` ("no ha commiteado nada"), with no receipt and no change of state. Test: `slice.TestVerificarSinTrabajoSeRechaza`.
3. `orq init --help` / `-h` prints usage and changes nothing. Test: `cli.TestInitHelpNoCambiaNada` (no `.orq/` or `.claude/` created).

## Close (orchestrator, 2026-11-19)
Integrated as 7be03d7 after 3 review rounds (`reviews/d2a-review*.md`; the Owner lifted the cost cap). `make ci` on 7be03d7 with the candidate (`modified=false`): exit 0, e2e 86/0, aislamiento 14/0. Orchestrator check: the review-3 repro (`abrir … --cmd "checktool …" --sin-conteo`, tool missing) is refused with "el examen sale con 127: falta una herramienta… no es un rojo"; no slice is created. Review 3 also confirmed a real end-to-end slice reaches VERIFICADO with no manual sandbox.ro and real gofmt/go vet/go test at exit 0, and that the deny list holds across symlink hops.
Known limits: the box mounts all of `~/.local/lib` read-only (not only the toolchain). A deleted project test is a warning on the page, not a refusal.
