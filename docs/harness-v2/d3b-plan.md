# D3b · Skill evals wired to `claude plugin eval` — Implementation Plan

> **For agentic workers:** executed inline with superpowers:executing-plans and superpowers:test-driven-development. Steps use checkbox (`- [ ]`) syntax.

**Goal:** orq scores a standalone skill through `claude plugin eval`, keeps one row per run, lists the scores, ranks a project's worker skills by score at launch, and says keep/discard between two skill versions.

**Architecture:** a new domain package `internal/skilleval` (layer 1, like `perfil`) holds the wrapper, the call to `claude`, the JSON parse, the score store, the ranking and the comparison. `cli/skill.go` gains three subcommands. `app.LanzarWorker` appends the ranked list to the worker's contract. The slice gets a `Tipo` field set by `orq slice abrir --tipo`; the default type comes from the stack.

**Tech Stack:** Go standard library only.

**Spec:** `docs/harness-v2/plan.md` › "Route" › "Approach" mechanism 2; the D3b brief; scout notes `docs/harness-v2/working/research/d3b-plugin-eval-2026-11-19.md`.

## Global Constraints

- Standard library only; no new dependency. `internal/arch_test.go` green: `skilleval` gets layer 1.
- Never touch `internal/gate` or `internal/sandbox`.
- Tests never call a real `claude`: a stub script on `PATH`; temp `ORQ_STATE`, `ORQ_CONFIG`, `ORQ_WORKTREES`.
- `orq skill eval` refuses to run without `--max-usd` (> 0).
- Command run: `claude plugin eval <plugin-dir> --json <path> --ablation none --max-cost-usd N --trust-plugin`.
- Store: `state.Dir()/skill-notas.jsonl`, one JSON row per run.
- `--margen` default `0.1`; `skills.minimo` default unset (no filter).
- User-facing text in Spanish like the rest of orq; commits in English with the Co-Authored-By line.

## Review Focus

1. A run that hit the cost ceiling (exit 2, `partial: true`) must not become the skill's ranking score → stored with `parcial`, ignored by ranking and `comparar` (test in Task 3 and Task 4).
2. `claude` exits 1 because a case scored below `--threshold` (default 1.0) — that is a valid result, not a failure → parsed and stored (Task 2).
3. `comparar` with hashes typed as the 12-char prefix that `notas` prints → prefix match; an ambiguous prefix is an error (Task 4).
4. A skill dir with no `evals/` or no `SKILL.md` → clear error before any money is spent (Task 1).
5. `claude` missing from PATH or an unknown `plugin eval` subcommand → "no disponible" message, nothing stored (Task 2).

---

### Task 1: wrapper and content hash

**Files:** Create `harness/internal/skilleval/skilleval.go`, `harness/internal/skilleval/skilleval_test.go`; Modify `harness/internal/arch_test.go` (add `"skilleval": 1`).

**Produces:**
- `func Hash(dir string) (string, error)` — sha256 hex over the dir's regular files, sorted by relative slash path, each as `path\x00content\x00`.
- `func Envolver(skillDir, dst string) (nombre string, err error)` — nombre = base name of skillDir. Writes `dst/.claude-plugin/plugin.json` = `{"name":nombre,"version":"0.0.0","description":"orq: one-skill wrapper for claude plugin eval"}`, copies the skill (without `evals/`) to `dst/skills/<nombre>/`, copies `skillDir/evals` to `dst/evals`. Errors: missing `SKILL.md` ("no es una skill: falta SKILL.md"), missing `evals/` ("sin casos: falta evals/ …").

- [ ] Test `TestEnvolverDejaUnPluginDeUnaSkill`: skill `s/mi-skill/{SKILL.md,ref.md,evals/c1/prompt.md}` → plugin.json name `mi-skill`; `skills/mi-skill/SKILL.md` and `ref.md` exist; `skills/mi-skill/evals` does not; `evals/c1/prompt.md` exists.
- [ ] Test `TestEnvolverRechaza`: no SKILL.md → error contains `SKILL.md`; no evals → error contains `evals`.
- [ ] Test `TestHashCambiaConElContenido`: same content in two dirs → equal hashes; one byte changed → different; 64 hex chars.
- [ ] See them fail, implement, green, commit.

### Task 2: run `claude plugin eval` and parse

**Files:** Modify `skilleval.go`; test file `skilleval_test.go` with a stub writer helper `stubClaude(t, script string)` that writes `claude` into a temp dir and prepends it to PATH via `t.Setenv`.

**Produces:**
- `type Caso struct { Nombre string; Nota float64 }`
- `type Resultado struct { Nota float64; Casos []Caso; Corridas int; CostoUSD float64; Claude string; Parcial bool }` — from `aggregates.overallScore`, `cases[].name`, `cases[].aggregates.score`, Corridas = Σ len(`cases[].arms.with`), `costUsd`, `claudeVersion`, `partial`.
- `func Leer(raw []byte) (Resultado, error)` — error when `schemaVersion != 1`.
- `var ErrNoDisponible = errors.New("claude plugin eval no está disponible en esta cuenta (acceso anticipado); no se evalúa")`
- `func Correr(ctx context.Context, plugin, jsonPath string, maxUSD float64) (Resultado, error)` — runs the Global Constraints command. Exit 0, 1 (a case under threshold) and 2 (cost ceiling: `Parcial` forced true) with a readable JSON → result. Otherwise, or `claude` not found: if stderr matches `(?i)early access|not available|unavailable|unknown command|claude.*not found` or the binary is missing → `ErrNoDisponible` wrapped with the first stderr line; else error with exit code and stderr tail.

- [ ] Test `TestCorrerPasaLosArgumentosYLee`: stub checks `$1 $2 = plugin eval`, `$3/.claude-plugin/plugin.json` and `$3/evals` exist, and the rest is exactly `--json P --ablation none --max-cost-usd 2.5 --trust-plugin`; writes a fixture JSON (2 cases 1.0 and 0.5, 3+3 runs, cost 0.41, version `2.1.328`) → Nota 0.75, Corridas 6, Casos names/scores, Claude `2.1.328`, not partial. Stub exits 1 on bad args with message; test asserts no error.
- [ ] Test `TestCorrerSalida1EsResultado` (exit 1 + JSON → result) and `TestCorrerTopeDeCostoEsParcial` (exit 2 + JSON → Parcial).
- [ ] Test `TestCorrerNoDisponible`: stub prints `Error: plugin eval is currently in early access` exit 1, no JSON → `errors.Is(err, ErrNoDisponible)` and message contains `early access`; PATH without claude → ErrNoDisponible.
- [ ] Fail, implement, green, commit.

### Task 3: score store, `orq skill eval` and `orq skill notas`

**Files:** Modify `skilleval.go`; Modify `harness/internal/perfil/perfil.go` (`Tipo`); Modify `harness/internal/cli/skill.go`, `cli.go` (usage lines); Create `harness/internal/cli/skilleval_test.go`.

**Produces:**
- `type Nota struct { Skill, Hash, Tipo string; Nota float64; Corridas int; CostoUSD float64; Claude string; Parcial bool; Fecha time.Time; Casos []Caso }` (JSON snake-case tags).
- `func RutaNotas(dir string) string` → `dir/skill-notas.jsonl`; `func Guardar(dir string, n Nota) error` (append); `func Notas(dir string) ([]Nota, error)` (missing file = empty).
- `func Listar(ns []Nota, tipo string) []Nota` — filter by tipo when given; order by skill, then hash groups by their newest row, newest first.
- `func (p Perfil) Tipo() string` in perfil — `strings.Join(Senales, "+")`, `"general"` when none.
- CLI `orq skill eval DIR [--tipo T] --max-usd N`: no/≤0 `--max-usd` → exit 1, stderr `--max-usd es obligatorio: cada corrida gasta dinero real`, stub never called. Tipo default: project's `Perfil().Tipo()`, outside a repo `general`. Wrap in `os.MkdirTemp`, run, store, print `mi-skill <hash12> tipo T: nota 0.75 en 6 corridas · USD 0.41 · claude 2.1.328` plus one line per case. ErrNoDisponible → exit 3 with the message.
- CLI `orq skill notas [--tipo T]`: one line per row `skill  hash12  tipo  nota  corridas  USD  claude  fecha` with ` (parcial)` when partial; empty → `sin notas todavía: orq skill eval DIR --max-usd N`.

- [ ] Test `TestSkillEvalExigeMaxUSD`: stub that `touch`es a marker; exit 1, message, marker absent.
- [ ] Test `TestSkillEvalGuardaYNotasLista`: temp ORQ_STATE; two evals (same skill, edited between) with `--tipo go` → `notas` prints two rows, newer hash first; `notas --tipo web` prints the empty message.
- [ ] Test `TestSkillEvalNoDisponible`: exit 3, stderr contains `acceso anticipado`, nothing stored.
- [ ] Test `TestTipoDelPerfil` in perfil: `{go,web}` → `go+web`; none → `general`.
- [ ] Fail, implement, green, commit.

### Task 4: `orq skill comparar`

**Produces:**
- `type Veredicto struct { Viejo, Nuevo Nota; Delta float64; Conservar bool }`
- `func Comparar(ns []Nota, viejo, nuevo, tipo string, margen float64) (Veredicto, error)` — each hash (or unique prefix) → newest non-partial row of that hash (and tipo if given); different tipos without `--tipo` → error naming both; Conservar = Delta > margen.
- CLI `orq skill comparar OLD NEW [--tipo T] [--margen 0.1]`: prints `conservar: nota 0.60 → 0.80 (+0.20 > margen 0.10)` or `descartar: … (+0.05 ≤ margen 0.10): el juez es ruidoso`; exit 0 either way; errors exit 1.

- [ ] Test `TestCompararConMargen` (unit): +0.2 keep at 0.1; +0.05 discard; −0.3 discard; margen 0.01 keeps +0.05; prefix lookup; ambiguous prefix error; partial row of NEW ignored (falls back to its older full row or errors when none).
- [ ] Test `TestSkillCompararCLI`: two stored rows, `comparar h1 h2` prints `conservar`, `--margen 0.5` prints `descartar`.
- [ ] Fail, implement, green, commit.

### Task 5: ranking and threshold at launch

**Files:** Modify `config/config.go` (`Skills`), `slice/slice.go` + `slice/nucleo.go` (`Tipo`), `cli/proyecto.go` (`--tipo` on `abrir`), `app/workers.go`; tests in `config/config_test.go`, `app/app_test.go`, `skilleval_test.go`.

**Produces:**
- config: `Skills Skills `json:"skills"``, `type Skills struct { Minimo *float64 `json:"minimo,omitempty"` }`; validation 0..1 (`skills.minimo debe estar entre 0 y 1`).
- slice: `Opciones.Tipo`, `Estado.Tipo `json:"tipo,omitempty"``, set in `Abrir`; `orq slice abrir --tipo T`.
- `type Eleccion struct { Skill string; Nota *float64 }`
- `func Ordenar(nombres []string, ns []Nota, tipo string, minimo *float64) (elegidas, fuera []Eleccion)` — latest non-partial row per skill for tipo; scored skills by score desc (ties by name), then unscored in given order; scored below minimo go to `fuera`.
- app: in `LanzarWorker`, tipo = `e.Tipo` or `p.Perfil().Tipo()`; names = `perfil.ParaRol(perfil.Lista(p.Perfil()), "worker")`; the contract text gets `Skills para el tipo T, de mejor a peor nota: a 0.92, b sin nota.` and, when any, `Fuera por nota bajo skills.minimo 0.50: c 0.30.` A store read error is returned (no launch on unreadable state).

- [ ] Test `TestOrdenar` (unit): order by score, unscored last, threshold drops, partial ignored, other tipo ignored.
- [ ] Test `TestConfigSkillsMinimo`: `{"skills":{"minimo":0.5}}` loads; `1.5` errors.
- [ ] Test `TestLanzarOrdenaSkillsPorNota` (app): ORQ_STATE temp with notes for `systematic-debugging` 0.9 and `receiving-code-review` 0.2 tipo `general`, project config `skills.minimo 0.5` → the `avisar` text lists `systematic-debugging 0.90` before `test-driven-development sin nota`, and `Fuera por nota bajo skills.minimo 0.50: receiving-code-review 0.20`.
- [ ] Test: slice opened with `Tipo: "go"` → contract says `tipo go`.
- [ ] Fail, implement, green, commit.

### Task 6: verify and report

- [ ] `gofmt -l harness`, `go vet ./...`, `go test -race ./...`, `make lint`, `make ci`; read the output.
- [ ] Write `docs/harness-v2/d3b-report.md`; commit.
