# Spec — `orq board` and skill `orq-board`

Roadmap: Owner requirements 2026-11-08, slices 5 and 10. Depends on `stoppers-autonomy.md` and `worker-state-handoff.md`.

## What the Owner gets

One screen that answers "how are we doing?" without jargon. Stoppers (things only the Owner can unblock) come first. Then one colored card per slice: what we are building, in product words, its % done, what is still missing, and who is working on it. Then a strip with one row per worker, and the project velocity. `orq board --html` writes one static file that opens offline. The `orq-board` skill lets Atena run `orq board --json` and explain the result in plain language. The skill only reads; it never writes state.

Rule: when a value cannot be computed from files, show `unknown`. Never estimate or invent a value.

## Sources (read-only)

| Data | File |
|---|---|
| Slice state | `.orq/slices/NN.json` (`slice.Estado`: `titulo`, `fase`, `motivo`, `equipo`, `abierto`, `recibo`) |
| Closed slices | `.orq/slices.jsonl` |
| Plan tasks | `.orq/planes/NN.json` (`tareas[].estado` pendiente/en_curso/hecha, `dificultad`, `worker`, `que`) |
| Stoppers | `.orq/stops.jsonl` (new, see stoppers spec) |
| Workers | `.orq/workers/<id>/state.json` (new, see worker spec) |
| Task events | `.orq/tasks.jsonl` (new): `{"ts","slice","task","event":"done"|"reopened","worker"}`, appended by `orq plan hecha` / `orq plan soltar` |

## Metrics (exact)

- **Task weight**: `baja`=1, `media`=2, `alta`=3. An empty value counts as `media`, the same rule as `plan.Leer`.
- **Slice % done** = round(100 × Σweight(estado=hecha) / Σweight(all tasks)). If there is no plan file or the plan has zero tasks, show `unknown`. Exception: a slice that is VERIFICADO, INTEGRANDO, or closed as accepted shows 100.
- **Overall %** = weighted mean of the slice % values, where each slice's weight is the Σweight of its tasks. Slices with `unknown` are left out, and the board shows `n of m slices measured`. If every slice is unknown, overall is `unknown`.
- **Missing** = the `que` field of each task not in `hecha`, in plan order, at most 3 shown, then `+N more`. A missing `que` shows the task id.
- **Who** = the distinct `tareas[].worker` values of tasks `en_curso`, joined with their worker state. With no worker, show `nobody`.
- **Status color**: red = has a `blocking` stopper or `fase`=BLOQUEADO · amber = has a `soon` stopper, or a worker in that slice is `stuck`/`dead`/`timeout` · green = VERIFICADO/INTEGRANDO/accepted · blue = IMPLEMENTANDO with no issue · grey = no plan. Every card also prints the status word, so the board does not depend on color alone.
- **Worker strip**, from state.json: `task`, heartbeat age = now − `heartbeat` (shown as `Nm`), `commits`, tests `passing/total` (or `unknown`), elapsed = now − `started` against `budget_min` (or `budget: none`), `context_pct` (or `unknown`), and the watcher verdict. `unknown` = the session is registered but `orq board` ran outside the runner, so nobody could ask whether it is alive.
- **Velocity (tasks/hour)** = count of `done` events in `tasks.jsonl` in the last 24 h ÷ hours between the first such event and now, with a floor of 1 h. Fewer than 2 events shows `unknown`.
- **Rework** per slice = count of `reopened` events + `(recibo attempt − 1)` from `intento`.

## Order of output

1. Stoppers with state `open`: blocking, then soon, then fyi; oldest first within each group.
2. Overall % with a text progress bar.
3. Slice cards: red, amber, blue, grey, green.
4. Worker strip.
5. Velocity and rework.

Terminal output uses ANSI color only when stdout is a TTY and `NO_COLOR` is unset. HTML output is one file with inline CSS and no JS or network requests, written to `.orq/board.html` (or to the `--out` path).

## `orq board --json` (schema v1)

```json
{
  "schema": "orq.board/v1",
  "generated_at": "RFC3339",
  "overall": {"pct": 0, "measured": 0, "total": 0},
  "stoppers": [{"id": "ST-0003", "type": "api_key", "severity": "blocking",
                 "slice": "S-04", "tasks": ["T2"], "text": "...", "age_min": 42}],
  "slices": [{"id": "S-04", "title": "...", "phase": "IMPLEMENTANDO",
              "color": "red|amber|green|blue|grey", "pct": 0,
              "tasks": {"done": 0, "total": 0, "weight_done": 0, "weight_total": 0},
              "missing": ["..."], "who": ["w-s04-t2"], "rework": 0, "blocked_reason": ""}],
  "workers": [{"id": "w-s04-t2", "slice": "S-04", "task": "T2", "phase": "running",
               "verdict": "ok|stuck|dead|timeout|exited|unknown",
               "heartbeat_age_s": 0, "commits": 0,
               "tests": {"passing": 0, "total": 0},
               "elapsed_min": 0, "budget_min": 0, "context_pct": 0}],
  "velocity": {"tasks_per_hour": 0, "window_h": 24}
}
```

When a number is unknown, the field is `null`. It is never `0`. The text renderers print `null` as `unknown`.

## Acceptance tests

1. **Given** S-01 with tasks of weight alta (done), baja (pending), and media (pending), **when** `orq board --json` runs, **then** `pct` = 50 and `missing` lists the two pending `que` values.
2. **Given** a slice with no `planes/NN.json`, **when** the board renders, **then** the card shows `unknown`, the color is grey, and `overall.measured` leaves it out.
3. **Given** an open `blocking` stopper on S-02 and an open `fyi` stopper on S-01, **when** the board renders, **then** the blocking stopper is listed first and the S-02 card is red.
4. **Given** a worker state.json with no `context_pct` and no tests, **when** `--json` runs, **then** `context_pct` and `tests` are `null`, and the terminal shows `unknown`.
5. **Given** any project, **when** `orq board --html` runs, **then** exactly one file is written, it contains no `<script` and no `http`, and no file under `.orq/` changes except that output file.
6. **Given** one `done` event in `tasks.jsonl`, **when** the board renders, **then** velocity is `unknown`.

## Out of scope

Live refresh or a server, charts over time, cost in money or tokens beyond `context_pct`, editing anything from the board, and LLM-written summaries inside the command (Atena explains them through the skill).
