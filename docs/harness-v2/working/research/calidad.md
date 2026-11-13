# Quality gates for agent-written code (orq) — scout R5
V = stated on the cited page. I = my inference/knowledge, NOT verified. N = not found.
All runtimes are I (no source gives them; they scale with the suite).

## (a) Mutation testing
- Stryker (JS/TS). V: `--mutate "src/a.ts:10-40"` (file + line range, no globs with ranges); `--incremental` reuses reports/stryker-incremental.json; `--force` reruns all; `thresholds.break` exits 1 below it (default null = never); `--dryRunOnly` runs baseline only; timeout = net*1.5 + 5000ms. https://stryker-mutator.io/docs/stryker-js/configuration/ I: `npx stryker run --mutate <ranges from git diff>`; minutes; offline once installed.
- mutmut (Py). V: `mutmut run "mod.func*"`; reruns only functions whose source changed; options `use_git_change_detection`, `mutate_only_covered_lines`, `only_mutate`; pragma `# pragma: no mutate`; results in mutants/; `export-cicd-stats` gives score JSON; needs fork (WSL ok); v3 skips code outside functions. https://github.com/boxed/mutmut Exit codes N. cosmic-ray: N. Offline: I yes.
- Go gremlins. V: `gremlins unleash`; KILLED/LIVED/NOT COVERED/TIMED OUT/NOT VIABLE; 0.x so flags may change; slow on big modules. https://github.com/go-gremlins/gremlins Diff/threshold flags: N (check `--help`). go-mutesting: N.
- Rust cargo-mutants. V: `cargo mutants --in-diff FILE`; exit 0 all caught, 2 missed, 3 timeout, 4 baseline red, 5 diff/tree mismatch, 6 bad diff. https://mutants.rs/in-diff.html  https://mutants.rs/exit-codes.html
- Score (I): killed/(killed+lived) on viable covered mutants; equivalent mutants make 100% unreachable, so use a floor. Report in fix lane; floor only on changed lines.

## (b) Adversarial tests
- Go fuzz. V: `go test -fuzz=FuzzX -fuzztime=30s`; -fuzz must match one test; seeds from f.Add and testdata/fuzz/FuzzX/ run under plain `go test`; failures saved there as regressions; limited arg types; coverage guidance needs amd64/arm64. Stdlib, offline. https://go.dev/doc/security/fuzz/
- Hypothesis. V: max_examples 100, deadline 200ms; `ci` profile (when CI env set) = derandomize, deadline None, database None. Register own profile for orq. https://hypothesis.readthedocs.io/en/latest/reference/api.html
- fast-check. V: `fc.configureGlobal({numRuns})` in a setup file (Jest/Vitest/Mocha). https://fast-check.dev/docs/configuration/global-settings/ Defaults, seed: N.
- Second model writes tests the worker never sees: no primary source found. Matches P8. I: keep them outside the box, run in orq verify, show the worker only pass/fail count.
- Class: seed replay = hard (free); exploration = report.

## (c) Security
- gitleaks. V: `gitleaks dir .`, `gitleaks git . --log-opts="base..HEAD"`; exit 1 on leak; `--redact`; `--baseline-path`; .gitleaks.toml; detect/protect deprecated (8.19). Offline: page silent, I yes. Seconds. Hard. https://github.com/gitleaks/gitleaks
- npm audit. V: needs registry (bulk advisory endpoint) + lockfile; `--audit-level`, `--omit=dev`. NOT offline. https://docs.npmjs.com/cli/v10/commands/npm-audit pnpm audit: N.
- pip-audit. V: PyPI JSON or OSV; `-r req.txt`, `--no-deps`; exit 1 on vuln; no documented offline mode. NOT offline. https://github.com/pypa/pip-audit
- govulncheck. V: `govulncheck ./...` reads vuln.go.dev (network). Exit 3 and `-db file://` offline were "not on page" — I. https://go.dev/doc/tutorial/govulncheck => run audits in an orq step outside the box; hard only for NEW critical/high vs base.
- Semgrep. V: `semgrep scan --error` exits 1 on findings (default 0). https://docs.semgrep.dev/getting-started/cli  Local rules, offline, baseline: N.
- gosec. V: `gosec -severity medium ./...`; exit 1 on finding; `#nosec`; flags `-nosec-require-rules/-justification` reject bare nosec. Needs deps present (vendor). https://github.com/securego/gosec
- bandit. V: `bandit -r . -ll -ii -f json`, `-b baseline.json`, `# nosec`, `--exit-zero`. https://bandit.readthedocs.io/en/latest/man/bandit.html
- Agent-specific. V: 16 models, 576k samples: >=5.2% hallucinated packages (commercial), 21.7% (open), 205,474 unique fake names. https://arxiv.org/abs/2406.10279 (Repeatability, "slopsquatting": not on page.) I gate: a dependency added in the diff must be in the lockfile AND on an Owner allowlist/trusted store (P5); else hard fail. I: also grep diff for `0.0.0.0`, `verify=False`, `InsecureSkipVerify`, `chmod 777`.

## (d) Typing, lint, suppressions
- tsc. V: strict = alwaysStrict, strictNullChecks, strictBindCallApply, strictFunctionTypes, strictPropertyInitialization, noImplicitAny, noImplicitThis, useUnknownInCatchVariables, strictBuiltinIteratorReturn. NOT in strict: noUncheckedIndexedAccess, exactOptionalPropertyTypes, noImplicitOverride, noFallthroughCasesInSwitch. https://www.typescriptlang.org/tsconfig/#strict  `tsc --noEmit`: seconds, offline (I).
- pyright. V: modes off/basic/standard(default)/strict; `# type: ignore` honored by default; `reportUnnecessaryTypeIgnoreComment` default none even in strict; exit 0 ok, 1 errors, 2 fatal, 3 bad config, 4 bad args; `--outputjson`. https://raw.githubusercontent.com/microsoft/pyright/main/docs/configuration.md mypy --strict: N.
- staticcheck. V: `checks = ["all","-ST1000"]`; `//lint:ignore Check reason` needs reason. Exit/network N. https://staticcheck.dev/docs/configuration/ `go vet ./...`: stdlib (I).
- No new suppressions (my design, I): scan added lines of `git diff base...HEAD -U0` for `@ts-ignore|@ts-expect-error|eslint-disable|as any|: any|# type: ignore|# pyright: ignore| # noqa|# nosec|#nosec|//nolint|//lint:ignore`. Hit = fail unless the story lists it. Protect strictness config (tsconfig, pyrightconfig, staticcheck.conf, .golangci.yml) like the red test. Seconds, hard.

## (e) Cheap performance checks (all I, nothing verified)
Exam wall time vs baseline: report >2x, hard >5x. Go `-benchtime=1x` smoke only. No
benchmark gating (noisy). TS bundle size: report.

## Quality profile for orq (design; thresholds are starting points)
Config: `quality.<lang>` in limits.json, level per lane.
| gate | rapido | normal | estricto | class |
| secrets on diff | on | on | on | hard |
| no new suppressions / weaker config | on | on | on | hard |
| typecheck / go vet | on | on | on | hard |
| new-dependency allowlist | on | on | on | hard |
| lint (staticcheck, eslint, ruff) | - | on | on | hard on errors |
| SAST new-vs-base | - | report | hard high | mixed |
| dep audit (outside box) | - | report | hard critical | mixed |
| fuzz/property seed replay | - | on | on | hard |
| fuzz/property exploration 30-60s | - | - | on | report |
| mutation on changed lines | - | report | floor ~60 | report, then hard |
| held-out tests (P8) | - | - | on | hard |
Lanes: fix = rapido (normal if deps/auth touched); slice = normal; estricto on the
planner's risk flag. All run in orq verify on the clean copy, in the box, no egress,
except audits. Receipt gets one `quality` block: per gate {name, level, status
pass|fail|skipped|report, hard, count, seconds, tool+version, command} and mutation
{killed, lived, uncovered, timed_out, score}. Missing tool = skipped with reason, never
pass (same rule as 0 tests run).

## What orq should adopt (ranked: value, effort)
1. No-new-suppressions + protected strictness config — high, S.
2. Secrets scan on the diff — high, S.
3. Strict typecheck/vet command table per language — high, S.
4. `quality` receipt block, skipped != pass — high, S-M.
5. New-dependency vs allowlist check (fake-package defense, with P5) — high, M.
6. Fuzz/property seed replay inside the exam — medium, S.
7. Mutation on changed lines, report first — high, M-L.
8. SAST with base comparison; audits outside the box — medium, M.
9. Held-out tests by a second model (P8) — high, M-L.

## What orq should NOT adopt
Mutation/fuzz exploration in the fix lane; a 100% mutation score; a coverage % gate;
benchmark gating; network audits inside the box; semgrep cloud rules; findings that
already exist at base; reimplementing scanners in Go.

## Not verified
Runtimes; mutation tools inside a no-network box; cosmic-ray, go-mutesting, mypy, ruff,
eslint, pnpm audit; govulncheck exit code/-db; semgrep local rules/offline/baseline;
fast-check defaults; gremlins flags; any evidence for second-model test writing.
