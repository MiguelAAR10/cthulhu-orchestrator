package app

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"orq/internal/agente"
	"orq/internal/canal"
	"orq/internal/config"
	"orq/internal/identidad"
	"orq/internal/orqdir"
	"orq/internal/terminal"
)

// Etiqueta es el nombre tipado de la pestaña de cada rol: «<rol> · <proyecto>».
func (p Proyecto) Etiqueta(rol string) string {
	return map[string]string{"orquestador": "orq", "planificador": "plan", "astra": "plan"}[rol] + " · " + orqdir.Slug(p.Root)
}

const arranqueRol = `You are the %[1]s of orq in this repository. Load skill %[2]s and run "orq skill %[1]s".
Read first: .orq/relevo-%[1]s.md if it exists, then "orq mem ver".
Talk to other roles only via "orq msg send". Then wait for instructions.`

func skillDe(rol string) string {
	n := "orq-orchestrator"
	switch rol {
	case "atena":
		n = "orq-atena"
	case "planificador", "astra":
		n = "orq-planner"
	}
	return identidad.Skills(n)
}

// entornoDe une ORQ_ROL con las variables de cuenta configuradas (CLAUDE_CONFIG_DIR, CODEX_HOME…).
func entornoDe(rol string, cuenta map[string]string) map[string]string {
	env := map[string]string{"ORQ_ROL": rol}
	for k, v := range cuenta {
		env[k] = v
	}
	return env
}

// argsRol: un rol Codex (Astra) escribe rojos y planes en los repos hermanos del producto, así que
// además del sandbox de worker puede escribir en la carpeta que los contiene.
func (p Proyecto) argsRol(ctx context.Context, a config.Agente, nombre string) []string {
	if a.Kind != "codex" {
		return agente.Args(a.Kind, a.Modelo, a.Esfuerzo, nombre)
	}
	return agente.ArgsWorker(a.Kind, a.Modelo, a.Esfuerzo, nombre, p.gitComun(ctx, p.Root), filepath.Dir(p.Root))
}

// LanzarRol abre un panel con el rol, lo nombra y le da el arranque. pedido trae lo que las banderas
// fijaron; lo demás sale de la configuración. Repetirlo tras una interrupción no abre otra sesión.
func (p Proyecto) LanzarRol(ctx context.Context, ses map[string]canal.Sesion, rol string, pedido config.Agente) (canal.Sesion, config.Agente, error) {
	a := p.Cfg.ParaRol(rol, pedido)
	if err := config.ValidarKind(a.Kind); err != nil {
		return canal.Sesion{}, a, err
	}
	gen := ses[rol].Gen + 1
	nombre := rol + "-" + orqdir.Slug(p.Root)
	if gen > 1 {
		nombre = fmt.Sprintf("%s-%d", nombre, gen)
	}
	// Distribución fija: orq en su pestaña, plan en la suya; Astra al lado del Planificador.
	s := sesionNueva{Nombre: nombre, Kind: a.Kind, Args: p.argsRol(ctx, a, nombre), Env: entornoDe(rol, a.Entorno), Cwd: p.Root, Etiqueta: p.Etiqueta(rol)}
	if pl, ok := ses["planificador"]; rol == "astra" && ok && pl.Pane != "" {
		s.Junto, s.Etiqueta = pl.Pane, ""
	}
	panel, _, err := p.abrirSesion(ctx, s)
	if err != nil {
		return canal.Sesion{}, a, err
	}
	if panel.Pestana == "" && s.Junto != "" {
		panel.Pestana = ses["planificador"].Tab
	}
	nueva := canal.Sesion{Agente: nombre, Pane: panel.ID, Tab: panel.Pestana, Desde: p.ahora().UTC().Truncate(time.Second), Gen: gen}
	ses[rol] = nueva
	if err := canal.SaveSesiones(p.SesPath(), ses); err != nil {
		return nueva, a, err
	}
	_, err = p.avisarUnaVez(ctx, nombre, "arranque-rol", nombre, fmt.Sprintf(arranqueRol, rol, skillDe(rol)), "", gen)
	return nueva, a, err
}

// RegistrarLlamador deja al rol que lanza (el Orquestador, que suele vivir en otro repo) como destino
// en las sesiones del repo del slice: así `orq msg send --para orquestador` del worker le llega directo.
// El multiplexor acepta el id del panel como destino.
func RegistrarLlamador(ses map[string]canal.Sesion, rol, pane string, now time.Time) {
	if rol == "" || pane == "" || esWorker(rol) {
		return
	}
	if s, ok := ses[rol]; ok && s.Pane == pane {
		return
	}
	ses[rol] = canal.Sesion{Agente: pane, Pane: pane, Desde: now.UTC().Truncate(time.Second), Gen: ses[rol].Gen}
}

func esWorker(rol string) bool { return len(rol) > 7 && rol[:7] == "worker-" }

// Subir es `orq up`: nombra el panel actual como Atena y abre el Orquestador.
func (p Proyecto) Subir(ctx context.Context, pedido config.Agente) (atena string, orq canal.Sesion, aviso error, err error) {
	if err := p.Dentro(); err != nil {
		return "", canal.Sesion{}, nil, err
	}
	if err := p.Excluir(); err != nil {
		return "", canal.Sesion{}, nil, err
	}
	ses, err := canal.LoadSesiones(p.SesPath())
	if err != nil {
		return "", canal.Sesion{}, nil, err
	}
	if o, ok := ses["orquestador"]; ok {
		return "", o, nil, fmt.Errorf("%w (%s, panel %s). Para una sesión fresca: orq relevo orquestador", ErrYaHay, o.Agente, o.Pane)
	}
	atena = "atena-" + orqdir.Slug(p.Root)
	if errR := p.Term.Renombrar(ctx, p.Term.PanelActual(), atena); errR != nil {
		aviso = fmt.Errorf("no se pudo nombrar este panel como %s (%v)", atena, errR)
	}
	ses["atena"] = canal.Sesion{Agente: atena, Pane: p.Term.PanelActual(), Desde: p.ahora().UTC().Truncate(time.Second), Gen: 1}
	orq, _, err = p.LanzarRol(ctx, ses, "orquestador", pedido)
	return atena, orq, aviso, err
}

// ErrYaHay: el rol ya tiene sesión.
var ErrYaHay = errors.New("ya hay sesión de ese rol")

// SesionViva dice si el agente de una sesión sigue existiendo. Sin respuesta clara, se asume viva:
// es la respuesta que no abre sesiones de más.
func (p Proyecto) SesionViva(ctx context.Context, s canal.Sesion) bool {
	v, err := p.Term.Estado(ctx, s.Agente)
	return err != nil && !errors.Is(err, terminal.ErrNoExiste) || v != terminal.Ausente
}
