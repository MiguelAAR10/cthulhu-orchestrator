# Revisión independiente · arreglos 2 del step 1 (b919885..f77463f)

`cd harness && go test ./internal/gate ./internal/slice ./internal/cli`: los tres ok. No edité código.
Usé tests temporales en `internal/slice` y `internal/gate` y después los borré (`git status` limpio).

## Ítems
1. **Base confiable: parcial.** `Base` ya no sale del estado (`contrato.go:237`) y se niegan `slice/*` y `Rama` (`contrato.go:402`). Pero quedan dos huecos:
   - **A (confirmado):** `c.Base` se lee de `<root>/.orq/config.json`, el mismo `.orq/` que el estado. Si alguien puede forjar el estado, también puede forjar ese archivo.
     Con `git branch x <falso>` desde el worktree, `config.json`={"base":"x"} y el estado con base=x, sha_examen=falso y su huella, `Convertir` da nil y `Verificar` pasa con un examen `exit 0`.
   - **B (confirmado, sin tocar config):** `git update-ref refs/heads/main <falso>` desde el worktree. El gate lo permite (lo comprobé con `Evaluate`, allow=true), y el ancestro de `main` pasa.
     El nombre de la base es confiable, pero su punta la mueve un worker.
   - Arreglo: la base no puede venir de `.orq/` (usar la config del usuario o una ref fijada por el Owner). El gate debe negar a los workers `update-ref` y `branch -f/-M/-C` sobre ramas que no sean su `slice/*`.
     Además, mostrar el SHA de la punta de la base en la corrida en seco y meterlo en el `code`, para que quien ratifica lo compare con su propio `main`.
2. **Corrida en seco + código: arreglado.** Sin `--ratifico` muestra cmd, bundle, rojo, examen, base y code, y no sella (`contrato.go:259`). El code se recalcula al sellar (`contrato.go:267`) y la CLI exige los dos flags juntos (`proyecto.go:584`).
   Salvedad: el code es un sha256 sin secreto. Ata el contenido, no a la persona, así que un worker también puede calcularlo. Ya se sabe (paso 3 del roadmap).
3. **`soltar`: arreglado.** Hay candado, comprobación de uso y diario antes de guardar (`contrato.go:282`), y es solo del orquestador (`reglas.go:163`). Hereda la debilidad del ítem 4.
4. **Chequeo de uso contra git: parcial.** Busca el worktree por la línea `branch refs/heads/<Rama>` (`contrato.go:322`).
   - **C (confirmado):** con un proceso vivo en el worktree, `git checkout --detach` dentro de él y `worktree` forjado en el estado, `Convertir` da nil. Un worktree desacoplado no tiene línea `branch`.
     Lo mismo pasa con un clon y un `worktree` forjado.
   - Arreglo: comprobar todos los worktrees de `git worktree list` que no sean `r.Root`, o los que cuelgan de `.git/worktrees/`, en vez de filtrar por rama. Para clones, buscar la marca `orq-clon`.

## Defectos nuevos
- `codigoConversion` (`contrato.go:339`) cubre el nombre de la base, pero no su SHA ni `Rama`/`NN`. El ítem 1 B no cambia el code mostrado.
- Comentario de `Convertir`: `ErrEnSeco` quedó metido entre el doc y la función, así que godoc ya no documenta `Convertir`. Es menor.

VEREDICTO: corregir — ítem 1: la base no puede salir de `.orq/config.json`, y el gate debe impedir que un worker mueva `main`/la base (`update-ref`, `branch -f`); ítem 4: el chequeo de uso no puede depender de la línea `branch` (worktree desacoplado).
