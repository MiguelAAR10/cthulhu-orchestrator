# orq2 agent-first plan

Author: Fable (planning only), 2026-11-19, three read-only Sonnet scouts. Go 1.25, standard library only.
Direction: the callers of `orq2` are agents in herdr panes, guided by the `orq2-*` skills. The Owner talks to
Atena and later watches a terminal UI. He types no orq2 command, yet keeps two locks: card approval and merge.
Paths: `H/` = `~/worktrees/cthulhu-build/harness/internal/`, `RS/` = the Real State `.orq/`, `T:` = Claude
transcript (first 8 characters of the file name, under `~/.claude-alt/projects/*AI-Agent-RealState/`) plus line.
**[V]** = I re-read the line myself. **[S]** = scout reading only. **[U]** = not checked by anyone.

## 1. Misuse inventory (Real State pilot, about 75 orq2 calls, 12 failed [S])
| # | What happened | Evidence | Cause | Fix |
|---|---|---|---|---|
| M1 | Orchestrator proposed and ran the removed `--en-sitio`; Atena authorised it | `T:718accfb:244-246`, `RS/canal.jsonl:27,29` [S] | The skill and the guide still teach it: `H/cli/orqskills/orq-orchestrator/SKILL.md:25`, `H/cli/guias/orquestador.md:8` [V]. The error (`H/slice/nucleo.go:199` [V]) explains why but names no command | Guide: delete it. CLI: error ends with `orq2 check --caja` |
| M2 | Planner built its own clone and `sandbox exec --ro --rw` instead of `check --caja`; its flags did not match the real box | `T:c36b00bc:299-323`, `RS/canal.jsonl:35` [S] | No skill or guide teaches `check --caja` (grep finds only plain `orq check` in `guias/worker.md:12` [V]) | Guide: teach it to Planner and Orchestrator. CLI: `la base no está sana` ends with it |
| M3 | Card caps learned one rejection at a time | `T:718accfb:326` [S] | Caps live only in code (`H/slice/tarjeta.go:31-34` [V]: 3 lines Historia, 5 per section, 200 characters, 400 words). Texts disagree: skill says ≤3, the orq2 rewrite says 25 lines, the guide stays ≤3 [S] | CLI: report every violation in one pass. Guide: caps printed from the constants |
| M4 | Orchestrator suggested `--force`; the gate blocked it and named no next step | `RS/canal.jsonl:3` [V], `T:718accfb:64` [S] | The refusal says "the Owner does it by hand" | CLI: refusal ends with `orq2 msg send --para atena --tipo pregunta` |
| M5 | Planner skill names `orq canal`, which does not exist | `H/cli/orqskills/orq-planner/SKILL.md:12` [V] | The guide test covers guides only, first word only (`H/cli/proyecto_test.go:89-97` [V]) | Guide: `orq msg leer`. Test: section 2.3 |
| M6 | `mem add` without `--imp`: the error lists values, not the flag | `T:6a748f7c:129` [S] | Message | CLI: name `--imp` |
| M7 | Base check failed 4 times (no network for uv, no `node_modules` in the box); the agent tried `UV_OFFLINE=1` | `T:718accfb:203,218,222,267` [S] | Refusal names no command; the box has no supported path for dependencies | CLI now: next command. Dependency mount: after |
| M8 | `slice aprobar 01` before any slice was open | `T:6a748f7c:166` [S] | Atena ran an Owner command | Guide: Atena never runs `aprobar`. Section 3 makes it a request |
Not orq2's fault: a zsh variable used as a command (`T:6a748f7c:142` [S]). `diario.jsonl` holds no command text,
so this inventory depends on transcripts; section 2.4 fixes that.

## 2. Agent contract
### 2.1 Rules for every command (orq2 identity only; `orq` output stays byte-identical)
1. **One table.** `H/cli/comandos.go`: `[]Comando{Nombre, Subs, Proposito, Flags, JSON bool, Rol}`. Usage text,
   `orq2 help`, `orq2 help CMD` and the guide test read it. Today usage is one hand-written const plus 27
   `uso:` strings, and `help` exits 1 on stderr [S]. `help` becomes exit 0 on stdout.
2. **`--json` everywhere** (today only `metricas` [V]). One envelope on stdout, also on failure:
   `{"esquema":1,"cmd":"slice abrir","ok":false,"datos":{},"error":{"codigo":"base_no_sana","texto":"…","siguiente":["orq2 check --caja"]}}`.
   Prose goes to stderr. `codigo` is a stable snake_case identifier; agents branch on it, never on the text.
3. **Every refusal ends with the next command**, built from the table. An error with no possible next step says
   who decides: `orq2 msg send --para atena --tipo pregunta`.
4. **Stable exit codes**, as named constants in one file and listed by `orq2 help codigos`. Existing meanings do
   not move:
   | Code | Meaning | Agent reaction |
   |---|---|---|
   | 0 | done | continue |
   | 1 | refused by a rule | run `error.siguiente` |
   | 2 | gate block or HALT | stop, tell Atena |
   | 3 | attention, not an error | read the output, continue |
   | 4 | delivery outcome unknown | check with `sesion ver`, do not resend blind |
   | 5 (new) | waits for the Owner | wait; never retry or work around |
   | 64 (new) | usage error, split from 1 | run `orq2 help CMD` |
   | 70 (new) | internal failure, split from 2 | report, do not retry |
5. **Flags.** New flags are Spanish and long. No rename without keeping the old name as an alias. A removed flag
   stays registered for one version with an error that names the replacement, then goes into a `removidos` list
   that the guide test rejects. "Skip with a reason" keeps its three names; unify later behind aliases.

### 2.2 Per command (purpose; `datos` under `--json`)
| Command | Purpose | `datos` |
|---|---|---|
| `status`, `doctor`, `watch`, `metricas` | health and numbers of the project | checks, HALT, warnings, counts |
| `panel` | read-only board | `Vista` (schema 1, panel plan) |
| `config ver` | effective settings and accounts per role | keys with origin, model and effort per role |
| `historia`, `pausar`, `reanudar`, `halt` | story and pause state | story id, paused flag |
| `msg send`, `msg leer`, `sesion abrir/ver/cerrar`, `up`, `relevo` | sessions and channel | message id, session, pane, delivery state |
| `mem add`, `mem ver` | facts with a source | fact id, superseded id |
| `plan cargar/debatido/ver` | the agreed plan | tasks, zone, difficulty, state |
| `check`, `check --caja`, `sandbox selftest/exec` | run the project's checks, in or out of the box | per check: command, exit, first line |
| `slice abrir/tarjeta/lanzar/estado/verificar/recibo/juntar/podar/reintentar/reabrir` | one slice through its phases | phase, card state, receipt counts, next command |
| `slice aprobar`, `slice decidir`, `slice cerrar --aceptado` | Owner locks | request id (exit 5), or the stamp |
| `skill ROL`, `skills`, `contexto`, `doc`, `fase`, `aprender`, `alinear` | guidance and drift | text, or drift list |
| `gate`, `init`, `version` | plumbing | verdict; paths; version |

### 2.3 Tests
- `TestGuiasYSkillsSoloNombranLoQueExiste`: for every embedded guide and skill, every `orq WORD [SUB]` (inside or
  outside backticks) must be in the table, every `--flag` on that line must belong to that command, and no token
  may be in `removidos`. Replaces the test at `proyecto_test.go:89-97`.
- `TestTablaIgualAlDespacho`: each table entry reaches a real handler and each `case` of the switch is in the table.
- `TestTodoRechazoNombraSiguiente`: each `codigo` has a `siguiente` whose first word pair is in the table.
- `TestJSONEnFallo`: with `--json`, a refusal prints a valid envelope on stdout and the documented exit code.
### 2.4 Journal
`diario.jsonl` gets one event per orq2 call: `cmd`, `rol`, `exit`, `codigo`. No arguments or text, so no secrets.
The next misuse inventory is then one query, not a transcript search.

## 3. The Owner's locks when agents run everything
Today: `slice aprobar` refuses when `ORQ_ROL` is set (`H/cli/tarjeta.go:93` [V]); merge and `--decision-owner`
are convention (K4, E11). An agent that unsets the variable passes.
**Proposal: a request queue plus one confirming process.**
1. For anyone, `slice aprobar NN`, `slice cerrar --aceptado|--rechazado` and the Owner merge become a **request**:
   they write `<state>/pedidos/pN.json` (closed action list, argv built by orq2), print the request id and exit 5.
2. Only `orq2 tui` (section 4) confirms. It starts only if stdin and stdout are a terminal and `ORQ_ROL` is
   empty, and it records its `HERDR_PANE` in `<state>/owner.json` (mode 0600). It shows who asked, the exact argv
   and the card or receipt, then reads one key from `/dev/tty`: flush pending input, ignore keys for 500 ms, one
   request at a time, 5 minute expiry. On `s` it performs the write in-process and journals `origen=owner-tty`.
3. Gate rules (`H/gate/reglas.go`), one test each: deny `herdr pane send-keys|run|send-text` and
   `orca terminal send` to the recorded pane, and to any dynamic target while the TUI runs; deny starting
   `orq2 tui`; deny writes to `pedidos/`, `owner.json` and to the approval fields under `.orq/slices/` [U: check
   whether the gate already denies direct `.orq` writes].
4. This replaces the HTTP server of panel plan section 8 with files: no port, no token, no Origin check.
**Limit, stated plainly:** all agents and the Owner share one Unix user. A process outside the gate can still
inject keys or write the file. This stops every gated agent, which is every pane orq2 opens. Stronger options,
later and only if needed: a 4-character code shown in the pane instead of `s`; a separate Unix user for agents.
Reversible: `ORQ2_PEDIDOS=0` restores today's direct `aprobar`.

## 4. The TUI
- **Data.** `Vista` from `orq2 panel --json` (branch `v2/panel`) is the only source. The TUI never reads `.orq/`.
  Refresh every 2 s and on `r`. Test: TUI model equals the decoded `panel --json` golden.
- **Where it lives.** Recommended: `orq2 tui` inside the binary, package `internal/tui` (layer above `panel`),
  ANSI escapes plus raw mode through `syscall` termios (Linux and WSL2 only; stdlib has no terminal package).
  Rendering is a pure function `Pintar(Vista, Estado, ancho, alto) string` with golden tests; the key loop is
  thin. Alternative: a separate binary in its own module with a TUI library, reading the same JSON. Choose it
  only if the stdlib version needs more than lists, a drawer and a strip. A Neovim plugin is a third reader of
  the same JSON, not planned.
- **Views.** (1) Backlog: the panel columns as groups, `j/k` to move, `Enter` opens the drawer. (2) Story drawer:
  what it is, steps, plan tasks, evidence, last events. (3) Agents: role, model, effort, account, from
  `config ver --json`. (4) Top strip: pending Owner requests, HALT, pause, RAM warning (from `doctor --json`
  [U: confirm doctor or watch exposes memory]), `sin dato` where the panel says so.
- **What it may write.** Nothing directly. Keys run existing commands and show their envelope: confirm a request
  (section 3), `pausar`, `reanudar`, and model per role through a `config` write command [U: confirm one exists;
  if not, add `orq2 config poner CLAVE VALOR` first]. Each write shows the argv before it runs.

## 5. Phasing
**Now (only what the pilot hit; text and message changes, one test):** M1–M8 fixes, the guide and skill test of
2.3 (first bullet, with a minimal table of names, subcommands and flags), `removidos` list.
**After the stop rule, in order:** (1) full table drives usage and `help`; exit codes 5, 64, 70; envelope and
`--json` on `status`, `slice estado`, `config ver`, `doctor`, `check`, then the rest; journal event per call.
(2) Request queue and gate rules with a plain `orq2 tui` that only lists and confirms requests. (3) TUI views.
(4) Flag aliases, dependency mount for the box (M7), `config ver` showing the orq 1.0.0 version under orq2 [S].
Not planned: renaming commands to English, a server, multi-project board.

## 6. Builder brief for "now" (Opus; tests first; one commit each; `cd harness && go test ./...` green each time)
1. `cli: command table and strict guide test`. Add `comandos.go` (names, subcommands, flags; no behaviour change)
   and `TestGuiasYSkillsSoloNombranLoQueExiste` over `guias/*.md` and `orqskills/*/SKILL.md`, with `removidos =
   {--en-sitio, --aislado}`. It must fail on `orq-orchestrator/SKILL.md:25`, `guias/orquestador.md:8` and
   `orq-planner/SKILL.md:12`. Add `TestTablaIgualAlDespacho`.
2. `skills: remove --en-sitio, fix orq canal`. Delete the opt-out sentence in both files; `orq canal` becomes
   `orq msg leer`. Test from step 1 goes green.
3. `skills: teach check --caja`. Planner and Orchestrator skill and guide: "to prove the base is healthy, run
   `orq check --caja`; never build your own `sandbox exec`". Atena skill: "`slice aprobar` is the Owner's".
4. `tarjeta: report all cap violations and print caps`. The validator collects every violation and returns them
   together; `slice tarjeta --topes` prints the four constants. Skill and guide text is produced from the same
   constants and the 3-versus-25 contradiction is removed. Tests: a card with three violations lists three; the
   skill text contains each constant's value.
5. `cli: refusals name the next command`. Five messages, orq2 only: `--en-sitio` and `la base no está sana` end
   with `orq2 check --caja`; `mem add` names `--imp`; the gate's force refusal ends with the `msg send` line;
   `no hay slice abierto` ends with `orq2 slice estado`. One test per message.
6. Reinstall skills into Real State (edited files are not overwritten: report which were skipped) and write
   `docs/harness-v2/agent-first-report.md`: what passed, what stayed [U].
Stop and ask if an `orq` golden would change, if a fix needs a new `.orq` field, or if step 4 changes which
cards are accepted.
