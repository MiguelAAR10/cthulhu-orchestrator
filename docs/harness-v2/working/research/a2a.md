# R1 — A2A vs orq's channel (scout report)
Sources: A2A spec https://a2a-protocol.org/latest/specification/ (v1.0.0 latest; read ~200k of 257k chars, §3.3.1 and §13 NOT read);
A2A vs MCP https://a2a-protocol.org/latest/topics/a2a-and-mcp/ ; Claude Code agent teams https://code.claude.com/docs/en/agent-teams ;
orq: harness/internal/canal/canal.go, cli/proyecto.go (runMsg).

## 1. Verified facts — A2A (spec URL above, section numbers inline)
- v1.0.0 is current; 0.3.0/0.2.6/0.1.0 earlier; client sends `A2A-Version` header (§3.6). Normative source is `spec/a2a.proto` (§1.4).
  Linux Foundation governance, v1.0 ~March 2026 (https://opensource.googleblog.com/2026/04/a-year-of-open-collaboration-celebrating-the-anniversary-of-a2a.html; secondary).
- Bindings: JSON-RPC over HTTPS, gRPC, HTTP+JSON/REST (§9-11). Everything is network-first: an HTTP endpoint per agent.
- Agent Card (§4.4, §8): JSON with identity, skills, endpoint/interfaces, capabilities (streaming, pushNotifications, extendedAgentCard), security schemes;
  optional JWS signature over JCS-canonicalized card (§8.4, RFC 7515/8785), `kid` required.
- Task states (§4.1.3): working SUBMITTED, WORKING; interrupted INPUT_REQUIRED, AUTH_REQUIRED; terminal COMPLETED, FAILED, CANCELED, REJECTED.
  Messages to a terminal task are rejected (UnsupportedOperationError, §3.1.1). Task IDs are server-generated; client cannot choose them (§3.4.2).
- Message (§4.1.4): messageId, contextId, taskId, role (USER|AGENT), parts, referenceTaskIds. Part (§4.1.6): exactly one of text/raw/url/data + mediaType.
  Artifact (§4.1.7): artifactId + parts. Spec guidance: final outputs go in Artifacts, not Messages (§3.7).
- Ops (§3.1): SendMessage, SendStreamingMessage, GetTask, ListTasks (cursor), CancelTask, SubscribeToTask, push-config CRUD.
- Streaming: each event is exactly one of task|message|statusUpdate|artifactUpdate, delivered in generation order (§3.2.3, §3.5.2).
- Push (§4.3): HTTP POST webhook; agent MUST attempt at-least-once, MAY retry with backoff; client MUST 2xx, SHOULD dedupe, MUST validate task ID.
- Idempotency (§3.3.1, via summary only, not read verbatim): GET naturally idempotent; SendMessage "may be" idempotent via messageId; CancelTask idempotent.
- Auth (§7, §4.5): API key, HTTP auth, OAuth2, OIDC, mTLS declared in the card; credentials in HTTP layer, not in message body. In-task auth uses AUTH_REQUIRED and out-of-band credentials (§7.6).
- A2A vs MCP: "A2A connects the agents to each other; MCP connects each agent to its own tools" (a2a-and-mcp page). A2A = stateful multi-turn peers; MCP = mostly stateless tools.

## 2. Verified facts — real harness practice
- Claude Code agent teams (experimental): per-agent mailbox JSON file `~/.claude/teams/{team}/inboxes/{agent}.json`; "sent" only when the file write succeeds;
  entries validated on read, malformed ones removed; shared task list with states pending/in-progress/completed + dependencies, claiming via file locking;
  hooks TeammateIdle/TaskCreated/TaskCompleted can veto with exit 2; idle notification carries the teammate's final answer; receiving agent is told the message came from another
  Claude session, not the user, and cannot give consent on the user's behalf (https://code.claude.com/docs/en/agent-teams, "Messages between agents").
  Known limits there: task status lags, no resume of teammates, one team per session. No network protocol; files + locks.
- Third-party reverse-engineering agrees on the inbox layout (https://dev.to/nwyin/reverse-engineering-claude-code-agent-teams-architecture-and-protocol-o49) — secondary, may drift.
- Not found: any evidence of a mainstream local coding harness using A2A on the wire between same-machine agents (my search, not proof of absence).

## 3. orq's channel today (read from code)
- canal.go: append-only JSONL, flock on append, ids `cN` from line count, `clave` dedupe (ErrDuplicado), pending/entregado via later "evento" lines,
  roles fixed list or `worker-*`, `De` taken from `--de`/pane lookup (RolDePane), types historia|pregunta|respuesta|estado|cierre|plan, text <= 4000 runes,
  `Attempt` and `Ref` optional strings. Delivery = type text into herdr pane; on herdr failure message stays stored, "may or may not have arrived", `orq msg reconciliar`.
- proyecto.go ~216-232: a `cierre` must carry the full attempt `S-NN/aK` and is refused if not the current attempt (stale results not delivered).

## 4. Mapping: which A2A concepts fix real weaknesses
| orq weakness | A2A concept | Verdict |
|---|---|---|
| `De` is self-declared (`--de`); `worker-x` can impersonate `atena`. | Auth lives in the transport (card security schemes), never in the body. | Borrow the principle, not the schemes: orq must derive sender from something the sender can't forge (per-role socket/FIFO owned by orq, or token file readable only by that role's sandbox user). Self-declared field becomes advisory. |
| No task state; `cierre`/`estado` are free messages; "is S-03/a2 done?" needs log grep. | Task with explicit states, terminal states reject further messages, server-assigned IDs. | Borrow. orq already has slice/attempt state; make a message with `Attempt` update that state machine only via the gate, and reject sends to a closed attempt generally (today only `cierre`). Map: SUBMITTED=plan sent, WORKING, INPUT_REQUIRED=`pregunta` open, COMPLETED/FAILED/CANCELED/REJECTED. |
| Results are prose in a 4000-char text; verifier can't parse them. | Message vs Artifact; Parts (text/data/url + mediaType). | Borrow. `cierre` carries `data` (JSON: commit SHA, bundle path, test counts) or a file URL to a receipt; text stays human-only. Harness reads the data, never the prose. |
| Delivery: stored, then typed into a pane; "may have arrived" ambiguity; can interleave with the agent's own typing. | Push = at-least-once + receiver dedupe; Get/Subscribe to recover missed state. | Borrow the contract: at-least-once with dedupe by id (orq already has the id). Fix the ambiguity by making the receiver ack (`orq msg leer` marks read) and by pull (inbox the agent reads) as the source of truth; the pane keystroke becomes only a doorbell ("1 new message"). This is the Claude Code mailbox shape. |
| Idempotency only if the sender supplied `clave`. | Client-chosen messageId; server-assigned task id. | Borrow: orq auto-derives clave = hash(de,para,ref,attempt,texto) when absent. |
| `Ref` is optional and unchecked; no thread id. | contextId / referenceTaskIds. | Minor: validate `Ref` exists; use `Attempt` as the context id. |
| Roles hard-coded; no capability discovery. | Agent Card + skills. | Skip for now; roles are fixed by the harness. A static roles.json per project is enough. |

## 5. Wire question: speak A2A or borrow the model?
Recommendation: borrow the model, do not speak A2A on the wire. (My inference, from facts above.)
- A2A is HTTP/gRPC/JSON-RPC with a server per agent, agent cards, OAuth/mTLS options. orq agents are CLI processes in herdr panes (Claude Code, Codex) that do not host servers; adding an A2A server per worker means a daemon, ports and an auth story to defend, for peers that are all on one machine under one owner.
- The peers orq talks to (Claude Code, Codex) do not expose A2A endpoints (not found in their docs; I did not verify Codex). Nothing to interoperate with locally.
- Interop would only pay off if an orq role were later hosted remotely or a third-party agent had to be called; then a thin A2A gateway in front of the file-based store is a cheap later add (SendMessage -> append, GetTask -> read state). Keep the on-disk shape close enough (id, state, parts, artifacts) so that adapter is trivial.
- Do not confuse with MCP: MCP is how an agent gets tools; orq's gate already sits on that side. Inter-agent messaging is not an MCP problem.

## 6. What orq should adopt (ranked)
1. Unforgeable sender identity. Why: today any worker can write `--de atena`; every other guarantee depends on who spoke. Effort M. Changes: `--de` is ignored for workers; orq resolves identity from a per-role credential/channel it created (inside the A3 sandbox, a role-specific file or socket); RolDePane remains only a fallback for humans.
2. Pull-based inbox + doorbell, replacing "typing the whole message into the pane". Why: removes interleaving and the "may or may not have arrived" state; message text never enters the agent's prompt unescaped from a stranger; matches Claude Code's mailbox. Effort M. Changes: `Format()` push becomes a one-line nudge; delivery state = read-by-recipient (`entregado` becomes `leído`); `reconciliar` shrinks.
3. Structured results: `cierre` and `estado` carry `data` (JSON) plus optional file refs; harness consumes only data. Why: lets the gate/verifier act on results mechanically and kills prose-parsing. Effort S-M. Changes: Msg gains `Data json.RawMessage`/`Archivos []string`; Validate checks schema per tipo.
4. Task/attempt state machine with terminal states enforced for all message types. Why: stale `plan`/`respuesta` to a closed attempt are rejected like the existing stale `cierre`; one place answers "what state is S-03/a2". Effort S. Changes: generalize the cierre check in proyecto.go into Validate-time rule using the slice state.
5. Auto-derived `clave` (hash) and validated `Ref`. Why: dedupe without sender discipline; threads can't dangle. Effort S. Changes: Append computes clave when empty.
6. Optional hook veto on message types (like TaskCompleted exit 2): `orq msg send --tipo cierre` runs the verifier precheck before acceptance. Why: same pattern Claude Code uses for quality gates. Effort S-M. Changes: cierre acceptance goes through the gate rather than only attempt-matching.

## 7. What orq should NOT adopt
- A2A over HTTP/gRPC/JSON-RPC now: needs a server per agent plus auth, for same-machine peers that don't speak it.
- Agent Cards and card signing: roles are static and trusted by construction; discovery solves a problem orq doesn't have.
- OAuth2/OIDC/mTLS schemes: identity should come from the OS/sandbox boundary, not bearer tokens in a single-user box.
- SSE streaming / SubscribeToTask: orq's coordinator reads state at stage boundaries; streaming adds long-lived connections for no gate benefit.
- Push-notification webhooks: SSRF/replay surface (spec §13.2, not read) with no remote consumer.
- Free peer-to-peer worker chatter à la agent teams: orq's workers are zone-isolated by design; keep worker↔worker messages rare and routed/logged by orq.
- Server-assigned task IDs from a remote agent: orq owns slice/attempt ids already (S-NN/aK).
- AUTH_REQUIRED/in-task credential flow: orq's agents never hold credentials mid-task (plan: dedicated credential decided by Owner).

## 8. Unverified / caveats
- A2A §3.3.1 (messageId idempotency wording) and §13 (security) not read verbatim.
- Claude Code mailbox details are from official docs; JSON schema of entries from third-party reverse engineering only.
- Whether Codex exposes any inter-agent protocol: not checked.
