package herdr

import (
	"slices"
	"testing"
)

// La marca de worker viaja como --env al crear el panel: queda en el entorno inicial del shell del
// panel y de todo lo que el agente lance (orq la busca en /proc/<ancestro>/environ).
func TestPanelRecibeMarcaDeWorker(t *testing.T) {
	var calls [][]string
	viejo := Run
	defer func() { Run = viejo }()
	Run = func(a ...string) ([]byte, error) {
		calls = append(calls, a)
		if a[0] == "pane" && a[1] == "list" {
			return []byte(`{"result":{"panes":[{"pane_id":"p9","tab_id":"t1"}]}}`), nil
		}
		return []byte(`{"result":{"tab":{"tab_id":"t1"},"pane":{"pane_id":"p9"}}}`), nil
	}
	env := map[string]string{"ORQ_WORKER_ID": "w1"}
	Tab("S-01", "/w", env)
	SplitDe("p1", "/w", env)
	Split("/w", env)
	for _, c := range []int{0, 2, 3} {
		if !slices.Contains(calls[c], "ORQ_WORKER_ID=w1") {
			t.Fatalf("sin marca: %v", calls[c])
		}
	}
}
