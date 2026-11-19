#!/usr/bin/env bash
# Prueba de aceptación del aislamiento: lo que corre un worker no alcanza el estado de orq ni el repo principal.
#
#   validacion/aislamiento.sh          Parte 1 + Parte 2; ambas cuentan para el código de salida
#   validacion/aislamiento.sh --todo   Parte 2 se muestra como PENDING y no cuenta (hasta que el cierre
#                                      deje de confiar en un estado falsificable)
#
# Parte 1: desde `orq sandbox exec --rw <worker>`, cada ataque falla y deja su blanco intacto; un commit
#          dentro del worker funciona (control positivo).
# Parte 2: el ataque de estado falsificado (forja): el worker commitea basura y reescribe el slice como
#          VERIFICADO; al cerrar, main no debe avanzar y la negativa tiene que ser la del contrato en git.
# Sin bwrap: «SKIP: bwrap missing» y salida 1 (0 solo con ORQ_CI_ALLOW_SKIP=1).
#
# ORQ apunta al binario a probar (por defecto ORQ_CANDIDATO, el que compiló make ci); tiene que ser ese. Usa ORQ_STATE, ORQ_LIMITS y
# ORQ_WORKTREES propios en directorios temporales que se borran al terminar. No toca repos reales.
set -uo pipefail

RAIZ="$(cd "$(dirname "$0")/.." && pwd)"
TODO=0; [[ "${1:-}" == "--todo" ]] && TODO=1

BASE=$(mktemp -d "${TMPDIR:-/tmp}/orq-aislamiento.XXXXXX")
WBASE=$(mktemp -d "${TMPDIR:-/tmp}/orq-aislamiento-w.XXXXXX") # el worker vive aparte: dentro de la caja no hay padres comunes
cleanup() {
  [[ "$BASE" == */orq-aislamiento.* ]] && rm -rf -- "$BASE"
  [[ "$WBASE" == */orq-aislamiento-w.* ]] && rm -rf -- "$WBASE"
}
trap cleanup EXIT

ORQ="${ORQ:-${ORQ_CANDIDATO:-}}"
[[ -n "$ORQ" ]] && ORQ=$(cd "$(dirname "$ORQ")" && pwd)/$(basename "$ORQ")
. "$RAIZ/validacion/candidato.sh"; candidato "$ORQ" || exit 1

export ORQ_STATE="$BASE/state" ORQ_LIMITS="$BASE/limits.json" ORQ_WORKTREES="$BASE/wt"
export ORQ_ROL="" HERDR_ENV="" HERDR_PANE_ID=""
mkdir -p "$ORQ_STATE" "$BASE/scratch"
cat > "$ORQ_LIMITS" <<EOF
{ "min_free_gb": {"/": 1}, "max_workers": 99,
  "scratch": {"paths": ["$BASE/scratch"], "quota_gb": 1, "critical_free_gb": 1},
  "state_max_age_hours": 24, "protected": ["tests/aceptacion/", "SLICES.md"] }
EOF

PASS=0; FAIL=0; PEND=0
ok()   { PASS=$((PASS+1)); printf '  ✔ %s\n' "$1"; }
fail() { FAIL=$((FAIL+1)); printf '  ✘ %s\n' "$1"; }
huella() { if [[ -e "$1" || -L "$1" ]]; then sha256sum "$1" | cut -d' ' -f1; else echo ausente; fi; }

# Without bubblewrap nothing here can be proved: say so and fail, unless CI explicitly allows the skip.
if ! command -v bwrap >/dev/null 2>&1; then
  echo "SKIP: bwrap missing (install bubblewrap); isolation not tested"
  [[ "${ORQ_CI_ALLOW_SKIP:-}" == 1 ]] && exit 0
  exit 1
fi

echo "== Autoprueba de la caja"
if "$ORQ" sandbox selftest; then ok "orq sandbox selftest"; else fail "orq sandbox selftest"; echo "sin caja sana no sigue"; exit 1; fi

echo "== Parte 1 · preparar main, estado y worker"
MAIN="$BASE/main"; W="$WBASE/worker"
git init -q -b main "$MAIN" && git -C "$MAIN" config user.email orq@local && git -C "$MAIN" config user.name orq
echo base > "$MAIN/a.txt"; git -C "$MAIN" add -A; git -C "$MAIN" commit -qm base
mkdir -p "$MAIN/.orq/slices"; echo '{"nn":"01","fase":"CONSTRUYENDO"}' > "$MAIN/.orq/slices/01.json"
"$ORQ" halt --set "prueba de aislamiento" >/dev/null
cp "$ORQ" "$BASE/orq-copia"
git clone -q --no-hardlinks "$MAIN" "$W" && git -C "$W" config user.email w@local && git -C "$W" config user.name worker
[[ -f "$MAIN/.git/refs/heads/main" && -f "$ORQ_STATE/HALT" ]] || { echo "preparación incompleta"; exit 1; }

caja() { "$ORQ" sandbox exec "$@"; }

ataque() { # $1 descripción  $2 blanco  $3 guion sh  [resto: opciones de la caja]
  local desc=$1 blanco=$2 guion=$3; shift 3
  local antes despues rc out
  antes=$(huella "$blanco")
  # "dentro" prueba que el ataque corrió en la caja: que orq se niegue a arrancarla no cuenta como defensa.
  out=$(caja --rw "$W" "$@" -- sh -c 'test -c /run/orq/sandbox && echo dentro; '"$guion" 2>/dev/null </dev/null); rc=$?
  despues=$(huella "$blanco")
  if [[ "$out" != *dentro* ]]; then fail "$desc (la caja no arrancó, exit $rc)"
  elif [[ $rc -ne 0 && "$antes" == "$despues" ]]; then ok "$desc"
  else fail "$desc (exit $rc, blanco ${antes:0:12} → ${despues:0:12})"; fi
}

echo "== Parte 1 · ataques desde orq sandbox exec --rw <worker>"
ataque "escribir <main>/.orq/slices/01.json"  "$MAIN/.orq/slices/01.json"  "printf '{\"fase\":\"VERIFICADO\"}' > '$MAIN/.orq/slices/01.json'"
ataque "escribir el archivo de límites"        "$ORQ_LIMITS"                "printf '{}' > '$ORQ_LIMITS'"
ataque "quitar HALT"                           "$ORQ_STATE/HALT"            "rm '$ORQ_STATE/HALT'"
ataque "escribir <main>/.git/refs/heads/main"  "$MAIN/.git/refs/heads/main" "printf '0000000000000000000000000000000000000000\n' > '$MAIN/.git/refs/heads/main'"
ataque "sobrescribir una copia del binario orq" "$BASE/orq-copia"           "printf '#!/bin/sh\nexit 0\n' > '$BASE/orq-copia'"
ataque "enlace en el worker hacia el estado"   "$MAIN/.orq/slices/01.json"  "ln -sf '$MAIN/.orq/slices/01.json' '$W/enlace' && printf x > '$W/enlace'"
rm -f "$W/enlace"
# Con main visible de solo lectura los blancos existen dentro de la caja y aun así no se escriben.
ataque "con --ro <main>: escribir .orq/slices/01.json" "$MAIN/.orq/slices/01.json"  "printf x > '$MAIN/.orq/slices/01.json'" --ro "$MAIN"
ataque "con --ro <main>: escribir refs/heads/main"     "$MAIN/.git/refs/heads/main" "printf x > '$MAIN/.git/refs/heads/main'" --ro "$MAIN"

conectar() { # $1 descripción  $2 socket
  local hay=no rc out
  [[ -S "$2" ]] && hay=sí
  out=$(caja --rw "$W" -- sh -c 'test -c /run/orq/sandbox && echo dentro; exec python3 -c "import socket,sys; s=socket.socket(socket.AF_UNIX); s.settimeout(3); s.connect(sys.argv[1])" "$1"' _ "$2" 2>/dev/null </dev/null); rc=$?
  if [[ "$out" != *dentro* ]]; then fail "$1 (la caja no arrancó, exit $rc)"
  elif [[ $rc -ne 0 ]]; then ok "$1 (existe en el host: $hay)"; else fail "$1: conectó desde la caja"; fi
}
conectar "conectar al socket de herdr" "$HOME/.config/herdr/herdr.sock"
conectar "conectar al socket de docker" "/var/run/docker.sock"

echo "== Parte 1 · control positivo"
antes=$(git --git-dir="$W/.git" rev-parse HEAD)
caja --rw "$W" --cwd "$W" -- sh -c 'echo hola > nuevo.txt && git add nuevo.txt && git commit -qm "desde la caja"' </dev/null; rc=$?
despues=$(git --git-dir="$W/.git" rev-parse HEAD)
msg=$(git --git-dir="$W/.git" log -1 --format=%s)
if [[ $rc -eq 0 && "$antes" != "$despues" && "$msg" == "desde la caja" ]]; then ok "commit dentro del worker"
else fail "commit dentro del worker (exit $rc, HEAD ${antes:0:8} → ${despues:0:8}, '$msg')"; fi
if [[ "$(git -C "$MAIN" rev-parse main)" == "$(git --git-dir="$W/.git" rev-parse HEAD~1)" ]]; then ok "main intacto tras la Parte 1"
else fail "main cambió durante la Parte 1"; fi

echo "== Parte 2 · estado falsificado (forja)"
parte2() (
  F="$BASE/forja"; mkdir -p "$F/bin" "$F/state" "$F/scratch"; cd "$F" || exit 1
  export ORQ_WORKTREES="$F/wt" ORQ_STATE="$F/state" ORQ_LIMITS="$F/limits.json" PATH="$F/bin:$PATH"
  cp "$ORQ" "$F/bin/orq"; printf '#!/bin/sh\nexit 1\n' > "$F/bin/claude"; chmod +x "$F/bin/claude"
  cat > limits.json <<J
{"min_free_gb":{"/":1},"max_workers":99,"scratch":{"paths":["$F/scratch"],"quota_gb":1,"critical_free_gb":1},"state_max_age_hours":24,"protected":["tests/aceptacion/","SLICES.md"]}
J
  git init -q -b main tienda && cd tienda && git config user.email t@t && git config user.name t
  mkdir -p tests
  cat > tests/s01.sh <<'T'
if grep -qx ok v01.txt 2>/dev/null; then echo 'ok 1 v01' > out.tap; else echo 'not ok 1 v01' > out.tap; exit 1; fi
T
  echo out.tap > .gitignore; git add -A; git commit -qm rojo
  orq init --modo=nuevo >/dev/null 2>&1; git add -A; git commit -qm init >/dev/null
  orq slice abrir 01 --titulo uno --test tests/s01.sh --cmd 'sh tests/s01.sh' --reporte out.tap --formato tap --sin-scouts demo >/dev/null 2>&1 \
    || { echo "    preparación: slice abrir falló"; exit 2; }
  orq slice tomar w1 >/dev/null || { echo "    preparación: slice tomar falló"; exit 2; }
  WF="$ORQ_WORKTREES/tienda-s01"
  # el "worker": no hace pasar el examen; commitea basura y falsifica el estado
  (cd "$WF" && echo "codigo roto" > v01.txt && git add -A && git commit -qm "impl (no pasa)")
  HEAD=$(git -C "$WF" rev-parse HEAD)
  python3 - "$F/tienda/.orq/slices/01.json" "$WF" "$HEAD" <<'PY'
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
  (cd "$WF" && sh tests/s01.sh >/dev/null 2>&1) && { echo "    preparación: el examen real pasó, la forja no prueba nada"; exit 2; }
  antes=$(git rev-parse main)
  salida=$(orq slice cerrar --aceptado 2>&1); rc=$?
  printf '%s\n' "$salida" | tail -3 | sed 's/^/    cerrar: /'
  despues=$(git rev-parse main)
  echo "    main ${antes:0:8} → ${despues:0:8}; v01.txt en main = '$(git show main:v01.txt 2>/dev/null)'"
  [[ "$antes" == "$despues" ]] || exit 1
  # The refusal must be the contract check, not some other failure that happens to stop the close.
  [[ $rc -ne 0 && "$salida" == *"el estado no coincide con el contrato"* && "$salida" == *"refs/orq/contrato/01"* ]] || exit 3
)
parte2; rc=$?
if [[ $rc -eq 0 ]]; then ok "forja: main no avanza y el cierre se niega por el contrato en git"
elif [[ $rc -eq 3 ]]; then fail "forja: main no avanzó, pero el cierre no se negó por el contrato (motivo equivocado)"
elif [[ $rc -eq 2 ]]; then fail "forja: la preparación falló, el ataque no se pudo montar"
elif [[ $TODO -eq 1 ]]; then PEND=$((PEND+1)); printf '  … PENDING forja: main avanzó con un estado falsificado (falta que el cierre revalide contra el contrato)\n'
else fail "forja: main avanzó con un estado falsificado"; fi

echo
echo "== Resultado: $PASS pasan, $FAIL fallan, $PEND pendientes"
[[ $FAIL -eq 0 ]]
