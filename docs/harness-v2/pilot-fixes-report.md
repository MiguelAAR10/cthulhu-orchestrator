# orq2 pilot fixes — report

Builder: Opus. Plan: `pilot-fixes-plan.md`, plus Owner addendum P7/P8/P9/P12. Branch `v2/build`.
Order kept: P6 → P1 → P2 → P5 → P3 → P7 → P8 → P9 → P12, one commit each. `orq-init.golden` and `cli/version_test.go` untouched.

| Item | Commit | Red seen | Green |
|---|---|---|---|
| P6 own-clone count | `89177c6` | build fail: `undefined: EsWorkerPropio`, `EsWorkerContable` | gate + app tests |
| P1 uv checks in init | `caa3e01` | build fail: `undefined: ChecksPara` | 5 `checks_uv_test.go` tests |
| P2 `check --caja` uv | `268a132` | `TestCheckCajaUv`: types said "corre … no venv (exit 3)" instead of the refusal; slice: `undefined: SondarUv` | slice + cli tests |
| P5 version | `9262893` | stdout `"1.0.0 revision=unknown…"` | `2.0.0-pilot …`, header `orq2 2.0.0-pilot ·` |
| P3 parallel walk | `307ffb7` | no red: pure perf refactor, equivalence test is green before and after (see below) | `-race` green |
| P7 optional max_workers | `007d2cb` | build fail `undefined: SinTope/TopeWorkers`; gate test nil-pointer panic | limits, recursos, gate, cli status |
| P8 Astra guide | `75bb0be` | `TestGuiaOrquestadorAstraSinCodex`: orq2 guide still said "Codex quota" | orq byte-identical, orq2 names the role |
| P9 plain card | `277252d` | `TestGuiaTarjetaClaraParaOwner`: orq2 planner skill lacked the card rules; orchestrator still capped Historia at 3 lines | orq skills byte-identical |
| P12 RAM warning + `doctor --ram` | `b6c24e0` | build fail: `undefined: AvisoRAM`, `ProcesosPorRSSDe`, `doctorRAM`, `env.AvisoRAM`; then `open templates/limits-orq2.json` | limits, app, gate, sysinfo, cli |

Final: `gofmt -l harness` empty · `go vet ./...` ok · `go test -race ./...` exit 0; after P9/P12: all packages ok except the host-dependent `TestStatusMuestraMaquinaYCapacidad` (see P9) · `make ci`: see the last section.

## P6 — orq2 counts only its own clones
`gate.EsWorkerPropio(dir)`: true only if a `.git` directory upward holds `orqdir.MarcaClon`. No linked-worktree branch,
relative/empty dir → false. `app.EsWorkerContable()` picks it under orq2, `gate.IsWorker` under orq; `IsWorker` unchanged.
Tests: (1) worktree, (2) plain repo, (3) marked clone, (4) repo nested in it → orq2 counts 3,4; orq counts 1,3,4.
Gate: 5 foreign worktree agents, `max_workers=4` → orq2 allows, orq denies.
- UNVERIFIED (a) closed: the installed orq 1.0.0 (`~/bin/orq`, built 2026-10-29) contains no `orq-clon` string
  (`grep -ac` → 0); the marker came in `e583e78` (2026-11-18). Clones made by an orq binary built from *this* tree
  also carry it and would count under orq2 — both are orq workers, acceptable.
- UNVERIFIED (b) closed: every slice worktree is a `crearClon` (`slice/nucleo.go:256`, slice without team) and every
  team member too (`slice/equipo.go:130`); no `git worktree add` in the code. Workers launch with cwd `e.Worktree`
  or `e.Equipo[i].Worktree` (`app/workers.go:137-147`).
- Owner's call still open: `max_workers` in `~/.config/orq2/limits.json` is still 12 (not touched; with P7 it can be removed).

## P1 — init proposes uv checks (orq2 only)
`perfil.ChecksPara(root, python)`; `Checks(root)` = `ChecksPara(root, false)`. The caller is `cli/v03.go` (not `app`),
which passes `identidad.EsOrq2()`; so the `arch_test` question did not arise. Commands exactly as the plan.
Plan reconciliation: test (4) "no uv.lock → today's omission" vs scope "clearer motive": under orq2 a pyproject without
uv.lock gets `Python sin uv.lock: escribe el comando en .orq/config.json`; orq keeps today's text.
Known cosmetic: `init` writes `&&` as `&&` in config.json (Go JSON escaping, same as any check with `&&`); valid JSON.

## P2 — `check --caja` on a uv check (orq2 only)
Static: no `--no-sync` → `uv sincroniza en la caja y no hay red; usa \`uv run --offline --no-sync\``; no `<root>/.venv` →
`falta .venv; corre \`uv sync\` en el checkout principal`. Then `slice.Repo.SondarUv`: `copiaDe("HEAD")` +
`correrEn(copy, "<prefix up to the tool> --version", 60s)`. The output line says it does not prove every import resolves.
Added: the word after `uv run` is not looked up on the host PATH (`slice.HerramientasUv`): it comes from `.venv`, and the
host lookup gave a false "fuera de $HOME" for `pytest` in `~/.local/bin`. `depsEn`, `Spec`, `entornoExamen` unchanged.
Bwrap skip helper in `sandbox_test.go` is `exigeBwrap` (UNVERIFIED closed). `dashboard/` limit recorded as K8.
Found in the pilot proof — K9 in `backlog-next.md`: the probe runs the tool by name, so when the `.venv` script cannot run
in the box (its shebang outside the mounts) a same-named host tool on a mounted dir answers. Seen only in a clone whose
`.venv` was a symlink to the Owner's checkout (`pytest 9.0.3` from `~/.local/bin` instead of the venv's 9.1.1; mypy
"Failed to spawn"). In a real checkout `.venv` is bound at its own path and shebangs resolve. Not fixed (out of the plan).

## P5 — version
`VersionOrq2 = "2.0.0-pilot"` for `version` and the usage header (`orq2 2.0.0-pilot · …`). UNVERIFIED closed:
`slice.Version` enters receipts (`Recibo.Runner`, `slice/nucleo.go:379`) — left alone.

## P3 — watch timing (`~/worktrees`, Owner's orq2 limits copied to a temp ORQ_LIMITS, temp ORQ_STATE)
| Run | Time | scratch_gb |
|---|---|---|
| before, cold cache | 131.97 s | 57.285181749 |
| before, warm | 27.62 s | 57.285198155 |
| after, first run (partly warm) | 19.67 s | 57.285202162 |
| after, warm | 11.44 s | — |
Totals differ by ~20 KB: live files in `~/worktrees` changing between runs, not the walk. UNVERIFIED (what costs) answered
partly: most of the cold 132 s is the cold page cache (directory reads), not stat; warm, the pool gives 2.4×. A cold "after"
was not measured (dropping caches needs root). Under 20 s warm; cold could still exceed it — no exclusions made (Owner's call).
No red: the test compares against a copy of the old serial walk (nested dirs, hardlink across two top-level children,
symlink, file root) 20 times under `-race`; green before and after by design.

## P7 — max_workers optional under orq2 (Owner addendum)
`limits.validate` requires `max_workers` only for orq. `limits.SinTope = -1`, `Limits.TopeWorkers()`. `recursos.Calcular`
with `SinTope` starts with no head cap, so RAM, CPU or carga limits; `Resumen` says `max_workers sin tope`. Gate: no
`max_workers` → no head-count refusal; if capacity cannot be measured either (`env.Capacidad == nil`) it refuses
(unknown denies). `status`: `max_workers=sin tope (deciden RAM, CPU y carga)` and `capacidad: N agentes a la vez (limita RAM|CPU|carga)`.
orq: still `falta max_workers`; status line byte-identical when present.

## P8 — orchestrator guide (Owner addendum)
Under orq2, `orq skill orquestador` replaces `({{rol:astra}}; \`--kind claude\` if no Codex quota)` with
`(<astra kind> {{rol:astra}}: the agent and model the astra role sets in \`orq config ver\`)` (then `orq`→`orq2` as usual).
The embedded `guias/orquestador.md` is unchanged, so orq prints it byte-identically (test compares with `cfg.Expandir(raw)`).

## P9 — the card is the Owner's decision document (Owner addendum)
Under orq2 only, `instalarSkillsOrq` passes each skill through `skillOrq2` before `identidad.Texto/Skills`:
the planner skill gains a `## Card` section before `## Astra` (plain Spanish a non-programmer reads in 2 minutes, max 25
lines, the five section names kept for `tarjeta.go`; Historia step by step plus `Valor: <metric> hoy <v> → meta <v>`;
Fuera in plain words; Examen one line per test group with the behaviour proven and the test path once; Zonas as what
changes for the user, task ids and technical words stay in `.orq/plan-sNN.json`; Preguntas only what the Owner answers,
each with the planner's recommendation, human gates as `- [ ] lo haces tú: …`). The orchestrator skill's 5b card rule
points to that section. orq's installed skills equal the embedded files byte for byte (test).
`TestStatusMuestraMaquinaYCapacidad` fails on this host before and after P9: it reads the real /proc, where 11–12 live
agents make CPU the limit instead of the RAM the test expects. Not caused by P9 or P12; the integrator should know.

## P12 — no head cap by default, RAM warning, `orq2 doctor --ram` (Owner addendum)
- No cap is expressed only by omitting `max_workers` → `TopeWorkers() == SinTope` (tested); nothing writes a negative
  value, so it holds with F7 (`v2/review-fixes` d448bbe: negative rejected, `SinTope = math.MinInt`). F7 is not in `v2/build`.
- Template: `templates/limits-orq2.json`, no `max_workers`, `aviso_ram_gb: 5`, scratch under `~/.local/state/orq2/`.
  `templates/limits.json` (orq) unchanged. `install2` still writes only `~/bin/orq2`; the gate's "no limits" message
  points orq2 to the new template.
- `limits.aviso_ram_gb` (optional, default 5, `Limits.AvisoRAM()`). `app.AvisoRAM` returns
  `RAM libre X GB (aviso ≤N): corre orq2 doctor --ram` when available RAM ≤ N, only under orq2.
- Printed by `orq2 status` (after the `máquina:` line), `orq2 watch` (after disk lines) and the gate on an allowed launch
  (`Decision.Aviso`, to stderr). The gate still refuses only on hard capacity (test). Caveat: Claude Code does not show a
  hook's stderr on exit 0 to the agent; the Owner sees it in verbose/transcript mode only.
- `orq2 doctor --ram` (orq ignores `--ram` as before): RAM header, top 8 by RSS (pid, name, GB, `agente` / `worker orq2`
  via `EsWorkerContable` = `EsWorkerPropio`, cwd), listening TCP v4/v6 ports with owner pid from `/proc/net/tcp*` +
  `/proc/<pid>/fd` `socket:[inode]`. Every line ends in `→ <suggestion>` (`cerrar panel…`, `detener servidor en puerto N`,
  `revisar quién lo abrió` for a socket with no visible owner). Read-only; idle panes out of scope. Tests use fake /proc
  trees in `t.TempDir()`; one smoke test on the real /proc. Live run on this host: 6.6 GB free, two `next-server` at 1.6 GB
  each on ports 13742/13743 at the top.

## Final proof on Real State (clone, nothing written in the Owner's checkout)
`git clone --no-local` of Real State (HEAD `139fc2f test(S-01): red scenario`) into the scratchpad; binary built to a
temp `orq2`; `ORQ_NOMBRE=orq2`, temp ORQ_STATE/ORQ_LIMITS/ORQ_CONFIG/ORQ_WORKTREES. `.orq/` is untracked there, so the
clone has no config. `.venv`: a symlink to the Owner's `.venv` first (gave K9), then a `cp -a` copy with shebangs
rewritten to the clone path — the same layout the real checkout has (venv at the root's own path).
- `orq2 version` → `2.0.0-pilot revision=unknown modified=unknown`.
- `orq2 init` → the three commands, identical to the ones the Owner wrote by hand in Real State's `.orq/config.json`.
- `orq2 check --caja` → exit 0: lint `ruff 0.15.20`, types `mypy 2.1.0 (compiled: yes)`, tests `pytest 9.1.1`, each
  "corre en la caja … monta <clone>/.venv".
- Each check once through the box (`copiaDe(HEAD)` + `correrEn`, the exam's path, via a temporary test file not committed):
  | Check | Exit | Why |
  |---|---|---|
  | lint | 1 | `ruff check` passes; `ruff format --check`: `tests/test_s01_controlled_outreach.py` would be reformatted |
  | types | 0 | `Success: no issues found in 113 source files` (24 s) |
  | tests | 1 | 13 failed (51–57 s): 10 in `test_s01_controlled_outreach.py` (the red acceptance test of HEAD — expected), 3 in `test_send_test_inbound.py` |
- The 3 inbound tests fail for env reasons, as the plan expected: their `bin/send-test-inbound` runs `uv run` without
  `--no-sync`, uv tries to build `dante-os`, needs `hatchling`, and there is no network. Command not weakened.
  This closes the plan's UNVERIFIED line: full `mypy src` and `pytest` do run in the box with these commands.
- Untracked files of the Owner's checkout (3 new tests) are not in HEAD, so they were not part of this run.
