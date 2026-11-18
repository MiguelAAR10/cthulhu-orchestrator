# Informe de revisión externa — `orq` (Cthulhu Orchestrator)

Revisado: commit `248c9c7` (rama `v2/build`), 18-nov-2026. Solo lectura; el árbol quedó limpio.
Rutas relativas a `harness/` salvo que se diga otra cosa. **[V]** = verificado (lo corrí o lo leí). **[I]** = inferido.

## Veredicto en corto

El núcleo (examen con huella, copia limpia en bubblewrap, recibo, cierre reanudable) es bueno y funciona.
Pero en el último commit **el guardia de los workers está apagado** y **un worker puede cambiar el examen y obtener "VERIFICADO"** — lo reproduje de punta a punta. Los tests no pasan. El README describe una versión anterior.

## 1. Qué es, según el código

Un CLI en Go (solo librería estándar, [V] `go.mod` sin dependencias) con tres piezas:

1. **Máquina de estados de "slices"** (`internal/slice`, ~3.200 líneas): abre una historia a partir de un test que falla, guarda su huella sha256 y un "contrato" en `refs/orq/`, da al worker un clon de git, y luego verifica, integra y cierra.
2. **Gate** (`internal/gate`, ~1.500 líneas): un hook `PreToolUse` de Claude Code que analiza el comando de shell y niega por lista (push, borrar, escribir el examen, lanzar agentes).
3. **Caja** (`internal/sandbox`): bubblewrap con raíz por lista permitida y sin red, para correr el examen.

Alrededor hay mucho más: mensajería entre sesiones, memoria, "instintos", escriba con LLM, relevos, plan de tareas, perfil de proyecto, métricas, adaptadores de herdr y Orca. Son ~12.200 líneas de Go sin tests y ~4.300 de tests [V].

## 2. Promesas del README frente al código

| Promesa | Estado | Evidencia |
|---|---|---|
| Go, solo stdlib | **Se cumple** [V] | `go.mod` |
| "~9k líneas · 86 tests · 22 comandos · 15 paquetes · v1.0.0" (`README.md:12,109`) | **No se cumple** [V] | 12.194 líneas, 156 funciones de test, 24 comandos, 26 paquetes; el HEAD es un WIP |
| Todo tool call del worker pasa por el gate | **No se cumple** en HEAD [V] | ver problema 1. Codex no pasa por ningún hook [I] (`agente/agente.go:26-38`) |
| El worker no puede editar el test | **Parcial** [V] | Verificar sí detecta el cambio (control S-02: BLOQUEADO). Pero ver problema 2 |
| El test se re-ejecuta intacto, con huella | **Se cumple** [V] | `slice/nucleo.go:86-110`, `:302-354`; corrí el flujo completo |
| "0 tests ≠ verde" | **Parcial** | Solo con `--reporte`. Con `--sin-conteo` manda el código de salida (`slice/verif.go:19`) [V]. El reporte lo escribe el mismo proceso que corre código del worker [I] |
| Rojo debe ser fallo de aserción | **Parcial** [V] | Solo si hay reporte (`nucleo.go:205-207`) |
| Merge rechaza commits fuera de zona o que tocan el test | **Se cumple** en lectura [V leído, no ejecutado] | `slice/equipo.go:168-233`, `slice/clones.go:225-252` |
| Máx. 4 workers, cada uno en un worktree enlazado | **No se cumple** [V leído] | ahora son clones (`clones.go:76`); el conteo solo ve worktrees enlazados (`app/capacidad.go:25`), así que cuenta 0 |
| Git destructivo bloqueado | **Se cumple** donde hay hook [V] | `gate/reglas.go:88-124`; `git push --force` → exit 2 |
| Disco, HALT, medición fresca | **Se cumple** en lectura [V leído] | `gate/gate.go:101-146` |
| 2 reparaciones → BLOQUEADO; 3 intentos | **Se cumple** [V leído] | `slice/slice.go:53,56`; `--decision-owner` lo salta |
| Falla cerrado si `orq` falta | **No verificado** | Depende de `onFailure: block` (`templates/claude-settings.json:5`); no pude confirmar que Claude Code lo respete |
| Cierre reanudable tras matar el proceso | **Se cumple** [V] | `e2e-v05.sh`: S-02 a S-05 pasan |
| "Próximo: workers en sandbox" | Coherente [V] | nada en `internal/app` usa la caja |

## 3. Problemas, por gravedad

**1. El gate no reconoce los clones: en HEAD el worker no tiene guardia.** [V]
`cli/cli.go:175` usa `gate.IsLinkedWorktree` (`gate/gate.go:209`: `.git` es un archivo). Desde el WIP, `orq slice abrir` crea un clon con `.git` como carpeta (`slice/clones.go:76`). La marca `orq-clon` existe para esto, pero el gate no la lee (`LeerMarca` solo se usa en `slice/cierre.go:401`).
Prueba: abrí un slice y mandé al gate, con `cwd` en el clon: Edit del examen, `echo > examen`, herramienta Agent, Write a `.orq/slices/01.json` del principal, `orq slice cerrar --aceptado`. **Los cinco: exit 0.** El e2e del propio proyecto lo confirma: "gate: un worker no cierra (esperaba 2, salió 0)".

**2. Un worker puede cambiar el examen y aprobarse solo.** [V]
`orq slice examen ARCHIVO --aprobado "texto"` (`cli/proyecto.go:716-725` → `slice/nucleo.go:512`) vuelve a tomar la huella con el examen ya modificado. La "aprobación" es texto libre. El gate solo reserva al orquestador `abrir, cerrar, reintentar, tomar, lanzar, juntar` (`gate/reglas.go:161`); `examen`, `alcance`, `rehuella`, `reabrir`, `verificar` y `halt --set` quedan libres.
Prueba completa: en el clon reescribí el test para que siempre pase, hice commit, corrí `orq slice examen … --aprobado "req ATN-1 de Atena"` y luego `verificar` → **"verificado: el examen pasa intacto"**; `cerrar --aceptado` lo integró en `main` con `src/total.txt` todavía en `0`.
No depende del problema 1: en un worktree enlazado real el gate también dejó pasar `cd <principal> && orq slice examen …` (exit 0) y `bash arreglo.sh` (exit 0).

**3. El ancla de confianza (`refs/orq/`) está al alcance del worker.** [V el gate; I el resto]
`slice/contrato.go:6` dice "un worker encerrado no puede escribir ahí". Pero los workers no corren encerrados: se lanzan tal cual en un panel (`app/workers.go:163-170`). El gate permitió `git -C <principal> update-ref refs/orq/contrato/01 …` y `update-ref refs/heads/slice/01` desde un worktree enlazado (exit 0). `validacion/aislamiento.sh` (14/14) prueba la caja con `orq sandbox exec`, que no es el camino por el que se lanza un worker.

**4. El último commit no pasa sus propios tests.** [V] Ver sección 4.

**5. El gate es una lista de negaciones sobre un analizador de bash de 756 líneas.** [V]
Pasaron en worktree enlazado: `awk 'BEGIN{print … > "tests/aceptacion/s01.sh"}'`, `python3 -c "open('tests/'+'aceptacion/…','w')"`, `curl -o tests/aceptacion/…`, `… | tar xf -`. `awk` no está en `esInterprete` (`gate/reglas.go:433`) y `nombra` (`:265`) busca la subcadena literal. Verificar atrapa estos casos después (salvo el problema 2), así que el gate sirve como aviso temprano, no como frontera de seguridad. El README lo vende como frontera.

**6. `orq slice cerrar` llama a `claude -p` siempre, sin opción de saltarlo.** [V]
`cli/proyecto.go:671` (`EscribirFicha`). `ORQ_SIN_DOC` solo se mira en `juntar` (`:766`). Cerrar depende de red, cuenta y coste.
**Aviso:** mis tres cierres de prueba dispararon tres llamadas reales a `claude -p` con el contenido del repo de juguete (nada del proyecto). No lo esperaba.

**7. `ORQ_CONFIG` significa dos cosas.** [V]
Archivo en `config/config.go:100`, carpeta en `instinto/instinto.go:43`. Con una carpeta (como pedían las reglas de esta revisión) `orq slice abrir` falla: "read …/config: is a directory".

**8. Documentación y ayuda desfasadas.** [V]
- `cli/cli.go:57,60` ofrece `--en-sitio`; `slice/nucleo.go:184` lo rechaza.
- `slice/contrato.go:108` manda a `orq slice convertir`, que no existe.
- `docs/REGLAS-HARNESS.md:16-18` define worker como "worktree enlazado".
- `orq slice estado` rotula como "bloqueado:" el motivo de un slice VERIFICADO.

**9. Tests que prueban menos de lo que parecen.** [V]
- `validacion/validar-mvp.sh:85` crea el worktree a mano con `git worktree add`: la demo "le ordenamos hacer trampa" no pasa por `orq slice abrir`, o sea no prueba el camino real. Exige `~/bin/orq` instalado (`:12`); no lo corrí.
- `gate/gate_test.go:41` y `decisiones_test.go:23` inyectan `IsWorktree`: 100 % verdes con el gate ciego a los clones.
- `gitx`, `escriba`, `state`, `orqdir` no tienen tests.

**10. El examen corre sin red y sin opción.** [I]
Un examen que necesite una base de datos local no puede verificarse (`slice/caja.go:155`); `agente.go:24` reconoce que los workers la usan.

**11. Código heredado y duplicado.** [V leído]
Dos modelos conviven (worktree enlazado 1.0.0 y clon): `esEnlazado` en `cierre.go:48,179`, `equipo.go:373`. `Huella` y `huellaSegura` hacen lo mismo. `--aislado` no hace nada (`cli/proyecto.go:446`). `cli/proyecto.go` son 857 líneas de `switch`.

## 4. Estado del último commit

`248c9c7` solo borra un documento; el código es el de `c592a25`, cuyo mensaje dice "go build passes; tests not run". Go 1.25.8, bwrap presente.

- `go build ./cmd/orq`: **pasa**. `go vet ./...`: **pasa**.
- `go test ./...`: **FALLA**. 174 ejecuciones: 166 pasan, **7 fallan**, 1 se omite (herdr en vivo). Todas en `internal/slice`; los otros 18 paquetes con tests pasan. Mismo resultado con `HOME` real y con `HOME` temporal.
  - `TestVerdeSinPruebasEjecutadasFalla`, `TestEstadoViejoSigueVerificandoYCerrando`, `TestEstadoViejoVerificaEnElWorktree`: "el estado no coincide con el contrato".
  - `TestVerificarAisladoPorDefectoYOptOut`: "--en-sitio ya no existe".
  - `TestAlinearDetectaDesvio`, `TestPrepararDeps`, `TestRehuella`.
- `validacion/aislamiento.sh`: **14 pasan, 0 fallan**.
- `validacion/e2e-v05.sh`: **67 pasan, 7 fallan** (2 del gate, 3 de `--en-sitio`, 2 de métricas).

## 5. ¿Vale la pena seguir?

**Sí el núcleo; no el conjunto tal como está.** La idea central es correcta y poco común: el veredicto lo da un programa, no el agente. Pero hoy el programa se deja engañar con un comando, y el 70 % del código es andamiaje de orquestación que no aporta a esa garantía.

**Conservar:** `sandbox`, y de `slice`: abrir en rojo, huella, contrato en git, examen en copia limpia, recibo, ingesta por bundle, cierre reanudable.

**Arreglar antes que nada:**
1. Que el gate lea la marca del clon.
2. Que cambiar examen, alcance o huella exija algo que el worker no pueda producir (hay un `~/.config/orq/owner.token` en la máquina; el código no lo usa).
3. Lanzar al worker dentro de la caja. Sin eso, `refs/orq/` no protege nada.
4. Dejar los tests en verde.

**Cortar o congelar:** memoria, instintos, escriba, perfil, métricas, relevos, el adaptador de Orca y el gate como "frontera". Dejar el gate en 5-6 reglas claras y decir que es un aviso.

**Alternativa más simple con casi todo el valor:** rama protegida + CODEOWNERS sobre `tests/aceptacion/` + un check obligatorio de CI que corre el test desde el commit base. Agentes en worktrees nativos de Claude Code o en contenedores, con el sandbox propio de Claude Code o Codex. Localmente, un script de ~200 líneas: `git diff --quiet <rojo>..HEAD -- <examen>` y el test en `bwrap --unshare-net`. Se pierde el flujo con roles; se gana una frontera que el agente no puede cruzar porque vive en otro sistema.

## 6. Lo bueno, que no debe perderse

- **La caja** (`sandbox/sandbox.go`): raíz por lista permitida, `$HOME` en tmpfs, sin red, entorno limpio, autoprueba antes de usarla. Es la parte más sólida.
- **`leerSeguro`** (`slice/caja.go:164`): lectura con `O_NOFOLLOW` paso a paso de archivos que escribió código no confiable.
- **Ingesta por bundle** con `fsck` y comprobación de que la punta desciende del rojo (`clones.go:142-205`).
- **Contrato y recibo en git**, comparados en cada camino que acepta trabajo. Bloqueó la forja de estado en `aislamiento.sh`.
- **Cierre reanudable** con fallos inyectados en cada etapa (`ORQ_FALLA_EN`): probado y funciona.
- **Mensajes de error** que dicen la causa y el comando siguiente.
- **Veredicto único** (`verif.go:18`) usado igual al verificar y al integrar.
- El mensaje del commit WIP dice la verdad ("tests not run").

## Lo que no revisé

Los prompts de roles (`skills/`, `cli/guias/`), el detalle de `canal`, `memoria`, `plan`, `perfil` y los adaptadores de terminal. No corrí `validar-mvp.sh --real` ni nada contra herdr u Orca. La cifra "50+ slices en AIRI" no es comprobable desde aquí.
