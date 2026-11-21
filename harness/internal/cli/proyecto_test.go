package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"orq/internal/app"
	"orq/internal/canal"
	"orq/internal/terminal"
	"orq/internal/terminal/fake"
)

// conTerminalFalso reemplaza el multiplexor por uno en memoria durante la prueba.
func conTerminalFalso(t *testing.T) *fake.Driver {
	t.Helper()
	d := fake.Nuevo()
	orig := app.NuevoDriver
	app.NuevoDriver = func(string) terminal.Driver { return d }
	t.Cleanup(func() { app.NuevoDriver = orig })
	return d
}

func run(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	var out, errb bytes.Buffer
	code := Run(args, strings.NewReader(""), &out, &errb)
	return out.String(), errb.String(), code
}

func TestMensajeSeGuardaYSeEntregaAlRolCorrecto(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ORQ_ROOT", root)
	t.Setenv("ORQ_ROL", "")
	term := conTerminalFalso(t)
	term.Actual = "w1:p1"

	os.MkdirAll(filepath.Join(root, ".orq"), 0o755)
	canal.SaveSesiones(filepath.Join(root, ".orq", "sesiones.json"), map[string]canal.Sesion{
		"atena":       {Agente: "atena-x", Pane: "w1:p1"},
		"orquestador": {Agente: "orquestador-x", Pane: "w1:p2"},
	})
	out, errs, code := run(t, "msg", "send", "--para", "orquestador", "--tipo", "historia", "Como", "Owner", "quiero", "X")
	if code != 0 {
		t.Fatalf("code %d: %s", code, errs)
	}
	avisos := term.Ops("avisar")
	if !strings.Contains(out, "orquestador-x") || len(avisos) != 1 || avisos[0].Destino != "orquestador-x" {
		t.Fatalf("out=%q avisos=%+v", out, avisos)
	}
	if !strings.Contains(avisos[0].Texto, "historia de atena") {
		t.Fatalf("formato: %q", avisos[0].Texto)
	}
	out, _, _ = run(t, "msg", "leer")
	if !strings.Contains(out, "atena→orquestador [historia] Como Owner quiero X") {
		t.Fatalf("leer: %q", out)
	}
	if _, _, code := run(t, "msg", "send", "--para", "orquestador", "--tipo", "chisme", "x"); code == 0 {
		t.Fatal("aceptó un tipo fuera de la lista")
	}
}

func TestMemoriaPorLineaDeComandos(t *testing.T) {
	t.Setenv("ORQ_ROOT", t.TempDir())
	if _, _, code := run(t, "mem", "add", "--imp", "critica", "--cat", "requisito", "--fuente", "owner", "el total incluye envío"); code != 0 {
		t.Fatal("no guardó")
	}
	if _, errs, code := run(t, "mem", "add", "--imp", "muy", "--cat", "requisito", "--fuente", "owner", "x"); code == 0 || !strings.Contains(errs, "critica|alta") {
		t.Fatalf("aceptó etiqueta inválida: %s", errs)
	}
	out, _, _ := run(t, "mem", "ver")
	if !strings.Contains(out, "[critica|requisito] el total incluye envío (owner)") {
		t.Fatalf("ver: %q", out)
	}
}

func TestOrqSkillDaLaGuiaDelRol(t *testing.T) {
	t.Setenv("ORQ_ROOT", t.TempDir())
	out, _, code := run(t, "skill", "worker")
	if code != 0 || !strings.Contains(out, "networkidle") || !strings.Contains(out, "No cargues") {
		t.Fatalf("%d %q", code, out)
	}
	if _, _, code := run(t, "--skill", "nadie"); code == 0 {
		t.Fatal("aceptó un rol inexistente")
	}
	// La guía solo nombra comandos de orq que existen.
	for _, rol := range []string{"atena", "orquestador", "planificador", "astra", "scout", "worker", "verificador"} {
		g, _ := guias.ReadFile("guias/" + rol + ".md")
		for _, l := range strings.Split(string(g), "`orq ")[1:] {
			cmd := strings.TrimRight(strings.Fields(l)[0], "`.,;)")
			if !strings.Contains(usage, "orq "+cmd) {
				t.Errorf("%s: la guía usa `orq %s`, que no existe", rol, cmd)
			}
		}
	}
}

// repoCLI: un repo de juguete con un examen TAP en rojo y un run.sh versionado que lo lanza.
func repoCLI(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "tienda")
	os.MkdirAll(filepath.Join(root, "tests"), 0o755)
	git := func(dir string, args ...string) {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
	git(root, "init", "-q", "-b", "main")
	git(root, "config", "user.email", "t@t")
	git(root, "config", "user.name", "t")
	os.WriteFile(filepath.Join(root, "total.txt"), []byte("0\n"), 0o644)
	os.WriteFile(filepath.Join(root, "tests", "datos.txt"), []byte("170\n"), 0o644)
	os.WriteFile(filepath.Join(root, "tests", "s01.sh"), []byte("if cmp -s total.txt tests/datos.txt; then echo 'ok 1 total' > out.tap; else echo 'not ok 1 total' > out.tap; exit 1; fi\n"), 0o644)
	os.WriteFile(filepath.Join(root, "run.sh"), []byte("sh tests/s01.sh\n"), 0o644)
	os.WriteFile(filepath.Join(root, ".gitignore"), []byte("out.tap\n.orq/\n"), 0o644)
	git(root, "add", "-A")
	git(root, "commit", "-qm", "rojo")
	t.Setenv("ORQ_ROOT", root)
	t.Setenv("ORQ_WORKTREES", filepath.Join(filepath.Dir(root), "wt"))
	t.Setenv("ORQ_LIMITS", filepath.Join(filepath.Dir(root), "limits.json"))
	t.Setenv("ORQ_ROL", "")
	return root
}

var abrirBase = []string{"slice", "abrir", "01", "--titulo", "total", "--test", "tests/s01.sh", "--cmd", "sh run.sh", "--bundle", "tests/datos.txt", "--sin-scouts", "e2e"}

func TestSliceAbrirExigeConteoOMotivo(t *testing.T) {
	repoCLI(t)
	if _, errs, code := run(t, abrirBase...); code != 1 || !strings.Contains(errs, "--sin-conteo") {
		t.Fatalf("code %d: %s", code, errs)
	}
	if _, errs, code := run(t, append(abrirBase, "--sin-conteo", "el runner no puede dejar reporte", "--aislado")...); code != 0 {
		t.Fatalf("--sin-conteo y --aislado (no-op) deben abrir: %d %s", code, errs)
	}
	out, _, _ := run(t, "slice", "estado")
	if !strings.Contains(out, "sin conteo: el runner no puede dejar reporte") || !strings.Contains(out, "ruta: slice") {
		t.Fatalf("estado: %s", out)
	}
}

func TestRunnerCambiadoSalePorExit3YSeAceptaConMotivo(t *testing.T) {
	root := repoCLI(t)
	if out, errs, code := run(t, append(abrirBase, "--reporte", "out.tap", "--formato", "tap")...); code != 0 || !strings.Contains(out, "Runner vigilado: run.sh") {
		t.Fatalf("%d %s %s", code, out, errs)
	}
	run(t, "slice", "tomar", "w1")
	wt := filepath.Join(filepath.Dir(root), "wt", "tienda-s01")
	os.WriteFile(filepath.Join(wt, "run.sh"), []byte("echo 'ok 1 total' > out.tap\n"), 0o644)
	for _, c := range [][]string{{"add", "-A"}, {"commit", "-qm", "runner de mentira"}} {
		cmd := exec.Command("git", c...)
		cmd.Dir = wt
		cmd.Run()
	}
	_, errs, code := run(t, "slice", "verificar")
	if code != 3 || !strings.Contains(errs, "run.sh") || !strings.Contains(errs, "--acepto-runner") {
		t.Fatalf("code %d: %s", code, errs)
	}
	if out, _, _ := run(t, "slice", "estado"); !strings.Contains(out, "fallos: 0/") {
		t.Fatalf("no debía contar como fallo: %s", out)
	}
	out, errs, code := run(t, "slice", "verificar", "--acepto-runner", "Atena aprobó el cambio")
	if code != 0 || !strings.Contains(out, "Atena aprobó el cambio") {
		t.Fatalf("code %d: %s %s", code, out, errs)
	}
	if rec, _, _ := run(t, "slice", "recibo"); !strings.Contains(rec, "Atena aprobó el cambio") {
		t.Fatalf("el recibo no lo dice: %s", rec)
	}
}

func TestRutaFixRechazaPlanYEquipoYSubeConElComando(t *testing.T) {
	repoCLI(t)
	if _, errs, code := run(t, append(abrirBase, "--reporte", "out.tap", "--formato", "tap", "--ruta", "fix")...); code != 0 {
		t.Fatalf("%d %s", code, errs)
	}
	if out, _, _ := run(t, "slice", "estado"); !strings.Contains(out, "ruta: fix") {
		t.Fatalf("estado: %s", out)
	}
	_, errs, code := run(t, "plan", "cargar", "plan.json")
	if code != 1 || !strings.Contains(errs, "orq slice ruta slice") {
		t.Fatalf("plan cargar en fix: %d %s", code, errs)
	}
	if _, errs, code := run(t, "slice", "ruta", "slice"); code != 1 {
		t.Fatalf("sin motivo debe fallar: %s", errs)
	}
	if out, errs, code := run(t, "slice", "ruta", "slice", "toca dos repos"); code != 0 || !strings.Contains(out, "sube a la ruta slice") {
		t.Fatalf("%d %s %s", code, out, errs)
	}
	if out, _, _ := run(t, "slice", "estado"); !strings.Contains(out, "ruta: slice (subió de fix: toca dos repos)") {
		t.Fatalf("estado: %s", out)
	}
	if _, errs, _ := run(t, "plan", "cargar", "plan.json"); strings.Contains(errs, "ruta fix") {
		t.Fatalf("ya no es fix: %s", errs)
	}
}

func TestMetricasPorRutaEnLaLineaDeComandos(t *testing.T) {
	repoCLI(t)
	out, _, code := run(t, "metricas")
	if code != 0 || !strings.Contains(out, "ruta fix: 0 aceptados") || !strings.Contains(out, "ruta slice: 0 aceptados") || !strings.Contains(out, "fixes que subieron a slice: 0") {
		t.Fatalf("%d %s", code, out)
	}
	if out, _, _ := run(t, "metricas", "--json"); !strings.Contains(out, "PorRuta") {
		t.Fatalf("json: %s", out)
	}
}

func TestConfigVerMuestraValorYOrigen(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ORQ_ROOT", root)
	out, errs, code := run(t, "config", "ver")
	if code != 0 || !strings.Contains(out, "roles.astra.agente") || !strings.Contains(out, "(defecto)") {
		t.Fatalf("code %d: %s %s", code, out, errs)
	}
	os.MkdirAll(filepath.Join(root, ".orq"), 0o755)
	ruta := filepath.Join(root, ".orq", "config.json")
	os.WriteFile(ruta, []byte(`{"terminal":"orca","roles":{"planificador":{"modelo":"m-plan"}}}`), 0o644)
	out, errs, code = run(t, "config", "ver")
	if code != 0 || !strings.Contains(out, "m-plan") || !strings.Contains(out, "("+ruta+")") || !strings.Contains(out, "orca") {
		t.Fatalf("code %d: %s %s", code, out, errs)
	}
	// La guía del Orquestador imprime el modelo configurado, no uno escrito en el texto.
	out, _, code = run(t, "skill", "orquestador")
	if code != 0 || !strings.Contains(out, "(m-plan)") || strings.Contains(out, "{{") {
		t.Fatalf("guía sin el modelo de la configuración (code %d): %s", code, out)
	}
	// Dos cuentas: se ve con cuál arranca cada rol.
	os.WriteFile(ruta, []byte(`{"roles":{"atena":{"entorno":{"CLAUDE_CONFIG_DIR":"/cuentas/alt"}}}}`), 0o644)
	out, errs, code = run(t, "config", "ver")
	if code != 0 || !strings.Contains(out, "cuentas:") || !strings.Contains(out, "CLAUDE_CONFIG_DIR=/cuentas/alt") || !strings.Contains(out, "la del entorno de quien lanza") {
		t.Fatalf("code %d: %s %s", code, out, errs)
	}
	// Un campo desconocido es un error, y lo dice.
	os.WriteFile(ruta, []byte(`{"terminal":"orca","modelos":{}}`), 0o644)
	_, errs, code = run(t, "config", "ver")
	if code == 0 || !strings.Contains(errs, "modelos") {
		t.Fatalf("aceptó un campo desconocido (code %d): %s", code, errs)
	}
	if _, errs, code = run(t, "mem", "ver"); code == 0 || !strings.Contains(errs, "orq config ver") {
		t.Fatalf("un comando siguió con la configuración rota (code %d): %s", code, errs)
	}
}

func TestStatusMuestraMaquinaYCapacidad(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ORQ_ROOT", dir)
	t.Setenv("ORQ_STATE", filepath.Join(dir, "state"))
	lim := filepath.Join(dir, "limits.json")
	os.WriteFile(lim, []byte(`{"min_free_gb":{"/":1},"max_workers":99,"scratch":{"paths":["`+dir+`/s"],"quota_gb":1,"critical_free_gb":1},"state_max_age_hours":24}`), 0o644)
	t.Setenv("ORQ_LIMITS", lim)
	out, errs, code := run(t, "status")
	for _, want := range []string{"máquina: RAM ", "núcleos, carga", "presupuesto por agente: 1.5 GB", "capacidad: ", "max_workers 99", "disco /"} {
		if !strings.Contains(out, want) {
			t.Errorf("falta %q en status (code %d):\n%s%s", want, code, out, errs)
		}
	}
	// Con un presupuesto imposible, status dice que no cabe nadie y por qué.
	os.MkdirAll(filepath.Join(dir, ".orq"), 0o755)
	// CPU y carga no deben limitar antes que la RAM, sea cual sea el host (agentes vivos reales).
	os.WriteFile(filepath.Join(dir, ".orq", "config.json"), []byte(`{"recursos":{"ram_por_agente_gb":100000,"cpu_por_agente":0.001,"reserva_cpus":0,"carga_max":1000000}}`), 0o644)
	out, _, _ = run(t, "status")
	if !strings.Contains(out, "limita RAM") || !strings.Contains(out, "no cabe otro agente por RAM") {
		t.Fatalf("status no explica la falta de RAM:\n%s", out)
	}
}
