# D3b review (80eff38..8238520)

Real run (CC 2.1.328, temp ORQ_*): skill `eco-hola` with one regex case: `orq skill eval … --max-usd 1` exits 0. Output: `eco-hola 4fb523b38dc6 tipo general: nota 1.00 en 1 corridas · USD 0.05`. One row is stored, and `orq skill notas` lists it. The account has plugin eval, and the wrapper loads.

Defects:
1. **A failed run is stored as a full, valid 0.00 score.** Code: `skilleval/correr.go:42-49` and `:84-88`. It never reads `cases[].arms.with[].error` or `.aborted`.
   - Repro (real): copy the skill and add `timeout_seconds: 1` to `prompt.md`.
   - The CLI exits 1. The JSON has `partial:false`, `overallScore:0` and `with[0].error:"timed out after 1s"`.
   - orq stores `eco-abort d1b6d7b3b88b general 0.00 … USD 0.00` with no partial flag.
   - The same happens for auth, rate-limit or sandbox errors. The 0 then counts in `Ordenar`: under `skills.minimo` the skill goes to "fuera". In `comparar` it becomes a baseline that any edit beats ("conservar").
   - Fix: treat a run with `error` or `aborted` as partial or invalid, and refuse to store it when every run failed.
2. **Ranking ignores the hash, so a score from another version of the skill is applied to the current files.** Code: `skilleval/ordenar.go:22-29` (keyed by skill name only) and `app/workers.go:225-233` (it never hashes the installed skill).
   - Repro: eval v1, edit to v2, eval v2 (low score), and `comparar` says `descartar`. Revert the files to v1. Launch now ranks v1 with v2's rejected score.
   - The reverse also happens: an edited skill that was never evaluated keeps its old score.
   - Fix: hash the skill as installed and take only rows with that hash; otherwise show "sin nota".
3. **One bad line in `skill-notas.jsonl` blocks every worker launch.** Code: `app/workers.go:229-232` returns the error from `Notas` (`notas.go:56`).
   - Repro: `echo '{' >> $ORQ_STATE/skill-notas.jsonl`, then launch any worker. It fails with "notas de skills: …".
   - A torn append can leave such a line. The list is only advice, so launch should warn and go on without scores.
4. Minor: `orq skill eval .` takes the name `filepath.Base(".") == "."` (`skilleval.go:47`). The row is stored as skill ".", which never matches a skill at launch.

Checked and fine: exit 2 is stored as partial, and ranking and `comparar` ignore it. A cancelled run (exit -1) is not stored. A skill with no score is listed last as "sin nota" without a crash. Order (score descending, then by name) and the threshold (`<` minimo goes out) are correct.

VERDICT: fix
