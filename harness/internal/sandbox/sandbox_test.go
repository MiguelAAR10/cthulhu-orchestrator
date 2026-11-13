package sandbox

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	// Nunca el estado ni los límites reales de orq.
	d, err := os.MkdirTemp("", "orq-sandbox-test-")
	if err != nil {
		panic(err)
	}
	os.Setenv("ORQ_STATE", filepath.Join(d, "state"))
	os.Setenv("ORQ_LIMITS", filepath.Join(d, "no-existe.json"))
	code := m.Run()
	os.RemoveAll(d)
	os.Exit(code)
}

// hostFalso: una máquina sin sorpresas para probar la construcción de argumentos.
func hostFalso(t *testing.T) (host, string) {
	t.Helper()
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home", "u")
	if err := os.MkdirAll(filepath.Join(home, ".config", "herdr"), 0o755); err != nil {
		t.Fatal(err)
	}
	return host{
		Home:        home,
		BaseRO:      []string{"/usr", "/etc"},
		Enlaces:     [][2]string{{"usr/bin", "/bin"}},
		Resolv:      "/mnt/wsl/resolv.conf",
		PathHost:    "/home/u/bin:" + filepath.Join(home, ".nvm", "bin") + ":/usr/bin:/opt/x/bin",
		Sensibles:   []string{filepath.Join(home, ".config", "herdr"), "/run", "/var/run", "/proc", "/dev"},
		SensiblesRW: []string{filepath.Join(tmp, "state"), filepath.Join(home, ".config", "orq")},
	}, tmp
}

func mkdir(t *testing.T, p string) string {
	t.Helper()
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

// par busca la secuencia flag a b en los argumentos.
func par(a []string, flag, x, y string) int {
	for i := 0; i+2 < len(a); i++ {
		if a[i] == flag && a[i+1] == x && a[i+2] == y {
			return i
		}
	}
	return -1
}

func TestConstruirListaPermitida(t *testing.T) {
	h, tmp := hostFalso(t)
	w := mkdir(t, filepath.Join(tmp, "w"))
	a, env, err := construir(Spec{RW: []string{w}, Cwd: w}, h, []string{"sh", "-c", "true"})
	if err != nil {
		t.Fatal(err)
	}
	if par(a, "--ro-bind", "/", "/") >= 0 || par(a, "--bind", "/", "/") >= 0 {
		t.Fatalf("la raíz del host nunca se monta: %v", a)
	}
	for _, want := range []string{"--unshare-all", "--die-with-parent", "--remount-ro"} {
		if !slices.Contains(a, want) {
			t.Fatalf("falta %s: %v", want, a)
		}
	}
	if slices.Contains(a, "--share-net") || par(a, "--ro-bind", h.Resolv, h.Resolv) >= 0 {
		t.Fatalf("sin Net no hay red ni resolv: %v", a)
	}
	if slices.Contains(a, "--file") || par(a, "--ro-bind", "/dev/null", Marca) < 0 {
		t.Fatalf("la marca va con --ro-bind /dev/null (--file cuelga con una tty): %v", a)
	}
	for _, d := range []string{"/tmp", "/run", h.Home} {
		i := slices.Index(a, d)
		if i < 1 || a[i-1] != "--tmpfs" {
			t.Fatalf("%s debe ser tmpfs: %v", d, a)
		}
	}
	if par(a, "--ro-bind", "/usr", "/usr") < 0 || par(a, "--symlink", "usr/bin", "/bin") < 0 || par(a, "--bind", w, w) < 0 {
		t.Fatalf("faltan montajes base o RW: %v", a)
	}
	for _, x := range a {
		if strings.HasPrefix(x, "/mnt") {
			t.Fatalf("/mnt no entra sin red: %v", a)
		}
	}
	sep := slices.Index(a, "--")
	if sep < 0 || !slices.Equal(a[sep+1:], []string{"sh", "-c", "true"}) || a[sep-2] != "--chdir" || a[sep-1] != w {
		t.Fatalf("comando o cwd mal puestos: %v", a)
	}
	// El tmpfs de $HOME va antes que cualquier montaje dentro de él.
	if slices.Index(a, h.Home) > par(a, "--bind", w, w) {
		t.Fatalf("tmpfs después de montajes: %v", a)
	}
	_ = env
}

func TestConstruirRed(t *testing.T) {
	h, _ := hostFalso(t)
	a, _, err := construir(Spec{Net: true}, h, []string{"true"})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(a, "--share-net") || par(a, "--ro-bind", h.Resolv, h.Resolv) < 0 {
		t.Fatalf("con Net se comparte la red y se monta resolv.conf: %v", a)
	}
	if a[slices.Index(a, "--")-1] != h.Home {
		t.Fatalf("sin Cwd se trabaja en $HOME: %v", a)
	}
}

func TestConstruirEntornoLimpio(t *testing.T) {
	h, tmp := hostFalso(t)
	nvm := mkdir(t, filepath.Join(h.Home, ".nvm"))
	h.ExtraRO = []string{nvm}
	_ = tmp
	t.Setenv("ORQ_SECRETO_DEL_LANZADOR", "x")
	_, env, err := construir(Spec{Env: map[string]string{"TERM": "xterm", "HOME": "/otro"}}, h, []string{"true"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"HOME=" + h.Home, "PATH=" + filepath.Join(h.Home, ".nvm", "bin") + ":/usr/bin:/usr/local/bin:/bin", "TERM=xterm"}
	if !slices.Equal(env, want) {
		t.Fatalf("entorno:\n got %v\nwant %v", env, want)
	}
	_, env, _ = construir(Spec{Env: map[string]string{"PATH": "/shim:/usr/bin"}}, h, []string{"true"})
	if !slices.Contains(env, "PATH=/shim:/usr/bin") {
		t.Fatalf("un PATH explícito se respeta: %v", env)
	}
	if _, _, err := construir(Spec{Env: map[string]string{"A=B": "c"}}, h, []string{"true"}); err == nil {
		t.Fatal("debía rechazar una variable con =")
	}
}

func TestConstruirPadresAntesQueHijos(t *testing.T) {
	h, tmp := hostFalso(t)
	w := mkdir(t, filepath.Join(tmp, "w"))
	nm := mkdir(t, filepath.Join(w, "node_modules"))
	a, _, err := construir(Spec{RO: []string{nm}, RW: []string{w}}, h, []string{"true"})
	if err != nil {
		t.Fatal(err)
	}
	if i, j := par(a, "--bind", w, w), par(a, "--ro-bind", nm, nm); i < 0 || j < 0 || i > j {
		t.Fatalf("el RO hijo debe montarse después del RW padre: %v", a)
	}
}

func TestConstruirRechazaRutasPeligrosas(t *testing.T) {
	h, tmp := hostFalso(t)
	state := mkdir(t, filepath.Join(tmp, "state"))
	ok := mkdir(t, filepath.Join(tmp, "ok"))
	enlace := filepath.Join(tmp, "enlace-home")
	if err := os.Symlink(h.Home, enlace); err != nil {
		t.Fatal(err)
	}
	casos := []struct {
		nombre string
		s      Spec
	}{
		{"relativa", Spec{RW: []string{"w"}}},
		{"raíz", Spec{RO: []string{"/"}}},
		{"home entero", Spec{RO: []string{h.Home}}},
		{"ancestro de home", Spec{RO: []string{filepath.Dir(h.Home)}}},
		{"enlace a home", Spec{RO: []string{enlace}}},
		{"contiene el socket de herdr", Spec{RO: []string{filepath.Join(h.Home, ".config")}}},
		{"/run", Spec{RO: []string{"/run"}}},
		{"estado de orq escribible", Spec{RW: []string{state}}},
		{"inexistente", Spec{RO: []string{filepath.Join(tmp, "nada")}}},
		{"RO y RW a la vez", Spec{RO: []string{ok}, RW: []string{ok}}},
		{"cwd relativo", Spec{Cwd: "w"}},
		{"sandbox.ro peligroso", Spec{}},
	}
	for _, c := range casos {
		hh := h
		if c.nombre == "sandbox.ro peligroso" {
			hh.ExtraRO = []string{filepath.Join(h.Home, ".config")}
		}
		if _, _, err := construir(c.s, hh, []string{"true"}); err == nil {
			t.Errorf("%s: debía fallar", c.nombre)
		}
	}
	// El estado sí puede verse de solo lectura.
	if _, _, err := construir(Spec{RO: []string{state}}, h, []string{"true"}); err != nil {
		t.Fatalf("el estado de solo lectura debía aceptarse: %v", err)
	}
	if _, _, err := construir(Spec{}, h, nil); err == nil {
		t.Fatal("sin comando debía fallar")
	}
}

func TestSinBwrapNoHayCaja(t *testing.T) {
	viejo := rutasBwrap
	rutasBwrap = []string{filepath.Join(t.TempDir(), "bwrap")}
	defer func() { rutasBwrap = viejo }()
	if _, err := Sondas(); err == nil {
		t.Fatal("sin bwrap la autoprueba debía fallar")
	}
	if err := disponible(); err == nil {
		t.Fatal("sin bwrap no hay caja disponible")
	}
}

// exigeBwrap: sin bwrap estas pruebas fallan; solo ORQ_SIN_BWRAP=1 las salta.
func exigeBwrap(t *testing.T) {
	t.Helper()
	if _, err := buscarBwrap(); err != nil {
		if os.Getenv("ORQ_SIN_BWRAP") == "1" {
			t.Skip("ORQ_SIN_BWRAP=1: sin bwrap")
		}
		t.Fatalf("%v (ORQ_SIN_BWRAP=1 para saltar a propósito)", err)
	}
}

func TestAutoprueba(t *testing.T) {
	exigeBwrap(t)
	ss, err := Sondas()
	if err != nil {
		t.Fatal(err)
	}
	if len(ss) < 11 {
		t.Fatalf("faltan sondas: %v", ss)
	}
	for _, s := range ss {
		if !s.OK {
			t.Errorf("sonda %q falla: %s", s.Nombre, s.Detalle)
		}
	}
	if err := Disponible(); err != nil {
		t.Fatal(err)
	}
}

func TestCommandDentroYFuera(t *testing.T) {
	exigeBwrap(t)
	if Dentro() {
		t.Fatal("la prueba no debería correr dentro de una caja")
	}
	w := t.TempDir()
	cmd, err := Command(context.Background(), Spec{RW: []string{w}, Cwd: w}, "sh", "-c", `test -c /run/orq/sandbox && pwd > salida && exit 7`)
	if err != nil {
		t.Fatal(err)
	}
	err = cmd.Run()
	var ee *exec.ExitError
	if !asExit(err, &ee) || ee.ExitCode() != 7 {
		t.Fatalf("el código de salida debía propagarse (7): %v", err)
	}
	raw, _ := os.ReadFile(filepath.Join(w, "salida"))
	if strings.TrimSpace(string(raw)) != w {
		t.Fatalf("cwd dentro de la caja: %q", raw)
	}
	if _, err := Command(context.Background(), Spec{}); err == nil {
		t.Fatal("sin comando debía fallar")
	}
}

func asExit(err error, ee **exec.ExitError) bool {
	e, ok := err.(*exec.ExitError)
	*ee = e
	return ok
}

// Control negativo: una caja con la raíz del host montada debe hacer fallar la autoprueba.
func TestAutopruebaDetectaCajaRota(t *testing.T) {
	exigeBwrap(t)
	viejo := hostFn
	defer func() { hostFn = viejo }()
	hostFn = func() (host, error) {
		h, err := hostReal()
		h.BaseRO = append(h.BaseRO, "/")
		return h, err
	}
	ss, err := Sondas()
	if err != nil {
		t.Fatal(err)
	}
	malas := 0
	for _, s := range ss {
		if !s.OK {
			malas++
		}
		if s.Nombre == "ruta no permitida ausente" && s.OK {
			t.Fatal("con / montado, la ruta no permitida debía verse")
		}
	}
	if malas == 0 {
		t.Fatal("una caja rota pasó la autoprueba")
	}
	if err := disponible(); err == nil {
		t.Fatal("una caja rota no puede estar disponible")
	}
	t.Logf("%d sondas fallan con / montado", malas)
}
