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

// conLimites writes a valid limits file where ORQ_LIMITS points.
func conLimites(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	raw := `{"min_free_gb":{"/":1},"max_workers":99,"scratch":{"paths":["` + dir + `/s"],"quota_gb":1,"critical_free_gb":1},"state_max_age_hours":24}`
	if err := os.WriteFile(os.Getenv("ORQ_LIMITS"), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestStatusListaVerificados(t *testing.T) {
	repoCLI(t)
	t.Setenv("ORQ_STATE", t.TempDir())
	conLimites(t)
	out, errs, code := run(t, "status")
	if code != 0 {
		t.Fatalf("%d %s", code, errs)
	}
	contiene(t, out, "esperan al Owner: ninguno")
	sliceVerdeCon(t, nil, nil)
	conLimites(t)
	out, _, _ = run(t, "status")
	contiene(t, out, "esperan al Owner (verificados, el más antiguo primero):", "S-01/a1 «total» hace 0m · orq slice decidir 01")
}

// Review fix 4: a historia linked to the slice wins over a later one; without the link and with
// several slices open the page says the story may be another slice's.
func TestDecidirHistoriaVinculadaYAviso(t *testing.T) {
	root := sliceVerdeCon(t, nil, nil)
	path := filepath.Join(root, ".orq", "canal.jsonl")
	for _, m := range []canal.Msg{
		{De: "atena", Para: "orquestador", Tipo: "historia", Attempt: "S-01/a1", Texto: "historia de uno"},
		{De: "atena", Para: "orquestador", Tipo: "historia", Attempt: "S-02/a1", Texto: "historia de dos"},
	} {
		if err := canal.Append(path, &m); err != nil {
			t.Fatal(err)
		}
	}
	out, _, code := run(t, "slice", "decidir", "01")
	if code != 0 {
		t.Fatal(out)
	}
	contiene(t, out, "historia de uno")
	if strings.Contains(out, "historia de dos") || strings.Contains(out, "another slice") {
		t.Fatalf("linked story must win, no warning:\n%s", out)
	}
}

func TestDecidirHistoriaSinVinculoConVariosAbiertosAvisa(t *testing.T) {
	root := sliceVerdeCon(t, nil, nil)
	if err := canal.Append(filepath.Join(root, ".orq", "canal.jsonl"), &canal.Msg{De: "atena", Para: "orquestador", Tipo: "historia", Texto: "historia suelta"}); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(root, "tests", "s02.sh"), []byte("true\n"), 0o644)
	for _, c := range [][]string{{"add", "-A"}, {"commit", "-qm", "rojo 2"}} {
		cmd := exec.Command("git", c...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", c, out)
		}
	}
	if _, errs, code := run(t, "slice", "abrir", "02", "--titulo", "iva", "--test", "tests/s02.sh", "--cmd", "sh run.sh", "--sin-conteo", "juguete", "--sin-scouts", "e2e"); code != 0 {
		t.Fatalf("abrir 02: %d %s", code, errs)
	}
	out, _, code := run(t, "slice", "decidir", "01")
	if code != 0 {
		t.Fatal(out)
	}
	contiene(t, out, "historia suelta", "!! story may belong to another slice")
}

// conChecks writes the project's checks block from inside sliceVerdeCon's hook, before verificar.
func conChecks(bloque string) func(wt string) {
	return func(wt string) {
		root := filepath.Join(filepath.Dir(filepath.Dir(wt)), "tienda")
		if err := os.WriteFile(filepath.Join(root, ".orq", "config.json"), []byte(bloque), 0o644); err != nil {
			panic(err)
		}
	}
}

func TestDecidirMuestraLosChecks(t *testing.T) {
	sliceVerdeCon(t, nil, conChecks(`{"checks": {"lint": {"cmd": "true"}, "types": {"cmd": "echo sin errores"}, "tests": {"omitir": "sin script test"}}}`))
	out, _, code := run(t, "slice", "decidir")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	contiene(t, out, "== Checks (recibo · checks de .orq/config.json)", "lint paso · exit 0 · true", "types paso · exit 0 · echo sin errores",
		"!! tests omitido: sin script test (no cuenta como pasado)")
	html := filepath.Join(t.TempDir(), "p.html")
	if _, errs, code := run(t, "slice", "decidir", "--html", html); code != 0 {
		t.Fatalf("%d %s", code, errs)
	}
	raw, _ := os.ReadFile(html)
	contiene(t, string(raw), "Checks", "tests omitido: sin script test")
}

func TestDecidirSinChecksAvisa(t *testing.T) {
	sliceVerdeCon(t, nil, nil)
	out, _, _ := run(t, "slice", "decidir")
	contiene(t, out, "!! sin checks en el recibo: corre orq init y vuelve a verificar")
}

func TestVerificarCLIRechazaConMensajeDeLaHerramienta(t *testing.T) {
	root := repoCLI(t)
	t.Setenv("ORQ_STATE", t.TempDir())
	if _, errs, code := run(t, append(append([]string{}, abrirBase...), "--reporte", "out.tap", "--formato", "tap")...); code != 0 {
		t.Fatalf("abrir: %d %s", code, errs)
	}
	run(t, "slice", "tomar", "w1")
	if err := os.WriteFile(filepath.Join(root, ".orq", "config.json"), []byte(`{"checks": {"lint": {"cmd": "true"}, "tests": {"cmd": "true"},
		"types": {"cmd": "echo \"src.ts(1,5): error TS2322: Type 'string' is not assignable to type 'number'.\"; exit 2"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	commitClon(t, filepath.Join(filepath.Dir(root), "wt", "tienda-s01"), "total.txt", "170\n")
	_, errs, code := run(t, "slice", "verificar")
	if code != 1 {
		t.Fatalf("verificar exit %d: %s", code, errs)
	}
	contiene(t, errs, "check types falla (exit 2)", "error TS2322")
	if _, errs, code := run(t, "slice", "cerrar", "--aceptado"); code == 0 {
		t.Fatalf("closed with a failing check: %s", errs)
	}
}

func TestVerificarCLIImprimeLosChecks(t *testing.T) {
	root := repoCLI(t)
	t.Setenv("ORQ_STATE", t.TempDir())
	if _, errs, code := run(t, append(append([]string{}, abrirBase...), "--reporte", "out.tap", "--formato", "tap")...); code != 0 {
		t.Fatalf("abrir: %d %s", code, errs)
	}
	run(t, "slice", "tomar", "w1")
	conChecks(`{"checks": {"lint": {"cmd": "true"}, "types": {"cmd": "true"}, "tests": {"omitir": "sin script test"}}}`)(filepath.Join(filepath.Dir(root), "wt", "tienda-s01"))
	commitClon(t, filepath.Join(filepath.Dir(root), "wt", "tienda-s01"), "total.txt", "170\n")
	out, errs, code := run(t, "slice", "verificar")
	if code != 0 {
		t.Fatalf("verificar exit %d: %s", code, errs)
	}
	contiene(t, out, "checks: lint paso · types paso · tests omitido")
}
