# D2b review (6f9d374..85ec577): independent; throwaway Go module + temp ORQ_* dirs (cleanup rm was denied; left in session scratchpad `e2e/` and /tmp/orq-r)

Defect (checks unusable on a standard install, and the diagnostic reports the opposite):
- Binding the toolchain skips intermediate symlinks. `sandbox/toolchain.go:114` binds `Dir(PATH hit)` and `Dir(EvalSymlinks)` only. Here `~/.local/bin/go -> ~/.local/lib/go/bin/go` and `~/.local/lib/go -> go-1.25.8`. The box gets `~/.local/bin` and `~/.local/lib/go-1.25.8`, but not the `~/.local/lib/go` link, so `~/.local/bin/go` dangles inside the box. Real run with no sandbox.ro: lint, types and tests all fail at exit 127 ("go: not found"). `orq check --caja` still prints "corre en la caja · monta …" for all three (`cli/check.go:74`), because it never probes the box. The exit-127 run also uses up a repair, and the next verificar reaches BLOQUEADO "sin progreso". Reproduced by hand with bwrap and the same two binds: `go: not found`. Fix: bind every symlink hop (or the realpath's toolchain root plus each link's directory), and have `--caja` run `command -v <tool>` inside the box.
- Related: the exam itself (`slice/nucleo.go:96`) gets no toolchain binding. A Go exam that calls `go` needs `~/.local/bin` and `~/.local/lib` in sandbox.ro.

Findings from d2a-review, rerun:
1. gofmt lint: fixed (`perfil/checks.go:14`). Missing gofmt gives exit 127, a parse error gives exit 2, and neither passes.
2. package.json scripts: fixed. A worker change from `"test":"exit 1"` to `"exit 0"` sets `rec.DefinicionesChecks=[package.json]`, the page shows the diff, and `cerrar --aceptado` is refused without `--acepto-checks` (`slice/cierre.go:103`).
3. Deleted test: fixed as a warning (`slice/nucleo.go:363`). `git rm x_test.go` shows "tests deleted" in the note and "test borrado por el slice" on the page. It still lets the slice close; acceptable, since the Owner sees it.
4. Suppressions: fixed. The default `supresiones` guard is on whenever a checks block exists. A `//nolint` on an added line fails verificar: "r.go:3: … //nolint".
5. Exit-127 label: fixed (`cli/decidir.go:82`, shown as "infraestructura").

E2E (orq init, red exam, a commit that turns it green, verificar), with sandbox.ro `["~/.local/lib","~/.local/bin"]`: VERIFICADO. The receipt shows lint/types/tests at exit 0 running the real `gofmt -l`, `go vet ./...` and `go test ./...` ("ok example.com/m"), and supresiones passes. Without that sandbox.ro, see the defect above. `go test ./...` in harness passes.

VERDICT: fix
