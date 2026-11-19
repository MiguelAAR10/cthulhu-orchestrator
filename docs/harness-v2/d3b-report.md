# D3b report · Skill evals wired to `claude plugin eval`

Branch `v2/build`, worktree `~/worktrees/cthulhu-build`, built on `f79d5c7`. Plan: `docs/harness-v2/d3b-plan.md`. Spec: `plan.md` › "Route" › "Approach" mechanism 2, and the scout notes `working/research/d3b-plugin-eval-2026-11-19.md`.

What I did not do:
- Install, push, or write to `~/bin/orq`, `~/.config/orq` or `~/.local/state/orq`.
- Call a real `claude`. Every test puts a stub `claude` script first on `PATH` and uses temp `ORQ_*` dirs.
- Touch `internal/gate` or `internal/sandbox`.
- Add dependencies.

## Commits

| Task | Commit | Subject |
|---|---|---|
| plan | `80eff38` | docs: D3b plan — skill evals wired to claude plugin eval |
| 1 | `b5c8dd4` | skilleval: wrap one skill as a plugin for claude plugin eval; content hash |
| 2 | `a56ded6` | skilleval: run claude plugin eval, parse aggregate-result.json, detect early-access refusal |
| 3 | `d33b173` | skill: orq skill eval stores one score row per run; orq skill notas lists them |
| 4 | `fad1745` | skill: orq skill comparar keeps an edit only when its score rises past a margin |
| 5 | `76546f4` | launch: worker skills ranked by latest score for the slice's task type; skills.minimo leaves low scores out |
| report | this commit | docs: D3b report |

## `make ci` on `76546f4` (clean tree, exit 0)

- Candidate: `/tmp/orq-ci.8gbEzv/bin/orq · 1.0.0 revision=76546f4dd56342725554972ca19655fd9d542004 modified=false`.
- `make lint` (staticcheck 2025.1.1): clean. `gofmt -l harness`: nothing listed. `go vet ./...`: clean.
- `go test -race ./...`: 23 packages ok, none failing.
  - 333 top-level tests pass, 0 fail, 2 skipped. 44 subtests pass. (D3 had 297 top-level tests and 36 subtests.)
  - The 2 skips are the usual ones: `TestPrepararDeps` and herdr's `TestLiveTabSplitClose`.
- `e2e-v05`: **86 pass, 0 fail**.
- `aislamiento`: **14 pass, 0 fail, 0 pending**.

## What was built

A new domain package, `internal/skilleval` (layer 1 in `arch_test.go`, like `perfil`). It holds:
- the plugin wrapper;
- the call to `claude plugin eval`;
- the JSON parse;
- the score store, `$ORQ_STATE/skill-notas.jsonl`, with one JSON row per run;
- the ranking and the comparison.

Commands:
- `orq skill eval DIR --max-usd N [--tipo T]`:
  - Wraps the skill in a temp dir as `.claude-plugin/plugin.json`, `skills/<name>/` (the skill without `evals/`) and `evals/` (the skill's cases, at the plugin root).
  - Runs `claude plugin eval <plugin> --json <tmp>/aggregate-result.json --ablation none --max-cost-usd N --trust-plugin`.
  - Stores skill, content hash (sha256 of the skill dir), task type, score, runs, cost, claude version, partial flag, date and per-case scores. Then it prints one line, plus one line per case.
- `orq skill notas [--tipo T]` lists the rows by skill. Within a skill, the hash with the newest run comes first.
- `orq skill comparar OLD NEW [--tipo T] [--margen 0.1]`:
  - Takes the newest full row of each hash. A 12-character prefix, as `notas` prints it, is accepted.
  - Prints `conservar` only when the score rises more than the margin; otherwise it prints `descartar`.
- `orq slice abrir … --tipo T` stores the slice's task type. Config `skills.minimo` sets the threshold (0..1; unset means no filter).
- Launch (`app.LanzarWorker`) appends one paragraph to the worker's contract:
  - `Skills para el tipo T, de mejor a peor nota: a 0.92, b sin nota.`
  - when needed, `Fuera por nota bajo skills.minimo 0.50: c 0.30.`
  - The type is the slice's type, or else the stack (`perfil.Tipo()`: detected signals joined with `+`, or `general`). The skills are the project's worker skills (`perfil.ParaRol(…, "worker")`).

## Exit tests, item by item

1. **Wrapper layout accepted by the stub's argument checks.**
   - `skilleval.TestEnvolverDejaUnPluginDeUnaSkill`: manifest name, skill files under `skills/<name>/`, cases at `evals/`, no `evals/` inside the skill.
   - `skilleval.TestEnvolverRechaza`: a missing `SKILL.md` or `evals/` is refused before any money is spent.
   - `skilleval.TestCorrerPasaLosArgumentosYLee`: the stub exits 9 unless it gets `plugin eval`, a dir holding `.claude-plugin/plugin.json` and `evals/`, and exactly `--json P --ablation none --max-cost-usd 2.5 --trust-plugin`. The test checks score 0.75, 6 runs, cost 0.41, version and the per-case scores.
2. **Scores stored and listed.**
   - `cli.TestSkillEvalGuardaYNotasLista`: two evals of the same skill, edited in between, give two rows with different hashes, newest first. A `--tipo` filter with no rows prints `sin notas todavía`.
   - `skilleval.TestGuardarYNotas` and `TestListarAgrupaPorSkillYHashMasNuevoPrimero` cover the store and the ordering.
   - Exit 1 (a case below the CLI's `--threshold`) is still a score: `TestCorrerSalida1EsResultado`. Exit 2 (cost ceiling) is stored as partial: `TestCorrerTopeDeCostoEsParcial`. A run with no JSON is an error carrying stderr: `TestCorrerFallaSinJSON`.
3. **Ranking and threshold at launch.**
   - `skilleval.TestOrdenar`:
     - skills are ordered by their latest score, ties by name, unscored skills last in catalog order;
     - partial rows and other types are ignored;
     - the threshold moves skills to "fuera".
   - `app.TestLanzarOrdenaSkillsPorNota`: with `skills.minimo 0.5`, the delivered contract lists `systematic-debugging 0.90` before `test-driven-development sin nota`. It also says `Fuera por nota bajo skills.minimo 0.50: receiving-code-review 0.20.`
   - `app.TestLanzarUsaElTipoDelSlice` and `cli.TestSliceAbrirGuardaElTipo`.
   - `config.TestConfigSkillsMinimo`: 0.5 loads, there is no default, and 1.5 is refused.
   - I removed the paragraph from the prompt and saw both app tests fail, then put it back.
4. **comparar with margin.**
   - `skilleval.TestCompararConMargen`:
     - +0.20 → keep; +0.05 → discard; −0.30 → discard; margin 0.01 keeps +0.05;
     - prefixes work, and an ambiguous prefix is an error;
     - a newer partial row (0.99) does not count, and a hash with only partial rows is refused;
     - two different types without `--tipo` are refused.
   - `cli.TestSkillCompararCLI`: `conservar: nota 0.60 → 0.80 (+0.20 > margen 0.10)`; with `--margen 0.5` it prints `descartar`.
5. **Refusal without `--max-usd`.** `cli.TestSkillEvalExigeMaxUSD`: with no flag or with `0`, it exits 1 with `--max-usd es obligatorio: cada corrida gasta dinero real`, and the stub's marker file is never created.
6. **Unavailable-eval message.**
   - `skilleval.TestCorrerNoDisponible`: stderr `Error: plugin eval is currently in early access` gives `ErrNoDisponible`, and the message keeps the CLI's line. A `PATH` without `claude` gives the same.
   - `cli.TestSkillEvalNoDisponible`: exit 3, the message says `acceso anticipado`, and nothing is stored.

Every test above was seen failing before its code existed: a build failure for a new symbol, then an assertion failure. For the launch paragraph I checked the assertion failure explicitly. All of them run in `make ci`.

## Design choices made from the code (no design changed)

- **Task type lives on the slice** (`Estado.Tipo`, set by `abrir --tipo`), not on the card. Fix-route slices have no card, and the card parser accepts exactly five sections. The type is not sealed in the contract: it only orders advice, not permissions.
- **Where "the page says which":** the list goes into the worker's contract text at launch, which is the page the worker reads. `orq skill worker` and the decision page are unchanged.
- **Partial runs** (cost ceiling) are stored and listed with `(parcial)`, but ranking and `comparar` ignore them.
- **`--trust-plugin`** is passed as in the scout's driver call. The flag set comes from the scout's `--help` reading of CC 2.1.328; I did not run `claude` here.
- **Unavailability detection** matches stderr against `early access|not available|unavailable|unknown command` (case-insensitive), or `claude` missing from `PATH`. Any other failure is reported as-is with its exit code.
- **The hash covers the whole skill dir, `evals/` included.** Changing the cases changes the hash, so two hashes may differ in their exam as well as in the skill.

## Not done, and why

- **No real eval was run.** Availability on the Owner's account (early access) is still unverified, as the scout said. The first real `orq skill eval … --max-usd 1` will answer it, either with a score or with the "no disponible" message.
- **No eval cases were written for orq's own skills.** The row asks for the wiring; cases and rubrics per skill are authoring work, for example with `claude plugin eval init`.
- **No automatic edit loop.** `comparar` gives the verdict; a person or worker edits and reruns.

## Open decisions for the Owner

1. Is the worker's contract the right place for the ranked list, or should `orq skill worker` and the decision page show it too?
2. Should the task type become a sealed card field once the card is used on the fix route as well?
3. What should the default margin be once real noise is measured? 0.1 is a starting point; three runs with an LLM judge may need more runs or a pinned `--judge-model`.

## Suggestions outside this row

- `orq skill eval` could pass `--model`/`--judge-model` from config, so that scores taken on different days stay comparable.
- staticcheck reports three pre-existing ST1005 capitalized error strings in `app/workers.go:85,88` and `cli/plan.go:21`. They are outside the lint target and untouched.

## Review fixes, round 1

Fixes for `reviews/d3b-review.md`. Each has a test that failed before the fix.

1. **Failed runs.** `skilleval.Leer` now reads `arms.with[].error` and `.aborted`. Any failed run makes the result partial (`Fallidas`, `Error`). If every run failed, `Correr` returns an error with the first error text (e.g. `timed out after 1s`) and nothing is stored. Ranking and `comparar` already skip partial rows. Test: the review's JSON, exit 1 (`TestCorrerNoGuardaSiTodasFallaron`). The shared fixture no longer carries an errored run.
2. **Hash at launch.** `Ordenar` takes the hash of each skill as installed (`skilleval.Hash` of `.claude/skills/<name>`, the same function `skill eval` uses) and counts only rows with that hash; otherwise "sin nota". Test: eval v1, eval v2, revert to v1 gives v1's own score; an edited, unevaluated v3 is "sin nota" (`TestLanzarUsaLaNotaDelHashInstalado`, `TestOrdenarSoloCuentaElHashInstalado`).
3. **Bad notes line.** New `LeerNotas` returns good rows plus the bad lines. Launch warns on stderr and goes on, even if the file cannot be read. `Notas` (used by `orq skill notas`/`comparar`) still fails on a bad line. Test: `TestLanzarSigueConUnaLineaRotaEnLasNotas`.
4. **`orq skill eval .`** resolves the directory with `filepath.Abs` before taking the name. Test: `TestSkillEvalPuntoUsaElNombreDeLaCarpeta`.

## Close (orchestrator, 2026-11-19)
Integrated as 49fbac8 after 1 review round (`reviews/d3b-review.md`) and its fix (4ea140d). `make ci` on 49fbac8 with the candidate (`modified=false`): exit 0, e2e 86/0, aislamiento 14/0. Orchestrator check: the fix's tests for the reviewer's repros fail on the pre-fix code (8238520) — `TestLanzarUsaLaNotaDelHashInstalado`, `TestLanzarSigueConUnaLineaRotaEnLasNotas`, `TestSkillEvalPuntoUsaElNombreDeLaCarpeta`; the `skilleval` package tests do not build against the old API — and pass on 49fbac8.
Real run by the reviewer: the account has `claude plugin eval`; a one-case skill scored 1.00 for USD 0.05.
Open for the Owner: the score margin once judge noise is measured; whether the task type is sealed on the card.
