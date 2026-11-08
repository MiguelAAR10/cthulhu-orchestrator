---
name: orq-supervisor
description: Use when you are the Orchestrator of orq and need to supervise running workers, when a "[orq · estado]" message from `vigia` arrives (worker dead, stuck, timeout or exited), or when a worker asks for verification. The loop is `orq worker list`, act on what is not ok, `orq slice verificar`, close, then `orq gc`. Code watches the workers; you do not poll them with "are you done?" messages.
---

# Supervisor loop · orq

Workers write their own state to `.orq/workers/<id>/state.json` (`orq worker beat|done`). The watcher in `orq watch` is code. It judges each worker as `ok`, `stuck`, `dead`, `timeout`, or `exited`, logs every change to `.orq/watch.jsonl`, and sends you one message per change. It never kills a process and never removes a worktree. Those actions are yours.

## Loop
1. `orq watch` runs the watcher once (it also measures disk). Then run `orq worker list` (or `--json`).
2. Act on each worker that is not `ok`:
   | Verdict or phase | What you do |
   |---|---|
   | `asks_verification` | `orq slice verificar --slice NN`. If it is green, close the slice (step 3). If it is red, send the worker the failure (`orq msg send --para worker-...`). |
   | `stuck` (no heartbeat and no new commits) | Read its herdr pane. Answer it if it is waiting on input. Otherwise ask it for one `orq worker beat --summary`. |
   | `dead` (process gone) | Read its last state and commits. Relaunch the task (`orq plan soltar ID`, then `orq slice lanzar --tarea ID`). Never delete its worktree by hand. |
   | `timeout` | At 1.0× the budget, check its progress. At 1.5×, the watcher raises a `confirmation/soon` stopper. Follow the autonomy mode (see below). |
   | `blocked` | It is waiting on a stopper (`orq stop list --open`). Work that does not depend on it continues. Launch other ready tasks (`orq plan listas`). |
   | `question` | Answer it through `orq msg send`. Escalate to Atena only if it is a product question. |
   | `exited` | Nothing to do. Its task is closed or released. |
3. Close: `orq slice cerrar --aceptado` only after the Owner accepts the demo. Product acceptance always reaches the Owner.
4. `orq gc` retires the finished worktrees safely. Anything doubtful goes to quarantine. Never use `--force`.

## Autonomy
- `orq autonomy` shows the mode. In `guided` mode, every `decision` and `confirmation` stopper goes to Atena.
- In `free` mode, you may resolve `decision` or `confirmation` stoppers of severity `soon` or `fyi` that are inside the approved scope and budget: `orq stop resolve ST-NNNN [--choice X] --note "auto: <reason>"`. The decision is saved to memory automatically.
- These always go to the Owner: `blocking` stoppers; `credential`, `api_key`, and `access` stoppers; product acceptance; anything irreversible (push, publish, spend, delete shared data, message real users); any change to the exam or contract; anything outside the scope or over the budget.

## herdr
Use herdr only to **read** a worker's pane and to **answer** it. Do not use it to ask for status. The status is in `orq worker list`.
