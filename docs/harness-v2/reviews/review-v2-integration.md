# Review: `git diff 75bb0be..harness-v2 -- harness/` (HEAD 309addf)

Fresh Opus, read-only. Scope: F1-F7, P9, P11, P12, agent-first steps 1 and 4, the two merge resolutions in cb65b9e.
Targeted tests run (fresh `mktemp -d` env, ORQ_SIN_DOC=1, -count=1): limits, recursos, sysinfo, slice (tarjeta/sonda),
perfil (checks), gate/app (AvisoRAM), cli (TestTabla*, TestGuias*, TestTarjetaTopes, TestGuiaTarjeta*, TestCheckCajaUv*,
TestDoctor*). All green. No `make ci`, no full `go test ./...`.

Verdict: **1 High** (H1). Gate and box are not weakened by anything in this range.

## High

### H1. P9 tells the Planner to write `## Zonas` as plain Spanish, which breaks the decision page's scope check
- `harness/internal/cli/tarjeta_guia.go:15`: "`## Zonas`: one line per zone saying what changes for the user. Task ids, **file**,
  function ... names stay in the plan."
- `## Zonas` is not prose to the code. `slice/tarjeta.go:99-105` validates every line as a repo path, and
  `slice/pagina.go:100-105` compares every changed file against `Tarjeta.Zonas` with `dentro(z, f)`. Any changed file
  outside the bundle that is not under a zone becomes `FueraDeTarjeta`. `cli/tarjeta.go:158-160` then prints
  "fuera de las zonas de la tarjeta: <file>" as an alert on the Owner's decision page.
- Failure scenario: the Planner follows the orq2 skill and writes `## Zonas` / `- El carrito muestra el total con cuotas`.
  The parser accepts it, because it is not absolute, not `..` and not the root. The Owner approves. At decision time
  every changed source file shows as outside the card's zones. The only machine-checked link between the approved card
  and the diff is lost: the Owner sees alerts on every file, or learns to ignore the alert. A line that starts with `/`
  ("/checkout cambia ...") is refused as an absolute path, so the guide also produces refusals.
- The merge kept this text, and `cli/tarjeta_guia_test.go:47` asserts "what changes for the user".
- Fix: keep `## Zonas` as repo paths (folders or files) in the P9 text, for example "one repo path per line (folder or
  file) that this slice touches; the Owner reads the meaning in Historia". If Zonas should carry Owner-facing text, put
  it in another section, or give Zonas a `ruta — explicación` form that the parser splits. Update the test assertion.

## Medium

### M1. P9's literal card sizes contradict `slice.TopesTarjeta()`, and the merge locked them in with a test (verdict §3 broken)
- `cli/tarjeta_guia.go:11` says "Max 25 lines". `:12` says "Historia: who, what they will see and do, step by step. Plus
  one line `Valor: ...`". `:24` (orchestrator) says "≤25 lines".
- The parser caps `## Historia` at 3 lines, every other section at 5, every line at 200 characters, and the card at 400
  words (`slice/tarjeta.go:30-35`).
- The rendered orq2 planner skill is `orq2Guia(skillOrq2(...))`. It carries both "Max 25 lines" (P9 block before
  `## Astra`) and, at the end, "Card caps ...: `## Historia` ≤3 lines, every other section ≤5 lines ...". The
  orchestrator guide `guias/orquestador.md:9` still says "story ≤3 lines", while the orq2 orchestrator skill says
  "≤25 lines". So the Orchestrator gets two different descriptions.
- Failure scenario: the Planner writes a step-by-step Historia (3 steps) plus the required `Valor:` line, which is 4
  lines. It stays within "25 lines", and `orq2 slice tarjeta` refuses it ("la sección Historia tiene 4 líneas; el tope
  es 3"). Preguntas (questions with recommendations plus the human-gate checklist) passes 5 lines the same way. This
  costs a refusal round trip on every slice, but fails closed.
- Integration verdict §3 says "No literal number in any guide ... Where texts disagree, the constants win." The merge
  resolution kept the literal 25. `tarjeta_guia_test.go:40` and `:62` now *require* "Max 25 lines" and "≤25 lines", so
  they guard the contradiction.
- Fix: drop "Max 25 lines" and "≤25 lines". In P9, say "Historia: who and what they see, within the Historia cap,
  `Valor:` counts as one of its lines". Point to `orq2 slice tarjeta --topes`. Change the two test assertions to require
  `slice.TopesTarjeta()` and to forbid `25 lines`.

### M2. Step 4 changes orq's (1.0.0) first error for some invalid cards: the byte-identical requirement is broken
- `slice/tarjeta.go:63,66,75` still `return` at once for structural errors: unknown section, duplicate section, text
  outside a section. Cap violations are now deferred (`excede`).
- In 1.0.0, a line over 200 characters returned at once, before any later structural error. Card `## Historia\n<201
  chars>\n## Otra\n...`: 1.0.0 printed "en Historia, una línea tiene 201 caracteres; el tope es 200". HEAD under
  identity `orq` prints "sección desconocida \"Otra\" ...".
- No golden covers it, so CI stays green.
- The same early returns also drop every cap violation already collected under orq2. That makes "every violation in one
  pass" untrue when a structural error comes after a cap error.
- Fix: under `!identidad.EsOrq2()`, return the character error at once, as before (`if !EsOrq2() { return ... }` inside
  `excede` for the scan phase). Or append the structural errors to `topes` and return `topes[0]` under orq, which keeps
  the order. Under orq2, add structural errors to `topes` too.

### M3. F5 fix adds a panic in `SondaUvDe` when a value flag is the last word
- `slice/caja.go:283-291`. For `uv run --no-sync -p`, the code moves `j` past the flag and again past its (missing)
  value, so `j == len(f)+1`. Then `j == len(f)` is false and `f[:j+1]` panics. Repro on a scratch copy of the function:
  "slice bounds out of range [:6] with capacity 4".
- Reached from `orq2 check --caja` (`app/check.go:144,163`, `HerramientasUv`, `SondaUvDe`) and `SondarUv`
  (`caja.go:315`), with a typo in `.orq/config.json` such as `"uv run --offline --no-sync --with"`. The CLI crashes with
  a Go trace instead of a diagnosis. It is not on the gate or exam path.
- Fix: `if j >= len(f) { return "" }`. Add the case to `TestSondaUvSaltaValorDeFlag`.

## Low

- L1. Step 1 strict test does not render like `skill.go`. `cli/comandos_test.go:135` checks
  `orq2Guia(d.Name(), raw)`, but installs use `orq2Guia(d.Name(), skillOrq2(d.Name(), raw))` (`skill.go:28`). P9's
  inserted text is never scanned. Today it names no command, so nothing fails yet. Fix: call `skillOrq2` in the test.
- L2. P12 gate warning is probably invisible to the agent. `cli/cli.go:308-309` writes `d.Aviso` to stderr with exit 0.
  A Claude Code PreToolUse hook that exits 0 does not show stderr to the model. `status` and `watch` do show it, so this
  is informational only. If the Orchestrator must see it, emit it as hook JSON (`systemMessage`/additional context) or
  accept that only status/watch carry it.
- L3. `aviso_ram_gb` is now a known field for orq too (`limits/limits.go:43`). A `limits.json` with that key, which 1.0.0
  refused under `DisallowUnknownFields`, now loads under `orq`. Valid files behave the same. Also, `aviso_ram_gb` ≤ 0 or
  NaN is not validated (harmless: it only turns the warning off). Fix if strictness matters: refuse it unless
  `EsOrq2()`, and require > 0.
- L4. `orq2 slice tarjeta --topes` needs a project (`runSlice` calls `abrirProyecto` first, `proyecto.go:425`). It fails
  outside a repo, though the caps are static. Cosmetic.
- L5. F6 `HerramientasUv` keeps a uv tool on the host lookup when any plain segment contains the same word, even as an
  argument (`... && echo ruff done`). Diagnostic only, fail-safe direction.

## Checks with no bug found

- **F1** `sysinfo.go:124-133`: Lstat before ReadDir. A symlink, a file or a missing path goes to the serial WalkDir.
  `err != nil` short-circuits before `fi.IsDir()`, so there is no nil deref. The test compares with `serialSize(link)`
  and fails without the fix (parallel=1000, serial=0).
- **F2** `perfil/checks.go:34`: the uv branch is taken only with `uv.lock`, or with no `package.json`. The test asserts
  orq2 equals orq and gives the npm commands. It fails without the fix (3x "sin uv.lock").
- **F3** flat layout lints `.`. With `src/`, it lints `src` or `src tests` as before. The test fails without the fix (old
  output `tests`).
- **F4** single-quote match. The test fails without it (3x omitted).
- **F5** value flags are skipped and `--` stops the scan. The test cases fail on old code. Only the M3 edge case is new.
- **F6** each `uv run` segment needs `--no-sync` and has its own probe. The first failing probe decides. The fallback to
  the whole command is kept when no segment has a tool. `TestCheckCajaUvCadaTramo` fails on old code (the old
  `Contains(cmd)` passed).
- **F7 + P12** `limits.go:99-100,116`: orq2 refuses `max_workers < 0`. `SinTope = math.MinInt`, so it cannot collide
  with an explicit value. Checked against `recursos.Calcular` and `gate.go:159-162`:
  - orq2 explicit `max_workers: 12`: the gate denies at n ≥ 12, and capacity is min(12, RAM, CPU, carga).
  - orq2 without the key: `TopeWorkers()==SinTope`, the head cap is `MaxInt`, RAM/CPU/carga decide, and the gate requires
    `env.Capacidad` (set in `runGate`).
  - orq (`ORQ_NOMBRE` unset) without the key: "falta max_workers", as in 1.0.0.
  - orq `-1`: Tope 0, limited by max_workers, and the gate denies (n ≥ -1). Status and gate agree again.
  - `TestMenosUnoNoEsSinTope` and `TestMaxWorkersNegativo` fail without the fix.
- **P12** under orq: `AvisoRAM` returns "" (identity check), `env.AvisoRAM` stays nil, and `doctor --ram` is wired only
  under orq2, so `orq doctor --ram` runs plain `doctor` as before. `doctor --ram` only reads /proc and stops nothing.
  The warning never denies (`TestAvisoRAMNoNiega`).
- **P11** overlays are applied only when `EsOrq2()`. The orq texts are byte-identical (asserted). `--en-sitio` is gone
  from the orq2 renders. The caps sentence comes from `slice.TopesTarjeta()`, which states the constants exactly
  (`TestTopesTarjetaDiceLasConstantes`).
- **Step 1** the `comandos` table matches the `Run`, `runSlice` and other switch cases through the AST
  (`TestTablaIgualAlDespacho`). The guide test passes. The L1 gap remains.
- **Step 4** orq2 returns `errors.Join` of the cap violations, and `slice tarjeta --topes` prints the constants. M2 has
  the two remaining gaps.
- **Merge, `skill.go:28`** composition `Skills(Texto(orq2Guia(n, skillOrq2(n, raw))))`. The order is right. Both overlays
  match orq-text substrings before `Texto`/`Skills` rename `orq` to `orq2`. Their targets do not overlap (planner: line
  12 replace, P9 insert before `## Astra`; orchestrator: lines 25 and 26). The P11 rules are appended last, so they end
  up after P9's block. Reversing the order would give the same result.
- **Merge, `tarjeta_guia_test.go`** the new assertion (`tarjetaOrq` absent) is correct. The kept "≤25 lines" and "Max 25
  lines" assertions keep the contradiction (M1), and "what changes for the user" keeps H1.
- **Goldens**: no golden, testdata or `version_test.go` file changed in `75bb0be..harness-v2`. Only
  `templates/limits-orq2.json` was added (orq's `templates/limits.json` is untouched).

## Per-item table

| Item | Result |
|---|---|
| F1 symlinked root | OK (test fails without fix) |
| F2 pyproject without uv.lock | OK |
| F3 flat layout | OK |
| F4 single quotes | OK |
| F5 uv value flags | Issue M3: panic when a value flag is the last word; otherwise OK |
| F6 every uv segment | OK (L5 cosmetic) |
| F7 negative max_workers / SinTope | OK: 12 caps at 12, absent means no head cap (orq2), orq same as 1.0.0 |
| P9 planner card guide | Issue H1 (Zonas as prose breaks the scope check), M1 (literal 25 lines contradicts the caps) |
| P11 orq2 overlays | OK |
| P12 no cap, RAM warning, doctor --ram | OK (L2 warning likely unseen by the agent in hooks, L3 limits key) |
| Step 1 command table + strict test | OK, with L1 (test skips `skillOrq2`) |
| Step 4 all violations + `--topes` | Issue M2 (orq first error changed for some inputs; structural errors drop the collected ones) |
| Merge `skill.go` | OK (order right, no overlap) |
| Merge `tarjeta_guia_test.go` | Issue: asserts P9's "25 lines" and "what changes for the user" (M1, H1) |
| Goldens / orq byte-identity | No golden changed; byte-identity broken only for doubly invalid cards (M2) |
