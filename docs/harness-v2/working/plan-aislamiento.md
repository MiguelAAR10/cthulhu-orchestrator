# Plan A3 · worker isolation for orq (harness-v2)

Planner · 2026-11-12 · read-only pass over README, REGLAS-HARNESS, slice.go, reporte.go, gate.go, v05.go, proyecto.go, herdr.go, sysinfo.go, orqdir.go, forja.sh.

## 1. Decision

**Rule:** anything a worker wrote (the agent itself, its code under the exam, git reading its repo) runs inside bwrap. Only data comes out: a git bundle, a report file, an exit code. orq state lives where no sandbox can write.

1. **Worker repo.** `git clone --shared --no-checkout` from main into `~/worktrees/<slug>-sNN[-zona]`. Main's `.git/objects` is bound read-only at the same path, so alternates resolve. The worker gets no linked worktree, so it has no `.git` file pointing at main, no shared refs and no shared config.
2. **Coming back.** Inside a short sandbox (clone read-only, `/out` writable), orq runs `git bundle create /out/w.bundle <rama> ^<sha_rojo>` and `git status --porcelain`; the output is plain text. Then, in main with orq's own config: `git bundle verify` and `git fetch w.bundle <rama>:refs/orq/sNN/<zona>`. orq never runs git with its cwd in a clone, because the clone's config (`core.fsmonitor`, `hooksPath`) is worker-controlled.
3. **Juntar and integrar need no working dir.** Both use `git merge-tree --write-tree` + `commit-tree` + `update-ref` (git 2.43 has these). `slice/NN` in main is a ref only orq moves.
4. **The exam always runs sandboxed** on `git archive <sha>` into an orq temp dir. `Aislado` becomes forced. That dir is the only writable path, network is off unless the slice declares `--red local`, and deps are bound read-only. The same applies to `--sano` and to the re-run in `integrar`.
5. **Sandbox root is an allowlist, not `--ro-bind / /`.** Bound: `/usr /bin /lib* /etc` read-only, plus the toolchains listed in `limits.json` (`sandbox.ro`). `$HOME` and `/run` are tmpfs; `/mnt` and docker are absent. Flags: `--unshare-all --die-with-parent`, a marker `/run/orq/sandbox`, and `env -i` plus an allowlist of variables. No `--new-session`, so the TUI keeps its tty; that is safe because TIOCSTI is off on this machine.
6. **Network.** The agent needs its model API. Each worker gets its own netns. Inside, `orq sandbox puente` listens on 127.0.0.1 and forwards to a unix socket served by orq outside. That outside proxy is a stdlib CONNECT proxy with a host allowlist (Anthropic or OpenAI by default; `--red total` lifts it). The agent gets `HTTPS_PROXY`.
7. **Agent home.** Per-worker `HOME` and `CLAUDE_CONFIG_DIR` / `CODEX_HOME` under the state dir, seeded by orq: a long-lived token in env (`CLAUDE_CODE_OAUTH_TOKEN`, or an API key for Codex), a `settings.json` with the gate hook, and read-only skills. The real `~/.claude*`, `~/.codex` and `~/bin/orq` are never writable. Today a worker can rewrite them, and they run outside any sandbox.
8. **Launch through herdr.** The pane's shell is replaced with `exec orq sandbox shell <worker>` (via `herdr pane run`). Then `herdr agent start` types `claude …` into a shell that is already inside the box. Fallback: a PATH shim. H4 decides.
9. **State.** `slices/`, `slices.jsonl`, `recibos.jsonl`, `salidas/`, `cuarentena.jsonl`, locks and the worker registry move to `$XDG_STATE_HOME/orq/p/<slug>-<hash8(root)>/`. Memory, the channel and handoffs stay in `.orq/`, which is read-only inside the box. Worker messages go to a writable spool (`/run/orq/buzon`); orq ingests them, with the sender taken from the spool path.
10. **Worker count.** Workers are counted from the registry: `{name, slice, pid, starttime, clone}`, written by `lanzarWorker`. Live means the pid exists and its starttime matches. The gate's "is a worker" check becomes: linked worktree (legacy) **or** the sandbox marker exists.

**Rejected:**
- `git clone --local`: it hard-links objects, so a worker that runs `chmod` and writes in place corrupts main's object store. (`prepararDeps`' `cp -al` has the same flaw today: a worker can edit the Owner's `node_modules` files.)
- `git fetch <clone>` from main: it runs `upload-pack` inside a repo the worker controls. A passive bundle file is enough.
- `--ro-bind / /`: a read-only mount still allows `connect()` to unix sockets. That exposes `herdr.sock` (drive any pane), `docker.sock` (the user is in the docker group, which is root) and the dbus bus.
- Network fully off for the worker: Claude Code and Codex cannot reach their API. Sharing the host netns instead exposes localhost services and abstract sockets.
- Copying `.credentials.json` / `auth.json` into the box: if OAuth refresh tokens rotate, the Owner's sessions and the worker would invalidate each other. Long-lived tokens or API keys avoid that (H5).
- A PATH shim as the primary route: `.zshrc` can reset PATH, and the shim only covers the first `claude` command. A sandboxed pane shell covers everything typed into that pane.
- HMAC-sealed state: once no sandbox can write the state dir, a seal adds nothing against workers. It does not help against unsandboxed roles either, since they hold the same key.
- Hardening the gate regex further: it stays as an early, friendly "no", not as the guarantee.

## 2. Threat model after the change (workers only)

**A worker can no longer:**
- write main's tree, `.git` (refs, config, hooks, objects), `.orq` or the state dir;
- write `limits.json`, HALT, `~/bin/orq`, the real Claude or Codex config, or another worker's clone;
- reach `herdr.sock`, `docker.sock`, dbus, the WSL interop socket or `/mnt/c`;
- see or signal outside pids, or use `sudo` (bwrap sets no_new_privs).

**A worker still can, honestly:**
- **Fake its own exam from inside.** Its code runs under the test, so it can monkeypatch the runner, write `out.tap` or call `exit 0`. Isolation contains this but does not detect it. That needs hidden variants or an out-of-process oracle (README "Next").
- **Read everything bound read-only:** the whole history through alternates, `.orq` memory, the channel and other workers' messages, deps, toolchains and its own token.
- **Send data to the allowlisted API hosts** (inherent to using a cloud model). With `--red total`, it can send anywhere.
- **Spawn sub-agents inside its own box** (cost, not integrity). It can also fill the disk or burn CPU/RAM: there are no cgroups, so `orq watch` and the quota are the only brake.
- **Prompt-inject the Orquestador or Atena** through messages, commit messages and file contents.
- **Ship code that runs unsandboxed after acceptance** (npm `postinstall`, `.envrc`, `.vscode/tasks.json`). Acceptance by a human is the gate there. B4a adds control-plane paths (`.claude/ .codex/ .envrc .githooks/ .gitattributes`) to `protected` by default, so a worker cannot plant hooks for the main sessions.
- **Escape through kernel, bwrap or userns bugs**, or through WSL interop if H3 fails.

**Out of scope:** the Orquestador, Planner, Astra and Atena stay unsandboxed and can still run `forja.sh`'s state edit. `rolArgs` even gives Astra write access to the parent of root. Sandboxing them needs a state broker (follow-up, not in this plan).

## 3. Activities (land after A1 and A2 are merged; rebase onto both)

| ID | Result · exit test | Files | Depends on | Par/serial | Size | Model |
|---|---|---|---|---|---|---|
| B0 | Spike: runs H1–H9 in scratch and writes `validacion/sandbox-spike.md` with PASS/FAIL per hypothesis. Exit: file present; H3, H4 and H5 decided. | scratch only + that md | — | serial, first | S | sonnet |
| B1 | Red acceptance test `validacion/aislamiento.sh`. It extends forja.sh: a fake worker runs inside the real sandbox and tries the five writes (state, limits, HALT, `.git` refs, `~/bin/orq` copy), herdr.sock and docker.sock, and a fake receipt. Exit: today red; green only when every attempt fails and `cerrar --aceptado` refuses. | validacion/aislamiento.sh | B0 | serial | S | opus |
| B2 | `internal/sandbox`: `Spec{RO, RW, Tmpfs, Net, Env, Cwd, Home}` → bwrap argv; `Exec`, `Run(ctx)`; `SelfTest()` runs probes and fails closed when bwrap is missing or a probe escapes. CONNECT proxy + `puente`. Exit: `go test ./internal/sandbox` covers socket, write, net and pid probes (skipped if no bwrap). | internal/sandbox/* (new) | B0 | parallel with B3, B4 | M | opus |
| B3 | State dir out of the repo: `orqdir.StateDir(root)`; `slice.Repo.Estado`; rewrite `dirSlices estadoPath logPath recibosPath CuarentenaPath candado save migrar`; `proyecto.slices()`; `memVigente`; metricas readers. Import of legacy `.orq/slices` (§5). Exit: existing slice tests green with `ORQ_STATE` temp dir; import test. | slice.go (path funcs only), orqdir.go, proyecto.go `slices()`, v05.go `memVigente`, slice/metricas.go | — | parallel with B2, B4 | M | sonnet |
| B4 | Worker registry `internal/registro` (Alta, Baja, Vivos with pid+starttime); `sysinfo.StartTime`; `cli.go` `env.Workers` and `orq status` use it; `gate.IsLinkedWorktree` → `gate.EsWorker` (legacy worktree OR sandbox marker). Without this, the protection silently turns off in clones, because their `.git` is a dir. Exit: unit tests; the gate test denies a protected write from a clone with the marker. | internal/registro/* (new), sysinfo.go, cli.go (Workers wiring), gate.go (`IsLinkedWorktree` only) | — | parallel with B2, B3 | S | sonnet |
| B5a | Clones + bundles: `Abrir` (clone instead of `worktree add`), `Sumar`, `reusar`, `Podar`, `podable`, `limpiarEquipo`, `quitarWorktree`, `libre`, `EnUso`, `Huerfanos`, `Alinear` (diff on `refs/orq/…`), `Juntar` (merge-tree), new `ingerir(e, w)`; delete `prepararDeps` in favour of `Deps(root)` read-only binds. Default control-plane protected paths. Exit: slice tests rewritten; the team merge test passes without any linked worktree. | slice.go (listed funcs) | B2, B3 | serial | L | opus |
| B5b | Sandboxed exam: `correr`/`RunCmdT` take a `Runner`; `Verificar` (ingest → archive → sandbox; dirty check from the sandboxed status), `integrar` (merge-tree + archive + sandbox), `--sano` in `Abrir`; read the report with `O_NOFOLLOW` and a 10 MB cap. Exit: an exam whose code tries to write `$ORQ_STATE` gets EROFS and the receipt stays honest. | slice.go `Verificar integrar correr`, reporte.go | B5a | serial | M | opus |
| B6a | Launch: `lanzarWorker` (registry entry, seed home and config dir, `herdr pane run … orq sandbox shell`, then `herdr.Start`), new `cli/sandbox.go` (`orq sandbox shell|exec|puente|selftest`); `workerArgs` (drop `--add-dir <git-common>`; Codex sandbox mode per H6); `contrato` and `datosRelevo` text (clone path, "only commits count"). Exit: `validacion/validar-mvp.sh --real` green with the worker inside bwrap. | v05.go `lanzarWorker workerArgs contrato datosRelevo`, cli/sandbox.go (new), herdr.go (`PaneRun`) | B2, B4, B5a | serial | M | opus |
| B6b | Worker channel: `orq msg send` inside the box writes `/run/orq/buzon/*.json`; `orq msg leer`, `status` and the Orquestador ingest it, with `De` taken from the spool owner. Exit: canal test with spool ingest; a spoofed `De` is ignored. | internal/canal/*, cli.go `runMsg` | B3 | parallel with B5a, B5b | S | sonnet |
| B7 | Compatibility + docs: legacy path (`Legado`) for slices opened by 1.0.0, `orq slice actualizar NN`, `orq doctor` checks for the sandbox; README guard rows, REGLAS "Qué cuenta como worker", `templates/limits.json` `sandbox` block. Exit: B1 green; `e2e-v05.sh` green; a legacy fixture closes. | slice.go (`Load` legacy branch), cli `doctor`, docs, templates | B5b, B6a | serial | M | sonnet |

Functions touched after A1/A2: `gate.IsLinkedWorktree` (A1's file, one function); `proyecto.slices()` and `slice/metricas.go` readers (A2's files, paths only). No verdict or open-flag logic from A2 is rewritten; `Verificar` keeps A2's verdict code and only swaps the dir and runner.

## 4. Hypotheses to validate first (B0; each under 10 min, toy repo in scratch, fresh `go build -o $S/orq`, never `~/bin/orq` or the real repo)

**Verified by reading this machine:** bwrap 0.9.0; `dev.tty.legacy_tiocsti=0`; the user is in the `docker` and `sudo` groups; `herdr.sock` is at `~/.config/herdr/`; WSLInterop binfmt has flags `PF`, so `/init` is preopened and hiding it does not stop the handler; the agents are `~/.local/bin/{claude,codex}` (Claude → `~/.local/share/claude/versions/…`); `CLAUDE_CONFIG_DIR=~/.claude-alt`; herdr says `agent start` needs "the pane at its interactive shell prompt" and "detects the agent in the same terminal".

**Must be tested:**
- **H1 Allowlist root works.** `bwrap` with the §1.5 binds runs `claude --version`, `codex --version`, `node -v`, `go version`, `git --version`. Each missing bind becomes a `sandbox.ro` entry.
- **H2 Sockets.** With `--ro-bind / /`, `curl --unix-socket /var/run/docker.sock http://x/version` answers. With the allowlist root it fails; same for `herdr.sock` and `$XDG_RUNTIME_DIR/bus`. This proves the reason allowlist root was chosen.
- **H3 WSL interop is dead inside.** Bind only `/mnt/c/Windows/System32/whoami.exe` → `/tmp/w.exe` (standing in for a PE the worker wrote), with `env -i` and `/run` as tmpfs, then run it. It must fail. If it runs, it is a blocker: there is no namespace fix, so the Owner decides `[interop] enabled=false` in `/etc/wsl.conf` or accepts the risk.
- **H4 herdr.**
  - (a) A pane whose shell is `exec bwrap … zsh -i` makes `herdr agent start --kind claude` reach "ready" within 90 s.
  - (b) `herdr agent prompt NAME` and the status/`--wait` paths work while `herdr.sock` is hidden inside. Check whether herdr's Claude hooks (`settings.json.bak-herdr` hints they exist) need the socket.
  - (c) If (a) fails for lack of rc/OSC133 prompt markers, retry with a read-only `~/.zshrc`, then with the PATH-shim fallback.
- **H5 Claude Code in the box.** Private `CLAUDE_CONFIG_DIR` + `CLAUDE_CODE_OAUTH_TOKEN` (from `claude setup-token`) + `HTTPS_PROXY` to a 40-line Go CONNECT proxy behind the bridge, in its own netns: `claude -p "reply ok" --model haiku` succeeds. List the files it wrote (`find -newer`): `.claude.json` location, daemon, sessions. Afterwards the Owner's session in `~/.claude-alt` still authenticates.
- **H6 Codex in the box.** `CODEX_HOME` private + API key + proxy: `codex exec "reply ok"` succeeds. Then check whether `-s workspace-write` works nested in bwrap (userns inside userns). If not, use `-s danger-full-access` inside, since bwrap is the boundary.
- **H7 git path.**
  - `clone --shared` with main's objects bound read-only: commits work inside.
  - `bundle create <rama> ^<base>` inside, then `verify` + `fetch` in main into `refs/orq/…`.
  - Set hostile `core.fsmonitor` / `core.hooksPath` / `uploadpack.*` in the clone's config: nothing runs (marker file absent).
  - `merge-tree --write-tree` gives the same tree as a worktree merge.
- **H8 Deps.**
  - `node_modules` bound read-only at `<clone>/node_modules`: a vitest/jest toy test runs.
  - `.venv` bound read-only at its absolute path: `pytest` runs.
  - `npm i` gets EROFS.
  - `--overlay-src … --tmp-overlay` works unprivileged on kernel 6.18 (needed for slices that change deps).
- **H9 Privilege.** Inside: `sudo -n true` fails (no_new_privs); `kill -0 <outside pid>` and `cat /proc/1/environ` show only the box; `ls /mnt` is empty.

## 5. Migration and rollback

**Install side by side.**
- Build to `~/bin/orq-2`, never over `~/bin/orq`.
- Try it in a toy project whose `.claude/settings.json` hook calls `$HOME/bin/orq-2 gate`.
- Swap `~/bin/orq` only when B1 and the e2e are green and `orq-2 doctor` passes `sandbox selftest`.
- Keep `~/bin/orq-1.0.0`.

**Slices in flight from 1.0.0 (other project).** The new binary reads `<root>/.orq/slices/NN.json` when the state dir lacks it.
- **Import.** Copy it to the state dir with `Legado: true, Importado: ts`, and rename the original to `NN.json.migrado`. Import is non-destructive and idempotent.
- **VERIFICADO.** A legacy state may already be forged (forja.sh), so the receipt is dropped and the slice goes back to `IMPLEMENTANDO`. It needs one fresh `orq slice verificar`, which now runs the exam sandboxed. Import prints `cmd` and the bundle so the Orquestador can check them against the plan.
- **INTEGRANDO/CERRANDO.** Continue as is: `integrar` re-runs the exam in the sandbox; `CERRANDO` only publishes.
- **Legacy branch.** While `Legado`, `Juntar`/`Verificar` read the linked worktree via the old code. That code is kept, but the exam still runs sandboxed. A running agent cannot be moved into a box retroactively.
- **`orq slice actualizar NN`.** The slice's branch already lives in main's refs, so this creates a clone from its tip, retires the old worktree when `EnUso` is empty, and clears `Legado`. `lanzar` on a legacy slice runs it automatically.

**Compatibility window.**
- From the swap, the legacy path accepts existing slices until they close; it never opens new ones.
- `orq status` lists legacy slices with a warning.
- The legacy code is deleted in the release after the in-flight project reports zero legacy slices, and no later than 30 days.

**Rollback.**
- Point the hook and `~/bin/orq` back to 1.0.0.
- Rename each `NN.json.migrado` back to `NN.json` (one loop, documented in `orq doctor --rollback`).
- 1.0.0 cannot drive clone-based slices: close or reject them first with the new binary. Nothing is lost, because the work is in `refs/orq/sNN/*` and `slice/NN` in main.
- The state dir is left in place, unused by 1.0.0.
- A failed `sandbox selftest` always blocks launches (fail closed). It never falls back to running unsandboxed. The only bypass is an explicit `limits.json` `"sandbox": "off"`, which `orq status` shows in red.
