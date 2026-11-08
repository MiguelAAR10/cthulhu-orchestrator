package identidad

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type tablaInfo struct {
	tablaFalsa
	info map[int]Info
}

func (t tablaInfo) Info(pid int) (Info, error) {
	in, ok := t.info[pid]
	if !ok {
		return Info{}, os.ErrNotExist
	}
	return in, nil
}

const pts3 = 136<<8 | 3
const hvc0 = 229 << 8

func TestConfiableFallaCerrado(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if _, err := Asegurar(); err != nil {
		t.Fatal(err)
	}
	// huérfano: 90 (orq) <- 80 (python setsid) <- 70 (subreaper sin tty) <- 2 (init WSL, consola hvc) <- 1
	tb := tablaInfo{tablaFalsa{ppid: map[int]int{90: 80, 80: 70, 70: 2, 2: 1}, env: map[int]map[string]string{}},
		map[int]Info{90: {PPID: 80, Start: 9}, 80: {PPID: 70, Start: 8}, 70: {PPID: 2, Start: 7}, 2: {PPID: 1, Start: 1, TTY: hvc0}}}
	if err := Confiable(tb, 90); err == nil || !strings.Contains(err.Error(), "session register") {
		t.Fatalf("un huérfano sin ancla ni terminal pasó: %v", err)
	}
	// terminal de persona en la cadena: vale
	tb.info[70] = Info{PPID: 2, Start: 7, TTY: pts3}
	if err := Confiable(tb, 90); err != nil {
		t.Fatal(err)
	}
	tb.info[70] = Info{PPID: 2, Start: 7}
	// ancla registrada con el arranque correcto: vale; con un pid reciclado (otro arranque), no.
	if _, err := Anclar(tb, 70, "owner"); err != nil {
		t.Fatal(err)
	}
	if err := Confiable(tb, 90); err != nil {
		t.Fatal(err)
	}
	tb.info[70] = Info{PPID: 2, Start: 700}
	if err := Confiable(tb, 90); err == nil {
		t.Fatal("pid reciclado aceptado como ancla")
	}
	// un ancestro worker anula todo, aunque haya terminal
	tb.info[70] = Info{PPID: 2, Start: 7, TTY: pts3}
	tb.env[80] = map[string]string{"ORQ_WORKER_ID": "w1"}
	if err := Confiable(tb, 90); err == nil {
		t.Fatal("descendiente de worker aceptado")
	}
}

func TestProcFSInfoReal(t *testing.T) {
	if _, err := os.Stat("/proc/self/stat"); err != nil {
		t.Skip("sin /proc")
	}
	in, err := ProcFS{}.Info(os.Getpid())
	if err != nil || in.PPID != os.Getppid() || in.Start == 0 {
		t.Fatalf("%+v %v", in, err)
	}
}

// Un ancla plantada a mano (sin HMAC del token, o con uno falso) no vale: el huérfano sigue rechazado.
func TestAnclaPlantadaSinHMAC(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if _, err := Asegurar(); err != nil {
		t.Fatal(err)
	}
	tb := tablaInfo{tablaFalsa{ppid: map[int]int{90: 70, 70: 1}, env: map[int]map[string]string{}},
		map[int]Info{90: {PPID: 70, Start: 9}, 70: {PPID: 1, Start: 7}}}
	os.MkdirAll(AnclasDir(), 0o700)
	for i, body := range []string{
		`{"pid":70,"start":7,"role":"owner"}`,
		`{"pid":70,"start":7,"role":"owner","mac":"00ff"}`,
	} {
		os.WriteFile(filepath.Join(AnclasDir(), fmt.Sprintf("70-7-%d.json", i)), []byte(body), 0o600)
		if err := Confiable(tb, 90); err == nil {
			t.Fatalf("ancla plantada aceptada: %s", body)
		}
	}
	if len(Anclas()) != 0 {
		t.Fatalf("Anclas devolvió anclas sin firma válida: %v", Anclas())
	}
	// el ancla firmada por Anclar sí vale; cambiarle el rol rompe la firma
	a, err := Anclar(tb, 70, "owner")
	if err != nil || Confiable(tb, 90) != nil {
		t.Fatal("ancla firmada rechazada", err)
	}
	if verificaMAC(Ancla{PID: a.PID, Start: a.Start, Rol: "atena", MAC: a.MAC}) {
		t.Fatal("la firma no cubre el rol")
	}
}

func TestAnclarSinTokenFalla(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	tb := tablaInfo{tablaFalsa{}, map[int]Info{70: {PPID: 1, Start: 7}}}
	if _, err := Anclar(tb, 70, "owner"); err == nil {
		t.Fatal("ancla sin token del Owner")
	}
}
