package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"orq/internal/canal"
	"orq/internal/diario"
)

// sliceVerdeCon opens slice 01 with extra flags, lets antes act in the worker clone, turns the exam
// green there and verifies. It returns the project root.
func sliceVerdeCon(t *testing.T, extra []string, antes func(wt string)) string {
	t.Helper()
	root := repoCLI(t)
	t.Setenv("ORQ_STATE", t.TempDir())
	abrir := append(append([]string{}, abrirBase...), "--reporte", "out.tap", "--formato", "tap")
	if _, errs, code := run(t, append(abrir, extra...)...); code != 0 {
		t.Fatalf("abrir: %d %s", code, errs)
	}
	run(t, "slice", "tomar", "w1")
	wt := filepath.Join(filepath.Dir(root), "wt", "tienda-s01")
	if antes != nil {
		antes(wt)
	}
	commitClon(t, wt, "total.txt", "170\n")
	if out, errs, code := run(t, "slice", "verificar"); code != 0 {
		t.Fatalf("verificar: %d %s %s", code, out, errs)
	}
	return root
}

func commitClon(t *testing.T, wt, file, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(wt, file), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, c := range [][]string{{"add", "-A"}, {"commit", "-qm", "cambio " + file}} {
		cmd := exec.Command("git", c...)
		cmd.Dir = wt
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", c, out)
		}
	}
}

func contiene(t *testing.T, out string, want ...string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(out, w) {
			t.Fatalf("missing %q in:\n%s", w, out)
		}
	}
}

func TestDecidirMuestraCadaCampoConSuFuente(t *testing.T) {
	sliceVerdeCon(t, []string{"--alcance", "total.txt", "--demo", "abre total.txt; mira que diga 170; listo"}, func(wt string) {
		commitClon(t, wt, "notas.txt", "fuera\n")
	})
	out, errs, code := run(t, "slice", "decidir", "01")
	if code != 0 {
		t.Fatalf("%d %s", code, errs)
	}
	contiene(t, out,
		"S-01/a1",
		"== Historia (título del slice)", "total",
		"== Pruebas (recibo, conteo del runner tap)", "1 ejecutadas, 0 omitidas, 0 fallidas · paso",
		"== Archivos (alcance de orq slice abrir y zonas, contra git diff del rojo a la punta)", "planeado: total.txt", "!! notas.txt fuera del alcance",
		"== Examen (diario .orq/diario.jsonl y estado del slice)", "sin cambios desde que se abrió",
		"== Recibo (refs/orq/recibo/01)", "cmd: sh run.sh · exit 0",
		"== Cómo verlo funcionar (orq slice abrir --demo)", "1. abre total.txt", "2. mira que diga 170", "3. listo",
		"orq slice cerrar --slice 01 --aceptado", `orq slice cerrar --slice 01 --rechazado "motivo"`)
	if strings.Contains(out, "!! total.txt") {
		t.Fatalf("total.txt is inside the scope:\n%s", out)
	}
}

func TestDecidirResaltaExamenCambiado(t *testing.T) {
	sliceVerdeCon(t, nil, func(wt string) {
		commitClon(t, wt, "tests/extra.sh", "true\n")
		if _, errs, code := run(t, "slice", "examen", "tests/extra.sh", "--aprobado", "req ATN-7"); code != 0 {
			t.Fatalf("examen: %d %s", code, errs)
		}
	})
	out, _, code := run(t, "slice", "decidir")
	if code != 0 {
		t.Fatal(out)
	}
	contiene(t, out, "!! examen cambiado tras abrir", "tests/extra.sh", "req ATN-7")
}

func TestDecidirSinDemoNiAlcance(t *testing.T) {
	sliceVerdeCon(t, nil, nil)
	out, _, _ := run(t, "slice", "decidir")
	contiene(t, out, "sin demo registrada", "sin alcance declarado: nada que marcar fuera")
}

func TestDecidirHTML(t *testing.T) {
	root := sliceVerdeCon(t, nil, nil)
	if err := canal.Append(filepath.Join(root, ".orq", "canal.jsonl"), &canal.Msg{De: "atena", Para: "orquestador", Tipo: "historia",
		Texto: "Como dueño quiero <script>x</script>\n\npara cobrar bien\ndado 170\nlínea 4 que no sale"}); err != nil {
		t.Fatal(err)
	}
	html := filepath.Join(t.TempDir(), "pagina.html")
	out, errs, code := run(t, "slice", "decidir", "--html", html)
	if code != 0 {
		t.Fatalf("%d %s", code, errs)
	}
	contiene(t, out, html, "== Historia (canal: historia", "para cobrar bien", "dado 170")
	if strings.Contains(out, "línea 4") {
		t.Fatalf("the story is three lines:\n%s", out)
	}
	raw, err := os.ReadFile(html)
	if err != nil {
		t.Fatal(err)
	}
	page := string(raw)
	contiene(t, page, "<!doctype html>", "&lt;script&gt;", "orq slice cerrar --slice 01 --aceptado")
	for _, no := range []string{"<script>", "<link", "src="} {
		if strings.Contains(page, no) {
			t.Fatalf("the page must be self-contained and escaped; found %q", no)
		}
	}
}

func TestDecidirHTMLResalta(t *testing.T) {
	sliceVerdeCon(t, []string{"--alcance", "total.txt"}, func(wt string) { commitClon(t, wt, "notas.txt", "x\n") })
	html := filepath.Join(t.TempDir(), "p.html")
	if _, errs, code := run(t, "slice", "decidir", "--html", html); code != 0 {
		t.Fatal(errs)
	}
	raw, _ := os.ReadFile(html)
	contiene(t, string(raw), `class="alerta"`, "notas.txt fuera del alcance")
}

func TestDecidirSoloVerificado(t *testing.T) {
	repoCLI(t)
	t.Setenv("ORQ_STATE", t.TempDir())
	if _, errs, code := run(t, append(abrirBase, "--reporte", "out.tap", "--formato", "tap")...); code != 0 {
		t.Fatal(errs)
	}
	if _, errs, code := run(t, "slice", "decidir", "01"); code != 1 || !strings.Contains(errs, "IMPLEMENTANDO") {
		t.Fatalf("%d %s", code, errs)
	}
}

func TestMetricasCLIImprimeMediana(t *testing.T) {
	root := repoCLI(t)
	now := time.Now().UTC()
	path := diario.Ruta(filepath.Join(root, ".orq"))
	for _, e := range []diario.Evento{
		{TS: now.Add(-3 * time.Hour), Tipo: "verificacion", Slice: "01", Resultado: "paso"},
		{TS: now.Add(-3*time.Hour + 4*time.Minute), Tipo: "decision", Slice: "01", Resultado: "aceptado"},
		{TS: now.Add(-3 * time.Hour), Tipo: "verificacion", Slice: "02", Resultado: "paso"},
		{TS: now.Add(-3*time.Hour + 12*time.Minute), Tipo: "decision", Slice: "02", Resultado: "rechazado"},
		{TS: now.Add(-30 * time.Hour), Tipo: "verificacion", Slice: "03", Resultado: "paso"},
		{TS: now.Add(-time.Hour), Tipo: "intento", Slice: "04", Intento: 2, Datos: map[string]any{"causa": "examen"}},
	} {
		if err := diario.Anotar(path, e); err != nil {
			t.Fatal(err)
		}
	}
	out, errs, code := run(t, "metricas")
	if code != 0 {
		t.Fatalf("%d %s", code, errs)
	}
	for _, want := range []string{"verificado → decisión: mediana 8 min (2 decididos)", "sin decidir tras 24 h: 1", "intentos nuevos por causa: worker 0 · examen 1"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
}
