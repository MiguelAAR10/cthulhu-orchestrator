package cli

import "orq/internal/lang"

func usageText() string { return lang.T(usageEN, usageES) }

// usageEN is the default help. Every Spanish name still works as an alias (ORQ_LANG=es shows the Spanish help).
const usageEN = `orq ` + Version + ` · harness for focused multi-agent development

Safety and resources
  orq gate            PreToolUse hook: reads JSON on stdin; exit 2 = block
  orq watch           measures scratch and disk; writes HALT when needed
  orq gc [--dry-run]  retires orq worktrees without losing anything (never --force); doubtful ones go to quarantine
  orq status          limits, measurements, HALT and workers
  orq halt [--clear | --set MSG]

Project
  orq init [--mode=improve|new] [--sync=local|commit|branch]   detects mode and stack, installs hooks, lists skills
  orq profile [set type=greenfield|brownfield|legacy criticality=trivial|low|medium|high stage=development|production stacks=a,b]
  orq skills [plan --phase X --role R | install [ID...] | vet ID [--file F] | pin | verify]
  orq skill ROLE                 role guide (atena|orchestrator|planner|astra|scout|worker|verifier) and its skills
  orq phase                      T·Translation, C·Context, A·Analysis, D1·Development, D2·Documentation
  orq context                    SessionStart hook: role, phase, handoff note and memory
  orq doc | orq doctor           living project sheet · leftover worktrees, branches, sessions and memory
  orq config agents [show | detect | set ROLE AGENT:MODEL]   agents and models per role (orq.agents.json)

Talking and remembering
  orq up                         from Atena's pane: creates the Orchestrator session
  orq msg send --to ROLE --type story|question|answer|status|closing [--attempt S-NN/aK] [--ref ID] [--key K] "text"
  orq msg read [-n 10] | reconcile [--resend ID]
  orq mem add --imp I --cat C [--source F] [--slice S-NN] "text ≤200" | show [--cat C] | compact
  orq handoff-note [--file-only] ROLE   writes .orq/relevo-ROLE.md and opens a fresh session (alias: relevo)
  orq learn [--global] show|add "lesson"|confirm ID|refute ID|promote ID
  orq align [--slice NN]         does the change still serve the story? exit 3 = ask Atena

Slices (with several open slices, pass --slice NN)
  orq slice open NN --title T --test PATH --cmd "CMD" [--bundle a,b] [--healthy "CMD"] [--scope paths]
         [--no-scouts "reason"] [--report F --format tap|junit|gotest-json] [--allow-skip] [--isolated] [--strict] [--timeout S]
                                 requires a healthy base and a red-by-assertion exam; creates the worktree
  orq slice take WHO [--handoff]     one writer per slice
  orq slice scope PATH[,PATH] --approved "req…"   widens the scope with Atena's approval
  orq slice exam PATH --approved "req…"           changes the acceptance exam with approval
  orq slice launch [--agent A] [--task ID|next] [--difficulty low|medium|high] [--zone Z --paths a,b] [--scope "..."]
                                 opens the worker in its pane (runner: herdr|tmux|process) with its contract,
                                 after admission (disk, HALT, worker and worktree caps)
  orq slice merge                integrates each team worker's commits; rejects out-of-zone or exam changes
  orq slice verify               runs the untouched exam, counts tests, writes a receipt (2 repairs or no progress → BLOCKED)
  orq slice retry "what changes" [--decision-owner]   new attempt from BLOCKED (max 3)
  orq slice close --accepted | --rejected "reason"   integrates, revalidates, publishes and cleans up; resumable
  orq slice status | receipt | contract | reopen "reason" | refingerprint "reason" | prune
  orq plan [--slice NN] load FILE.json | show | ready | review | done ID | release ID | debated
  orq front brief|receipt F [--slice NN]   one-page brief / aggregated receipt of a front (plan.fronts)
  orq session show | open planner|astra | close NAME   (alias: sesion)

Owner visibility
  orq worker beat|done|list      live worker state (.orq/workers/<id>/state.json, ORQ_WORKER_ID)
  orq token                      the Owner's token (export ORQ_TOKEN="$(orq token)"); never reaches a worker
  orq session register [--pid N] [--role R]   anchor the Owner's shell (signed with the token) so
                                 its descendants count as privileged without a terminal
  orq stop raise|list|resolve|cancel   requests to the Owner (credential|api_key|decision|confirmation|access;
                                 blocking|soon|fyi); only atena/owner resolve; never a secret in text
  orq autonomy [show|guided|free --scope ...]   free mode is approved by the Owner
  orq board [--json|--html [--out F]]   Owner board: stoppers, slices, workers, velocity
  orq handoff [--out F]          HANDOFF.md so another person can pick up the project
  orq revive                     deterministic context for a fresh session
  orq sync push|pull [--theirs]  memory on the orq/state branch
  orq metrics [--since YYYY-MM-DD] [--json]

Quality
  orq qa run S-NN                test pyramid and QA receipt (orchestrator only)
  orq qa finding add S-NN --requirement R --commit SHA --repro CMD --observed O --expected E --impact I | orq qa show S-NN
  orq evidence add S-NN --kind target|before|after --file PNG [--note "..."]
  orq version | orq help

Messages are in English; ORQ_LANG=es switches them (help, errors, status lines) to Spanish.
`
