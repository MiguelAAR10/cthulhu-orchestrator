# orq setup · look & feel (scout R6, 2026-11-13)
Read-only. [V]=verified in a file/URL, [I]=my inference. Site = `creative-tech/ai-edgerunners-landing-next` (SITE); repo = this repo.

## 1 Palette  [V] SITE/DESIGN.md §0,§2.1-2.2; components/lab/tool.css:7-9,178-210 (the site's own terminal)
Rule: "No colour. Ever." / "Never add a hue" (DESIGN.md §0, §2.2). Terminal = Noir surface. Octopus GIFs (assets/) are greys only.
| token | hex | 256 | truecolor fg | use |
|---|---|---|---|---|
| field `--tool-field` | #0a0a09 | 232 | 38;2;10;10;9 | background (leave terminal bg; use only for inverse text) |
| bone `--bone` | #e9e7e1 | 254 | 38;2;233;231;225 | body text, selected row bg |
| tool-dim | #9b988f | 247 | 38;2;155;152;143 | labels, help |
| bone-dim `--snav-dim` | #8b887f | 245 | 38;2;139;136;127 | hints, rules, status |
| chip `.tool-term__ok` | #2a2926 | 235 | 48;2;42;41;38 | "ok/changed" chip bg |
| paper `--paper` | #f2f1ed | 255 | 38;2;242;241;237 | light-terminal text bg swap |
| pen `--pen` / meta | #121212 / #807e77 | 233 / 244 | 38;2;18;18;18 / 38;2;128;126;119 | light-terminal text / dim |
256 codes = nearest on the xterm grey ramp (8+10i); [I] computed by hand, not by a tool. `ESC[38;5;Nm`, bg `ESC[48;5;Nm`, truecolor `ESC[38;2;R;G;Bm`.
Selected row = inverse (bone bg, field fg) like `.tool-term__no`; BLOCKED/error = inverse too, never red (DESIGN.md §4.5 "a glitch is ink inversion").
Fallbacks: 16 colours: body `37`, dim `90`, selected `ESC[7m`, chip `ESC[1m`. NO_COLOR set and non-empty: emit no colour codes but keep `ESC[7m`/bold (no-color.org: rule covers colour only, "present and not an empty string"). TERM=dumb or no tty: no escapes.
Depth detect [I, de-facto convention, no standard]: COLORTERM=truecolor|24bit -> truecolor; TERM has "256color" -> 256; else 16.

## 2 Art  [V] wordmark = straight cuts, 5x6 grid, 45 deg cuts (SITE components/site/Wordmark.tsx:1-5); never set the name as a typeface logo (DESIGN.md §2.3)
Mine is a new block-pixel `orq` with cut corners (not the site's paths); octopus = Edge Octo traits from DESIGN.md §4.5: round head, tall oval eyes (3 rows), curly tentacles. Dissolve rule = Bayer-dither edge motif (§3.5). Unicode (<=79 cols x 6 rows):
```
                                                                  ▄▄██████▄▄
   ██████    ████████      ██████                                ████▀██▀████
 ██      ██  ██      ██  ██      ██   · CTHULHU                  ████ ██ ████
 ██      ██  ████████    ██      ██     ORCHESTRATOR             ▀██████████▀
 ██      ██  ██    ██      ████████                             ▄█▀ ██  ██ ▀█▄
   ██████    ██      ██          ██                             ▀█ ▀█    █▀ █▀
 ██████████████████████████████████████████████████████████████████▓▓▓▒▒░░░ · ·
```
Pure ASCII (same grid; `#` full, `"` top half, `_` bottom half, `%:.` dither):
```
                                                                  __######__
   ######    ########      ######                                ####"##"####
 ##      ##  ##      ##  ##      ##   - CTHULHU                  #### ## ####
 ##      ##  ########    ##      ##     ORCHESTRATOR             "##########"
 ##      ##  ##    ##      ########                             _#" ##  ## "#_
   ######    ##      ##          ##                             "# "#    #" #"
 ##################################################################%%%::... - -
```
Use ASCII when LC_ALL/LC_CTYPE/LANG lack "UTF-8" or TERM=linux/dumb [I]. Characters never animate except a 160 ms ink-inversion blink on change (DESIGN.md §2.5).

## 3 Mock `orq setup` 80x24 (row 1-24; every line <=79 cols, checked by script)
```
                                                                  ▄▄██████▄▄
   ██████    ████████      ██████                                ████▀██▀████
 ██      ██  ██      ██  ██      ██   · CTHULHU                  ████ ██ ████
 ██      ██  ████████    ██      ██     ORCHESTRATOR             ▀██████████▀
 ██      ██  ██    ██      ████████                             ▄█▀ ██  ██ ▀█▄
   ██████    ██      ██          ██                             ▀█ ▀█    █▀ █▀
 ██████████████████████████████████████████████████████████████████▓▓▓▒▒░░░ · ·
   los agentes escriben el código. un programa decide cuándo termina.
 ▌ terminal         [x] herdr   [ ] orca
   atena            opus 5.5     esfuerzo medio
   orquestador      sonnet 5.5   esfuerzo medio
   planificador     opus 5.5     esfuerzo alto
   astra            codex        esfuerzo alto
   verificador      sonnet 5.5   esfuerzo bajo
   worker fácil     haiku 5.5    esfuerzo bajo
   worker medio     sonnet 5.5   esfuerzo medio
   worker difícil   opus 5.5     esfuerzo alto
   calidad          [ ] rápido   [x] normal   [ ] estricto
   workers máx      < 3 >  de 1 a 4
   presupuesto RAM  < 6.0 GB >  libres ahora: 9.4 GB
   caja             bubblewrap · 11/11 pruebas · siempre activa
 ──────────────────────────────────────────────────────────────────────────────
 ▸ herdr abre cada agente en un panel. orq decide cuándo y cuántos.
 ↑↓ mover   ←→ cambiar   enter guardar   r valores por defecto   q salir
```
Sizes below 80x24: refuse the screen, print "terminal muy chica: 80x24 mínimo" and fall to the line prompt (§4). Selected row = inverse from col 2 to 79.

## 4 Keys with stdlib only  [V] tested on this box (go1.25.8, throwaway program in scratchpad)
- tty test: `syscall.Syscall(SYS_IOCTL, fd, TCGETS, &t)` with `t syscall.Termios` succeeds only on a real tty. `os.ModeCharDevice` is NOT enough: `</dev/null` reported chardev=true but TCGETS failed (ENOTTY). Under `script` TCGETS succeeded. Size: `TIOCGWINSZ` into `struct{r,c,x,y uint16}`; ioctl returned 0x0 under `script` (pty without size) -> default to 80x24 [V].
- raw: copy golang.org/x/term makeRaw (golang/term term_unix.go): save termios, clear Iflag IGNBRK|BRKINT|PARMRK|ISTRIP|INLCR|IGNCR|ICRNL|IXON, Lflag ECHO|ECHONL|ICANON|ISIG|IEXTEN (+Oflag OPOST only if you draw with \n; for a full-screen UI keep OPOST ON), set Cc[VMIN]=1, Cc[VTIME]=0, write with `TCSETS`. Restore with the saved copy via `defer` AND from the signal handler. Needs `unsafe`; stdlib `syscall` only. Not run in raw mode here beyond TCGETS [I: the set path is the same ioctl].
- shortcut [V stty exists, I not tested]: `exec.Command("stty","-F","/dev/tty","-icanon","-echo","min","1")`; works but spawns a process and breaks if stty is missing; prefer ioctl.
- reading keys: `os.Stdin.Read` 1 byte at a time; `ESC [ A/B/C/D` = arrows. With VMIN=1 a lone Esc is ambiguous: use VMIN=0,VTIME=1 (100 ms) for the byte after ESC [I]. Ctrl-C is a plain 0x03 byte once ISIG is off: treat as quit. Keep ISIG ON instead if you prefer the OS to deliver SIGINT [I].
- must restore on: normal exit, panic (`defer`), SIGINT/SIGTERM/SIGHUP, SIGTSTP (restore, then re-raise). `signal.Notify` can catch these [V pkg.go.dev/os/signal: SIGHUP,SIGINT,SIGTERM, SIGTSTP/TTIN/TTOU, SIGWINCH; SIGKILL cannot]. SIGWINCH -> redraw.
- alt screen [I: standard xterm sequences]: enter `ESC[?1049h`, leave `ESC[?1049l`; hide cursor `ESC[?25l`/`ESC[?25h`. Redraw whole frame with `ESC[H` + per-line `ESC[K`; no flicker lib needed.
- Not a tty (CI, pipe, `orq setup < file`): no escapes at all. Line mode: print each setting `N) terminal [herdr] (herdr/orca) >`, read a line, empty = keep, invalid = repeat once then keep. `--yes`/`--no-interactivo` prints the resulting JSON and exits 0 [I].

## 5 Copy (Spanish, voseo-free, short, lowercase kickers; tone from SITE data/site.ts bootLines and DESIGN.md §9: plain, no hype, no invented claims)
- title line: `orq · setup` · tagline: "los agentes escriben el código. un programa decide cuándo termina." (README tagline, translated)
- terminal: "quién abre los paneles" · calidad: "rápido = una pasada · normal = prueba + verificador · estricto = prueba oculta + revisión" (strict wording depends on P8; drop if not built)
- roles: "atena: la única voz con el Owner" · "orquestador: reparte, no escribe código" · "planificador: el plan mínimo" · "astra: discute el plan" · "verificador: ojos frescos"
- workers: "fácil: cambios de una línea" · "medio: una función con pruebas" · "difícil: tocar varios módulos"
- capacidad: "workers máx: de 1 a 4; más no ayuda en trabajo secuencial" (P3) · "RAM: orq no lanza si queda menos"
- caja: "siempre activa. no se apaga." (plan Decisions: no sandbox-off switch) · if selftest fails: inverse "CAJA ROTA · no se lanza nada"
- keys: "↑↓ mover · ←→ cambiar · enter guardar · r por defecto · q salir" · status: "SIN CAMBIOS" / "CAMBIOS SIN GUARDAR" / "GUARDADO ✓"
Unverified: Spanish register for Peru is [I] (site copy is English; README rulebook is Spanish, docs/REGLAS-HARNESS.md); model names/efforts in the mock are placeholders, not the real defaults.
