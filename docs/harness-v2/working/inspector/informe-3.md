# Informe 3 — consejo para desarrollar esto en corto

Fuentes: `informe.md` (I1), `informe-2.md` (I2), el código de la copia y `docs/harness-v2/` del repo real. No ejecuté nada en esta ronda.
**[J]** = juicio mío. Sin marca = sale del código, de mis informes o de la investigación del equipo, que cito.
Rutas de código relativas a `harness/internal/`; `research/` es `docs/harness-v2/working/research/`.

## 1. Límites duros

**Lo que ningún arnés así puede garantizar:**

- **Que el modelo entienda la intención.** Los rieles limitan dónde actúa, no qué interpreta. Lo "no pedido" solo se detecta si alguien lo declaró antes.
- **Que un verde signifique que la historia se cumple.** Un test prueba que ese chequeo pasó. La Dirección ya lo dice así.
- **Que el código bajo prueba no engañe a su propio runner.** Corre en el mismo proceso. Astra lo deja como límite honesto (`astra/critica.md`).
- **Que un test visible no se sobreajuste.** METR vio modelos modificando tests y puntuación (`research/orquestacion.md`, c).
- **Aislamiento total en una máquina.** La caja es el mismo usuario y el mismo kernel. No cubre agotamiento de RAM o disco, ni código ya aceptado que se ejecuta después.
- **Más rendimiento que la atención de un Owner.** Tú apruebas la historia y aceptas la demo: ese es el tope real [J].
- **Roles de control inmunes.** Orquestador y Atena leen código y documentos que pueden traer instrucciones (`astra/critica-agil.md`, sección 4).
- **Presupuesto duro de tokens.** La pausa del 13-nov fue con la cuota de 7 días al 99 %. Solo hay estimaciones.
- **Estabilidad de la plataforma.** El arnés depende de nombres de herramientas, formato de hooks, banderas de `claude` y `codex`, y del modo de autenticación. Ya ocurrió un renombre (`gate/gate.go`: "Task es el nombre anterior de Agent").

**Lo que es propio de este diseño y se puede levantar:**

- El worker corre sin caja (B6).
- El gate no ve los clones (B5; I1, problema 1).
- La autoridad del Owner es texto que cualquier rol escribe (I2, parte 2, punto 5).
- Los workers de Codex no pasan por ningún hook; solo la caja los contiene (I1, tabla).
- El examen no tiene red: un test que necesite base de datos no corre (I1, problema 10).
- Todo paso mecánico espera un turno del Orquestador (I2, parte 2, punto 1).
- No se mide nada (plan: "Spend: not measured").

## 2. Lo que puede ser, en tres niveles

### (a) En 2 semanas: un verificador en el que se puede creer, con un worker

- **Necesita:** B5 terminado con prueba de salida amplia (`go test`, `e2e-v05.sh`, `aislamiento.sh`, y una prueba nueva donde el worker intenta `orq slice examen --aprobado` y falla). Integración continua mínima. Una historia real de principio a fin.
- **Entrega, con honestidad:** el test no se tocó, corrió sobre una copia limpia del commit, y el cierre se puede reanudar. El worker todavía no está encerrado; eso se dice.
- **Lo que hoy no puedes hacer:** fiarte del recibo. Hoy la trampa pasa (I1, problema 2).
- **Señal:** las tres suites en verde en cada commit; la prueba de la trampa falla como debe; una historia aceptada y una rechazada con motivo preciso (criterio 8 del plan).

### (b) En 1–2 meses: dejar a uno o dos agentes trabajando sin mirar

- **Necesita:** worker en la caja (B6); prueba de recuperación (criterio 7); métricas mínimas (C4); un worker por defecto en el código (P3, P4); las cuatro barreras baratas de calidad (sección 3); bases por proyecto con reglas nacidas de tus correcciones; el reactor de I2 solo si la medición lo pide.
- **Entrega:** historias pequeñas de uno o dos proyectos que avanzan sin ti y se retoman tras una interrupción. Revisas recibo y evidencia; el diff completo solo en zonas críticas.
- **Señal:** en una muestra de unas diez tareas comparables, frente a "un agente + instrucciones del repo + CI" (la línea base que fija el plan): minutos tuyos por historia aceptada, aceptaciones que luego hubo que revertir, y correcciones por historia a la baja.

### (c) El techo al que vale la pena aspirar

- **Qué es [J]:** un arnés personal de entrega. Para historias pequeñas y bien definidas, en un repo con bases maduras, solo tocas dos puntos: aprobar la historia con su test, y aceptar la demo. Paralelo de dos a cuatro workers únicamente en zonas disjuntas. Tests ocultos y mutación solo donde hay riesgo.
- **Qué no es:** una fábrica sin humanos, ni un producto para terceros. `orq verify` como producto aparte es una hipótesis, como dice la Dirección.
- **Señal:** sostenido un mes: porcentaje de historias aceptadas a la primera demo, tiempo tuyo por historia, tasa de reversión y coste por historia aceptada, todos mejores que la línea base.

## 3. Rieles contra lo no pedido, el código de más y los bucles

**Ya existe en el código:**

- **Test de aceptación declarado antes, protegido y con rojo real** (`slice/nucleo.go:125-235`).
- **Alcance por rutas comprobado sobre el diff ingerido,** no solo en el hook (`slice/clones.go:225-252`); zonas por worker en `juntar` (`slice/equipo.go:168-233`).
- **`orq alinear`:** avisa si lo cambiado sale de lo que la historia justifica. Lo vi funcionar en I1.
- **Corte de bucles:** tres verificaciones fallidas, detección de "sin progreso" por firma del fallo, tres intentos (`slice/nucleo.go:402-414`, `slice/slice.go:53,56`).
- **Vigilancia de archivos del runner** (`slice/verif.go`).

**Límite de todo eso:** una ruta permitida no es una funcionalidad pedida. Código de más dentro del alcance pasa.

**Falta, ordenado por valor sobre esfuerzo [J]:**

| # | Mecanismo | Fuente | Esfuerzo |
|---|---|---|---|
| 1 | Sin supresiones nuevas ni configuración de rigor debilitada | `research/calidad.md`, n.º 1 | Bajo |
| 2 | Todo test nuevo debe fallar antes del cambio, no solo el de aceptación | `research/practicantes.md`, n.º 1 | Bajo |
| 3 | Un worker por defecto; Astra y scouts solo en trabajo grande | P3, P4 | Bajo |
| 4 | Presupuesto de tamaño del diff frente a lo estimado en la historia | `diseno-agil.md` lo menciona | Bajo |
| 5 | Secretos en el diff | `research/calidad.md`, n.º 2 | Bajo |
| 6 | Cada corrección repetida se vuelve regla (lint, grafo de imports) | `research/practicantes.md`, n.º 3 | Bajo-medio |
| 7 | Medir tiempo, intentos y coste por carril | P2 | Bajo-medio |
| 8 | Dependencia nueva solo si está en una lista permitida | `research/calidad.md`, n.º 5 | Medio |
| 9 | Tests ocultos y mutación sobre líneas cambiadas, primero como informe | P8, `research/calidad.md`, n.º 7 y 9 | Medio-alto |

**Notas:**

- **El n.º 6 es el que más ataca la basura a largo plazo.** `orq` ya lo hace consigo mismo (`arch_test.go`), pero lo que aprende de los proyectos lo inyecta como texto al arrancar la sesión, no como chequeo.
- **El n.º 8 tiene evidencia:** entre un 5,2 % y un 21,7 % de paquetes inventados según el modelo (`research/calidad.md`, c).
- **El n.º 9 no tiene evidencia** de que un segundo modelo escribiendo tests funcione; el propio scout lo marca como no verificado.
- **"Código innecesario" no lo decide ningún programa.** Solo hay aproximaciones: tamaño del diff, complejidad, mutantes que sobreviven [J].
- **Un bucle sin tope queda en el diseño:** Planificador y Astra "hasta el consenso" (`cli/orqskills/orq-orchestrator/SKILL.md:24`). Se corta con P4.
- **No haría ahora:** la pantalla de configuración (P10) ni la traducción del repo (L1).

## 4. Lo que ya existe y no hay que reconstruir

| Pieza | Qué cubre | Qué haría |
|---|---|---|
| GitHub: rama protegida, chequeos obligatorios, CODEOWNERS | Que nada llegue a `main` sin chequeo ni cambie el test sin un humano, fuera del alcance del agente | Usarlo en todo repo que se publique. Es además la línea base del piloto |
| Hooks de Claude Code | `PreToolUse` ya se usa. `TaskCompleted` y `Stop` pueden vetar con salida 2 (`research/a2a.md`, sección 2) | "No digas terminado sin los tres verdes" sin escribir código en `orq` |
| Equipos de agentes de Claude Code | Buzón por archivo, lista de tareas compartida con candados (`research/a2a.md`, sección 2). Experimental | Probarlo en el piloto antes de construir C6. Podría sustituir `canal` y parte de `plan` [J] |
| Subagentes de Claude Code | Scouts y revisores de solo lectura | Ya se usan para scouts; extenderlo a revisores |
| Caja de Codex (`-s workspace-write`) | Límite de escritura del worker Codex (`agente/agente.go:33`) | Mantener, dentro de la caja propia |
| Skills de Pocock | Entrevista, partir en tickets, revisión en dos pasadas (`research/practicantes.md`, sección 1) | La entrevista ya está en el catálogo. La revisión en dos pasadas en lugar de Astra en trabajo mediano |
| pstack | Skill de verificación por app con mapa de funcionalidades; "atacar la premisa" | Tomar esas dos. El resto son principios en prosa |
| SwarmForge | Roles, worktrees, tmux, demonio de relevos | Nada. Es la misma forma de orquestación, sin veredicto |
| gitleaks, Stryker, mutmut, cargo-mutants | Secretos y mutación | Llamarlos; nunca reescribirlos en Go |

**Lo que solo tiene `orq` y sí hay que construir:** el examen tomado del commit rojo, corrido en la caja sobre una copia limpia, con recibo, contrato en git, ingesta por bundle y cierre reanudable. Todo lo demás es prestable [J].

No comprobé si Claude Code trae hoy una caja propia que sustituya el envoltorio de B6; la investigación del equipo tampoco lo cubre. Conviene mirarlo antes de construir B6.

## 5. Las tres preguntas que te haría

1. **¿Esto es una herramienta para ti o un producto para otros?**
   Es la que más cambia lo que se construye. Si es para ti, sobran la traducción al inglés, la pantalla de configuración, el adaptador de Orca, el segundo proveedor y buena parte de la documentación.

2. **¿Cuál es el proyecto del piloto, y qué necesitan sus tests de aceptación: base de datos, red, navegador?**
   Si los necesitan, el examen sin red no sirve tal como está y hay que resolver servicios dentro de la caja antes que cualquier otra cosa. Si no, el nivel (a) está a dos semanas.

3. **¿Qué vas a hacer siempre tú, y cómo se demuestra que fuiste tú?**
   Leer el test antes de aprobarlo, aceptar la demo, autorizar un cambio de examen, leer el diff en qué zonas. De esto depende el canal del Owner, que hoy no existe en ningún diseño, y cuánta revisión automática hace falta.

Una cuarta, barata: **¿cronometrarías cinco historias esta semana con el flujo actual?** Sin ese número, todo lo demás, incluido el reactor, se decide a ciegas.
