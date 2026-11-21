package app

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"sort"
	"strings"

	"orq/internal/canal"
	"orq/internal/diario"
	"orq/internal/identidad"
	"orq/internal/orqdir"
	"orq/internal/terminal"
)

// Layout es la distribución de pestañas, siempre igual y con nombre:
//
//	pestaña del Owner      → atena-<proyecto>
//	«orq · <proyecto>»     → orquestador-<proyecto>
//	«plan · <proyecto>»    → planificador-<proyecto> | astra-<proyecto>
//	«S-NN · <proyecto>»    → worker-<zona>-sNN (uno al lado del otro)
const Layout = `atena-P (Owner's tab) · "orq · P" orquestador-P · "plan · P" planificador-P | astra-P · "S-NN · P" worker-<zone>-sNN`

// ErrFueraDelTerminal: el comando necesita correr en un panel del multiplexor configurado.
var ErrFueraDelTerminal = errors.New("fuera del terminal configurado")

// Dentro comprueba que el comando corre en un panel del multiplexor configurado.
func (p Proyecto) Dentro() error {
	if p.Term.Dentro() {
		return nil
	}
	return fmt.Errorf("%w: corre esto desde un panel de %s (terminal en `orq config ver`)", ErrFueraDelTerminal, p.Term.Nombre())
}

// sesionNueva es lo que hace falta para abrir un agente en su panel.
type sesionNueva struct {
	Nombre   string
	Kind     string
	Args     []string
	Env      map[string]string
	Cwd      string
	Etiqueta string // pestaña nueva
	Junto    string // o al lado de este panel
	Slice    string
	Intento  int
}

// abrirSesion crea el panel y arranca el agente como mucho una vez, y lo deja escrito en el diario:
// intento → panel → arranque. Repetir el comando converge: si el agente ya existe se adopta su panel;
// si el arranque quedó desconocido no se lanza otro hasta saber.
func (p Proyecto) abrirSesion(ctx context.Context, s sesionNueva) (terminal.Panel, terminal.Resultado, error) {
	evs, err := diario.Leer(p.DiarioPath())
	if err != nil {
		return terminal.Panel{}, 0, err
	}
	previo, hayPrevio := diario.Ultimo(evs, func(e diario.Evento) bool {
		return e.Tipo == "lanzamiento" && e.Paso == "panel" && e.Actor == s.Nombre
	})
	panelPrevio := terminal.Panel{}
	if hayPrevio {
		panelPrevio.ID, _ = previo.Datos["panel"].(string)
		panelPrevio.Pestana, _ = previo.Datos["pestana"].(string)
	}
	vida, err := p.Term.Estado(ctx, s.Nombre)
	if err != nil && !errors.Is(err, terminal.ErrNoExiste) || vida == terminal.Desconocida {
		return panelPrevio, 0, fmt.Errorf("no se sabe si %s ya existe en %s (%w); no se lanza otro. Revisa el panel %s y repite", s.Nombre, p.Term.Nombre(), errors.Join(terminal.ErrDesconocido, err), def(panelPrevio.ID, "del agente"))
	}
	op := diario.NuevaOp()
	base := diario.Evento{Op: op, Tipo: "lanzamiento", Actor: s.Nombre, Slice: s.Slice, Intento: s.Intento}
	if vida == terminal.Vivo {
		if panelPrevio.ID == "" {
			return panelPrevio, 0, fmt.Errorf("ya existe un agente %s en %s que orq no lanzó; ciérralo o renómbralo antes de lanzar", s.Nombre, p.Term.Nombre())
		}
		ev := base
		ev.Paso, ev.Resultado, ev.Datos = "arranque", "ya_estaba", map[string]any{"panel": panelPrevio.ID}
		return panelPrevio, terminal.YaEstaba, p.anotar(ev)
	}
	ev := base
	ev.Paso, ev.Datos = "intento", map[string]any{"kind": s.Kind, "cwd": s.Cwd}
	if err := p.anotar(ev); err != nil {
		return terminal.Panel{}, 0, err
	}
	// Un panel de un arranque anterior que no llegó a tener agente es un sobrante: se cierra antes de abrir otro.
	if panelPrevio.ID != "" {
		if err := p.Term.Cerrar(ctx, panelPrevio.ID); err != nil {
			return panelPrevio, 0, fmt.Errorf("quedó el panel %s de un arranque anterior y no se pudo cerrar: %w", panelPrevio.ID, err)
		}
	}
	panel, err := p.Term.CrearPanel(ctx, terminal.PanelNuevo{Etiqueta: s.Etiqueta, Junto: s.Junto, Cwd: s.Cwd, Env: s.Env})
	if err != nil {
		ev = base
		ev.Paso, ev.Resultado, ev.Detalle = "panel", resultadoDe(err), err.Error()
		return panel, 0, errors.Join(err, p.anotar(ev))
	}
	if panel.Pestana == "" {
		panel.Pestana = panelPrevio.Pestana
	}
	ev = base
	ev.Paso, ev.Resultado, ev.Datos = "panel", "ok", map[string]any{"panel": panel.ID, "pestana": panel.Pestana}
	if err := p.anotar(ev); err != nil {
		return panel, 0, err
	}
	res, err := terminal.ArrancarSeguro(ctx, p.Term, terminal.Arranque{Nombre: s.Nombre, Kind: s.Kind, Panel: panel.ID, Args: s.Args, Env: s.Env, Cwd: s.Cwd})
	ev = base
	ev.Paso, ev.Resultado, ev.Datos = "arranque", resultadoDe(err), map[string]any{"panel": panel.ID}
	if err != nil {
		ev.Detalle = err.Error()
		if errors.Is(err, terminal.ErrDesconocido) {
			// No se cierra: el agente pudo arrancar. El próximo intento lo resuelve con Estado.
			return panel, 0, errors.Join(fmt.Errorf("%w (el panel %s queda abierto: míralo y repite el comando)", err, panel.ID), p.anotar(ev))
		}
		// Fallo claro: sin agente el panel no sirve y se cierra.
		if errC := p.Term.Cerrar(ctx, panel.ID); errC != nil {
			return panel, 0, errors.Join(fmt.Errorf("%w (el panel %s quedó abierto; ciérralo)", err, panel.ID), p.anotar(ev))
		}
		return panel, 0, errors.Join(fmt.Errorf("%w (panel %s cerrado)", err, panel.ID), p.anotar(ev))
	}
	return panel, res, p.anotar(ev)
}

func resultadoDe(err error) string {
	switch {
	case err == nil:
		return "ok"
	case errors.Is(err, terminal.ErrDesconocido):
		return "desconocido"
	}
	return "fallo"
}

func def(s, d string) string {
	if s == "" {
		return d
	}
	return s
}

// avisarUnaVez entrega un texto ligado a una operación (el arranque de un rol, el contrato de un worker)
// solo si el diario no dice que ya se entregó. Un envío sin respuesta queda como desconocido y no se repite solo.
// In orq2 the delivery belongs to the pane it went to: a worker relaunched in a new pane gets its own
// contract, and two deliveries at once take the same lock, so the second sees the first.
func (p Proyecto) avisarUnaVez(ctx context.Context, actor, paso, destino, panel, texto string, slice string, intento int) (yaEstaba bool, err error) {
	orq2 := identidad.EsOrq2()
	if orq2 {
		soltar, err := orqdir.Candado(filepath.Join(p.Dir, "avisos"))
		if err != nil {
			return false, err
		}
		defer soltar()
	}
	evs, err := diario.Leer(p.DiarioPath())
	if err != nil {
		return false, err
	}
	mismo := func(e diario.Evento) bool {
		return e.Tipo == "lanzamiento" && e.Actor == actor &&
			(e.Paso == paso && e.Slice == slice && e.Intento == intento || e.Paso == "reenviar")
	}
	if orq2 {
		mismo = func(e diario.Evento) bool {
			if e.Tipo != "lanzamiento" || e.Actor != actor || e.Slice != slice || e.Intento != intento || !mismoPanel(e, panel) {
				return false
			}
			return e.Paso == paso || e.Paso == "reenviar" && e.Datos["paso"] == paso
		}
	}
	if u, ok := diario.Ultimo(evs, mismo); ok {
		switch u.Resultado {
		case "ok":
			return true, nil
		case "desconocido":
			return false, fmt.Errorf("%w: el %s de %s pudo llegar o no en un envío anterior. Mira su panel; si no llegó, `orq sesion reenviar %s` y repite el comando", terminal.ErrDesconocido, paso, actor, actor)
		}
	}
	errEnvio := p.Term.Avisar(ctx, destino, texto)
	ev := diario.Evento{Tipo: "lanzamiento", Paso: paso, Actor: actor, Slice: slice, Intento: intento, Resultado: resultadoDe(errEnvio)}
	if orq2 {
		ev.Datos = map[string]any{"panel": panel}
	}
	if errEnvio != nil {
		ev.Detalle = errEnvio.Error()
	}
	return false, errors.Join(errEnvio, p.anotar(ev))
}

// mismoPanel: the event went to this pane. A row written before deliveries carried their pane
// counts for any pane, so an upgrade never re-sends a contract to a live worker.
func mismoPanel(e diario.Evento, panel string) bool {
	v, ok := e.Datos["panel"]
	return !ok || v == panel
}

// pasosDeLanzamiento son los pasos del arranque; los demás son entregas (contrato, arranque del rol).
var pasosDeLanzamiento = map[string]bool{"intento": true, "panel": true, "arranque": true, "reenviar": true}

// Reenviar olvida un envío desconocido para que el próximo comando lo repita. Lo decide quien miró el panel.
// In orq2 it forgets only the actor's last delivery, not every step the actor ever had.
func (p Proyecto) Reenviar(actor string) error {
	if !identidad.EsOrq2() {
		return p.anotar(diario.Evento{Tipo: "lanzamiento", Paso: "reenviar", Actor: actor, Resultado: "reenviar"})
	}
	soltar, err := orqdir.Candado(filepath.Join(p.Dir, "avisos"))
	if err != nil {
		return err
	}
	defer soltar()
	evs, err := diario.Leer(p.DiarioPath())
	if err != nil {
		return err
	}
	u, ok := diario.Ultimo(evs, func(e diario.Evento) bool {
		return e.Tipo == "lanzamiento" && e.Actor == actor && !pasosDeLanzamiento[e.Paso]
	})
	if !ok {
		return fmt.Errorf("%s no tiene ningún envío que repetir", actor)
	}
	datos := map[string]any{"paso": u.Paso}
	if v, ok := u.Datos["panel"]; ok {
		datos["panel"] = v
	}
	return p.anotar(diario.Evento{Tipo: "lanzamiento", Paso: "reenviar", Actor: actor, Slice: u.Slice, Intento: u.Intento, Resultado: "reenviar", Datos: datos})
}

// guardarSesiones applies the change made to ses for the given keys. orq saves the whole map, as
// 1.0.0 did; orq2 writes only those keys, under the lock, on what is on disk now (a key missing from
// ses is deleted), so another writer's sessions are never lost.
func (p Proyecto) guardarSesiones(ses map[string]canal.Sesion, claves ...string) error {
	if !identidad.EsOrq2() {
		return canal.SaveSesiones(p.SesPath(), ses) // orq keeps 1.0.0
	}
	return canal.ActualizarSesiones(p.SesPath(), func(m map[string]canal.Sesion) {
		for _, k := range claves {
			if s, ok := ses[k]; ok {
				m[k] = s
			} else {
				delete(m, k)
			}
		}
	})
}

// CerrarSesiones cierra los paneles que cumplen el filtro y los quita del registro. Atena es del
// Owner y el Orquestador se releva, no se cierra: nunca entran aquí.
func (p Proyecto) CerrarSesiones(ctx context.Context, ses map[string]canal.Sesion, sirve func(string) bool) ([]string, error) {
	if identidad.EsOrq2() {
		// Decide on what is registered now, not on a map read before a long verification.
		fresco, err := canal.LoadSesiones(p.SesPath())
		if err != nil {
			return nil, err
		}
		clear(ses)
		maps.Copy(ses, fresco)
	}
	var cerradas []string
	for k, s := range ses {
		if k == "atena" || k == "orquestador" || !sirve(k) {
			continue
		}
		if s.Pane != "" && p.Term.Dentro() {
			if err := p.Term.Cerrar(ctx, s.Pane); err != nil {
				sort.Strings(cerradas)
				return cerradas, errors.Join(fmt.Errorf("%s: %w", k, err), p.guardarSesiones(ses, cerradas...))
			}
		}
		delete(ses, k)
		cerradas = append(cerradas, k)
	}
	sort.Strings(cerradas)
	return cerradas, p.guardarSesiones(ses, cerradas...)
}

// PanelDelSlice da un panel ya abierto de ese slice: los workers comparten la pestaña «S-NN · P».
func PanelDelSlice(ses map[string]canal.Sesion, nn string) (canal.Sesion, bool) {
	var ks []string
	for k := range ses {
		ks = append(ks, k)
	}
	sort.Strings(ks) // orden estable: el mismo panel en cada llamada
	for _, k := range ks {
		if s := ses[k]; strings.HasPrefix(k, "worker-") && strings.HasSuffix(k, "-s"+nn) && s.Pane != "" {
			return s, true
		}
	}
	return canal.Sesion{}, false
}
