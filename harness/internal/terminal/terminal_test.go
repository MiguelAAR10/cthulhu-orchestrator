package terminal_test

import (
	"context"
	"errors"
	"testing"

	"orq/internal/terminal"
	"orq/internal/terminal/fake"
)

var arranque = terminal.Arranque{Nombre: "worker-x-s01", Kind: "claude", Panel: "p1"}

func TestArrancarSeguroArrancaUnaVez(t *testing.T) {
	d := fake.Nuevo()
	r, err := terminal.ArrancarSeguro(context.Background(), d, arranque)
	if err != nil || r != terminal.Arranco || len(d.Ops("arrancar")) != 1 {
		t.Fatalf("r=%v err=%v arranques=%d", r, err, len(d.Ops("arrancar")))
	}
	// Repetir el mismo comando no lanza otro agente.
	r, err = terminal.ArrancarSeguro(context.Background(), d, arranque)
	if err != nil || r != terminal.YaEstaba || len(d.Ops("arrancar")) != 1 {
		t.Fatalf("la repetición lanzó de nuevo: r=%v err=%v arranques=%d", r, err, len(d.Ops("arrancar")))
	}
}

func TestArranqueVencidoSeResuelveConEstadoSinRelanzar(t *testing.T) {
	// El multiplexor venció pero el agente arrancó: cuenta como arrancado, con un solo lanzamiento.
	d := fake.Nuevo()
	d.ArrancaTarde = true
	r, err := terminal.ArrancarSeguro(context.Background(), d, arranque)
	if err != nil || r != terminal.Arranco || len(d.Ops("arrancar")) != 1 {
		t.Fatalf("r=%v err=%v arranques=%d", r, err, len(d.Ops("arrancar")))
	}

	// Venció y el agente no aparece: desconocido, y tampoco se relanza.
	d = fake.Nuevo()
	d.Fallos["arrancar"] = []error{terminal.ErrDesconocido}
	_, err = terminal.ArrancarSeguro(context.Background(), d, arranque)
	if !errors.Is(err, terminal.ErrDesconocido) || len(d.Ops("arrancar")) != 1 {
		t.Fatalf("err=%v arranques=%d", err, len(d.Ops("arrancar")))
	}

	// Venció y el estado tampoco se pudo leer: desconocido.
	d = fake.Nuevo()
	d.Fallos["arrancar"] = []error{terminal.ErrDesconocido}
	d.Fallos["estado"] = []error{nil, terminal.ErrDesconocido}
	_, err = terminal.ArrancarSeguro(context.Background(), d, arranque)
	if !errors.Is(err, terminal.ErrDesconocido) || len(d.Ops("arrancar")) != 1 {
		t.Fatalf("err=%v arranques=%d", err, len(d.Ops("arrancar")))
	}
}

func TestSinSaberSiExisteNoSeArranca(t *testing.T) {
	d := fake.Nuevo()
	d.Fallos["estado"] = []error{terminal.ErrDesconocido}
	if _, err := terminal.ArrancarSeguro(context.Background(), d, arranque); !errors.Is(err, terminal.ErrDesconocido) || len(d.Ops("arrancar")) != 0 {
		t.Fatalf("arrancó a ciegas: err=%v arranques=%d", err, len(d.Ops("arrancar")))
	}
	d = fake.Nuevo()
	d.EstadoFijo[arranque.Nombre] = terminal.Desconocida
	if _, err := terminal.ArrancarSeguro(context.Background(), d, arranque); !errors.Is(err, terminal.ErrDesconocido) || len(d.Ops("arrancar")) != 0 {
		t.Fatalf("arrancó con estado desconocido: err=%v", err)
	}
	// Un fallo claro del arranque (no un vencimiento) se devuelve tal cual.
	d = fake.Nuevo()
	claro := errors.New("panel ocupado")
	d.Fallos["arrancar"] = []error{claro}
	if _, err := terminal.ArrancarSeguro(context.Background(), d, arranque); !errors.Is(err, claro) || errors.Is(err, terminal.ErrDesconocido) {
		t.Fatalf("err=%v", err)
	}
}
