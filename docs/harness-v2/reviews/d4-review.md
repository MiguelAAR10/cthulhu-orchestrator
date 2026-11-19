# D4 review (aafda23..39b88a5)

Pause, gate/CLI orchestrator-only checks, failing verify never merging, and the repeat-close no-op look sound. Three real defects, reproduced with throwaway tests (since removed):

1. **Merge-on-close runs for a name that is no session.** `harness/internal/app/integrar.go:22-29` checks only the regexp `worker-.+-sNN` and the phase; `cli/sesion.go:79` calls it before `CerrarSesiones` checks the name exists. Repro (app test, `proyectoDePrueba` + `lanzar` + commit `ok` in the clone): `p.CerrarWorker(ctx, "worker-nadie-s01")` returns nil, slice 01 goes to VERIFICADO, and 1 `integracion ok` row is written. Through the CLI, `orq sesion cerrar worker-typo-s01` verifies the slice, then exits 1 "no hay sesión", while the real worker is still running and its pane stays open. On a team slice, a wrong name also runs `Juntar` and writes a row. Fix: require `ses[nombre]` to exist (and, for a team, that the name is a member) before integrating.
2. **A story assignment can hide an open slice from `orq status`.** `cli/historia.go:103-127` lists only children of stories that exist, and "sin historia" takes only `Historia == ""` (`:131`). Repro (cli test): open story `pagos` and child 01, then change `"historia": "pagos"` to `"otra"` in `.orq/slices/01.json`. Status shows `pagos «Pagos»` and S-01 appears nowhere. The report itself says this field can be edited by hand.
3. **One bad story file hides every slice.** `slice/historia.go:84-93` (`Historias`) fails on the first unreadable file, and `cli/historia.go:97-101` then returns before listing any slice. Repro: add `.orq/historias/Mala.json` (or a corrupt `x.json`). Status prints `historias: no se pudieron leer …` and S-01, which has no story, is gone too.

Minor: the alert dedup key includes `Detalle` (`app/alertas.go:72-81`), and `fuera_de_alcance` puts the file list in `Detalle` (`:61`). So one out-of-scope slice is journaled again each time a new file is added; it is not one alert per slice.

Not defects: moving a child between stories does not get it past its card or exam, since both are per slice and are checked in `LanzarWorker`/`Verificar`. A story id that is missing makes `Pausado` fail closed.

VERDICT: fix
