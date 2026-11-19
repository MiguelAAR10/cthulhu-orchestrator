package cli

import (
	"strings"
	"testing"
)

func abrirHijo(t *testing.T, nn string) {
	t.Helper()
	args := []string{"slice", "abrir", nn, "--titulo", "hijo " + nn, "--test", "tests/s01.sh", "--cmd", "sh run.sh", "--sin-scouts", "e2e", "--sin-conteo", "prueba", "--historia", "pagos"}
	if _, errs, code := run(t, args...); code != 0 {
		t.Fatalf("abrir %s: %d %s", nn, code, errs)
	}
}

func TestStatusAgrupaHistoria(t *testing.T) {
	repoCLI(t)
	t.Setenv("ORQ_STATE", t.TempDir())
	conLimites(t)
	if out, errs, code := run(t, "historia", "abrir", "pagos", "--titulo", "Pagos"); code != 0 || !strings.Contains(out, "pagos") {
		t.Fatalf("historia abrir: %d %s %s", code, out, errs)
	}
	for _, nn := range []string{"01", "02", "03"} {
		abrirHijo(t, nn)
	}
	out, errs, code := run(t, "status")
	if code != 0 {
		t.Fatalf("%d %s", code, errs)
	}
	var lineas []string
	for _, l := range strings.Split(out, "\n") {
		if strings.Contains(l, "S-0") {
			lineas = append(lineas, l)
		}
	}
	if len(lineas) != 1 {
		t.Fatalf("one line per story, got %d:\n%s", len(lineas), out)
	}
	contiene(t, lineas[0], "pagos «Pagos»", "S-01 IMPLEMENTANDO", "S-02 IMPLEMENTANDO", "S-03 IMPLEMENTANDO")
}

func TestPausarCLI(t *testing.T) {
	repoCLI(t)
	t.Setenv("ORQ_STATE", t.TempDir())
	conLimites(t)
	run(t, "historia", "abrir", "pagos", "--titulo", "Pagos")
	abrirHijo(t, "01")
	if _, errs, code := run(t, "historia", "pausar", "pagos"); code != 0 {
		t.Fatalf("pausar: %d %s", code, errs)
	}
	out, _, _ := run(t, "status")
	contiene(t, out, "pagos «Pagos» PAUSADA")
	run(t, "historia", "reanudar", "pagos")
	if _, errs, code := run(t, "pausar", "almuerzo"); code != 0 {
		t.Fatalf("pausar: %d %s", code, errs)
	}
	out, _, _ = run(t, "status")
	if !strings.HasPrefix(out, "PAUSA: almuerzo") || strings.Contains(out, "PAUSADA") {
		t.Fatalf("global pause first:\n%s", out)
	}
	run(t, "reanudar")
	out, _, _ = run(t, "status")
	if strings.Contains(out, "PAUSA") {
		t.Fatalf("resumed:\n%s", out)
	}
	if _, errs, code := run(t, "historia", "pausar", "nada"); code == 0 || !strings.Contains(errs, "orq historia abrir") {
		t.Fatalf("unknown story: %d %s", code, errs)
	}
}

func TestStatusAlertasArriba(t *testing.T) {
	sliceVerdeCon(t, nil, func(wt string) {
		aprobada(t)(wt)
		commitClon(t, wt, "tests/extra.sh", "true\n")
		if _, errs, code := run(t, "slice", "examen", "tests/extra.sh", "--aprobado", "req ATN-9"); code != 0 {
			t.Fatalf("examen: %d %s", code, errs)
		}
	})
	conLimites(t)
	out, errs, code := run(t, "status")
	if code != 0 {
		t.Fatalf("%d %s", code, errs)
	}
	l := strings.Split(out, "\n")
	if !strings.HasPrefix(l[0], "ALERTAS") || !strings.Contains(l[1], "S-01 examen_tras_aprobar") || !strings.Contains(l[1], "req ATN-9") {
		t.Fatalf("alerts first:\n%s", out)
	}
}
