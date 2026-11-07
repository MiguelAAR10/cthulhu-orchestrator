package herdr

import (
	"os"
	"testing"
)

// Prueba contra un herdr vivo: ORQ_HERDR_LIVE=1 go test ./internal/herdr/ -run Live
func TestLiveTabSplitClose(t *testing.T) {
	if os.Getenv("ORQ_HERDR_LIVE") != "1" {
		t.Skip("solo con herdr vivo")
	}
	tab, p1, err := Tab("orq-test · borrar", os.TempDir(), map[string]string{"ORQ_ROL": "planificador"})
	if err != nil || tab == "" || p1 == "" {
		t.Fatalf("tab: %q %q %v", tab, p1, err)
	}
	p2, err := SplitDe(p1, os.TempDir(), map[string]string{"ORQ_ROL": "astra"})
	if err != nil || p2 == "" {
		t.Fatalf("split: %q %v", p2, err)
	}
	t.Logf("tab %s panes %s %s", tab, p1, p2)
	if err := ClosePane(p2); err != nil {
		t.Fatal(err)
	}
	if err := ClosePane(p1); err != nil {
		t.Fatal(err)
	}
}
