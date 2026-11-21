package slice

import (
	"strings"
	"testing"

	"orq/internal/diario"
)

func TestHistoriaConTresHijos(t *testing.T) {
	t.Parallel()
	r := repoDeJuguete(t)
	if _, err := r.AbrirHistoria("pagos", "Pagos", t0); err != nil {
		t.Fatal(err)
	}
	if _, err := r.AbrirHistoria("pagos", "otra", t0); err == nil {
		t.Fatal("a repeated story id must be refused")
	}
	if _, err := r.AbrirHistoria("Pagos con espacio", "x", t0); err == nil {
		t.Fatal("an id outside [a-z0-9-] must be refused")
	}
	o := Opciones{SinConteo: "juguete", Bundle: []string{"tests/datos.txt"}, Historia: "pagos"}
	for _, nn := range []string{"01", "02", "03"} {
		if _, _, err := r.Abrir(nn, "hijo "+nn, "tests/s01.sh", examen, o, t0); err != nil {
			t.Fatalf("abrir %s: %v", nn, err)
		}
	}
	if _, _, err := r.Abrir("04", "x", "tests/s01.sh", examen, Opciones{SinConteo: "j", Historia: "nada"}, t0); err == nil || !strings.Contains(err.Error(), "orq historia abrir") {
		t.Fatalf("unknown story: %v", err)
	}
	ab, cer, err := r.Hijos("pagos")
	if err != nil || len(ab) != 3 || len(cer) != 0 {
		t.Fatalf("hijos: %d abiertos %d cerrados %v", len(ab), len(cer), err)
	}
	for _, e := range ab {
		if e.Historia != "pagos" {
			t.Fatalf("%s historia %q", e.ID(), e.Historia)
		}
	}
	if _, _, _, err := r.Cerrar("03", false, "no hacía falta", t0); err != nil {
		t.Fatal(err)
	}
	ab, cer, err = r.Hijos("pagos")
	if err != nil || len(ab) != 2 || len(cer) != 1 || cer[0].NN != "03" || cer[0].Historia != "pagos" {
		t.Fatalf("after closing 03: %d abiertos %+v %v", len(ab), cer, err)
	}
	hs, err := r.Historias()
	if err != nil || len(hs) != 1 || hs[0].Titulo != "Pagos" {
		t.Fatalf("historias: %+v %v", hs, err)
	}
}

func TestPausaHistoriaYGlobal(t *testing.T) {
	t.Parallel()
	r := repoDeJuguete(t)
	r.AbrirHistoria("h", "H", t0)
	e := &Estado{NN: "02", Historia: "h"}
	suelto := &Estado{NN: "01"}
	if err := r.Pausado(e); err != nil {
		t.Fatal(err)
	}
	if err := r.PausarHistoria("h", true, t0); err != nil {
		t.Fatal(err)
	}
	if err := r.Pausado(e); err == nil || !strings.Contains(err.Error(), "orq historia reanudar h") {
		t.Fatalf("paused story: %v", err)
	}
	if err := r.Pausado(suelto); err != nil {
		t.Fatalf("a slice outside the story is not paused: %v", err)
	}
	r.PausarHistoria("h", false, t0)
	if err := r.PausarTodo(true, "almuerzo", t0); err != nil {
		t.Fatal(err)
	}
	if err := r.Pausado(suelto); err == nil || !strings.Contains(err.Error(), "orq reanudar") || !strings.Contains(err.Error(), "almuerzo") {
		t.Fatalf("global pause: %v", err)
	}
	if motivo, ok := r.PausaGlobal(); !ok || motivo != "almuerzo" {
		t.Fatalf("PausaGlobal: %q %v", motivo, ok)
	}
	r.PausarTodo(false, "", t0)
	if err := r.Pausado(e); err != nil {
		t.Fatal(err)
	}
	evs, _ := diario.Leer(diario.Ruta(r.OrqDir))
	var ps []string
	for _, ev := range evs {
		if ev.Tipo == "pausa" {
			ps = append(ps, ev.Resultado+":"+ev.Detalle)
		}
	}
	if strings.Join(ps, " ") != "pausada:h reanudada:h pausada:todo reanudada:todo" {
		t.Fatalf("journal: %v", ps)
	}
	if err := r.PausarHistoria("nada", true, t0); err == nil {
		t.Fatal("pausing an unknown story must fail")
	}
}

func TestIntegracionBaseAnota(t *testing.T) {
	t.Parallel()
	r := repoDeJuguete(t)
	e := abrir(t, r)
	commitEn(t, e.Worktree, "total.txt", "170\n")
	if _, _, err := r.Verificar("", t0); err != nil {
		t.Fatal(err)
	}
	reg, _, _, err := r.Cerrar("", true, "", t0)
	if err != nil {
		t.Fatal(err)
	}
	evs, _ := diario.Leer(diario.Ruta(r.OrqDir))
	var filas []diario.Evento
	for _, ev := range evs {
		if ev.Tipo == "integracion" {
			filas = append(filas, ev)
		}
	}
	if len(filas) != 1 || filas[0].Paso != "base" || filas[0].Resultado != "ok" || filas[0].Datos["sha"] != reg.SHA || reg.SHA == "" {
		t.Fatalf("one ledger row with the integrated SHA %s: %+v", reg.SHA, filas)
	}
}
