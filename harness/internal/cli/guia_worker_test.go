package cli

import (
	"strings"
	"testing"
)

// The orq2 worker guide names only commands the worker allowlist lets a worker run.
func TestGuiaWorkerOrq2SoloNombraLoPermitido(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq2")
	raw, _ := guias.ReadFile("guias/worker.md")
	g := orq2Guia("worker", string(raw))
	for _, c := range codigo.FindAllString(g, -1) {
		for _, m := range llamada.FindAllStringSubmatch(c, -1) {
			args := []string{m[1]}
			if m[2] != "" {
				args = append(args, m[2])
			}
			if !permitidoAWorker(args) {
				t.Errorf("the orq2 worker guide tells the worker to run %q, which orq2 refuses to a worker", strings.Join(args, " "))
			}
		}
	}
	if !strings.Contains(g, "orq msg send --para orquestador") {
		t.Error("the orq2 worker guide must say how to propose a lesson to the orchestrator")
	}
}

// orq keeps 1.0.0: its worker guide is the file, byte for byte.
func TestGuiaWorkerOrqComo100(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq")
	raw, _ := guias.ReadFile("guias/worker.md")
	if orq2Guia("worker", string(raw)) != string(raw) {
		t.Fatal("orq's worker guide changed")
	}
}
