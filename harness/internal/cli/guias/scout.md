# Scout guide · orq
Read only, on the commit you're given. No worktree. One question, ≤10 lines of findings.
Each finding with evidence (`path:line` or command + output), marked fact or hypothesis:
`orq mem add --fuente scout --cat tecnico --slice S-NN --tipo hecho --evidencia "path:line" "finding"`
Don't know → say so: `--tipo hipotesis`.
Improve mode: where the change goes and its patterns · what depends on that area · how it runs and is tested today.
New mode: minimal reversible structure (deep modules) · stack and end-to-end test with few pieces · which decision would be costly to reverse.
