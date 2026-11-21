# Review 2 fixes report (branch v2/review2-fixes, from harness-v2 at 309addf)

Source: `docs/harness-v2/reviews/review-v2-integration.md`. Each item: red test first, then the smallest fix.
No golden, testdata golden or `cli/version_test.go` changed. gofmt clean, `go vet ./...` clean.

| Item | Red test | Fix | Commit |
|---|---|---|---|
| H1 | `TestGuiaTarjetaClaraParaOwner` (cli): requires "`## Zonas`: one repo path per line", "relative to the repo root, never absolute", Owner meaning in Historia/Examen; forbids "what changes for the user" | `tarjeta_guia.go`: Zonas are repo paths (folder or file, relative, never absolute, `..` or root) that orq checks changed files against; plain Spanish applies to every section except Zonas | c5a7dc1 |
| M1 | `TestGuiaTarjetaClaraParaOwner`: planner and orchestrator skills must contain `slice.TopesTarjeta()`, never "25 lines"; planner mentions `orq2 slice tarjeta --topes` and that the `Valor:` line counts inside the Historia cap; the orq2 orchestrator guide drops "story ≤3 lines" | Removed "Max 25 lines" and "≤25 lines"; both point to the P11 caps line (built from `slice.TopesTarjeta()`, not duplicated). Historia no longer asks for step-by-step text plus an extra line. `guias_orq2.go`: orq2 overlay replaces the orchestrator guide's "story ≤3 lines" with "story within the card caps below" | cfbda8b |
| M2 | `TestLeerTarjetaErroresComo100` (slice): under `orq`, compares error text with a verbatim copy of the 1.0.0 parser (`leerTarjeta100`) on 8 doubly invalid cards, including a 201-char line followed by an unknown section; under `orq2`, requires both the cap and the structural error | `tarjeta.go`: unknown/duplicate section and text outside a section are appended to the same list as the caps, in document order (lines under an unknown or duplicate section are skipped). orq returns the first, which is where 1.0.0 stopped; orq2 returns all with `errors.Join` | bff4c88 |
| M3 | `TestSondaUvSaltaValorDeFlag`: added `uv run --no-sync -p` and `uv run --offline --no-sync --with` (both want `""`); panicked before the fix | `caja.go`: `if j >= len(f) { return "" }` | 5845ef9 |
| L1 | `TestGuiasYSkillsSoloNombranLoQueExiste` now renders skills through `skillOrq2` as `instalarSkillsOrq` does. Checked it bites: a bogus `orq slice nadaxx` in the P9 text fails the new test and passes the old one | test-only change | ae9a121 |
| L3 | `TestAvisoRAMValidado` (limits): orq2 refuses `aviso_ram_gb` 0 and -1; orq refuses the key with 1.0.0's exact error (`json: unknown field "aviso_ram_gb"`) | `limits.go`: under orq, decode into `limitsOrq` (same fields, `aviso_ram_gb` tagged `json:"-"`, converted to `Limits`; the conversion stops compiling if the fields diverge). `validate` refuses `aviso_ram_gb` that is not > 0 (also NaN), which can only be set under orq2 | bdf40db |

## Not done (out of scope for this round)

- L2: P12 gate RAM warning on stderr with exit 0 is likely invisible to the agent in a PreToolUse hook. Informational; status/watch show it.
- L4: `orq2 slice tarjeta --topes` needs a project. Cosmetic.
- L5: `HerramientasUv` keeps a host lookup when a plain segment mentions the tool as an argument. Diagnostic only, fail-safe.

## Final test run

From `harness/`, isolated env (`ORQ_STATE`, `ORQ_LIMITS`, `ORQ_WORKTREES`, `ORQ_CONFIG` in a fresh `mktemp -d`, `ORQ_SIN_DOC=1`):
`go test -count=1 ./...` exit 0: 24 packages ok, 0 failures (slice 251.8s, cli 106.1s under machine load).
