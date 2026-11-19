#!/usr/bin/env bash
# Prueba de punta a punta de orq con un repo de juguete:
# rojo con conteo → trampa bloqueada → reintento → verde con recibo → cierre viejo rechazado →
# relevo sin duplicar → cierre interrumpido de verdad en cada etapa y retomado →
# conteo obligatorio, rutas fix/slice, runner vigilado, examen siempre en copia aislada (sin --en-sitio),
# el gate dentro del clon del worker y un slice de orq 1.0.0 rechazado.
# Uso: ORQ_CANDIDATO=/ruta/al/orq validacion/e2e-v05.sh [carpeta-temporal] (make ci lo hace). No toca nada fuera de esa carpeta.
set -u
GUARDA="$(cd "$(dirname "$0")" && pwd)/candidato.sh"
BASE="${1:-$(mktemp -d)}"
mkdir -p "$BASE" && cd "$BASE" || exit 1
export ORQ_WORKTREES="$BASE/wt" ORQ_ROL="" HERDR_ENV="" HERDR_PANE_ID=""
# Estado, límites y configuración propios: nunca los reales del usuario.
export ORQ_STATE="${ORQ_STATE:-$BASE/state}" ORQ_LIMITS="${ORQ_LIMITS:-$BASE/limits.json}" ORQ_CONFIG="$BASE/config.json"
mkdir -p "$ORQ_STATE" "$BASE/scratch"
# La máquina nunca limita aquí: la prueba no depende de la RAM ni de la carga de quien la corre.
echo '{"recursos":{"ram_por_agente_gb":0.01,"reserva_ram_gb":0,"cpu_por_agente":0.01,"reserva_cpus":0,"carga_max":100000}}' > "$ORQ_CONFIG"
[ -f "$ORQ_LIMITS" ] || cat > "$ORQ_LIMITS" <<EOF
{"min_free_gb":{"/":1},"max_workers":99,"scratch":{"paths":["$BASE/scratch"],"quota_gb":1,"critical_free_gb":1},"state_max_age_hours":24,"protected":["tests/aceptacion/","SLICES.md"]}
EOF
mkdir -p "$BASE/bin" && printf '#!/bin/sh\nexit 1\n' > "$BASE/bin/claude" && chmod +x "$BASE/bin/claude"
export PATH="$BASE/bin:$PATH"   # el escriba no responde: todo debe funcionar igual
. "$GUARDA"; candidato "$(command -v orq)" || exit 1
pasa=0; falla=0
ok() { if [ "$1" = "$2" ]; then pasa=$((pasa+1)); echo "  ✓ $3"; else falla=$((falla+1)); echo "  ✗ $3 (esperaba $2, salió $1)"; fi; }
q() { orq "$@" >/tmp/orq-e2e.$$ 2>&1; echo $?; }

git init -q -b main tienda && cd tienda && git config user.email t@t && git config user.name t
examen() { # examen NN: TAP; pasa cuando vNN.txt dice ok
  mkdir -p tests && cat > "tests/s$1.sh" <<EOT
if grep -qx ok v$1.txt 2>/dev/null; then echo 'ok 1 v$1' > out.tap; else echo 'not ok 1 v$1' > out.tap; exit 1; fi
EOT
  echo esperado > "tests/d$1.txt"; echo 'out.tap' > .gitignore
  git add -A && git commit -qm "test(S-$1): rojo"
}
orq init --modo=nuevo >/dev/null 2>&1; git add -A; git commit -qm "chore: orq init" >/dev/null

echo "== S-01: rojo, trampa, reintento, recibo"
examen 01
ok "$(q slice abrir 01 --titulo uno --test tests/s01.sh --cmd 'sh tests/s01.sh' --bundle tests/d01.txt --reporte out.tap --formato tap --sin-scouts e2e)" 0 "abre en rojo por aserción"
ok "$(q slice tomar w1)" 0 "w1 toma el slice"
ok "$(q slice tomar w2)" 1 "w2 no puede escribir a la vez"
W="$ORQ_WORKTREES/tienda-s01"
(cd "$W" && echo ok > v01.txt && echo trampa > tests/d01.txt && git add -A && git commit -qm trampa)
ok "$(q slice verificar)" 1 "la trampa en el fixture se detecta"
grep -q BLOQUEADO <(orq slice estado) ; ok $? 0 "queda BLOQUEADO"
ok "$(q slice reintentar 'sin tocar el fixture')" 0 "reintento con motivo → a2"
(cd "$W" && git checkout -q HEAD~1 -- tests/d01.txt && git commit -qam "restaura fixture")
orq slice tomar w1 >/dev/null
ok "$(q slice verificar)" 0 "verifica en verde"
grep -q '"ejecutados": 1' <(orq slice recibo); ok $? 0 "el recibo cuenta 1 prueba ejecutada"
ok "$(q msg send --de orquestador --para atena --tipo cierre --intento a1 'listo')" 5 "un cierre del intento viejo se rechaza"
ok "$(q msg send --de orquestador --para atena --tipo cierre --intento a2 'listo')" 0 "el cierre del intento vigente pasa"
antes=$(git worktree list | wc -l)
ok "$(q relevo --solo-archivo orquestador)" 0 "relevo escrito aunque el escriba falle"
grep -q "no crear otro" .orq/relevo-orquestador.md; ok $? 0 "el relevo dice que siga en el mismo worktree"
ok "$(git worktree list | wc -l)" "$antes" "el relevo no creó worktrees"
ok "$(q slice cerrar --aceptado)" 0 "cierra aceptado"

n=2
for etapa in integrar integrado publicado limpiando; do
  NN=$(printf %02d $n); echo "== S-$NN: interrupción real en '$etapa'"
  examen $NN
  q slice abrir $NN --titulo "s$NN" --test tests/s$NN.sh --cmd "sh tests/s$NN.sh" --reporte out.tap --formato tap --sin-scouts e2e >/dev/null
  (cd "$ORQ_WORKTREES/tienda-s$NN" && echo ok > v$NN.txt && git add -A && git commit -qm impl)
  orq slice verificar >/dev/null 2>&1
  ok "$(ORQ_FALLA_EN=$etapa q slice cerrar --aceptado)" 9 "el proceso muere en '$etapa'"
  ok "$(q slice cerrar --aceptado)" 0 "repetir el cierre termina"
  ok "$(git log --oneline main | grep -c "slice S-$NN/")" 1 "S-$NN integrado exactamente una vez"
  ok "$(cat v$NN.txt 2>/dev/null)" ok "main tiene el cambio"
  n=$((n+1))
done
echo "== dos slices a la vez, contrato y gate"
examen 10; examen 11
q slice abrir 10 --titulo diez --test tests/s10.sh --cmd 'sh tests/s10.sh' --bundle tests/d10.txt --reporte out.tap --formato tap --sin-scouts e2e >/dev/null
ok "$(q slice abrir 11 --titulo once --test tests/s11.sh --cmd 'sh tests/s11.sh' --reporte out.tap --formato tap --sin-scouts e2e)" 0 "abre un segundo slice en paralelo"
ok "$(q slice verificar)" 1 "sin --slice con dos abiertos pide cuál"
orq slice --slice 10 tomar w10 >/dev/null; orq slice --slice 11 tomar w11 >/dev/null
orq slice contrato --slice 10 --alcance "solo v10.txt" > contrato.txt 2>&1
grep -q "CONTRATO S-10/a1 · worker: w10" contrato.txt && grep -q "solo v10.txt" contrato.txt && grep -q "tests/d10.txt" contrato.txt; ok $? 0 "el contrato trae intento, alcance y paquete protegido"
W10="$ORQ_WORKTREES/tienda-s10"
gate() { printf '{"tool_name":"%s","cwd":"%s","tool_input":%s}' "$1" "$2" "$3" | orq gate >/dev/null 2>&1; echo $?; }
ok "$(gate Bash "$W10" '{"command":"orq slice cerrar --aceptado"}')" 2 "gate: un worker no cierra"
ok "$(gate Bash "$W10" '{"command":"cd tests && echo x > d10.txt"}')" 2 "gate: el worker no toca el fixture ni con cd"
ok "$(gate Bash "$W10" '{"command":"git push origin main"}')" 2 "gate: nada de push"
ok "$(gate Bash "$W10" '{"command":"echo ok > v10.txt"}')" 0 "gate: el trabajo normal pasa"
[ -f "$W10/.git/orq-clon" ]; ok $? 0 "the worker copy is an orq clone (marker in .git)"
ok "$(gate Bash "$W10" '{"command":"orq slice examen tests/s10.sh --aprobado \"req ATN-1\""}')" 2 "gate: a worker in a clone cannot approve an exam"
ok "$(gate Bash "$W10" "{\"command\":\"cd $PWD && orq slice examen tests/s10.sh --aprobado x\"}")" 2 "gate: nor by cd to the main checkout"
(cd "$W10" && echo ok > v10.txt && git add -A && git commit -qm impl10)
(cd "$ORQ_WORKTREES/tienda-s11" && echo ok > v11.txt && git add -A && git commit -qm impl11)
ok "$(q slice --slice 10 verificar)" 0 "S-10 verifica"
ok "$(q slice --slice 11 verificar)" 0 "S-11 verifica"
orq slice --slice 10 cerrar --aceptado >/tmp/orq-e2e.a.$$ 2>&1 & orq slice --slice 11 cerrar --aceptado >/tmp/orq-e2e.b.$$ 2>&1 & wait
rm -f /tmp/orq-e2e.a.$$ /tmp/orq-e2e.b.$$
ok "$(git log --oneline main | grep -c 'slice S-1[01]/')" 2 "los dos cierres simultáneos integran una vez cada uno"
ok "$(cat v10.txt v11.txt | tr -d '\n')" okok "main tiene los dos cambios"
orq metricas > met.txt; grep -q "slices aceptados e integrados: 7" met.txt; ok $? 0 "métricas: 7 aceptados"
echo "== verificación honesta: conteo, rutas, runner vigilado, copia aislada"
examen 20
ok "$(q slice abrir 20 --titulo veinte --test tests/s20.sh --cmd 'sh tests/s20.sh' --sin-scouts e2e)" 1 "abrir sin --reporte ni --sin-conteo se rechaza"
ok "$(q slice abrir 20 --titulo veinte --test tests/s20.sh --cmd 'sh tests/s20.sh' --sin-conteo 'el runner no deja reporte' --ruta fix --aislado --sin-scouts e2e)" 0 "abre un fix con --sin-conteo (y --aislado sigue aceptado)"
orq slice estado | grep -q "ruta: fix"; ok $? 0 "estado muestra la ruta fix"
orq slice estado | grep -q "sin conteo: el runner no deja reporte"; ok $? 0 "estado repite el motivo de no contar"
echo '{"slice":"20","tareas":[]}' > plan20.json
ok "$(q plan cargar plan20.json)" 1 "orq plan cargar se rechaza en un fix"
grep -q "orq slice ruta slice" /tmp/orq-e2e.$$; ok $? 0 "el rechazo apunta a la subida de ruta"
ok "$(q slice ruta slice)" 1 "subir de ruta sin motivo se rechaza"
(cd "$ORQ_WORKTREES/tienda-s20" && echo ok > v20.txt && git add -A && git commit -qm impl20)
ok "$(q slice verificar)" 0 "el fix verifica"
orq slice recibo | grep -q "el runner no deja reporte"; ok $? 0 "el recibo repite el motivo de no contar"
ok "$(q slice cerrar --aceptado)" 0 "cierra el fix"

examen 21
q slice abrir 21 --titulo veintiuno --test tests/s21.sh --cmd 'sh tests/s21.sh' --reporte out.tap --formato tap --ruta fix --sin-scouts e2e >/dev/null
ok "$(q slice ruta slice 'toca dos repos')" 0 "un fix sube a slice con motivo"
orq slice estado | grep -q "ruta: slice (subió de fix: toca dos repos)"; ok $? 0 "estado recuerda que subió"
ok "$(q slice ruta slice 'otra vez')" 1 "una ruta slice no vuelve a subir"
(cd "$ORQ_WORKTREES/tienda-s21" && echo ok > v21.txt && git add -A && git commit -qm impl21)
orq slice verificar >/dev/null 2>&1; ok "$(q slice cerrar --aceptado)" 0 "cierra el fix ascendido"

examen 22; echo 'sh tests/s22.sh' > run22.sh; git add -A; git commit -qm "runner s22"
q slice abrir 22 --titulo runner --test tests/s22.sh --cmd 'sh run22.sh' --reporte out.tap --formato tap --sin-scouts e2e >/dev/null
(cd "$ORQ_WORKTREES/tienda-s22" && echo ok > v22.txt && echo '# ajuste' >> run22.sh && git add -A && git commit -qm impl22)
ok "$(q slice verificar)" 3 "un runner cambiado sale con 3 sin correr"
grep -q "run22.sh" /tmp/orq-e2e.$$; ok $? 0 "el mensaje lista el archivo cambiado"
orq slice estado | grep -q "fallos: 0/"; ok $? 0 "no cuenta como reparación fallida"
ok "$(q slice verificar --acepto-runner 'Atena aprobó el ajuste')" 0 "con --acepto-runner verifica"
orq slice recibo | grep -q "Atena aprobó el ajuste"; ok $? 0 "el recibo deja escrito el motivo"
ok "$(q slice cerrar --aceptado)" 0 "cierra con runner aceptado"

ruido() { # ruido NN: pasa con vNN.txt = ok o con un archivo ignorado ruidoNN.txt
  mkdir -p tests && cat > "tests/s$1.sh" <<EOT
if grep -qx ok v$1.txt 2>/dev/null || [ -f ruido$1.txt ]; then echo 'ok 1 v$1' > out.tap; else echo 'not ok 1 v$1' > out.tap; exit 1; fi
EOT
  echo "ruido$1.txt" >> .git/info/exclude; git add -A && git commit -qm "test(S-$1): rojo"
}
ruido 23
q slice abrir 23 --titulo aislado --test tests/s23.sh --cmd 'sh tests/s23.sh' --reporte out.tap --formato tap --sin-scouts e2e >/dev/null
W23="$ORQ_WORKTREES/tienda-s23"; echo x > "$W23/ruido23.txt"   # archivo ignorado: el worktree sigue limpio
ok "$(q slice verificar)" 1 "un archivo ignorado del worker no hace pasar el examen (copia aislada)"
(cd "$W23" && echo ok > v23.txt && git add -A && git commit -qm impl23)
ok "$(q slice verificar)" 0 "el trabajo real sí verifica"
ok "$(q slice cerrar --aceptado)" 0 "cierra el slice aislado"
ruido 24
ok "$(q slice abrir 24 --titulo ensitio --test tests/s24.sh --cmd 'sh tests/s24.sh' --reporte out.tap --formato tap --en-sitio 'el examen lee un artefacto ignorado' --sin-scouts e2e)" 1 "--en-sitio is refused: the exam always runs in the box"
grep -q -- "--en-sitio ya no existe" /tmp/orq-e2e.$$; ok $? 0 "the refusal says why"
ok "$(git branch --list slice/24 | wc -l)" 0 "the refused open leaves no slice branch"
orq metricas > met2.txt
grep -q "slices aceptados e integrados: 11" met2.txt; ok $? 0 "métricas: 11 aceptados"
grep -q "ruta fix: 1 aceptados" met2.txt; ok $? 0 "métricas: 1 fix aceptado"
grep -q "ruta slice: 10 aceptados" met2.txt; ok $? 0 "métricas: 10 en la ruta slice"
grep -q "fixes que subieron a slice: 1" met2.txt; ok $? 0 "métricas: 1 fix subió a slice"
echo "== S-30: a slice opened by orq 1.0.0 is not inherited"
mkdir -p .orq/slices
printf '{"nn":"30","intento":1,"fase":"VERIFICADO","test":"tests/s30.sh","cmd":"sh tests/s30.sh","base":"main","rama":"slice/30","worktree":"%s"}' "$ORQ_WORKTREES/tienda-s30" > .orq/slices/30.json
msg30="S-30 was opened by orq 1.0.0; close it with orq 1.0.0 before using this version"
for sub in verificar juntar "cerrar --aceptado" "cerrar --rechazado x"; do
  ok "$(q slice --slice 30 $sub)" 1 "1.0.0 slice: $sub refuses"
  grep -qF "$msg30" /tmp/orq-e2e.$$; ok $? 0 "1.0.0 slice: $sub says to close it with orq 1.0.0"
done
ok "$(git for-each-ref refs/orq/contrato/30 | wc -l)" 0 "1.0.0 slice: no contract written"
rm .orq/slices/30.json
echo "== higiene"
orq doctor >/tmp/orq-e2e.$$ 2>&1; d=$?
if [ $d -ne 0 ]; then cat /tmp/orq-e2e.$$; fi
ok $d 0 "doctor: sin worktrees, ramas ni cuarentena sobrantes"
ok "$(git branch --list 'slice/*' | wc -l)" 0 "sin ramas slice/*"
rm -f /tmp/orq-e2e.$$
echo; echo "Resultado: $pasa pasan, $falla fallan"; [ $falla -eq 0 ]
