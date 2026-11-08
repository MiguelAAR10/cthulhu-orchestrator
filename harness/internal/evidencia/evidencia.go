// Package evidencia guarda la evidencia visual de un slice en docs/slices/S-NN/
// (target.png, before.png, after.png), mide la diferencia de píxeles entre after y target
// y genera un README corto y determinista en lenguaje de negocio.
package evidencia

import (
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// MaxLineas del README del slice.
const MaxLineas = 40

var Kinds = []string{"target", "before", "after"}

type Item struct {
	File string `json:"file"`
	Note string `json:"note,omitempty"`
}

// Evidencia es el índice de docs/slices/S-NN/evidence.json.
type Evidencia struct {
	Slice   string          `json:"slice"`
	Items   map[string]Item `json:"items"`
	DiffPct *float64        `json:"diff_pct,omitempty"` // after vs target
}

func id(nn string) string { return "S-" + strings.TrimPrefix(nn, "S-") }

// Dir es docs/slices/S-NN dentro del repo.
func Dir(root, nn string) string { return filepath.Join(root, "docs", "slices", id(nn)) }

func Load(root, nn string) Evidencia {
	ev := Evidencia{Slice: id(nn), Items: map[string]Item{}}
	if raw, err := os.ReadFile(filepath.Join(Dir(root, nn), "evidence.json")); err == nil {
		json.Unmarshal(raw, &ev)
		if ev.Items == nil {
			ev.Items = map[string]Item{}
		}
	}
	return ev
}

// Add copia el PNG como KIND.png, actualiza el índice y recalcula el diff si hay after y target.
func Add(root, nn, kind, src, note string) (Evidencia, error) {
	ev := Load(root, nn)
	if !slices.Contains(Kinds, kind) {
		return ev, fmt.Errorf("--kind debe ser target|before|after, no %q", kind)
	}
	if _, err := decodificar(src); err != nil {
		return ev, fmt.Errorf("%s no es un PNG legible: %v", src, err)
	}
	d := Dir(root, nn)
	if err := os.MkdirAll(d, 0o755); err != nil {
		return ev, err
	}
	dst := filepath.Join(d, kind+".png")
	if err := copiar(src, dst); err != nil {
		return ev, err
	}
	ev.Items[kind] = Item{File: kind + ".png", Note: note}
	ev.DiffPct = nil
	_, okA := ev.Items["after"]
	_, okT := ev.Items["target"]
	if okA && okT {
		pct, err := DiffPct(filepath.Join(d, "after.png"), filepath.Join(d, "target.png"))
		if err != nil {
			return ev, err
		}
		ev.DiffPct = &pct
	}
	raw, _ := json.MarshalIndent(ev, "", "  ")
	return ev, os.WriteFile(filepath.Join(d, "evidence.json"), append(raw, '\n'), 0o644)
}

func copiar(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func decodificar(p string) (image.Image, error) {
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return png.Decode(f)
}

// DiffPct: porcentaje de píxeles distintos sobre el lienzo que cubre ambas imágenes,
// redondeado a 2 decimales. Lo que solo existe en una cuenta como distinto.
func DiffPct(a, b string) (float64, error) {
	ia, err := decodificar(a)
	if err != nil {
		return 0, err
	}
	ib, err := decodificar(b)
	if err != nil {
		return 0, err
	}
	ra, rb := ia.Bounds(), ib.Bounds()
	w, h := max(ra.Dx(), rb.Dx()), max(ra.Dy(), rb.Dy())
	if w*h == 0 {
		return 0, errors.New("imagen vacía")
	}
	dif := 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			pa := image.Pt(ra.Min.X+x, ra.Min.Y+y)
			pb := image.Pt(rb.Min.X+x, rb.Min.Y+y)
			inA, inB := pa.In(ra), pb.In(rb)
			if !inA || !inB {
				dif++
				continue
			}
			r1, g1, b1, a1 := ia.At(pa.X, pa.Y).RGBA()
			r2, g2, b2, a2 := ib.At(pb.X, pb.Y).RGBA()
			if r1 != r2 || g1 != g2 || b1 != b2 || a1 != a2 {
				dif++
			}
		}
	}
	pct := float64(dif) * 100 / float64(w*h)
	return float64(int64(pct*100+0.5)) / 100, nil
}

// Info es lo que el README cuenta en lenguaje de negocio.
type Info struct {
	Titulo     string
	Resultado  string // un párrafo: qué puede hacer ahora el usuario
	Aceptacion string
	Recibos    []string
}

// Readme es determinista y nunca pasa de MaxLineas.
func Readme(ev Evidencia, in Info) string {
	t := in.Titulo
	if t == "" {
		t = ev.Slice
	}
	l := []string{"# " + t, "", "Slice " + ev.Slice + ".", "", "## Outcome", ""}
	res := strings.Join(strings.Fields(in.Resultado), " ")
	if res == "" {
		res = "No outcome recorded yet."
	}
	l = append(l, res, "", "## Acceptance", "")
	ac := in.Aceptacion
	if ac == "" {
		ac = "No QA receipt yet."
	}
	l = append(l, "- Result: "+ac)
	if ev.DiffPct != nil {
		l = append(l, fmt.Sprintf("- Visual difference after vs target: %.2f%%", *ev.DiffPct))
	} else {
		l = append(l, "- Visual difference: needs target.png and after.png")
	}
	l = append(l, "", "## Evidence", "")
	for _, k := range Kinds {
		if it, ok := ev.Items[k]; ok {
			s := fmt.Sprintf("- %s: [%s](%s)", k, it.File, it.File)
			if it.Note != "" {
				s += " — " + strings.Join(strings.Fields(it.Note), " ")
			}
			l = append(l, s)
		}
	}
	l = append(l, "", "## Receipts", "")
	libre := MaxLineas - len(l)
	rs := in.Recibos
	if len(rs) == 0 {
		rs = []string{"none"}
	}
	if len(rs) > libre {
		resto := len(rs) - (libre - 1)
		rs = append(rs[:libre-1:libre-1], fmt.Sprintf("… and %d more", resto))
	}
	for _, r := range rs {
		l = append(l, "- "+r)
	}
	return strings.Join(l, "\n") + "\n"
}

// Pulir pide al modelo de docs una versión más clara. Nunca es necesario:
// si falla, tarda o se pasa de MaxLineas, queda el texto determinista.
func Pulir(md string, ask func(string) (string, error)) (string, bool) {
	if ask == nil {
		return md, false
	}
	out, err := ask("Polish this slice README for a business reader. Keep every number, link and heading; max " +
		fmt.Sprint(MaxLineas) + " lines; reply with Markdown only.\n\n" + md)
	out = strings.TrimSpace(out)
	if err != nil || out == "" || !strings.HasPrefix(out, "# ") || len(strings.Split(out, "\n")) > MaxLineas {
		return md, false
	}
	return out + "\n", true
}

func Escribir(root, nn, md string) error {
	d := Dir(root, nn)
	if err := os.MkdirAll(d, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(d, "README.md"), []byte(md), 0o644)
}
