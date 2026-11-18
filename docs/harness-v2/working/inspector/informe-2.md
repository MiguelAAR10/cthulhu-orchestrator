# Informe 2 — con el plan y la investigación a la vista

Revisado: `docs/harness-v2/plan.md` y `docs/harness-v2/working/` del repo real (solo lectura), contra el código de la copia (`248c9c7`).
En esta ronda no ejecuté `orq`; los resultados de pruebas son los de la ronda 1. **[V]** = verificado (corrido en ronda 1 o leído). **[I]** = inferido.
Rutas de código relativas a `harness/internal/`.

**Aviso.** En la ronda 1 listé nombres y fechas de `~/.config/orq` y `~/.local/state/orq` para comprobar que no los había tocado. No leí contenidos. Ahí vi que existe un `owner.token` y lo mencioné en una recomendación; la retiro, no debí apoyarme en eso.

## Parte 1 — ¿Cambia mi perspectiva?

### Lo que reviso

1. **"El equipo no sabe que está roto": falso.** Juzgué el código contra el README. El plan ya dice que el README promete de más (actividad A4, pendiente), que el WIP no corrió tests, y clasifica los mismos 7 fallos que encontré. Mis hallazgos 1, 3, 4 y 8 son trabajo pendiente conocido, no defectos ocultos.
2. **"Cerrarlo como producto y extraer el núcleo": lo retiro.** La Dirección del Owner ya fija el orden correcto: B5 probado → prueba de recuperación → piloto → decidir con resultados. Eso es mejor que mi recomendación, porque decide con datos.
3. **Mi "alternativa más simple" (CI + rama protegida) ya está en el plan,** como línea base contra la que se mide el piloto (sección *Measure*). No es un reemplazo: es el control del experimento.
4. **"Invirtió al revés": injusto en parte.** Un worker por defecto (P3), Astra solo en trabajo grande (P4) y el rechazo a enjambres ya están decididos. Aún no están en el código.

### Lo que mantengo

- **El hueco de auto-aprobación** (`orq slice examen --aprobado "texto"`). B6 lo cierra para el worker, porque dentro de la caja no ve `.orq`. Pero ninguna actividad lo prueba, y sigue abierto para los roles "de confianza": el Orquestador puede escribir ese texto. Astra señala lo mismo para `--owner` (crítica ágil, hallazgo ALTO).
- **`cerrar` llama a `claude -p` sin opción de saltarlo** [V].
- **`ORQ_CONFIG` es archivo en `config/config.go:100` y carpeta en `instinto/instinto.go:43`** [V]. El plan (C1) lo define como archivo.
- **Lo bueno:** la caja, la ingesta por bundle, el contrato en git, el cierre reanudable.

### Lo que refuerzo

- **La prueba de salida de B5 es demasiado estrecha.** El plan exige `aislamiento.sh` en verde sin `--todo`. Hoy da **14 de 14** [V]. Con ese criterio B5 parecería casi terminado, y sin embargo el gate no ve los clones y la trampa del examen pasa. La forja de estado está cerrada; la frontera del worker, no.
- **El criterio de aceptación 2 retrocedió.** Se verificó en `b950633` sobre worktrees enlazados. Con clones, `e2e-v05.sh` pasó de 74/74 a 67/7 [V], y dos de los fallos son del gate.
- **La propia investigación del equipo apoya reducir** (ver Parte 2, punto 2).

### Dónde el plan y el código no coinciden

| Plan | Código |
|---|---|
| B5 incluye "gate: detección de worker, lectura del contrato" | `cli/cli.go:175` sigue con `IsLinkedWorktree`; la marca del clon no se lee [V] |
| B5 incluye `orq slice convertir` | Solo aparece en un mensaje de error (`slice/contrato.go:108`); el comando no existe [V] |
| "El worker corre en la caja; `.orq` ausente dentro" (B6) | Se lanza sin caja (`app/workers.go:163-170`) [V leído]. B6 no empezó |
| Aceptación 6: slices 1.0.0 siguen verificando | Rota; el checkpoint lo reconoce |
| Regla 4: test `go/ast` que falla si se descarta un error en caminos de aceptación | No existe ningún test con `go/ast` [V]. `memoria.Add` sin revisar en `cli/proyecto.go:519,574` |
| Regla 5: `go test -race` limpio en cada commit | El WIP no lo cumple; reconocido |
| A4: integración continua | No hay `.github/` [V] |
| "Fuera de alcance: adaptador de Orca" (sección de aceptación) | `terminal/orca/orca.go`, 349 líneas, hecho en C2. El plan se contradice a sí mismo |
| P3: un worker por defecto | La skill dice "en paralelo cuando estén listas" (`cli/orqskills/orq-orchestrator/SKILL.md:26`) |
| P4 y Astra: quitar el consenso obligatorio | `app/workers.go:68-74` exige `plan debatido` |
| Checkpoint: `aislamiento.sh` 13 pasan, 1 pendiente | Hoy 14 pasan [V]: B5 avanzó de verdad |

## Parte 2 — El bus de mensajes con orquestador general y memoria de colmena

### 1. ¿Qué cuello de botella quita, y existe?

**Existe, pero no es de transporte.** El Orquestador es el único salto de todo evento, y cada paso mecánico espera un turno suyo:

- Todo mensaje tiene un solo destinatario (`--para`), y se entrega **tecleándolo en su panel** (`cli/proyecto.go:205`). Si el terminal no responde, "puede haber llegado o no" (`:207`).
- Nada se dispara solo. Que un worker termine no lanza la verificación: el `cierre` es prosa que el modelo lee y luego decide teclear `juntar`, `plan hecha`, `verificar`, `msg send`, `cerrar`, `sesion cerrar`, `doctor` (pasos 7 a 10 de su skill).
- Astra lo contó: 9 acciones de CLI del Orquestador para un fix, 12 o más para una funcionalidad (crítica ágil, sección 1).

**Lo que nadie sabe es cuánto cuesta.** El plan dice que `orq` "no ha medido nada" y las métricas (C4) no están construidas. El freno es real en la estructura y desconocido en tamaño [I].

**Lo que un bus no quitaría:** la integración va en serie a propósito (candado `integracion`, `slice/cierre.go:119`). Hay un solo `main`.

### 2. ¿Cuánto es sobreingeniería para un Owner en una máquina?

**Casi todo.** Con hasta cuatro workers no hay volumen que justifique publicar y suscribir.

- **Coste en código:** ya existen un canal (`canal/canal.go`, 199 líneas, JSONL con candado y clave de duplicados) y un diario de eventos (`diario/diario.go`, con identificador de operación). Un bus con suscriptores que actúan añade un proceso residente o varias sesiones permanentes.
- **Coste en tokens:** cada copia de cada mensaje la lee un modelo. Con N agentes suscritos, el gasto por mensaje se multiplica por N [I].
- **Acciones duplicadas:** entrega "al menos una vez" más consumidores que son modelos, que no son idempotentes. Dos agentes pueden lanzar o juntar lo mismo. Hoy la clave de duplicados solo existe si el emisor la pone.
- **Carreras al integrar:** el candado protege git, no la decisión. Dos agentes decidiendo el orden de `juntar` se contradicen.
- **Deriva de la memoria compartida:** Astra ya lo marcó: "contexto que nunca caduca es una promesa falsa"; la evidencia de un hecho es texto libre (`memoria/memoria.go`).
- **Quién es dueño de integrar:** el plan nombra un único dueño. La colmena lo disuelve.

**Lo que dice la investigación del propio equipo** (`working/research/orquestacion.md`, `a2a.md`):

- Estudio de Google/MIT: en trabajo secuencial, toda variante multiagente rinde entre un 39 % y un 70 % menos. Solo gana en trabajo divisible.
- Amplificación de errores: 17,2 veces con agentes independientes, 4,4 con un orquestador que valida. Un bus con suscriptores autónomos va del segundo caso al primero.
- Anthropic: su sistema multiagente gasta unas 15 veces más tokens, y el propio texto dice que programar tiene pocas subtareas paralelizables.
- Cognition: cada acción lleva decisiones implícitas que chocan entre agentes en paralelo.
- Conclusión del scout: "no adoptar enjambres ni charla entre agentes"; "memoria persistente: no es un problema de entrega". El plan lista como **rechazado**: enjambres de agentes.
- Claude Code ya trae equipos de agentes con buzón por archivo y lista de tareas compartida (`a2a.md`, sección 2). Si se quisiera esa forma, existe; sus límites están documentados ahí.

### 3. ¿Está alineado con la Dirección y su orden?

**No.**

- La Dirección pone la prueba de recuperación "antes de cualquier trabajo más de memoria automática". La memoria de colmena es justo eso.
- "Cada rol extra nombra el problema que resuelve" y "mínimo de agentes, un worker real". El bus añade roles.
- Se solapa con dos actividades ya aprobadas: C6 (buzón que se lee, timbre de una línea, datos estructurados) y W1 (vigía).
- Contradice a W1 en lo esencial: el vigía "nunca acepta, integra ni lanza". En la propuesta, los agentes suscritos sí integran y lanzan.
- Va contra la meta del propio Owner: sin bucles, sin sobreingeniería.

### 4. La versión mínima que sí integraría

**Un reactor dentro de `orq`, no un bus. Los suscriptores son funciones, no agentes.**

- **Qué:** cuando llega el `cierre` de un worker con datos estructurados (el SHA del commit), `orq` ejecuta por sí mismo `Juntar` y `Verificar` y deja el recibo en el buzón del Orquestador. El modelo solo interviene si falla, se bloquea o `alinear` pide parar.
- **Dónde:** en la entrega de `msg send` (`cli/proyecto.go:195-208`; `app/mensajes.go` cuando exista C6). Llama a `slice.Verificar` (`slice/nucleo.go:302`), que ya toma el candado y vuelve a comprobar el estado.
- **Qué reemplaza:** los pasos 7 y 8 de la skill del Orquestador, y la entrega del `cierre` tecleada en el panel.
- **Qué necesita antes:** B6 y la identidad de C6. Sin ellas, un worker podría dispararlo a voluntad y gastar las dos reparaciones del intento.
- **Memoria:** nada nuevo. El archivo de decisiones por slice (P7) y el diario bastan hasta que pase la prueba de recuperación, que es la prueba real de memoria compartida.

**La prueba de que compensa:**

1. Unitaria: el mismo `cierre` entregado dos veces produce una sola verificación y un solo recibo. Dos `cierre` de slices distintos a la vez se verifican ambos y se integran en serie.
2. En el piloto: acciones del Orquestador por fix (hoy 9, según Astra) y tiempo entre el último commit del worker y el recibo, con y sin reactor, en tareas comparables. Compensa si baja a cuatro o menos sin recibos duplicados.

### 5. Quién tiene razón

**El Owner tiene razón en:**
- El Orquestador como único salto es un freno real.
- La meta: conservar intención y valor, dar rieles, humano que decide.
- La Dirección que escribió: medir, piloto, no prometer de más.

**El Owner se equivoca en:**
- La forma de la solución. Publicar-suscribir con copias a agentes y memoria de colmena es lo que su propia investigación rechaza.
- El ritmo de añadir. El 13 de noviembre aprobó P1 a P10 y el vigía, incluida una pantalla de configuración a pantalla completa (U1) y traducir el repo al inglés (L1), con B5 sin terminar y la cuota al 99 %. Esta propuesta sigue ese patrón.

**El equipo tiene razón en:**
- El diseño de aislamiento: clones, bundles, contrato en git, examen en la caja.
- Los recortes de Astra y el rechazo a enjambres y a A2A por red.
- Un checkpoint honesto, que dice lo que no se probó.

**El equipo se equivoca en:**
- **La prueba de salida de B5.** Se puede pasar con el gate ciego.
- **Dejar la integración continua para el final (A4).** Con ella al principio, la caída de 74 a 67 en el e2e se habría visto en el commit.
- **Mantener el README con cifras que saben falsas** hasta la última actividad.
- **El orden técnico** (Q1, U1, C6, W1, C4, C5, L1 tras B6) no respeta la Dirección, que dice que todo lo posterior a B5 espera al piloto.
- **Construir el adaptador de Orca** que su propia sección de aceptación deja fuera de alcance.

**Los dos se equivocan en:**
- **Nadie ha medido nada.** Se discute arquitectura sin una sola ejecución del piloto.
- **La autoridad del Owner es texto.** `--aprobado`, `--decision-owner`, `--acepto-runner` y `--owner` los puede escribir cualquier rol. Los "rieles" dependen de un canal del Owner que no existe en ningún diseño.
- **Tratar a Orquestador y Atena como de confianza** mientras tienen esas banderas en la mano.

## Lo que haría, en orden

1. Terminar B5 con una prueba de salida más amplia: `go test`, `e2e-v05.sh` y `aislamiento.sh` en verde, más una prueba nueva donde un worker intenta `orq slice examen --aprobado` y falla.
2. Integración continua mínima ya, no al final.
3. B6, la prueba de recuperación y el piloto, como dice la Dirección.
4. Medir en el piloto las acciones del Orquestador. Solo si el número duele, el reactor del punto 4.
5. Nada de bus, colmena, U1 ni L1 hasta tener esos resultados.

## Lo que no revisé

No leí completos `diseno-agil.md`, `plan-aislamiento.md`, `spike-resultados.md` ni `research/calidad.md`, `worktrees-skills.md` y `tema-terminal.md`: solo encabezados y pasajes. No ejecuté nada en esta ronda. Las cifras de los estudios son las que cita el scout del equipo, que marca varias como de segunda mano; no las comprobé en las fuentes.
