package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"orq/internal/agentes"
	"orq/internal/escriba"
	"orq/internal/orqdir"
	"orq/internal/runner"
)

// ---------- runner y agentes ----------

// rn es el runner elegido: ORQ_RUNNER, luego "runner" de orq.agents.json; por defecto herdr si
// estamos dentro o está instalado, si no un proceso hijo con log.
var rn = func() runner.Runner {
	cfg := ""
	if p, err := abrirProyecto(); err == nil {
		if c, err := p.agentes(); err == nil {
			cfg = c.Runner
		}
	}
	return runner.Select(cfg)
}

func (p proyecto) agentesPath() string   { return filepath.Join(p.root, "orq.agents.json") }
func (p proyecto) propuestaPath() string { return filepath.Join(p.dir, "agents.propuesta.json") }

// agentes carga builtin < propuesta de detect < ~/.config/orq/agents.json < orq.agents.json.
func (p proyecto) agentes() (*agentes.Config, error) {
	c, err := agentes.Load(agentes.GlobalPath(), p.propuestaPath(), p.agentesPath())
	if err != nil {
		return nil, fmt.Errorf("orq.agents.json: %v", err)
	}
	if d := c.Rol("docs"); d.Agent == "claude" && d.Model != "" {
		escriba.ModeloDocs = d.Model
	}
	return c, nil
}

// rolCfg traduce los roles de sesión a los roles de orq.agents.json.
var rolCfg = map[string]string{"orquestador": "main", "planificador": "planner", "astra": "adversary", "atena": "atena"}

// eleccion: el rol manda; --kind/--modelo/--esfuerzo del Owner ganan. Astra (adversario) nunca
// es de la misma familia que el planificador.
func (a *arranque) eleccion(c *agentes.Config, rol string) (agentes.Eleccion, error) {
	e := c.Rol(rolCfg[rol])
	if a.kind != "" && a.kind != e.Agent {
		ag, ok := c.Agents[a.kind]
		if !ok {
			_, err := c.Para(a.kind, "")
			return e, err
		}
		e = agentes.Eleccion{Agent: a.kind, Model: ag.Default, Effort: e.Effort}
	}
	if a.modelo != "" {
		e.Model = a.modelo
	}
	if a.esfuerzo != "" {
		e.Effort = a.esfuerzo
	}
	if rol == "astra" {
		pl := c.Rol("planner").Agent
		if f := c.Agents[e.Agent].Family; f == c.Agents[pl].Family {
			return e, fmt.Errorf("Astra (%s) debe ser de otra familia que el Planificador (%s): ambos son %s", e.Agent, pl, f)
		}
	}
	a.kind, a.modelo, a.esfuerzo = e.Agent, e.Model, e.Effort
	return e, nil
}

func gitCommon(dir string) string {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--path-format=absolute", "--git-common-dir").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func gateDe(c *agentes.Config, agente string) string {
	if c.Agents[agente].TieneHooks() {
		return "hook"
	}
	return "post-only"
}

// ---------- H-06: el gate presente donde trabaja el worker ----------

// hookEn: ¿la configuración efectiva de Claude en dir llama a `orq gate`?
func hookEn(dir string) bool {
	for _, f := range []string{"settings.json", "settings.local.json"} {
		if raw, err := os.ReadFile(filepath.Join(dir, ".claude", f)); err == nil && strings.Contains(string(raw), "orq gate") {
			return true
		}
	}
	return false
}

// asegurarHook deja el hook del gate en el worktree del worker: si falta, copia el
// .claude/settings.json del proyecto. Sin hook, un worker Claude no se lanza.
func asegurarHook(dir, root string) error {
	if hookEn(dir) {
		return nil
	}
	src := filepath.Join(root, ".claude", "settings.json")
	raw, err := os.ReadFile(src)
	if err != nil || !strings.Contains(string(raw), "orq gate") {
		return fmt.Errorf("el proyecto no tiene el hook del gate en %s: corre `orq init` antes de lanzar workers Claude", src)
	}
	dst := filepath.Join(dir, ".claude", "settings.json")
	if _, err := os.Stat(dst); err == nil {
		return fmt.Errorf("%s existe pero no llama a `orq gate`: une el bloque hooks de `orq init --hooks`", dst)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if err := orqdir.WriteAtomic(dst, raw); err != nil {
		return err
	}
	if !hookEn(dir) {
		return errors.New("no se pudo dejar el hook del gate en " + dir)
	}
	return nil
}

// protegerExamen quita la escritura a los archivos del examen en el worktree del worker: guardia del
// sistema operativo para agentes sin hooks. Solo orq la devuelve (restaurarExamen).
func protegerExamen(dir string, bundle []string) error {
	var errs []error
	for _, f := range bundle {
		p := filepath.Join(dir, f)
		st, err := os.Stat(p)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			errs = append(errs, err)
			continue
		}
		errs = append(errs, os.Chmod(p, st.Mode().Perm()&^0o222))
	}
	return errors.Join(errs...)
}

func restaurarExamen(dir string, bundle []string) {
	for _, f := range bundle {
		p := filepath.Join(dir, f)
		if st, err := os.Stat(p); err == nil {
			os.Chmod(p, st.Mode().Perm()|0o200)
		}
	}
}

// ---------- orq config agents ----------

var buscarCLI = func(cli string) bool { _, err := exec.LookPath(cli); return err == nil }

func runConfig(args []string, stdout, stderr io.Writer) int {
	uso := "uso: orq config agents [show | detect | set ROL AGENTE:MODELO]   (ROL worker.low|worker.medium|worker.high para workers)"
	if len(args) == 0 || args[0] != "agents" {
		fmt.Fprintln(stderr, uso)
		return 1
	}
	p, err := abrirProyecto()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	sub := "show"
	if len(args) > 1 {
		sub = args[1]
	}
	c, err := p.agentes()
	if err != nil && sub != "set" {
		fmt.Fprintln(stderr, err)
		return 1
	}
	switch sub {
	case "show":
		fmt.Fprintf(stdout, "runner: %s\n", rn().Name())
		for _, l := range c.Mostrar() {
			fmt.Fprintln(stdout, l)
		}
		fmt.Fprintf(stdout, "capas: builtin < %s < %s < %s (el Owner gana)\n", p.propuestaPath(), agentes.GlobalPath(), p.agentesPath())
		return 0
	case "detect":
		pr := c.Detect(buscarCLI)
		if len(pr.Instalados) == 0 {
			fmt.Fprintln(stderr, "ningún CLI de agente en el PATH ("+strings.Join(agentes.Detectables, ", ")+")")
			return 1
		}
		raw, _ := json.MarshalIndent(map[string]any{"installed": pr.Instalados, "roles": pr.Roles}, "", "  ")
		if err := orqdir.WriteAtomic(p.propuestaPath(), raw); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintf(stdout, "instalados: %s\n", strings.Join(pr.Instalados, ", "))
		for rol, e := range pr.Roles {
			fmt.Fprintf(stdout, "propuesta %s → %s:%s\n", rol, e.Agent, e.Model)
		}
		fmt.Fprintf(stdout, "propuesta en %s; lo que fijes en %s o %s siempre gana\n", p.propuestaPath(), p.agentesPath(), agentes.GlobalPath())
		return 0
	case "set":
		if len(args) != 4 || !strings.Contains(args[3], ":") {
			fmt.Fprintln(stderr, uso)
			return 1
		}
		if c == nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		ag, mod, _ := strings.Cut(args[3], ":")
		if err := c.Set(args[2], ag, mod); err != nil {
			fmt.Fprintln(stderr, "orq config:", err)
			return 1
		}
		if err := agentes.SetEnArchivo(p.agentesPath(), args[2], ag, mod); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintf(stdout, "%s → %s:%s en %s\n", args[2], ag, mod, p.agentesPath())
		return 0
	}
	fmt.Fprintln(stderr, uso)
	return 1
}
