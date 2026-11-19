# D2b review (2104abd..5923aae)

Real run: static app, @playwright/test 1.65.1, temp ORQ_*, python3 http.server as `checks.e2e.servidor`. `init` wrote `e2e`. The exam opened red in the box on the assertion (Received "0"), with the server up. A wrong fix ("17") stayed red, failing both the exam and e2e. A screenshot PNG was listed in the receipt. No 4173 server was left running. The browsers bind passes the deny list (`tocaProhibido`). Commits that edit `playwright.config.*` or `package.json` hit the runner watch (`--acepto-runner`).

## Defects

1. **A `test.only` in a new spec file gets VERIFICADO without the feature.** The exam runs `npx playwright test` over the whole test dir. Playwright then runs only the `.only` test, and the exam's test is dropped from the junit report. The count check (`verif.go:18`) sees 1 run and 0 skipped. Nothing ties the count to the exam's own test, and the supresiones marks (`config.go:70`) do not include `.only(`. `CI=1` is set (`caja.go:96`), but that only helps when the Owner's config has `forbidOnly: !!process.env.CI`.
   Repro: abrir S-01 as above (config without forbidOnly). In the worker clone, add `e2e/smoke.spec.ts` with `test.only('page loads', …body visible)` and commit, leaving index.html at "0". `orq slice verificar 01` prints `verificado: el examen pasa intacto`, exit 0. The receipt says `resultado: paso, ejecutados: 1`, e2e `paso`, supresiones `paso`, alignment ok.
   Fix: run the exam (and e2e) with `--forbid-only`, or add `.only(`/`describe.only` to the supresiones marks, or require the exam's test names in the report.

2. **An exam that runs Playwright through an npm script never gets the server.** `nucleo.go:97` wraps the exam only when `e.Cmd` contains "playwright". But `perfil/web.go:118-124` makes `checks.e2e.cmd` `npm run <script>` whenever a script runs `playwright test`. If the planner reuses that command for the exam (`npm run test:e2e`), the exam runs with no server. In a Next or Vite app it fails with `ERR_CONNECTION_REFUSED`, at abrir and again at every verificar. So the red is accepted for the wrong reason and the slice can never turn green. The same holds for `--sano` (`sanoEn`, which never wraps).
   Fix: wrap any exam whenever `checks.e2e.servidor` is set, or resolve the npm script before matching.

Lower risk, not blocking: the runner watch does not cover the files behind `servidor` (e.g. `node server.js`, `vite.config.*`, `next.config.*`). With a custom server, a worker can point it at a stub without any guard noticing. For the default `npm run dev` path, `package.json` is watched.

VERDICT: fix
