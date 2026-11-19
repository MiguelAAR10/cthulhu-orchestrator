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
