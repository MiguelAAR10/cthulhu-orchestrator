package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"orq/internal/canal"
	"orq/internal/config"
	"orq/internal/identidad"
	"orq/internal/memoria"
	"orq/internal/orqdir"
	"orq/internal/perfil"
	"orq/internal/slice"
)

// ---------- init ----------

const hookSettings = `{
  "hooks": {
    "PreToolUse": [
      {"matcher": "Bash|Agent|Task|Edit|Write|MultiEdit|NotebookEdit|mcp__.*",
       "hooks": [{"type": "command", "command": "$HOME/bin/orq gate", "timeout": 5, "onFailure": "block"}]}
    ],
    "SessionStart": [
      {"hooks": [{"type": "command", "command": "$HOME/bin/orq contexto", "timeout": 10}]}
    ],
    "PreCompact": [
      {"hooks": [{"type": "command", "command": "$HOME/bin/orq relevo --auto", "timeout": 120}]}
    ]
  }
}
`

// hooks is the hooks block init writes: orq keeps hookSettings byte for byte; orq2 calls
// itself by its absolute path.
func hooks() string {
	if !identidad.EsOrq2() {
		return hookSettings
	}
	bin, _ := json.Marshal(identidad.Binario() + " ")
	return strings.ReplaceAll(hookSettings, "$HOME/bin/orq ", strings.Trim(string(bin), `"`))
}

// hooksOrq1 are the commands orq 1.0.0 puts in a project's .claude/settings (orq2's never match).
var hooksOrq1 = []string{"orq gate", "orq contexto", "orq relevo"}

// hookOrq1 returns the first orq 1.0.0 hook command in raw as written in the file ("" if none).
// A hit that follows ORQ_NOMBRE=orq2 on its line is orq2's own hook (the form it writes for a
// binary not named orq2); "orq2 gate" never contains "orq gate".
func hookOrq1(raw string) string {
	for _, h := range hooksOrq1 {
		for i, resto := 0, raw; ; {
			j := strings.Index(resto, h)
			if j < 0 {
				break
			}
			i += j
			ini := strings.LastIndexByte(raw[:i], '\n') + 1
			if !strings.Contains(raw[ini:i], "ORQ_NOMBRE=orq2") {
				return comandoEn(raw, i)
			}
			i += len(h)
			resto = raw[i:]
		}
	}
	return ""
}

// comandoEn is the quoted string of raw that contains offset i (the whole hook command), so the
// message quotes it with its path, as the file has it.
func comandoEn(raw string, i int) string {
	ini := strings.LastIndexByte(raw[:i], '"') + 1
	fin := strings.IndexByte(raw[i:], '"')
	if fin < 0 || strings.ContainsRune(raw[ini:i], '\n') {
		return strings.TrimSpace(raw[i:min(len(raw), i+40)])
	}
	return raw[ini : i+fin]
}

// gestionadoPorOrq1 says why orq 1.0.0 manages the project at root ("" if it does not): its hooks
// in .claude/settings.json or settings.local.json, or slices it opened. It writes nothing.
func gestionadoPorOrq1(root string) []string {
	var por []string
	for _, f := range []string{"settings.json", "settings.local.json"} {
		ruta := filepath.Join(root, ".claude", f)
		raw, err := os.ReadFile(ruta)
		if err != nil {
			continue
		}
		if h := hookOrq1(string(raw)); h != "" {
			por = append(por, fmt.Sprintf("%s tiene los hooks de orq 1.0.0 (%q)", ruta, h))
		}
	}
	if ids := slice.AbiertosOrq1(filepath.Join(root, ".orq")); len(ids) > 0 {
		por = append(por, "slices abiertos por orq 1.0.0: "+strings.Join(ids, ", "))
	}
	return por
}

func runInit(args []string, stdout, stderr io.Writer) int {
	for _, a := range args {
		if a == "--help" || a == "-h" {
			if identidad.EsOrq2() {
				fmt.Fprintln(stdout, "uso: orq init [--modo=mejora|--modo=nuevo] [--convivir]\nDetecta el perfil del proyecto, escribe .orq/ y los checks. --help no cambia nada.\n--convivir: en un proyecto que maneja orq 1.0.0 no escribe nada en .claude y dice qué añadir a mano.")
				return 0
			}
			fmt.Fprintln(stdout, "uso: orq init [--modo=mejora|--modo=nuevo]\nDetecta el perfil del proyecto, escribe .orq/ y los checks. --help no cambia nada.")
			return 0
		}
	}
	convivir := false
	if identidad.EsOrq2() {
		convivir = slices.Contains(args, "--convivir")
		if root, err := orqdir.Root(); err == nil {
			if por := gestionadoPorOrq1(root); len(por) > 0 && !convivir {
				fmt.Fprintf(stderr, "orq init: este proyecto lo maneja orq 1.0.0 y no se toca:\n  - %s\n"+
					"Cierra sus slices con orq 1.0.0 y quita sus hooks, o corre `orq init --convivir`: no escribe nada en .claude y dice qué añadir a mano.\n",
					strings.Join(por, "\n  - "))
				return 1
			}
		}
	}
	p, err := abrirProyecto()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := p.Excluir(); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	pf := perfil.Detect(p.Root)
	for _, a := range args {
		if a == "--modo=mejora" || a == "--modo=nuevo" {
			pf.Modo = strings.TrimPrefix(a, "--modo=")
		}
	}
	raw, _ := json.MarshalIndent(pf, "", "  ")
	if _, err := os.Stat(p.PerfilPath()); convivir && err == nil {
		fmt.Fprintf(stdout, "perfil ya existe en %s: no se tocó (--convivir)\n", p.PerfilPath())
	} else if err := orqdir.WriteAtomic(p.PerfilPath(), raw); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "modo: %s (%d archivos de código) · stack: %s\n", pf.Modo, pf.Fuentes, strings.Join(append([]string{"-"}, pf.Senales...)[min(1, len(pf.Senales)):], ", "))
	web := slices.Contains(pf.Senales, "web")
	if web {
		if err := escribirMapa(p.Root, stdout); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
	}
	if err := escribirChecks(p.Root, web, stdout); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if convivir {
		fmt.Fprintf(stdout, "--convivir: no se escribió nada en %s. Para usar orq en este proyecto añade a mano:\n"+
			"  - en .claude/settings.local.json, junto a los hooks de orq 1.0.0, este bloque hooks:\n%s"+
			"  - las skills de orq (%s) no se instalaron en .claude/skills: las guías de cada rol salen de `orq skill ROL`\n",
			filepath.Join(p.Root, ".claude"), hooks(), strings.Join(nombresSkillsOrq(), ", "))
		return runSkills(nil, stdout, stderr)
	}
	if nuevas, distintas := instalarSkillsOrq(p.Root); len(nuevas)+len(distintas) > 0 {
		if len(nuevas) > 0 {
			fmt.Fprintf(stdout, "skills de orq en el repo: %s\n", strings.Join(nuevas, ", "))
		}
		if len(distintas) > 0 {
			fmt.Fprintf(stdout, "aviso: %s difiere de la de orq %s; no se pisó\n", strings.Join(distintas, ", "), Version)
		}
	}
	set := filepath.Join(p.Root, ".claude", "settings.json")
	if _, err := os.Stat(set); os.IsNotExist(err) {
		os.MkdirAll(filepath.Dir(set), 0o755)
		os.WriteFile(set, []byte(hooks()), 0o644)
		fmt.Fprintf(stdout, "hooks instalados en %s: guardia, contexto al arrancar y relevo antes de compactar\n", set)
	} else {
		raw, _ := os.ReadFile(set)
		if !strings.Contains(string(raw), identidad.Texto("orq contexto")) {
			fmt.Fprintf(stdout, "ya existe %s: une a mano el bloque hooks de `orq init --hooks`\n", set)
		}
	}
	if len(args) > 0 && args[0] == "--hooks" {
		fmt.Fprint(stdout, hooks())
	}
	return runSkills(nil, stdout, stderr)
}

// escribirMapa deja en docs/mapa.md del repo el esqueleto con las rutas del router, si aún no hay mapa
// (tampoco uno viejo en .orq/): después es del Owner y de los workers, que lo versionan.
func escribirMapa(root string, stdout io.Writer) error {
	if viejo := slice.MapaEn(root); viejo != "" {
		fmt.Fprintf(stdout, "mapa de rutas ya existe en %s: no se tocó\n", viejo)
		return nil
	}
	ruta := filepath.Join(root, slice.RutaMapa)
	rs := perfil.Rutas(root)
	if err := os.MkdirAll(filepath.Dir(ruta), 0o755); err != nil {
		return err
	}
	if err := orqdir.WriteAtomic(ruta, []byte(perfil.Mapa(rs))); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "mapa de rutas en %s: %d rutas; completa cómo llegar y qué muestra cada una, y commitéalo\n", ruta, len(rs))
	return nil
}

// escribirChecks suma a <repo>/.orq/config.json el bloque checks detectado, sin tocar las demás claves,
// con el check e2e de Playwright en un proyecto web. Si el bloque ya existe es del Owner: no se pisa.
func escribirChecks(root string, web bool, stdout io.Writer) error {
	ruta := config.RutaProyecto(root)
	capa := map[string]json.RawMessage{}
	if raw, err := os.ReadFile(ruta); err == nil {
		if err := json.Unmarshal(raw, &capa); err != nil {
			return fmt.Errorf("orq init: %s: %w", ruta, err)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	var e2e config.Check
	if web {
		var sin string
		if e2e, sin = perfil.E2E(root); sin != "" {
			fmt.Fprintf(stdout, "sin check e2e: %s\n", sin)
		}
	}
	if propio, ok := capa["checks"]; ok {
		fmt.Fprintf(stdout, "checks ya definidos en %s: no se tocaron\n", ruta)
		var cs map[string]json.RawMessage
		json.Unmarshal(propio, &cs)
		if _, tiene := cs[config.E2E]; !tiene && e2e.Cmd != "" {
			raw, _ := json.Marshal(e2e)
			fmt.Fprintf(stdout, "añade a checks: \"e2e\": %s\n", raw)
		}
		return nil
	}
	cs := perfil.ChecksPara(root, identidad.EsOrq2())
	if e2e.Cmd != "" {
		cs[config.E2E] = e2e
	}
	raw, err := json.Marshal(cs)
	if err != nil {
		return err
	}
	capa["checks"] = raw
	out, err := json.MarshalIndent(capa, "", "  ")
	if err != nil {
		return err
	}
	if err := orqdir.WriteAtomic(ruta, append(out, '\n')); err != nil {
		return err
	}
	for _, ch := range (config.Config{Checks: cs}).ChecksEnOrden() {
		if ch.Interno() {
			fmt.Fprintf(stdout, "check %s: %s\n", ch.Nombre, slice.CmdSupresiones)
		} else if ch.Servidor != "" {
			fmt.Fprintf(stdout, "check %s: %s · servidor %s en 127.0.0.1:%d\n", ch.Nombre, ch.Cmd, ch.Servidor, ch.Puerto)
		} else if ch.Cmd != "" {
			fmt.Fprintf(stdout, "check %s: %s\n", ch.Nombre, ch.Cmd)
		} else {
			fmt.Fprintf(stdout, "check %s omitido: %s\n", ch.Nombre, ch.Omitir)
		}
	}
	fmt.Fprintf(stdout, "checks en %s: edítalos ahí; `orq check` los corre\n", ruta)
	return nil
}

// ---------- skills ----------

func runSkills(args []string, stdout, stderr io.Writer) int {
	p, err := abrirProyecto()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	ls := perfil.Lista(p.Perfil())
	sub := ""
	if len(args) > 0 {
		sub = args[0]
	}
	switch sub {
	case "fijar":
		m := map[string]perfil.Fijada{}
		for _, s := range ls {
			if h, err := perfil.HuellaSkill(p.Root, s.Nombre); err == nil && perfil.Instalada(p.Root, s.Nombre) {
				m[s.Nombre] = perfil.Fijada{Repo: s.Repo, Huella: h, Roles: s.Roles, Adapta: s.Adapta}
			}
		}
		raw, _ := json.MarshalIndent(m, "", "  ")
		if err := orqdir.WriteAtomic(p.LockPath(), raw); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintf(stdout, "%d skills fijadas en %s (versión revisada; un cambio se avisará)\n", len(m), p.LockPath())
		return 0
	case "verificar":
		ps := perfil.Verificar(p.Root, ls, p.Fijadas())
		for _, x := range ps {
			fmt.Fprintf(stdout, "%-6s %-32s %s\n", x.Tipo, x.Skill, x.Detalle)
		}
		cfg := os.Getenv("CLAUDE_CONFIG_DIR")
		if home, _ := os.UserHomeDir(); cfg == "" {
			cfg = filepath.Join(home, ".claude")
		}
		if cfg != "" {
			if m, _ := filepath.Glob(filepath.Join(cfg, "plugins", "cache", "*", "superpowers")); len(m) > 0 {
				fmt.Fprintln(stdout, "aviso: el plugin superpowers está instalado para todos los proyectos; su using-superpowers se carga en cada sesión. Desactívalo en /plugin si quieres solo las piezas de este repo.")
			}
		}
		if len(ps) == 0 {
			fmt.Fprintln(stdout, "skills en orden: las del proyecto, con sus dependencias y sin cambios")
			return 0
		}
		return 1
	}
	instalar := sub == "instalar"
	// orq skills instalar a b c: solo esas (y lo que requieren), no todo el catálogo.
	if instalar && len(args) > 1 {
		pedidas := map[string]bool{}
		for _, n := range args[1:] {
			pedidas[n] = true
		}
		for _, s := range ls {
			if pedidas[s.Nombre] {
				for _, r := range s.Requiere {
					pedidas[r] = true
				}
			}
		}
		var sel []perfil.Skill
		for _, s := range ls {
			if pedidas[s.Nombre] {
				sel = append(sel, s)
				delete(pedidas, s.Nombre)
			}
		}
		for n := range pedidas {
			fmt.Fprintf(stderr, "%s no está en el catálogo de este proyecto\n", n)
			return 1
		}
		ls = sel
	}
	porRepo := map[string][]string{}
	var repos []string
	for _, s := range ls {
		mark := "✓"
		if !perfil.Instalada(p.Root, s.Nombre) {
			mark = "·"
			if _, ok := porRepo[s.Repo]; !ok {
				repos = append(repos, s.Repo)
			}
			porRepo[s.Repo] = append(porRepo[s.Repo], s.Nombre)
		}
		fmt.Fprintf(stdout, "  %s %-32s %-26s %s\n", mark, s.Nombre, strings.Join(s.Roles, ","), s.Repo)
	}
	faltan := 0
	for _, repo := range repos {
		miss := porRepo[repo]
		faltan += len(miss)
		if !instalar {
			continue
		}
		cmd := perfil.ComandoInstalar(repo, miss)
		c := exec.Command(cmd[0], cmd[1:]...)
		c.Dir = p.Root
		if out, err := c.CombinedOutput(); err != nil {
			fmt.Fprintf(stderr, "falló %s: %v\n%s\n", strings.Join(cmd, " "), err, tail(string(out), 5))
			return 1
		}
		faltan -= len(miss)
	}
	switch {
	case faltan == 0 && instalar:
		fmt.Fprintln(stdout, "skills instaladas solo en este repo. Revísalas y fíjalas: `orq skills fijar`")
	case faltan == 0:
		fmt.Fprintln(stdout, "skills del proyecto completas (solo en .claude/skills de este repo)")
	case instalar:
		fmt.Fprintf(stdout, "faltan %d skills\n", faltan)
	default:
		fmt.Fprintf(stdout, "faltan %d: `orq skills instalar` las deja solo en este repo\n", faltan)
	}
	return 0
}

func tail(s string, n int) string {
	l := strings.Split(strings.TrimSpace(s), "\n")
	return strings.Join(l[max(0, len(l)-n):], "\n")
}

// ---------- fase ----------

func runFase(stdout io.Writer, stderr io.Writer) int {
	p, err := abrirProyecto()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	f, d := p.Fase()
	fmt.Fprintf(stdout, "%s · %s\n", f, d)
	return 0
}

// ---------- contexto (hook SessionStart) ----------

func runContexto(stdout, stderr io.Writer) int {
	p, err := abrirProyecto()
	if err != nil {
		return 0 // fuera de un repo el hook no aporta nada y no debe estorbar
	}
	if _, err := os.Stat(p.PerfilPath()); err != nil {
		return 0 // proyecto sin orq init
	}
	rol := p.RolActual()
	pf := p.Perfil()
	f, d := p.Fase()
	fmt.Fprintf(stdout, "# orq · %s\nRol: %s · modo: %s · fase: %s (%s)\n", filepath.Base(p.Root), def(rol, "sin rol"), pf.Modo, f, d)
	if rol != "" {
		fmt.Fprintf(stdout, "Tu guía y tus skills: `orq skill %s`\n", rol)
		if raw, err := os.ReadFile(p.RelevoPath(rol)); err == nil {
			fmt.Fprintf(stdout, "\n%s\n", strings.TrimSpace(string(raw)))
		}
	}
	items := p.MemVigente()
	if len(items) > 0 {
		lines := strings.Split(strings.TrimSpace(memoria.Render(items, "")), "\n")
		if len(lines) > 25 {
			lines = append(lines[:25], fmt.Sprintf("… %d más: orq mem ver", len(lines)-25))
		}
		fmt.Fprintf(stdout, "\n## Memoria\n%s\n", strings.Join(lines, "\n"))
	}
	fmt.Fprint(stdout, p.Lecciones())
	if _, err := os.Stat(p.FichaPath()); err == nil {
		fmt.Fprintf(stdout, "\nFicha del proyecto: %s\n", p.FichaPath())
	}
	return 0
}

func def(s, d string) string {
	if s == "" {
		return d
	}
	return s
}

// ---------- doc (fase D2, modelo barato) ----------

func runDoc(stdout, stderr io.Writer) int {
	p, err := abrirProyecto()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := p.EscribirFicha(); err != nil {
		fmt.Fprintln(stderr, "orq doc:", err)
		return 1
	}
	fmt.Fprintf(stdout, "ficha actualizada: %s\n", p.FichaPath())
	return 0
}

// ---------- doctor (leases e higiene) ----------

func runDoctor(stdout, stderr io.Writer) int {
	p, err := abrirProyecto()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	problemas := 0
	r := p.Slices()
	if h, err := r.Huerfanos(); err == nil {
		for _, w := range h {
			problemas++
			fmt.Fprintf(stdout, "worktree sobrante: %s\n  quitar: git worktree remove %s\n", w, w)
		}
	}
	abiertos, err := r.Abiertos()
	if err != nil {
		problemas++
		fmt.Fprintf(stdout, "estado de slices ilegible: %v\n", err)
	}
	out, _ := exec.Command("git", "-C", p.Root, "branch", "--list", "slice/*", "--merged").Output()
	for _, b := range strings.Fields(strings.ReplaceAll(string(out), "*", "")) {
		if slices.ContainsFunc(abiertos, func(e *slice.Estado) bool { return e.Rama == b }) {
			continue
		}
		problemas++
		fmt.Fprintf(stdout, "rama ya integrada: %s\n  quitar: git branch -d %s\n", b, b)
	}
	for _, e := range abiertos {
		if _, err := os.Stat(e.Worktree); err != nil {
			problemas++
			fmt.Fprintf(stdout, "%s abierto pero su worktree no existe: %s\n", e.Attempt(), e.Worktree)
		}
		if age := time.Since(e.Abierto); age > 48*time.Hour {
			problemas++
			fmt.Fprintf(stdout, "S-%s lleva %.0f h abierto: ciérralo o pártelo\n", e.NN, age.Hours())
		}
	}
	if raw, err := os.ReadFile(r.CuarentenaPath()); err == nil {
		q := 0
		for _, l := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
			if l != "" {
				q++
				problemas++
				fmt.Fprintf(stdout, "en cuarentena: %s\n", l)
			}
		}
		if q > 0 {
			fmt.Fprintf(stdout, "  revisa cada uno; cuando esté resuelto, vacía %s\n", r.CuarentenaPath())
		}
	}
	for _, e := range abiertos {
		if e.Fase == slice.Bloqueado {
			problemas++
			fmt.Fprintf(stdout, "%s BLOQUEADO: %s\n", e.Attempt(), e.Motivo)
		}
	}
	ses, _ := canal.LoadSesiones(p.SesPath())
	if p.Term.Dentro() {
		for rol, s := range ses {
			if !p.SesionViva(context.Background(), s) {
				problemas++
				fmt.Fprintf(stdout, "la sesión de %s (%s) ya no existe: orq relevo %s\n", rol, s.Agente, rol)
			}
		}
	}
	items, _ := memoria.Load(p.MemPath())
	if v := len(memoria.Vigentes(items)); v > memoria.Tope {
		problemas++
		fmt.Fprintf(stdout, "memoria con %d líneas (tope %d): orq mem compactar\n", v, memoria.Tope)
	}
	if problemas == 0 {
		fmt.Fprintln(stdout, "todo en orden: sin worktrees ni ramas sobrantes, sesiones vivas, memoria bajo el tope")
		return 0
	}
	fmt.Fprintf(stdout, "%d cosa(s) por revisar. orq no borra nada solo: los comandos de arriba los corres tú o el Orquestador.\n", problemas)
	return 3
}
