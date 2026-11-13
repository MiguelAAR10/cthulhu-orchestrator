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

Crear un worker, un panel o una copia de trabajo desde herdr, desde Orca (`orca` y `orca-ide`), con `codex` o con `claude -p`. Solo esos comandos pasan por las comprobaciones de disco, HALT, medición y tope de workers. Leer o listar, como `orca status` o `herdr agent list`, pasa sin trabas. Desde el 23-oct-2026 el guardia reconoce Orca; antes solo veía herdr y la línea de comandos. Desde un worktree no se lanza nada: ni esos comandos ni la herramienta Agent, aunque el worker no sea un subagente.

## Qué pasa en cada caso

- **Lanzar con disco bajo, HALT o medición vieja:** bloqueado, con la causa y el comando para resolverla.
- **Cómo lee el guardia un comando:** como bash. Parte el comando en segmentos (`;`, `&&`, `||`, `|`, `&`, saltos de línea, subshells), quita comillas y escapes, y abre lo que corre otro comando: `sh -c`, `bash -c`, `zsh -c`, `eval`, `xargs`, `find -exec`, `env`, `sudo`, `nohup`, `time`, `command`, `npx`, `$(...)` y heredocs. Las reglas miran la palabra de comando: `grep "git push"` o un mensaje de commit que nombra `git push` pasan.
- **El worker edita `tests/aceptacion/`** con Edit, Write, redirección, `tee`, `sed -i`, `cp` (también `cp -t`), `mv`, `install -t`, `rm`, `find -delete` o `git checkout`/`restore`: bloqueado. Correr o leer el test, aunque sea con `2>&1`, está permitido.
- **El worker escribe lo que no se puede ver de antemano:** `git apply`, `git am`, `patch` y `git stash pop|apply` se bloquean en un worktree con examen protegido. Código en línea (`python -c`, `node --eval`, `-e`, `-p`, `-r`, heredoc por stdin) que nombra una ruta protegida, también.
- **El worker usa una expansión sin resolver** (`$var`, `${...}`, `$(...)`, comillas invertidas, argumentos que llegan por `xargs`) como palabra de comando o como destino de escritura: bloqueado dentro de un worktree, con el aviso de escribir la ruta o el comando literal. Fuera de un worktree no cambia nada.
- **Archivos del harness:** desde un worktree, o siendo subagente, nadie escribe en el `.orq/` del checkout principal, en el directorio de estado (`ORQ_STATE`), en `limits.json` (`ORQ_LIMITS`) ni en ningún `.claude/settings.json` o `.claude/settings.local.json`. Invocar `orq` sí está permitido.
- **Herramientas MCP** (`mcp__*`, también en el matcher del hook): desde un worktree se bloquean si algún texto de su entrada es una ruta protegida.
- **El worker escribe el test por una vía que el gate no reconoce**, por ejemplo un script de Python en un archivo: lo atrapa el paso Verificar, porque el `git diff` del test debe salir vacío.
- **`orq` no está o falla:** el hook tiene `onFailure: block` y bloquea en vez de dejar pasar.

## Comandos

```
orq status          límites, disco, medición, HALT y la lista de workers vivos
orq watch           mide y escribe HALT si corresponde
orq halt --clear    quitar HALT, solo después de liberar espacio
```
