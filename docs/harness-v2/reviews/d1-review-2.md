# D1 fix review (0ea3528), independent

To check each item I reran the D1 reproduction in a temporary test, deleted afterwards. The commit's own tests and `TestDecidirHistoria*` pass.
1. Clock restarts on re-verify: **fixed**. `slice/metricas.go:129` keeps the first `paso`. With paso 10:00, paso 10:30 and decision 10:40, the median is now 40 (it was 10).
2. VERIFICADO→BLOQUEADO: **fixed**. `slice/metricas.go:137` stops the clock on any transition out of VERIFICADO except to INTEGRANDO. paso, then the BLOQUEADO event, then +48h gives SinDecidir24h=0. Block, re-verify and decide: the clock starts again from the new `paso` (10 min), which is correct.
3. Mixed commits: **fixed for Cambiados**. `slice/pagina.go:51` diffs red→`rec.Head`. `:46` raises `!!` when the clone tip ≠ `rec.Head` (only for single-clone slices).
4. Story from another slice: **fixed**. `app/pagina.go:29` uses the story linked to this slice when one exists. Otherwise `:45` raises `!!` when more than one slice is open.
5. Stamp before the run: **fixed**. `slice/nucleo.go:295,402` stamps `now + time.Since(inicio)`.
Hints: **fixed**. Both now include `--causa`: `slice/slice.go:382` and `app/relevo.go:92`.

New, reproduced (decision page wrong): `slice/pagina.go:52-59` does not recompute `Fuera` for `rec.Head`. It only filters the tip-based `Fuera` down to files that `rec.Head` changed. Repro: Alcance=[total.txt]. The verified commit adds `fuera.txt`. The worker then deletes it in a new commit. Result: `Cambiados=[fuera.txt total.txt]` but `Fuera=[]`, so the page shows no out-of-scope `!!` for a verified commit that touches an out-of-scope file. Only the generic "tip moved" `!!` remains.
Fix: test each `fs` entry against the allowed paths (`Test`+`Bundle`+`Alcance`+each worker's `Rutas`), the same rule `Alinear` uses.

VERDICT: fix
