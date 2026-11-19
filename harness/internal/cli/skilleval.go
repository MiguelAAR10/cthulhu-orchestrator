package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"orq/internal/skilleval"
	"orq/internal/state"
)

// tipoDefecto es el stack del proyecto donde se corre, o "general" fuera de un repo.
func tipoDefecto() string {
	if p, err := abrirProyecto(); err == nil {
		return p.Perfil().Tipo()
	}
	return "general"
}

// runSkillEval puntúa una skill con claude plugin eval y guarda una fila por corrida.
func runSkillEval(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("skill eval", flag.ContinueOnError)
	fs.SetOutput(stderr)
	tipo := fs.String("tipo", "", "tipo de tarea (por defecto, el stack del proyecto)")
	maxUSD := fs.Float64("max-usd", 0, "tope de gasto en dólares de la corrida (obligatorio)")
	if len(args) < 1 || fs.Parse(args[1:]) != nil {
		fmt.Fprintln(stderr, "uso: orq skill eval DIR_DE_LA_SKILL --max-usd N [--tipo T]")
		return 1
	}
	if *maxUSD <= 0 {
		fmt.Fprintln(stderr, "orq skill eval: --max-usd es obligatorio: cada corrida gasta dinero real")
		return 1
	}
	if *tipo == "" {
		*tipo = tipoDefecto()
	}
	dir := args[0]
	hash, err := skilleval.Hash(dir)
	if err != nil {
		fmt.Fprintln(stderr, "orq skill eval:", err)
		return 1
	}
	tmp, err := os.MkdirTemp("", "orq-skill-eval-")
	if err != nil {
		fmt.Fprintln(stderr, "orq skill eval:", err)
		return 1
	}
	defer os.RemoveAll(tmp)
	plugin := filepath.Join(tmp, "plugin")
	nombre, err := skilleval.Envolver(dir, plugin)
	if err != nil {
		fmt.Fprintln(stderr, "orq skill eval:", err)
		return 1
	}
	r, err := skilleval.Correr(context.Background(), plugin, filepath.Join(tmp, "aggregate-result.json"), *maxUSD)
	if err != nil {
		fmt.Fprintln(stderr, "orq skill eval:", err)
		if errors.Is(err, skilleval.ErrNoDisponible) {
			return 3
		}
		return 1
	}
	n := skilleval.Nota{Skill: nombre, Hash: hash, Tipo: *tipo, Nota: r.Nota, Corridas: r.Corridas, CostoUSD: r.CostoUSD,
		Claude: r.Claude, Parcial: r.Parcial, Fecha: time.Now().UTC().Truncate(time.Second), Casos: r.Casos}
	if err := skilleval.Guardar(state.Dir(), n); err != nil {
		fmt.Fprintln(stderr, "orq skill eval:", err)
		return 1
	}
	fmt.Fprintf(stdout, "%s %s tipo %s: nota %.2f en %d corridas · USD %.2f · claude %s%s\n", nombre, corto(hash), n.Tipo, n.Nota, n.Corridas, n.CostoUSD, n.Claude, parcial(n))
	for _, c := range r.Casos {
		fmt.Fprintf(stdout, "  %s %.2f\n", c.Nombre, c.Nota)
	}
	return 0
}

func parcial(n skilleval.Nota) string {
	if n.Parcial {
		return " (parcial: llegó al tope de gasto)"
	}
	return ""
}

// runSkillNotas lista las notas por skill, el hash más nuevo primero.
func runSkillNotas(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("skill notas", flag.ContinueOnError)
	fs.SetOutput(stderr)
	tipo := fs.String("tipo", "", "solo ese tipo de tarea")
	if fs.Parse(args) != nil {
		return 1
	}
	ns, err := skilleval.Notas(state.Dir())
	if err != nil {
		fmt.Fprintln(stderr, "orq skill notas:", err)
		return 1
	}
	ls := skilleval.Listar(ns, *tipo)
	if len(ls) == 0 {
		fmt.Fprintln(stdout, "sin notas todavía: orq skill eval DIR --max-usd N")
		return 0
	}
	for _, n := range ls {
		fmt.Fprintf(stdout, "%s %s %s %.2f %d corridas USD %.2f claude %s %s%s\n", n.Skill, corto(n.Hash), n.Tipo, n.Nota, n.Corridas, n.CostoUSD, n.Claude, n.Fecha.Format(time.RFC3339), parcial(n))
	}
	return 0
}

func corto(h string) string {
	if len(h) > 12 {
		return h[:12]
	}
	return h
}
