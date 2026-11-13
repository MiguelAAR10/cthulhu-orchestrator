# Brief B1+B2 · the sandbox package and its acceptance test (branch v2/sandbox, worktree ~/worktrees/cthulhu-sandbox)

Read /tmp/claude-1000/-home-miguel-projects-MiguelAR10-Builder-AI-Edge-Runners-experiments-cthulhu-orchestrator/97993ac3-28de-4b5f-887a-6d3b8a94f645/scratchpad/briefs/COMMON.md first, then /tmp/claude-1000/-home-miguel-projects-MiguelAR10-Builder-AI-Edge-Runners-experiments-cthulhu-orchestrator/97993ac3-28de-4b5f-887a-6d3b8a94f645/scratchpad/spike-resultados.md (measured facts about bwrap, git, herdr and deps on this machine: build on them, do not re-derive) and the "Decisions from analysis (A3)" section of docs/harness-v2/plan.md.

## Why
A coding agent ("worker") runs as the same Unix user as orq. Today it can write orq's state and the main repo. The fix is containment: everything a worker runs, and every exam that executes worker code, runs inside bubblewrap with an allowlisted filesystem.

## You own
harness/internal/sandbox/ (new) · harness/internal/cli/sandbox.go (new) · one new `case "sandbox":` line in `Run` and one usage line in harness/internal/cli/cli.go · harness/internal/limits/ · templates/limits.json · validacion/aislamiento.sh (new).

## Result required
1. **Package `sandbox`** with this API (another worker will code against it, keep the names):
   ```go
   type Spec struct {
       RO   []string          // absolute paths bound read-only at the same path
       RW   []string          // absolute paths bound read-write at the same path
       Cwd  string            // working directory inside the box
       Net  bool              // false: no network at all
       Env  map[string]string // the only variables the process sees, plus PATH and HOME set by the package
   }
   func Disponible() error                                   // bwrap present and the self-test passes; cached per process
   func Command(ctx context.Context, s Spec, argv ...string) (*exec.Cmd, error)
   func Dentro() bool                                        // true when the current process is inside a box (marker)
   ```
   Rules, all proven by the spike: allowlist root (never `--ro-bind / /`); `--unshare-all --die-with-parent`; `$HOME`, `/run`, `/tmp` are tmpfs; `/mnt` absent; marker at /run/orq/sandbox made with `--ro-bind /dev/null`; bwrap itself started with a clean environment so /proc/1/environ leaks nothing; with Net true also bind /mnt/wsl/resolv.conf when /etc/resolv.conf points there. Base read-only set: /usr /bin /lib /lib64 /etc when they exist, plus the `sandbox.ro` list from limits.json (toolchains such as ~/.nvm, ~/.local/bin, ~/.local/lib, ~/.local/share/claude). There is no "off" switch: when bwrap is missing or the self-test fails, `Command` returns an error.
2. **Self-test** (`Disponible`): inside a box, writing outside RW fails, a path not allowlisted is absent, ~/.config/herdr/herdr.sock and /var/run/docker.sock are absent, the network is unreachable when Net is false, an outside pid is invisible, and `sudo -n true` fails.
3. **limits.json** gains an optional `sandbox` block `{"ro": [...]}`; `limits.Load` uses DisallowUnknownFields, so add the field there. templates/limits.json shows the block with the paths the spike needed.
4. **CLI**: `orq sandbox selftest` (prints each probe, exit 0/1) and `orq sandbox exec [--rw DIR]... [--ro DIR]... [--red] [--cwd DIR] -- CMD ARGS...` (runs CMD in a box, propagates its exit code).
5. **Unit tests** for argv construction and, when bwrap is present, the self-test probes. A missing bwrap makes the probe tests fail, not skip, unless ORQ_SIN_BWRAP=1 is set explicitly.
6. **validacion/aislamiento.sh**: the acceptance test for containment, written against the CLI above with a toy repo in a temp dir and a freshly built binary (`ORQ` env var, as validar-mvp.sh does). Part 1, must pass when you finish: from inside `orq sandbox exec --rw <worker-dir>`, each of these fails and leaves the target unchanged: write `<main>/.orq/slices/01.json`, write the limits file, remove the HALT file, write `<main>/.git/refs/heads/main`, overwrite a copy of the orq binary outside the box, connect to the herdr and docker sockets; and one positive control succeeds (commit inside the worker dir). Part 2, expected red until another activity lands, printed as PENDING and excluded from the exit code behind `--todo`: the forged-state attack of /tmp/claude-1000/-home-miguel-projects-MiguelAR10-Builder-AI-Edge-Runners-experiments-cthulhu-orchestrator/97993ac3-28de-4b5f-887a-6d3b8a94f645/scratchpad/forja.sh ends with main not advanced. Take forja.sh as the model for part 2.
