# D2b report · the worker checks web apps like a person

Branch `v2/build`, worktree `~/worktrees/cthulhu-build`. Plan: `docs/harness-v2/d2b-plan.md`. Spec: `plan.md` › "Route" D2 and "Approach" mechanism 1; built on `d2a-report.md`.

What I did not do:
- Install, push, or write to `~/bin/orq`, `~/.config/orq` or `~/.local/state/orq`.
- Call a real `claude` or `codex`. Every test used temp `ORQ_*` dirs.
- Touch `internal/gate`.
- Add dependencies.

## Commits

| Task | Commit | Subject |
|---|---|---|
| plan | `2104abd` | docs: D2b plan — Playwright in the box and the feature map |
| 1 | `c812b64` | init: feature map and e2e Playwright check for web projects |
| 2 | `5ed743b` | verificar: Playwright in the box with the app server on localhost, browsers bound, screenshots kept |
| 3 | `6f3eaeb` | check, page, guide: Playwright and the feature map for web workers |
| real-run fix | `3a5a2a7` | verificar: a Playwright exam with a counted report keeps its server |
| report | this commit | docs: D2b report |

## `make ci` on `3a5a2a7` (clean tree, exit 0)

- Candidate: `/tmp/orq-ci.JPTD8z/bin/orq · 1.0.0 revision=3a5a2a709b0b58d0edac68bdfcef99a307028184 modified=false`.
- `make lint` (staticcheck 2025.1.1): clean. `gofmt -l harness`: nothing listed. `go vet ./...`: clean.
- `go test -race ./...`: every package ok. 282 top-level tests pass, 0 fail, 2 skipped (the same 2 as D2a: `TestPrepararDeps` and herdr's `TestLiveTabSplitClose`). 25 subtests pass.
- `e2e-v05`: **86 pass, 0 fail**.
- `aislamiento`: **14 pass, 0 fail, 0 pending**.

## How it works

The `checks` block gets one more optional entry, `e2e`, and it can carry the app server:

```json
"e2e": {"cmd": "npx playwright test", "servidor": "npm run dev", "puerto": 3000}
```

- `servidor` and `puerto` are only valid on `checks.e2e`, and they must come together (`1 ≤ puerto ≤ 65535`, with `cmd`). Anything else is refused at load.
- `Check.Ejecutable()` turns this into one sh script:
  - It starts the server with `setsid`, so the server and its children are one process group.
  - It waits up to 60 s for `127.0.0.1:puerto`, using `bash`'s `/dev/tcp`.
  - If the port never opens, or the server exits first, it fails with exit 1, prints `orq: el servidor de checks.e2e no abrió 127.0.0.1:N` and the last 20 lines of the server's log.
  - Otherwise it runs `cmd` and kills the server group on exit. The script exits with `cmd`'s code, so exit 127 is still reported as infrastructure.
- `orq check` (on the host) and `verificar` (in the box) run the same script.
- The box has its own network namespace. Loopback works inside it and nothing reaches outside. I checked this with bwrap by hand before writing the code.

## What the Owner can do now, item by item

**1 · `orq init` in a web project writes `.orq/mapa.md` and, with a Playwright config, `checks.e2e`.**
- The map has one line per route: `` - `/cart` → `app/(shop)/cart/page.tsx` · reach: ? · shows: ? ``.
  - Routes come from the Next app router (route groups `(x)` and slots `@x` are dropped), the Next pages router (`_app`, `_document`, `_error` and `api/` are skipped), and `path="…"`/`path: "…"` in files that import `react-router`.
  - It is written only when missing. After that the file belongs to the Owner and the workers.
- `e2e` is written only when a `playwright.config.{ts,js,mjs,cjs,mts,cts}` exists. It uses `npm run <script>` when a script runs `playwright test`, otherwise `npx playwright test`.
  - The server is `npm run dev` on port 3000 for Next or 5173 for Vite.
  - When the Playwright config has its own `webServer`, orq starts no server. Otherwise Playwright would fail with "port in use".
- With no Playwright config there is no `e2e` check. `init` prints `sin check e2e: sin playwright.config.* …`, and `orq check --caja` prints `e2e  sin check: <reason>`.
- If the Owner already has a `checks` block, it stays byte-identical, and `init` prints the entry to add (`añade a checks: "e2e": {…}`).
- Evidence:
  - perfil: `TestRutasNext`, `TestRutasReactRouter`, `TestMapaUnaLineaPorRuta`, `TestE2EConConfig`, `TestE2EUsaScript`, `TestE2ENoArrancaServidorConWebServer`, `TestE2ESinConfig`.
  - config: `TestE2EServidorValido`.
  - cli (a fixture Next-like repo): `TestInitWebEscribeMapaYE2E`, `TestInitWebSinPlaywrightNoEscribeE2E`, `TestInitSugiereE2EConChecksDelOwner`, `TestCheckCajaSinE2EDicePorQue`.

**2 · The worker guide and the contract.**
- `guias/worker.md` tells a web worker to reproduce the bug or exercise the feature in a real browser (Playwright MCP or `npx playwright`) before REQUEST VERIFICATION, finding the screen through `.orq/mapa.md`.
- When the main checkout has `.orq/mapa.md`, `orq slice contrato` adds one line with its absolute path and the same instruction.
- Evidence: `TestGuiaWorkerWeb`, `TestContratoNombraElMapa`.
- **Deviation from the row (read "Open decisions" 1).** The row says the worker "updates the map line for any route it adds". But `.orq/` is control space: git excludes it and the gate blocks a worker's writes to it. So the guide tells the worker to put the new map line in its reply instead.

**3 · A Playwright test as the exam, and `e2e` as a check, in the box.**
- When a project has `checks.e2e`, the box binds the Playwright browsers dir read-only and sets `PLAYWRIGHT_BROWSERS_PATH`. The dir is `$PLAYWRIGHT_BROWSERS_PATH` or `~/.cache/ms-playwright`.
  - `sandbox.Navegadores` uses the D2a deny list (`tocaProhibido`). A dir inside `~/.config`, outside `$HOME`, or missing is never bound, and the reason is given.
- An exam whose command names `playwright` runs behind the `checks.e2e` server.
- After the checks, the PNGs under the copy's `test-results/` (Playwright's output dir) are copied to `.orq/capturas/<attempt>/`, at most 20.
  - Symlinks are not followed: `WalkDir` and `leerSeguro` both refuse them.
  - The e2e result in the receipt lists `servidor` and `capturas`. The page shows `captura: <path>` under the check.
- `orq check --caja` reports `navegadores de playwright → <dir>` in the bind list, or `no corre en la caja: no hay navegadores de Playwright en <dir>` (exit 1).
- Evidence, real box with a stub toolchain under a temp HOME (no network, no browser):
  - `TestE2EEnLaCaja`: a stub `npm run dev` serves 127.0.0.1:4173 with `python3 -m http.server` and leaves a mark. A stub `npx playwright test` needs that mark, the browsers dir and an open port, then writes a PNG. The result is VERIFICADO, e2e `paso` with `servidor`, and one screenshot under `.orq/capturas/` with the stub's bytes.
  - `TestE2EServidorQueNoAbre`: a server that dies fails at once, with the port and its log.
  - `TestCapturasNoSiguenEnlaces`.
  - `TestExamenPlaywrightArrancaServidor` and `TestExamenPlaywrightConReporteArrancaServidor`: the red comes from the assertion, with the server up.
  - `sandbox.TestNavegadores`, `cli.TestCheckCajaE2E`, `cli.TestDecidirMuestraCapturas`.
- Every test above was seen failing before its code existed.

### Real run (outside `make ci`)

The browsers exist on this machine (`~/.cache/ms-playwright`, headless shell `chromium_headless_shell-1255`).

Setup:
- A throwaway repo under the session scratchpad. `ORQ_STATE`, `ORQ_LIMITS` (missing file, so no `sandbox.ro`), `ORQ_WORKTREES` and `ORQ_CONFIG` were temp. The binary was built into the scratchpad.
- The app is a tiny static `index.html` showing `Total: 0`.
- `@playwright/test` 1.65.1 was installed into the app's `node_modules`. This ran on the host, with network.
- The exam is `e2e/total.spec.ts`: go to `/`, expect `#total` to be `170`. It uses `screenshot: 'on'` and a junit report.

Steps:
1. `orq init` wrote `.orq/mapa.md` (0 routes: a static page has no router) and `checks.e2e = npx playwright test`.
2. A static app has no `dev` script, so I added `"servidor": "python3 -m http.server 4173 --bind 127.0.0.1", "puerto": 4173` by hand.
3. `orq check --caja` showed `npx → ~/.nvm/…/npx`, `navegadores de playwright → ~/.cache/ms-playwright`, and binding `~/.cache/ms-playwright, ~/.local/bin, ~/.nvm/versions/node/v24.12.0`.
4. `orq slice abrir 01 … --cmd "npx playwright test" --bundle playwright.config.ts --reporte results.xml --formato junit` opened red in the box: `Expected: "170" Received: "0"`.
5. The worker clone changed `0` to `170` and committed.
6. `orq slice verificar` reached **VERIFICADO** (exit 0).

The first attempt found a bug. With `--reporte`, the exam rebuilt its command from `e.Cmd` and lost the server. The red was `net::ERR_CONNECTION_REFUSED`, not the assertion. Fixed in `3a5a2a7`, with a test seen failing first. The run above was redone from a fresh clone of the app.

Receipt excerpt (`.orq/recibos.jsonl`):

```json
{"attempt": "S-01/a1", "head": "9f20dd8ea3ebcf488c9da3751eddd94b8ac060f1", "cmd": "npx playwright test",
 "exit": 0, "resultado": "paso", "contado": true, "ejecutados": 1,
 "checks": [
  {"nombre": "lint", "resultado": "omitido", "motivo": "package.json no tiene script lint"},
  {"nombre": "types", "resultado": "omitido", "motivo": "package.json no tiene script typecheck ni typescript con tsconfig.json"},
  {"nombre": "tests", "resultado": "omitido", "motivo": "package.json no tiene script test"},
  {"nombre": "e2e", "cmd": "npx playwright test", "exit": 0, "resultado": "paso",
   "salida": "\nRunning 1 test using 1 worker\n\n[1/1] e2e/total.spec.ts:2:1 › cart shows the total\n  1 passed (714ms)",
   "servidor": "python3 -m http.server 4173 --bind 127.0.0.1",
   "capturas": [".../app/.orq/capturas/S-01-a1/test-results/total-cart-shows-the-total/test-finished-1.png"]},
  {"nombre": "supresiones", "exit": 0, "resultado": "paso"}]}
```

- The screenshot is a real `PNG image data, 1280 x 720`.
- No `http.server` process was left running after the run.
- `orq slice decidir` shows `e2e paso · exit 0 · npx playwright test` with `captura: …/test-finished-1.png` under it.

## Not done, and why

- **Screenshots from the exam itself** are not kept. Only the e2e check's run keeps them, because the item names "the e2e result like any check". The exam's output still names its attachments.
- **`orq check` on the host** starts the server on the host's port. If the Owner already runs a dev server on that port, the wait succeeds at once against the Owner's server. In the box this cannot happen.
- **Server detection is narrow.** It finds only `npm run dev` with `next` (3000) or `vite` (5173), and only when the Playwright config has no `webServer`. Other cases get an `e2e` with no server, which the Owner edits.
- **The map is not refreshed.** `init` writes it once, so later routes are added by hand or from a worker's reply.

## Open decisions for the Owner

1. **Who writes `.orq/mapa.md` after `init`?** The row says the worker. Today the worker sends its line in the reply, because a worker cannot write `.orq/`. The options:
   - (a) keep it this way; the Orchestrator or Owner pastes the line;
   - (b) move the map into the repo (for example `docs/mapa.md`), so a worker commits it and the diff shows it;
   - (c) `orq slice cerrar --aceptado` copies the lines from the reply.
2. Should screenshots from a Playwright exam also go into the receipt, not just those from `checks.e2e`?
3. Should the 60 s server wait be configurable per project? Slow Next builds may need more.

## Suggestions outside this row

- `orq check --caja` probes only the first word of the server wrapper (`mktemp`). It could probe the `servidor` tools by name, so a missing `python3` or `npm` shows before the first close.
- `perfil.Detect` treats any `.html` as `web`, so a Go project with HTML templates gets a map with 0 routes. It is harmless, but it is noise.

## Review fixes, round 1

Fixes for `reviews/d2b-review.md`, strongest layer first. Each test below was seen failing before its code existed. Every commit has `gofmt -l harness` empty, `go vet ./...` and `go test -race ./...` green. `make ci` exits 0 (14 pass, 0 fail).

1. **`test.only` cannot drop the exam** (`be7bf1f`).
   - Every `playwright test` in the exam command and in `checks.e2e` gets `--forbid-only` (`config.SinOnly`).
   - The default supresiones marks now include `.only(`, `describe.only(`, `fit(` and `fdescribe(`. A mark that starts with a letter must start a word, so `benefit(` does not match `fit(`.
   - The flag cannot be added inside an npm script. There the `.only(` mark is the layer that stops it, as the real run below shows.
   - Evidence: `TestTestOnlyNoVerifica` (the reviewer's repro; each layer alone stops it), `TestSupresionesFocused`.
2. **Every exam and `--sano` run behind the `checks.e2e` server** (`54a9296`). This includes `npm run test:e2e`, not only commands that name playwright.
   - A server that is down now fails `--sano` at `abrir`. Before, it failed at the exam.
   - Evidence: `TestExamenNpmScriptArrancaServidor` (`--cmd "npm run test:e2e"`, `--sano` that needs the server's mark). `TestE2EServidorQueNoAbre` now kills the server after `abrir`.
3. **The files behind the server are watched like runner files** (`b6ca6fe`). With `checks.e2e.servidor` set, these join the watch: `vite.config.*`, `next.config.*`, `playwright.config.*` and the program of the server command (`node|tsx|ts-node|deno|python[3] <file>`; `-m`, `-c` and `-e` name no file). A change needs `--acepto-runner`.
   - Evidence: `TestServidorVigilado`.
4. **The feature map lives in `docs/mapa.md`** (`edbc754`). This was the Owner's open decision 1, option (b).
   - `orq init` writes the map only when there is neither `docs/mapa.md` nor an old `.orq/mapa.md`.
   - The worker guide and the contract tell the worker to add the map line in the same commit as the route.
   - An old `.orq/mapa.md` is still read. Then the contract keeps the old line: "send it in your reply".
   - The decision page lists the new lines as `mapa: …`, stripped of control characters.
   - `docs/mapa.md` belongs to every zone. `juntar` does not reject it, and a declared scope does not flag it.
   - Evidence: `TestInitWebEscribeMapaYE2E`, `TestGuiaWorkerWeb`, `TestContratoNombraElMapa`, `TestDecidirMuestraElMapa`, `TestEquipoFrontBackTestEnUnSlice`.
5. **`checks.e2e.espera_s`** (`be54780`): the server wait in seconds, 1 to 600, default 60. It is valid only with `servidor`. The timeout message names the key. This was the Owner's open decision 3.
   - Evidence: `TestEsperaServidor`.

### Real run, again

Same setup as above: a static app, `@playwright/test` 1.65.1, temp `ORQ_*`, and a binary built into the scratchpad. The only differences:
- The script is `"test:e2e": "playwright test"`, so `init` wrote `checks.e2e = npm run test:e2e`.
- The Playwright config has **no** `forbidOnly`.
- `servidor` is `python3 -m http.server 4173 --bind 127.0.0.1`, with `espera_s: 30`.

Steps:
1. `orq init` wrote `docs/mapa.md` (0 routes).
2. `orq slice abrir 01 --test e2e/total.spec.ts --cmd "npm run test:e2e" --bundle playwright.config.ts --reporte results.xml --formato junit` opened red on the assertion, with the server up. Before this round, it opened red with `ERR_CONNECTION_REFUSED`.
3. **Reviewer's repro.** The worker added `e2e/smoke.spec.ts` with `test.only`, leaving `0`. `verificar` printed `1 passed` for the exam, but did not verify: `check supresiones falla (exit 1): e2e/smoke.spec.ts:2: test.only('page loads', …`.
4. The worker dropped `.only`, set `170`, and added `` - `/` → `index.html` · reach: open the site · shows: the cart total (170) `` to `docs/mapa.md`. The result was **VERIFICADO**: 2 run, 0 failed.
5. `orq slice decidir` showed `e2e paso · exit 0 · npm run test:e2e`, two `captura:` lines, and `mapa: - `/` → `index.html` · …`.
6. No `http.server` was left running.

Receipt excerpt (`.orq/recibos.jsonl`, paths shortened):

```json
{"attempt": "S-01/a1", "head": "c04e70d2ad8e47092946672807f546943e522435", "cmd": "npm run test:e2e",
 "exit": 0, "resultado": "paso", "contado": true, "ejecutados": 2,
 "checks": [
  {"nombre": "lint", "resultado": "omitido", "motivo": "package.json no tiene script lint"},
  {"nombre": "types", "resultado": "omitido", "motivo": "package.json no tiene script typecheck ni typescript con tsconfig.json"},
  {"nombre": "tests", "resultado": "omitido", "motivo": "package.json no tiene script test"},
  {"nombre": "e2e", "cmd": "npm run test:e2e", "exit": 0, "resultado": "paso",
   "salida": "\n> test:e2e\n> playwright test\n\n\nRunning 2 tests using 2 workers\n\n[1/2] e2e/smoke.spec.ts:2:1 › page loads\n[2/2] e2e/total.spec.ts:2:1 › cart shows the total\n  2 passed (1.4s)",
   "servidor": "python3 -m http.server 4173 --bind 127.0.0.1",
   "capturas": [".../app/.orq/capturas/S-01-a1/test-results/smoke-page-loads/test-finished-1.png",
                ".../app/.orq/capturas/S-01-a1/test-results/total-cart-shows-the-total/test-finished-1.png"]},
  {"nombre": "supresiones", "exit": 0, "resultado": "paso"}]}
```

Still open:
- Through an npm script, `--forbid-only` is not injected. The `.only(` mark covers it.
  - The mark sees only lines the slice adds. A `.only` already in the red commit is not flagged.
  - Possible next step: resolve `npm run X` from `package.json` and append `-- --forbid-only` when the script is a bare `playwright test`.
- The rejected `test.only` attempt used one of the two repairs. That is correct (the commit did not pass), but the Orchestrator should know it.

## Review fixes, round 2

Fixes for `reviews/d2b-review-2.md`. The strongest layer comes first.

1. **The exam's own tests must pass by name.** When `abrir` has a counted report (junit, tap or gotest-json), it records the names of the tests that fail in the red. They go in `pruebas_examen`, in the estado and in the contract in git.
   - `verificar` and the revalidation after the merge require every one of those names to appear in the report as passed. If one is missing, the slice is not verified, and the message names the missing test.
   - A focused test cannot drop the exam this way, whatever its spelling: `test.only (`, `test['only']`, an alias, `--grep`, `describe.only`.
   - Names: junit is `classname › name`. TAP is the description without its number, since the number shifts when fewer tests run. gotest-json is `Package Test`.
   - `orq slice examen` clears the list when it changes the command or corrects an exam file that is already protected, because the names may change. Adding a new exam file keeps the list.
   - Evidence: `TestFocoNoSaltaElExamen` is the reviewer's repro under `npm run test:e2e` with a JUnit report and no `package.json` script. It covers three spellings. It was seen failing with the check disabled. `TestContarNombres` also covers this.
2. **`--forbid-only` reaches Playwright through package scripts.** `config.SinOnlyEn(dir, cmd)` reads `package.json` in the clean copy. It resolves `npm run X`, `npm run-script X`, `npm test`, `pnpm [run] X` and `yarn [run] X`.
   - When the script is a bare `playwright test` (flags allowed), it appends `-- --forbid-only` for npm. If a `--` is already there, it appends only `--forbid-only`. For pnpm and yarn, it appends `--forbid-only`.
   - A compound script (`playwright test && …`) is left alone, because the flag would reach the last command. Layer 1 covers it.
   - This applies to the exam and to every check (`Check.EjecutableEn`).
   - Evidence: `TestSinOnlyEnScripts` and `TestForbidOnlyPorScriptNpm`. The second has no report, so only this layer can stop it. It was seen failing with the old call.

### Real run (reviewer's repro)

- Setup: a static app with `@playwright/test` 1.65.1 and a config with no `forbidOnly`. The script is `"test:e2e": "playwright test"`, so `orq init` wrote `checks.e2e.cmd = npm run test:e2e`.
- `servidor` is `python3 -m http.server 4173 --bind 127.0.0.1`. The `ORQ_*` variables were temporary, and the binary came from this tree.
- `abrir 01 --cmd "npm run test:e2e" --bundle playwright.config.ts --reporte results.xml --formato junit` opened red on the assertion and recorded `"pruebas_examen": ["total.spec.ts › cart shows the total"]`.
- The worker committed `e2e/smoke.spec.ts` with `test.only ('page loads', …)` (with a space), and `index.html` still showed `0`. `orq slice verificar 01` exited 1:

```
> webapp@1.0.0 test:e2e
> playwright test --forbid-only
Error: item focused with '.only' is not allowed due to the '--forbid-only' CLI flag: "smoke.spec.ts page loads"
   at smoke.spec.ts:2
orq slice: 1 prueba(s) del examen no aparecen como pasadas en el reporte (¿un test enfocado, --grep o un alias de .only?): total.spec.ts › cart shows the total; check e2e falla (exit 1): 
(reparación 1 de 2)
```

Both layers fired: the exam got `--forbid-only` through the npm script, and the report lacked the exam's test by name.

- The worker removed `.only` and set `170`. `verificar` exited 0: `playwright test --forbid-only`, `2 passed`, `S-01/a1 verificado`, 2 run and 0 failed. No `http.server` was left running.

Still open:
- When the exam runs without a report (`--sin-conteo`), there are no names to require. A compound npm script then relies on the `.only(` mark alone.
