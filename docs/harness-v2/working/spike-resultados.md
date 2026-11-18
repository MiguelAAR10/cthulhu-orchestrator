# B0 · sandbox spike results (2026-11-13)

Scratch only, under `.../scratchpad/spike`. bwrap 0.9.0, git 2.43.0, kernel 6.18.58.1-microsoft-standard-WSL2.
Helper: `box.sh` (allowlist root: `--unshare-all --die-with-parent`, ro `/usr /etc`, tmpfs `/tmp /home /run`, ro `~/.nvm ~/.local/{bin,lib} ~/.local/share/{claude,uv,codex-panel}`, marker `/run/orq/sandbox`, `env -i`).

| H | Result | Note |
|---|---|---|
| H1 | PASS | all five run; `codex-cli` is broken on the host itself |
| H2 | PASS | docker, herdr and bus sockets unreachable in the allowlist root |
| H3 | PASS | the PE does not run; WSL handler fails (`UtilGetPpid ... /proc/1/stat ... (bwrap)`) |
| H4(a) | PASS via shim, FAIL via `exec bwrap ... zsh` | `agent start` ready in 4.3 s with the PATH shim |
| H4(b) | PASS | `agent prompt --wait` worked with `herdr.sock` hidden |
| H4(c) | PARTIAL | `+m` fixed the pgid but not "not an available shell"; the PATH shim works; read-only `.zshrc` not tried |
| H5 | NOT RUN (variant PASS) | the real config dir rw + `--share-net` works; this is not the isolated-token H5 |
| H6 | NOT RUN | no credentials supplied |
| H7 | PASS | clone, commit, bundle, fetch, hostile config and merge-tree all behave |
| H8 | PARTIAL | ro `node_modules` and ro `.venv` PASS; `--overlay-src` FAIL (bwrap 0.9.0) |
| H9 | PASS | but see the `/proc/1/environ` leak under H1/H9 |

## H1 · allowlist root works: PASS
Command: `./box.sh env -i PATH=$PATH HOME=/home/miguel bash -c "<cmd>"` for each tool.
Decisive output: `2.1.323 (Claude Code)`, `v24.12.0`, `go version go1.25.8 linux/amd64`, `git version 2.43.0`, `codex-cli 0.162.1`.
`sandbox.ro` entries needed beyond `/usr /bin /lib* /etc`:
- `~/.nvm`
- `~/.local/bin`
- `~/.local/lib` (`~/.local/bin/go` is a symlink to `~/.local/lib/go` → `go-1.25.8`)
- `~/.local/share/claude`
- `~/.local/share/codex-panel`

Findings:
- `codex` is a shell script that runs `python3 .../codex-panel/panel.py`.
- `~/.local/bin/codex-cli` fails on the host as well: `bad interpreter: .../uv/tools/codex-cli/bin/python: no such file`.
- Network name resolution: `/etc/resolv.conf` is a symlink to `/mnt/wsl/resolv.conf`. With `--share-net`, DNS only works if that file is bound too (`--ro-bind /mnt/wsl/resolv.conf ...`); without it, `getaddrinfo EAI_AGAIN`.
- **Leak:** `cat /proc/1/environ` inside the box shows the host environment, because pid 1 is bwrap itself (`SHELL=/usr/bin/zsh`, ...). That would leak any token in the launcher's environment.
  - Workaround verified: `env -i PATH=... bwrap ...`. `SPIKE_SECRET` went from 1 hit to 0, and pid 1 had 3 env vars.
- **Gotcha:** `--file 0 <path>` reads stdin until EOF, so it hangs when stdin is a tty. Use `--ro-bind /dev/null /run/orq/sandbox`. This cost a hang in the pane.

## H2 · sockets: PASS
Control: `bwrap --ro-bind / / ... curl --unix-socket /var/run/docker.sock http://x/version` → `{"Platform":...,"Version":"29.1.3",...}`, and `herdr.sock CONNECT OK`.
Allowlist root (`python3 h2.py` in the box):
```
/home/miguel/.config/herdr/herdr.sock [Errno 2] No such file or directory
/var/run/docker.sock [Errno 2] No such file or directory
/run/user/1000/bus [Errno 2] No such file or directory
```
`curl` to docker.sock → rc=7.
Note: a first attempt without `</dev/null` hung (stdin), not a sandbox issue.

## H3 · WSL interop dead inside: PASS
Command: `./box.sh --ro-bind /mnt/c/Windows/System32/whoami.exe /tmp/w.exe env -i PATH=$PATH /tmp/w.exe </dev/null`
Host control prints `miguel-asus-zen\arias`.
Boxed: no whoami output, rc=1, only `<3>WSL (2 - ) ERROR: UtilGetPpid:1368: Failed to parse: /proc/1/stat, content: 1 (bwrap) S 0 ...`.
Reason: the interop handler cannot parse pid 1's stat in the pid namespace. This depends on `--unshare-pid`; keep `--unshare-all`. Not a blocker. Not tested: a real PE other than whoami.exe, or a box sharing the host pid ns.

## H4 · herdr (tab `spike` in w3X, own panes only)
The agent kind used was `claude --model haiku`, with the real config dir rw and `--share-net` (the H5 variant below).

- **(a) `exec bwrap ... zsh -i` as the pane shell → FAIL.**
  - Command: `herdr pane run <pane> "exec pane-box.sh"` then `herdr agent start spikeagent --kind claude --pane <pane> --timeout 90000 -- --model haiku`.
  - Decisive line: `{"error":{"code":"agent_pane_busy","message":"agent target pane w3X:p7 is not an available shell"}}`.
  - Cause 1: interactive zsh with job control moves into its own pgrp; `process-info` showed `foreground_process_group_id` 1498181 ≠ `shell_pid` 1493007 (the bwrap).
  - Retried with `zsh -i +m` so the pgid matches (`foreground_process_group_id` = `shell_pid` = 1498555): still `not an available shell`. The pane's `shell_pid` is bwrap, not a shell, and herdr seems to reject that.
  - Not tried: a read-only `.zshrc` with OSC133 markers. With an empty HOME, zsh also shows its first-run menu (needs `.zshrc` or `zsh -f`).
- **(c) PATH-shim fallback → PASS.**
  - Setup: normal host shell pane, `export PATH=$S/shim:$PATH; hash -r`; `shim/claude` is `exec env -i bwrap ... /home/miguel/.local/bin/claude "$@"`.
  - Command: `herdr agent start spikeagent --kind claude --pane w3X:p9 --timeout 90000 -- --model haiku`.
  - Decisive line: `"agent_status":"idle" ... "interactive_ready":true` after 4.348 s total.
  - Caveat from the plan: the shim only covers the first `claude`; a worker that types `claude` again, or an absolute path, bypasses it. `agent start` itself typed a bare `claude`, which resolved to the shim.
- **(b) prompt / wait with `herdr.sock` hidden → PASS.**
  - Command: `herdr agent prompt spikeagent "Run this in bash and report its output only: ls /run/orq; ..." --wait` returned `"agent_status":"done"` in 18.6 s.
  - `agent read` showed `● sandbox` and `done`, so the agent ran inside the box (marker present).
  - `herdr integration status` says `claude: not installed (.../herdr-agent-state.sh)`: detection is by terminal screen, not a hook, so no socket is needed for Claude. Codex's hook is installed (`codex: current (v10)`); not tested whether it needs the socket.
- Accident: closing the last pane (`w3X:p8`) closed the tab, so I recreated one tab `spike` (t5) and closed it at the end. Other tabs in w3X were not touched (the `herdr tab list` afterwards shows only `🐙 orq · harness-v2` and `harness-v2 · workers`).

## H5 · variant only (clearly NOT the real H5): PASS for the variant; real H5 NOT RUN
No tokens were created and no credential file was read or copied.
Variant: the real `CLAUDE_CONFIG_DIR=/home/miguel/.claude-alt` bound **read-write** in the allowlist-root bwrap with `--share-net` (host netns, no proxy, no private home token).
Command: `timeout 100 shim/claude -p "reply ok" --model haiku </dev/null`
Decisive output: `ok` (8.9 s).
Files under the config dir newer than the marker (`find ~/.claude-alt -newer ref.t -type f`):
- `.claude.json`
- `state/mcp-discover-verdicts.json`
- `session-env/<id>/sessionstart-hook-0.sh`
- `plugins/store/cc-plugin-agents-md_builtin-*.json`
- `projects/<cwd-slug>/<id>.jsonl` (this session's transcript; the list also contains other live sessions' transcripts and `plugins/synced/.../.last-complete-round`, which are noise from other sessions)

Conclusion: with the real dir rw, a worker could rewrite `settings.json`, hooks and credentials, as the plan warns. This variant proves only that Claude works in an allowlist-root bwrap with a shared netns. It says nothing about OAuth rotation, the isolated token, the proxy path or the `.claude.json` location in a private dir.

## H6 · NOT RUN
No credentials were provided.

## H7 · git path: PASS
Setup: `main` toy repo; `git clone -q --shared --no-checkout main clone1`; `clone1/.git/objects/info/alternates` → main's objects.
- Inside the box (main's `.git/objects` ro at the same path, clone rw, `/out` rw): `git checkout -b w1 <base> && commit && git bundle create /out/w.bundle w1 ^<base> && git status --porcelain` → `inside-ok`.
- In main: `git bundle verify ../out/w.bundle` ("The bundle uses this hash algorithm: sha1") then `git fetch ../out/w.bundle w1:refs/orq/s01/w1` → `a1d2189 work` / `2f76ec4 base`.
- Hostile config: I set `core.hooksPath`, `core.fsmonitor`, `uploadpack.packObjectsHook` and `core.pager` in the clone to scripts that touch a marker file. After main-side ops (`fetch` from the bundle, `bundle verify`) no marker: `no PWNED marker after main ops (PASS)`.
- Control: `git -C clone1 status` (cwd in the clone) **did** run the fsmonitor script: `PWNED-fsm`. This confirms the rule "orq never runs git with cwd in a clone".
- `git merge-tree --write-tree main w1` → `c15ba042ba73e9780fd60d90de94588c7c0ede6a`; a real `git merge w1` → tree `c15ba042ba73e9780fd60d90de94588c7c0ede6a`. Same tree.
- Environment used: `GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null` and fixed author vars.

## H8 · deps: PARTIAL
- **node: PASS.** Box with `--bind clone clone --ro-bind proj/node_modules clone/node_modules`: `node --test a.test.js` → `ℹ pass 1 / ℹ fail 0`.
  - `touch node_modules/x` → `Read-only file system` (rc 1); overwriting `node_modules/dep/index.js` → `Read-only file system`.
  - Plain `node --test` with no file argument printed nothing in my first runs; passing the test file works. Not investigated.
  - `npm i` with `--share-net`: `EAI_AGAIN` (DNS) because `/mnt/wsl/resolv.conf` was not bound in `box.sh`; with no net it cannot succeed either, so EROFS on `npm i` was shown by the direct writes above, not by `npm i` itself.
- **python: PASS.** A venv at an absolute path bound ro (`--ro-bind $V $V`): `$V/bin/python -m pytest -q -p no:cacheprovider` → `1 passed in 0.01s`; `touch $V/x` → `Read-only file system`.
  - pytest was installed into the venv beforehand on the host (`pip install -q pytest`, network, outside the box).
- **`--overlay-src … --tmp-overlay`: FAIL.** `bwrap: Unknown option --overlay-src` (bwrap 0.9.0; overlay flags need ≥ 0.11).
  - Kernel overlay is available: `unshare -Urm sh -c 'mount -t overlay ...'` → `kernel overlay via unshare OK`. Not tested inside bwrap.
  - Cheapest verified workaround: copy the deps into the rw clone with `cp -a` (not `cp -al`). A write to the copy succeeded, and the original stayed `module.exports=()=>42`. Cost: disk and time for big trees.

## H9 · privilege: PASS
Command (inside the box): `sudo -n true; kill -0 <outside pid>; cat /proc/1/environ; ls /mnt; grep NoNewPrivs /proc/self/status; ps -e | wc -l`
- `sudo: The "no new privileges" flag is set, which prevents sudo from running as root.` → rc 1
- `bash: line 1: kill: (1478471) - No such process` → rc 1
- `pid1 cmd: bwrap --unshare-all ...` (pid 1 is the box's bwrap)
- `ls: cannot access '/mnt': No such file or directory`
- `NoNewPrivs: 1`; `ps -e | wc -l` → 5
- **Leak found:** pid 1's environ is the launcher's environment (see H1). Fix: launch bwrap under `env -i`.

## FAIL workarounds summary
- H4(a): use the PATH shim (verified, 4.3 s). Both `exec bwrap` and `+m` failed. Read-only `.zshrc` untested.
- H8 overlay: `cp -a` copy (verified). Kernel overlay under `unshare -Urm` works but is not verified inside bwrap; a newer bwrap (≥ 0.11) was not tried.
- `/proc/1/environ` leak: `env -i bwrap ...` (verified).
- DNS under `--share-net`: bind `/mnt/wsl/resolv.conf` (verified; the interactive and `-p` runs reached the API).

## Blockers / decisions for the Owner
- None blocking H3 (interop is dead inside under `--unshare-all`).
- H5 and H6 need credentials: isolated token, proxy, Codex API key and the `-s workspace-write` nested test were not run.
- H4 consequence: the plan's primary route (pane shell replaced with `exec orq sandbox shell`) does not work with herdr 'agent start'; the shim is the route that works, with the weakness above.
