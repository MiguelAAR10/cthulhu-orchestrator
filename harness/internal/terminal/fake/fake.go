// Package fake es un multiplexor de mentira para pruebas: registra las llamadas y deja simular
// tiempos vencidos, arranques tardíos y fallos.
package fake

import (
	"context"
	"fmt"
	"sync"

	"orq/internal/terminal"
)

// Llamada es una operación recibida, en orden.
type Llamada struct {
	Op      string // crear|arrancar|avisar|estado|renombrar|cerrar
	Destino string
	Texto   string
	Args    []string
	Env     map[string]string
	Cwd     string
}

// Driver cumple terminal.Driver en memoria.
type Driver struct {
	mu sync.Mutex

	EstaDentro bool
	Actual     string

	// Fallos programados por operación: se consume uno por llamada.
	Fallos map[string][]error
	// ArrancaTarde: Arrancar devuelve ErrDesconocido pero el agente queda vivo (herdr venció, el agente arrancó).
	ArrancaTarde bool
	// EstadoFijo fuerza la respuesta de Estado para un nombre.
	EstadoFijo map[string]terminal.Vida

	Llamadas []Llamada
	Agentes  map[string]string // nombre → panel
	Paneles  map[string]string // id → pestaña
	n        int
}

// Nuevo devuelve un driver dentro de un panel "p0".
func Nuevo() *Driver {
	return &Driver{EstaDentro: true, Actual: "p0", Fallos: map[string][]error{}, EstadoFijo: map[string]terminal.Vida{},
		Agentes: map[string]string{}, Paneles: map[string]string{"p0": "t0"}}
}

func (d *Driver) fallo(op string) error {
	if l := d.Fallos[op]; len(l) > 0 {
		d.Fallos[op] = l[1:]
		return l[0]
	}
	return nil
}

func (d *Driver) Nombre() string      { return "fake" }
func (d *Driver) Dentro() bool        { return d.EstaDentro }
func (d *Driver) PanelActual() string { return d.Actual }

func (d *Driver) CrearPanel(_ context.Context, p terminal.PanelNuevo) (terminal.Panel, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.Llamadas = append(d.Llamadas, Llamada{Op: "crear", Destino: p.Junto, Texto: p.Etiqueta, Env: p.Env, Cwd: p.Cwd})
	if err := d.fallo("crear"); err != nil {
		return terminal.Panel{}, err
	}
	d.n++
	id := fmt.Sprintf("p%d", d.n)
	tab := fmt.Sprintf("t%d", d.n)
	if p.Junto != "" {
		tab = d.Paneles[p.Junto]
	}
	d.Paneles[id] = tab
	return terminal.Panel{ID: id, Pestana: tab}, nil
}

func (d *Driver) Arrancar(_ context.Context, a terminal.Arranque) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.Llamadas = append(d.Llamadas, Llamada{Op: "arrancar", Destino: a.Nombre, Texto: a.Kind, Args: a.Args, Env: a.Env, Cwd: a.Panel})
	if err := d.fallo("arrancar"); err != nil {
		return err
	}
	d.Agentes[a.Nombre] = a.Panel
	if d.ArrancaTarde {
		return terminal.ErrDesconocido
	}
	return nil
}

func (d *Driver) Avisar(_ context.Context, destino, texto string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.Llamadas = append(d.Llamadas, Llamada{Op: "avisar", Destino: destino, Texto: texto})
	return d.fallo("avisar")
}

func (d *Driver) Estado(_ context.Context, nombre string) (terminal.Vida, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.Llamadas = append(d.Llamadas, Llamada{Op: "estado", Destino: nombre})
	if err := d.fallo("estado"); err != nil {
		return terminal.Desconocida, err
	}
	if v, ok := d.EstadoFijo[nombre]; ok {
		return v, nil
	}
	if _, ok := d.Agentes[nombre]; ok {
		return terminal.Vivo, nil
	}
	return terminal.Ausente, nil
}

func (d *Driver) Renombrar(_ context.Context, destino, nombre string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.Llamadas = append(d.Llamadas, Llamada{Op: "renombrar", Destino: destino, Texto: nombre})
	if err := d.fallo("renombrar"); err != nil {
		return err
	}
	d.Agentes[nombre] = destino
	return nil
}

func (d *Driver) Cerrar(_ context.Context, panel string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.Llamadas = append(d.Llamadas, Llamada{Op: "cerrar", Destino: panel})
	if err := d.fallo("cerrar"); err != nil {
		return err
	}
	delete(d.Paneles, panel)
	for n, p := range d.Agentes {
		if p == panel {
			delete(d.Agentes, n)
		}
	}
	return nil
}

// Ops devuelve las llamadas de una operación.
func (d *Driver) Ops(op string) []Llamada {
	d.mu.Lock()
	defer d.mu.Unlock()
	var out []Llamada
	for _, l := range d.Llamadas {
		if l.Op == op {
			out = append(out, l)
		}
	}
	return out
}
