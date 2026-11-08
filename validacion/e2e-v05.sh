#!/usr/bin/env bash
# Prueba de punta a punta de orq con un repo de juguete:
# rojo con conteo → trampa bloqueada → reintento → verde con recibo → cierre viejo rechazado →
# relevo sin duplicar → cierre interrumpido de verdad en cada etapa y retomado.
# Uso: validacion/e2e-v05.sh [carpeta-temporal]. No toca nada fuera de esa carpeta.
set -u
REPO="$(cd "$(dirname "$0")/.." && pwd)"
BASE="${1:-$(mktemp -d)}"
mkdir -p "$BASE" && cd "$BASE" || exit 1
export ORQ_WORKTREES="$BASE/wt" ORQ_ROL="" HERDR_ENV="" HERDR_PANE_ID=""
mkdir -p "$BASE/bin" && printf '#!/bin/sh\nexit 1\n' > "$BASE/bin/claude" && chmod +x "$BASE/bin/claude"
export PATH="$BASE/bin:$PATH"   # el escriba no responde: todo debe funcionar igual
# Siempre prueba el código de este repo, nunca un orq viejo del PATH (ORQ_BIN lo sobrescribe).
if [ -n "${ORQ_BIN:-}" ]; then cp "$ORQ_BIN" "$BASE/bin/orq"; else (cd "$REPO/harness" && go build -o "$BASE/bin/orq" ./cmd/orq) || exit 1; fi
[ "$(command -v orq)" = "$BASE/bin/orq" ] || { echo "orq no es el binario del repo"; exit 1; }
pasa=0; falla=0
ok() { if [ "$1" = "$2" ]; then pasa=$((pasa+1)); echo "  ✓ $3"; else falla=$((falla+1)); echo "  ✗ $3 (esperaba $2, salió $1)"; fi; }
q() { orq "$@" >/tmp/orq-e2e.$$ 2>&1; echo $?; }
gate0() { printf '{"tool_name":"%s","cwd":"%s","tool_input":%s}' "$1" "$2" "$3" | orq gate >/dev/null 2>&1; echo $?; }

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
echo "== S-01: QA independiente"
ok "$(q slice cerrar --aceptado)" 1 "criticidad medium: sin recibo QA no cierra"
grep -q '"qa"' .orq/config.json 2>/dev/null && { echo "config.json ya trae qa: ajusta el e2e"; exit 1; }
printf '{"sync":"local","qa":{"layers":[{"name":"unit","cmd":"sh tests/s01.sh"},{"name":"mutation","cmd":"false","min_criticality":"high"}]}}\n' > .orq/config.json
ok "$(ORQ_ROL=tester gate0 Write "$W" '{"file_path":"v01.txt"}')" 2 "gate: el tester no escribe"
ok "$(ORQ_ROL=tester gate0 Bash "$W" '{"command":"sh tests/s01.sh"}')" 0 "gate: el tester sí prueba"
ok "$(q qa finding add S-01 --requirement R1 --repro true)" 1 "hallazgo incompleto rechazado"
ok "$(q qa finding add S-01 --requirement R1 --commit x --repro true --observed o --expected e --impact i)" 0 "hallazgo con repro que pasa: no confirmado"
ok "$(q qa run S-01)" 0 "QA verde; mutation omitida en medium"
grep -q '"skipped": true' .orq/qa/S-01/receipt.json; ok $? 0 "el recibo QA registra la capa omitida"
ok "$(q slice cerrar --aceptado)" 0 "cierra aceptado"
printf '{"sync":"local","criticality":"low"}\n' > .orq/config.json   # el resto del e2e prueba el cierre, no QA

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
(cd "$W10" && echo ok > v10.txt && git add -A && git commit -qm impl10)
(cd "$ORQ_WORKTREES/tienda-s11" && echo ok > v11.txt && git add -A && git commit -qm impl11)
ok "$(q slice --slice 10 verificar)" 0 "S-10 verifica"
ok "$(q slice --slice 11 verificar)" 0 "S-11 verifica"
orq slice --slice 10 cerrar --aceptado >/tmp/orq-e2e.a.$$ 2>&1 & orq slice --slice 11 cerrar --aceptado >/tmp/orq-e2e.b.$$ 2>&1 & wait
rm -f /tmp/orq-e2e.a.$$ /tmp/orq-e2e.b.$$
ok "$(git log --oneline main | grep -c 'slice S-1[01]/')" 2 "los dos cierres simultáneos integran una vez cada uno"
ok "$(cat v10.txt v11.txt | tr -d '\n')" okok "main tiene los dos cambios"
orq metricas > met.txt; grep -q "slices aceptados e integrados: 7" met.txt; ok $? 0 "métricas: 7 aceptados"
echo "== higiene"
orq doctor >/tmp/orq-e2e.$$ 2>&1; d=$?
if [ $d -ne 0 ]; then cat /tmp/orq-e2e.$$; fi
ok $d 0 "doctor: sin worktrees, ramas ni cuarentena sobrantes"
ok "$(git branch --list 'slice/*' | wc -l)" 0 "sin ramas slice/*"
rm -f /tmp/orq-e2e.$$
echo; echo "Resultado: $pasa pasan, $falla fallan"; [ $falla -eq 0 ]
