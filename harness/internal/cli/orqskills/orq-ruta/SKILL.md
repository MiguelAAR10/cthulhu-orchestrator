---
name: orq-ruta
description: Lane router for orq. Use before starting any Owner request (Atena), on every incoming `historia` (Orchestrator), and when a message carries `[lane: fix]` (Planner). Picks the lightest lane that still ends in a receipt and a demo - answer, fix or slice - and says when a fix must be upgraded.
---

# Lanes · orq

Every request takes the **lightest lane that still ends in proof**. Proof is the same in every lane that changes code: a red test turned green by `orq slice verificar`, and a demo the Owner runs. What changes between lanes is how many sessions you open to get there.

## Pick the lane (Atena, before interviewing)

| Lane | Take it when | Sessions opened |
|---|---|---|
| **answer** | The Owner asks a question or wants status. Nothing changes in the product. | none |
| **fix** | All five hold (below). | Planner (red test only) + 1 worker |
| **slice** | Anything else. | full TCADD: scouts, Planner, Astra, team |

**fix** needs all five:
1. One observable behaviour changes, and you can write it now as a single given/when/then with real data.
2. One repo.
3. Three files or fewer are expected to change.
4. The Owner already knows what "right" looks like; there is no product decision left.
5. It touches none of: personal or health data, money, auth or security, schema or migrations, external accounts, anything irreversible.

One of the five is uncertain → ask the Owner one question. Still uncertain → **slice**.

## Lane: answer
Atena answers from memory and from reality (`orq slice estado`, `orq plan ver`, `orq metricas`). Three lines, product words.

## Lane: fix

**Atena.** Story = value in one line, the given/when/then, a demo of two steps or fewer. Read it back; on "aprobado":
```
orq msg send --para orquestador --tipo historia "[lane: fix] <story>"
```

**Orchestrator.** Go straight from story to worker:
1. Planner open or `orq sesion abrir planificador`; forward the story with its `[lane: fix]` tag.
2. With the Planner's flags: `orq slice abrir NN --titulo --test --cmd --reporte --formato --alcance <paths> --sin-scouts "lane fix: <one-line reason>"`.
3. One worker, writing directly in the slice worktree: `orq slice lanzar --agente claude --dificultad baja --alcance "<what it leaves done>"`. Use `media` only when the cause is unknown.
4. On the worker's report: `orq slice verificar`, then run the demo yourself.
5. `orq msg send --para atena --tipo cierre --intento S-NN/aK`, then `orq slice cerrar --aceptado|--rechazado "reason"`.

Done when `orq doctor` is clear.

**Planner.** On `[lane: fix]` you deliver the red test and stop: write the acceptance test that fails by assertion with a counted report, commit `test(S-NN): red scenario`, and reply to the Orchestrator with the exact `orq slice abrir` flags including `--alcance`.

## Upgrade: fix → slice
The lane is a bet that the work is small. It is lost the moment any of these happens:
- `orq slice verificar` leaves the slice BLOCKED.
- `orq alinear` exits 3, or the worker asks for paths outside `--alcance`.
- A flag from condition 5 shows up in the code.

Then the Orchestrator stops the worker and tells Atena: `orq msg send --para atena --tipo pregunta "fix outgrew its lane: <the fact>"`. Atena confirms with the Owner in one line. The same slice continues on the **slice** lane from the Planner step: plan, Astra, `orq plan cargar`, `orq plan debatido`, team. The red test and the commits stay.

A slice already running stays on the slice lane until it closes.
