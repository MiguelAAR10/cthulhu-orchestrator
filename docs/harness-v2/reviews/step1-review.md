# Revisión independiente · step 1 (e583e78^..6b04c1d, harness/ validacion/ Makefile)

Rango corregido por el usuario: incluye e583e78 (detección de clones en el gate).
`cd harness && go test ./...` corrió en mi sandbox: 21 paquetes ok (bwrap presente). No corrí `make ci` completo.
No edité código. Usé un test temporal para confirmar el hallazgo 1 y lo borré.

## 1. [ALTA] Un repo anidado dentro del clon apaga el gate
`harness/internal/gate/gate.go:223-236` (`IsWorker`). La función se detiene en el primer `.git` que
encuentra al subir. Si es un directorio sin marca, devuelve false.
- Escenario: el worker, con cwd en el clon, corre `git init -q sub` (el gate lo deja pasar) y luego `cd sub`.
  El cwd se conserva entre llamadas porque sigue dentro del directorio de trabajo. Desde ahí
  `IsWorker(clon/sub)` da false y `wt` queda en false. Lo confirmé con un test sobre el gate real:
  `orq slice examen … --aprobado` → Allow, `cd <main> && orq slice examen` → Allow, y un Edit de
  `<clon>/tests/aceptacion/s01.sh` → Allow. `escrituraProhibida` ni siquiera corre, así que el `.orq`
  del checkout principal también queda abierto. `git clone . sub` sirve igual.
- Arreglo: seguir subiendo después de un `.git` sin marca. Si algún ancestro tiene `.git/orq-clon` o un
  `.git` archivo, el dir es de un worker. Añadir un test con un repo anidado.

## 2. [ALTA] La adopción automática confía en un campo del estado falsificable (degradación de esquema)
`harness/internal/slice/contrato.go:236-241` (`adoptarSiLegado`) y `:210` (`Convertir`). La única
señal de "esto es 1.0.0" es `e.Esquema` y que falte `refs/orq/contrato/NN`. Las dos cosas las controla
quien escribe el estado y las refs.
- Escenario: un slice actual. Un worker borra el contrato (`git -C <main> update-ref -d
  refs/orq/contrato/01`, el gate no lo bloquea; el informe lo admite) y reescribe el JSON con un script
  de intérprete: `esquema` 0, `cmd` débil y una `huella` que cuadra. Al siguiente `verificar` el slice se
  adopta solo y el contrato nuevo sella el estado falsificado. Así el contrato no protege ni a los slices
  nuevos, que son justo los que existe para proteger.
- Arreglo: `verificar` no adopta nunca y devuelve `ErrSinContrato`. Solo `convertir` adopta, y como mínimo
  exige `--ratifico` como pide el spec (plan.md, Design).

## 3. [ALTA] `Convertir` no ancla el rojo ni el examen a nada confiable
`contrato.go:213` usa `e.baseExamen()`, `e.Bundle` y `e.Cmd`, todos leídos del estado. El chequeo de
descendencia (`clones.go:224`) usa `e.SHARojo`, que también viene del estado.
- Escenario A: un estado 1.0.0 con `sha_examen` apuntando a un commit del propio worker, donde
  `tests/s01.sh` es `exit 0`, y la `huella` calculada sobre ese commit. La huella cuadra, la punta
  desciende del rojo real y se adopta un examen trivial.
- Escenario B: `sha_rojo`/`sha_examen` = `slice/01` o `HEAD`. Nada resuelve el valor a un SHA completo.
  `git show slice/01:…` y `merge-base --is-ancestor slice/01 …` funcionan, y el contrato guarda un nombre
  de ref que se mueve. Entonces "lo que cambió desde el rojo" queda vacío para siempre.
- Escenario C: `sha_rojo` de menos de 7 caracteres. `proyecto.go:588` (y `:603`) hace pánico con
  `e.SHARojo[:7]`, después de que `Convertir` ya selló el contrato.
- Arreglo: resolver con `rev-parse --verify <x>^{commit}` y exigir un hex de 40. Exigir que SHARojo y
  SHAExamen sean ancestros de `Base` en main, no del branch del worker. Mostrar cmd/bundle para ratificar.

## 4. [MEDIA] "Procesos detenidos" no se prueba
`nucleo.go:77-90` (`candadoLibre`). Las órdenes de orq toman el flock de `slice-NN` solo mientras corren.
Un worker 1.0.0 vivo entre dos órdenes no tiene el candado, así que la adopción pasa con él corriendo.
El informe lo reconoce. Lo listo porque el mensaje y el uso dicen "must be stopped" y el código no lo
comprueba. Arreglo: mirar el lease `Escritor` y los procesos con cwd en `e.Worktree` (`sysinfo.Workers`).

## 5. [BAJA] `IsWorker`/`LeerMarca` entran en un bucle infinito con un cwd vacío o relativo
`gate.go:224`, `clones.go:37`: `filepath.Dir(".") == "."`. Con `cwd` "" o relativo y sin `.git` en el
cwd del proceso, el hook se cuelga. Claude Code trata un hook colgado o con timeout como error que no
bloquea, así que el gate falla abierto. Arreglo: si `cwd` no es absoluto → tratarlo como worker (o
`filepath.Abs`) y parar cuando `Dir(d) == d`.

## Tests que no prueban lo que dice su nombre
- `slice_test.go:1342` `TestEstadoViejoNoCorreGitEnElWorktree` ignora el resultado de `r.Verificar`.
  Si verify falla antes de cualquier git (adopción rechazada, error de punta), el centinela no aparece y
  el test pasa sin probar nada. Hay que exigir que verify llegue al examen (recibo emitido).
- `gate/worker_test.go` y `decisiones_test.go` cubren el cwd en la raíz del clon o en un subdirectorio,
  pero ningún caso tiene un `.git` intermedio. Por eso el hallazgo 1 quedó verde.
- `TestConvertirEstadoViejo` solo falsifica `huella` sola. No hay un caso con `huella`+`sha_examen`
  coherentes (hallazgo 3), que es el ataque real que el nombre "hash checked" sugiere cubrir.

## `make ci` y scripts
No encontré una ruta a verde con una suite sin correr:
- `set -e` cubre los subshells.
- `e2e-v05.sh` termina en `[ $falla -eq 0 ]`.
- `aislamiento.sh` sale con 1 sin bwrap y termina en `[[ $FAIL -eq 0 ]]`.
- `ORQ_STATE=`/`ORQ_LIMITS=` vacíos caen en los defaults de `$BASE` por el `:-`.
Única observación: `go test` acepta `t.Skip` en silencio. Hoy solo `TestPrepararDeps` (declarado) y el
test de herdr usan Skip.

VEREDICTO: corregir
Debe corregirse antes de integrar:
1. `IsWorker` debe ver la marca a través de repos anidados (hallazgo 1), con su test.
2. Sin adopción automática en `verificar`: solo `convertir`, con ratificación (hallazgo 2).
3. `Convertir`: resolver y validar SHAs completos, anclar rojo/examen a `Base` y no hacer pánico con un SHA
   corto (hallazgo 3).
4. `TestEstadoViejoNoCorreGitEnElWorktree` debe comprobar que verify llegó a ejecutarse.
