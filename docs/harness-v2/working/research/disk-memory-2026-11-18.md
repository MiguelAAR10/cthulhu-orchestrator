# Informe: disco y memoria de los worktrees paralelos (solo lectura)

Alcance: `/home/miguel/worktrees/*` = 94 entradas (93 dirs + `s08-api.log`). Nada se tocó.
Disco: 1007 GB, 492 GB usados, 464 GB libres. RAM: 22 GB, 14,6 GB usados, 162 MB libres.

## 1. Tamaño
- Suma aparente por dir: **97,4 GB**. Medido de una sola vez (`du` cuenta hardlinks una vez): **59,2 GB reales**.
  Los ~38 GB de diferencia ya los comparte el store de pnpm (ver 2).
- Por categoría (aparente, incluye `apps/*/`, profundidad 4):
  - `node_modules`: **52,5 GB** (en ~47 dirs; 40,8 GB son de primer nivel)
  - `.next`/`dist`/`.turbo`: **20,5 GB** (mayores: metricas-superadmin 3,4; ventia-metricas-m24-entrega 3,2; airiflow-port-prod 1,1)
  - `.venv`: **3,7 GB** (airiflow-* ~0,25 c/u; odontoflow-s62-vertex-smoke 2,7 con su node_modules/venv anidado)
  - `.run` (corridas odontoflow): **7,3 GB en 6 dirs** (s62-vertex-smoke 4,4; s63-sim-stack 2,9; 3 corridas de 1,47 GB c/u)
  - `.git` / gitdir de worktree: **~0,1 GB** (los worktrees comparten objetos; no pesan)
  - Código, docs y evidencias: el resto, ~15 GB (`apps/` sin deps es grande en metricas-superadmin y airiflow-port-prod)
- Los 10 dirs más grandes: 5,4 / 4,4 / 3,8 / 3,4 / 3,3 / 2,9 / 2,8 / 2,6 / 2,5 / 2,4 GB.

## 2. Gestor de paquetes
- Lockfiles: 52 dirs con `pnpm-lock.yaml`, 64 con `package-lock.json` (conviven en muchos), 8 con `uv.lock` (odontoflow-backend). Sin yarn, bun ni go.
- 50 `node_modules` con layout pnpm (`.pnpm`) y 75 planos (npm).
- pnpm usa **store con hardlinks**: archivos de `ventia-entrega-base/node_modules/.pnpm/...` tienen 52 enlaces (`stat -c %h`).
  Store: `~/.local/share/pnpm/store/v11` = 5,0 GB. Por eso una réplica pnpm cuesta poco disco real.
- npm **copia**: `~/.npm` = 6,2 GB de caché, y cada `node_modules` plano es una copia completa.
- uv: caché `~/.cache/uv` = 19 GB.

## 3. Estado git (83 con repo; 12 sin `.git`)
- Repos de origen: ventia-resumidor-v2 (21), ventia-monorepo (21), airiflow-erp (21), ventia-metricas (10), odontoflow-backend (8), odontoflow-frontend (1), cthulhu-orchestrator (1).
- **Limpios: 55. Con cambios sin commit: 28.** Sin rama (HEAD suelto): 15.
- Limpios **y** ya fusionados a main/master/develop: **solo 1** (ningún `--merged` útil). Casi todas las ramas llevan commits propios (de 74 hasta 3054 por delante de la base).
  Posible causa: fusiones squash/PR que `merge-base --is-ancestor` no detecta. No lo pude confirmar.
- Limpios con HEAD ya en un remoto: **7** (airiflow-prod-base, centro-anuncios, metricas-superadmin, meta-ads-renzo, ux-ultimate-lectura, ventia-m24-costos-sin-daily, ventia-metricas-m24-entrega).
- Limpios sin push: 48. Con cambios y sin push: 18. Con cambios y con push: 10.
- Sin `.git` (12): airiflow-owner-fe, cam--e2e, cam--rails, vm--b3, vm--demo, vm--e2e, ventia-resumidor-v-s79, v-s79-s79-final, v-s79-s79-suite-messaging, v-s111-corridas, v-s111-s111-corridas, s08-api.log. Son carpetas sueltas o worktrees rotos: no se puede decir si hay trabajo único.

## 4. Procesos con cwd en estos dirs
- Solo **16 procesos, 715 MB RSS**, casi todos en `cthulhu-build` (claude 394 MB, playwright-mcp ~190 MB, uvx notebooklm ~120 MB) y 2 `zsh` en airiflow-fix-b.
- La RAM grande NO está en `/home/miguel/worktrees`: `next-server` (v16) **7,2 GB** en `~/.herdr/worktrees/ventia-monorepo/...` (4,6 GB en feat-whatsapp-bloqueo-contactos, 2,6 GB en csv-fe). Además 3,6 GB en procesos `claude`, ~0,9 GB postgres.
- Con 162 MB libres y 14,6 GB usados, el cuello de memoria son esos dev servers Next, no los worktrees.

## 5. Instalar de cero vs reusar (estimado, sin correr nada)
- **Node (ventia-entrega-base, pnpm)**: `pnpm-lock.yaml` 15 448 líneas, ~1417 paquetes; `node_modules` 1,63 GB aparente.
  - Instalación nueva con store caliente (5 GB en disco): hardlinks + symlinks, **~20–60 s y casi 0 GB nuevos reales** (solo inodos y enlaces).
  - Con store frío: descarga ~1,5 GB, **~2–4 min**, +1,6 GB.
  - Reusar `node_modules` del checkout principal (`ventia-resumidor-v2`, 1,63 GB) en solo lectura: 0 GB y 0 s, pero los symlinks `.pnpm` apuntan a rutas absolutas del checkout y las deps de `apps/*` no se comparten; riesgo si una rama cambia el lockfile.
- **Python (odontoflow-backend, uv)**: `uv.lock` ~2500 líneas, ~90 paquetes. El `.venv` en s01/s62 pesa 4 KB (casi vacío), así que no están instalados ahí.
  - Instalar con `uv sync` y caché caliente (19 GB): **~5–15 s**, los wheels se enlazan por hardlink/clon. Con caché fría: ~1–2 min.
  - Reusar el `.venv` de otro checkout: posible solo si el lockfile es idéntico (`diff uv.lock`).
- Son estimaciones por tamaño de lockfile y de store, no cronometradas.

## Top 3 ahorros
1. **`node_modules` de los 55 worktrees limpios: ~29,8 GB aparentes** (24,6 GB en limpios sin push, 5,1 GB en limpios con push). Se regeneran con el lockfile. El real liberado es menor en los pnpm por los hardlinks (no medido).
2. **Builds `.next`/`dist`/`.turbo`: ~20,5 GB aparentes**, 100 % regenerables; solo importan si hay un dev server vivo en ese dir.
3. **Pasar los 75 `node_modules` npm planos a pnpm con el store** (o compartir): el estado 59,2 GB reales ya demuestra el efecto; cada copia npm repite ~1,6 GB. Alternativa: `.run` de odontoflow, 7,3 GB, **riesgosa** (son evidencia de corridas; revisar antes).

## Seguro vs riesgoso
- **Seguro**: borrar `node_modules`, `.next`, `dist`, `.turbo` de worktrees limpios (reinstalar y reconstruir). No toca código ni commits.
- **Con cuidado**: quitar un worktree limpio entero. Solo 7 tienen HEAD en remoto; los otros 48 pueden tener commits que viven solo aquí.
- **No tocar sin revisar**: los 28 con cambios sin commit, los 12 sin `.git`, y `.run` (evidencia).
- Antes de borrar `node_modules` en un dir, comprobar que no haya `next-server`/`node` corriendo ahí (ahora ninguno en el alcance).

## Lo que no pude medir
- Ahorro real exacto por dir: `du` por dir cuenta los hardlinks completos; el único dato real es el total (59,2 GB).
- Si las ramas están fusionadas por squash/PR; no hay API de GitHub ni `gh` consultada.
- `~/.herdr/worktrees` (otro grupo de worktrees con dev servers) está fuera de alcance: no medí su tamaño.
- Tiempos de instalación: solo estimados, sin ejecutar nada.
- No clasifiqué contenido de proyecto ni evidencias; solo rutas y estados.
