package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"orq/internal/canal"
	"orq/internal/gitx"
	"orq/internal/identidad"
	"orq/internal/memoria"
	"orq/internal/orqdir"
	"orq/internal/slice"
)

// Fase deduce dónde está el trabajo, sin estado extra: canal, memoria y slice.
func (p Proyecto) Fase() (string, string) {
	ab, _ := p.Slices().Abiertos()
	if len(ab) > 1 {
		var xs []string
		for _, e := range ab {
			xs = append(xs, e.Attempt()+" "+e.Fase)
		}
		return "D·Desarrollo", strings.Join(xs, " · ") + " (usa --slice NN)"
	}
	if len(ab) == 1 {
		e := ab[0]
		switch e.Fase {
		case slice.Verificado:
			return "D2·Documentación", e.Attempt() + " verificado: demo con el Owner y cierre"
		case slice.Integrando, slice.Cerrando:
			return "D2·Documentación", e.Attempt() + " cierre a medias: repite `orq slice cerrar --aceptado`"
		case slice.Bloqueado:
			return "D1·Desarrollo", e.Attempt() + " BLOQUEADO (" + e.Motivo + "): decide reintentar o rechazar"
		}
		return "D1·Desarrollo", e.Attempt() + " en rojo: un escritor con TDD, scouts si falta contexto"
	}
	ms, _ := canal.Load(p.CanalPath())
	historia := false
	for _, m := range ms {
		historia = historia || m.Tipo == "historia"
	}
	if !historia {
		return "T·Traducción", "el Atena entiende con el Owner qué y para qué"
	}
	nn := "S-" + p.Slices().Siguiente()
	items, _ := memoria.Load(p.MemPath())
	for _, it := range items {
		if it.Slice == nn && it.Fuente == "scout" {
			return "A·Análisis", nn + ": plan corto y examen en rojo"
		}
	}
	return "C·Contexto", nn + ": scouts mapean el proyecto antes de decidir"
}

const lineasFicha = 60

// EscribirFicha regenera .orq/PROYECTO.md con el escriba; rechaza una ficha sin sus secciones.
func (p Proyecto) EscribirFicha() error {
	items, err := memoria.Load(p.MemPath())
	if err := memoria.Legible(err); err != nil && identidad.EsOrq2() { // orq keeps 1.0.0
		return fmt.Errorf("la memoria no se pudo leer: %w", err)
	}
	var keep []memoria.Item
	for _, it := range memoria.Vigentes(items) {
		if it.Cat != "riesgo" || it.Imp != "baja" {
			keep = append(keep, it)
		}
	}
	files, _ := gitx.Salida(fondo(), p.Root, "ls-files")
	fl := strings.Split(files, "\n")
	if len(fl) > 80 {
		fl = fl[:80]
	}
	slices, _ := os.ReadFile(filepath.Join(p.Dir, "slices.jsonl"))
	pf, _ := json.Marshal(p.Perfil())
	md, err := p.Escriba().Doc(string(pf), strings.Join(fl, ", "), memoria.Render(keep, ""), string(slices), lineasFicha)
	if err != nil {
		return err
	}
	head := fmt.Sprintf("# %s · ficha viva\n_Generada por orq el %s con un modelo barato. No editar a mano: corrige la memoria._\n\n", filepath.Base(p.Root), p.ahora().Format("2006-01-02 15:04"))
	return orqdir.WriteAtomic(p.FichaPath(), []byte(head+md))
}
