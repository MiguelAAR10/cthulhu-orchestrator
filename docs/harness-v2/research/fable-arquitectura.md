# orq2 como sistema confiable: arquitectura objetivo · Fable · 20 nov 2026

**Marcas.** [V] = lo leí yo en el código o en los archivos del piloto. [U] = no lo verifiqué yo: es lectura de un scout (Sonnet, solo lectura), de otro informe, o una recomendación mía.
**Rutas.** `F/` = `~/worktrees/cthulhu-fix/harness/internal/` (rama `v2/review-fixes`; `canal`, `diario`, `state` y `orqdir` son idénticos en las tres ramas [U]). `RS/` = `.orq/` del piloto Real State. No ejecuté pruebas ni toqué estado.

**La idea en una frase.** orq2 está bien hecho para que *no pase nada malo* y no tiene nada que haga que *pase algo bueno*. En ciencias de la computación eso se llama tener **seguridad (safety)** sin **vivacidad (liveness)**. Las 17 h 46 min de S-01 no son un descuido: son el resultado exacto de ese diseño. El bus que propusiste apuntaba al problema correcto; la forma correcta es más pequeña.

---

## 1. Mapa de hoy

```
 Owner (terminal propia)
   │ escribe la historia a Atena · corre `orq2 slice aprobar`
   ▼
 Sesiones de modelo en paneles de herdr:  Atena ─ Orquestador ─ Planificador ─ Astra ─ worker-*
   │ cada una ejecuta comandos `orq2 …` (proceso corto: nace, lee, escribe, muere)
   │ los mensajes entre ellas se TECLEAN en el panel del destinatario (herdr agent prompt)
   ▼
 <repo>/.orq/                                   quién escribe · cómo
   diario.jsonl     bitácora de lo que orq hizo  cualquier comando · agrega + candado + fsync
   canal.jsonl      mensajes + «entregado»       `msg send` · agrega + candado, SIN fsync
   slices/NN.json   estado vivo de la historia   comandos `slice` · candado slice-NN + rename
   planes/NN.json   tareas y dependencias        `plan` · rename
   sesiones.json    rol → panel de herdr         lanzar/relevo · rename, SIN candado
   memoria.jsonl    hechos                       `mem add` · candado + reescribe todo + rename
   *.lock           abrir · slice-NN · integracion · memoria
 ~/.local/state/orq2/  state.json, HALT (disco)  global de la máquina, no del proyecto
 ~/.config/orq2/limits.json                      global de la máquina
 Clones de worker en ~/worktrees/ · examen dentro de la caja bwrap, sin red
 Hooks de Claude Code: PreToolUse→gate · SessionStart→contexto · PreCompact→relevo
```

Evidencia:
- **No hay ningún proceso residente.** Cada `orq2` es una invocación corta; en el código que no es de prueba no hay un solo bucle de espera, temporizador ni vigilancia de archivos. [V] `grep` de `for {`, `time.Sleep`, tickers sobre `F/`: solo aparecen lectores y el proxy de la caja.
- **`watch` no vigila agentes.** Mide disco una vez y escribe `HALT`. [V] `F/watch/watch.go:20-54`.
- **El mensaje se guarda, se teclea en el panel y luego se marca entregado.** [V] `F/cli/proyecto.go:195`, `:215`; `F/canal/canal.go:79-120`.
- **Aprobar la tarjeta escribe el estado y una línea en el diario, y no avisa a nadie.** [V] `F/slice/tarjeta.go:157-169`. El comando solo imprime «Ya se puede lanzar el worker». [V] `F/cli/tarjeta.go:111`.
- **`.orq` se ubica por Git** (carpeta común del repo), así que todos los clones comparten una. [V] `F/orqdir/orqdir.go:14-23`.
- **Orden de candados fijo** abrir → slice-NN → integracion; violarlo se rechaza. [U] `F/slice/nucleo.go:51-83`.

---

## 2. Principios: qué optimizamos hoy y qué falta

| Principio (y qué significa aquí) | Hoy | Hueco |
|---|---|---|
| **Seguridad vs vivacidad** — «nada malo ocurre» frente a «lo debido termina ocurriendo» | Seguridad fuerte: lo desconocido deniega. [V] `F/limits/limits.go:2`; un arranque dudoso no se repite. [V] `F/app/sesiones.go:63-66` | Vivacidad nula: nadie reintenta, nadie despierta a nadie. Es el hueco n.º 1 |
| **Consistencia vs disponibilidad** — ante la duda, ¿paro o sigo? | Paro siempre. Correcto para una sola máquina y un solo `main` | Parar sin que nadie reanude equivale a no entregar |
| **Escritor único** — una sola mano cambia cada cosa | Un escritor por slice (`Escritor`) y candado `integracion` para `main`. [U] `F/slice/slice.go:78`, `F/slice/cierre.go:119` | `sesiones.json` se lee, se cambia y se guarda sin candado: dos comandos a la vez pierden un cambio. [V] `F/canal/canal.go:174-189` |
| **Bitácora que solo crece / event sourcing** — el estado se puede reconstruir de los eventos | El diario solo agrega y se escribe *antes* que el estado. [V] `F/diario/diario.go:51-83`; [U] `F/slice/nucleo.go:25-49` | El diario no es la fuente de verdad: el estado vive aparte en `slices/NN.json`. Los eventos no llevan número de orden, ni proyecto, ni causa. [V] `F/diario/diario.go:22-36` |
| **Idempotencia** — repetir un comando no repite su efecto | Lanzar y entregar contrato consultan el diario antes de actuar. [V] `F/app/sesiones.go:50-66`, `:139-155`. El cierre se reanuda. [U] `F/slice/cierre.go` | La clave anti-duplicado de mensajes solo existe si el emisor la pone a mano. [V] `F/canal/canal.go:96-103`. El `op` del diario une pasos pero nadie lo usa para evitar duplicados. [V] `F/diario/diario.go:42-48` |
| **Entrega «al menos una vez»** — puede llegar dos veces, nunca cero | Mensajes: sí, con reconciliación manual (`msg reconciliar`). [U] | El receptor es un modelo, que no es idempotente. No hay acuse de *lectura*, solo de tecleo |
| **Consistencia ante cortes** — un apagón no deja archivos a medias | Diario: guarda línea rota y hace fsync. [V] `F/diario/diario.go:73-82` | Canal: sin fsync y sin guarda; una línea cortada se pega a la siguiente y se pierden las dos; el id sale de contar líneas, así que puede repetirse. [V] `F/canal/canal.go:104-106`, `:153-160`. `WriteAtomic` renombra sin fsync. [V] `F/orqdir/orqdir.go:57-70`. `HALT` se escribe sin rename. [V] `F/state/state.go:67-73`. En WSL2, con una máquina que se apaga, esto importa |
| **Contrapresión y límites** — no aceptar más de lo que cabe | Capacidad = mínimo de tope, RAM y CPU; con carga alta no entra nadie. [V] `F/recursos/recursos.go:34-57` | Al llegar al límite se **rechaza**, no se encola. [U] `B:app/workers.go:129-131`. Sin alguien que reintente, rechazar es perder el trabajo |
| **Plazos y reintentos** | Todo llamado externo tiene plazo: herdr 15 s/120 s, git 60 s, examen 600 s. [V] grep de constantes | Cero reintentos automáticos y candados sin plazo (`flock` bloqueante). [V] `F/orqdir/orqdir.go:73-83` |
| **Supervisión** — quién reinicia a quién | Nadie. Las alertas `no_arranco` y `sin_progreso` se calculan solo cuando alguien corre `status`, y solo para workers **ya lanzados**. [V] `F/app/alertas.go:108-131`, `:136-157` | «Aprobada y sin worker» no produce alerta. herdr solo responde vivo/ausente/desconocido; «trabajando» y «esperando» son lo mismo. [V] `F/terminal/terminal.go:47-53` |
| **Observabilidad** | Diario con milisegundos; métricas derivadas | No registra minutos del Owner, costo ni quién debía actuar. [U] `F/cli/metricas.go:69` |
| **Reversibilidad** | Abrir se deshace solo si falla; rechazar conserva la rama; el trabajo no integrado va a cuarentena. [U] | Una aceptación publicada en `main` no tiene vuelta atrás dentro de orq2. Con piloto automático esto pasa a ser crítico |
| **Capas** | cli → app → dominio → adaptadores, con prueba que lo exige. [U] `F/arch_test.go:14-50` | Bien. El despachador nuevo cabe en `app` sin romper la regla |

---

## 3. Eventos y bus

**¿orq2 es dirigido por eventos hoy?** No. Es **registrado por eventos**: los escribe y nadie está suscrito. Todo es «tirar al invocar»: cada comando relee los archivos enteros y decide. Un evento que nadie relee no produce nada. [V] La aprobación de S-01 es la última línea del diario (`RS/diario.jsonl:15`, 04:49:47Z) y después no hay ninguna.

**Lo que tu bus arreglaría, y es real:**
1. *Trabajo atascado.* Un evento («aprobada») debe provocar una acción («lanzar T1») sin esperar el turno de un modelo.
2. *Despacho.* Alguien reparte tareas listas según capacidad y destraba.
3. *Enrutado entre proyectos.* Cada mensaje sabe de qué proyecto es.

**Lo que cuesta el bus completo (publicar y suscribir, con agentes suscritos y memoria de colmena):**
- Cada copia de cada mensaje la lee un modelo: el gasto se multiplica por el número de suscriptores. [U] `informe-2.md:66`
- «Al menos una vez» con consumidores que son modelos produce acciones dobles: dos agentes lanzan o integran lo mismo. [U] `informe-2.md:67-68`
- La investigación citada por el equipo dice que agentes independientes amplifican errores 17,2 veces, y 4,4 con un coordinador que valida. [U] `orquestacion.md:28` (cifras de segunda mano según el propio scout)

**Juicio justo.** Los expertos acertaron al rechazar la *forma* y se quedaron cortos en la *función*. Propusieron «un reactor dentro de orq» y lo dejaron «solo si el número duele» (`informe-2.md:93-104`, `:139` [V leído]). El número dolió: 88 % del tiempo de reloj de S-01 fue «nadie actuando». [U] Tu diagnóstico (el Orquestador como único salto es un freno) era correcto y ya lo reconocían (`informe-2.md:108-109` [V leído]).

**Recomendación: el diario es el bus; un solo despachador residente lo lee; los suscriptores son funciones, no agentes.**

- **Bitácora única.** Cada evento del diario gana: número de orden (`seq`), `proyecto`, `causa` (qué evento lo provocó) y `origen` (owner-tty, autopilot, agente, despachador). Los mensajes del canal se anotan también como evento. Así hay una sola línea de tiempo.
- **Despachador = bucle de reconciliación.** Concepto: en vez de reaccionar a cada evento (*por flanco*), cada pocos segundos compara «cómo debería estar» con «cómo está» y hace lo que falta (*por nivel*). Aquí significa: si se pierde un aviso, no importa, porque en la siguiente vuelta se ve que hay una tarjeta aprobada con tarea lista y sin worker. Es el patrón de los controladores de Kubernetes.
- **Deudas con plazo.** Una función pura lee el estado y devuelve la lista de lo que se debe: qué, quién, desde cuándo, cuándo vence. Ejemplos: «aprobada + tarea lista + cabe → lanzar en 60 s»; «worker sin commit en 30 min → empujar; al doble → parar y relanzar; a la tercera → escalar»; «`cierre` con SHA → verificar»; «verificado → cerrar (piloto automático) o avisar al Owner».
- **Arriendo con latido (lease/heartbeat).** Una tarea tomada lleva dueño y vencimiento; se renueva sola mientras haya señales de vida (commit, evento, estado de herdr). Si vence, vuelve a la cola. Aquí significa: ningún trabajo queda «en curso» para siempre a nombre de una sesión muerta.
- **Los agentes reciben trabajo por asignación, nunca por suscripción.** Un mensaje, un destinatario, una tarea. Eso conserva el costo y evita las acciones dobles.
- **Un despachador por proyecto**, con candado de instancia única (elección de líder con `flock`). Entre proyectos solo se comparte la medición de capacidad de la máquina, que ya existe.

Esto da las tres cosas que buscabas con el bus y no añade ningún lector de modelo. Las piezas ya están: `--tarea siguiente` elige la primera tarea lista [V] `F/app/workers.go:94-99`; lanzar es repetible [V] `F/app/sesiones.go:50-66`; las alertas ya se calculan [V] `F/app/alertas.go`. Falta el bucle que las llame.

---

## 4. Fallos

Regla propuesta: **cerrado** (denegar y parar) para todo lo que escribe en `main`, el examen o una aprobación; **abierto** (seguir y anotar) para lo que solo informa; **reintento acotado** para infraestructura; **escalar** cuando se acaba el presupuesto.

| Qué falla | Hoy | Debería | Peligro con piloto automático |
|---|---|---|---|
| Aprobada y nadie lanza | Nada. 17 h 46 min. [V] `RS/diario.jsonl:15` | Despachador lanza en segundos; si no cabe, encola y reintenta | — (lo resuelve) |
| Worker muerto o esperando una respuesta | Indistinguible de vivo. [V] `F/terminal/terminal.go:47-53` | Leer el estado de herdr; arriendo vence → empujar, parar, relanzar, escalar | Medio: relanzar sin tope gasta cuota |
| Arranque o envío «desconocido» | Se niega a repetir y pide mirar el panel. [V] `F/app/sesiones.go:63-66`, `:152-153` | Resolver observando (¿existe el agente?, ¿llegó el contrato?) y solo entonces reintentar | Alto si se reintenta a ciegas: dos workers en el mismo clon |
| Límite de capacidad | Rechaza. [V] `F/recursos/recursos.go:60` | Encolar; el despachador reintenta cuando quepa | Bajo |
| Examen falla | Hasta 2 reparaciones, luego BLOQUEADO; 3 intentos y decide el Owner. [V] `F/slice/slice.go:53-56`, `F/slice/nucleo.go:482`, `:529` | Igual. El tope no lo sube nadie más que el Owner | Alto si el piloto pudiera subir el tope: bucle infinito con costo |
| El examen no cubre lo que la historia toca | S-01 promete tablero y el examen solo corre Python. [U] `astra-evaluacion.md:17` | Cerrado: no se abre la historia si una carpeta tocada no tiene comprobación | **El más grave**: integra en `main` código no probado y lo llama aceptado |
| Tarjeta mal entendida | El Owner la lee y aprueba | Sesión de contraste: re-pregunta la historia contra el código; duda abierta → no auto-aprobar | Alto: se construye bien lo equivocado |
| «Aceptado» anotado y `main` sin publicar | La decisión se registra antes de publicar; un cambio local en `main` deja la publicación esperando. [U] `astra-evaluacion.md:55` | El recibo final es el commit en `main`; sin él no hay «entregado» | Alto: el Owner cree entregado lo que no está |
| Gate sin datos | Deniega. [V] `F/cli/v03.go:30`; lo que no es lanzamiento ni escritura pasa. [V] `F/gate/gate.go:107-108` | Igual | Bajo |
| Sin `bwrap` | No corre nada de un worker. [U] `B:sandbox/sandbox.go:5-6` | Igual | Bajo |
| Aprobación suplantada | Solo se comprueba que `ORQ_ROL` esté vacío; no es una comprobación de terminal real. [V] `F/cli/tarjeta.go:98-101` | Modo manual: tecla en `/dev/tty`. Modo piloto: la aprobación es un evento `origen=autopilot` con la regla que la disparó | Medio: en piloto el riesgo se convierte en «¿qué regla aprobó?», que sí se audita |
| Apagón a media escritura | Diario bien; canal, `sesiones.json` y memoria pueden perder lo último. [V] §2 | fsync en `WriteAtomic` y canal; guarda de línea rota en canal | Medio: el despachador reconstruye desde el diario, que sí es durable |
| Memoria con un hecho falso | Se inyecta a cada sesión nueva. [U] `F/cli/v03.go:411-440` | §6 | Alto: nadie humano lo filtra |
| El despachador muere | — | Candado de instancia única; lo levanta `orq2 up` o el hook de inicio; al volver, reconcilia desde el estado | Medio: sin él se vuelve al comportamiento de hoy, que es seguro |

**Lo que el piloto automático nunca puede hacer:** cambiar el examen después de sellado, subir topes de intentos, `--force`, `push`, aceptar con comprobaciones modificadas, ni integrar con una carpeta sin cubrir. Todo eso **escala y para**.

---

## 5. Latencia y cuellos de botella

| Tramo | Medido | Dónde se pierde | Arreglo más barato |
|---|---|---|---|
| **Nadie actuando** tras aprobar | 17 h 46 min (≈88 % del reloj) [V] `RS/diario.jsonl:15`; [U] reparto | Ningún proceso reacciona a la aprobación | Despachador (§3). De horas a segundos |
| **Owner**: tarjeta lista → aprobada | 2 h 03 min 45 s [V] `RS/diario.jsonl:14-15` | Espera humana; orq2 no avisa | Piloto automático la elimina; en manual, un aviso |
| **Agentes**: primer evento → tarjeta lista | 17 min 27 s [V] `RS/diario.jsonl:1`, `:14` | ≈6 min fueron fricción al abrir (sin red en la caja, dependencias) [U] `RS/canal.jsonl:27-39` | `check --caja` antes de la primera historia |
| **Agentes**: pregunta → respuesta | mediana 24 s, máx. 138 s, n=6 [U] | Turnos de modelo. No es el cuello | Nada; quitar turnos mecánicos con el reactor |
| **Pasos mecánicos por turno de modelo** | 9 acciones del Orquestador por fix, 12+ por funcionalidad [U] `informe-2.md:55` | Cada paso espera a que un modelo lea prosa y teclee | Reactor: `cierre` con SHA dispara juntar y verificar |
| **Arnés**: entrega de mensaje | ≤1 s [U] | No es cuello | — |
| **Arnés**: cada comando relee archivos enteros; `canal.Append` recorre todo el canal por mensaje [V] `F/canal/canal.go:92-104` | Irrelevante con 52 líneas | Crece al cuadrado con el tiempo | Cursor del despachador; no tocar hasta ~10 000 líneas |
| **Pruebas** | `go test -race` 4 min 24 s; `internal/slice` 245 s (dato del brief) | Cada prueba crea un repo Git real y corre la caja real; cero paralelismo. Bloquean `t.Parallel()`: `t.Setenv` y la variable global `EnUso` [U] `F/slice/slice_test.go:17-52`, `:416-424` | (1) `t.Parallel()` en las que no tocan entorno; (2) caja falsa en las que no prueban la caja; (3) `-short` para el ciclo de edición, completo solo en el candidato |

**Paralelismo, en una línea por nivel.** Tareas: el plan ya es un grafo (T1 → T2–T5 en paralelo → T6 [U] `RS/planes/01.json`); el despachador lanza todas las listas hasta donde diga `recursos.Cabe`. Verificaciones de historias distintas: en paralelo. Integración a `main`: **en serie a propósito**, con su candado. Pruebas del arnés: en paralelo por paquete y dentro del paquete.

---

## 6. Memoria

**Dónde vive y cómo se busca.** Un archivo `memoria.jsonl` por proyecto, hoy con 13 líneas, tope de aviso 60 y 200 caracteres por hecho. No hay índice ni búsqueda: se lee entero, se ordena por importancia y fecha, y se inyecta (hasta 25 líneas) al iniciar cada sesión. [U] `F/memoria/memoria.go:28-31`, `:174-186`; `F/cli/v03.go:411-440`. `.orq/` está excluido de Git, así que un clon nuevo no trae memoria. [U] `astra-evaluacion.md:67`

**Cómo entran los hechos falsos (m12 paso a paso).**
1. El autor se declara a sí mismo: `--fuente` es un parámetro, por defecto la variable `ORQ_ROL`. [V] `F/cli/proyecto.go:58`
2. «Confirmado» se calcula **a partir de quién dice ser**, no de una prueba: si la fuente es owner, atena o filtro, queda confirmado. [V] `F/memoria/memoria.go:63-66`, `:153-156`
3. La evidencia solo se exige a agentes que escriben `tipo=hecho`; Atena y las decisiones están exentas. [V] `F/memoria/memoria.go:86`
4. La evidencia es texto libre; nada la comprueba. El comprobador `VerificarCita` existe en `v2/memoria` pero ningún comando lo llama. [U] `MEM/memoria/cita.go:19`
5. Resultado: m12 dice «S-01 **se abrió** con --en-sitio» (un suceso), escrito por Atena en el mismo segundo en que *autorizó* hacerlo; 11 s después el comando falló porque la opción ya no existía. Quedó `confirmado`, sin evidencia. [V] `RS/memoria.jsonl:12`; [U] `RS/canal.jsonl` c15–c16
6. La corrección m13 es prosa, sin el campo `reemplaza`: los dos siguen vigentes y se inyectan juntos. [V] `RS/memoria.jsonl:13`

**El error de fondo:** se mezcla *autoridad* (quién puede decidir) con *verdad* (qué ocurrió). Atena tiene autoridad para decidir; no por eso lo que afirma ocurrió.

**Diseño que lo impide (procedencia = de dónde sale cada afirmación, comprobable por máquina):**
- **Tres clases, cada una con su prueba obligatoria.**
  - *Hecho del código*: cita `archivo:línea` + fragmento, más el commit y la huella del archivo. El arnés la comprueba al escribir.
  - *Suceso* («se abrió», «se aprobó», «falló»): **no se escribe en memoria**; se enlaza al evento del diario por su `seq`. El diario es la única verdad de lo ocurrido. m12 no habría podido existir: no había evento de apertura con esa opción.
  - *Decisión*: enlaza al evento que la registra (tecla del Owner o regla del piloto automático).
  - Todo lo demás es *intención* o *hipótesis* y se muestra como tal.
- **«Confirmado» lo calcula el arnés** cuando la prueba pasa. Nadie lo declara. Quien escribe no puede confirmar lo suyo.
- **El autor lo pone el arnés** a partir del panel desde el que se ejecuta el comando (`RolDePane` ya existe [V] `F/canal/canal.go:192-199`), no de un parámetro.
- **Caducidad.** Al inyectar, se compara la huella guardada con la del archivo actual; si cambió, el hecho se marca «por reverificar» y no se presenta como cierto.
- **Reemplazo con motivo y evento en el diario** (la librería ya está en `v2/memoria`; falta el comando [U]).
- **Índice mínimo.** Con decenas de hechos no hace falta base de datos ni búsqueda semántica. Basta un índice en memoria por ruta citada y por historia, para inyectar solo los hechos cuyas rutas tocan el alcance de la historia (S-01 declara 23 rutas [U] `RS/slices/01.json`).
- **Qué va a dónde:** hechos locales en `memoria.jsonl`; lo ocurrido en el diario; reglas duraderas en la guía versionada del repo, que sí viaja con Git.

---

## 7. Reutilizable en otros proyectos

- **Identidad hoy: no existe.** `proyecto.json` solo guarda modo y señales [V] `RS/proyecto.json`. Los agentes se nombran `<rol>-<nombre de carpeta>` recortado a 18 caracteres [V] `F/orqdir/orqdir.go:36-54`: dos repos con la misma carpeta, o con los mismos 18 primeros caracteres, chocan. El piloto ya vio sesiones ajenas contadas como workers propios (P6). [U] `pilot-log.md`
- **Qué es global de la máquina:** `limits.json`, el `HALT` de disco [V] `F/state/state.go:24-29`, `~/worktrees`, la medición de RAM/CPU. Es correcto que el disco sea global; no lo es que no haya forma de saber qué proyecto ocupa qué.
- **Propuesta:** `orq2 init` genera un `id` estable en `proyecto.json` (corto, aleatorio, más el nombre). Ese `id` va en cada evento, en el nombre de cada sesión y en una etiqueta del clon. Un registro de máquina (`~/.local/state/orq2/proyectos.json`: id → ruta) permite que cada despachador reconozca solo lo suyo y que un tablero vea todos. No hace falta bus entre proyectos: cada uno tiene su diario y su despachador.
- **Genérico (sirve tal cual):** canal, diario, slice, gate, caja, límites, capas, despachador.
- **Atado a Real State o a un solo lenguaje:** las comprobaciones se eligen desde la raíz y la caja monta `node_modules` de la raíz; la configuración del piloto solo tiene Python aunque hay un tablero React. [U] `astra-evaluacion.md:61`. Lo genérico pendiente es **comprobaciones por carpeta**: cada carpeta que la historia toca declara su comando, y sin él la historia no se abre.

---

## 8. Recomendación

**Arquitectura objetivo (10 puntos).**
1. **Una bitácora por proyecto** (el diario) con número de orden, proyecto, causa y origen. Es el bus.
2. **Un despachador residente por proyecto**, instancia única, que reconcilia cada pocos segundos: estado → deudas → acciones idempotentes. Es código, no un modelo.
3. **Deudas con responsable y plazo** como función pura y visible (`orq2 pendientes`): nada puede quedar sin dueño.
4. **Arriendo con latido por tarea**: empujar → parar → relanzar → escalar, con topes.
5. **Equipo por capacidad**: la tarea ya declara zona y dificultad; una tabla las convierte en tipo de agente, modelo y guías. El despachador asigna; los modelos solo intervienen donde hay juicio (fallo, bloqueo, duda).
6. **Un mensaje, un destinatario.** Nadie se suscribe. Sin colmena.
7. **Piloto automático = política escrita**, elegida en `init`: cada aprobación automática es un evento con la regla que la disparó; con presupuesto de intentos, tiempo y gasto; con una lista corta de cosas que solo escalan.
8. **Examen por carpeta tocada + sesión de contraste** antes de construir: son las dos condiciones para que el piloto pueda aprobar solo.
9. **Memoria con procedencia**: «confirmado» lo calcula el arnés; los sucesos se enlazan al diario; lo citado caduca cuando cambia el código.
10. **Fallar cerrado en `main`, examen y aprobaciones; durable en disco** (fsync, guarda de línea rota, candado en `sesiones.json`).

**Orden de construcción** (de menor a mayor; cada paso, su valor y su prueba):

| # | Paso | Valor para «escribo la historia y nada más» | Cómo se prueba |
|---|---|---|---|
| 1 | `id` de proyecto + `seq`, `proyecto`, `origen` en eventos | Cada orquestador reconoce lo suyo | Dos repos con el mismo nombre de carpeta no se ven entre sí |
| 2 | Durabilidad: fsync en `WriteAtomic` y canal, guarda de línea rota, candado en `sesiones.json` | El despachador podrá fiarse del disco tras un apagón | Corte simulado a media escritura (ya existe `ORQ_FALLA_EN`): no se pierde ni duplica nada |
| 3 | `orq2 pendientes` (solo lectura) | Se ve quién debe qué y desde cuándo | Sobre una copia del `.orq` de S-01 imprime: «S-01 aprobada hace 17 h, T1 lista, sin worker» |
| 4 | `orq2 despachar --una-vez`, luego residente | Elimina el atasco de 17 h 46 min | Terminal falsa: aprobar → worker lanzado en <5 s; dos pasadas → un solo worker; matar a medias → converge |
| 5 | Supervisión: estado de herdr + arriendo + escalado | Destraba o reemplaza agentes sin ti | Worker falso que nunca hace commit: empuje, parada, relanzamiento y aviso, en ese orden y con tope |
| 6 | Reactor de verificación: `cierre` con SHA → juntar y verificar | Menos turnos de modelo por historia | Mismo `cierre` dos veces → una verificación; acciones del Orquestador por fix de 9 a ≤4 |
| 7 | Comprobaciones por carpeta + sesión de contraste | El examen cubre todo lo tocado; la historia se cuestiona contra el código | Historia que toca `dashboard/` sin comprobación declarada no abre |
| 8 | Memoria con procedencia (cablear `VerificarCita`, autor por panel, sucesos al diario, caducidad, `mem reemplazar`) | Memoria sin hechos falsos | Repetir el caso m12: queda como intención, nunca confirmado |
| 9 | Piloto automático: política, presupuestos, lista de «solo escalar» | El objetivo completo | S-02 de punta a punta: una sola acción del Owner (la historia), commit en `main`, demo y recibo |
| 10 | Pruebas rápidas (`t.Parallel`, caja falsa, `-short`) | Cada paso anterior se construye más rápido | `internal/slice` de 245 s a <60 s sin perder casos |

Los pasos 1–4 son pequeños y por sí solos habrían evitado el atasco de S-01. El paso 9 no debe adelantarse al 7 y al 8: sin ellos, el piloto automático integraría código sin probar guiado por memoria sin comprobar.

**Lo que no haría:** bus con agentes suscritos, memoria de colmena, base de datos o cola externa, búsqueda semántica, un segundo orquestador de modelo, ni un servidor. Nada de eso acerca la primera historia entregada sin vigilancia.

**Lo que no verifiqué.** No corrí pruebas ni comandos de orq/herdr. No leí completos `slice/cierre.go`, `gate/shell.go` ni la rama `v2/locks`. Las cifras de estudios externos son las del scout del equipo. Lo marcado [U] con ruta es lectura de scouts que no repetí.
