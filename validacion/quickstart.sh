#!/usr/bin/env bash
# Hermetic quickstart: the steps a newcomer runs, each one asserted. Touches nothing outside a temp dir.
# Usage: bash validacion/quickstart.sh   → prints "QUICKSTART OK" or exits non-zero.
set -u
REPO="$(cd "$(dirname "$0")/.." && pwd)"
BASE="$(mktemp -d)"; trap 'rm -rf "$BASE"' EXIT
export ORQ_WORKTREES="$BASE/wt" ORQ_ROL="" HERDR_ENV="" HERDR_PANE_ID="" ORQ_WORKER_ID=""
mkdir -p "$BASE/bin"
printf '#!/bin/sh\nexit 1\n' > "$BASE/bin/claude"; chmod +x "$BASE/bin/claude"   # fake agent: never launched
export PATH="$BASE/bin:$PATH"
(cd "$REPO/harness" && timeout 300 go build -o "$BASE/bin/orq" ./cmd/orq) || { echo "build failed"; exit 1; }
LOG="$BASE/log"
step() { local d="$1"; shift; if timeout 60 "$@" >"$LOG" 2>&1; then echo "  ok  $d"; else echo "  FAIL $d"; cat "$LOG"; exit 1; fi; }
has()  { grep -q -- "$1" "$LOG" || { echo "  FAIL output lacks '$1'"; cat "$LOG"; exit 1; }; }

cd "$BASE" && git init -q -b main toy && cd toy && git config user.email q@q && git config user.name q
mkdir -p tests && cat > tests/acceptance.sh <<'T'
if grep -qx hello greeting.txt 2>/dev/null; then echo 'ok 1 greeting' > out.tap; else echo 'not ok 1 greeting' > out.tap; exit 1; fi
T
echo out.tap > .gitignore; git add -A && git commit -qm "test: failing acceptance test"

step "orq init"                 orq init --mode=new
git add -A && git commit -qm "chore: orq init" >/dev/null
step "orq profile set"          orq profile set type=greenfield
step "orq profile"              orq profile; has greenfield
step "orq slice open (red exam)" orq slice open 01 --title greeting --test tests/acceptance.sh --cmd "sh tests/acceptance.sh" \
                                   --report out.tap --format tap --no-scouts "quickstart"
W="$ORQ_WORKTREES/toy-s01"; [ -d "$W" ] || { echo "  FAIL no worktree $W"; exit 1; }
step "orq slice take"           orq slice take w1
(cd "$W" && echo hello > greeting.txt && git add -A && git commit -qm "feat: greeting") || exit 1   # simulated worker
step "orq slice verify (green)" orq slice verify
step "orq board"                orq board; has "S-01"; has "%"
step "orq stop raise"           orq stop raise --type api_key --severity blocking --slice S-01 --where DEMO_API_KEY --text "Need the API key for the demo"
step "orq stop list"            orq stop list; has api_key
step "orq handoff"              orq handoff; [ -f HANDOFF.md ] || { echo "  FAIL HANDOFF.md missing"; exit 1; }
# Default criticality is medium: closing needs an independent QA receipt (`orq qa run`).
# A toy is low stakes; the profile is the single source of truth for criticality.
step "orq profile set criticality=low" orq profile set criticality=low
step "orq slice close --accepted" orq slice close --accepted
grep -qx hello greeting.txt || { echo "  FAIL main lacks the fix"; exit 1; }
step "orq gc --dry-run"         orq gc --dry-run
echo "QUICKSTART OK"
