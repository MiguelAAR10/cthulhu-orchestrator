# orq2: qué vamos a desarrollar al retomar — definición de Fable (una pasada, solo lectura)

Marcas: [V] = leído en la ruta citada · [U] = decisión, estimación o inferencia mía. Rutas cortas: `v2.1/` =
`~/worktrees/cthulhu-docs/docs/harness-v2/v2.1/` · `harness/` = `…/cthulhu-orchestrator/harness/` · H = `lean/hechos.md` ·
T01–T15 = transcripciones en `~/cthulhu-evidence/2026-11-20|21/transcripts/` (número de archivo @ minuto).
No corrí nada. Los tokens son órdenes de magnitud [U]; se miden en el entregable 1 y se recalibran.

## 1. Qué es orq2 a partir de ahora

orq2 es **la fábrica de software del Owner**: una cocina Michelin, no una máquina de platos. El Owner es el chef que
diseña la cocina (entorno, recetas, controles) y prueba por muestreo; los agentes cocinan [V T15 @49:42–49:58]. Es
para un solo dueño con pocos tokens que construye productos reales (hoy Real State) y no acepta "slop".
La columna es la lógica de Lauren Tan: la confianza **se sube por una escalera** hecha de verificación y entorno, no
se regala [V T01 @1:23–1:32; T15 @18:43–19:16]; lo determinista va en scripts y el juicio en agentes [V T01
@2:03–2:27]; cada error repetido se vuelve una restricción [V T15 @31:43–32:01]; contexto, no control [V `v2.1/rumbo.md` D33].
**Valor en una frase:** cada pedido recibe solo la ceremonia que su tamaño y su riesgo merecen, y sale con prueba.

## 2. Todas las D evaluadas

Ahora = entra en esta versión (E1–E6 de la sección 3) · Siguiente = versión posterior · Después = solo con tokens de sobra.

| D | Veredicto | Razón en una línea |
|---|---|---|
| D1 identidad por proyecto | Siguiente | Riesgo real pero no frenó S-02; el perfil por carpeta lo puede hacer el Owner fuera de orq2 [V `v2.1/owner-decisions.md`] |
| D2 agentes sugestionables | Ahora (regla) | No se construye: decide la evidencia, revisores independientes y de solo lectura (E6) |
| D3 referencia primero | Ahora (E5) | **Se fusiona con D8, D12 y D19**: una sola tabla de referencias |
| D4 criterios de ingeniería | Ahora (regla) | Va en la tarjeta de la ruta feature; no aplica al fix |
| D5 skills por proyecto | Siguiente | **Igual que D20.** El catálogo ya existe [V `harness/internal/perfil/catalogo.json`]; ahora solo se muestra |
| D6 escala por tarea | Ahora (E2) | **Igual que D28**: es el router |
| D7 cercas contra bucles | Ahora (E3) | Tope de intentos que cuenta solo código; salida explícita por ruta [V T07 @7:22 "max two rounds"] |
| D8 base externa de verdad | Ahora (E5) | Fusionada en D3 |
| D9 documentos vivos | Después | C4 vivo en cada cierre es ceremonia; queda solo la ficha de una página de D17 |
| D10 re-preguntar | Ahora (E2) | Ganó M1 [V `v2.1/test-day-score.md`]; el router re-pregunta si una palabra cambia el alcance |
| D11 verificación al límite | Después | Mutación solo a pedido en rutas de riesgo; ya dio 19/19 una vez [V mismo archivo, M6] |
| D12 mapa de capacidades | Ahora (E5) | Fusionada en D3/D18 |
| D13 entorno compartido | Ahora (E1, E3) | **Igual que D23.** Fue la causa del intento perdido en S-02 [V E06–E09] |
| D14 identidad y pulpo | Ahora (E2) | **Igual que D29**: una skill dentro de Claude Code, cero Go |
| D15 preguntas de arranque | Ahora (E2) | Modelos, cuota, importancia: 5 respuestas guardadas |
| D16 nivel de autonomía | Ahora (E2) | Se decide **por tipo de puerta**, no solo por proyecto [V T15 @56:04–56:11] |
| D17 documento de producto | Ahora, mínimo (E5) | Una página y solo en la ruta producto; las once etapas completas, Después (miedo F5) |
| D18 usar el arte existente | Ahora (E5, E6) | El mockup de S-02 no usó el diseño aprobado [V `v2.1/owner-vision-gaps.md`] |
| D19 repos de referencia | Ahora (E5) | Fusionada en D3 |
| D20 skills validadas | Siguiente | Fusionada en D5; primero medir si las que hay evitan algo |
| D21 wireframes y flujos | Siguiente | Solo cuando haya pasos nuevos; no hay caso medido todavía |
| D22 avisar al humano | Siguiente | Útil, pero en fix y pregunta el Owner ya está en la sesión |
| D23 contexto vivo y breve | Ahora (E1) | Fusionada en D13 |
| D24 código como base de datos | Siguiente | Ahora solo la semilla: una fila por componente tocado; índice completo cuando se pruebe que se usa |
| D25 arquitectura de experto | Siguiente | Necesita la ruta producto funcionando (E5) y un caso real |
| D26 skills que aprenden | Ahora (E6) | El motor existe sin conectar [V `harness/internal/instinto/instinto.go:1-3`] |
| D27 el usuario define el nivel | Ahora (E2) | **Se fusiona con D16 y D31**: un archivo corto de política del proyecto |
| D28 router liviano | Ahora (E2) | Fusionada en D6 |
| D29 interfaz en Claude Code | Ahora (E2) | Fusionada en D14 |
| D30 expertos como herramienta | Ahora, 2 de 4 (E6) | Seguridad y gusto visual; datos y scout de repos, Siguiente |
| D31 política + solo lectura | Ahora (E2, E6) | Revisor de solo lectura por definición [V T12 @5:29]; línea de mantenimiento, Después |
| D32 fábrica medible | Ahora parcial (E4) | Registro de corrida y métricas ahora; fábrica como código y jueces, Siguiente; benchmarks, Después |
| D33 lógica Poteto | Ahora (columna) | Verificación con manos (E1), restricciones (E6), puertas y muestreo (E2); cola de hallazgos, Siguiente |

## 3. Lo que vamos a desarrollar: versión siguiente ("Escalera, peldaños 1 y 2")

Ordenado por valor para el Owner. Cada entregable se usa solo y se puede parar ahí. Presupuesto total ≈5–6 M tokens [U].

**E1 · La cocina lista: skill de verificación del proyecto, con manos y ojos.** (el más valioso)
- El Owner ve: cualquier agente nuevo levanta Real State y lo prueba como usuario con un solo comando, sin buscar.
- Cierra: D13, D23, D33.1; baja E06, E08, E09, E27. Poteto la llama la skill más importante; sin script, cada agente
  "reconstruye el mundo" cada vez [V T15 @18:43–19:16, @22:56–23:12]. Eso fueron los bucles de entorno de S-02.
- Reuso/nuevo: **skill + un script pequeño dentro de la skill** (nuevo, sin Go). El script lleva lo determinista:
  comando por clase de prueba (unitaria, integración, e2e), servicios, puertos, nombre del `.env`, 2 capturas.
- Modelo: Sonnet. Tamaño: 1 día. Tokens ≈0,6 M.
- Prueba: un agente recién abierto, sin ayuda, corre todas las clases de prueba y entrega 2 capturas (1280 y 360).

**E2 · El pase: entrada `/cthulhu` + router por tamaño y por puerta.**
- El Owner ve: el pulpo, 5 elecciones (modelos, cuota, importancia, acabado, autonomía) y, por pedido, una línea:
  "ruta X, porque Y, cuesta Z". Una pregunta abre 0 sesiones; un fix no despierta al equipo.
- Cierra: D6, D10, D14, D15, D16, D27, D28, D29, D31 (archivo de política); esquiva E01, E02, E05, E20 en lo chico.
- Reuso/nuevo: **skill**. Extiende la guía que ya existe: 3 rutas, 5 condiciones para fix, solo sube [V
  `harness/internal/cli/orqskills/orq-ruta/SKILL.md`]. Nuevo: ruta "chico pero de riesgo" y ruta "decidir". Regla de
  puertas: ida y vuelta (se revierte) puede seguir sola y se revisa por muestreo; de una sola vía (datos, pagos,
  irreversible) siempre para en el Owner [V T15 @52:45, @56:04–56:11]. Empezar liviano y subir si crece [V T10 @3:42–4:04].
- Modelo: Sonnet. Tamaño: 1 día. Tokens ≈0,5 M.
- Prueba: 4 pedidos (pregunta, etiqueta, cambio de cobro, producto nuevo) dan 4 recorridos distintos y explicados.

**E3 · La línea honesta: el arnés deja de castigar lo que no es código.**
- El Owner ve: 0 intentos gastados por entorno o papeleo; tras aprobar, el primer worker arranca solo.
- Cierra: lean 1, 3 (recortado), 4 y 6 [V `v2.1/lean-plan.md`] = E07, E08, E10, E12, E13, E14, E15; E17.
- Reuso/nuevo: **Go**, sobre lo que hay. El conteo de intentos por causa ya tiene dónde vivir [V
  `harness/internal/slice/metricas.go` campo `IntentosPorCausa`]. El preflight llama al script de E1.
- Modelo: Sonnet (worker corre todos los checks: Haiku). Tamaño: 2–2,5 días. Tokens ≈2 M.
- Prueba: herramienta ausente → contador igual; `type: ignore` en el examen → rechazado al sellar; test viejo roto
  → rechazado al juntar; después de `aprobar`, ningún otro comando.

**E4 · El plato con su ficha: hoja de prueba en git + registro de la corrida.**
- El Owner ve: una página por historia cerrada (qué se pidió, qué se hizo, que funciona, cómo se ve, segunda
  opinión, cuánto costó), guardada en el repo, sin que nadie la arme.
- Cierra: huecos de prueba [V H§B: `.orq/` excluido de git, `costo n/d`], lean 7 sin demo propia, D32.2 y D32.5.
  Métricas: tiempo de ciclo e intervenciones humanas por historia [V T13 @12:53–13:02].
- Reuso/nuevo: **Go + plantilla**. Datos ya existentes (recibos, diario, registro); capturas del script de E1. Nuevo:
  ruta, modelo, duración y quién decidió por lanzamiento.
- Modelo: Sonnet; plantilla Haiku. Tamaño: 1 día. Tokens ≈1 M.
- Prueba: cerrar una historia → la página con 6 secciones y 2 capturas se abre desde un clon nuevo.

**E5 · La carta: ruta "decidir producto" en una página.**
- El Owner ve: antes de construir, una ficha (usuario, problema, éxito, exclusiones), 2 opciones con lo que se
  pierde en cada una, tabla de repos de referencia y el arte aprobado vigente. Parar aquí con una decisión es éxito.
- Cierra: D3, D8, D12, D17 (mínimo), D18, D19. Es la respuesta a "no solo dame el componente" [V `v2.1/owner-vision-gaps.md`].
- Reuso/nuevo: **skill** que encadena las del catálogo (`grilling`, `to-spec`, `codebase-design` [V `catalogo.json`]);
  nuevas solo dos piezas: scout de referencias (Haiku) e inventario de arte.
- Modelo: Sonnet conduce, Haiku busca. Tamaño: 1 día. Tokens ≈0,8 M.
- Prueba: una decisión pendiente de Real State; el Owner elige o descarta con las 4 piezas a la vista.

**E6 · El control de calidad que aprende: 2 expertos de solo lectura + errores que se vuelven restricciones.**
- El Owner ve: un veredicto corto de seguridad y otro de gusto visual cuando la señal lo pide; y una tanda de máximo
  5 lecciones propuestas para aprobar, cada una preferiblemente como check automático.
- Cierra: D2, D26, D30 (2 de 4), D31, D33.2–3; E16. En S-02 la falta de control de sesión en `/api/properties` se
  encontró por casualidad [V `v2.1/rumbo.md` D30]. Las reglas en texto se olvidan y se acumulan; los guardarraíles no
  [V T02 @4:48–4:57, @6:31–6:55].
- Reuso/nuevo: **subagentes de Claude Code (archivos) + `orq2 aprender`** [V `harness/internal/cli/comandos.go:20`].
  Primera cosecha: una pasada Haiku sobre las sesiones del 20 y 21. `skill eval` NO se usa: depende de una función de
  acceso anticipado [V `harness/internal/skilleval/correr.go:85-86`].
- Modelo: Sonnet; minería Haiku. Tamaño: 1 día. Tokens ≈0,6 M.
- Prueba: sobre el diff de S-02, el experto de seguridad marca la ruta sin sesión; el revisor no puede escribir.

### Cómo y cuándo se CIERRA la versión (línea de alto)
Se cierra con lo **primero** que ocurra de estas tres cosas, sin prórroga:
1. **Meta cumplida:** los 6 entregables pasaron su prueba y hay 3 pedidos reales de Real State hechos con la versión
   (1 pregunta, 1 fix, 1 feature), cada uno con su hoja de prueba.
2. **Tope de gasto:** 7 M tokens u 8 sesiones de trabajo [U]. Lo que no entró pasa a la versión siguiente.
3. **Freno por evidencia:** si al terminar E3 el fix real no mejora a S-02 en los números de la sección 5, se para
   y se revisa el rumbo antes de gastar en E4–E6.
El cierre deja: la hoja de comparación contra S-02, el registro de defectos al día y una lista corta de "siguiente".

## 4. Fuera de alcance de esta versión (y por qué)

- **Re-sello en un paso y sesiones con vencimiento** (lean 2 y 5): solo duelen en feature con paneles y sin vigilancia;
  la decisión ya tomada es posponerlos hasta que el Owner quiera corridas desatendidas [V `v2.1/rumbo.md`].
- **Demo en clon propio** (lean 7) y paquete de roces (lean 8, salvo E17): las capturas salen del script de E1.
- **Integración automática a main y disparadores externos 24/7:** es el peldaño 3; requiere confianza ganada.
- **Aislamiento de credenciales dentro de orq2 (D1), C4 vivo (D9), mutación en cada historia (D11).**
- **Índice completo, grafo, base vectorial, etiquetas por archivo (D24); etapa de arquitectura (D25); wireframes (D21).**
- **Avisos (D22), catálogo ampliado de skills (D5/D20), expertos de datos y de repos (D30).**
- **Fábrica como código, jueces automáticos y benchmarks propios (D32):** valen cuando haya 5+ corridas registradas.
- **Debate de planes entre modelos por rutina:** es el patrón más caro y puede no converger [V `v2.1/rumbo.md`, tabla Google].
- **Panel nuevo, bus de mensajes nuevo, tokens exactos.** Basta modelo × duración.

## 5. Criterio de "hecho bien"

1. **Escalera:** peldaño 1 = cualquier agente verifica solo (E1); peldaño 2 = un fix de puerta de ida y vuelta llega
   a "listo para integrar" sin que el Owner mire, y él revisa por muestreo. La versión está bien si llega al 2.
2. **Números contra S-02** [V `v2.1/lean-plan.md`, H]: historia → primer worker ≈25 min → ≤15; intentos sin defecto
   de código 3 → 0; tipos de intervención a mano ≈10 → ≤2; fix corto ≤10 min y 0 preguntas [U meta].
3. **Evidencia:** toda historia cerrada tiene su hoja en git, con quién decidió cada cosa; nada se afirma sin recibo.
4. **Oficio, no slop:** la pantalla nueva usa el arte aprobado (lo confirma el experto visual con capturas); el
   experto de seguridad no deja pasar un High; ningún archivo fuera del alcance de la tarjeta.
5. **Menos reglas, más restricciones:** al cierre, al menos 2 errores repetidos quedaron como check automático y
   ninguna skill creció en texto.
6. **Honestidad:** lo que no se midió se dice "no medido"; tests en verde solos no cierran nada.

## 6. Primer paso concreto al retomar

Una sola sesión, **Sonnet**, en el clon de prueba de Real State, solo E1: escribir la skill de verificación y su
script, y probarla con un agente recién abierto. Sale: la skill, el script, 2 capturas, y **la medida real de tokens
y minutos** de esa sesión. Con esa medida se recalibran los presupuestos de E2–E6 y se confirma o baja el tope de 7 M.
Si el agente nuevo no logra verificar solo, no se avanza a E2: se arregla E1.

## 7. Lo que más me preocupa

- Que la fábrica produzca papel: E5 se juzga por una decisión real del Owner, no por tener la skill.
- Un router que clasifica hacia abajo un cambio de riesgo: por eso solo sube y las puertas de una vía siempre paran.
- Posponer el re-sello (E23–E25 son gravedad A): si una feature vuelve a tropezar ahí, se adelanta y sale E6.
- Mis tamaños no están medidos y no leí por dentro `init`, `contexto` ni la caja; E3 es el que más puede crecer.
- El script de E1 puede envejecer y todos le creerán: el preflight de E3 es lo que lo mantiene honesto.
