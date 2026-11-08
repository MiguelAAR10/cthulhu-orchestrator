package qa

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var t0 = time.Date(2026, 11, 8, 10, 0, 0, 0, time.UTC)

// fake runner: exit code by command.
func fake(codes map[string]int, outs map[string]string) Runner {
	return func(dir, cmd string, _ time.Duration) (string, int) { return outs[cmd], codes[cmd] }
}

func completo() Finding {
	return Finding{Requirement: "R1 login", Commit: "abc123", Repro: "repro.sh", Observed: "500", Expected: "200", Impact: "users cannot log in"}
}

func TestFindingIncompletoSeRechaza(t *testing.T) {
	f := completo()
	f.Expected, f.Impact = "", "  "
	err := f.Validar()
	if err == nil || !strings.Contains(err.Error(), "expected") || !strings.Contains(err.Error(), "impact") {
		t.Fatalf("debe rechazar y nombrar los campos que faltan: %v", err)
	}
	dir := t.TempDir()
	if _, err := AddFinding(dir, "07", f, fake(nil, nil), t0); err == nil {
		t.Fatal("AddFinding no debe guardar un hallazgo incompleto")
	}
	if fs := Findings(dir, "07"); len(fs) != 0 {
		t.Fatalf("no debía guardarse nada: %v", fs)
	}
}

func TestFindingSoloConfirmadoSiReproFalla(t *testing.T) {
	dir := t.TempDir()
	pasa, falla := completo(), completo()
	pasa.Repro, falla.Repro = "true", "false"
	run := fake(map[string]int{"true": 0, "false": 1}, nil)
	f1, err := AddFinding(dir, "07", pasa, run, t0)
	if err != nil || f1.Confirmed {
		t.Fatalf("repro que pasa no confirma: %+v %v", f1, err)
	}
	f2, err := AddFinding(dir, "07", falla, run, t0)
	if err != nil || !f2.Confirmed {
		t.Fatalf("repro que falla confirma: %+v %v", f2, err)
	}
	if n := Confirmados(Findings(dir, "07")); n != 1 {
		t.Fatalf("solo cuenta el confirmado: %d", n)
	}
}

func capas() []Layer {
	return []Layer{
		{Name: "unit", Cmd: "go test ./..."},
		{Name: "integration", Cmd: "it.sh"},
		{Name: "e2e", Cmd: "npx playwright test"},
		{Name: "mutation", Cmd: "mut.sh", MinCriticality: "high"},
		{Name: "adversarial", Cmd: "adv.sh", MinCriticality: "high"},
	}
}

func TestMutacionSeOmiteEnMediaYCorreEnAlta(t *testing.T) {
	ran := map[string]int{}
	run := func(dir, cmd string, _ time.Duration) (string, int) { ran[cmd]++; return "3 passed", 0 }
	rec := Run(".", "07", "abc", capas(), "medium", run, t0)
	if ran["mut.sh"] != 0 || ran["adv.sh"] != 0 {
		t.Fatalf("en media no corre mutación ni adversarial: %v", ran)
	}
	var omitida bool
	for _, l := range rec.Layers {
		if l.Name == "mutation" && l.Skipped {
			omitida = true
		}
	}
	if !omitida || rec.Result != "pass" {
		t.Fatalf("la capa queda registrada como omitida: %+v", rec)
	}
	rec = Run(".", "07", "abc", capas(), "high", run, t0)
	if ran["mut.sh"] != 1 || ran["adv.sh"] != 1 {
		t.Fatalf("en alta corren: %v", ran)
	}
	if rec.Layers[0].Passed != 3 {
		t.Fatalf("cuenta por capa: %+v", rec.Layers[0])
	}
}

func TestPiramideEnOrdenYFallo(t *testing.T) {
	var orden []string
	run := func(dir, cmd string, _ time.Duration) (string, int) {
		orden = append(orden, cmd)
		if cmd == "it.sh" {
			return "--- PASS: A\n--- FAIL: B\n", 1
		}
		return "ok", 0
	}
	rec := Run(".", "07", "abc", capas(), "medium", run, t0)
	if strings.Join(orden, ",") != "go test ./...,it.sh,npx playwright test" {
		t.Fatalf("orden de la pirámide: %v", orden)
	}
	if rec.Result != "fail" || rec.Layers[1].Failed != 1 || rec.Layers[1].Passed != 1 {
		t.Fatalf("recibo: %+v", rec)
	}
}

func TestCierreBloqueadoSinReciboEnMedia(t *testing.T) {
	dir := t.TempDir()
	if err := PuedeCerrar(dir, "07", Config{}); err == nil {
		t.Fatal("criticidad media por defecto: sin recibo QA no se cierra")
	}
	if err := PuedeCerrar(dir, "07", Config{Criticality: "low"}); err != nil {
		t.Fatalf("baja no exige recibo: %v", err)
	}
	rec := Run(dir, "07", "abc", []Layer{{Name: "unit", Cmd: "x"}}, "medium", fake(map[string]int{"x": 0}, nil), t0)
	if err := Guardar(dir, rec); err != nil {
		t.Fatal(err)
	}
	if err := PuedeCerrar(dir, "07", Config{}); err != nil {
		t.Fatalf("con recibo verde se cierra: %v", err)
	}
	// un hallazgo confirmado abierto también bloquea
	f := completo()
	f.Repro = "false"
	AddFinding(dir, "07", f, fake(map[string]int{"false": 1}, nil), t0)
	if err := PuedeCerrar(dir, "07", Config{}); err == nil {
		t.Fatal("un hallazgo confirmado bloquea el cierre")
	}
}

func TestConfigTolerante(t *testing.T) {
	dir := t.TempDir()
	if c := LoadConfig(dir); c.Crit() != "medium" {
		t.Fatalf("por defecto medium: %q", c.Crit())
	}
	os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"project":{"criticality":"high"},"qa":{"layers":[{"name":"unit","cmd":"go test"}],"require_receipt_from":"high"}}`), 0o644)
	c := LoadConfig(dir)
	if c.Crit() != "high" || len(c.Layers) != 1 || c.RequireFrom != "high" {
		t.Fatalf("lee project.criticality y qa: %+v", c)
	}
	os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"criticality":"production-critical"}`), 0o644)
	if c := LoadConfig(dir); c.Crit() != "high" {
		t.Fatalf("criticality en raíz y alias: %q", c.Crit())
	}
	os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"criticality":5}`), 0o644)
	if c := LoadConfig(dir); c.Crit() != "medium" {
		t.Fatalf("valor raro → medium: %q", c.Crit())
	}
}
