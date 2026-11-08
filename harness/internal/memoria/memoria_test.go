package memoria

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var t0 = time.Date(2026, 10, 26, 10, 0, 0, 0, time.UTC)

func TestAddValidaEtiquetasYTexto(t *testing.T) {
	p := filepath.Join(t.TempDir(), "m.jsonl")
	if _, _, err := Add(p, Item{Imp: "urgente", Cat: "requisito", Fuente: "owner", Texto: "x"}, t0); err == nil {
		t.Fatal("aceptó una importancia fuera de la lista")
	}
	if _, _, err := Add(p, Item{Imp: "alta", Cat: "requisito", Fuente: "owner", Texto: strings.Repeat("a", 201)}, t0); err == nil {
		t.Fatal("aceptó un texto largo")
	}
	it, n, err := Add(p, Item{Imp: "critica", Cat: "requisito", Fuente: "owner", Texto: "el carrito guarda la talla"}, t0)
	if err != nil || n != 1 || it.ID != "m1" {
		t.Fatalf("got %v %d %v", it, n, err)
	}
	it, n, _ = Add(p, Item{Imp: "baja", Cat: "tecnico", Fuente: "scout", Texto: "usa pytest"}, t0.Add(time.Hour))
	if n != 2 || it.ID != "m2" {
		t.Fatalf("got %v %d", it, n)
	}
}

func TestRenderPoneLoCriticoPrimero(t *testing.T) {
	items := []Item{
		{ID: "m1", Imp: "baja", Cat: "tecnico", Fuente: "scout", Texto: "b", TS: t0},
		{ID: "m2", Imp: "critica", Cat: "requisito", Fuente: "owner", Texto: "c", TS: t0},
	}
	out := Render(items, "")
	if strings.Index(out, "m2") > strings.Index(out, "m1") {
		t.Fatalf("crítico no va primero:\n%s", out)
	}
}

func items(n int, imp string) []Item {
	var out []Item
	for i := 0; i < n; i++ {
		out = append(out, Item{ID: "x", Imp: imp, Cat: "tecnico", Fuente: "worker", Texto: "t", TS: t0})
	}
	return out
}

func TestCompactConservaLoCriticoYRespetaElTope(t *testing.T) {
	all := append(items(3, "critica"), items(10, "media")...)
	sum := func(rest []Item, max int) ([]Item, error) {
		if len(rest) != 10 || max != 5 {
			t.Fatalf("resumidor recibió %d con tope %d", len(rest), max)
		}
		return []Item{{Imp: "media", Cat: "aprendizaje", Texto: "resumen"}}, nil
	}
	out, err := Compact(all, 8, sum, t0)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 4 || out[3].Fuente != "escriba" {
		t.Fatalf("got %+v", out)
	}
}

func TestCompactRechazaSalidaMala(t *testing.T) {
	all := items(10, "media")
	casos := map[string]Summarizer{
		"se pasa del tope": func(r []Item, m int) ([]Item, error) { return items(m+1, "media"), nil },
		"inventa crítico":  func(r []Item, m int) ([]Item, error) { return items(1, "critica"), nil },
		"etiqueta rota":    func(r []Item, m int) ([]Item, error) { return []Item{{Imp: "media", Cat: "otra", Texto: "x"}}, nil },
	}
	for name, sum := range casos {
		if _, err := Compact(all, 5, sum, t0); err == nil {
			t.Errorf("%s: no rechazó", name)
		}
	}
}

func TestCompactSinEspacioPideDecisionDelOwner(t *testing.T) {
	if _, err := Compact(items(6, "critica"), 5, nil, t0); err == nil || !strings.Contains(err.Error(), "Owner") {
		t.Fatalf("got %v", err)
	}
}

func TestUnAgenteNoMarcaCriticoYReemplazarOcultaLoViejo(t *testing.T) {
	p := filepath.Join(t.TempDir(), "m.jsonl")
	if _, _, err := Add(p, Item{Imp: "critica", Cat: "requisito", Fuente: "worker", Texto: "x"}, t0); err == nil {
		t.Fatal("un worker marcó algo crítico")
	}
	a, _, _ := Add(p, Item{Imp: "alta", Cat: "decision", Fuente: "atena", Texto: "pago con Yape"}, t0)
	b, _, err := Add(p, Item{Imp: "alta", Cat: "decision", Fuente: "atena", Texto: "pago con Yape y Plin", Reemplaza: a.ID}, t0)
	if err != nil || b.Conf != "confirmado" {
		t.Fatalf("%v %v", b, err)
	}
	items, _ := Load(p)
	out := Render(items, "")
	if strings.Contains(out, "pago con Yape (") || !strings.Contains(out, "Yape y Plin") {
		t.Fatalf("%s", out)
	}
	if _, _, err := Add(p, Item{Imp: "baja", Cat: "tecnico", Fuente: "scout", Texto: "x", Reemplaza: "m99"}, t0); err == nil {
		t.Fatal("aceptó reemplazar un id inexistente")
	}
}

func TestTiposDeMemoria(t *testing.T) {
	p := filepath.Join(t.TempDir(), "m.jsonl")
	if _, _, err := Add(p, Item{Imp: "alta", Cat: "tecnico", Fuente: "scout", Tipo: "hecho", Texto: "usa pytest"}, t0); err == nil {
		t.Fatal("un hecho de scout sin evidencia")
	}
	if _, _, err := Add(p, Item{Imp: "alta", Cat: "tecnico", Fuente: "scout", Tipo: "hecho", Evidencia: "pyproject.toml:12", Texto: "usa pytest"}, t0); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Add(p, Item{Imp: "critica", Cat: "requisito", Fuente: "owner", Tipo: "hipotesis", Texto: "x"}, t0); err == nil {
		t.Fatal("hipótesis crítica")
	}
	viejo := Item{Cat: "tecnico", Conf: "inferido"}
	if viejo.TipoEfectivo() != "hipotesis" {
		t.Fatal("una línea inferida vieja debe leerse como hipótesis")
	}
	its := []Item{{ID: "m1", Vence: "S-01"}, {ID: "m2", Vence: "S-02"}, {ID: "m3"}}
	if v := SinVencer(its, map[string]bool{"S-01": true}); len(v) != 2 || v[0].ID != "m2" {
		t.Fatalf("%+v", v)
	}
}

func TestCompactNoAsciendeHipotesisYConservaConfirmado(t *testing.T) {
	all := append(items(5, "media"), Item{ID: "c", Imp: "alta", Cat: "decision", Fuente: "atena", Conf: "confirmado", Texto: "pago con Yape", TS: t0})
	sum := func(rest []Item, max int) ([]Item, error) {
		return []Item{{Imp: "alta", Cat: "tecnico", Tipo: "hecho", Fuente: "owner", Conf: "confirmado", Texto: "resumen"}}, nil
	}
	out, err := Compact(all, 3, sum, t0)
	if err != nil {
		t.Fatal(err)
	}
	var conf, res bool
	for _, it := range out {
		conf = conf || it.Texto == "pago con Yape"
		if it.Texto == "resumen" {
			res = it.Tipo == "hipotesis" && it.Conf == "inferido" && it.Fuente == "escriba"
		}
	}
	if !conf || !res {
		t.Fatalf("%+v", out)
	}
}

func TestAddConcurrenteNoPierdeLineasYNadieOcultaLoCritico(t *testing.T) {
	p := filepath.Join(t.TempDir(), "m.jsonl")
	crit, _, _ := Add(p, Item{Imp: "critica", Cat: "requisito", Fuente: "owner", Texto: "con IGV"}, t0)
	if _, _, err := Add(p, Item{Imp: "baja", Cat: "tecnico", Fuente: "worker", Texto: "sin IGV", Reemplaza: crit.ID}, t0); err == nil {
		t.Fatal("un worker ocultó un requisito del Owner")
	}
	done := make(chan bool)
	for i := 0; i < 20; i++ {
		go func() { Add(p, Item{Imp: "baja", Cat: "tecnico", Fuente: "scout", Texto: "x"}, t0); done <- true }()
	}
	for i := 0; i < 20; i++ {
		<-done
	}
	items, _ := Load(p)
	ids := map[string]bool{}
	for _, it := range items {
		ids[it.ID] = true
	}
	if len(items) != 21 || len(ids) != 21 {
		t.Fatalf("%d líneas, %d ids", len(items), len(ids))
	}
}

// El contexto respeta el presupuesto: lo crítico primero, luego el slice activo, luego el resto.
func TestPresupuestoPrimeroSliceActivo(t *testing.T) {
	var items []Item
	for i := 0; i < 40; i++ {
		items = append(items, Item{ID: fmt.Sprintf("o%d", i), Imp: "alta", Cat: "decision", Texto: "otro", Slice: "S-01", TS: t0.Add(time.Duration(i) * time.Minute)})
	}
	items = append(items, Item{ID: "act", Imp: "media", Cat: "tecnico", Texto: "activo", Slice: "S-02", TS: t0})
	items = append(items, Item{ID: "crit", Imp: "critica", Cat: "requisito", Texto: "regla", TS: t0})
	sel, resto := Presupuesto(items, "S-02", 5)
	if len(sel) != 5 || resto != 37 {
		t.Fatalf("tope roto: %d líneas, %d fuera", len(sel), resto)
	}
	if sel[0].ID != "crit" || sel[1].ID != "act" {
		t.Fatalf("orden: %v %v", sel[0].ID, sel[1].ID)
	}
	if s, r := Presupuesto(items[:3], "", 5); len(s) != 3 || r != 0 {
		t.Fatalf("%d %d", len(s), r)
	}
}

// mem add acepta los valores en inglés (y los españoles de siempre); se guardan en la forma canónica.
func TestValoresEnIngles(t *testing.T) {
	t.Setenv("ORQ_LANG", "")
	p := filepath.Join(t.TempDir(), "mem.jsonl")
	it, _, err := Add(p, Item{Imp: "high", Cat: "requirement", Fuente: "orchestrator", Tipo: "hypothesis", Texto: "x"}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if it.Imp != "alta" || it.Cat != "requisito" || it.Fuente != "orquestador" || it.Tipo != "hipotesis" {
		t.Fatalf("no normalizó: %+v", it)
	}
	if _, _, err := Add(p, Item{Imp: "media", Cat: "riesgo", Fuente: "worker", Texto: "y"}, time.Now()); err != nil {
		t.Fatal(err)
	}
	_, _, err = Add(p, Item{Imp: "huge", Cat: "risk", Fuente: "worker", Texto: "z"}, time.Now())
	if err == nil || !strings.Contains(err.Error(), "critical|high|medium|low") || strings.Contains(err.Error(), "usa") {
		t.Fatalf("mensaje: %v", err)
	}
	_, _, err = Add(p, Item{Imp: "low", Cat: "risk", Fuente: "worker", Tipo: "fact", Texto: "z"}, time.Now())
	if err == nil || !strings.Contains(err.Error(), "needs --evidence") {
		t.Fatalf("mensaje: %v", err)
	}
}
