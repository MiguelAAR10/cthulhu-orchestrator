package herdr

import (
	"context"
	"os"
	"testing"

	"orq/internal/terminal"
)

// Prueba contra un herdr vivo: ORQ_HERDR_LIVE=1 go test ./internal/terminal/herdr/ -run Live
func TestLiveTabSplitClose(t *testing.T) {
	if os.Getenv("ORQ_HERDR_LIVE") != "1" {
		t.Skip("solo con herdr vivo")
	}
	d, ctx := Nuevo(), context.Background()
	p1, err := d.CrearPanel(ctx, terminal.PanelNuevo{Etiqueta: "orq-test · borrar", Cwd: os.TempDir(), Env: map[string]string{"ORQ_ROL": "planificador"}})
	if err != nil || p1.Pestana == "" || p1.ID == "" {
		t.Fatalf("tab: %+v %v", p1, err)
	}
	p2, err := d.CrearPanel(ctx, terminal.PanelNuevo{Junto: p1.ID, Cwd: os.TempDir(), Env: map[string]string{"ORQ_ROL": "astra"}})
	if err != nil || p2.ID == "" {
		t.Fatalf("split: %+v %v", p2, err)
	}
	t.Logf("tab %s panes %s %s", p1.Pestana, p1.ID, p2.ID)
	if err := d.Cerrar(ctx, p2.ID); err != nil {
		t.Fatal(err)
	}
	if err := d.Cerrar(ctx, p1.ID); err != nil {
		t.Fatal(err)
	}
}
