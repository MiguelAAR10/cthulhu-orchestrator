package slice

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var t0 = time.Date(2026, 10, 26, 10, 0, 0, 0, time.UTC)

const examen = "sh tests/s01.sh"

// repoDeJuguete: el examen pasa cuando total.txt dice 170; tests/datos.txt es un fixture.
func repoDeJuguete(t *testing.T) Repo {
	t.Helper()
	base := t.TempDir()
	root := filepath.Join(base, "tienda")
	os.MkdirAll(filepath.Join(root, "tests"), 0o755)
	for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"config", "user.email", "t@t"}, {"config", "user.name", "t"}} {
		if _, err := Git(root, args...); err != nil {
			t.Fatal(err)
		}
	}
	os.WriteFile(filepath.Join(root, "total.txt"), []byte("0\n"), 0o644)
	os.WriteFile(filepath.Join(root, "tests", "datos.txt"), []byte("170\n"), 0o644)
	os.WriteFile(filepath.Join(root, "tests", "s01.sh"), []byte("cmp -s total.txt tests/datos.txt\n"), 0o644)
	Git(root, "add", "-A")
	Git(root, "commit", "-qm", "rojo")
	orq := filepath.Join(root, ".orq")
	os.MkdirAll(orq, 0o755)
	os.WriteFile(filepath.Join(root, ".git", "info", "exclude"), []byte(".orq/\n"), 0o644)
	return Repo{Root: root, OrqDir: orq, Worktree: filepath.Join(base, "wt")}
}

func commitEn(t *testing.T, dir, file, body string) {
	t.Helper()
	os.WriteFile(filepath.Join(dir, file), []byte(body), 0o644)
	Git(dir, "add", "-A")
	if _, err := Git(dir, "commit", "-qm", "cambio"); err != nil {
		t.Fatal(err)
	}
}

func abrir(t *testing.T, r Repo) *Estado {
	t.Helper()
	e, _, err := r.Abrir("01", "total", "tests/s01.sh", examen, Opciones{Bundle: []string{"tests/datos.txt"}, Sano: "test -f total.txt"}, t0)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func TestCicloCompletoAceptado(t *testing.T) {
	r := repoDeJuguete(t)
	e := abrir(t, r)
	if e.Fase != Implementando || e.Attempt() != "S-01/a1" || e.Base != "main" {
		t.Fatalf("got %+v", e)
	}
	if _, _, err := r.Abrir("01", "otro", "tests/s01.sh", examen, Opciones{}, t0); err == nil {
		t.Fatal("abrió dos veces el mismo slice")
	}
	if _, _, _, err := r.Cerrar("", true, "", t0); err == nil {
		t.Fatal("cerró sin recibo")
	}
	commitEn(t, e.Worktree, "total.txt", "170\n")
	e, _, err := r.Verificar("", t0)
	if err != nil || e.Fase != Verificado || e.Recibo.Attempt != "S-01/a1" || e.Recibo.Exit != 0 {
		t.Fatalf("%+v %v", e, err)
	}
	reg, q, _, err := r.Cerrar("", true, "", t0)
	if err != nil || reg.Result != "aceptado" || len(q) != 0 {
		t.Fatalf("%+v %v %v", reg, q, err)
	}
	if got, _ := os.ReadFile(filepath.Join(r.Root, "total.txt")); string(got) != "170\n" {
		t.Fatal("la rama principal no avanzó")
	}
	for _, p := range []string{e.Worktree, e.Worktree + "-int"} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("sigue existiendo %s", p)
		}
	}
	if out, _ := Git(r.Root, "branch", "--list", "slice/01"); out != "" {
		t.Fatal("la rama sigue existiendo")
	}
	if h, _ := r.Huerfanos(); len(h) != 0 {
		t.Fatalf("huérfanos: %v", h)
	}
}

func TestBaseSanaYRojoSonDistintos(t *testing.T) {
	r := repoDeJuguete(t)
	if _, _, err := r.Abrir("01", "t", "tests/s01.sh", examen, Opciones{Sano: "false"}, t0); err == nil || !strings.Contains(err.Error(), "sana") {
		t.Fatalf("got %v", err)
	}
	commitEn(t, r.Root, "total.txt", "170\n")
	if _, _, err := r.Abrir("01", "t", "tests/s01.sh", examen, Opciones{}, t0); err == nil || !strings.Contains(err.Error(), "rojo") {
		t.Fatalf("got %v", err)
	}
}

func TestDescubreAntesDeCrear(t *testing.T) {
	r := repoDeJuguete(t)
	Git(r.Root, "branch", "slice/01")
	if _, _, err := r.Abrir("01", "t", "tests/s01.sh", examen, Opciones{}, t0); err == nil || !strings.Contains(err.Error(), "ya existe") {
		t.Fatalf("got %v", err)
	}
}

func TestUnSoloEscritor(t *testing.T) {
	r := repoDeJuguete(t)
	abrir(t, r)
	if _, err := r.Tomar("", "worker-1", false); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Tomar("", "worker-2", false); err == nil {
		t.Fatal("dos escritores en el mismo slice")
	}
	e, err := r.Tomar("", "worker-2", true)
	if err != nil || e.Escritor != "worker-2" {
		t.Fatalf("el relevo no heredó el slice: %v", err)
	}
}

func TestFixtureTocadoEsTrampaYBloquea(t *testing.T) {
	r := repoDeJuguete(t)
	e := abrir(t, r)
	commitEn(t, e.Worktree, "tests/datos.txt", "0\n")
	_, _, err := r.Verificar("", t0)
	if err == nil || !strings.Contains(err.Error(), "trampa") {
		t.Fatalf("got %v", err)
	}
	if e, _ = r.Load(); e.Fase != Bloqueado {
		t.Fatalf("fase %s", e.Fase)
	}
}

func TestSinProgresoBloqueaYReintentarInvalidaRecibosViejos(t *testing.T) {
	r := repoDeJuguete(t)
	e := abrir(t, r)
	if _, _, err := r.Verificar("", t0); err == nil || strings.Contains(err.Error(), "BLOQUEADO") {
		t.Fatalf("el primer fallo no debe bloquear: %v", err)
	}
	if _, _, err := r.Verificar("", t0); err == nil || !strings.Contains(err.Error(), "sin progreso") {
		t.Fatalf("got %v", err)
	}
	if _, err := r.Reintentar("", "", false); err == nil {
		t.Fatal("reintentó sin decir qué cambia")
	}
	e, err := r.Reintentar("", "leer el total desde el fixture", false)
	if err != nil || e.Attempt() != "S-01/a2" || e.Fase != Implementando {
		t.Fatalf("%+v %v", e, err)
	}
	commitEn(t, e.Worktree, "total.txt", "170\n")
	if e, _, err = r.Verificar("", t0); err != nil || e.Recibo.Attempt != "S-01/a2" {
		t.Fatalf("%+v %v", e, err)
	}
	// Un commit posterior al recibo lo deja viejo: no se integra.
	commitEn(t, e.Worktree, "otro.txt", "x\n")
	if _, _, _, err := r.Cerrar("", true, "", t0); err == nil || !strings.Contains(err.Error(), "recibo") {
		t.Fatalf("got %v", err)
	}
}

// Interrupción: el dueño tiene cambios sin commit cuando se cierra. Nada se pierde,
// y al repetir el cierre continúa sin integrar dos veces.
func TestCierreInterrumpidoSeRetoma(t *testing.T) {
	r := repoDeJuguete(t)
	e := abrir(t, r)
	commitEn(t, e.Worktree, "total.txt", "170\n")
	r.Verificar("", t0)
	os.WriteFile(filepath.Join(r.Root, "README"), []byte("borrador del dueño\n"), 0o644)
	Git(r.Root, "add", "README")
	if _, _, _, err := r.Cerrar("", true, "", t0); err == nil || !strings.Contains(err.Error(), "continúa") {
		t.Fatalf("got %v", err)
	}
	if e, _ = r.Load(); e.Fase != Cerrando || e.Integrado == "" {
		t.Fatalf("no recordó el paso: %+v", e)
	}
	if got, _ := os.ReadFile(filepath.Join(r.Root, "README")); string(got) != "borrador del dueño\n" {
		t.Fatal("tocó el trabajo del dueño")
	}
	Git(r.Root, "commit", "-qm", "dueño")
	// La rama del dueño avanzó: no es fast-forward; el cierre vuelve solo a integrar.
	if _, _, _, err := r.Cerrar("", true, "", t0); err == nil {
		t.Fatal("publicó encima de un commit nuevo del dueño")
	}
	if e, _ = r.Load(); e.Fase != Integrando {
		t.Fatalf("quedó atascado en %s", e.Fase)
	}
	reg, _, _, err := r.Cerrar("", true, "", t0)
	if err != nil || reg.Result != "aceptado" {
		t.Fatalf("%+v %v", reg, err)
	}
	log, _ := Git(r.Root, "log", "--oneline")
	if strings.Count(log, "slice S-01") != 1 {
		t.Fatalf("integró más de una vez:\n%s", log)
	}
}

func TestWorktreeSucioVaACuarentena(t *testing.T) {
	r := repoDeJuguete(t)
	e := abrir(t, r)
	os.WriteFile(filepath.Join(e.Worktree, "nota.txt"), []byte("sin commit\n"), 0o644)
	reg, q, _, err := r.Cerrar("", false, "no era lo pedido", t0)
	if err != nil || reg.Result != "rechazado" || len(q) != 1 {
		t.Fatalf("%+v %v %v", reg, q, err)
	}
	if _, err := os.Stat(filepath.Join(e.Worktree, "nota.txt")); err != nil {
		t.Fatal("borró trabajo sin commit")
	}
	if out, _ := Git(r.Root, "branch", "--list", "slice/01"); out == "" {
		t.Fatal("se borró la rama rechazada")
	}
}

func TestIntegraElSHADelRecibo(t *testing.T) {
	r := repoDeJuguete(t)
	e := abrir(t, r)
	commitEn(t, e.Worktree, "total.txt", "170\n")
	r.Verificar("", t0)
	e, _ = r.Load()
	e.Fase = Integrando // cierre interrumpido justo antes de integrar
	r.save(e)
	commitEn(t, e.Worktree, "extra.txt", "sin verificar\n")
	if _, _, _, err := r.Cerrar("", true, "", t0); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(r.Root, "extra.txt")); err == nil {
		t.Fatal("integró un commit posterior al recibo")
	}
}

func TestRechazarQuitaWorktreeIntegracionYNoTrasPublicar(t *testing.T) {
	r := repoDeJuguete(t)
	e := abrir(t, r)
	os.MkdirAll(e.Worktree+"-int", 0o755)
	Git(r.Root, "worktree", "add", "--detach", e.Worktree+"-int2", "main")
	if _, _, _, err := r.Cerrar("", false, "no", t0); err != nil {
		t.Fatal(err)
	}
	if existe(e.Worktree) {
		t.Fatal("no quitó el worktree")
	}
	r2 := repoDeJuguete(t)
	e2 := abrir(t, r2)
	commitEn(t, e2.Worktree, "total.txt", "170\n")
	r2.Verificar("", t0)
	e2, _ = r2.Load()
	e2.Fase = Cerrando
	r2.save(e2)
	if _, _, _, err := r2.Cerrar("", false, "tarde", t0); err == nil {
		t.Fatal("rechazó un slice que ya se estaba publicando")
	}
}

func TestQuitarNoHacePruneGlobal(t *testing.T) {
	r := repoDeJuguete(t)
	ajeno := filepath.Join(filepath.Dir(r.Root), "ajeno")
	Git(r.Root, "worktree", "add", "--detach", ajeno, "main")
	os.Rename(ajeno, ajeno+"-movido") // un worktree ajeno quedó "prunable"
	e := abrir(t, r)
	r.Cerrar("", false, "no", t0)
	if out, _ := Git(r.Root, "worktree", "list"); !strings.Contains(out, "ajeno") {
		t.Fatalf("borró metadatos de un worktree ajeno:\n%s", out)
	}
	_ = e
}

// repoConReporte: el examen escribe un reporte TAP en out.tap.
func repoConReporte(t *testing.T, script string) Repo {
	t.Helper()
	r := repoDeJuguete(t)
	commitEn(t, r.Root, "tests/s01.sh", script)
	return r
}

const tapReal = `if cmp -s total.txt tests/datos.txt; then printf 'ok 1 total\n' > out.tap; else printf 'not ok 1 total\n' > out.tap; exit 1; fi
`

func abrirConReporte(r Repo, o Opciones) (*Estado, string, error) {
	o.Reporte, o.Formato = "out.tap", "tap"
	return r.Abrir("01", "total", "tests/s01.sh", examen, o, t0)
}

func TestVerdeSinPruebasEjecutadasFalla(t *testing.T) {
	r := repoConReporte(t, tapReal)
	e, _, err := abrirConReporte(r, Opciones{})
	if err != nil {
		t.Fatal(err)
	}
	// el worker "arregla" sin tocar el examen, pero el runner no ejecuta nada
	commitEn(t, e.Worktree, "total.txt", "170\n")
	e.Cmd = "printf '1..0\n' > out.tap"
	r.save(e)
	e.Huella, _ = Huella(e.Worktree, e.Bundle, e.Cmd)
	r.save(e)
	if _, _, err := r.Verificar("", t0); err == nil || !strings.Contains(err.Error(), "0 pruebas") {
		t.Fatalf("got %v", err)
	}
}

func TestOmitidosNoPermitidosFallan(t *testing.T) {
	r := repoConReporte(t, `if cmp -s total.txt tests/datos.txt; then printf 'ok 1 total\nok 2 iva # SKIP luego\n' > out.tap; else printf 'not ok 1 total\n' > out.tap; exit 1; fi
`)
	e, _, err := abrirConReporte(r, Opciones{})
	if err != nil {
		t.Fatal(err)
	}
	commitEn(t, e.Worktree, "total.txt", "170\n")
	if _, _, err = r.Verificar("", t0); err == nil || !strings.Contains(err.Error(), "omitid") {
		t.Fatalf("got %v", err)
	}
}

func TestRojoSinAsercionNoAbre(t *testing.T) {
	r := repoConReporte(t, "exit 2\n") // "error de compilación": falla sin reporte
	if _, _, err := abrirConReporte(r, Opciones{}); err == nil || !strings.Contains(err.Error(), "aserción") {
		t.Fatalf("got %v", err)
	}
}

func TestReciboTraeContadores(t *testing.T) {
	r := repoConReporte(t, tapReal)
	e, _, err := abrirConReporte(r, Opciones{})
	if err != nil {
		t.Fatal(err)
	}
	commitEn(t, e.Worktree, "total.txt", "170\n")
	e, _, err = r.Verificar("", t0)
	if err != nil || e.Recibo.Ejecutados != 1 || e.Recibo.Fallidos != 0 || e.Recibo.Artefacto == "" {
		t.Fatalf("%+v %v", e.Recibo, err)
	}
}

func TestRestaurarDentroDeLaCorridaNoEnganaEnAislado(t *testing.T) {
	// el examen lee un archivo ignorado por git que el worker puede plantar
	r := repoConReporte(t, `cat local.txt 2>/dev/null | grep -q 170 || cmp -s total.txt tests/datos.txt
`)
	os.WriteFile(filepath.Join(r.Root, ".gitignore"), []byte("local.txt\nout.tap\n"), 0o644)
	Git(r.Root, "add", "-A")
	Git(r.Root, "commit", "-qm", "ignore")
	e, _, err := r.Abrir("01", "t", "tests/s01.sh", examen, Opciones{Aislado: true}, t0)
	if err != nil {
		t.Fatal(err)
	}
	commitEn(t, e.Worktree, "otro.txt", "x\n")
	os.WriteFile(filepath.Join(e.Worktree, "local.txt"), []byte("170\n"), 0o644)
	if _, _, err := r.Verificar("", t0); err == nil {
		t.Fatal("un archivo ignorado del worker hizo pasar el examen aislado")
	}
}

func TestTimeoutCortaElExamen(t *testing.T) {
	r := repoDeJuguete(t)
	e, _, err := r.Abrir("01", "t", "tests/s01.sh", examen, Opciones{Timeout: 1}, t0)
	if err != nil {
		t.Fatal(err)
	}
	e.Cmd = "sleep 30"
	e.Huella, _ = Huella(e.Worktree, e.Bundle, e.Cmd)
	r.save(e)
	ini := time.Now()
	r.Verificar("", t0)
	if time.Since(ini) > 10*time.Second {
		t.Fatal("no cortó por tiempo")
	}
}

func TestSinProgresoPorEspaciosYTopeDeIntentos(t *testing.T) {
	r := repoDeJuguete(t)
	e := abrir(t, r)
	r.Verificar("", t0)
	commitEn(t, e.Worktree, "total.txt", "0 \n") // solo espacios
	if _, _, err := r.Verificar("", t0); err == nil || !strings.Contains(err.Error(), "sin progreso") {
		t.Fatalf("got %v", err)
	}
	for i := 2; i <= MaxIntentos; i++ {
		if _, err := r.Reintentar("", "cambio", false); err != nil {
			t.Fatal(err)
		}
		e, _ = r.Load()
		r.bloquear(e, "x")
	}
	if _, err := r.Reintentar("", "otra vez", false); err == nil || !strings.Contains(err.Error(), "Owner") {
		t.Fatalf("got %v", err)
	}
	if _, err := r.Reintentar("", "otra vez", true); err != nil {
		t.Fatal(err)
	}
}

func TestDosReparacionesAntesDeBloquear(t *testing.T) {
	r := repoDeJuguete(t)
	e := abrir(t, r)
	for i, v := range []string{"1\n", "2\n"} {
		commitEn(t, e.Worktree, "total.txt", v)
		if _, _, err := r.Verificar("", t0); err == nil || strings.Contains(err.Error(), "BLOQUEADO") {
			t.Fatalf("fallo %d bloqueó antes de tiempo: %v", i+1, err)
		}
	}
	commitEn(t, e.Worktree, "total.txt", "3\n")
	if _, _, err := r.Verificar("", t0); err == nil || !strings.Contains(err.Error(), "BLOQUEADO") {
		t.Fatalf("got %v", err)
	}
}

func TestCierreNoQuitaWorktreeConProcesoVivo(t *testing.T) {
	r := repoDeJuguete(t)
	e := abrir(t, r)
	commitEn(t, e.Worktree, "total.txt", "170\n")
	r.Verificar("", t0)
	orig := EnUso
	EnUso = func(d string) []string {
		if d == e.Worktree {
			return []string{"claude (pid 7)"}
		}
		return nil
	}
	_, _, _, err := r.Cerrar("", true, "", t0)
	EnUso = orig
	if !errors.Is(err, ErrEnUso) || !existe(e.Worktree) {
		t.Fatalf("got %v", err)
	}
	if _, _, _, err := r.Cerrar("", true, "", t0); err != nil {
		t.Fatalf("al liberarse no terminó: %v", err)
	}
}

func TestMetricasDelCiclo(t *testing.T) {
	r := repoDeJuguete(t)
	if m := r.Metricas(time.Time{}); m.Aceptados != 0 || m.HorasMedias != 0 {
		t.Fatalf("%+v", m)
	}
	e := abrir(t, r)
	r.Verificar("", t0)
	r.Verificar("", t0) // mismo head: sin progreso
	r.Reintentar("", "otra cosa", false)
	commitEn(t, e.Worktree, "total.txt", "170\n")
	r.Verificar("", t0)
	r.Cerrar("", true, "", t0.Add(2*time.Hour))
	m := r.Metricas(time.Time{})
	if m.Aceptados != 1 || m.Intentos != 2 || m.Verificaciones != 3 || m.FallosSinProgreso != 1 || m.VerdesALaPrimera != 0 {
		t.Fatalf("%+v", m)
	}
}

// dosSlices: S-01 cambia total.txt y S-02 cambia iva.txt; no se tocan.
func dosSlices(t *testing.T) (Repo, *Estado, *Estado) {
	t.Helper()
	r := repoDeJuguete(t)
	r.MaxSlices = 2
	os.WriteFile(filepath.Join(r.Root, "tests", "s02.sh"), []byte("grep -qx 18 iva.txt\n"), 0o644)
	Git(r.Root, "add", "-A")
	Git(r.Root, "commit", "-qm", "rojo 2")
	a := abrir(t, r)
	b, _, err := r.Abrir("02", "iva", "tests/s02.sh", "sh tests/s02.sh", Opciones{}, t0)
	if err != nil {
		t.Fatal(err)
	}
	commitEn(t, a.Worktree, "total.txt", "170\n")
	commitEn(t, b.Worktree, "iva.txt", "18\n")
	return r, a, b
}

func TestDosSlicesIndependientesYTope(t *testing.T) {
	r, _, _ := dosSlices(t)
	if _, _, err := r.Abrir("03", "x", "tests/s01.sh", examen, Opciones{}, t0); err == nil || !strings.Contains(err.Error(), "tope") {
		t.Fatalf("got %v", err)
	}
	if _, err := r.Load(); !errors.Is(err, ErrVarios) {
		t.Fatalf("sin --slice con dos abiertos: %v", err)
	}
	if _, _, err := r.Verificar("01", t0); err != nil {
		t.Fatal(err)
	}
	if _, _, err := r.Verificar("02", t0); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := r.Cerrar("01", true, "", t0); err != nil {
		t.Fatal(err)
	}
	// S-02 se integra sobre la base que ya trae S-01
	if _, _, _, err := r.Cerrar("02", true, "", t0); err != nil {
		t.Fatal(err)
	}
	for f, v := range map[string]string{"total.txt": "170\n", "iva.txt": "18\n"} {
		if got, _ := os.ReadFile(filepath.Join(r.Root, f)); string(got) != v {
			t.Fatalf("%s = %q", f, got)
		}
	}
	if r.Siguiente() != "03" {
		t.Fatalf("siguiente %s", r.Siguiente())
	}
}

func TestDosCierresSimultaneosNoIntegranDosVeces(t *testing.T) {
	r, _, _ := dosSlices(t)
	r.Verificar("01", t0)
	r.Verificar("02", t0)
	errs := make(chan error, 3)
	for _, nn := range []string{"01", "02", "01"} {
		go func(nn string) { _, _, _, err := r.Cerrar(nn, true, "", t0); errs <- err }(nn)
	}
	ok := 0
	for i := 0; i < 3; i++ {
		if <-errs == nil {
			ok++
		}
	}
	log, _ := Git(r.Root, "log", "--oneline", "main")
	if strings.Count(log, "slice S-01/") != 1 || strings.Count(log, "slice S-02/") != 1 || ok < 2 {
		t.Fatalf("ok=%d\n%s", ok, log)
	}
}

func TestMigraSliceJsonV04(t *testing.T) {
	r := repoDeJuguete(t)
	raw := `{"nn":"07","intento":1,"fase":"IMPLEMENTANDO","test":"tests/s01.sh","cmd":"x","rama":"slice/07","worktree":"/nada"}`
	os.WriteFile(filepath.Join(r.OrqDir, "slice.json"), []byte(raw), 0o644)
	e, err := r.Load()
	if err != nil || e.NN != "07" || existe(filepath.Join(r.OrqDir, "slice.json")) || r.Siguiente() != "08" {
		t.Fatalf("%+v %v", e, err)
	}
}

func TestCerrarRepetidoNoDuplicaRegistroYBundleIgnoradoNoAbre(t *testing.T) {
	r := repoDeJuguete(t)
	e := abrir(t, r)
	commitEn(t, e.Worktree, "total.txt", "170\n")
	r.Verificar("", t0)
	if _, _, _, err := r.Cerrar("01", true, "", t0); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := r.Cerrar("01", true, "", t0); !errors.Is(err, ErrYaCerrado) {
		t.Fatalf("got %v", err)
	}
	if n := len(leerJSONL[Registro](r.logPath())); n != 1 {
		t.Fatalf("%d registros", n)
	}
	os.WriteFile(filepath.Join(r.Root, ".gitignore"), []byte("local.cfg\n"), 0o644)
	os.WriteFile(filepath.Join(r.Root, "local.cfg"), []byte("x"), 0o644)
	Git(r.Root, "add", ".gitignore")
	Git(r.Root, "commit", "-qm", "ign")
	if _, _, err := r.Abrir("02", "t", "tests/s01.sh", "false", Opciones{Bundle: []string{"local.cfg"}}, t0); err == nil || !strings.Contains(err.Error(), "versionado") {
		t.Fatalf("got %v", err)
	}
}

// El examen pasa cuando front y back están: total.txt = 170 y api.txt = ok.
// Un worker juntado se poda (worktree, rama y equipo); uno sin juntar no. La zona podada vuelve a nacer.
func TestPodarWorkerJuntado(t *testing.T) {
	r := repoDeJuguete(t)
	if _, _, err := r.Abrir("01", "carrito", "tests/s01.sh", examen, Opciones{Bundle: []string{"tests/datos.txt"}}, t0); err != nil {
		t.Fatal(err)
	}
	_, front, _ := r.Sumar("01", "w-front", "claude", "front", []string{"web"})
	_, back, _ := r.Sumar("01", "w-back", "codex", "back", []string{"api"})
	os.MkdirAll(filepath.Join(front.Worktree, "web"), 0o755)
	commitEn(t, front.Worktree, "web/total.txt", "170\n")
	if _, _, err := r.Juntar("01"); err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(filepath.Join(back.Worktree, "api"), 0o755)
	commitEn(t, back.Worktree, "api/api.txt", "ok\n") // sin juntar
	e, inf, err := r.Podar("01", []string{"w-front", "w-back"})
	if err != nil {
		t.Fatal(err)
	}
	if existe(front.Worktree) || len(e.Equipo) != 1 || e.Equipo[0].Nombre != "w-back" {
		t.Fatalf("%v %s", e.Equipo, strings.Join(inf, "\n"))
	}
	if b, _ := Git(r.Root, "branch", "--list", front.Rama); b != "" {
		t.Fatalf("la rama %s sigue", front.Rama)
	}
	if !existe(back.Worktree) {
		t.Fatal("back no estaba juntado: no se poda")
	}
	if _, w, err := r.Sumar("01", "w-front", "claude", "front", []string{"web"}); err != nil || !existe(w.Worktree) {
		t.Fatalf("la zona podada debe volver a nacer: %v", err)
	}
}

func TestEquipoFrontBackTestEnUnSlice(t *testing.T) {
	r := repoDeJuguete(t)
	commitEn(t, r.Root, "tests/s01.sh", "cmp -s web/total.txt tests/datos.txt && grep -qx ok api/api.txt\n")
	e, _, err := r.Abrir("01", "carrito", "tests/s01.sh", examen, Opciones{Bundle: []string{"tests/datos.txt"}}, t0)
	if err != nil {
		t.Fatal(err)
	}
	_, front, err := r.Sumar("01", "w-front", "claude", "front", []string{"web"})
	if err != nil {
		t.Fatal(err)
	}
	_, back, _ := r.Sumar("01", "w-back", "codex", "back", []string{"api"})
	_, test, _ := r.Sumar("01", "w-test", "claude", "test", []string{"tests/unit"})
	if _, _, err := r.Sumar("01", "w-x", "claude", "otro", []string{"web/css"}); err == nil {
		t.Fatal("dos workers con la misma ruta")
	}
	if _, _, err := r.Sumar("01", "w-x", "claude", "trampa", []string{"tests"}); err == nil {
		t.Fatal("una zona incluye el examen")
	}
	os.MkdirAll(filepath.Join(front.Worktree, "web"), 0o755)
	os.MkdirAll(filepath.Join(back.Worktree, "api"), 0o755)
	os.MkdirAll(filepath.Join(test.Worktree, "tests", "unit"), 0o755)
	commitEn(t, front.Worktree, "web/total.txt", "170\n")
	commitEn(t, back.Worktree, "api/api.txt", "ok\n")
	os.MkdirAll(filepath.Join(back.Worktree, "web"), 0o755)
	commitEn(t, back.Worktree, "web/hack.txt", "x\n") // back se sale de su zona
	commitEn(t, test.Worktree, "tests/unit/t.sh", "true\n")
	e, inf, err := r.Juntar("01")
	if err == nil || !strings.Contains(strings.Join(inf, "\n"), "w-back: tocó fuera de su zona") {
		t.Fatalf("%v\n%s", err, strings.Join(inf, "\n"))
	}
	// back lo corrige y se vuelve a juntar; los ya juntados no se repiten
	Git(back.Worktree, "rm", "-q", "web/hack.txt")
	Git(back.Worktree, "commit", "-qm", "deshace")
	// lo neto ya queda dentro de su zona: se junta
	if e, inf, err = r.Juntar("01"); err != nil {
		t.Fatalf("%v\n%s", err, strings.Join(inf, "\n"))
	}
	if _, _, err := r.Verificar("01", t0); err != nil {
		t.Fatal(err)
	}
	if _, q, _, err := r.Cerrar("01", true, "", t0); err != nil || len(q) != 0 {
		t.Fatalf("%v %v", q, err)
	}
	for _, w := range []*Worker{front, back, test} {
		if existe(w.Worktree) {
			t.Fatalf("sigue %s", w.Worktree)
		}
		if b, _ := Git(r.Root, "branch", "--list", w.Rama); b != "" {
			t.Fatalf("sigue la rama %s", w.Rama)
		}
	}
	for _, f := range []string{"web/total.txt", "api/api.txt", "tests/unit/t.sh"} {
		if !existe(filepath.Join(r.Root, f)) {
			t.Fatalf("falta %s en main", f)
		}
	}
	_ = e
}

// La zona sigue con su worker: la tarea siguiente del plan reusa su worktree y suma rutas.
func TestZonaSigueConSuWorker(t *testing.T) {
	r := repoDeJuguete(t)
	if _, _, err := r.Abrir("01", "carrito", "tests/s01.sh", examen, Opciones{Bundle: []string{"tests/datos.txt"}}, t0); err != nil {
		t.Fatal(err)
	}
	_, back, err := r.Sumar("01", "w-back", "claude", "back", []string{"api"})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := r.Sumar("01", "w-otro", "claude", "back", []string{"db"}); err == nil || !strings.Contains(err.Error(), "ya es de w-back") {
		t.Fatalf("otro nombre no hereda la zona: %v", err)
	}
	os.MkdirAll(filepath.Join(back.Worktree, "api"), 0o755)
	commitEn(t, back.Worktree, "api/api.txt", "ok\n")
	if _, inf, err := r.Juntar("01"); err != nil {
		t.Fatalf("%v\n%s", err, strings.Join(inf, "\n"))
	}
	_, otra, err := r.Sumar("01", "w-back", "claude", "back", []string{"api", "db"})
	if err != nil {
		t.Fatalf("la tarea siguiente de la zona: %v", err)
	}
	if otra.Worktree != back.Worktree || len(otra.Rutas) != 2 {
		t.Fatalf("debe reusar el worktree y sumar db: %+v", otra)
	}
	if _, _, err := r.Sumar("01", "w-front", "claude", "front", []string{"db/x"}); err == nil {
		t.Fatal("la ruta nueva de back ya es suya")
	}
	os.MkdirAll(filepath.Join(back.Worktree, "db"), 0o755)
	commitEn(t, back.Worktree, "db/m.txt", "1\n")
	if _, inf, err := r.Juntar("01"); err != nil {
		t.Fatalf("db ya es de su zona: %v\n%s", err, strings.Join(inf, "\n"))
	}
}

func TestAmpliarAlcance(t *testing.T) {
	r := repoDeJuguete(t)
	if _, _, err := r.Abrir("01", "total", "tests/s01.sh", examen, Opciones{Bundle: []string{"tests/datos.txt"}, Alcance: []string{"total.txt"}}, t0); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Ampliar("01", []string{"tests"}); err == nil {
		t.Fatal("el examen no entra al alcance")
	}
	e, err := r.Ampliar("01", []string{"db/m.txt", "total.txt"})
	if err != nil || len(e.Alcance) != 2 {
		t.Fatalf("%v %v", e.Alcance, err)
	}
}

// H-04b: una ampliación de alcance mientras corre el examen no se pierde, y el resultado se descarta.
func TestVerificarNoPisaAmpliacionConcurrente(t *testing.T) {
	r := repoDeJuguete(t)
	sync := t.TempDir()
	corre, sigue := filepath.Join(sync, "corre"), filepath.Join(sync, "sigue")
	os.WriteFile(sigue, nil, 0o644)
	cmd := "touch " + corre + "; while [ ! -f " + sigue + " ]; do sleep 0.02; done; " + examen
	e, _, err := r.Abrir("01", "total", "tests/s01.sh", cmd, Opciones{Bundle: []string{"tests/datos.txt"}, Alcance: []string{"total.txt"}}, t0)
	if err != nil {
		t.Fatal(err)
	}
	commitEn(t, e.Worktree, "total.txt", "170\n")
	os.Remove(sigue)
	os.Remove(corre)
	hecho := make(chan error, 1)
	go func() {
		for {
			if _, err := os.Stat(corre); err == nil {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		_, err := r.Ampliar("01", []string{"db/m.txt"}) // no debe esperar a la corrida
		os.WriteFile(sigue, nil, 0o644)
		hecho <- err
	}()
	_, _, err = r.Verificar("01", t0)
	if errA := <-hecho; errA != nil {
		t.Fatal(errA)
	}
	if err == nil || !strings.Contains(err.Error(), "contrato") {
		t.Fatalf("debió descartar el resultado: %v", err)
	}
	ahora, _ := r.Load("01")
	if len(ahora.Alcance) != 2 || ahora.Recibo != nil || ahora.Fase != Implementando {
		t.Fatalf("se pisó la ampliación: %+v", ahora)
	}
}

// Sin alcance el slice toca todo: ampliar no puede encogerlo a las rutas sumadas.
func TestAmpliarSinAlcanceNoEncoge(t *testing.T) {
	r := repoDeJuguete(t)
	if _, _, err := r.Abrir("01", "total", "tests/s01.sh", examen, Opciones{Bundle: []string{"tests/datos.txt"}}, t0); err != nil {
		t.Fatal(err)
	}
	e, err := r.Ampliar("01", []string{"src/bff/proxy.ts"})
	if err != nil || len(e.Alcance) != 0 {
		t.Fatalf("debe seguir libre: %v %v", e.Alcance, err)
	}
}

func TestAlinearDetectaDesvio(t *testing.T) {
	r := repoDeJuguete(t)
	e, _, err := r.Abrir("01", "total", "tests/s01.sh", examen, Opciones{Bundle: []string{"tests/datos.txt"}, Alcance: []string{"total.txt"}}, t0)
	if err != nil {
		t.Fatal(err)
	}
	commitEn(t, e.Worktree, "total.txt", "170\n")
	if _, a, _ := r.Alinear(""); len(a.Avisos) != 0 || len(a.Cambiados) != 1 {
		t.Fatalf("dentro del alcance no avisa: %+v", a)
	}
	os.MkdirAll(filepath.Join(e.Worktree, "infra"), 0o755)
	commitEn(t, e.Worktree, "infra/k8s.yaml", "x\n")
	_, a, _ := r.Alinear("")
	if len(a.Fuera) != 1 || a.Fuera[0] != "infra/k8s.yaml" || len(a.Avisos) != 1 {
		t.Fatalf("debe avisar del desvío: %+v", a)
	}
}

// Los worktrees nuevos traen node_modules y .venv del repo si git los ignora; si no, nada.
func TestPrepararDeps(t *testing.T) {
	r := repoDeJuguete(t)
	commitEn(t, r.Root, ".gitignore", "node_modules/\n.venv\n")
	os.MkdirAll(filepath.Join(r.Root, "node_modules", "vite"), 0o755)
	os.WriteFile(filepath.Join(r.Root, "node_modules", "vite", "index.js"), []byte("x"), 0o644)
	os.MkdirAll(filepath.Join(r.Root, ".venv", "bin"), 0o755)
	e, _, err := r.Abrir("01", "deps", "tests/s01.sh", examen, Opciones{Bundle: []string{"tests/datos.txt"}}, t0)
	if err != nil {
		t.Fatal(err)
	}
	if !existe(filepath.Join(e.Worktree, "node_modules", "vite", "index.js")) {
		t.Fatal("falta node_modules en el worktree del slice")
	}
	if fi, err := os.Lstat(filepath.Join(e.Worktree, ".venv")); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf(".venv debe ser un enlace: %v", err)
	}
	if st, _ := Git(e.Worktree, "status", "--porcelain"); st != "" {
		t.Fatalf("el worktree queda sucio: %s", st)
	}
	_, w, err := r.Sumar("01", "w-front", "claude", "front", []string{"web"})
	if err != nil || !existe(filepath.Join(w.Worktree, "node_modules", "vite", "index.js")) {
		t.Fatalf("falta node_modules en la zona: %v", err)
	}
}

func TestPrepararDepsNoEnsucia(t *testing.T) {
	r := repoDeJuguete(t)
	os.MkdirAll(filepath.Join(r.Root, ".venv"), 0o755) // .venv sin ignorar
	e, _, err := r.Abrir("01", "deps", "tests/s01.sh", examen, Opciones{Bundle: []string{"tests/datos.txt"}}, t0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(e.Worktree, ".venv")); err == nil {
		t.Fatal("un enlace que git no ignora ensucia el worktree")
	}
}

// Una reparación pedida tras verificar reabre el mismo intento con su trabajo; el recibo deja de valer.
func TestReabrirVerificado(t *testing.T) {
	r := repoDeJuguete(t)
	e, _, err := r.Abrir("01", "total", "tests/s01.sh", examen, Opciones{Bundle: []string{"tests/datos.txt"}}, t0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Reabrir("01", "x"); err == nil {
		t.Fatal("en construcción no se reabre")
	}
	commitEn(t, e.Worktree, "total.txt", "170\n")
	if _, _, err := r.Verificar("01", t0); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Reabrir("01", " "); err == nil {
		t.Fatal("sin motivo no se reabre")
	}
	head, _ := Git(e.Worktree, "rev-parse", "HEAD")
	e2, err := r.Reabrir("01", "quitar el token de la lectura (Atena)")
	if err != nil || e2.Fase != Implementando || e2.Recibo != nil || e2.Intento != e.Intento {
		t.Fatalf("%+v %v", e2, err)
	}
	if h, _ := Git(e2.Worktree, "rev-parse", "HEAD"); h != head {
		t.Fatal("perdió el trabajo")
	}
}

// Una ruta aprobada después del lanzamiento entra a la zona del worker y juntar ya no la rechaza.
func TestAmpliarZona(t *testing.T) {
	r := repoDeJuguete(t)
	if _, _, err := r.Abrir("01", "z", "tests/s01.sh", examen, Opciones{Bundle: []string{"tests/datos.txt"}}, t0); err != nil {
		t.Fatal(err)
	}
	_, back, _ := r.Sumar("01", "w-back", "claude", "back", []string{"api"})
	r.Sumar("01", "w-front", "claude", "front", []string{"web"})
	os.MkdirAll(filepath.Join(back.Worktree, "db"), 0o755)
	commitEn(t, back.Worktree, "db/lista.txt", "x\n")
	if _, _, err := r.Juntar("01"); err == nil {
		t.Fatal("db aún no es de back")
	}
	if _, err := r.AmpliarZona("01", "back", []string{"web/y"}); err == nil {
		t.Fatal("no pisa la zona front")
	}
	if _, err := r.AmpliarZona("01", "back", []string{"tests"}); err == nil {
		t.Fatal("el examen no entra")
	}
	if _, err := r.AmpliarZona("01", "back", []string{"db/lista.txt"}); err != nil {
		t.Fatal(err)
	}
	if _, inf, err := r.Juntar("01"); err != nil {
		t.Fatalf("%v\n%s", err, strings.Join(inf, "\n"))
	}
}

// Una huella mal tomada se repara solo si el examen es idéntico al del rojo; un examen cambiado no.
func TestRehuella(t *testing.T) {
	r := repoDeJuguete(t)
	e, _, err := r.Abrir("01", "total", "tests/s01.sh", examen, Opciones{Bundle: []string{"tests/datos.txt"}}, t0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Rehuella("01", "x"); err == nil {
		t.Fatal("en construcción no se repara")
	}
	commitEn(t, e.Worktree, "total.txt", "170\n")
	e2, _ := r.Load("01")
	e2.Huella = "mal"
	r.save(e2)
	if _, _, err := r.Verificar("01", t0); err == nil {
		t.Fatal("con huella mala debe bloquear")
	}
	if _, err := r.Rehuella("01", "edición sin commit al abrir (Atena)"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := r.Verificar("01", t0); err != nil {
		t.Fatal(err)
	}
}

// Si el slice cambió el examen, rehuella no lo bendice.
func TestRehuellaNoBendiceTrampa(t *testing.T) {
	r := repoDeJuguete(t)
	e, _, err := r.Abrir("01", "total", "tests/s01.sh", examen, Opciones{Bundle: []string{"tests/datos.txt"}}, t0)
	if err != nil {
		t.Fatal(err)
	}
	commitEn(t, e.Worktree, "tests/datos.txt", "1\n")
	commitEn(t, e.Worktree, "total.txt", "1\n")
	r.Verificar("01", t0)
	if _, err := r.Rehuella("01", "x"); err == nil {
		t.Fatal("un examen cambiado no se repara")
	}
}

// H-01: un examen cambiado con aprobación invalida los recibos previos; hay que volver a verificar.
func TestSumarExamenInvalidaRecibos(t *testing.T) {
	r := repoDeJuguete(t)
	e := abrir(t, r)
	commitEn(t, e.Worktree, "tests/extra.sh", "true\n")
	head, _ := Git(e.Worktree, "rev-parse", "HEAD")
	e.Recibo = &Recibo{Attempt: e.Attempt(), Head: strings.TrimSpace(head)}
	if err := r.save(e); err != nil {
		t.Fatal(err)
	}
	e, err := r.SumarExamen("01", []string{"tests/extra.sh"}, "sh tests/extra.sh", "m1")
	if err != nil {
		t.Fatal(err)
	}
	if e.Recibo != nil {
		t.Fatal("el recibo viejo sobrevivió al cambio de examen")
	}
	if e, _ = r.Load("01"); e.Recibo != nil {
		t.Fatal("el recibo viejo quedó guardado")
	}
	if _, _, _, err := r.Cerrar("", true, "", t0); err == nil {
		t.Fatal("cerró sin volver a verificar")
	}
}

// H-03: un solo evaluador. La base agrega config que omite una prueba; tras integrar, 1 corrida y
// 1 omitida no es verde aunque el worker haya verificado 2/0.
func TestIntegracionRechazaOmitidosQueTraeLaBase(t *testing.T) {
	r := repoConReporte(t, `if cmp -s total.txt tests/datos.txt; then printf 'ok 1 total\n' > out.tap; if [ -f skip.cfg ]; then printf 'ok 2 iva # SKIP cfg\n' >> out.tap; else printf 'ok 2 iva\n' >> out.tap; fi; else printf 'not ok 1 total\n' > out.tap; exit 1; fi
`)
	e, _, err := abrirConReporte(r, Opciones{})
	if err != nil {
		t.Fatal(err)
	}
	commitEn(t, e.Worktree, "total.txt", "170\n")
	if e, _, err = r.Verificar("01", t0); err != nil || e.Recibo.Ejecutados != 2 || e.Recibo.Omitidos != 0 {
		t.Fatalf("%+v %v", e.Recibo, err)
	}
	commitEn(t, r.Root, "skip.cfg", "iva\n")
	if _, _, _, err := r.Cerrar("01", true, "", t0); err == nil || !strings.Contains(err.Error(), "omitid") {
		t.Fatalf("integró con una prueba omitida por la base: %v", err)
	}
}

func TestEvaluarUnico(t *testing.T) {
	e := &Estado{}
	casos := []struct {
		nombre string
		e      *Estado
		ok     bool
		cnt    *Conteo
		huella bool
		paso   bool
	}{
		{"verde contado", e, true, &Conteo{Ejecutados: 2}, true, true},
		{"exit malo", e, false, &Conteo{Ejecutados: 2}, true, false},
		{"cero pruebas", e, true, &Conteo{}, true, false},
		{"fallidas", e, true, &Conteo{Ejecutados: 2, Fallidos: 1}, true, false},
		{"omitidas", e, true, &Conteo{Ejecutados: 1, Omitidos: 1}, true, false},
		{"omitidas permitidas", &Estado{Omitir: true}, true, &Conteo{Ejecutados: 1, Omitidos: 1}, true, true},
		{"huella rota", e, true, &Conteo{Ejecutados: 2}, false, false},
		{"sin conteo, laxo", e, true, nil, true, true},
		{"sin conteo, estricto", &Estado{Estricto: true}, true, nil, true, false},
	}
	for _, c := range casos {
		if paso, _ := Evaluar(c.e, c.ok, c.cnt, c.huella); paso != c.paso {
			t.Errorf("%s: paso=%v", c.nombre, paso)
		}
	}
}

func TestEstrictoExigeReporte(t *testing.T) {
	r := repoDeJuguete(t)
	if _, _, err := r.Abrir("01", "total", "tests/s01.sh", examen, Opciones{Estricto: true}, t0); err == nil || !strings.Contains(err.Error(), "--reporte") {
		t.Fatalf("estricto sin reporte abrió: %v", err)
	}
}

// El SHA integrado tiene su propio recibo; cerrar exige ese recibo para el SHA que se entrega.
func TestReciboDeIntegracion(t *testing.T) {
	r := repoConReporte(t, tapReal)
	e, _, err := abrirConReporte(r, Opciones{})
	if err != nil {
		t.Fatal(err)
	}
	commitEn(t, e.Worktree, "total.txt", "170\n")
	if _, _, err := r.Verificar("01", t0); err != nil {
		t.Fatal(err)
	}
	reg, _, _, err := r.Cerrar("01", true, "", t0)
	if err != nil {
		t.Fatal(err)
	}
	var integ *Recibo
	for _, rc := range leerJSONL[Recibo](r.recibosPath()) {
		if rc.Tipo == "integracion" {
			integ = &rc
		}
	}
	if integ == nil || integ.Head != reg.SHA || integ.Ejecutados != 1 || integ.Resultado != "paso" || integ.TS.IsZero() {
		t.Fatalf("sin recibo de integración para %s: %+v", reg.SHA, integ)
	}
}

func TestCerrarSinReciboDeIntegracionNoPublica(t *testing.T) {
	r := repoDeJuguete(t)
	e := abrir(t, r)
	commitEn(t, e.Worktree, "total.txt", "170\n")
	e, _, _ = r.Verificar("01", t0)
	e.Fase, e.Integrado = Cerrando, e.Recibo.Head // "integrado" sin recibo propio
	r.save(e)
	if _, _, _, err := r.Cerrar("01", true, "", t0); err == nil || !strings.Contains(err.Error(), "integración") {
		t.Fatalf("publicó sin recibo de integración: %v", err)
	}
}

// Todo cambio de contrato invalida los recibos (del worker y de integración).
func TestCambiosDeContratoInvalidanRecibos(t *testing.T) {
	falso := func(e *Estado) {
		e.Recibo = &Recibo{Attempt: e.Attempt(), Head: "x"}
		e.ReciboInt = &Recibo{Attempt: e.Attempt(), Head: "y", Tipo: "integracion"}
		e.Integrado = "y"
	}
	ops := map[string]struct {
		fase string
		op   func(Repo) (*Estado, error)
	}{
		"ampliar":    {Verificado, func(r Repo) (*Estado, error) { return r.Ampliar("01", []string{"api"}) }},
		"rehuella":   {Bloqueado, func(r Repo) (*Estado, error) { return r.Rehuella("01", "m1 huella") }},
		"reintentar": {Bloqueado, func(r Repo) (*Estado, error) { return r.Reintentar("01", "otro enfoque", false) }},
		"reabrir":    {Verificado, func(r Repo) (*Estado, error) { return r.Reabrir("01", "reparar") }},
	}
	for nombre, c := range ops {
		r := repoDeJuguete(t)
		e, _, err := r.Abrir("01", "total", "tests/s01.sh", examen, Opciones{Bundle: []string{"tests/datos.txt"}, Alcance: []string{"total.txt"}}, t0)
		if err != nil {
			t.Fatal(err)
		}
		falso(e)
		e.Fase = c.fase
		r.save(e)
		if _, err := c.op(r); err != nil {
			t.Errorf("%s: %v", nombre, err)
			continue
		}
		e, _ = r.Load("01")
		if e.Recibo != nil || e.ReciboInt != nil || e.Integrado != "" || e.Fase == Verificado {
			t.Errorf("%s dejó recibos vivos: fase %s recibo %v int %v %q", nombre, e.Fase, e.Recibo, e.ReciboInt, e.Integrado)
		}
	}
}

// Exploit 4: rechazar no borra un ignorado único sin respaldo verificado.
func TestRechazarRespaldaIgnoradoUnico(t *testing.T) {
	r := repoDeJuguete(t)
	e := abrir(t, r)
	os.WriteFile(filepath.Join(e.Worktree, ".gitignore"), []byte("out.tap\n"), 0o644)
	Git(e.Worktree, "add", ".gitignore")
	Git(e.Worktree, "commit", "-qm", "ignora")
	os.WriteFile(filepath.Join(e.Worktree, "out.tap"), []byte("ok 1 único\n"), 0o644)
	if _, _, _, err := r.Cerrar("", false, "no", t0); err != nil {
		t.Fatal(err)
	}
	if existe(filepath.Join(e.Worktree, "out.tap")) {
		return // conservado o en cuarentena: no se perdió
	}
	bs, _ := filepath.Glob(filepath.Join(r.OrqDir, "backups", "*.tgz"))
	if len(bs) == 0 {
		t.Fatal("borró out.tap sin respaldo")
	}
}
