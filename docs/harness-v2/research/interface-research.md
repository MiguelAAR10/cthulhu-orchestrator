# orq2 interface research: one self-updating page now, a localhost interface later

Author: Fable (research and planning only), 2026-11-20, three read-only Sonnet scouts plus my own doc searches.
**After the pilot's stop rule; nothing is built now.** **[V]** read by me, **[S]** scout or public doc, **[U]** unverified.

## 1. Direction (Owner, final)
1. **NOW: one self-contained HTML file that updates itself.** `orq2 panel --html` regenerated on each journal
   event or by `watch`. Its buttons copy the exact text to paste into Atena or the terminal. Goal: speed and
   comprehension. No server, no port, no action from the page.
2. **LATER, optional: a localhost interface** (web and terminal faces, requests). Researched below, not planned.

Unchanged: `orq2 panel --json` (`Vista`, `esquema: 1`) is the single data contract, additive fields do not bump
it, and `panel --html` already renders the Kanban with copy buttons (`panel-report.md` [V]); Owner locks are a
request queue confirmed only on his TTY (`agent-first-plan.md` §3 [V]); `v2/panel` is parked (verdict §1 [V]).

## 2. NOW: the self-updating page
### 2.1 How it updates (no server)
| Trigger | Cost | Risk | Reliability | Reversibility |
|---|---|---|---|---|
| **After each orq2 command** (hook in `cli`, orq2 identity only, when `panel_html` is configured) | ~25 lines | adds a few ms per command; must skip `gate` | good: every journal event comes from a command | one config key |
| **`orq2 watch`** also regenerates (heartbeat for time-based fields) | ~5 lines | none if warning-only | good | same key |

Recommended: both together (a long-lived `panel --seguir` loop only if they prove too slow). Builder rules:
- The hook lives in `cli` after dispatch, never in `diario` (`panel` imports `diario`; the reverse breaks the
  layer test). `diario.Anotar` stays untouched [V: `diario.go:51`]. Skip `gate`, `panel`, `help`, `version`.
- Write to a temp file in the same directory, then `rename`: the browser never reads half a page.
- The file lives outside `.orq/` (`TestPanelNoEscribeOrq` must stay green), path from one config key; unset = off.
- A panel failure prints one warning line on stderr and never changes the command's exit code or HALT.
- Under identity `orq` nothing changes: no key, no hook, byte-identical output (existing golden tests).
- The page reloads itself (`<meta http-equiv="refresh">`, 5 s), keeps the open drawer in `location.hash` and
  the scroll in `sessionStorage`, and shows "generado hace N s" from the embedded time so a stale page is visible.
  `fetch` does not work on `file://`, so reload is the only no-server way. [U: flicker; `\\wsl$` from Windows.]

### 2.2 Copy buttons: exact text for two destinations
Today each card carries `comandos[]` and a copy button with a select-text fallback, because `file://` is not a
secure context for the clipboard API [V: `panel.html.tmpl:188-192`]. Add one additive field:
`Tarjeta.textos[] {destino: terminal|atena, etiqueta, texto}`, built in `Construir`, never typed in the template.
- **terminal**: the exact command, for what only the Owner may run today (`orq2 slice aprobar NN`,
  `slice decidir NN`, `slice cerrar --slice NN --aceptado`). Atena must not run these (misuse M8 [V]).
- **atena**: one Spanish sentence with the story id, for everything else: "¿Qué bloquea S-03 y qué necesitas
  de mí?", "Pausa S-02", "Explícame el plan de S-01". Atena is an agent, so this text never approves anything.
- Tests: every `terminal` text is in the command table; every text is visible in a `<code>` (works with JS off).

### 2.3 Top strip: speed and comprehension
Additive fields, same rule as today (`Dato`, never a bare 0):
- **Te toca**: count and ids from the columns `aprobacion` and `decision` (already in `Vista`).
- **Siguiente paso**: the first step marked `now` of the oldest open card, and who owns it (derivable today).
- **Avance**: "3 de 8 historias cerradas", never a naked %. The base is printed beside it: stories agreed in
  `canal.jsonl` plus slices opened; unknown base = `sin dato`. Rejected stories are counted apart.
- **Hitos**: no source exists in `.orq/` today [U]; `sin dato` until a small `hitos` file is agreed.

### 2.4 Smallest build steps (after the stop rule; tests first, one commit each)
| # | Commit | Size | Proof |
|---|---|---|---|
| 0 | Merge `v2/panel` with its own review (parked; first merge of the next window) | done work | `make ci` |
| **1** | **`panel: page reloads itself and shows its age`** (template only) | ~15 lines JS/HTML | golden HTML; script still ≤60 lines |
| **2** | **`cli: regenerate the panel page after each orq2 command`** + `watch` | ~30 lines | exit code unchanged on failure; `orq` byte-identical; `.orq` hash unchanged; `gate` skipped |
| 3 | `panel: copy texts for Atena and the terminal` | ~60 lines | texts in the command table; visible in `<code>` |
| 4 | `panel: top strip` (te toca, siguiente paso, N de M; milestones once a `hitos` format is agreed) | ~100 lines | goldens; `TestSinDatoNuncaCero` extended |
| 5 | Brand pass: embedded fonts, three mascots, state mapping (§3) | assets + CSS | `TestHTMLAutonomo`: no URL, size cap |

Steps 1 and 2 are the whole "self-updating file": about 45 lines, removable by deleting one key and one call.

## 3. Visual identity (`creative-tech/ai-edgerunners-landing-next/`, read-only; `DESIGN.md` §2, §4.5)
- **Keep the Kanban layout** of `orq2-backlog.html`: columns, cards, tags, drawer, theme button; add the strip.
- **Tokens [V].** Day: paper `#f2f1ed`, pen `#121212`. Noir: field `#0a0a09`, bone `#e9e7e1`. Alert `#c1121f`
  (`app/globals.css:40`). Radius 0, 2–3 px rules, hard offset shadows, `.striae`/`.dither` patterns [S].
- **One conflict.** `DESIGN.md:18` says "No colour. Ever." [V]; the Kanban prototype uses six state hues [S].
  Recommendation: brand ground, ink and type; keep the hues only as the small state tags; the Owner decides.
- **Fonts.** Jersey 10 and Pixelify Sans through `next/font/google` (`brand-fonts.ts` [V]): no woff2 in the
  repo; SIL OFL 1.1 [U: no licence file]. Embed Latin subsets as base64 (10–30 KB [U]) with the OFL text.
- **Mascots.** Only the Owner's three: Edge Octo, Edge Prime Octo, The Dreamer; `design/source/cast/` also has
  fan art of third-party model mascots [V]: keep those out. No LICENSE file, `"private": true` [V]. Edge Octo
  anims: `idle, think, build, sleep, glitch, run, …` [V]. WebP [V]: think 3.1 KB, build 2.1 KB, dreamer 17.6 KB.
  `idle` and `glitch` are sprite frames only and must be exported; no pose exists for "waiting" or "blocked".

| State | From `Vista` | Page (one mascot per column header) | Terminal, later |
|---|---|---|---|
| idle | nothing open, or `cerrado` | Edge Octo `idle` | `·` dim |
| working | `construccion`, `cerrando` | Edge Octo `build`; `think` while planning | `▶` spinner |
| waiting for Owner | `aprobacion`, `decision` | The Dreamer [S: `wake`], beside "Te toca" | `◆` red |
| blocked | `bloqueado`, HALT | Edge Octo `glitch` | `✖` red, inverse |

## 4. LATER, optional: a localhost interface (researched, not planned)
### 4.1 References only: how two agent tools ship a local web UI
| | What the page connects to | Transport | Security |
|---|---|---|---|
| **OpenClaw** Control UI [S: docs.openclaw.ai/web, /gateway/security] | the Gateway daemon, same port as its API (default 18789) | WebSocket; token or password sent in the handshake | auth required by default; loopback bind recommended; remote access through Tailscale Serve or an SSH tunnel, not an open bind |
| **Hermes Agent** `hermes dashboard` [S: hermes-agent.nousresearch.com docs, web-dashboard] | a local FastAPI/Uvicorn server, `127.0.0.1:9119`; the Chat tab embeds the TUI behind a pseudo-terminal | HTTP plus a stream for the pty [U: exact transport] | loopback by default; reported to read `.env` with no built-in password; a non-loopback bind requires registration or basic auth [U: config keys] |

Lessons: both put a **daemon with authority** behind the page, so the page can act. orq2 must not copy that:
agents and the Owner share one Unix user and any local process can read a token [S]. Worth copying: loopback
only, auth on by default, tunnel instead of open bind, the terminal face embedded in the page (Hermes).

### 4.2 Options, if it is ever built
| Face | Option | Cost | Risk | Reliability | Reversibility | Maintainability |
|---|---|---|---|---|---|---|
| Web | Polling `GET /vista.json` with ETag | ~20 lines [S] | a local port | highest | high | high |
| Web | **SSE** (`http.Flusher`) | ~50 lines [S] | 6 connections per origin on HTTP/1.1 [S] | `EventSource` reconnects itself [S] | high | high |
| Web | WebSocket | `x/net` (not stdlib) or ~300 hand-rolled lines [S][U] | cross-site hijack if Origin unchecked | manual reconnect | medium | low |
| Terminal | **`orq2 panel --seguir`** redraw loop, no raw mode | ~30 lines [S] | low | good | high | high |
| Terminal | `orq2 tui`, stdlib termios via `syscall` | 400–800 lines [S][U] | medium; Linux/WSL2 only | good with a pure `Pintar` | medium | medium |
| Terminal | Bubble Tea v2 in its own module (`charm.land/bubbletea/v2` [S]), or a Neovim plugin (Lua; Neovim **not installed** [S]) | ~300 lines + deps | young API; `send-keys` can drive an editor: never the lock | medium | high | low |

### 4.3 Shape and rules
- Web and terminal are two thin readers of `Vista` plus one event `{"esquema":1,"version":"<hash>"}` (stat poll
  of `.orq/` every 500 ms; the event carries no data, a reader re-reads `Vista`). Neither reads `.orq/`. The
  files are the bus, so the two faces agree without talking to each other; agents never subscribe.
- **Actions only through the request queue, confirmed on the Owner's TTY.** No HTTP confirm endpoint, ever.
  **The web may at most create a request** (`POST /pedir`, closed action list, argv built by orq2); that is
  harmless because an agent can already do it (exit 5). Agent customisation (role, model, account) is a view
  first (`config ver --json` [U: missing]) and a request after.
- Server: bind `127.0.0.1:0` only (WSL2 NAT reaches it from Windows [S]); start only on a TTY; reject any
  `Host` but `127.0.0.1|localhost:PORT` (DNS rebinding) [U]; per-run token as an `HttpOnly; SameSite=Strict`
  cookie; GET without side effects; POST behind Go 1.25 `http.CrossOriginProtection` [S]; no CORS headers.
- Order if chosen: request queue and minimal `orq2 tui` (agent-first §3) → agents view → SSE read-only → `POST /pedir`.

## 5. What NOT to do
- No server, port, token, action or agent editing in the NOW version; it writes one HTML file and nothing else.
- No hook inside `diario` or `gate`, no change to `orq` output, no write under `.orq/`, no non-atomic write.
- No naked percentage and no invented milestone: a number without its base is worse than `sin dato`.
- No approval text for Atena: approve, decide and close are terminal texts until the request queue exists.
- No third-party mascots, no runtime font or script fetch: the file must open with the network off.
- Later: no confirm over HTTP, no "trusted browser", no WebSocket, no third-party module in the core `go.mod`,
  no bind to `0.0.0.0`, no lock in Neovim or any `send-keys` target. Nothing before the stop rule.

## 6. Open points for the Owner
1. Palette: brand ground and ink with the prototype's hues as small tags (recommended), or strict monochrome?
2. Milestones: who writes them, and is a small `hitos` file acceptable as their only source?
3. Where the page file lives (one fixed path the Owner bookmarks; recommended: the project's state directory).
