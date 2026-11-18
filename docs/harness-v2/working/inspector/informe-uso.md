# Uso real de orq: informe de scout (solo lectura)

Fecha de lectura: 2026-11-18. Rutas base: `V` = `/home/miguel/projects/ventia-work/VentIA-Latam`, `A` = `/home/miguel/projects/MiguelAR10-Builder/AI-Edge-Runners/products/airiflow`.
**[M]** = medido en archivos. **[I]** = inferido. Solo leí metadatos (fase, intento, veredicto, sellos de tiempo, motivos de una línea).

## 0. Qué hay en cada `.orq`
| Proyecto | Estado en disco |
|---|---|
| `V/ventia-metricas/.orq` | Completo: `slices.jsonl`, `recibos.jsonl`, `canal.jsonl` (958), `memoria.jsonl` (138). 6.868 archivos, en su mayoría evidencia de M24. |
| `A/odontoflow-backend/.orq` | Completo: 47 estados en `slices/`, 63 recibos, canal de 289 mensajes. |
| `A/odontoflow-frontend/.orq` | Completo: 34 estados, 48 recibos, canal de 484. |
| `A/airiflow-erp/.orq` | 1 estado, 9 recibos, canal de 1.229. Cierres en carpetas ad hoc (`g0`, `g1`, `sec1`, `dis1`). |
| `A/airiflow-planning/.orq` | Sin ledger ni recibos. 52 carpetas `sNN/` con planes y canal de 6.206 mensajes (el concentrador de todo AirIFlow). |
| `V/ventia-monorepo/.orq` | Solo `slice-51.lock`. Los worktrees `ventia-resumidor-v-*` guardan solo XML y logs de pruebas. |
| `projects/cthulhu-harness/.orq`, `odontoflow-voice`, `odontoflow-sim`, `airiflow-Brain` | Vacío o un `proyecto.json`. Cero slices. |

## 1. Slices y resultado
- **metricas [M]**: 6 en `slices.jsonl` (S-01..S-06), las 6 `aceptado`. Cada una con sha, abierto y cerrado.
- **odontoflow-backend [M]**: 49 slices (47 estados + 2 rechazadas en `slices.jsonl`).
  - 43 en fase `VERIFICADO`.
  - 4 siguen en `IMPLEMENTANDO` (S-67, S-70, S-71, S-73), abiertas el 2-nov. No hay cierre posterior.
  - 2 `rechazado` (S-18, S-56). Ambas se reabrieron sin trabajo de workers: S-18 por un examen mal calculado del Planificador, S-56 por un bundle protegido demasiado amplio.
- **odontoflow-frontend [M]**: 34 estados. 32 `VERIFICADO`, 2 `IMPLEMENTANDO` (S-58, S-59). No tiene `slices.jsonl`.
- **airiflow-erp [M]**: 5 slices con recibos (m0, g0, g1, dis1, sec1). Solo `m0` figura `aceptado` en el ledger. Las otras cuatro tienen recibo verde pero ninguna fila de cierre. Falta la ramita `slice/*` en git.
- **monorepo (ventia-resumidor) [I]**: el estado de orq no está en `.orq`. Solo por asuntos de commit con etiqueta `S-NN/aK` salen 106 slices distintas (S-1..S-118) entre 2026-10-30 y 11-11.
- **Abandonadas [M]**: no hay un estado «abandonado». Lo más parecido son las 2 rechazadas y las 6 slices abiertas sin cierre.
- **Aceptado de verdad frente a «verificado» [I]**: en odonto solo hay ledger de cierre para 2 filas (ambas rechazos). Las 75 slices en `VERIFICADO` no tienen fila `aceptado`. O se cerraron por otro camino, o `VERIFICADO` fue el estado final. Los archivos no permiten saberlo.

## 2. Intentos y reparación
- **backend [M]**: de 47 estados, 34 con `intento` 1, 12 con 2 y 1 con 4. Tasa de primer intento: **72 %**.
- **frontend [M]**: 31 de 34 con intento 1 (91 %), 2 con intento 2 y 1 con intento 3.
- **erp [M]**: g1 tiene recibos a1 pasa, pasa, **falla**, pasa. No subió de intento, pero el verde falló una vez tras pasar dos.
- **metricas [M]**: todas `intento` 1 (6/6). Pero S-01 tuvo 2 recibos rojos antes del verde y S-02 uno.
- **monorepo [I, solo git]**: 82 de 106 con a1 como máximo (77 %), 12 con a2, 1 con a3 y 1 con a4 (S-58). Los máximos de 3 y 4 son S-103 y S-58. 10 slices no llevan sufijo de intento.
- **Causa del intento > 1 [M]**: en backend, 12 de los 13 intentos 2 o más dicen `examen sumado o corregido` (`motivo` del estado). Lo pidió el Planificador o Astra y se registró con `orq slice examen --aprobado`. O sea, **la «reparación» es casi siempre corregir el examen, no corregir al worker**. Frontend sigue el mismo patrón (S-22, 26, 30, 31, 39, 54, 57 [M]).
- **Recibos [M]**: 13 de 129 son `fallo`. Backend 3 de 63 (S-08/a1 y S-64/a2 dos veces). Frontend 6 de 48 (S-11, 13, 22, 26, 30, 31, todos a1 y todos seguidos de verde). ERP 1 de 9 (exit 124, tiempo agotado). Metricas 3 de 9. El primer recibo de una slice con fallo suele ser el rojo del examen, así que no todos son «el worker dijo hecho y falló». Ver §6.
- **Commits por slice [M, ruidoso]**: mediana 6 en backend y 4,5 en frontend. Máximo 230 y 115, inflados porque la base de comparación se movió. No uso esos máximos.

## 3. Tiempos (apertura a cierre)
- **metricas [M]** (`slices.jsonl`): mediana **0,28 h** (17 min). Peor **6,5 h** (S-01). Valores: S-01 6,5 h, S-02 6,4 h, S-03 0,29 h, S-04 0,05 h, S-05 0,27 h, S-06 0,38 h.
  - S-01 y S-02 se cerraron con 2 min de diferencia (21:21 y 21:23). Es un cierre en lote tras 6 h abiertas.
  - S-03 y S-04 se cerraron con 3 min de diferencia entre ellas.
- **backend [I]**: no hay `cerrado`. Medido del `abierto` al primer recibo verde: mediana **0,7 h**, peor **29 h** (43 slices).
- **frontend [I]**: mediana **0,2 h**, peor **8,7 h** (32 slices).
- **erp [I]**: m0 0,84 h (de 00:04 a 00:54), g1 con recibos entre 11-03 06:33 y 11-04 01:46, casi 19 h.
- **monorepo [I]**: primer a último commit por slice, mediana 1,0 h, p90 7,7 h, máximo 159,7 h.
- Advertencia: el primer recibo verde no es el cierre. Es un piso del tiempo real.

## 4. Fallos y bloqueos más repetidos
Ranking con conteo [M] salvo donde dice [I].
1. **Examen sumado o corregido por Planificador/Astra**: 12 slices backend, 7 frontend, 1 erp (g1).
2. **Examen o `huella` mal fijado al abrir**: S-07 backend (la huella venía de una edición sin commit en el checkout principal), S-08 (Atena exige 3 desenlaces en la demo), S-18 (examen del Planificador mal calculado, se rechazó), S-56 (bundle protegido demasiado amplio). 4 casos.
3. **Fallos de suite heredados, no del worker**: aparecen en muchos reportes de worker («falla IGUAL con la base», 1 y 3 pruebas rojas preexistentes). En el canal de backend hay al menos 8 reportes así [I, por lectura de texto].
4. **Cierre de worktrees que se negó** [M]: `cuarentena.jsonl` de metricas, 13 líneas, todas «git worktree remove … contains modified or untracked files» (S-03 a S-06). Quedan 9 worktrees de metricas sin poda.
5. **Los workers no arrancaron o no vieron el repo** [M/I]: 4 workers Codex nunca corrieron (modelo pasado como un solo argumento, zsh) mientras Herdr decía `done` (`memoria.jsonl` metricas, 2-nov 21:40). Otros: orq `slice abrir` rechazó un rojo legítimo (S-02 backend, 28-oct) porque leía un `/tmp/*.xml` viejo.
- Qué bloqueó el trabajo en metricas [M] (`cierre-bloqueos.md`): 7 bloqueos (B1..B7). Casi todos dependen de personas fuera del harness: datos de Renzo, reglas de atribución, migraciones sin autorizar por el Owner, falta de mockups de Sebastián. No son fallos de worker.

## 5. Cuellos de botella (humano y orquestador)
- **Tiempo de respuesta a preguntas de workers [M, canal]**: mediana de 0,1 a 0,8 min en todos los proyectos. El orquestador responde en segundos. Hay colas largas:
  - metricas: 2 de 14 preguntas > 30 min, máximo 126 min.
  - frontend: 1 de 20 > 30 min, máximo 112 min.
  - planning: 12 de 184 > 30 min, máximo 2.041 min (34 h). De esas 184, 82 son al Atena, que es la capa humana.
- **El humano interviene en límites del sistema [M]** (`memoria.jsonl`): subir `state_max_age_hours` de 24 a 48 «para destrabar el gate» (28-oct), subir `max_workers` de 8 a 14 y de 6 a 8, autorizar Docker, migraciones o push a mano (varias veces), resetear la cuota de Codex. Son al menos 6 decisiones del Owner que tocan `limits.json` o la autorización de cada paso.
- **Cierre manual** [M]: «Cierra los 4 panes fallidos y relanza» (Atena al orquestador, 2-nov 21:37) o «publicar solo rama, pendiente literal aceptado» (erp). Es decir, el humano debe escribir una aceptación literal antes del push.
- **Orquestador como cuello [I]**: pide al worker que «corte y reporte en 25 min» (backend, S-62), lo que indica que la regresión de 25 archivos era lenta. Los tiempos de worker «terminado» a «verificado» no se pueden medir con precisión (ver §8).
- **Cierre en lote [M]**: en metricas, S-01 y S-02 cierran con 2 min de diferencia tras 6 h. En ERP, g0, sec1 y dis1 tienen recibo verde y siguen sin fila `aceptado`.

## 6. Falsos «hecho» que cazó el harness
- **Con prueba directa [M]: 0 casos limpios** en los que un worker declare verde y la verificación automática lo tumbe. No hay una marca del tipo «worker dijo hecho». Los workers suelen reportar fallos con honestidad: «28 passed / 4 failed», «NO probado», «exit 1 honesto».
- **Casos cercanos [M]**:
  - **Atena, no el harness**, rechazó el cierre en S-08 backend y S-11 frontend (`fallos`=1, `motivo`: «Aceptación Atena S08EVID», «Atena S11CHECK»).
  - **Recibos `fallo` seguidos de verde**: S-08/a1 backend, S-11/13/22/26/30/31 frontend, S-01 y S-02 metricas, g1 erp (exit 124). Son 10 slices. No sé qué afirmó el worker antes.
  - S-64/a2 falló dos veces (01:07, 01:18) antes de pasar como a4 (01:34): 27 min.
- **Falso «hecho» que cazó una persona, no el harness [M]**: 4 workers Codex mostrados como `done` en Herdr que nunca corrieron (metricas, 2-nov). Lo detectó Atena revisando sesiones. El orquestador lo reconoció («no fue mío»). El aprendizaje quedó escrito: «verificar modelo y primera lectura en 60 s antes de dar *lanzado*».
- **Conclusión [I]**: el harness cierra por recibo (comando, exit, hash del bundle), no por lo que el worker dice. Eso evita el falso verde. Pero el recibo no distingue «pasó porque probó» de «pasó porque no ejecutó nada»: varios recibos de metricas traen la nota `sin contadores: declara --reporte y --formato`.

## 7. Señales de que el harness se saltó o se rodeó
- **Trabajo grande fuera del ledger [M]**: en metricas, `slices.jsonl` termina en S-06 (2-nov 15:22), pero el canal sigue hasta el 4-nov 23:35 y hay 27 historias (`historias/`: OLA-1, M24-CIERRE, M24-FIDELIDAD, S-ULT, S-UX2…). M24 tiene 569 MB de evidencia y una rama de entrega, sin ninguna slice en el ledger. Corre por historia y plan, no por slice. [I] El harness fue la mensajería y la memoria, no el ciclo rojo→verde→cerrado.
- **Ledger incompleto en odonto/erp [M]**: 75 `VERIFICADO` sin cierre aceptado, 2 `rechazado`. En frontend, 19 `slice-NN <sufijo>.lock` con nombres con espacio («slice-34 doc», «slice-43 integrar-panel-mes») que no son slices del ledger: sub-slices informales.
- **Otras desviaciones [M]**:
  - Un lock vacío `slice-.lock` en backend.
  - Worktrees fuera de orq: `memoria` de metricas dice que «no son slices orq». Hay 95 carpetas en `/home/miguel/worktrees`, 9 de metricas.
  - Los gates se aflojaron a mano: `state_max_age_hours`, `max_workers`, `push_ramas` en `limits.json`.
  - La huella de S-07 backend se «rebasó» por decisión de Atena porque el examen se había editado sin commit.
  - El monorepo (106 slices según git) no deja ningún ledger en `.orq`.

## 8. Lo que no se puede medir con estos archivos
- Tiempo real entre «worker terminó» y «verificación» o «cierre». No hay sello de «worker dijo hecho» en el estado, solo en texto libre del canal.
- Cierre aceptado de odonto, erp y monorepo. No hay `cerrado` ni `aceptado` en el estado salvo los 9 casos de ledger (6 metricas, 2 rechazos odonto-back, 1 erp).
- Si un recibo verde probó algo: muchos recibos no traen contadores.
- Cuántos fallos son del worker y cuántos del examen. Solo tengo el `motivo` de una línea por slice.
- El monorepo: su estado de orq está en otro sitio (`~/.config/orq` u otro). No lo leí.
- Costo y tokens por slice. No hay registro de costo en estas carpetas.
- Si el estado `VERIFICADO` llega a un merge. Solo 1 rama por repo figura fusionada en su base.
- Tiempo del humano frente al de máquina. El canal tiene sellos pero no marca presencia.

*Archivos fuente principales:* `V/ventia-metricas/.orq/{slices,recibos,canal,memoria,cuarentena}.jsonl`, `A/odontoflow-backend/.orq/{slices/*.json,slices.jsonl,recibos.jsonl,canal.jsonl,memoria.jsonl}`, igual en `odontoflow-frontend` y `airiflow-erp`, y `git log` de `ventia-monorepo`, `odontoflow-*` y `airiflow-erp`.
