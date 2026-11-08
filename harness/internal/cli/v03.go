package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"orq/internal/identidad"
	"orq/internal/lang"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"orq/internal/canal"
	"orq/internal/escriba"
	"orq/internal/memoria"
	"orq/internal/orqdir"
	"orq/internal/perfil"
	"orq/internal/slice"
)

func (p proyecto) perfilPath() string { return filepath.Join(p.dir, "proyecto.json") }
func (p proyecto) fichaPath() string  { return filepath.Join(p.dir, "PROYECTO.md") }

func (p proyecto) perfil() perfil.Perfil {
	var pf perfil.Perfil
	if raw, err := os.ReadFile(p.perfilPath()); err == nil && json.Unmarshal(raw, &pf) == nil && pf.Modo != "" {
		return pf
	}
	return perfil.Detect(p.root)
}

// rolActual sale de ORQ_ROL (paneles creados por orq) o del panel registrado del Atena.
func (p proyecto) rolActual() string {
	if r := os.Getenv("ORQ_ROL"); r != "" {
		return r
	}
	ses, _ := canal.LoadSesiones(p.sesPath())
	return canal.RolDePane(ses, rn().Current())
}

// ---------- init ----------

const hookSettings = `{
  "hooks": {
    "PreToolUse": [
      {"matcher": "Bash|Agent|Task|Edit|Write|MultiEdit|NotebookEdit|Read|Grep|Glob",
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

func runInit(args []string, stdout, stderr io.Writer) int {
	p, err := abrirProyecto()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := p.excluir(); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	pf := perfil.Detect(p.root)
	for _, a := range args {
		if a == "--modo=mejora" || a == "--modo=nuevo" {
			pf.Modo = strings.TrimPrefix(a, "--modo=")
		}
	}
	raw, _ := json.MarshalIndent(pf, "", "  ")
	if err := orqdir.WriteAtomic(p.perfilPath(), raw); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := p.initSync(args, stdout); err != nil {
		fmt.Fprintln(stderr, "orq init:", err)
		return 1
	}
	fmt.Fprintf(stdout, "modo: %s (%d archivos de código) · stack: %s\n", pf.Modo, pf.Fuentes, strings.Join(append([]string{"-"}, pf.Senales...)[min(1, len(pf.Senales)):], ", "))
	if nuevas, distintas := instalarSkillsOrq(p.root); len(nuevas)+len(distintas) > 0 {
		if len(nuevas) > 0 {
			fmt.Fprintf(stdout, "skills de orq en el repo: %s\n", strings.Join(nuevas, ", "))
		}
		if len(distintas) > 0 {
			fmt.Fprintf(stdout, "aviso: %s difiere de la de orq %s; no se pisó\n", strings.Join(distintas, ", "), Version)
		}
	}
	set := filepath.Join(p.root, ".claude", "settings.json")
	if _, err := os.Stat(set); os.IsNotExist(err) {
		os.MkdirAll(filepath.Dir(set), 0o755)
		os.WriteFile(set, []byte(hookSettings), 0o644)
		fmt.Fprintf(stdout, "hooks instalados en %s: guardia, contexto al arrancar y relevo antes de compactar\n", set)
	} else {
		raw, _ := os.ReadFile(set)
		if !strings.Contains(string(raw), "orq contexto") {
			fmt.Fprintf(stdout, "ya existe %s: une a mano el bloque hooks de `orq init --hooks`\n", set)
		}
	}
	if len(args) > 0 && args[0] == "--hooks" {
		fmt.Fprint(stdout, hookSettings)
	}
	// El token del Owner se crea aquí, fuera del repo; init lo corre el Owner en su terminal.
	if os.Getenv("ORQ_WORKER_ID") == "" {
		if _, err := tokenInicial(); err != nil {
			fmt.Fprintln(stderr, "orq init:", err)
		} else {
			fmt.Fprintf(stdout, "owner token: %s (privileged actions need export ORQ_TOKEN=\"$(orq token)\")\n", identidad.Path())
		}
	}
	return runSkills(nil, stdout, stderr)
}

func tail(s string, n int) string {
	l := strings.Split(strings.TrimSpace(s), "\n")
	return strings.Join(l[max(0, len(l)-n):], "\n")
}

// ---------- fase ----------

// fase deduce dónde está el trabajo, sin estado extra: canal, memoria y slice.
func (p proyecto) fase() (string, string) {
	ab, _ := p.slices().Abiertos()
	if len(ab) > 1 {
		var xs []string
		for _, e := range ab {
			xs = append(xs, e.Attempt()+" "+e.Fase)
		}
		return "D·Desarrollo", strings.Join(xs, " · ") + " (usa --slice NN)"
	}
	if len(ab) == 1 {
		e := ab[0]
		switch e.Fase {
		case slice.Verificado:
			return "D2·Documentación", e.Attempt() + " verificado: demo con el Owner y cierre"
		case slice.Integrando, slice.Cerrando:
			return "D2·Documentación", e.Attempt() + " cierre a medias: repite `orq slice cerrar --aceptado`"
		case slice.Bloqueado:
			return "D1·Desarrollo", e.Attempt() + " BLOQUEADO (" + e.Motivo + "): decide reintentar o rechazar"
		}
		return "D1·Desarrollo", e.Attempt() + " en rojo: un escritor con TDD, scouts si falta contexto"
	}
	ms, _ := canal.Load(p.canalPath())
	ms = canal.Vigentes(ms)
	historia := false
	for _, m := range ms {
		historia = historia || m.Tipo == "historia"
	}
	if !historia {
		return "T·Traducción", "el Atena entiende con el Owner qué y para qué"
	}
	nn := "S-" + p.slices().Siguiente()
	items, _ := memoria.Load(p.memPath())
	for _, it := range items {
		if it.Slice == nn && it.Fuente == "scout" {
			return "A·Análisis", nn + ": plan corto y examen en rojo"
		}
	}
	return "C·Contexto", nn + ": scouts mapean el proyecto antes de decidir"
}

func runFase(stdout io.Writer, stderr io.Writer) int {
	p, err := abrirProyecto()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	f, d := p.fase()
	fmt.Fprintf(stdout, "%s · %s\n", f, d)
	return 0
}

// ---------- contexto (hook SessionStart) ----------

func runContexto(stdout, stderr io.Writer) int {
	p, err := abrirProyecto()
	if err != nil {
		return 0 // fuera de un repo el hook no aporta nada y no debe estorbar
	}
	if _, err := os.Stat(p.perfilPath()); err != nil {
		return 0 // proyecto sin orq init
	}
	rol := p.rolActual()
	pf := p.perfil()
	f, d := p.fase()
	fmt.Fprintf(stdout, "# orq · %s\nRol: %s · modo: %s · fase: %s (%s)\n", filepath.Base(p.root), def(rol, "sin rol"), pf.Modo, f, d)
	if rol != "" {
		fmt.Fprintf(stdout, "Tu guía y tus skills: `orq skill %s`\n", rol)
		fmt.Fprint(stdout, p.roleSkills(rol))
		if raw, err := os.ReadFile(p.relevo(rol)); err == nil {
			fmt.Fprintf(stdout, "\n%s\n", strings.TrimSpace(string(raw)))
		}
	}
	// Presupuesto de memoria: lo crítico, luego el slice activo, luego el resto; nunca más de 25 líneas.
	activo := ""
	if e, err := p.slices().Load(); err == nil && e != nil {
		activo = e.ID()
	}
	if sel, fuera := memoria.Presupuesto(p.memVigente(), activo, presupuestoMem); len(sel) > 0 {
		txt := strings.TrimSpace(memoria.Render(sel, ""))
		if fuera > 0 {
			txt += fmt.Sprintf("\n… %d más: orq mem ver", fuera)
		}
		fmt.Fprintf(stdout, "\n## Memoria\n%s\n", txt)
	}
	fmt.Fprint(stdout, p.lecciones())
	if _, err := os.Stat(p.fichaPath()); err == nil {
		fmt.Fprintf(stdout, "\nFicha del proyecto: %s\n", p.fichaPath())
	}
	return 0
}

const presupuestoMem = 25

func def(s, d string) string {
	if s == "" {
		return d
	}
	return s
}

// ---------- doc (fase D2, modelo barato) ----------

const lineasFicha = 60

func (p proyecto) escribirFicha() error {
	items, _ := memoria.Load(p.memPath())
	var keep []memoria.Item
	for _, it := range memoria.Vigentes(items) {
		if it.Cat != "riesgo" || it.Imp != "baja" {
			keep = append(keep, it)
		}
	}
	files, _ := exec.Command("git", "-C", p.root, "ls-files").Output()
	fl := strings.Split(strings.TrimSpace(string(files)), "\n")
	if len(fl) > 80 {
		fl = fl[:80]
	}
	slices, _ := os.ReadFile(filepath.Join(p.dir, "slices.jsonl"))
	pf, _ := json.Marshal(p.perfil())
	md, err := escriba.Doc(string(pf), strings.Join(fl, ", "), memoria.Render(keep, ""), string(slices), lineasFicha)
	if err != nil {
		return err
	}
	head := fmt.Sprintf("# %s · ficha viva\n_Generada por orq el %s con un modelo barato. No editar a mano: corrige la memoria._\n\n", filepath.Base(p.root), time.Now().Format("2006-01-02 15:04"))
	return orqdir.WriteAtomic(p.fichaPath(), []byte(head+md))
}

func runDoc(stdout, stderr io.Writer) int {
	p, err := abrirProyecto()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := p.escribirFicha(); err != nil {
		fmt.Fprintln(stderr, "orq doc:", err)
		return 1
	}
	fmt.Fprintf(stdout, "ficha actualizada: %s\n", p.fichaPath())
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
	r := p.slices()
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
	out, _ := exec.Command("git", "-C", p.root, "branch", "--list", "slice/*", "--merged").Output()
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
				fmt.Fprintf(stdout, "en cuarentena: %s\n", lineaCuarentena(l))
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
	ses, _ := canal.LoadSesiones(p.sesPath())
	if rn().Inside() {
		for rol, s := range ses {
			if !rn().Alive(s.Agente) {
				problemas++
				fmt.Fprintf(stdout, "la sesión de %s (%s) ya no existe: orq relevo %s\n", rol, s.Agente, rol)
			}
		}
	}
	items, _ := memoria.Load(p.memPath())
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

// lineaCuarentena: una entrada de cuarentena.jsonl como texto legible en el idioma de orq (las
// claves JSON del archivo son internas). Si no es JSON, la línea tal cual.
func lineaCuarentena(l string) string {
	var c struct {
		Ruta, Rama, Slice, Motivo string
	}
	if json.Unmarshal([]byte(l), &c) != nil || (c.Ruta == "" && c.Rama == "") {
		return l
	}
	que := c.Ruta
	if que == "" {
		que = lang.T("branch ", "rama ") + c.Rama
	}
	motivo := c.Motivo
	if !lang.ES() {
		if r, ok := strings.CutPrefix(motivo, "no está integrada en "); ok {
			motivo = "not merged into " + r
		}
	}
	return fmt.Sprintf("%s (%s): %s", que, c.Slice, motivo)
}
