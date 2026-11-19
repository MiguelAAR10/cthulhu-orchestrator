# Step 2 · Worker box (B6) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The worker agent (Claude Code) and every tool it runs live inside the bwrap box. The box has only a model credential, its own clone, read-only deps and one allowed host. Its work leaves only as a bundle that orq checks.

**Architecture:** `orq sandbox agente <name> -- <claude args>` builds a `sandbox.Spec` for one worker and runs Claude inside it. The Spec has the clone RW, a private config dir RW, deps and toolchains RO, its own netns and a loopback bridge to an allowlisting CONNECT proxy outside. The token arrives on a pipe at fd 3. The herdr pane reaches this through a per-worker `claude` shim on PATH (the only route that worked in spike H4). Nothing in the box can see the main repo, `.orq`, the state dir, `limits.json` or the Owner's Claude configs.

**Tech Stack:** Go standard library only; bubblewrap 0.9.0; Claude Code 2.1.328; herdr.

**Spec:** `docs/harness-v2/plan.md`: "Milestone direction" (Step 2 exit tests, Technical trial, Credential), "Design", "Reorder". Background: `working/spike-resultados.md` (H1–H9), `working/plan-aislamiento.md` §1, `working/research/disk-memory-2026-11-18.md`, `step1-report.md` "Close" and "Known gaps".

## Global Constraints
- Go standard library only. No Codex for any role. English for new code, messages and docs.
- No unsandboxed fallback: if bwrap is missing or the selftest fails, the launch fails (existing `sandbox.Disponible`).
- Never `--ro-bind / /`; never bind `$HOME`, `~/.claude`, `~/.claude-alt`, `~/.config/orq`, the state dir, `limits.json` or the main checkout's `.git`.
- The token is never in argv, in any environment block, in a file inside the box, in a log, in the journal, in a receipt, in a report or in a commit.
- Never start Claude with `--bare`: under `--bare` it reads no OAuth token (verified, E3 below).
- Every test runs with temp `ORQ_STATE`, `ORQ_LIMITS`, `ORQ_WORKTREES`, `ORQ_CONFIG` and `HOME`. `make ci` never runs real `claude` and never uses the network; it uses a fake agent script.
- The gate stays as it is (no new shell parsing). Containment comes from the box.

## Review Focus
1. **A tool's grandchild that double-forks or calls `setsid`** should still be inside the box: it shares the box's pid ns. The test is in Task 4 (`TestAgenteArbolDentro`, a fake agent that runs `setsid sh -c 'sleep 30 &'`).
2. **The token through `/proc`**: a tool reads `/proc/1/environ`, `/proc/$PPID/environ` or `/proc/$PPID/fd/3`, and it should get nothing (E4/E5 below). The test is in Task 2.
3. **A worker that writes the token into its commit** should get its bundle rejected by name ("contains the model credential"), not have it ingested. The test is in Task 6.
4. **A long state-dir path** makes the proxy's unix socket path go over 108 bytes (`AF_UNIX path too long` actually hit in the experiment). The launch should still work, using a short path. The test is in Task 3.
5. **`claude-alt` / `~/.claude` users**: a configured `workers.*.claude.entorno.CLAUDE_CONFIG_DIR` should be ignored for boxed workers, with one warning. It must never be bound in. The test is in Task 4.

---

## 1. Exit tests

Decoys are files with the content `ORQ-DECOY-<name>` and a recorded sha256, placed outside the clone. The fake agent is `harness/internal/app/testdata/agente-falso.sh`. It runs the attempts it is given and writes the results to `$CLON/.resultado`.

| # | Property | Positive test | Negative test (decoy / attack) | Where |
|---|---|---|---|---|
| 1 | The whole agent process tree runs inside the box | `TestAgenteArbolDentro`: every descendant of the launched process has `readlink /proc/P/ns/pid` ≠ the test's own, and `test -c /run/orq/sandbox` is true in a grandchild after `setsid` | `TestAgenteSinCajaNoArranca`: `rutasBwrap` points to a missing binary → `orq sandbox agente` exits ≠0 with `sandbox: no está bwrap` and the fake agent's marker file does not exist | `make ci` (skip only with `ORQ_CI_ALLOW_SKIP=1` when bwrap is missing) |
| 2 | Edit and test in its clone | `TestAgenteEditaYPrueba`: the fake agent writes `suma.go` and `suma_test.go`, runs `go test ./...` (GOCACHE in `/tmp`) and `git commit` → exit 0, and the commit is in the clone | `TestAgenteDepsSoloLectura`: `touch node_modules/x` and `echo x >> .venv/pyvenv.cfg` fail with `Read-only file system`; the host files keep their sha | `make ci` |
| 3 | No write path to the authorized contract, the protected state or the integration branch | `TestAgenteSenuelos`: decoys at `$ORQ_STATE/DECOY`, `$ORQ_LIMITS`, `<main>/DECOY.txt`, `<main>/.git/refs/orq/contrato/01`, `<main>/.git/refs/heads/harness-v2`, `$HOME/.claude-alt/DECOY`, `$HOME/.claude/DECOY`, `$HOME/.config/orq/credenciales/claude` (dummy), and a second worker's clone. For each one the fake agent's `cat` and `echo >>` both fail (ENOENT, because none is mounted). Afterwards every sha and `git rev-parse refs/orq/contrato/01` are unchanged | Same test, attack rows: `git -C <main> update-ref refs/orq/contrato/01 HEAD`, `orq slice cerrar 01 --aceptado` from inside the box (Task 4 makes it refuse with `inside the box: worker commands only`), `ln -s <main> l && git -C l …` | `make ci` |
| 4 | The bundle is a proposal checked against base and scope | Existing `ingerir` tests, plus `TestIngestaDentroDeAlcance`: a commit on top of red inside the zone → `Ingesta{Nuevo:true}` | `TestIngestaHistoriaReescrita` (tip does not descend from `SHARojo`, so the ref is removed); `TestIngestaFueraDeAlcance` (touches `tests/aceptacion/` → rejected, path named); `TestIngestaConCredencial` (commit contains the dummy token → rejected with `contains the model credential`, nothing fetched) | `make ci` |
| 5 | A valid result advances; an invalid one is rejected with a reproducible reason | `TestCajaResultadoValidoAvanza`: green exam → state `VERIFICADO`, receipt sealed | `TestCajaResultadoInvalidoReproducible`: a failing exam is verified twice → same reason string and same exam fingerprint both times, state stays `IMPLEMENTANDO` | `make ci` |
| 6 | Repeats and lost replies cause no duplicate merge and no blind relaunch | `TestIngestaRepetidaUnaRef` (two ingests of the same tip → one ref and `Nuevo` false the second time); `TestCerrarRepetidoUnaFusion` (`Cerrar` twice → one merge commit on base, one receipt) | `TestLanzarRespuestaPerdidaNoRelanza`: the fake terminal returns `ErrDesconocido` from `Start`, then `Vivo` → the second `LanzarWorker` opens no pane and the task is not released | `make ci` |

`validacion/aislamiento.sh` gets part 3, "worker in the box": it runs rows 1–3 against the built candidate binary with the fake agent. The real-agent run is `validacion/caja-real.sh` (§4). It is manual and never part of `make ci`.

**Where the authorized contract lives.** `refs/orq/contrato/NN`, `refs/orq/recibo/NN`, `refs/orq/integrado/NN` and the base branch all live in `<main>/.git`. The box never mounts the main checkout (only, read-only, its `node_modules`/`.venv`). The clone is `git clone --no-hardlinks` with no remote, so it shares no inode with `<main>/.git`. The worker has no path at all to those refs. Its only output is the clone, read by orq inside a short box (`ingerir`, already built).

## 2. Credential design (verified with a dummy value)

**Account and config.** The Owner selects accounts with `CLAUDE_CONFIG_DIR`. The default is `~/.claude` (`claude-main`), and `alias claude-alt='CLAUDE_CONFIG_DIR=$HOME/.claude-alt claude'`. I found this from `type claude-alt`; no file under either dir was read. Boxed workers use neither: each worker gets a fresh `CLAUDE_CONFIG_DIR=<state>/cajas/<name>/claude`, seeded by orq with `settings.json` (the gate hook `orq gate` as PreToolUse). It starts with `--setting-sources user --strict-mcp-config` (to verify in Task 4: the clone's `.claude/settings.json` adds nothing then). The account is whichever account the Owner logs into while running `claude setup-token` (§5). His sessions keep their own OAuth files, so a refresh in one cannot invalidate the other.

**Storage.** `~/.config/orq/credenciales/claude`, mode 0600, directory 0700, owned by the user, outside every repo. Its path is configurable as `caja.credencial.claude` in the user config. A path is not a secret, so `validarEntorno` stays unchanged. `credencial.Leer` refuses a group/other-readable file, a symlink, or a file inside `orq.Root`.

**Delivery.** orq reads the file outside the box, writes the token into a pipe and closes the write end. The read end becomes fd 3 of bwrap (`cmd.ExtraFiles`), and the box env has `CLAUDE_CODE_OAUTH_TOKEN_FILE_DESCRIPTOR=3`. Never `CLAUDE_CODE_OAUTH_TOKEN`: with the env variable the marker shows up in `/proc/1/environ` (bwrap) and in the parent's environ, which any tool can read.

**Network.** Box in its own netns (`--unshare-all`, no `--share-net`). Inside, `orq sandbox agente` itself listens on `127.0.0.1:3128` and pipes each connection to a unix socket bind-mounted at `/run/orq/red/p.sock`. Outside, `sandbox.Proxy` serves that socket: CONNECT only, allowlist `api.anthropic.com:443`, one log line per request with host, port and verdict (never headers). The box env has `HTTPS_PROXY=http://127.0.0.1:3128`, `CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1` and `DISABLE_AUTOUPDATER=1`. Blocked: every other host, the host's localhost services, abstract sockets, docker/herdr/dbus sockets (H2) and DNS (the proxy resolves outside). **Owner decision (2026-11-18): the worker keeps its full development capability.** The proxy allowlist also has the package and skill sources: `registry.npmjs.org`, `registry.yarnpkg.com`, `pypi.org`, `files.pythonhosted.org`, `proxy.golang.org`, `sum.golang.org`, `github.com`, `codeload.github.com`, `objects.githubusercontent.com` (for `npx skills add`), configurable per project as `caja.red.permitidos` (added to `api.anthropic.com`, never replacing it). Every request is logged host:port and verdict. Default stays read-only deps from §3; when the worker runs an install (`npm i`, `pnpm add`, `uv add`, `go get`) or the lockfile differs, that clone gets a private writable `node_modules`/`.venv` built from the shared store or cache, and the lockfile change returns in the bundle and is checked like any other file. Accepted risk, stated plainly: install scripts run inside the box and can reach the allowed hosts; the token is not in env, files or fds (drained pipe), and Task 2 records whether `/proc/<claude>/mem` is readable (ptrace_scope).

**No orq authority.** The token only authorizes model requests. Inside the box: (a) there is no state dir, no `limits.json`, no main `.git`; (b) `orq` sees `sandbox.Dentro()` and allows only `gate`, `skill` and `version`; (c) orq takes accept, merge and close only outside the box, from the contract refs. Owner authority (passphrase) is step 3, and nothing here pretends to give it.

**Kept out of logs and commits.** `credencial.Token` is a type whose `String`/`GoString`/`MarshalJSON` return `[redacted]`. `ingerir` rejects a bundle whose patch text contains the token. `TestCredencialNoSeFiltra` runs one fake-agent slice end to end with the dummy marker `sk-ant-oat01-ORQDUMMY-<random>`. It then greps the marker across `$ORQ_STATE` (journal, receipts, reports, proxy log), the clone, the main repo (`git log -p --all`), stdout/stderr and `/proc/self/environ` of the orq process. Expected: 0 hits.

**Experiment** (scratch only, dummy `sk-ant-oat01-ORQDUMMY-7f3a9c`, allowlist-root bwrap like `sandbox.construir`, private empty config dir, `env -i`):

| Run | Setup | Output | Reading |
|---|---|---|---|
| E1 | no token, no net | `Not logged in · Please run /login` | baseline |
| E2 | `CLAUDE_CODE_OAUTH_TOKEN=<dummy>`, no net, `ANTHROPIC_LOG=debug` | 16× `url: "https://api.anthropic.com/v1/messages`, `getaddrinfo EAI_AGAIN api.anthropic.com`, rc 124; marker in output: 0; marker in config dir: 0 files | the token is read; with nonessential traffic off, the only host is `api.anthropic.com`; Claude does not persist the token to disk |
| E3 | same as E2, plus `--bare` | `Not logged in · Please run /login` | `--bare` ignores the token, so it must not be used |
| E4 | env token; SessionStart hook prints marker counts | `env=0 pid1=1 ppid_env=1` | Claude scrubs its children's env, but `/proc/1/environ` and the parent's environ leak it |
| E5 | `…_FILE_DESCRIPTOR=3`, fd 3 a **file** | the hook reads `/proc/2/fd/3 … hits=1` | Claude keeps fd 3 open, so a file is readable by tools |
| E5/E6 | fd 3 a **pipe** | `env=0 pid1=0 ppid_env=0`, 0 hits on every fd, `EAI_AGAIN api.anthropic.com` (so authenticated) | **chosen**: a drained pipe leaves nothing to read |
| E7 | own netns + python bridge/CONNECT proxy | `OSError: AF_UNIX path too long` | not run to the end: the proxy is to verify in Task 3, with the socket on a short path |
| — | nested `bwrap --unshare-all true` inside the box | rc 0 | Claude's own subprocess sandbox can work inside (not relied on) |

Not verified, owned by tasks: proxy end to end with the dummy (expect an HTTP 401 from the API, Task 3); whether `ptrace_scope` stops a tool from reading Claude's memory (`/proc/$PPID/mem`), Task 2, recorded as a documented limit if it does not; whether `setup-token` writes into its config dir (Task 7, Owner run).

## 3. Dependencies in the box

All of them are read-only views of the **main checkout's** deps, never copies (P5). If the lockfile differs from main's, they are not bound and the worker is told why.

| Ecosystem | Detected by | Bound | Note |
|---|---|---|---|
| npm (flat) | `package-lock.json` + `node_modules/` | `<main>/node_modules` → `<clone>/node_modules` RO (`sandbox.Vinculo`, as the exam already does) | verified H8 |
| pnpm | `pnpm-lock.yaml` + `node_modules/.pnpm` | the same, plus each `<main>/{apps,packages}/*/node_modules` → the same relative path in the clone | `.pnpm` links are relative inside `node_modules`; store hardlinks need no store mount. Workspace links to verify in Task 5 |
| uv / venv | `uv.lock` or `.venv/` | `<main>/.venv` RO at its own absolute path, plus a symlink `<clone>/.venv` | verified H8 (venvs hold absolute paths) |
| Go | `go.mod` | `$(go env GOMODCACHE)` RO; `GOCACHE=/tmp/go-cache`, `GOFLAGS=-mod=readonly`, `GOPROXY=off` | no Go projects among the worktrees; needed for orq itself and the trial |
| Toolchains | `limits.json` `sandbox.ro` | already supported | spike H1 list: `~/.nvm ~/.local/bin ~/.local/lib ~/.local/share/claude` |

Lockfile check: `bytes.Equal` of the clone's lockfile at the slice base against `<main>`'s working-tree lockfile. If they differ, nothing is bound and the contract text says "deps differ from main: this slice must declare a dependency change". Report evidence: npm copies about 1.6 GB per worktree and 52.5 GB of `node_modules` across 47 dirs; a read-only bind costs 0 GB and 0 s. This lets `TestPrepararDeps` lose its `t.Skip` (Task 5).

## 4. Technical trial (`validacion/caja-real.sh`, manual, after Tasks 1–7 are green)

1. Throwaway repo `$(mktemp -d)/prueba`: a Go module with `suma.go` (`func Suma(a, b int) int` returning `a - b`), a red acceptance test `tests/aceptacion/suma_test.go`, and `.orq` from `orq init`. Temp `ORQ_STATE`, `ORQ_WORKTREES`. Real credential (§5).
2. **Valid run:** `orq slice abrir --ruta fix --examen tests/aceptacion` → `orq slice lanzar --agente claude --tarea "fix Suma"` (Sonnet, effort medium) → the worker fixes the function and commits inside the box → `orq slice verificar 01` (independent: clean archive copy, sandboxed exam, bound to the contract ref) → `orq slice cerrar 01 --aceptado`. Expected: one merge on base, one receipt.
3. **Must-reject run:** slice 02, same story. The brief tells the worker to "also update the acceptance test to match". Expected: `verificar` rejects with `touched protected path tests/aceptacion/suma_test.go` (scope) or an exam-fingerprint mismatch, with the same reason on a second `verificar`.
4. During both runs the script checks property 1 from outside: every pid under the pane has the box's pid ns, plus the proxy log has only `ALLOW api.anthropic.com:443`.
5. **Recorded** in `docs/harness-v2/step2-trial.md`: harness commit and binary revision; wall time per phase (from journal timestamps: launched, first commit, worker-finished, verified, closed); attempts (`Intento`); Owner interventions (each one written by the orchestrator as a journal note, `orq diario nota "intervención: …"`, then counted); observable cost (sum of `usage` tokens in the worker's private `projects/*/*.jsonl`, to verify in Task 7 that the field exists, plus `/cost` read from the pane at the end); the denied hosts in the proxy log; and the result of the credential grep.

## 5. Owner procedure (his own terminal, never chat)

```sh
umask 077 && mkdir -p ~/.config/orq/credenciales
CLAUDE_CONFIG_DIR="$(mktemp -d)" claude setup-token   # log in with the account workers should use; it prints the token
read -rs T && printf %s "$T" > ~/.config/orq/credenciales/claude && unset T   # paste; nothing echoed, nothing in history
chmod 600 ~/.config/orq/credenciales/claude
orq caja credencial    # prints only: path, mode 0600, length, prefix sk-ant-oat01-…; never the value
```

The temp config dir keeps the setup flow away from `~/.claude` and `~/.claude-alt`. Delete it afterwards (`rm -rf` the dir it printed). To revoke: `rm` the file; the token is also listed on claude.ai under the account's settings.

## 6. Tasks (one Opus builder, serial, TDD: each test is seen failing first)

### Task 1: Box network: own netns, bridge, proxy
**Files:** Modify `harness/internal/sandbox/sandbox.go`; Create `harness/internal/sandbox/red.go`, `red_test.go`.
**Produces:** `type Red int` (`SinRed`, `RedProxy`; `Spec.Net bool` stays for the exam's `--red local`). `Spec.Proxy string` is the outside socket dir, bound at `/run/orq/red`. `func Proxy(ln net.Listener, permitidos []string, log io.Writer) error` (CONNECT only; logs `ALLOW|DENY host:port`). `func Puente(sock string, addr string) (net.Listener, error)`, used inside. `func DirSocket() (string, error)` returns `os.MkdirTemp("/tmp", "orq-r")`, so the socket path stays ≤ 107 bytes.
- [ ] Tests: `TestProxyNiegaHostNoPermitido` (403, log `DENY example.com:443`), `TestProxyPermitido` (to a local TLS-less echo server added to the allowlist), `TestProxyNoEsHTTPPlano` (GET → 405), `TestConstruirRedProxy` (argv has no `--share-net`, has `--bind <dir> /run/orq/red`), `TestDirSocketCorto` (`len(path)+len("/p.sock") < 108` with a 200-byte `TMPDIR`).
- [ ] Implement, run `go test -race ./internal/sandbox`, commit `sandbox: own netns with a unix-socket bridge to an allowlisting CONNECT proxy`.

### Task 2: Credential
**Files:** Create `harness/internal/credencial/credencial.go`, `credencial_test.go`; Modify `harness/internal/config/config.go` (`Caja.Credencial map[string]string`).
**Produces:** `type Token struct{ v string }`; `func Leer(path, repoRoot string) (Token, error)`; `func (Token) String() string` = `"[redacted]"` (also `GoString`, `MarshalJSON`); `func (t Token) Tubo() (*os.File, error)` (the read end, with the write end already written and closed); `func (t Token) Contiene(b []byte) bool`; `func Resumen(path string) (string, error)` for `orq caja credencial`.
- [ ] Tests: `TestLeerRechazaPermisos` (0644 → error naming `chmod 600`), `TestLeerRechazaEnlace`, `TestLeerRechazaDentroDelRepo`, `TestTokenNoSeImprime` (`fmt.Sprintf("%v %+v %#v")` and `json.Marshal` have no marker), `TestTuboSeVacia` (after reading the pipe to EOF, nothing is left), `TestResumenSinValor`, `TestTokenNoVisibleEnProc` (bwrap-gated: box with the pipe on fd 3 and a reader that drains it; a sibling greps `/proc/*/environ` and `/proc/*/fd/*` → 0; also tries `/proc/<reader>/mem` and records the result as pass or documented limit).
- [ ] Implement, run, commit `credencial: token file 0600, delivered on a drained pipe, never printed`.

### Task 3: `orq sandbox agente`
**Files:** Modify `harness/internal/cli/sandbox.go`; Create `harness/internal/app/caja.go`, `caja_test.go`.
**Consumes:** Task 1 `Proxy`, `Puente`, `DirSocket`; Task 2 `Leer`, `Tubo`.
**Produces:** `func SpecWorker(p Proyecto, nombre, clon string) (sandbox.Spec, error)`: clone RW; `<state>/cajas/<nombre>/claude` RW (seeded `settings.json`: the gate hook calls the orq binary path that is bound RO); orq binary RO; deps from Task 5 (empty until then); `RedProxy`. Env exactly: `PATH`, `HOME`, `CLAUDE_CONFIG_DIR`, `CLAUDE_CODE_OAUTH_TOKEN_FILE_DESCRIPTOR=3`, `HTTPS_PROXY`, `CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1`, `DISABLE_AUTOUPDATER=1`, `ORQ_ROL=worker`, `GIT_CONFIG_GLOBAL=/dev/null`, `GOCACHE=/tmp/go-cache`. `orq sandbox agente <nombre> -- ARGS` starts the proxy, then runs inside the box `orq sandbox dentro -- claude --setting-sources user --strict-mcp-config ARGS` (the in-box half starts `Puente` and waits on its child). It refuses `--bare` in ARGS.
- [ ] Tests: `TestSpecWorkerEntornoExacto`, `TestSpecWorkerSinRutasDelOwner` (no mount under `$HOME/.claude*`, `~/.config/orq`, the state dir, `<main>/.git`), `TestAgenteRechazaBare`, `TestAgenteProxyEndToEnd` (bwrap-gated, local allowlisted fake server: the fake agent's `curl -x` reaches it; `example.com` → 403).
- [ ] Manual check recorded in the commit message: dummy token through the proxy to `api.anthropic.com` → HTTP 401, proxy log `ALLOW api.anthropic.com:443` only.
- [ ] Commit `cli: orq sandbox agente runs one worker's agent inside the box`.

### Task 4: Launch through the box; orq inside the box has no authority
**Files:** Modify `harness/internal/app/workers.go` (`LanzarWorker`), `harness/internal/cli/cli.go` (dispatch guard), `harness/internal/app/sesiones.go` (pane env); Create `harness/internal/app/testdata/agente-falso.sh`.
**Produces:** `func shimWorker(dir, nombre string) (pathDir string, err error)` writes `<state>/cajas/<nombre>/bin/claude` = `exec <orq> sandbox agente <nombre> -- "$@"`. The pane env gets `PATH=<that dir>:$PATH` and no `CLAUDE_CONFIG_DIR`. If config set one for the worker, warn once: `boxed workers ignore workers.*.claude.entorno.CLAUDE_CONFIG_DIR`. `cli` guard: when `sandbox.Dentro()`, only `gate`, `skill`, `version` run; the contract text (`app.Contrato`) says "commit, then say done in this pane" instead of "ask for verification on the channel"; anything else exits 2 with `inside the box: worker commands only`. Codex workers: refused while boxed (out of scope; step 7).
- [ ] Tests: exit-test rows 1 and 3 (`TestAgenteArbolDentro`, `TestAgenteSinCajaNoArranca`, `TestAgenteSenuelos`), `TestLanzarIgnoraConfigDirDelOwner`, `TestOrqDentroSinAutoridad`, `TestLanzarRespuestaPerdidaNoRelanza`.
- [ ] Add `validacion/aislamiento.sh` part 3 (rows 1–3 against the candidate binary). Run `make ci`, commit `app: workers launch inside the box; orq in the box refuses authority`.

### Task 5: Read-only deps in the worker box
**Files:** Modify `harness/internal/slice/caja.go` (`depsEn` → `func (r Repo) Deps(destino string) (ro []string, v []sandbox.Vinculo, aviso string)`), `harness/internal/app/caja.go` (use it), the test that has `TestPrepararDeps`.
- [ ] Tests: `TestDepsNpm`, `TestDepsPnpmWorkspace` (`apps/web/node_modules` bound), `TestDepsVenvRutaAbsoluta`, `TestDepsGoModCache`, `TestDepsLockfileDistintoNoSeMonta` (aviso set, nothing bound), row 2 `TestAgenteEditaYPrueba`, `TestAgenteDepsSoloLectura`; remove the `t.Skip` in `TestPrepararDeps`.
- [ ] Run, commit `slice: worker box sees main's deps read-only, per ecosystem`.

### Task 6: Return path checks and idempotency
**Files:** Modify `harness/internal/slice/clones.go` (`ingerir`: credential scan on `git log -p SHARojo..ref` before the ref is kept), `harness/internal/slice/slice_test.go`.
**Consumes:** Task 2 `Token.Contiene`. `Repo` gets the field `Secretos []credencial.Token`, set by `app` when it is loaded.
- [ ] Tests: rows 4–6 (`TestIngestaDentroDeAlcance`, `TestIngestaHistoriaReescrita`, `TestIngestaFueraDeAlcance`, `TestIngestaConCredencial`, `TestCajaResultadoValidoAvanza`, `TestCajaResultadoInvalidoReproducible`, `TestIngestaRepetidaUnaRef`, `TestCerrarRepetidoUnaFusion`); `TestCredencialNoSeFiltra` (§2) in `app`.
- [ ] Run `make ci`, commit `slice: reject bundles carrying the credential; repeat-safe ingest and close`.

### Task 7: Technical trial and report
**Files:** Create `validacion/caja-real.sh`, `docs/harness-v2/step2-trial.md`; Modify `docs/harness-v2/plan.md` (Checkpoint).
- [ ] Owner runs §5. Builder runs `orq caja credencial`, then `validacion/caja-real.sh`. Confirm the `usage` fields exist in the private transcripts, and confirm whether `setup-token` wrote into its temp dir.
- [ ] Write the trial record (§4 fields), using the report format in plan.md "Reporting". Commit `docs: step 2 technical trial`.

**Added from the step 1b review (Task 6):** `TestIngestaSoloRamaDelSlice`: the worker's clone moves its own `main` (`checkout main && reset --soft HEAD~1`, `branch --copy --force slice/01 main`) and creates other branches; after `ingerir`, the main repo's `main` and every ref except `refs/orq/…/NN` bookkeeping are unchanged, and only `slice/NN` was fetched. Linked worktrees are no longer used for workers.

**Added from the step 1b report (Task 6):** `verificar` refuses a slice in `BLOQUEADO`; the receipt states the number of tests the exam counted. Each gets a failing test first.

**Out of scope:** Codex or any second provider in the box (step 7); boxing the Orchestrator, Planner, Astra or Atena (they need a state broker); the worker message spool (workers report "done" through the pane; ingest is pulled by `verificar`); cgroup limits on CPU, RAM and disk; hidden exam variants; detecting a worker that fakes its own exam from inside; the Owner passphrase (step 3); installing over `~/bin/orq`; any change to the gate's parsing.
