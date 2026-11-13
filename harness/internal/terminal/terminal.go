// Package terminal es el puerto del multiplexor: lo único que orq necesita de herdr u Orca.
// Crear un panel, arrancar un agente en él, tocarle el timbre, leer su estado y cerrarlo.
//
// Reglas del puerto, iguales para todo adaptador:
//   - cada llamada tiene tiempo máximo;
//   - si el multiplexor no responde a tiempo, el resultado es ErrDesconocido: ni éxito ni fallo;
//   - un arranque desconocido se resuelve con Estado antes de cualquier reintento (ArrancarSeguro).
package terminal

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ErrDesconocido: el multiplexor no respondió a tiempo. Lo pedido pudo ocurrir o no.
var ErrDesconocido = errors.New("el terminal no respondió a tiempo: resultado desconocido")

// ErrNoExiste: el panel o el agente nombrado no existe.
var ErrNoExiste = errors.New("no existe en el terminal")

// Panel identifica un panel y la pestaña donde vive.
type Panel struct {
	ID      string
	Pestana string
}

// PanelNuevo pide un panel: en una pestaña nueva con Etiqueta, o al lado de Junto en su pestaña.
type PanelNuevo struct {
	Etiqueta string
	Junto    string
	Cwd      string
	Env      map[string]string // variables del panel; el agente que arranque en él las hereda
}

// Arranque pide arrancar un agente interactivo en un panel ya creado.
type Arranque struct {
	Nombre string // nombre único del agente: es su clave de idempotencia
	Kind   string // claude | codex
	Panel  string
	Args   []string
	Env    map[string]string // las mismas del panel, para los multiplexores que las reciben al arrancar
	Cwd    string            // el del panel, para los multiplexores que no lo fijan al crearlo
}

// Vida es lo que el multiplexor sabe de un agente.
type Vida int

const (
	Ausente     Vida = iota // no existe
	Vivo                    // existe (trabajando o esperando)
	Desconocida             // el multiplexor no supo decir
)

func (v Vida) String() string {
	return [...]string{"ausente", "vivo", "desconocido"}[v]
}

// Driver es el contrato de un multiplexor.
type Driver interface {
	// Nombre del driver: herdr | orca.
	Nombre() string
	// Dentro dice si este proceso corre en un panel del multiplexor.
	Dentro() bool
	// PanelActual es el panel de este proceso ("" fuera del multiplexor).
	PanelActual() string
	// CrearPanel abre un panel sin quitar el foco al Owner.
	CrearPanel(ctx context.Context, p PanelNuevo) (Panel, error)
	// Arrancar arranca el agente y espera a que esté listo. Tiempo vencido = ErrDesconocido.
	Arrancar(ctx context.Context, a Arranque) error
	// Avisar entrega una línea al agente o panel destino, sin esperar su respuesta.
	Avisar(ctx context.Context, destino, texto string) error
	// Estado dice si el agente existe. Nunca devuelve Vivo o Ausente sin haberlo visto.
	Estado(ctx context.Context, nombre string) (Vida, error)
	// Renombrar pone nombre al agente de un panel.
	Renombrar(ctx context.Context, destino, nombre string) error
	// Cerrar cierra un panel. Cerrar uno que ya no existe no es un error.
	Cerrar(ctx context.Context, panel string) error
}

// Resultado de ArrancarSeguro.
type Resultado int

const (
	Arranco  Resultado = iota // este llamado lo arrancó
	YaEstaba                  // ya existía un agente con ese nombre: no se lanzó otro
)

// LimiteEstado acota la consulta que resuelve un arranque desconocido.
const LimiteEstado = 15 * time.Second

// ArrancarSeguro arranca como mucho una vez.
//
//	antes:    si el agente ya existe, no lanza (repetir el comando no duplica);
//	si el estado previo es desconocido, no lanza;
//	después:  si el arranque venció, consulta Estado: vivo cuenta como arrancado; ausente o
//	          desconocido devuelven ErrDesconocido con lo que se sabe. Nunca relanza a ciegas.
func ArrancarSeguro(ctx context.Context, d Driver, a Arranque) (Resultado, error) {
	antes, err := d.Estado(ctx, a.Nombre)
	switch {
	case err != nil && !errors.Is(err, ErrNoExiste):
		return 0, fmt.Errorf("no se arranca %s sin saber si ya existe: %w", a.Nombre, err)
	case antes == Vivo:
		return YaEstaba, nil
	case antes == Desconocida:
		return 0, fmt.Errorf("no se arranca %s sin saber si ya existe: %w", a.Nombre, ErrDesconocido)
	}
	err = d.Arrancar(ctx, a)
	if err == nil {
		return Arranco, nil
	}
	if !errors.Is(err, ErrDesconocido) {
		return 0, err
	}
	// Vencido: el agente pudo arrancar tarde. Se pregunta con un plazo propio, no el ya gastado.
	ctx2, cancel := context.WithTimeout(context.WithoutCancel(ctx), LimiteEstado)
	defer cancel()
	if v, errE := d.Estado(ctx2, a.Nombre); errE == nil && v == Vivo {
		return Arranco, nil
	} else if errE == nil && v == Ausente {
		return 0, fmt.Errorf("%w; %s no aparece en %s: revisa el panel %s antes de relanzar", ErrDesconocido, a.Nombre, d.Nombre(), a.Panel)
	}
	return 0, fmt.Errorf("%w; tampoco se pudo leer el estado de %s: revisa el panel %s antes de relanzar", ErrDesconocido, a.Nombre, a.Panel)
}
