# D2b Implementation Plan · the worker checks web apps like a person

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax.

**Goal:** for a web project, `orq init` writes a feature map `.orq/mapa.md` and an `e2e` Playwright check; `verificar` runs Playwright (check or exam) in the box with the app's server started on localhost and the browsers bound read-only; the receipt and page list the screenshots.

**Architecture:** detection lives in `perfil` (routes, e2e check). The server is a property of the `e2e` check (`servidor`, `puerto`) in `config`, turned into one shell script by `Check.Ejecutable()`, so host (`orq check`) and box (`verificar`) run the same thing. The box binds the Playwright browsers dir through the toolchain deny list (`sandbox.Navegadores`). Screenshots under the copy's `test-results/` are copied to `.orq/capturas/` and listed in the e2e result.

**Tech Stack:** Go standard library; bwrap box from D2a.

**Spec:** `docs/harness-v2/plan.md` › "Route" D2, "Approach" mechanism 1; the D2b brief; `docs/harness-v2/d2a-report.md`.

## Global Constraints
- Standard library only; `internal/gate` untouched; `internal/arch_test.go` green.
- Tests use temp `ORQ_STATE/ORQ_LIMITS/ORQ_WORKTREES/ORQ_CONFIG`; no network, no real browser, no real `claude`/`codex` in `make ci`.
- Every commit: `gofmt -l harness` empty, `go vet ./...`, `go test -race ./...`, `make lint`.
- The browsers dir is bound only when the D2a deny list allows it (`tocaProhibido`), never `$HOME` or an ancestor of a denied dir.

## Review Focus
1. A Playwright config with `webServer`: orq must not start a second server on the same port (Playwright's own server would fail "port in use"). → Task 1 test `TestE2ENoArrancaServidorConWebServer`.
2. A server that never opens its port, or dies: the check fails in bounded time with the server's log, not a hang. → Task 2 test `TestE2EServidorQueNoAbre`.
3. A screenshot path that is a symlink out of the copy: never followed. → Task 2 test `TestCapturasNoSiguenEnlaces`.
4. No browsers on the host (`~/.cache/ms-playwright` missing): `orq check --caja` says so and exits 1. → Task 3 test `TestCheckCajaE2E`.
5. A project whose checks block predates D2b: `orq init` leaves the block byte-identical and prints the `e2e` entry to add. → Task 1 test `TestInitSugiereE2EConChecksDelOwner`.

---

### Task 1: feature map and e2e check from `orq init`

**Files:** Create `harness/internal/perfil/web.go`, `harness/internal/perfil/web_test.go`. Modify `harness/internal/config/config.go` (Check fields + validation), `harness/internal/config/checks_test.go`, `harness/internal/cli/v03.go` (runInit), `harness/internal/cli/proyecto_test.go` or a new `harness/internal/cli/web_test.go`.

**Interfaces — produces:**
- `config.Check` gains `Servidor string \`json:"servidor,omitempty"\`` and `Puerto int \`json:"puerto,omitempty"\``; valid only on `checks.e2e`, both or neither, `1 ≤ puerto ≤ 65535`.
- `const config.E2E = "e2e"`.
- `type perfil.Ruta struct{ Path, Archivo string }`
- `func perfil.Rutas(root string) []Ruta` — from `git ls-files`: Next app router `(src/)?app/**/page.{tsx,jsx,ts,js,mdx}` (drop `(group)` and `@slot` segments), Next pages router `(src/)?pages/**/*.{tsx,jsx,ts,js}` (skip `_app`, `_document`, `_error`, `api/`; `index` is its parent), React Router `path="..."` / `path: "..."` in files that import `react-router`. Sorted by path, no repeats.
- `func perfil.Mapa(rs []Ruta) string` — markdown: a header that says who fills what, then one line per route `` - `PATH` → `FILE` · reach: ? · shows: ? ``; with no routes, a line saying none were found in router files.
- `func perfil.E2E(root string) (config.Check, string)` — the check, or "" Cmd and the reason there is none (`sin playwright.config.*`). Cmd: `npm run NAME` for the first script (by name) whose value contains `playwright test`, else `npx playwright test`. Servidor: none when the config text contains `webServer`; else `npm run dev` when a `dev` script exists, puerto 3000 with `next`, 5173 with `vite`; otherwise no servidor.
- `runInit`: in a web project (`perfil` signal `web`) writes `.orq/mapa.md` only when absent; adds `checks.e2e` to a new block; with an existing block lacking `e2e`, prints the JSON entry to add and leaves the file byte-identical.

- [ ] Tests (fail first): `TestRutasNext` (fixture: `app/page.tsx`, `app/(shop)/cart/page.tsx`, `app/users/[id]/page.tsx`, `pages/about.tsx`, `pages/_app.tsx`, `pages/api/x.ts` → exactly `/`, `/about`, `/cart`, `/users/[id]` with their files); `TestRutasReactRouter` (`src/main.tsx` importing `react-router-dom` with `path="/login"` and `path: "/settings"`); `TestMapaUnaLineaPorRuta`; `TestE2EConConfig` (`playwright.config.ts`, `next` dep, `dev` script → `npx playwright test`, `npm run dev`, 3000); `TestE2EUsaScript` (`"test:e2e": "playwright test"` → `npm run test:e2e`); `TestE2ENoArrancaServidorConWebServer`; `TestE2ESinConfig` (Cmd "", reason names `playwright.config`); config `TestE2EServidorValido` (servidor without puerto, puerto 0, servidor on `lint` are refused); cli `TestInitWebEscribeMapaYE2E` (fixture Next-like repo: `.orq/mapa.md` has the `/cart` line; `.orq/config.json` has `checks.e2e.cmd`; a second `init` does not overwrite an edited `mapa.md`), `TestInitSugiereE2EConChecksDelOwner`.
- [ ] Implement; green; commit `init: feature map and e2e Playwright check for web projects`.

### Task 2: Playwright in the box — server, browsers, screenshots

**Files:** Modify `harness/internal/config/config.go` (`Ejecutable`), `harness/internal/sandbox/toolchain.go` (`Navegadores`), `harness/internal/slice/caja.go` (`correrEn`), `harness/internal/slice/checks.go` (`ResultadoCheck`, capturas), `harness/internal/slice/nucleo.go` (exam wrap). Tests in `harness/internal/slice/web_test.go`, `harness/internal/sandbox/toolchain_test.go`, `harness/internal/config/checks_test.go`.

**Interfaces — produces:**
- `func (ch config.Check) Ejecutable() string` — `Cmd` when no `Servidor`; else a POSIX sh script that starts `Servidor` in the background (its own process group, log in a temp file), waits up to 60 s for `127.0.0.1:Puerto` (`bash -c ': </dev/tcp/127.0.0.1/P'`), fails with exit 1 and the log tail if the port never opens or the server exits, runs `Cmd`, and kills the server group on exit; the script's exit is `Cmd`'s.
- `func sandbox.Navegadores() Herramienta` — `Nombre: "navegadores de playwright"`, `Ruta`/`RO` = `$PLAYWRIGHT_BROWSERS_PATH` or `~/.cache/ms-playwright`; `Motivo` when missing or when the deny list refuses it.
- `correrEn`: when the Repo has an `e2e` check, binds `Navegadores().RO` and sets `PLAYWRIGHT_BROWSERS_PATH` in the box env.
- `CorrerChecks` runs `c.Ejecutable()`; the result keeps `Cmd` and gains `Servidor string \`json:"servidor,omitempty"\`` and `Capturas []string \`json:"capturas,omitempty"\``.
- `checksEn`: after the checks, copies regular `*.png` files under the copy's `test-results/` (no symlinks followed, `leerSeguro`, at most 20) to `<OrqDir>/capturas/<attempt with / as ->/` and lists those paths on the `e2e` result.
- Exam: when its command names `playwright` and the `e2e` check has a `Servidor`, `examen` runs `e2e.Ejecutable()` with the exam command in place of `Cmd`.

- [ ] Tests (fail first): config `TestEjecutableSinServidor` (equals Cmd); sandbox `TestNavegadores` (temp HOME with `.cache/ms-playwright` → RO is it; missing → Motivo; `PLAYWRIGHT_BROWSERS_PATH` inside `~/.config` → Motivo); slice, real box, stub toolchain under a temp HOME (like `TestCheckVeLaToolchainDelHome`): `TestE2EEnLaCaja` — a stub `npm` whose `run dev` serves 127.0.0.1:PORT with `python3 -m http.server` and writes `dev-arrancado`; a stub `npx` whose `playwright test` requires that marker, reads `$PLAYWRIGHT_BROWSERS_PATH/chromium-x/ok`, connects to the port, and writes `test-results/a/shot.png`; verificar → VERIFICADO, the receipt's `e2e` result is `paso` and `Capturas` has one path under `.orq/capturas/` whose bytes equal the stub's; `TestE2EServidorQueNoAbre` (stub server exits at once → e2e `fallo`, salida names the port, under 70 s); `TestCapturasNoSiguenEnlaces` (`test-results/x.png` symlink → not listed); `TestExamenPlaywrightArrancaServidor` (exam `npx playwright test` red at open, green after a commit; the stub test needs the server).
- [ ] Implement; green; commit `verificar: Playwright in the box with the app server on localhost, browsers bound, screenshots kept`.

### Task 3: `orq check --caja`, the page, the worker guide and contract

**Files:** Modify `harness/internal/app/check.go`, `harness/internal/cli/check.go`, `harness/internal/cli/decidir.go`, `harness/internal/cli/guias/worker.md`, `harness/internal/app/relevo.go` (Contrato). Tests in `harness/internal/cli/check_test.go`, `decidir_test.go`, and the existing guide/contract tests' files.

**Interfaces:**
- `app.CheckCaja` gains `Navegadores *sandbox.Herramienta` (set on `e2e`); Herramientas computed from `Ejecutable()`.
- `func app.SinE2E(cwd string) string` — in a web project with no `checks.e2e`, the reason (`perfil.E2E`); "" otherwise.
- `runCheckCaja` prints `e2e  sin check: REASON` (exit unchanged) and, for `e2e`, `navegadores de playwright → DIR` in the bind list or `no corre en la caja: MOTIVO` (exit 1).
- Page: under each check, `captura: PATH` per screenshot.
- Guide (worker.md), Web bullet: reproduce the bug or exercise the feature in a real browser (Playwright MCP or `npx playwright`), finding it with `.orq/mapa.md` of the main checkout, before REQUEST VERIFICATION; the map line of any route you add goes in your reply (you cannot write `.orq/`).
- Contract: when `<root>/.orq/mapa.md` exists, one line with its absolute path and the same instruction.

- [ ] Tests (fail first): `TestCheckCajaE2E` (no browsers dir → exit 1, says `ms-playwright`; with it → lists it), `TestCheckCajaSinE2EDicePorQue` (web fixture without Playwright config → `e2e  sin check:` and `playwright.config`), `TestDecidirMuestraCapturas`, `TestGuiaWorkerWeb` (guide names `.orq/mapa.md` and `playwright`), `TestContratoNombraElMapa`.
- [ ] Implement; green; commit `check, page, guide: Playwright and the feature map for web workers`.

### Task 4: real run and report
- [ ] With `~/.cache/ms-playwright` present: scratchpad throwaway repo, tiny static app (`index.html`), `@playwright/test` from the npm cache or global install, `checks.e2e` with `servidor: python3 -m http.server 4173`, a red Playwright exam, a fix, `orq slice verificar` → VERIFICADO; paste the receipt excerpt. If Playwright cannot be installed offline, say so and stop.
- [ ] `make ci`, read it; write `docs/harness-v2/d2b-report.md`; commit.
