# Revisión independiente · arreglos del step 1 (b1a1d4a..4ceba77)

`cd harness && go test ./internal/gate ./internal/slice`: ok, los dos paquetes (slice tardó 22 s). No edité código.
Usé un test temporal en `internal/slice` para confirmar el bypass del ítem 3 y después lo borré (`git status` limpio).

## Must-fix
1. **IsWorker con repos anidados: arreglado.** `gate.go:225` sigue subiendo después de un `.git` sin marca.
   Cuenta como worker si cualquier ancestro tiene un `.git` archivo o `.git/orq-clon`. `LeerMarca` (`clones.go:38`) hace lo mismo.
   El test es `worker_test.go:120`, con `git init sub` y `git clone . sub`.
2. **Sin adopción automática: arreglado.** `nucleo.go:319` llama a `confiable`, que devuelve `ErrSinContrato`, y `adoptarSiLegado` ya no existe.
   Ahora `--ratifico` es obligatorio (`proyecto.go:579`). Queda una salvedad, que el uso ya declara: la ratificación se guarda pero no se autentica.
3. **Anclaje de `Convertir`: parcial.** El SHA completo, `^{commit}` y `Corto` están bien (`contrato.go:260-296`), y el pánico de C está resuelto.
   Pero la ascendencia se comprueba contra `refs/heads/`+`e.Base` (`contrato.go:269`), y `e.Base` sale del mismo estado falsificable.
   - **Bypass (confirmado con un test):** un slice 1.0.0 en worktree enlazado. La rama `slice/01` vive en `refs/heads` del repo principal.
     El worker hace un commit con `tests/s01.sh` = `exit 0`. Luego reescribe el estado: `base`=`slice/01`, `sha_examen`=ese commit y una `huella` recalculada con `HuellaEnCommit`.
     Resultado: `Convertir` devuelve nil, `Verificar` da nil y se emite un recibo. Es el escenario A, con un solo campo más.
     Tampoco hace falta `Base`=`Rama`: basta `git branch x <commit>` desde el worktree (los refs son compartidos, I3).
   - Arreglo: no leer `Base` del estado. Tomarlo de la config o rama por defecto del proyecto (o del `.orq` del dueño) y exigir que sea igual.
     Rechazar también `Base == Rama` o cualquier `slice/*`.
   - Mostrar cmd/bundle no sirve para ratificar. Se imprimen en la misma corrida que sella (`contrato.go:245`), y `--ratifico` ya llegó antes.
     Quien ratifica no ve el `cmd` antes de aprobar. Hace falta una corrida en seco (sin `--ratifico`, que muestre y no selle) o una confirmación.
4. **El test verifica de verdad: arreglado.** `legado_test.go:260` exige un recibo y que haya una sola línea en recibos.
   Lo aprueba con `Ampliar(".gitattributes")` antes de correr.

## Hallazgos 4–5
- **H4, procesos detenidos: parcial.** Se comprueba el candado, el lease `Escritor` y `EnUso(e.Worktree)` (`contrato.go:223-226`).
  Pero `escritor` y `worktree` vienen del estado. Un worker vivo que pone `escritor:""` y `worktree:"/tmp/x"` pasa los dos chequeos.
  Es menor si el ítem 3 se cierra y la ratificación es humana.
- **H5, cwd vacío o relativo: arreglado.** `IsWorker` y `LeerMarca` tratan un cwd no absoluto como worker. `CheckoutPrincipal` (`reglas.go:467`) devuelve "".
  Los tres paran cuando `Dir(d)==d`. El test es `worker_test.go:165`.

## Defectos nuevos
- Ningún comando libera `Escritor` (lo admite el informe). Un slice 1.0.0 que todavía nombra a un escritor solo se convierte editando el JSON a mano.
  Eso enseña justo la práctica que el contrato quiere eliminar. Hace falta una salida legítima, por ejemplo `tomar --relevo` o `soltar`.
- Un `sha_examen` de mitad de slice no está en `Base`, así que se rechaza siempre (lo admite el informe). Es aceptable si el Owner lo sabe.
- Ya no queda ningún `[:7]`/`[:8]` sobre SHAs fuera de los tests. No vi regresiones en el gate.

VEREDICTO: corregir — ítem 3: anclar `Base` a una fuente que no sea el estado (y negar `slice/*`); el ratificador ve cmd/bundle antes de sellar.
