# Qué hace el harness `orq` y qué número controla cada cosa

Para el Owner. Los números viven en `~/.config/orq/limits.json`; cambiarlos no requiere tocar código. Si el archivo falta o tiene un campo mal escrito, `orq` bloquea lanzamientos hasta que se arregle: prefiere parar a adivinar.

| Qué protege | Campo | Valor inicial | Por qué |
|---|---|---|---|
| Disco de Linux (`/`) | `min_free_gb["/"]` | 60 GB | Margen para un worktree con dependencias |
| Disco de Windows (`/mnt/c`) | `min_free_gb["/mnt/c"]` | 40 GB | Es el disco físico; el incidente de octubre fue cuando llegó a 0 |
| Workers simultáneos | `max_workers` | 4 | Hasta 4 workers en paralelo (Claude y Codex); el Orquestador usa solo los necesarios |
| Directorios desechables | `scratch.paths` | `orq/scratch`, `worktrees` (agrega los tuyos en tu limits.json local) | Lo desechable que crece sin control |
| Cuota de scratch | `scratch.quota_gb` | 200 GB hoy; bajarla tras limpiar `orq/` | Pasada la cuota, `orq watch` escribe HALT y nada se lanza |
| Disco crítico | `scratch.critical_free_gb` | 30 GB | Por debajo, HALT inmediato |
| Antigüedad de la medición | `state_max_age_hours` | 24 h | Sin medición fresca no se lanza |
| Archivos que el worker no toca | `protected` | `tests/aceptacion/`, `SLICES.md` | El test es la spec; el ledger es la memoria |

## Qué cuenta como worker

Un proceso `claude` o `codex` de primer nivel cuyo directorio es un worktree enlazado de git. Tus sesiones abiertas en el checkout principal o en tu carpeta personal no cuentan. Un worker tampoco puede lanzar otro: él mismo ya ocupa el cupo.

## Qué cuenta como lanzamiento

Crear un worker, un panel o una copia de trabajo desde herdr, desde Orca (`orca` y `orca-ide`), con `codex` o con `claude -p`. Solo esos comandos pasan por las comprobaciones de disco, HALT, medición y tope de workers. Leer o listar, como `orca status` o `herdr agent list`, pasa sin trabas. Desde el 23-oct-2026 el guardia reconoce Orca; antes solo veía herdr y la línea de comandos.

## Qué pasa en cada caso

- **Lanzar con disco bajo, HALT o medición vieja:** bloqueado, con la causa y el comando para resolverla.
- **El worker edita `tests/aceptacion/`** con Edit, Write, redirección, `tee`, `sed -i`, `cp`, `mv`, `rm` o `git checkout`/`restore`: bloqueado. Correr el test, aunque sea con `2>&1`, está permitido.
- **El worker escribe el test por una vía que el gate no reconoce**, por ejemplo un script de Python: lo atrapa el paso Verificar, porque el `git diff` del test debe salir vacío.
- **El worker cambia el contrato de aceptación** con `orq slice examen` o `orq slice alcance`: bloqueado en el gate y también en el propio comando (desde un worktree enlazado falla aunque falte el hook). Solo el Orquestador lo cambia, con `--aprobado` empezando por el id de una decisión registrada (`mNN` de memoria con fuente owner/atena, o `cNN` de un mensaje de atena); cambiar el examen anula el recibo previo y obliga a verificar de nuevo.
- **`orq` no está o falla:** el hook tiene `onFailure: block` y bloquea en vez de dejar pasar.

## Comandos

```
orq status          límites, disco, medición, HALT y la lista de workers vivos
orq watch           mide y escribe HALT si corresponde
orq halt --clear    quitar HALT, solo después de liberar espacio
```
