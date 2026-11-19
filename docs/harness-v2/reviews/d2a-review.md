# D2a review (74eb76a..e69a847) — independent, temporary tests in internal/slice, deleted after

Defects that make a closed slice's checks untrustworthy:
1. The Go lint check passes when gofmt never ran. `internal/perfil/checks.go:13` writes `! gofmt -l . | grep .`. If gofmt is missing (exit 127) or cannot parse a file (exit 2), stdout is empty, grep exits 1, and `!` turns that into 0. Repro: with the default box (no sandbox.ro), a real Go module verified with `types` and `tests` at exit 127 but `lint` recorded as `paso`. Also `sh -c '! nosuchgofmt -l . | grep .'` exits 0, and so does a file with `func {`. So the receipt and the page show "lint paso" for a lint that never ran. Fix: `test -z "$(gofmt -l . 2>&1)"`, or check gofmt's exit code too.
2. A worker can rewrite what a check runs by editing `package.json` scripts. Checks run `npm test`, `npm run lint` and so on against the worker's commit (`slice/checks.go:82`). `package.json` is not in the protected paths (`slice/clones.go:55`, only `.orq/`, `.claude/` and other control paths). Repro: root `package.json` has `"test":"exit 1"`, the tests check runs that script, the worker commits `"test":"exit 0"`, the slice reaches VERIFICADO with tests `paso`. The same goes for `eslint.config.*`, `tsconfig.json` (`strict:false`), `.golangci.yml` and the like.
3. Deleting a project test file is not caught. Repro: the worker adds and then `git rm`s a non-exam `x_test.go`; the result is VERIFICADO and all checks `paso`. Only the frozen exam bundle is protected; that part is caught (BLOQUEADO "trampa").
4. Suppressions (`//nolint`, `eslint-disable`, `@ts-ignore`) are not caught by default. `orq init` writes no suppression guardrail. They are caught only if the Owner hand-writes an extra check, as in `TestCheckGuardiaExtraBloquea`. The spec's mechanism 3 asks for lint bans per project, and nothing proposes one.

What holds:
- Editing `.orq/config.json` `checks` in a clone is caught. The checks come from the main checkout (`app/proyecto.go:98`, `app/check.go:28`), and `.orq/` is protected in the diff.
- A failing lint, type, test or extra check blocks VERIFICADO and `cerrar` with the tool's lines (`slice/nucleo.go:362`).
- `omitido` is never shown as passed. Note: an omitted check, or a project with no `checks` block at all, still lets the slice close. The page warns, but the Owner may want a refusal.
- `orq init` writes only scripts that exist and skips the `no test specified` stub. Codex is absent from the defaults and refused everywhere (the config, perfil and app tests pass).
- Toolchain: on this machine, every Go check fails with exit 127 labelled infrastructure (except lint, see 1). With `ORQ_LIMITS` adding `sandbox.ro: ["~/.local/bin","~/.local/lib"]`, real gofmt, `go vet` and `go test` ran and passed in the box. The fix is usable, but no real project works until the Owner sets sandbox.ro. Node (~/.nvm) needs the same. Minor: the page labels exit 127 as `fallo`, not infrastructure (`cli/decidir.go:79`).

VERDICT: fix
