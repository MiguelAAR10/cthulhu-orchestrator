package app

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"orq/internal/canal"
	"orq/internal/identidad"
	"orq/internal/memoria"
	"orq/internal/slice"
)

// Relevo son los datos duros que una sesión nueva necesita, sin pasar por un modelo.
type Relevo struct {
	Rol         string   `json:"rol"`
	Objetivo    string   `json:"objetivo"`
	Restriccion []string `json:"restricciones"` // lo crítico del Owner, intacto
	Decisiones  []string `json:"decisiones"`
	Hipotesis   []string `json:"hipotesis,omitempty"`
	Slice       string   `json:"slice,omitempty"` // S-NN/aK · fase
	Worktree    string   `json:"worktree,omitempty"`
	Escritor    string   `json:"escritor,omitempty"`
	Head        string   `json:"head,omitempty"`
	Comprobado  string   `json:"comprobado"`
	Falla       string   `json:"falla,omitempty"`
	Reproducir  string   `json:"reproducir,omitempty"`
	Siguiente   string   `json:"siguiente"`
	Pendientes  []string `json:"mensajes_pendientes,omitempty"`
	Memoria     string   `json:"aviso_memoria,omitempty"` // lo que no se pudo leer de la memoria
	Referencias []string `json:"referencias"`
}

// DatosRelevo reúne lo comprobado del proyecto para el rol.
func (p Proyecto) DatosRelevo(rol string) Relevo {
	rv := Relevo{Rol: rol, Objetivo: "(sin historia en curso)", Comprobado: "nada todavía",
		Referencias: []string{"orq slice estado", "orq slice recibo", "orq mem ver", "orq msg leer"}}
	ms, _ := canal.Load(p.CanalPath())
	for _, m := range ms {
		if m.Tipo == "historia" {
			rv.Objetivo = Recortar(strings.ReplaceAll(m.Texto, "\n", " "), 300) + " (" + m.ID + ")"
		}
	}
	for _, m := range canal.Pendientes(ms) {
		rv.Pendientes = append(rv.Pendientes, fmt.Sprintf("%s %s→%s %s", m.ID, m.De, m.Para, m.Tipo))
	}
	vigente, errMem := p.MemVigente()
	if errMem != nil && identidad.EsOrq2() { // orq keeps 1.0.0: silent
		rv.Memoria = errMem.Error()
	}
	for _, it := range memoria.Sorted(vigente) {
		switch {
		case it.Imp == "critica":
			rv.Restriccion = append(rv.Restriccion, it.Texto)
		case it.TipoEfectivo() == "decision":
			rv.Decisiones = append(rv.Decisiones, it.ID+" "+it.Texto)
		case it.TipoEfectivo() == "hipotesis" && len(rv.Hipotesis) < 5:
			rv.Hipotesis = append(rv.Hipotesis, it.ID+" "+it.Texto)
		}
	}
	ab, _ := p.Slices().Abiertos()
	if len(ab) > 1 {
		var xs []string
		for _, x := range ab {
			xs = append(xs, fmt.Sprintf("%s · %s · %s · escritor %s", x.Attempt(), x.Fase, x.Worktree, Valor(x.Escritor)))
		}
		rv.Slice = strings.Join(xs, " | ")
		rv.Siguiente = "varios slices abiertos: `orq slice estado` y trabaja cada uno con --slice NN"
		return rv
	}
	var e *slice.Estado
	if len(ab) == 1 {
		e = ab[0]
	}
	if e == nil {
		rv.Siguiente = "sin slice abierto: " + strings.Join(func() []string { f, d := p.Fase(); return []string{f, d} }(), " · ")
		return rv
	}
	rv.Slice, rv.Worktree, rv.Escritor = e.Attempt()+" «"+e.Titulo+"» · "+e.Fase, e.Worktree, Valor(e.Escritor)
	if h, err := slice.Git(e.Worktree, "rev-parse", "--short", "HEAD"); err == nil {
		rv.Head = h
	}
	rv.Reproducir = fmt.Sprintf("cd %s && %s", e.Worktree, e.Cmd)
	if e.Recibo != nil {
		rv.Comprobado = fmt.Sprintf("recibo %s en %s: exit %d, %d ejecutadas", e.Recibo.Attempt, slice.Corto(e.Recibo.Head), e.Recibo.Exit, e.Recibo.Ejecutados)
	}
	if e.Motivo != "" {
		rv.Falla = e.Motivo
	} else if e.Fallos > 0 {
		rv.Falla = fmt.Sprintf("%d verificación(es) fallida(s) en %s; salida en .orq/salidas/", e.Fallos, e.Attempt())
	}
	switch e.Fase {
	case slice.Implementando:
		rv.Siguiente = fmt.Sprintf("seguir en el MISMO worktree; escritor %s (un relevo usa `orq slice tomar <quien> --relevo`); luego `orq slice verificar`", Valor(e.Escritor))
	case slice.Verificado:
		rv.Siguiente = "demo con el Owner vía Atena; después `orq slice cerrar --aceptado|--rechazado`"
	case slice.Bloqueado:
		rv.Siguiente = "decidir: `orq slice reintentar --causa worker|examen \"qué cambia\"`, preguntar al Atena o rechazar"
	default:
		rv.Siguiente = "repetir `orq slice cerrar --aceptado`: continúa donde quedó"
	}
	return rv
}

func (rv Relevo) Markdown(fecha, ahora string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Relevo · %s · %s\n\n", rv.Rol, fecha)
	fmt.Fprintf(&b, "## Objetivo\n- %s\n\n## Lo que pide el Owner (crítico, intacto)\n%s\n\n", rv.Objetivo, lista(rv.Restriccion, "(sin requisitos críticos)"))
	fmt.Fprintf(&b, "## Decisiones vigentes\n%s\n\n", lista(rv.Decisiones, "(ninguna)"))
	if rv.Slice != "" {
		fmt.Fprintf(&b, "## Slice\n- %s\n- worktree: %s (no crear otro)\n- escritor: %s · commit: %s\n\n", rv.Slice, rv.Worktree, rv.Escritor, rv.Head)
	}
	fmt.Fprintf(&b, "## Comprobado\n- %s\n\n", rv.Comprobado)
	if rv.Falla != "" {
		fmt.Fprintf(&b, "## Qué falla\n- %s\n- reproducir: `%s`\n\n", rv.Falla, rv.Reproducir)
	}
	if len(rv.Hipotesis) > 0 {
		fmt.Fprintf(&b, "## Hipótesis (sin comprobar: no son reglas)\n%s\n\n", lista(rv.Hipotesis, ""))
	}
	if rv.Memoria != "" {
		fmt.Fprintf(&b, "## Memoria incompleta\n- %s\n\n", rv.Memoria)
	}
	if len(rv.Pendientes) > 0 {
		fmt.Fprintf(&b, "## Mensajes sin entrega confirmada\n%s\n- `orq msg reconciliar`\n\n", lista(rv.Pendientes, ""))
	}
	fmt.Fprintf(&b, "## Siguiente acción\n- %s\n\n", rv.Siguiente)
	if ahora != "" {
		fmt.Fprintf(&b, "## Resumen del escriba (propuesto, verifícalo)\n%s\n\n", ahora)
	}
	fmt.Fprintf(&b, "## Referencias\n- `%s`\n", strings.Join(rv.Referencias, "` · `"))
	return b.String()
}

// Contrato es el Paso C del research: lo mínimo que un worker necesita, con los datos reales del slice.
func (p Proyecto) Contrato(nn, worker, alcance, suite, presupuesto string) (string, error) {
	e, err := p.Slices().Load(nn)
	if err != nil || e == nil {
		return "", errors.Join(err, errors.New("no hay slice abierto"))
	}
	rv := p.DatosRelevo("orquestador")
	rv.Objetivo = strings.TrimSpace(rv.Objetivo)
	if worker == "" {
		worker = Valor(e.Escritor)
	}
	var crit []string
	vigente, _ := p.MemVigente() // its read problem is already in rv.Memoria, written below
	for _, it := range vigente {
		if it.Slice == e.ID() && it.Fuente == "scout" {
			crit = append(crit, fmt.Sprintf("%s %s (%s)", it.ID, it.Texto, it.TipoEfectivo()))
		}
	}
	wt, rama := e.Worktree, e.Rama
	var yo *slice.Worker
	for i := range e.Equipo {
		if e.Equipo[i].Nombre == worker {
			yo = &e.Equipo[i]
			wt, rama = yo.Worktree, yo.Rama
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "CONTRATO %s · worker: %s\n", e.Attempt(), worker)
	fmt.Fprintf(&b, "Objetivo del slice: %s\n", rv.Objetivo)
	fmt.Fprintf(&b, "Worktree: %s · rama %s · base %s@%s. No crees otro; comprueba `git rev-parse --show-toplevel`.\n", wt, rama, e.Base, slice.Corto(e.SHARojo))
	if yo != nil {
		fmt.Fprintf(&b, "Tu zona: %s → solo cambias %s. Lo de otra zona se lo pides a su dueño; si tocas fuera, `orq slice juntar` rechaza tu trabajo.\n", yo.Zona, strings.Join(yo.Rutas, ", "))
		var otros []string
		for _, o := range e.Equipo {
			if o.Nombre != worker {
				otros = append(otros, fmt.Sprintf("%s (%s: %s)", o.Nombre, o.Zona, strings.Join(o.Rutas, ", ")))
			}
		}
		if len(otros) > 0 {
			fmt.Fprintf(&b, "Tu equipo, trabajando a la vez: %s. Acuerden contratos (endpoints, tipos, datos) temprano: `orq msg send --de %s --para <worker> --tipo pregunta|respuesta|estado \"...\"`.\n", strings.Join(otros, "; "), worker)
		}
		b.WriteString("El examen de aceptación pasa cuando el Orquestador junta a todo el equipo; tú corre tus pruebas de zona. Commitea seguido: el Orquestador junta lo commiteado.\n")
	}
	fmt.Fprintf(&b, "Alcance: %s. Nada de refactors fuera de esto.\n", alcance)
	fmt.Fprintf(&b, "Aceptación: `%s` (protegido, igual que %s: no lo edites).\n", e.Cmd, strings.Join(e.Bundle, ", "))
	if suite != "" {
		fmt.Fprintf(&b, "Suite rápida: `%s`\n", suite)
	}
	fmt.Fprintf(&b, "Decisiones vigentes:\n%s\n", lista(rv.Decisiones, "(ninguna)"))
	fmt.Fprintf(&b, "Criterios de scouts:\n%s\n", lista(crit, "(ninguno)"))
	if rv.Memoria != "" {
		fmt.Fprintf(&b, "Aviso: decisiones y criterios pueden faltar. %s\n", rv.Memoria)
	}
	if len(rv.Restriccion) > 0 {
		fmt.Fprintf(&b, "Lo que exige el Owner:\n%s\n", lista(rv.Restriccion, ""))
	}
	fmt.Fprintf(&b, "Presupuesto: %s · el examen tolera %d reparaciones; después se bloquea.\n", presupuesto, slice.MaxFallos-1)
	switch mapa := slice.MapaEn(p.Root); {
	case mapa == filepath.Join(p.Root, slice.RutaMapa):
		fmt.Fprintf(&b, "Web: antes de pedir verificación, reproduce el bug o recorre la función en un navegador real (Playwright MCP o `npx playwright`); el mapa de rutas %s de tu copia dice cómo llegar. Si sumas una ruta, suma su línea al mapa en el mismo commit.\n", slice.RutaMapa)
	case mapa != "":
		fmt.Fprintf(&b, "Web: antes de pedir verificación, reproduce el bug o recorre la función en un navegador real (Playwright MCP o `npx playwright`); el mapa de rutas %s dice cómo llegar. Si sumas una ruta, manda su línea del mapa en tu respuesta.\n", mapa)
	}
	b.WriteString("Cómo: TDD con tests propios fuera del paquete; commits pequeños; ante fallo, reproduce antes de cambiar. No borres código ajeno.\n")
	b.WriteString("Para y pregunta si: ambigüedad de producto, cambia alcance o aceptación, más presupuesto, riesgo (datos, dinero, seguridad, irreversible) o el examen parece mal.\n")
	b.WriteString("No hagas: merge, push, limpiar worktrees, abrir o cerrar slices, hablar con el Owner.\n")
	b.WriteString("Responde ≤12 líneas: PIDO VERIFICACIÓN | BLOQUEADO | PREGUNTA · head · archivos · qué probaste y salida · qué no · riesgos.\n")
	fmt.Fprintf(&b, "Canal: `orq msg send --de %s --para orquestador --tipo pregunta|estado \"...\"`. Guía completa: `orq skill worker`.\n", worker)
	return b.String(), nil
}
