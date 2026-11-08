// Package agentes describe los CLIs de agentes como DATOS (orq.agents.json) y elige agente y
// modelo por rol. Las reglas que no son datos viven aquí: el adversario es de otra familia que el
// planificador, el verificador arranca en sesión nueva y docs usa el modelo más barato.
package agentes

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"orq/internal/runner"
)

//go:embed builtin.json
var builtin []byte

// Roles conocidos. "worker" se elige por riesgo (low|medium|high).
var Roles = []string{"atena", "main", "scout", "planner", "adversary", "worker", "verifier", "docs"}

// CLIs que `orq config agents detect` busca en el PATH.
var Detectables = []string{"claude", "codex", "cursor-agent", "gemini", "opencode", "aider"}

type Eleccion struct {
	Agent  string `json:"agent,omitempty"`
	Model  string `json:"model,omitempty"`
	Effort string `json:"effort,omitempty"`
	Fresh  bool   `json:"fresh,omitempty"`
}

type Agente struct {
	CLI        string              `json:"cli"`
	Kind       string              `json:"kind,omitempty"`        // tipo para herdr
	Family     string              `json:"family"`                // proveedor del modelo
	Hooks      string              `json:"hooks,omitempty"`       // "claude" = pasa por orq gate; "none" = post-only
	Args       []string            `json:"args"`                  // interactivo: {model} {effort} {name}
	WorkerArgs []string            `json:"worker_args,omitempty"` // + para workers y roles: {git_common}
	RoleArgs   []string            `json:"role_args,omitempty"`   // + para roles: {parent}
	Headless   []string            `json:"headless,omitempty"`    // sin TTY: {model} {prompt}
	Prompt     string              `json:"prompt,omitempty"`      // stdin | arg
	Models     []string            `json:"models,omitempty"`
	Default    string              `json:"default,omitempty"`
	Cheapest   string              `json:"cheapest,omitempty"`
	ByRisk     map[string]Eleccion `json:"by_risk,omitempty"`
}

func (a Agente) TieneHooks() bool { return a.Hooks == "claude" }

type capa struct {
	Runner string                     `json:"runner,omitempty"`
	Agents map[string]Agente          `json:"agents,omitempty"`
	Roles  map[string]json.RawMessage `json:"roles,omitempty"`
}

type Config struct {
	Runner string
	Agents map[string]Agente
	roles  map[string]Eleccion // incluye worker.low|medium|high
}

// GlobalPath: ~/.config/orq/agents.json (o ORQ_AGENTS_GLOBAL).
func GlobalPath() string {
	if p := os.Getenv("ORQ_AGENTS_GLOBAL"); p != "" {
		return p
	}
	d, _ := os.UserConfigDir()
	return filepath.Join(d, "orq", "agents.json")
}

// Load superpone: builtin < propuesta de detect < global < proyecto. Global y proyecto son del
// Owner y siempre ganan a la propuesta.
func Load(global, propuesta, proyecto string) (*Config, error) {
	c := &Config{Agents: map[string]Agente{}, roles: map[string]Eleccion{}}
	if err := c.aplicar(builtin); err != nil {
		return nil, fmt.Errorf("agentes builtin: %v", err)
	}
	for _, p := range []string{propuesta, global, proyecto} {
		raw, err := os.ReadFile(p)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if err := c.aplicar(raw); err != nil {
			return nil, fmt.Errorf("%s: %v", p, err)
		}
	}
	return c, c.Validar()
}

func encima(base, e Eleccion) Eleccion {
	if e.Agent != "" {
		if e.Agent != base.Agent && e.Model == "" {
			base.Model, base.Effort = "", ""
		}
		base.Agent = e.Agent
	}
	if e.Model != "" {
		base.Model = e.Model
	}
	if e.Effort != "" {
		base.Effort = e.Effort
	}
	base.Fresh = base.Fresh || e.Fresh
	return base
}

func (c *Config) aplicar(raw []byte) error {
	var k capa
	if err := json.Unmarshal(raw, &k); err != nil {
		return err
	}
	if k.Runner != "" {
		c.Runner = k.Runner
	}
	for n, a := range k.Agents { // una entrada de agente reemplaza entera a la anterior
		c.Agents[n] = a
	}
	for rol, r := range k.Roles {
		if !slices.Contains(Roles, rol) {
			return fmt.Errorf("rol desconocido %q (roles: %s)", rol, strings.Join(Roles, ", "))
		}
		if rol == "worker" {
			var w struct {
				ByRisk map[string]Eleccion `json:"by_risk"`
			}
			if err := json.Unmarshal(r, &w); err != nil {
				return fmt.Errorf("roles.worker: %v", err)
			}
			for risk, e := range w.ByRisk {
				c.roles["worker."+risk] = encima(c.roles["worker."+risk], e)
			}
			continue
		}
		var e Eleccion
		if err := json.Unmarshal(r, &e); err != nil {
			return fmt.Errorf("roles.%s: %v", rol, err)
		}
		c.roles[rol] = encima(c.roles[rol], e)
	}
	return nil
}

// Validar aplica las reglas que no se dejan a los datos.
func (c *Config) Validar() error {
	var errs []error
	ks := make([]string, 0, len(c.roles))
	for k := range c.roles {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	for _, k := range ks {
		if _, ok := c.Agents[c.roles[k].Agent]; !ok {
			errs = append(errs, fmt.Errorf("rol %s: agente %q no está definido en agents", k, c.roles[k].Agent))
		}
	}
	pl, ad := c.roles["planner"], c.roles["adversary"]
	if f := c.Agents[pl.Agent].Family; f != "" && f == c.Agents[ad.Agent].Family {
		errs = append(errs, fmt.Errorf("el adversario (%s) debe ser de otra familia que el planificador (%s): ambos son %s", ad.Agent, pl.Agent, f))
	}
	return errors.Join(errs...)
}

// Rol da la elección efectiva de un rol con sus reglas.
func (c *Config) Rol(rol string) Eleccion {
	e := c.roles[rol]
	switch rol {
	case "verifier":
		e.Fresh = true
	case "docs":
		if m := c.barato(e.Agent); m != "" {
			e.Model = m
		}
	}
	return e
}

func (c *Config) barato(agente string) string {
	a := c.Agents[agente]
	if a.Cheapest != "" {
		return a.Cheapest
	}
	if len(a.Models) > 0 {
		return a.Models[0]
	}
	return ""
}

// Riesgo traduce la dificultad del plan (baja|media|alta) a low|medium|high.
func Riesgo(d string) string {
	switch d {
	case "baja", "low":
		return "low"
	case "alta", "high":
		return "high"
	}
	return "medium"
}

// Worker elige agente y modelo por riesgo. Con agente forzado, usa la elección del rol si es ese
// agente; si no, la tabla por riesgo del propio agente; si no, su modelo por defecto.
func (c *Config) Worker(agente, dificultad string) (Eleccion, error) {
	risk := Riesgo(dificultad)
	e := c.roles["worker."+risk]
	if agente == "" || agente == e.Agent {
		return e, nil
	}
	return c.Para(agente, risk)
}

// Para da la elección de un agente concreto (por ejemplo, --kind codex en un rol).
func (c *Config) Para(agente, risk string) (Eleccion, error) {
	a, ok := c.Agents[agente]
	if !ok {
		var ks []string
		for k := range c.Agents {
			ks = append(ks, k)
		}
		sort.Strings(ks)
		return Eleccion{}, fmt.Errorf("agente %q no configurado (hay: %s; ver orq config agents show)", agente, strings.Join(ks, ", "))
	}
	if r, ok := a.ByRisk[Riesgo(risk)]; ok {
		r.Agent = agente
		return r, nil
	}
	return Eleccion{Agent: agente, Model: a.Default}, nil
}

// Contexto llena las variables de las plantillas que no son de la elección.
type Contexto struct {
	GitCommon string // {git_common}
	Parent    string // {parent}
}

func sustituir(tpl []string, v map[string]string) []string {
	var out []string
	for _, x := range tpl {
		vacio := false
		for k, val := range v {
			if strings.Contains(x, "{"+k+"}") {
				if val == "" {
					vacio = true
				}
				x = strings.ReplaceAll(x, "{"+k+"}", val)
			}
		}
		if vacio { // sin valor no va el argumento, ni la bandera que lo precede
			if n := len(out); n > 0 && strings.HasPrefix(out[n-1], "-") {
				out = out[:n-1]
			}
			continue
		}
		out = append(out, x)
	}
	return out
}

// Launch arma el arranque a partir de los datos. uso: "worker" | "role" | "" (solo args).
func (c *Config) Launch(e Eleccion, name string, ctx Contexto, uso string) (runner.Launch, error) {
	a, ok := c.Agents[e.Agent]
	if !ok {
		return runner.Launch{}, fmt.Errorf("agente %q no configurado", e.Agent)
	}
	v := map[string]string{"model": e.Model, "effort": e.Effort, "name": name, "git_common": ctx.GitCommon, "parent": ctx.Parent}
	tpl := slices.Clone(a.Args)
	if uso == "worker" || uso == "role" {
		tpl = append(tpl, a.WorkerArgs...)
	}
	if uso == "role" {
		tpl = append(tpl, a.RoleArgs...)
	}
	hv := map[string]string{"model": e.Model, "effort": e.Effort, "name": name}
	kind := a.Kind
	if kind == "" {
		kind = a.CLI
	}
	return runner.Launch{Kind: kind, CLI: a.CLI, Args: sustituir(tpl, v), Headless: sustituir(a.Headless, hv), PromptMode: a.Prompt}, nil
}

// Propuesta es lo que detect sugiere; se guarda aparte y el Owner siempre gana.
type Propuesta struct {
	Instalados []string            `json:"installed"`
	Roles      map[string]Eleccion `json:"roles"`
}

// Detect propone agentes instalados para los roles cuyo agente no está en el PATH, y un
// adversario de otra familia que el planificador.
func (c *Config) Detect(hay func(cli string) bool) Propuesta {
	p := Propuesta{Roles: map[string]Eleccion{}}
	inst := map[string]bool{}
	for _, n := range Detectables {
		if a, ok := c.Agents[n]; ok && hay(a.CLI) {
			p.Instalados = append(p.Instalados, n)
			inst[n] = true
		}
	}
	if len(p.Instalados) == 0 {
		return p
	}
	primero := p.Instalados[0]
	for _, rol := range []string{"atena", "main", "scout", "planner", "verifier", "docs"} {
		if e := c.roles[rol]; !inst[e.Agent] {
			p.Roles[rol] = Eleccion{Agent: primero, Model: c.Agents[primero].Default}
		}
	}
	pl := c.roles["planner"].Agent
	if x, ok := p.Roles["planner"]; ok {
		pl = x.Agent
	}
	ad := c.roles["adversary"].Agent
	if !inst[ad] || c.Agents[ad].Family == c.Agents[pl].Family {
		for _, n := range p.Instalados {
			if c.Agents[n].Family != c.Agents[pl].Family {
				p.Roles["adversary"] = Eleccion{Agent: n, Model: c.Agents[n].Default, Effort: "high"}
				break
			}
		}
	}
	return p
}

// Set cambia un rol (worker.low|medium|high para workers) en memoria y valida las reglas.
func (c *Config) Set(rol, agente, modelo string) error {
	base := strings.SplitN(rol, ".", 2)[0]
	if !slices.Contains(Roles, base) || (base == "worker") != strings.Contains(rol, ".") {
		return fmt.Errorf("rol %q: usa uno de %s (worker.low|worker.medium|worker.high)", rol, strings.Join(Roles, ", "))
	}
	if _, ok := c.Agents[agente]; !ok {
		return fmt.Errorf("agente %q no configurado", agente)
	}
	antes := c.roles[rol]
	c.roles[rol] = encima(antes, Eleccion{Agent: agente, Model: modelo})
	if err := c.Validar(); err != nil {
		c.roles[rol] = antes
		return err
	}
	return nil
}

// SetEnArchivo escribe la elección del Owner en el archivo del proyecto, conservando lo demás.
func SetEnArchivo(path, rol, agente, modelo string) error {
	m := map[string]any{}
	if raw, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(raw, &m); err != nil {
			return fmt.Errorf("%s: %v", path, err)
		}
	}
	roles, _ := m["roles"].(map[string]any)
	if roles == nil {
		roles = map[string]any{}
	}
	e := map[string]any{"agent": agente, "model": modelo}
	if base, risk, ok := strings.Cut(rol, "."); ok {
		w, _ := roles[base].(map[string]any)
		if w == nil {
			w = map[string]any{}
		}
		br, _ := w["by_risk"].(map[string]any)
		if br == nil {
			br = map[string]any{}
		}
		br[risk] = e
		w["by_risk"] = br
		roles[base] = w
	} else {
		roles[rol] = e
	}
	m["roles"] = roles
	raw, _ := json.MarshalIndent(m, "", "  ")
	return os.WriteFile(path, append(raw, '\n'), 0o644)
}

// Mostrar da las elecciones efectivas, una por línea.
func (c *Config) Mostrar() []string {
	var out []string
	for _, rol := range Roles {
		if rol == "worker" {
			for _, r := range []string{"low", "medium", "high"} {
				e := c.roles["worker."+r]
				out = append(out, fmt.Sprintf("%-16s %s:%s %s", "worker."+r, e.Agent, e.Model, e.Effort))
			}
			continue
		}
		e := c.Rol(rol)
		extra := ""
		if e.Fresh {
			extra = " (sesión nueva)"
		}
		a := c.Agents[e.Agent]
		gate := "hook"
		if !a.TieneHooks() {
			gate = "post-only"
		}
		out = append(out, fmt.Sprintf("%-16s %s:%s %s · familia %s · gate %s%s", rol, e.Agent, e.Model, e.Effort, a.Family, gate, extra))
	}
	return out
}
