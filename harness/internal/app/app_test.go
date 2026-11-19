package app

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"orq/internal/canal"
	"orq/internal/config"
	"orq/internal/diario"
	"orq/internal/limits"
	"orq/internal/plan"
	"orq/internal/slice"
	"orq/internal/sysinfo"
	"orq/internal/terminal"
	"orq/internal/terminal/fake"
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "orq-app-test-")
	if err != nil {
		panic(err)
	}
	// Nunca la configuración, los límites ni el estado reales del usuario.
	os.Setenv("ORQ_CONFIG", filepath.Join(dir, "config.json"))
	os.Setenv("ORQ_STATE", filepath.Join(dir, "state"))
	os.Setenv("ORQ_LIMITS", filepath.Join(dir, "limits.json"))
	os.Unsetenv("ORQ_ESCRIBA_MODELO")
	os.Unsetenv("ORQ_ROL")
	os.WriteFile(filepath.Join(dir, "limits.json"), []byte(`{"min_free_gb":{"/":1},"max_workers":99,"scratch":{"paths":["`+dir+`/scratch"],"quota_gb":1,"critical_free_gb":1},"state_max_age_hours":24,"protected":["tests/aceptacion/"]}`), 0o644)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// proyectoDePrueba: repo de juguete con un slice abierto en rojo y un terminal de mentira.
func proyectoDePrueba(t *testing.T, cfgJSON string) (Proyecto, *fake.Driver, *slice.Estado) {
	t.Helper()
	base := t.TempDir()
	root := filepath.Join(base, "tienda")
	os.MkdirAll(filepath.Join(root, "tests"), 0o755)
	git := func(args ...string) {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = root
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
	git("init", "-q", "-b", "main")
	git("config", "user.email", "t@t")
	git("config", "user.name", "t")
	os.WriteFile(filepath.Join(root, "tests", "s01.sh"), []byte("grep -qx ok v01.txt\n"), 0o644)
	os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".orq/\n"), 0o644)
	git("add", "-A")
	git("commit", "-qm", "rojo")
	t.Setenv("ORQ_WORKTREES", filepath.Join(base, "wt"))
	dir := filepath.Join(root, ".orq")
	os.MkdirAll(dir, 0o755)
	var rutas []string
	if cfgJSON != "" {
		ruta := filepath.Join(dir, "config.json")
		os.WriteFile(ruta, []byte(cfgJSON), 0o644)
		rutas = append(rutas, ruta)
	}
	cfg, err := config.CargarDe(rutas...)
	if err != nil {
		t.Fatal(err)
	}
	term := fake.Nuevo()
	p := Proyecto{Root: root, Dir: dir, Cfg: cfg, Term: term, Ahora: time.Now, Medir: maquinaDe(sysinfo.Maquina{MemTotalGB: 64, MemDisponibleGB: 48, CPUs: 16, Carga1: 1}, 0)}
	e, out, err := p.Slices().Abrir("01", "uno", "tests/s01.sh", "sh tests/s01.sh", slice.Opciones{SinConteo: "prueba"}, time.Now())
	if err != nil {
		t.Fatalf("abrir: %v\n%s", err, out)
	}
	return p, term, aprobarTarjeta(t, p, e.NN)
}

const tarjetaDePrueba = "## Historia\nuno\n## Fuera\nnada más\n## Examen\ntests/s01.sh pasa\n## Zonas\nv01.txt\n## Preguntas\nninguna\n"

// aprobarTarjeta deja el slice listo para lanzar en la ruta completa: tarjeta puesta y aprobada.
func aprobarTarjeta(t *testing.T, p Proyecto, nn string) *slice.Estado {
	t.Helper()
	tj, err := slice.LeerTarjeta(tarjetaDePrueba)
	if err == nil {
		_, err = p.Slices().PonerTarjeta(nn, tj, time.Now())
	}
	e, _, errA := p.Slices().AprobarTarjeta(nn, time.Now())
	if err = errors.Join(err, errA); err != nil {
		t.Fatal(err)
	}
	return e
}

func TestLanzarSinTarjetaAprobadaSeNiega(t *testing.T) {
	p, term, _ := proyectoDePrueba(t, "")
	ctx := context.Background()
	if _, _, err := p.Slices().Abrir("02", "dos", "tests/s01.sh", "sh tests/s01.sh", slice.Opciones{SinConteo: "prueba"}, time.Now()); err != nil {
		t.Fatal(err)
	}
	ped := PedidoWorker{Slice: "02", Agente: "claude", Alcance: "x", Presupuesto: "10 min"}
	if _, _, err := p.LanzarWorker(ctx, ped); err == nil || !strings.Contains(err.Error(), "orq slice tarjeta 02") {
		t.Fatalf("sin tarjeta: %v", err)
	}
	tj, _ := slice.LeerTarjeta(tarjetaDePrueba)
	if _, err := p.Slices().PonerTarjeta("02", tj, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, _, err := p.LanzarWorker(ctx, ped); err == nil || !strings.Contains(err.Error(), "orq slice aprobar 02") {
		t.Fatalf("tarjeta sin aprobar: %v", err)
	}
	if n := len(term.Ops("crear")); n != 0 {
		t.Fatalf("no se abre ningún panel sin aprobación: %d", n)
	}
	if _, _, err := p.Slices().AprobarTarjeta("02", time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, _, err := p.LanzarWorker(ctx, ped); err != nil {
		t.Fatalf("aprobada se lanza: %v", err)
	}
}

func TestLanzarFixSinTarjeta(t *testing.T) {
	p, term, _ := proyectoDePrueba(t, "")
	if _, _, err := p.Slices().Abrir("02", "dos", "tests/s01.sh", "sh tests/s01.sh", slice.Opciones{SinConteo: "prueba", Ruta: slice.RutaFix}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, _, err := p.LanzarWorker(context.Background(), PedidoWorker{Slice: "02", Agente: "claude", Alcance: "x", Presupuesto: "10 min"}); err != nil || len(term.Ops("crear")) != 1 {
		t.Fatalf("un fix se lanza sin tarjeta: %v", err)
	}
}

// maquinaDe fija la medición: las pruebas no dependen de la RAM ni de la carga de quien las corre.
func maquinaDe(m sysinfo.Maquina, vivos int) Medir {
	return Medir{
		Maquina: func() (sysinfo.Maquina, error) { return m, nil },
		Vivos:   func(limits.Limits) ([]sysinfo.Proc, error) { return make([]sysinfo.Proc, vivos), nil },
	}
}

func pasos(t *testing.T, p Proyecto, actor string) []string {
	t.Helper()
	evs, err := diario.Leer(p.DiarioPath())
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range evs {
		if e.Actor == actor {
			out = append(out, e.Paso+"="+e.Resultado)
		}
	}
	return out
}

func TestLanzarWorkerUnaVezYRepetirNoDuplica(t *testing.T) {
	p, term, e := proyectoDePrueba(t, `{"workers":{"alta":{"claude":{"modelo":"m-alto","esfuerzo":"high","entorno":{"CLAUDE_CONFIG_DIR":"/cuentas/dos"}}}}}`)
	ctx := context.Background()
	ped := PedidoWorker{Agente: "claude", Dificultad: "alta", Alcance: "x", Presupuesto: "10 min"}
	nombre, ya, err := p.LanzarWorker(ctx, ped)
	if err != nil || ya || nombre != "worker-claude-s01" {
		t.Fatalf("%q %v %v", nombre, ya, err)
	}
	crear, arrancar, avisar := term.Ops("crear"), term.Ops("arrancar"), term.Ops("avisar")
	if len(crear) != 1 || len(arrancar) != 1 || len(avisar) != 1 {
		t.Fatalf("crear=%d arrancar=%d avisar=%d", len(crear), len(arrancar), len(avisar))
	}
	if crear[0].Cwd != e.Worktree || crear[0].Env["ORQ_ROL"] != nombre || crear[0].Env["CLAUDE_CONFIG_DIR"] != "/cuentas/dos" {
		t.Fatalf("panel: %+v", crear[0])
	}
	if !slices.Contains(arrancar[0].Args, "m-alto") || !slices.Contains(arrancar[0].Args, "high") || arrancar[0].Env["CLAUDE_CONFIG_DIR"] != "/cuentas/dos" {
		t.Fatalf("el modelo, el esfuerzo y la cuenta salen de la configuración: %+v", arrancar[0])
	}
	if !strings.Contains(avisar[0].Texto, "CONTRATO S-01/a1") {
		t.Fatalf("contrato: %q", avisar[0].Texto)
	}
	ses, _ := canal.LoadSesiones(p.SesPath())
	if ses[nombre].Pane == "" {
		t.Fatalf("sesión sin panel: %+v", ses)
	}
	if got := strings.Join(pasos(t, p, nombre), " "); got != "intento= panel=ok arranque=ok contrato=ok" {
		t.Fatalf("diario: %s", got)
	}
	// El mismo comando otra vez: ni panel, ni agente, ni contrato nuevos.
	nombre2, ya, err := p.LanzarWorker(ctx, ped)
	if err != nil || !ya || nombre2 != nombre {
		t.Fatalf("repetición: %q %v %v", nombre2, ya, err)
	}
	if len(term.Ops("crear")) != 1 || len(term.Ops("arrancar")) != 1 || len(term.Ops("avisar")) != 1 {
		t.Fatalf("la repetición lanzó de nuevo: %+v", term.Llamadas)
	}
}

func TestArranqueDesconocidoNoSeRelanzaACiegas(t *testing.T) {
	p, term, _ := proyectoDePrueba(t, "")
	ctx := context.Background()
	ped := PedidoWorker{Agente: "claude"}
	// El terminal vence y el agente no aparece: desconocido. El panel no se cierra.
	term.Fallos["arrancar"] = []error{terminal.ErrDesconocido}
	nombre, _, err := p.LanzarWorker(ctx, ped)
	if !errors.Is(err, terminal.ErrDesconocido) {
		t.Fatalf("err = %v", err)
	}
	if len(term.Ops("arrancar")) != 1 || len(term.Ops("cerrar")) != 0 || len(term.Ops("avisar")) != 0 {
		t.Fatalf("tras un vencimiento no se cierra ni se reintenta: %+v", term.Llamadas)
	}
	if got := strings.Join(pasos(t, p, nombre), " "); got != "intento= panel=ok arranque=desconocido" {
		t.Fatalf("diario: %s", got)
	}
	// Mientras el terminal no sepa decir si existe, no se lanza otro.
	term.EstadoFijo[nombre] = terminal.Desconocida
	if _, _, err := p.LanzarWorker(ctx, ped); !errors.Is(err, terminal.ErrDesconocido) || len(term.Ops("arrancar")) != 1 {
		t.Fatalf("lanzó sin saber: %v", err)
	}
	// Ya se sabe que no arrancó: se cierra el panel sobrante y se lanza uno, una vez.
	delete(term.EstadoFijo, nombre)
	if _, ya, err := p.LanzarWorker(ctx, ped); err != nil || ya {
		t.Fatalf("%v %v", ya, err)
	}
	if len(term.Ops("arrancar")) != 2 || len(term.Ops("cerrar")) != 1 || len(term.Ops("crear")) != 2 || len(term.Ops("avisar")) != 1 {
		t.Fatalf("%+v", term.Llamadas)
	}
}

func TestArranqueTardioSeAdoptaYRecibeElContratoUnaVez(t *testing.T) {
	p, term, _ := proyectoDePrueba(t, "")
	ctx := context.Background()
	// El terminal venció pero el agente arrancó: Estado lo resuelve, el contrato se entrega.
	term.ArrancaTarde = true
	nombre, ya, err := p.LanzarWorker(ctx, PedidoWorker{Agente: "claude"})
	if err != nil || ya || len(term.Ops("arrancar")) != 1 || len(term.Ops("avisar")) != 1 {
		t.Fatalf("%v %v %+v", ya, err, term.Llamadas)
	}
	// Un contrato cuyo envío no respondió queda desconocido y no se repite solo…
	p2, term2, _ := proyectoDePrueba(t, "")
	term2.Fallos["avisar"] = []error{terminal.ErrDesconocido}
	n2, _, err := p2.LanzarWorker(ctx, PedidoWorker{Agente: "claude"})
	if !errors.Is(err, terminal.ErrDesconocido) {
		t.Fatalf("err = %v", err)
	}
	if _, _, err := p2.LanzarWorker(ctx, PedidoWorker{Agente: "claude"}); !errors.Is(err, terminal.ErrDesconocido) || len(term2.Ops("avisar")) != 1 {
		t.Fatalf("reenvió un contrato de resultado desconocido: %v", err)
	}
	// …hasta que quien miró el panel lo pide.
	if err := p2.Reenviar(n2); err != nil {
		t.Fatal(err)
	}
	if _, _, err := p2.LanzarWorker(ctx, PedidoWorker{Agente: "claude"}); err != nil || len(term2.Ops("avisar")) != 2 || len(term2.Ops("arrancar")) != 1 {
		t.Fatalf("%v %+v", err, term2.Llamadas)
	}
	_ = nombre
}

func TestFalloClaroCierraElPanelYDevuelveLaTarea(t *testing.T) {
	p, term, e := proyectoDePrueba(t, "")
	pl := &plan.Plan{Slice: e.NN, Debatido: true, Tareas: []plan.Tarea{{ID: "T1", Zona: "back", Rutas: []string{"src"}, Dificultad: "baja", Estado: "pendiente", Que: "api"}}}
	if err := plan.Guardar(p.PlanPath(e.NN), pl); err != nil {
		t.Fatal(err)
	}
	term.Fallos["arrancar"] = []error{errors.New("agent_pane_busy")}
	_, _, err := p.LanzarWorker(context.Background(), PedidoWorker{Agente: "claude", Tarea: "siguiente"})
	if err == nil || errors.Is(err, terminal.ErrDesconocido) {
		t.Fatalf("err = %v", err)
	}
	if len(term.Ops("cerrar")) != 1 {
		t.Fatal("un panel sin agente no se deja abierto")
	}
	if got, _ := plan.Leer(p.PlanPath(e.NN)); got.Tareas[0].Estado != "pendiente" || got.Tareas[0].Worker != "" {
		t.Fatalf("la tarea no volvió a pendiente: %+v", got.Tareas[0])
	}
	// Relanzar funciona, con el modelo de la dificultad de la tarea.
	nombre, _, err := p.LanzarWorker(context.Background(), PedidoWorker{Agente: "claude", Tarea: "T1"})
	if err != nil || nombre != "worker-back-s01" {
		t.Fatalf("%q %v", nombre, err)
	}
	m, _ := p.Cfg.Worker("claude", "baja")
	if a := term.Ops("arrancar"); !slices.Contains(a[len(a)-1].Args, m) {
		t.Fatalf("modelo de la tarea: %+v", a[len(a)-1])
	}
	if got, _ := plan.Leer(p.PlanPath(e.NN)); got.Tareas[0].Estado != "en_curso" || got.Tareas[0].Worker != nombre {
		t.Fatalf("%+v", got.Tareas[0])
	}
}

// El Owner usa dos cuentas de Claude: cada rol arranca con la suya.
func TestDosRolesArrancanConCuentasDistintas(t *testing.T) {
	p, term, _ := proyectoDePrueba(t, `{"roles":{
	  "orquestador":{"entorno":{"CLAUDE_CONFIG_DIR":"/cuentas/uno"}},
	  "planificador":{"entorno":{"CLAUDE_CONFIG_DIR":"/cuentas/dos"}}}}`)
	ctx := context.Background()
	ses := map[string]canal.Sesion{}
	if _, usado, err := p.LanzarRol(ctx, ses, "orquestador", config.Agente{}); err != nil || usado.Entorno["CLAUDE_CONFIG_DIR"] != "/cuentas/uno" {
		t.Fatalf("%+v %v", usado, err)
	}
	if _, _, err := p.LanzarRol(ctx, ses, "planificador", config.Agente{}); err != nil {
		t.Fatal(err)
	}
	// Astra no tiene cuenta configurada: no recibe la de otro rol.
	if _, _, err := p.LanzarRol(ctx, ses, "astra", config.Agente{}); err != nil {
		t.Fatal(err)
	}
	crear, arrancar := term.Ops("crear"), term.Ops("arrancar")
	if len(crear) != 3 || len(arrancar) != 3 {
		t.Fatalf("%+v", term.Llamadas)
	}
	uno, dos := crear[0].Env["CLAUDE_CONFIG_DIR"], crear[1].Env["CLAUDE_CONFIG_DIR"]
	if uno != "/cuentas/uno" || dos != "/cuentas/dos" || uno == dos {
		t.Fatalf("orquestador %q, planificador %q", uno, dos)
	}
	if arrancar[0].Env["CLAUDE_CONFIG_DIR"] != uno || arrancar[1].Env["CLAUDE_CONFIG_DIR"] != dos {
		t.Fatalf("el arranque no lleva la cuenta: %+v %+v", arrancar[0].Env, arrancar[1].Env)
	}
	if _, hay := crear[2].Env["CLAUDE_CONFIG_DIR"]; hay || crear[2].Env["ORQ_ROL"] != "astra" {
		t.Fatalf("astra: %+v", crear[2].Env)
	}
	// Astra abre al lado del Planificador, en su pestaña.
	if crear[2].Destino != ses["planificador"].Pane || ses["astra"].Tab != ses["planificador"].Tab {
		t.Fatalf("astra junto a %q; sesiones %+v", crear[2].Destino, ses)
	}
	// El escriba sin sesión usa la cuenta de su rol.
	if e := p.Cfg.Rol("escriba"); len(e.Entorno) != 0 {
		t.Fatalf("escriba = %+v", e)
	}
}

func TestFueraDelTerminalNoSeLanza(t *testing.T) {
	p, term, _ := proyectoDePrueba(t, "")
	term.EstaDentro = false
	if _, _, err := p.LanzarWorker(context.Background(), PedidoWorker{Agente: "claude"}); !errors.Is(err, ErrFueraDelTerminal) || len(term.Ops("crear")) != 0 {
		t.Fatalf("%v", err)
	}
}

// El Orquestador lanza desde otro repo: queda registrado ahí para que el worker le avise directo.
func TestRegistrarLlamador(t *testing.T) {
	ses := map[string]canal.Sesion{"worker-back-s02": {Agente: "worker-back-s02", Pane: "w3D:pX"}}
	RegistrarLlamador(ses, "orquestador", "w3D:pQ", time.Now())
	if s := ses["orquestador"]; s.Agente != "w3D:pQ" || s.Pane != "w3D:pQ" {
		t.Fatalf("%+v", s)
	}
	propio := map[string]canal.Sesion{"orquestador": {Agente: "orquestador-x-3", Pane: "w3D:pQ", Gen: 3}}
	RegistrarLlamador(propio, "orquestador", "w3D:pQ", time.Now())
	if propio["orquestador"].Agente != "orquestador-x-3" {
		t.Fatal("en su propio repo no se pisa la sesión")
	}
	RegistrarLlamador(ses, "worker-front-s04", "w3D:pV", time.Now())
	if _, ok := ses["worker-front-s04"]; ok {
		t.Fatal("un worker no se registra como destino")
	}
}

// slice lanzar mira la máquina: sin RAM, sin CPU o con la carga alta no lanza, y dice qué número lo decide.
func TestLanzarWorkerRespetaLaCapacidadDeLaMaquina(t *testing.T) {
	p, term, _ := proyectoDePrueba(t, `{"recursos":{"ram_por_agente_gb":2,"reserva_ram_gb":2,"cpu_por_agente":1,"reserva_cpus":1,"carga_max":1.5}}`)
	ctx := context.Background()
	for nombre, c := range map[string]struct {
		m     sysinfo.Maquina
		vivos int
		dice  string
	}{
		"RAM":   {sysinfo.Maquina{MemTotalGB: 8, MemDisponibleGB: 3.5, CPUs: 8}, 0, "por RAM"},
		"CPU":   {sysinfo.Maquina{MemTotalGB: 64, MemDisponibleGB: 40, CPUs: 1}, 0, "por CPU"},
		"carga": {sysinfo.Maquina{MemTotalGB: 64, MemDisponibleGB: 40, CPUs: 4, Carga1: 10}, 1, "máquina cargada"},
	} {
		p.Medir = maquinaDe(c.m, c.vivos)
		_, _, err := p.LanzarWorker(ctx, PedidoWorker{Agente: "claude"})
		if err == nil || !strings.Contains(err.Error(), c.dice) {
			t.Errorf("%s: err = %v", nombre, err)
		}
	}
	if len(term.Ops("crear")) != 0 {
		t.Fatalf("lanzó sin capacidad: %+v", term.Llamadas)
	}
	// Sin poder medir, tampoco se lanza.
	p.Medir = Medir{Maquina: func() (sysinfo.Maquina, error) { return sysinfo.Maquina{}, errors.New("sin /proc") },
		Vivos: func(limits.Limits) ([]sysinfo.Proc, error) { return nil, nil }}
	if _, _, err := p.LanzarWorker(ctx, PedidoWorker{Agente: "claude"}); err == nil || !strings.Contains(err.Error(), "sin medir") {
		t.Fatalf("err = %v", err)
	}
	// Con sitio, lanza. Y un worker que ya vive se puede «relanzar» (no ocupa un cupo nuevo) aunque no quepa otro.
	p.Medir = maquinaDe(sysinfo.Maquina{MemTotalGB: 8, MemDisponibleGB: 4.5, CPUs: 8}, 0)
	if _, _, err := p.LanzarWorker(ctx, PedidoWorker{Agente: "claude"}); err != nil {
		t.Fatal(err)
	}
	p.Medir = maquinaDe(sysinfo.Maquina{MemTotalGB: 8, MemDisponibleGB: 1, CPUs: 8}, 1)
	if _, ya, err := p.LanzarWorker(ctx, PedidoWorker{Agente: "claude"}); err != nil || !ya {
		t.Fatalf("repetir con la máquina llena no debe fallar: %v %v", ya, err)
	}
}

func TestCodexNoSeLanza(t *testing.T) {
	p, term, _ := proyectoDePrueba(t, "")
	ctx := context.Background()
	if _, _, err := p.LanzarWorker(ctx, PedidoWorker{Agente: "codex"}); err == nil || !strings.Contains(err.Error(), "regla del Owner") {
		t.Fatalf("worker codex: %v", err)
	}
	if _, _, err := p.LanzarRol(ctx, nil, "astra", config.Agente{Kind: "codex"}); err == nil || !strings.Contains(err.Error(), "regla del Owner") {
		t.Fatalf("astra codex: %v", err)
	}
	if len(term.Ops("crear")) != 0 {
		t.Fatalf("opened a pane: %+v", term.Llamadas)
	}
}

func TestSlicesLlevaLasDefiniciones(t *testing.T) {
	p, _, _ := proyectoDePrueba(t, `{"checks":{"lint":{"cmd":"true"},"types":{"omitir":"sin tipos"},"tests":{"cmd":"true"},"definiciones":{"archivos":["Makefile"]}}}`)
	if got := p.Slices().Definiciones; len(got) != 1 || got[0] != "Makefile" {
		t.Fatalf("definiciones = %v", got)
	}
}

func TestSlicesLlevaLosChecksDelProyecto(t *testing.T) {
	p, _, _ := proyectoDePrueba(t, `{"checks":{"lint":{"cmd":"true"},"types":{"omitir":"sin tipos"},"tests":{"cmd":"go test ./..."}}}`)
	cs := p.Slices().Checks
	if len(cs) != 4 || cs[3].Nombre != config.Supresiones || cs[0].Nombre != "lint" || cs[1].Omitir != "sin tipos" || cs[2].Cmd != "go test ./..." {
		t.Fatalf("checks = %+v", cs)
	}
}

// reescribirEstado edita a mano .orq/slices/NN.json, como lo haría quien no pasa por orq.
func reescribirEstado(t *testing.T, p Proyecto, nn, de, a string) {
	t.Helper()
	f := filepath.Join(p.Slices().OrqDir, "slices", nn+".json")
	raw, err := os.ReadFile(f)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), de) {
		t.Fatalf("%s no contiene %q", f, de)
	}
	if err := os.WriteFile(f, []byte(strings.Replace(string(raw), de, a, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLanzarNoConfiaEnAprobacionForjada(t *testing.T) {
	p, term, _ := proyectoDePrueba(t, "")
	if _, _, err := p.Slices().Abrir("02", "dos", "tests/s01.sh", "sh tests/s01.sh", slice.Opciones{SinConteo: "prueba"}, time.Now()); err != nil {
		t.Fatal(err)
	}
	tj, _ := slice.LeerTarjeta(tarjetaDePrueba)
	if _, err := p.Slices().PonerTarjeta("02", tj, time.Now()); err != nil {
		t.Fatal(err)
	}
	reescribirEstado(t, p, "02", `"aprobada": "0001-01-01T00:00:00Z"`, `"aprobada": "2026-01-01T00:00:00Z"`)
	_, _, err := p.LanzarWorker(context.Background(), PedidoWorker{Slice: "02", Agente: "claude", Alcance: "x", Presupuesto: "10 min"})
	if !errors.Is(err, slice.ErrEstadoForjado) || len(term.Ops("crear")) != 0 {
		t.Fatalf("aprobación forjada debe negarse: %v, paneles %d", err, len(term.Ops("crear")))
	}
}

func TestLanzarNoConfiaEnRutaRebajadaAFix(t *testing.T) {
	p, term, _ := proyectoDePrueba(t, "")
	if _, _, err := p.Slices().Abrir("02", "dos", "tests/s01.sh", "sh tests/s01.sh", slice.Opciones{SinConteo: "prueba"}, time.Now()); err != nil {
		t.Fatal(err)
	}
	reescribirEstado(t, p, "02", `"nn": "02"`, `"nn": "02", "ruta": "fix"`)
	_, _, err := p.LanzarWorker(context.Background(), PedidoWorker{Slice: "02", Agente: "claude", Alcance: "x", Presupuesto: "10 min"})
	if !errors.Is(err, slice.ErrEstadoForjado) || !strings.Contains(err.Error(), "ruta") || len(term.Ops("crear")) != 0 {
		t.Fatalf("ruta rebajada debe negarse: %v, paneles %d", err, len(term.Ops("crear")))
	}
}

func TestSubirRutaSigueSiendoValida(t *testing.T) {
	p, _, _ := proyectoDePrueba(t, "")
	if _, _, err := p.Slices().Abrir("02", "dos", "tests/s01.sh", "sh tests/s01.sh", slice.Opciones{SinConteo: "prueba", Ruta: slice.RutaFix}, time.Now()); err != nil {
		t.Fatal(err)
	}
	e, err := p.Slices().CambiarRuta("02", slice.RutaSlice, "creció")
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Slices().Confiar(e); err != nil {
		t.Fatalf("subir la ruta re-sella el contrato: %v", err)
	}
}
