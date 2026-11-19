# D2b review, round 2 (0f03881..9818369)

Setup: binary built from 9818369. Static app, @playwright/test 1.65.1, config with no forbidOnly, `"test:e2e": "playwright test"`. `init` wrote `checks.e2e.cmd = npm run test:e2e`. I added `servidor: python3 -m http.server 4173`. Temp ORQ_*.
- (b) holds: `abrir 01 --cmd "npm run test:e2e" … --reporte results.xml --formato junit` opened red on the assertion, with the server up.
- (a) holds as reported: a worker `test.only('page loads', …)` was rejected by supresiones, at repair 1 of 2.
- Item 2 is not a defect: with `.only` already at the red commit, `npx playwright test --forbid-only` exits 1 with "focused item", and `abrir` rejects it as not an assertion red (`nucleo.go:231`, 0 failures in junit). Only `--sin-conteo` would accept it, and that already accepts any non-assertion red.
- Item 3 is not a defect: `docs/mapa.md` is exempt only from zone/scope (`equipo.go:243,365`). Bundle and protected paths are still checked, the page adds lines and does not replace the file list, control characters are stripped (`pagina.go:73`), and nothing runs the file.

## Defect

1. **The `.only(` mark is a plain substring, so `test.only (` (with a space) gets VERIFICADO without the feature whenever the exam is an npm script.** This is the default `init` command. `SinOnly` (`config.go:605`) only adds `--forbid-only` to a literal `playwright test`, so through `npm run test:e2e` the only guard is the `.only(` mark (`config.go:79-80`, matched by `tieneMarca` in `guardias.go:187`). JS accepts `test.only ('x', …)`, `test.only\n(`, `test['only'](…)` and `const t = test.only; t(…)`. None of these contain `.only(`.
   Repro: continuing slice 01 above, the worker commits `e2e/smoke.spec.ts` with `test.only ('page loads', … body visible)`, with index.html still at "0". `orq slice verificar 01` prints `S-01/a1 verificado: el examen pasa intacto` (1 run, 0 skipped), `e2e paso · supresiones paso`, alignment ok, exit 0.
   Fix: do not rely on the text mark. Resolve `npm run X` to its script and append `-- --forbid-only`. Or require the exam's own test names in the junit report. That last fix also covers grep, describe.only and aliasing.

VERDICT: fix
