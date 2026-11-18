# Informe 4 — la propuesta para aprobar

Fuentes: mis informes 1 a 3, el código, el plan y el informe de uso (`informe-uso.md`, lo encontré una carpeta más arriba de donde decía el encargo; "uso §N" lo cita). No ejecuté nada. **[J]** = juicio mío.

**Dos cosas que los datos de uso me corrigen:**
- Dije que el valor central era el verificador contra trampas. En unas 200 slices cazó **cero** falsos "hecho" (uso §6). Los workers informan con honestidad.
- Llamé "hueco" a `orq slice examen --aprobado`. Es la vía de reparación más usada: 12 de 13 segundos intentos en backend (uso §2). El problema real es que el examen nace mal, no que alguien haga trampa.

## 1. El producto, en un párrafo

Cthulhu es el arnés interno de AI Edge Runners para llevar historias de usuario con agentes de varios proveedores en los proyectos del equipo. Ya funciona: unas 200 slices, entre el 72 % y el 91 % resueltas al primer intento, y una mediana de 15 minutos a 1 hora hasta el primer verde (uso §2, §3). Lo que no hace todavía es lo que más te cuesta: **que veas cómo se entendió la historia antes de que haya código, y que aceptar el resultado te lleve minutos**. Hoy hay 75 slices verificadas sin cierre, el trabajo grande se sale del ciclo y el examen se corrige en una de cada cuatro slices (uso §1, §7, §2). El producto es eso: un desarrollador supervisa varias corrientes de trabajo, dedica su criterio a los detalles, y la corrección de base (lint, tipos, tests) la impone un programa.

## 2. Qué significa "excelente" aquí

Todo lo mide el propio Owner con `orq metricas`, una vez instrumentado (paso 2). Umbrales [J].

| Criterio | Cómo se mide | Hoy | Umbral |
|---|---|---|---|
| **Carga de revisión** | Minutos tuyos entre "verificado" y aceptar o rechazar | No medible (uso §8) | Mediana ≤ 10 min |
| **Nada se queda sin cerrar** | Slices verificadas sin decisión tras 24 h | 75 (uso §1) | 0 |
| **Entendimiento antes del código** | Historias con tarjeta aprobada antes del primer commit del worker | 0 % | 100 % en el carril completo |
| **Examen bien hecho** | Slices cuyo examen se corrige tras abrir | 26 % backend, 21 % frontend | ≤ 5 % |
| **Primer intento** | Slices cerradas en el intento 1 | 72–91 % | ≥ 85 % en todo proyecto |
| **Corrección por stack** | Recibos con tests contados, lint y tipos | Varios sin contadores (uso §6) | 100 %; "omitido" nunca cuenta como pasa |
| **Varios modelos** | Workers "lanzados" que nunca arrancaron | 4 de Codex (uso §4) | 0; las mismas 5 historias pasan con dos proveedores |
| **Conocimiento del proyecto** | Tiempo desde `orq init` hasta la primera historia aceptada en un repo nuevo | Sin dato | ≤ 1 día |
| **Sin código de más** | Archivos tocados fuera de lo declarado al aceptar; supresiones nuevas | Sin dato | 0 y 0 |
| **El trabajo grande sigue dentro** | Integraciones a la rama base sin fila en el registro | M24 y el monorepo, fuera (uso §7) | 0 |
| **Los límites no se aflojan a mano** | Ediciones tuyas de `limits.json` al mes | ≥ 6 (uso §5) | ≤ 1 |
| **Coste** | Registro por slice | No existe | Existe en el 100 % |

## 3. El valor del orquestador

**Lo que solo él puede hacer** (no la integración continua, ni un hook, ni un agente solo):

1. **Mantener un solo registro** de historia → examen → recibo → aceptación entre agentes y proveedores. La integración continua no ve nada antes del PR, y los hooks son por sesión y solo de Claude.
2. **Confirmar que cada worker arrancó de verdad,** sea del proveedor que sea. Es el único falso "hecho" real que hubo.
3. **Partir el trabajo grande en slices hijas** para que no se salga del ciclo, como pasó con M24.
4. **Dar el estado como datos** al filtro, sin que nadie lea el canal.
5. **Integrar en serie.** Hay una sola rama base.

**Lo que debe dejar de hacer:**

- **Ser el paso obligado de lo mecánico.** Al llegar el cierre de un worker, `orq` junta y verifica solo.
- **Contar la corrección del examen como un intento del worker.** Es un defecto del plan y se registra como tal.
- **Dejar slices en "verificado".** O se cierran o le aparecen al filtro.
- **Crecer como guardia contra trampas.** Se mantiene lo barato (examen en la caja, diff del examen al ingerir) y se congela el analizador de shell del gate.
- **Depender de que tú edites límites.** La capacidad ya sale de la máquina (C3).
- **El consenso sin tope entre Planificador y Astra.**

**El filtro (Atena) y su relación con él:**

- **Poderes:** aprobar la tarjeta de entendimiento; pausar una slice o al orquestador; pedir trabajo en paralelo como historia nueva; aceptar o rechazar; autorizar un cambio de examen después de aprobado.
- **Límites:** no escribe código, no integra, no abre slices y no edita el examen.
- **Dos niveles de autoridad:**
  - **Atena como modelo:** informa, pregunta, recomienda y puede pausar. Pausar no destruye nada, así que no necesita prueba.
  - **Solo la persona:** aprobar, aceptar, autorizar cambio de examen y aflojar límites.
- **Cómo se hace infalsificable [J]:** esas cuatro acciones piden una frase de paso tecleada en tu terminal, fuera de cualquier panel de agente. `orq` solo guarda su huella y anota el evento firmado en el diario. Límite honesto: protege mientras ningún agente pueda leer ese terminal. En repos publicados, la aprobación de un PR con tu cuenta puede hacer de aceptación.
- **El cambio de examen deja de ser un texto libre:** queda registrado y se te muestra resaltado antes de cerrar.

## 4. Puntos de control contra tu dolor principal

| Cuándo | Qué ves | Tiempo objetivo [J] |
|---|---|---|
| **Antes del código** | **Tarjeta de entendimiento:** la historia en tres líneas, lo que **no** se va a hacer, el examen en lenguaje llano con su archivo, las zonas que se tocarán, la estimación, los riesgos y las preguntas abiertas | ≤ 3 min |
| **Durante** | **Una línea por slice:** fase, tareas hechas, último recibo. Y avisos solo si hay desvío: se salió del alcance, piden corregir el examen, un worker no arrancó, no hay progreso. En trabajo grande, una demo por slice hija | ≤ 1 min por vistazo |
| **Antes de cerrar** | **Paquete de evidencia:** recibo con tests, lint y tipos; lo planeado frente a lo tocado; cualquier cambio de examen resaltado; la demo en tres pasos; el diff solo de las zonas críticas | ≤ 10 min |

La tarjeta ataca dos cosas a la vez: ves cómo se entendió, y se detecta el examen mal hecho antes de abrir, que es la causa número uno de retrabajo.

## 5. Hoja de ruta

Respeta la Dirección (B5 probado → recuperación → piloto → decidir), adaptada a los datos. "Visión N" son tus cinco puntos.

| # | Paso | Prueba de salida | Qué desbloquea | Visión |
|---|---|---|---|---|
| 1 | **Terminar B5 e integración continua** | `go test`, `e2e-v05.sh` y `aislamiento.sh` en verde en cada commit; el gate reconoce los clones | Un recibo en el que se puede creer | 5 |
| 2 | **Instrumentar y quitar el paso mecánico:** sellos de "worker terminó", "verificado" y "aceptado"; causa de cada intento; coste; verificación automática al llegar el cierre; confirmación de arranque en 60 s; aviso de verificadas sin cerrar | `orq metricas` reproduce las cifras del informe de uso sin leer el canal; un cierre repetido da un solo recibo; un worker que no arranca se marca en 60 s | Poder medir el dolor | 2, 3 |
| 3 | **Tarjeta de entendimiento y autoridad de la persona;** examen revisado antes de abrir; su corrección registrada aparte | En 10 historias reales: corrección de examen ≤ 10 %, aprobación ≤ 3 min de mediana; un rol sin la frase de paso no puede aceptar | Ves el entendimiento antes del código | 2, 5 |
| 4 | **Prueba de recuperación** (criterio 7 del plan) | La que define el plan | Trabajo de varios días | 5 |
| 5 | **Corrección por stack:** `orq init` deja en el contrato el comando de lint, tipos y tests de cada proyecto; el recibo lleva ese bloque; sin supresiones nuevas | En odontoflow backend y frontend, 100 % de recibos completos; una supresión plantada falla | "El código es correcto" | 1, 4 |
| 6 | **Piloto medido,** con una historia grande partida en slices hijas | Los criterios de la sección 2, medidos en 10 tareas comparables frente a "un agente + CI". **Aquí se decide** | Seguir, recortar o parar | 2 |
| 7 | **Varios modelos de verdad:** worker en la caja (B6) y un segundo proveedor | Las mismas 5 historias pasan con dos proveedores; 0 "hecho" sin haber corrido | Elegir modelo por tarea | 3 |
| 8 | **Conocimiento portable:** la skill de bases (evidencia → criterios → reglas con línea base) y catálogo abierto a fuentes externas, con versión fijada y verificada | Un repo nuevo llega a su primera historia aceptada en ≤ 1 día; las bases de un proyecto se aplican a otro del mismo stack | Entrar a un proyecto y llevarse su lógica | 4 |

**Dos ajustes al plan del equipo [J]:**
- **B6 baja al paso 7.** El plan lo quiere antes del piloto. Con 200 slices sin una trampa, la caja del worker es menos urgente que ver y cerrar. El piloto puede correr como hasta ahora, diciéndolo.
- **`perfil` hoy detecta por extensiones y recomienda de un catálogo fijo de 11 entradas.** En el paso 5 basta con que lea la configuración de lint, tipos y tests que ya exista. Lo demás espera al paso 8.

**Cortado o congelado:**
- Bus publicar-suscribir, memoria de colmena y segundo orquestador.
- Pantalla de configuración (U1) y traducción del repo (L1).
- Adaptador de Orca.
- Canal completo (C6) y vigía como proceso aparte (W1): lo útil de ambos entra en el paso 2.
- Perfil de calidad completo (Q1), mutación y tests ocultos, hasta que el piloto muestre aceptaciones equivocadas.
- Memoria compactada por modelo e "instintos" como texto inyectado.

## 6. Lo que no haría

- **Añadir agentes antes de medir.** El orquestador ya responde en segundos (uso §5); el cuello está en ver y en cerrar.
- **Vender el arnés como "caza tramposos".** Los datos no lo sostienen.
- **Exigir tu firma en cada corrección de examen.** Es la reparación más frecuente; se muestra al cerrar.
- **Seguir ampliando el gate** para cubrir más formas de shell.
- **Aprobar propuestas nuevas con un paso sin terminar.** Una a la vez, cada una con su prueba de salida.
- **Dejar que un modelo acepte por ti,** ni aflojar un control porque molesta sin registrar por qué.
- **Reescribir lo que ya existe:** escáneres de secretos, mutación, buzones de agentes.
- **Tratar lo que escribe un agente como verdad permanente.** El contexto se rehace por historia.
- **Publicarlo como producto para terceros** antes de que el piloto dé números.

## Lo que queda sin saber

Las cifras de uso salen de metadatos: no hay tiempo humano, coste ni cierre real en la mayoría de proyectos (uso §8). Los umbrales de la sección 2 son puntos de partida y se ajustan con la primera medición del paso 2.
