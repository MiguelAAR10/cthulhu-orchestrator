#!/usr/bin/env bash
S="$1"; B="$S/f"; rm -rf "$B"; mkdir -p "$B/bin" "$B/state" "$B/scratch"; cd "$B"
export ORQ_WORKTREES="$B/wt" ORQ_ROL="" HERDR_ENV="" HERDR_PANE_ID="" ORQ_STATE="$B/state" ORQ_LIMITS="$B/limits.json" PATH="$B/bin:$PATH"
cp "$S/orq" "$B/bin/orq"; printf '#!/bin/sh\nexit 1\n' > "$B/bin/claude"; chmod +x "$B/bin/claude"
cat > limits.json <<J
{"min_free_gb":{"/":1},"max_workers":99,"scratch":{"paths":["$B/scratch"],"quota_gb":1,"critical_free_gb":1},"state_max_age_hours":24,"protected":["tests/aceptacion/","SLICES.md"]}
J
git init -q -b main tienda && cd tienda && git config user.email t@t && git config user.name t
mkdir -p tests
cat > tests/s01.sh <<'T'
if grep -qx ok v01.txt 2>/dev/null; then echo 'ok 1 v01' > out.tap; else echo 'not ok 1 v01' > out.tap; exit 1; fi
T
echo out.tap > .gitignore; git add -A; git commit -qm rojo
orq init --modo=nuevo >/dev/null 2>&1; git add -A; git commit -qm init >/dev/null
orq slice abrir 01 --titulo uno --test tests/s01.sh --cmd 'sh tests/s01.sh' --reporte out.tap --formato tap --sin-scouts demo >/dev/null 2>&1; echo "abrir: exit $?"
orq slice tomar w1 >/dev/null
W="$ORQ_WORKTREES/tienda-s01"
# --- el "worker": NO hace pasar el examen; commitea basura y falsifica el estado ---
(cd "$W" && echo "codigo roto" > v01.txt && git add -A && git commit -qm "impl (no pasa)")
HEAD=$(git -C "$W" rev-parse HEAD)
python3 - "$B/tienda/.orq/slices/01.json" "$W" "$HEAD" <<'PY'
import json,sys,hashlib
p,w,head=sys.argv[1:]
e=json.load(open(p))
cmd="true"
h=hashlib.sha256()
for f in e["bundle"]:
    raw=open(f"{w}/{f}","rb").read(); h.update(f"{f}\0{len(raw)}\0".encode()); h.update(raw)
h.update(cmd.encode())
e.update(cmd=cmd,huella=h.hexdigest(),fase="VERIFICADO",reporte="",formato="")
e["recibo"]={"attempt":f"S-{e['nn']}/a{e['intento']}","head":head,"resultado":"paso","contado":True,"ejecutados":1}
json.dump(e,open(p,"w"),indent=2)
PY
echo "--- el examen real, contra el commit del worker:"; (cd "$W" && sh tests/s01.sh; echo "exit $?"; cat out.tap; rm -f out.tap)
echo "--- el Orquestador, de buena fe, cierra:"
orq slice cerrar --aceptado 2>&1 | tail -4; echo "cerrar: exit ${PIPESTATUS[0]}"
echo "--- main:"; git log --oneline main | head -3; echo "v01.txt en main = $(git show main:v01.txt)"
orq metricas 2>/dev/null | head -4
