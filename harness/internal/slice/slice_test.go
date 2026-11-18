package slice

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
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
	e, _, err := r.Abrir("01", "total", "tests/s01.sh", examen, Opciones{SinConteo: "juguete", Bundle: []string{"tests/datos.txt"}, Sano: "test -f total.txt"}, t0)
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
	if _, _, err := r.Abrir("01", "otro", "tests/s01.sh", examen, Opciones{SinConteo: "juguete"}, t0); err == nil {
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
	if _, _, err := r.Abrir("01", "t", "tests/s01.sh", examen, Opciones{SinConteo: "juguete", Sano: "false"}, t0); err == nil || !strings.Contains(err.Error(), "sana") {
		t.Fatalf("got %v", err)
	}
	commitEn(t, r.Root, "total.txt", "170\n")
	if _, _, err := r.Abrir("01", "t", "tests/s01.sh", examen, Opciones{SinConteo: "juguete"}, t0); err == nil || !strings.Contains(err.Error(), "rojo") {
		t.Fatalf("got %v", err)
	}
}

func TestDescubreAntesDeCrear(t *testing.T) {
	r := repoDeJuguete(t)
	Git(r.Root, "branch", "slice/01")
	if _, _, err := r.Abrir("01", "t", "tests/s01.sh", examen, Opciones{SinConteo: "juguete"}, t0); err == nil || !strings.Contains(err.Error(), "ya existe") {
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

// A green exit with 0 tests run is not green. The fixture goes through Abrir: the runner itself reports
// «1..0» once total.txt is right, so state and contract agree.
func TestVerdeSinPruebasEjecutadasFalla(t *testing.T) {
	r := repoConReporte(t, `if cmp -s total.txt tests/datos.txt; then printf '1..0\n' > out.tap; else printf 'not ok 1 total\n' > out.tap; exit 1; fi
`)
	e, _, err := abrirConReporte(r, Opciones{})
	if err != nil {
		t.Fatal(err)
	}
	// el worker "arregla" sin tocar el examen, pero el runner no ejecuta nada
	commitEn(t, e.Worktree, "total.txt", "170\n")
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
	e, _, err := r.Abrir("01", "t", "tests/s01.sh", examen, Opciones{SinConteo: "juguete", Aislado: true}, t0)
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
	e, _, err := r.Abrir("01", "t", "tests/s01.sh", examen, Opciones{SinConteo: "juguete", Timeout: 1}, t0)
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
	b, _, err := r.Abrir("02", "iva", "tests/s02.sh", "sh tests/s02.sh", Opciones{SinConteo: "juguete"}, t0)
	if err != nil {
		t.Fatal(err)
	}
	commitEn(t, a.Worktree, "total.txt", "170\n")
	commitEn(t, b.Worktree, "iva.txt", "18\n")
	return r, a, b
}

func TestDosSlicesIndependientesYTope(t *testing.T) {
	r, _, _ := dosSlices(t)
	if _, _, err := r.Abrir("03", "x", "tests/s01.sh", examen, Opciones{SinConteo: "juguete"}, t0); err == nil || !strings.Contains(err.Error(), "tope") {
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
	if _, _, err := r.Abrir("02", "t", "tests/s01.sh", "false", Opciones{SinConteo: "juguete", Bundle: []string{"local.cfg"}}, t0); err == nil || !strings.Contains(err.Error(), "versionado") {
		t.Fatalf("got %v", err)
	}
}

// El examen pasa cuando front y back están: total.txt = 170 y api.txt = ok.
// Un worker juntado se poda (worktree, rama y equipo); uno sin juntar no. La zona podada vuelve a nacer.
func TestPodarWorkerJuntado(t *testing.T) {
	r := repoDeJuguete(t)
	if _, _, err := r.Abrir("01", "carrito", "tests/s01.sh", examen, Opciones{SinConteo: "juguete", Bundle: []string{"tests/datos.txt"}}, t0); err != nil {
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
	e, _, err := r.Abrir("01", "carrito", "tests/s01.sh", examen, Opciones{SinConteo: "juguete", Bundle: []string{"tests/datos.txt"}}, t0)
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
	if _, _, err := r.Abrir("01", "carrito", "tests/s01.sh", examen, Opciones{SinConteo: "juguete", Bundle: []string{"tests/datos.txt"}}, t0); err != nil {
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
	if _, _, err := r.Abrir("01", "total", "tests/s01.sh", examen, Opciones{SinConteo: "juguete", Bundle: []string{"tests/datos.txt"}, Alcance: []string{"total.txt"}}, t0); err != nil {
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

// Sin alcance el slice toca todo: ampliar no puede encogerlo a las rutas sumadas.
func TestAmpliarSinAlcanceNoEncoge(t *testing.T) {
	r := repoDeJuguete(t)
	if _, _, err := r.Abrir("01", "total", "tests/s01.sh", examen, Opciones{SinConteo: "juguete", Bundle: []string{"tests/datos.txt"}}, t0); err != nil {
		t.Fatal(err)
	}
	e, err := r.Ampliar("01", []string{"src/bff/proxy.ts"})
	if err != nil || len(e.Alcance) != 0 {
		t.Fatalf("debe seguir libre: %v %v", e.Alcance, err)
	}
}

func TestAlinearDetectaDesvio(t *testing.T) {
	r := repoDeJuguete(t)
	e, _, err := r.Abrir("01", "total", "tests/s01.sh", examen, Opciones{SinConteo: "juguete", Bundle: []string{"tests/datos.txt"}, Alcance: []string{"total.txt"}}, t0)
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

// Decision 5 (step 1): node_modules and .venv of the main checkout reach every clone as a read-only
// link, when git ignores them. Not built: until B6 the worker runs outside the box with the owner's
// uid, so any link into the main checkout is writable by the worker, and P5 forbids per-worker copies.
// See docs/harness-v2/step1-report.md, decision 5.
func TestPrepararDeps(t *testing.T) {
	t.Skip("blocked until B6 (worker inside the box): see docs/harness-v2/step1-report.md, decision 5")
	r := repoDeJuguete(t)
	commitEn(t, r.Root, ".gitignore", "node_modules/\n.venv\n")
	os.MkdirAll(filepath.Join(r.Root, "node_modules", "vite"), 0o755)
	os.WriteFile(filepath.Join(r.Root, "node_modules", "vite", "index.js"), []byte("x"), 0o644)
	os.MkdirAll(filepath.Join(r.Root, ".venv", "bin"), 0o755)
	e, _, err := r.Abrir("01", "deps", "tests/s01.sh", examen, Opciones{SinConteo: "juguete", Bundle: []string{"tests/datos.txt"}}, t0)
	if err != nil {
		t.Fatal(err)
	}
	soloLectura := func(dir string) {
		t.Helper()
		if !existe(filepath.Join(dir, "node_modules", "vite", "index.js")) {
			t.Fatalf("node_modules missing in %s", dir)
		}
		if fi, err := os.Lstat(filepath.Join(dir, ".venv")); err != nil || fi.Mode()&os.ModeSymlink == 0 {
			t.Fatalf(".venv must be a link in %s: %v", dir, err)
		}
		if err := os.WriteFile(filepath.Join(dir, "node_modules", "vite", "index.js"), []byte("planted"), 0o644); err == nil {
			t.Fatalf("writing through node_modules in %s must fail", dir)
		}
		if err := os.WriteFile(filepath.Join(dir, ".venv", "bin", "python"), []byte("planted"), 0o755); err == nil {
			t.Fatalf("writing through .venv in %s must fail", dir)
		}
	}
	soloLectura(e.Worktree)
	if st, _ := Git(e.Worktree, "status", "--porcelain"); st != "" {
		t.Fatalf("el worktree queda sucio: %s", st)
	}
	_, w, err := r.Sumar("01", "w-front", "claude", "front", []string{"web"})
	if err != nil {
		t.Fatal(err)
	}
	soloLectura(w.Worktree)
	if got, _ := os.ReadFile(filepath.Join(r.Root, "node_modules", "vite", "index.js")); string(got) != "x" {
		t.Fatal("a worker changed the main checkout's node_modules")
	}
}

func TestPrepararDepsNoEnsucia(t *testing.T) {
	r := repoDeJuguete(t)
	os.MkdirAll(filepath.Join(r.Root, ".venv"), 0o755) // .venv sin ignorar
	e, _, err := r.Abrir("01", "deps", "tests/s01.sh", examen, Opciones{SinConteo: "juguete", Bundle: []string{"tests/datos.txt"}}, t0)
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
	e, _, err := r.Abrir("01", "total", "tests/s01.sh", examen, Opciones{SinConteo: "juguete", Bundle: []string{"tests/datos.txt"}}, t0)
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
	if _, _, err := r.Abrir("01", "z", "tests/s01.sh", examen, Opciones{SinConteo: "juguete", Bundle: []string{"tests/datos.txt"}}, t0); err != nil {
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
// Rehash only from BLOQUEADO. The fixture is a hash orq took wrong at open: state and contract both
// carry it, verify blocks on it, and rehuella takes it again from the red commit.
func TestRehuella(t *testing.T) {
	r := repoDeJuguete(t)
	e, _, err := r.Abrir("01", "total", "tests/s01.sh", examen, Opciones{SinConteo: "juguete", Bundle: []string{"tests/datos.txt"}}, t0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Rehuella("01", "x"); err == nil || !strings.Contains(err.Error(), "solo se repara desde "+Bloqueado) {
		t.Fatalf("from IMPLEMENTANDO it must refuse: %v", err)
	}
	commitEn(t, e.Worktree, "total.txt", "170\n")
	e.Huella = "mal"
	if err := errors.Join(r.sellar(e), r.save(e)); err != nil {
		t.Fatal(err)
	}
	if got, _, err := r.Verificar("01", t0); err == nil || got.Fase != Bloqueado {
		t.Fatalf("a wrong hash must block: %+v %v", got, err)
	}
	if _, err := r.Rehuella("01", "edición sin commit al abrir (Atena)"); err != nil {
		t.Fatal(err)
	}
	if got, _, err := r.Verificar("01", t0); err != nil || got.Fase != Verificado {
		t.Fatalf("%+v %v", got, err)
	}
}

// Si el slice cambió el examen, rehuella no lo bendice.
func TestRehuellaNoBendiceTrampa(t *testing.T) {
	r := repoDeJuguete(t)
	e, _, err := r.Abrir("01", "total", "tests/s01.sh", examen, Opciones{SinConteo: "juguete", Bundle: []string{"tests/datos.txt"}}, t0)
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

// ---------- verificación honesta y rutas ----------

func TestVeredictoUnico(t *testing.T) {
	casos := []struct {
		nombre string
		salio  bool
		c      *Conteo
		omitir bool
		ok     bool
		nota   string
	}{
		{"sin reporte y exit 0", true, nil, false, true, ""},
		{"sin reporte y exit 1", false, nil, false, false, ""},
		{"exit 1 con reporte verde", false, &Conteo{Ejecutados: 3}, false, false, ""},
		{"exit 0 y 3 pruebas", true, &Conteo{Ejecutados: 3}, false, true, ""},
		{"exit 0 y 0 pruebas", true, &Conteo{}, false, false, "0 pruebas"},
		{"omitidas sin permiso", true, &Conteo{Ejecutados: 2, Omitidos: 1}, false, false, "omitidas"},
		{"omitidas con permiso", true, &Conteo{Ejecutados: 2, Omitidos: 1}, true, true, ""},
		{"exit 0 pero fallidas", true, &Conteo{Ejecutados: 2, Fallidos: 1}, false, false, "fallidas"},
	}
	for _, c := range casos {
		ok, nota := veredicto(c.salio, c.c, c.omitir)
		if ok != c.ok || !strings.Contains(nota, c.nota) || (c.nota == "" && nota != "") {
			t.Errorf("%s: got %v %q, quería %v %q", c.nombre, ok, nota, c.ok, c.nota)
		}
	}
}

// integrar usa el mismo veredicto que Verificar: una prueba omitida en el merge no pasa.
func TestIntegrarRechazaOmitidosComoVerificar(t *testing.T) {
	r := repoConReporte(t, `if cmp -s total.txt tests/datos.txt; then printf 'ok 1 total\n' > out.tap; if test -f extra.txt; then printf 'ok 2 extra # SKIP luego\n' >> out.tap; fi; else printf 'not ok 1 total\n' > out.tap; exit 1; fi
`)
	e, _, err := abrirConReporte(r, Opciones{})
	if err != nil {
		t.Fatal(err)
	}
	commitEn(t, e.Worktree, "total.txt", "170\n")
	if _, _, err := r.Verificar("", t0); err != nil {
		t.Fatal(err) // en la rama del slice no hay extra.txt: nada omitido
	}
	commitEn(t, r.Root, "extra.txt", "x\n") // la base avanza y trae una prueba omitida al merge
	_, _, _, err = r.Cerrar("", true, "", t0)
	if err == nil || !strings.Contains(err.Error(), "omitidas") {
		t.Fatalf("integrar aceptó un merge con pruebas omitidas: %v", err)
	}
	if st, _ := r.Load(""); st == nil || st.Fase == Cerrando {
		t.Fatalf("el cierre no debía avanzar: %+v", st)
	}
}

func TestAbrirExigeConteoOMotivo(t *testing.T) {
	r := repoConReporte(t, tapReal)
	if _, _, err := r.Abrir("01", "t", "tests/s01.sh", examen, Opciones{}, t0); err == nil || !strings.Contains(err.Error(), "--sin-conteo") {
		t.Fatalf("abrió sin conteo ni motivo: %v", err)
	}
	if _, _, err := r.Abrir("01", "t", "tests/s01.sh", examen, Opciones{Reporte: "out.tap", Formato: "tap", SinConteo: "x"}, t0); err == nil {
		t.Fatal("--sin-conteo con --reporte debe rechazarse")
	}
	if _, _, err := r.Abrir("01", "t", "tests/s01.sh", examen, Opciones{Reporte: "out.tap"}, t0); err == nil {
		t.Fatal("--reporte sin --formato debe rechazarse")
	}
	if _, err := r.Load(""); err != nil {
		t.Fatal(err)
	}
}

func TestSinConteoQuedaEnElEstadoYEnCadaRecibo(t *testing.T) {
	r := repoDeJuguete(t)
	e, _, err := r.Abrir("01", "t", "tests/s01.sh", examen, Opciones{SinConteo: "el runner es un cmp"}, t0)
	if err != nil || e.SinConteo != "el runner es un cmp" {
		t.Fatalf("%+v %v", e, err)
	}
	if _, _, err := r.Verificar("", t0); err == nil { // sigue en rojo: recibo de fallo
		t.Fatal("el rojo no debe pasar")
	}
	commitEn(t, e.Worktree, "total.txt", "170\n")
	e, _, err = r.Verificar("", t0)
	if err != nil || !strings.Contains(e.Recibo.Nota, "el runner es un cmp") || e.Recibo.Contado {
		t.Fatalf("%+v %v", e.Recibo, err)
	}
	recibos := leerJSONL[Recibo](r.recibosPath())
	if len(recibos) != 2 {
		t.Fatalf("recibos: %d", len(recibos))
	}
	for _, rc := range recibos {
		if !strings.Contains(rc.Nota, "el runner es un cmp") {
			t.Fatalf("un recibo no repite el motivo: %+v", rc)
		}
	}
}

// Lo que hay en el worktree y git ignora no puede hacer pasar el examen: se corre sobre una copia de HEAD.
// There is no opt-out: --en-sitio was removed and opening with it is refused, opening nothing.
func TestVerificarSiempreAisladoYEnSitioRechazado(t *testing.T) {
	prep := func(t *testing.T, o Opciones) (Repo, *Estado, error) {
		r := repoConReporte(t, `cmp -s total.txt tests/datos.txt || test -f ruido.txt
`)
		os.WriteFile(filepath.Join(r.Root, ".git", "info", "exclude"), []byte(".orq/\nruido.txt\n"), 0o644)
		o.SinConteo = "juguete"
		e, _, err := r.Abrir("01", "t", "tests/s01.sh", examen, o, t0)
		if err != nil {
			return r, e, err
		}
		commitEn(t, e.Worktree, "otro.txt", "x\n")
		os.WriteFile(filepath.Join(e.Worktree, "ruido.txt"), []byte("x\n"), 0o644) // ignorado: el worktree sigue limpio
		return r, e, nil
	}
	r, _, err := prep(t, Opciones{})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := r.Verificar("", t0); err == nil {
		t.Fatal("un archivo ignorado del worker hizo pasar el examen por defecto")
	}
	r, e, err := prep(t, Opciones{EnSitio: "el examen lee un artefacto ignorado"})
	if err == nil || e != nil || !strings.Contains(err.Error(), "--en-sitio ya no existe") {
		t.Fatalf("--en-sitio must be refused with its reason: %+v %v", e, err)
	}
	if ab, _ := r.Abiertos(); len(ab) != 0 {
		t.Fatalf("a refused open must leave nothing open: %+v", ab)
	}
	if out, _ := Git(r.Root, "branch", "--list", "slice/01"); out != "" {
		t.Fatal("a refused open must not create the slice branch")
	}
	r, _, err = prep(t, Opciones{Aislado: true}) // --aislado sigue aceptándose y no cambia nada
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := r.Verificar("", t0); err == nil {
		t.Fatal("--aislado debe seguir siendo el modo por defecto")
	}
}

func TestAisladoRecibeLasDependenciasIgnoradas(t *testing.T) {
	r := repoConReporte(t, `cmp -s total.txt tests/datos.txt && test -f node_modules/p/ok && test -L .venv
`)
	commitEn(t, r.Root, ".gitignore", "node_modules/\n.venv\nout.tap\n")
	os.MkdirAll(filepath.Join(r.Root, "node_modules", "p"), 0o755)
	os.WriteFile(filepath.Join(r.Root, "node_modules", "p", "ok"), []byte("x"), 0o644)
	os.MkdirAll(filepath.Join(r.Root, ".venv", "bin"), 0o755)
	e, _, err := r.Abrir("01", "deps", "tests/s01.sh", examen, Opciones{SinConteo: "juguete"}, t0)
	if err != nil {
		t.Fatal(err)
	}
	commitEn(t, e.Worktree, "total.txt", "170\n")
	e, out, err := r.Verificar("", t0)
	if err != nil || !e.Recibo.Aislado {
		t.Fatalf("la copia aislada no trae las dependencias: %v\n%s", err, out)
	}
	if ents, _ := filepath.Glob(filepath.Join(os.TempDir(), "orq-exec-*")); len(ents) > 0 {
		for _, d := range ents {
			if existe(filepath.Join(d, "node_modules")) {
				t.Fatalf("la copia aislada quedó sin borrar: %s", d)
			}
		}
	}
}

func TestArchivosDelRunner(t *testing.T) {
	casos := map[string][]string{
		"npm test":                         {"package.json"},
		"CI=1 pnpm run test:e2e":           {"package.json"},
		"yarn jest":                        {"package.json", "jest.config.ts", "jest.config.js", "jest.config.mjs", "jest.config.cjs", "jest.config.mts", "jest.config.cts", "jest.config.json"},
		"bun test && echo listo":           {"package.json"},
		"make test":                        {"Makefile", "makefile", "GNUmakefile"},
		"make -f build/m.mk check":         {"Makefile", "makefile", "GNUmakefile", "build/m.mk"},
		"pytest -q":                        {"pyproject.toml", "pytest.ini", "setup.cfg", "tox.ini", "conftest.py"},
		"python -m pytest tests":           {"pyproject.toml", "pytest.ini", "setup.cfg", "tox.ini", "conftest.py"},
		"sh tests/run.sh":                  {"tests/run.sh"},
		"bash -e ci/examen.sh --rapido":    {"ci/examen.sh"},
		"npx playwright test":              {"package.json", "playwright.config.ts", "playwright.config.js", "playwright.config.mjs", "playwright.config.cjs", "playwright.config.mts", "playwright.config.cts"},
		"cd web && vitest run":             {"vitest.config.ts", "vitest.config.js", "vitest.config.mjs", "vitest.config.cjs", "vitest.config.mts", "vitest.config.cts", "vite.config.ts", "vite.config.js", "vite.config.mjs", "vite.config.cjs", "vite.config.mts", "vite.config.cts"},
		"go test ./...":                    nil,
		"cmp -s total.txt tests/datos.txt": nil,
	}
	for cmd, quiero := range casos {
		if got := archivosDelRunner(cmd); !slices.Equal(got, quiero) {
			t.Errorf("%q: got %v, quería %v", cmd, got, quiero)
		}
	}
}

// repoConRunner: el examen lo lanza run.sh, un archivo versionado que no es parte del paquete protegido.
func repoConRunner(t *testing.T) (Repo, *Estado) {
	t.Helper()
	r := repoDeJuguete(t)
	commitEn(t, r.Root, "run.sh", "sh tests/s01.sh\n")
	e, _, err := r.Abrir("01", "t", "tests/s01.sh", "sh run.sh", Opciones{SinConteo: "juguete", Bundle: []string{"tests/datos.txt"}}, t0)
	if err != nil {
		t.Fatal(err)
	}
	return r, e
}

func TestRunnerVigiladoSeGuardaAlAbrir(t *testing.T) {
	_, e := repoConRunner(t)
	if len(e.Vigilados) != 1 || e.Vigilados[0].Ruta != "run.sh" || len(e.Vigilados[0].Blob) < 40 {
		t.Fatalf("vigilados: %+v", e.Vigilados)
	}
	r := repoDeJuguete(t) // sin archivos de runner versionados: no se inventa nada
	e2 := abrir(t, r)
	if len(e2.Vigilados) != 0 {
		t.Fatalf("vigilados: %+v", e2.Vigilados)
	}
}

func TestRunnerCambiadoNoVerificaNiCuentaComoFallo(t *testing.T) {
	r, e := repoConRunner(t)
	commitEn(t, e.Worktree, "run.sh", "exit 0\n") // el comando ahora no corre el examen
	_, out, err := r.Verificar("", t0)
	var rc *ErrRunnerCambio
	if !errors.As(err, &rc) || len(rc.Archivos) != 1 || rc.Archivos[0] != "run.sh" || !strings.Contains(err.Error(), "--acepto-runner") {
		t.Fatalf("got %v", err)
	}
	if out != "" || len(leerJSONL[Recibo](r.recibosPath())) != 0 {
		t.Fatal("no debía correr nada ni dejar recibo")
	}
	ahora, _ := r.Load("")
	if ahora.Fallos != 0 || ahora.Fase != Implementando || ahora.HeadFallo != "" {
		t.Fatalf("contó como reparación fallida: %+v", ahora)
	}
	// Repetirlo tampoco bloquea por «sin progreso».
	for range 4 {
		if _, _, err := r.Verificar("", t0); !errors.As(err, &rc) {
			t.Fatalf("got %v", err)
		}
	}
	if ahora, _ = r.Load(""); ahora.Fase != Implementando {
		t.Fatalf("fase %s", ahora.Fase)
	}
	// Aceptado con motivo: corre y el recibo lo deja escrito.
	e, _, err = r.Verificar("", t0, VerificarOpc{AceptoRunner: "Atena aprobó cambiar el runner"})
	if err != nil || !strings.Contains(e.Recibo.Nota, "Atena aprobó cambiar el runner") || !strings.Contains(e.Recibo.Nota, "run.sh") {
		t.Fatalf("%+v %v", e.Recibo, err)
	}
}

func TestRunnerBorradoTambienSeDetecta(t *testing.T) {
	r, e := repoConRunner(t)
	Git(e.Worktree, "rm", "-q", "run.sh")
	Git(e.Worktree, "commit", "-qm", "borra el runner")
	if _, _, err := r.Verificar("", t0); err == nil || !strings.Contains(err.Error(), "run.sh (borrado)") {
		t.Fatalf("got %v", err)
	}
}

func TestRunnerIntactoVerificaSinFlag(t *testing.T) {
	r, e := repoConRunner(t)
	commitEn(t, e.Worktree, "total.txt", "170\n")
	if e, _, err := r.Verificar("", t0); err != nil || e.Fase != Verificado {
		t.Fatalf("%v", err)
	}
}

func TestRutaFixNoTieneEquipoYSubeAUnSlice(t *testing.T) {
	r := repoDeJuguete(t)
	if _, _, err := r.Abrir("01", "t", "tests/s01.sh", examen, Opciones{SinConteo: "juguete", Ruta: "otra"}, t0); err == nil {
		t.Fatal("ruta inválida")
	}
	e, _, err := r.Abrir("01", "t", "tests/s01.sh", examen, Opciones{SinConteo: "juguete", Ruta: "fix"}, t0)
	if err != nil || e.Ruta != RutaFix || e.RutaEfectiva() != RutaFix {
		t.Fatalf("%+v %v", e, err)
	}
	if _, _, err := r.Sumar("01", "w-front", "claude", "front", []string{"web"}); err == nil || !strings.Contains(err.Error(), "orq slice ruta slice") {
		t.Fatalf("Sumar en fix debe rechazarse y apuntar a la subida: %v", err)
	}
	if _, err := r.CambiarRuta("", "slice", "  "); err == nil {
		t.Fatal("subir sin motivo")
	}
	if _, err := r.CambiarRuta("", "fix", "x"); err == nil {
		t.Fatal("bajar a fix no existe")
	}
	e, err = r.CambiarRuta("", "slice", "toca schema y dos repos")
	if err != nil || e.Ruta != RutaSlice || !e.Ascendido || e.RutaMotivo != "toca schema y dos repos" {
		t.Fatalf("%+v %v", e, err)
	}
	if _, err := r.CambiarRuta("", "slice", "otra vez"); err == nil {
		t.Fatal("ya va por slice")
	}
	if _, w, err := r.Sumar("01", "w-front", "claude", "front", []string{"web"}); err != nil || w == nil {
		t.Fatalf("tras subir, el equipo debe poder sumarse: %v", err)
	}
}

func TestRutaPorDefectoEsSliceYQuedaEnElRegistro(t *testing.T) {
	r := repoDeJuguete(t)
	e := abrir(t, r)
	if e.Ruta != "" || e.RutaEfectiva() != RutaSlice {
		t.Fatalf("%+v", e)
	}
	if _, _, err := r.Sumar("01", "w-front", "claude", "front", []string{"web"}); err != nil {
		t.Fatalf("slice admite equipo: %v", err)
	}
	r.Cerrar("", false, "prueba", t0.Add(time.Hour))
	reg := leerJSONL[Registro](r.logPath())
	if len(reg) != 1 || reg[0].Ruta != RutaSlice || reg[0].Ascendido {
		t.Fatalf("%+v", reg)
	}
}

func TestMetricasPorRuta(t *testing.T) {
	r := repoDeJuguete(t)
	cerrar := func(nn, ruta string, ascender bool, horas int) {
		e, _, err := r.Abrir(nn, "t", "tests/s01.sh", examen, Opciones{SinConteo: "juguete", Ruta: ruta}, t0)
		if err != nil {
			t.Fatal(err)
		}
		if ascender {
			r.CambiarRuta(nn, "slice", "creció")
		}
		commitEn(t, e.Worktree, "total.txt", "170\n")
		if _, _, err := r.Verificar(nn, t0); err != nil {
			t.Fatal(err)
		}
		if _, _, _, err := r.Cerrar(nn, true, "", t0.Add(time.Duration(horas)*time.Hour)); err != nil {
			t.Fatal(err)
		}
		Git(r.Root, "checkout", "-q", "main")
		os.WriteFile(filepath.Join(r.Root, "total.txt"), []byte("0\n"), 0o644) // vuelve a rojo para el siguiente
		Git(r.Root, "commit", "-qam", "rojo otra vez")
	}
	cerrar("01", "fix", false, 1)
	cerrar("02", "fix", false, 3)
	cerrar("03", "fix", true, 5) // empezó como fix, terminó como slice
	cerrar("04", "", false, 8)
	m := r.Metricas(time.Time{})
	if m.Aceptados != 4 || m.Ascendidos != 1 {
		t.Fatalf("%+v", m)
	}
	f, s := m.PorRuta[RutaFix], m.PorRuta[RutaSlice]
	if f.Aceptados != 2 || f.VerdesALaPrimera != 2 || f.HorasMedias != 2 || s.Aceptados != 2 || s.HorasMedias != 6.5 {
		t.Fatalf("fix %+v slice %+v", f, s)
	}
}

// Un registro de orq 1.0.0 no trae ruta: cuenta como slice.
func TestMetricasRegistrosViejosSonSlice(t *testing.T) {
	r := repoDeJuguete(t)
	appendJSON(r.logPath(), map[string]any{"nn": "01", "intento": 1, "titulo": "viejo", "resultado": "aceptado",
		"abierto": t0, "cerrado": t0.Add(2 * time.Hour)})
	m := r.Metricas(time.Time{})
	if m.PorRuta[RutaSlice].Aceptados != 1 || m.PorRuta[RutaFix].Aceptados != 0 || m.PorRuta[RutaSlice].HorasMedias != 2 {
		t.Fatalf("%+v", m)
	}
}

// aLegado turns a slice opened by this version into what orq 1.0.0 left on disk: a linked worktree
// instead of a clone, a state without the new fields (schema 0) and no contract in git.
func aLegado(t *testing.T, r Repo, e *Estado) {
	t.Helper()
	if err := os.RemoveAll(e.Worktree); err != nil {
		t.Fatal(err)
	}
	if _, err := Git(r.Root, "worktree", "add", "-q", e.Worktree, e.Rama); err != nil {
		t.Fatal(err)
	}
	if _, err := Git(r.Root, "update-ref", "-d", refContrato(e.NN)); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(r.estadoPath(e.NN))
	var m map[string]any
	json.Unmarshal(raw, &m)
	for _, k := range []string{"sin_conteo", "en_sitio", "vigilados", "ruta", "ascendido", "ruta_motivo", "aislado", "esquema"} {
		delete(m, k)
	}
	m["procedencia"] = "git"
	raw, _ = json.Marshal(m)
	os.WriteFile(r.estadoPath(e.NN), raw, 0o644)
}

// Acceptance 6: a slice opened by orq 1.0.0 (schema 0, linked worktree, no contract in git) is
// adopted on verify — contract written from the state, provenance «legado» — and then closes.
func TestEstadoViejoSigueVerificandoYCerrando(t *testing.T) {
	r := repoDeJuguete(t)
	e := abrir(t, r)
	aLegado(t, r, e)
	commitEn(t, e.Worktree, "total.txt", "170\n")
	got, _, err := r.Verificar("", t0)
	if err != nil || got.Fase != Verificado || got.Recibo.Contado || got.Procedencia != "legado" {
		t.Fatalf("%+v %v", got, err)
	}
	var c Contrato
	if hay, err := r.leerBlob(refContrato("01"), &c); err != nil || !hay || c.Huella != got.Huella {
		t.Fatalf("adoption must write the contract to git: %v %v %+v", hay, err, c)
	}
	if reg, _, _, err := r.Cerrar("", true, "", t0.Add(time.Hour)); err != nil || reg.Result != "aceptado" || reg.Ruta != RutaSlice {
		t.Fatalf("%+v %v", reg, err)
	}
	if _, err := os.Stat(e.Worktree); !os.IsNotExist(err) {
		t.Fatal("the 1.0.0 linked worktree must be removed on close")
	}
}

// A legacy slice is verified like any other: on a clean copy of its commit in the box, never in the
// worktree. A file that only exists in the worktree does not make it pass.
func TestEstadoViejoVerificaEnCopiaLimpia(t *testing.T) {
	r := repoConReporte(t, `cmp -s total.txt tests/datos.txt || test -f ruido.txt
`)
	os.WriteFile(filepath.Join(r.Root, ".git", "info", "exclude"), []byte(".orq/\nruido.txt\n"), 0o644)
	e, _, err := r.Abrir("01", "t", "tests/s01.sh", examen, Opciones{SinConteo: "juguete"}, t0)
	if err != nil {
		t.Fatal(err)
	}
	aLegado(t, r, e)
	commitEn(t, e.Worktree, "otro.txt", "x\n")
	os.WriteFile(filepath.Join(e.Worktree, "ruido.txt"), []byte("x\n"), 0o644) // ignored: only in the worktree
	got, _, err := r.Verificar("", t0)
	if err == nil || got.Fase == Verificado || got.Procedencia != "legado" {
		t.Fatalf("a legacy slice must verify on a clean copy, not in the worktree: %+v %v", got, err)
	}
	if rec := leerJSONL[Recibo](r.recibosPath()); len(rec) != 1 || !rec[0].Aislado {
		t.Fatalf("receipt must say it ran on a clean copy: %+v", rec)
	}
}

// Adoption refuses while the old processes still hold the slice lock, and when the exam hash in the
// state does not match the exam at the red commit. `Convertir` does the same adoption explicitly.
func TestConvertirEstadoViejo(t *testing.T) {
	r := repoDeJuguete(t)
	e := abrir(t, r)
	aLegado(t, r, e)
	soltar, err := r.candado("slice-01")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := r.Verificar("", t0); err == nil || !strings.Contains(err.Error(), "stop") {
		t.Fatalf("adoption with the lock held must refuse and say to stop the processes: %v", err)
	}
	soltar()
	raw, _ := os.ReadFile(r.estadoPath("01"))
	bien := string(raw)
	os.WriteFile(r.estadoPath("01"), []byte(strings.Replace(bien, e.Huella, strings.Repeat("0", 64), 1)), 0o644)
	if _, err := r.Convertir("01"); err == nil || !strings.Contains(err.Error(), "hash") {
		t.Fatalf("a state whose exam hash does not match the red commit must not be adopted: %v", err)
	}
	if hay, _ := r.leerBlob(refContrato("01"), &Contrato{}); hay {
		t.Fatal("a refused adoption must not write a contract")
	}
	os.WriteFile(r.estadoPath("01"), []byte(bien), 0o644)
	got, err := r.Convertir("01")
	if err != nil || got.Procedencia != "legado" || r.confiable(got) != nil {
		t.Fatalf("%+v %v", got, err)
	}
	if _, err := r.Convertir("01"); err == nil {
		t.Fatal("converting a slice that already has a contract must say so")
	}
}
