package evidencia

import (
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// pintar crea un PNG w×h blanco con las primeras n columnas en negro.
func pintar(t *testing.T, dir, nombre string, w, h, n int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := color.RGBA{255, 255, 255, 255}
			if x < n {
				c = color.RGBA{0, 0, 0, 255}
			}
			img.Set(x, y, c)
		}
	}
	p := filepath.Join(dir, nombre)
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestDiffPixeles(t *testing.T) {
	d := t.TempDir()
	a := pintar(t, d, "a.png", 10, 10, 0)
	b := pintar(t, d, "b.png", 10, 10, 3)
	pct, err := DiffPct(a, a)
	if err != nil || pct != 0 {
		t.Fatalf("idénticas: %v %v", pct, err)
	}
	pct, err = DiffPct(a, b)
	if err != nil || pct != 30 {
		t.Fatalf("3 de 10 columnas: %v %v", pct, err)
	}
	c := pintar(t, d, "c.png", 20, 10, 0) // tamaño distinto: lo que no se solapa cuenta como diferente
	if pct, _ = DiffPct(a, c); pct != 50 {
		t.Fatalf("tamaño distinto: %v", pct)
	}
	if _, err := DiffPct(a, filepath.Join(d, "no.png")); err == nil {
		t.Fatal("archivo ausente es error")
	}
}

func TestAddYReadme(t *testing.T) {
	root := t.TempDir()
	src := t.TempDir()
	tg := pintar(t, src, "t.png", 10, 10, 0)
	af := pintar(t, src, "x.png", 10, 10, 1)
	if _, err := Add(root, "07", "lateral", tg, ""); err == nil {
		t.Fatal("kind inválido")
	}
	if _, err := Add(root, "07", "target", filepath.Join(src, "nada.txt"), ""); err == nil {
		t.Fatal("solo PNG")
	}
	if _, err := Add(root, "07", "target", tg, "diseño aprobado"); err != nil {
		t.Fatal(err)
	}
	ev, err := Add(root, "07", "after", af, "tras el cambio")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "docs/slices/S-07/after.png")); err != nil {
		t.Fatal("guarda after.png")
	}
	if ev.DiffPct == nil || *ev.DiffPct != 10 {
		t.Fatalf("diff after vs target: %+v", ev.DiffPct)
	}
	info := Info{Titulo: "Login con correo", Resultado: "Los clientes entran con su correo y ven su panel.", Aceptacion: "QA pass (12 passed, 0 failed)",
		Recibos: []string{".orq/qa/S-07/receipt.json"}}
	for i := 0; i < 60; i++ {
		info.Recibos = append(info.Recibos, "extra")
	}
	md := Readme(ev, info)
	if n := len(strings.Split(strings.TrimRight(md, "\n"), "\n")); n > MaxLineas {
		t.Fatalf("README de %d líneas", n)
	}
	for _, w := range []string{"# Login con correo", "10.00%", "target.png", "after.png", "QA pass"} {
		if !strings.Contains(md, w) {
			t.Fatalf("falta %q en:\n%s", w, md)
		}
	}
	if md != Readme(ev, info) {
		t.Fatal("determinista")
	}
	// pulido opcional: si falla o se pasa de largo, queda el determinista
	p, usado := Pulir(md, func(string) (string, error) { return "", errors.New("timeout") })
	if usado || p != md {
		t.Fatal("pulido fallido no cambia nada")
	}
	p, usado = Pulir(md, func(string) (string, error) { return strings.Repeat("x\n", 80), nil })
	if usado || p != md {
		t.Fatal("pulido demasiado largo se descarta")
	}
	p, usado = Pulir(md, func(string) (string, error) { return "# Login con correo\nok\n", nil })
	if !usado || !strings.Contains(p, "ok") {
		t.Fatal("pulido válido se usa")
	}
	if err := Escribir(root, "07", md); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "docs/slices/S-07/README.md")); err != nil {
		t.Fatal(err)
	}
}
