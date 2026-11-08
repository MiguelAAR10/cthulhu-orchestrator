# Quickstart

Ten minutes, one toy repo, one slice from red to closed. No real agent is launched: you play the worker.
Every step below is exactly what [`validacion/quickstart.sh`](../validacion/quickstart.sh) runs and asserts.

## 0. Build orq

```bash
cd cthulhu-harness/harness && go build -o ~/bin/orq ./cmd/orq   # Go 1.25+, stdlib only
export ORQ_WORKTREES=/tmp/orq-wt                                # where slice worktrees live
```

## 1. A toy repo with a failing acceptance test

```bash
mkdir /tmp/toy && cd /tmp/toy && git init -b main
mkdir tests && cat > tests/acceptance.sh <<'T'
if grep -qx hello greeting.txt 2>/dev/null; then echo 'ok 1 greeting' > out.tap; else echo 'not ok 1 greeting' > out.tap; exit 1; fi
T
echo out.tap > .gitignore && git add -A && git commit -m "test: failing acceptance test"
```

## 2. Install orq in the project

```bash
orq init --mode=new                # detects stack, installs hooks, lists skills
git add -A && git commit -m "chore: orq init"
orq profile set type=greenfield    # type, criticality, stage, stacks
orq profile                        # check it
```

## 3. Open the slice with its exam

```bash
orq slice open 01 --title greeting --test tests/acceptance.sh --cmd "sh tests/acceptance.sh" \
  --report out.tap --format tap --no-scouts "quickstart"
orq slice take w1                  # one writer per slice
```

`open` refuses unless the exam is red **by assertion** and fingerprints the test bundle.
It creates the worktree `$ORQ_WORKTREES/toy-s01`.

## 4. Be the worker

Normally `orq slice launch` opens an agent in that worktree. Here you do its job:

```bash
(cd $ORQ_WORKTREES/toy-s01 && echo hello > greeting.txt && git add -A && git commit -m "feat: greeting")
```

## 5. Verify

```bash
orq slice verify                   # runs the untouched exam, counts tests, writes a receipt
orq slice receipt
```

## 6. Look at the board

```bash
orq board                          # stoppers, slices with %, workers, velocity
orq board --html --out board.html  # same, as a page
```

## 7. Ask the Owner for something (a stopper)

```bash
orq stop raise --type api_key --severity blocking --slice S-01 \
  --where DEMO_API_KEY --text "Need the API key for the demo"
orq stop list
```

Never put a secret in the text: say *where* the Owner should put it. Only Atena or the Owner resolves it
(`orq stop resolve`).

## 8. Hand the project to someone else

```bash
orq handoff                        # writes HANDOFF.md
```

## 9. QA and close

Default criticality is `medium`, which requires an independent QA receipt before closing:

```bash
printf '{"sync":"local","qa":{"layers":[{"name":"acceptance","cmd":"sh tests/acceptance.sh"}]}}\n' > .orq/config.json
orq qa run S-01
orq slice close --accepted         # integrates into main, revalidates, cleans the worktree
cat greeting.txt                   # hello — it is on main now
```

With `"criticality":"low"` in `.orq/config.json` you can skip `orq qa run`.

## 10. Housekeeping

```bash
orq gc --dry-run                   # what would be retired; never --force
orq doctor                         # leftover worktrees, branches, sessions
```

## Next

- Real agents: `orq config agents detect`, then `orq slice launch` (runner `herdr`, `tmux` or `process`, see `orq.agents.json`).
- Roles: `orq skill atena|orchestrator|planner|worker|verifier`.
- Run the whole thing hermetically: `bash validacion/quickstart.sh` → `QUICKSTART OK`.
- Spanish aliases exist for every command (`orq slice abrir`, `verificar`, `cerrar --aceptado`); `ORQ_LANG=es` for Spanish messages.
