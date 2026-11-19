# Step 2 · Tasks 1–3 · Builder report (stopped early)

**Status:** Task 1 done and committed. Tasks 2 and 3 not started: the Orchestrator asked to stop after the current task because the Owner is re-planning.

## Commits
- `180ee54` sandbox: own netns with a unix-socket bridge to an allowlisting CONNECT proxy (Task 1).

## Checks (on the Task 1 tree, before the commit)
- `gofmt -l harness`: empty. `go vet ./...`: clean. `go test -race ./...`: 22 packages ok, 0 failing.
- `make ci`: exit 0. e2e-v05: 86 pass, 0 fail. aislamiento: 14 pass, 0 fail, 0 pending.
- Tested candidate: `orq version` = `1.0.0 revision=10a950a38c25fbae864808b4fcff625311af4700 modified=true` (base tip plus the uncommitted Task 1 files, built by `make ci`).

## Task 1 tests (each one seen failing first, against stubs, then passing)
| Test | Result |
|---|---|
| TestProxyNiegaHostNoPermitido (403, `DENY example.com:443`, no headers in the log) | pass |
| TestProxyPermitido (CONNECT to a local echo server on the allowlist, tunnel works) | pass |
| TestProxyPuertoPorDefecto (an entry with no port allows only :443) | pass |
| TestProxyNoEsHTTPPlano (GET → 405) | pass |
| TestPuenteReenvia (TCP inside → unix socket → proxy → echo) | pass |
| TestConstruirRedProxy (no `--share-net`, `--bind <dir> /run/orq/red` after the `/run` tmpfs, no resolv.conf) | pass |
| TestConstruirRedProxyInvalida (no dir, relative dir, with `Net`, dir under `/run` → error) | pass |
| TestDirSocketCorto (200-byte `TMPDIR`, path + `/p.sock` < 108, dir 0700) | pass |

## Rulings (my calls where the plan was not specific)
- The plan names `type Red int` but no field for it. I added `Spec.Red Red` next to `Spec.Proxy`. `RedProxy` together with `Net` is an error. If this is wrong, it is one field to rename.
- Allowlist entries are `host:port`. An entry with no port means `:443`, so `caja.red.permitidos` can list plain hostnames.
- A non-CONNECT request is logged as `DENY <METHOD> host`. If the upstream dial fails, the proxy logs `FAIL host:port` and answers 502.
- I added `TestPuenteReenvia` and `TestProxyPuertoPorDefecto`, which the plan does not list, so that `Puente` and the default port have their own failing-first tests.

## Not done
- **Task 2 (credential):** not started. Nothing has checked yet whether `/proc/<pid>/mem` can be read.
- **Task 3 (`orq sandbox agente`):** not started. The proxy has not run end to end inside a real box. The manual dummy-token check to `api.anthropic.com` (expect 401) was not run, so this session made no real network call.
- No final whole-branch review yet. It should run once Tasks 1–3 (or the re-planned scope) are done.
