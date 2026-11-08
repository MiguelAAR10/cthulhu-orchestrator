package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"orq/internal/escriba"
	"orq/internal/evidencia"
	"orq/internal/qa"
	"orq/internal/slice"
)

// qaRunner corre la pirámide y los repro; se reemplaza en pruebas.
var qaRunner qa.Runner = qa.ShellRunner

// pulirDocs pule el README con el modelo barato del rol docs; nunca es requisito.
var pulirDocs = escriba.Ask

// dirSlice: el worktree del slice abierto, o la raíz del proyecto.
func (p proyecto) dirSlice(nn string) (string, *slice.Estado) {
	if e, err := p.slices().Load(nn); err == nil && e != nil {
		if _, err := os.Stat(e.Worktree); err == nil {
			return e.Worktree, e
		}
		return p.root, e
	}
	return p.root, nil
}

func runQA(args []string, stdout, stderr io.Writer) int {
	uso := "uso: orq qa run S-NN | finding add S-NN --requirement R --commit SHA --repro CMD --observed O --expected E --impact I | ver S-NN"
	p, err := abrirProyecto()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	switch {
	case len(args) >= 2 && args[0] == "run":
		if _, w := workerEfectivo(); w || os.Getenv("ORQ_ROL") == "tester" || os.Getenv("ORQ_ROL") == "repairer" {
			fmt.Fprintln(stderr, "orq qa: `qa run` lo pide solo el Orquestador")
			return 1
		}
		nn := nnDe(args[1])
		cfg := qa.LoadConfig(p.dir)
		if len(cfg.Layers) == 0 {
			fmt.Fprintln(stderr, `orq qa: sin capas. Declara la pirámide en .orq/config.json: {"qa":{"layers":[{"name":"unit","cmd":"..."},{"name":"e2e","cmd":"npx playwright test"},{"name":"mutation","cmd":"...","min_criticality":"high"}]}}`)
			return 1
		}
		dir, _ := p.dirSlice(nn)
		rec := qa.RunVigilado(dir, nn, cfg.Layers, cfg.Crit(), qaRunner, time.Now(), cfg.Permitidos())
		if err := qa.Guardar(p.dir, rec); err != nil {
			fmt.Fprintln(stderr, "orq qa:", err)
			return 1
		}
		for _, l := range rec.Layers {
			if l.Skipped {
				fmt.Fprintf(stdout, "  %-12s omitida (%s)\n", l.Name, l.Reason)
				continue
			}
			fmt.Fprintf(stdout, "  %-12s exit %d · %d pasan · %d fallan\n", l.Name, l.Exit, l.Passed, l.Failed)
		}
		if rec.Result == "invalid" {
			fmt.Fprintf(stderr, "orq qa: the run changed the worktree outside the allowed reports (%s); the receipt is invalid:\n", strings.Join(cfg.Permitidos(), ", "))
			for _, c := range rec.Tree.Changed {
				fmt.Fprintln(stderr, "  "+c)
			}
		}
		fmt.Fprintf(stdout, "QA %s %s (criticidad %s) → .orq/qa/%s/receipt.json\n", rec.Slice, rec.Result, rec.Criticality, rec.Slice)
		if rec.Result != "pass" {
			return 1
		}
		return 0
	case len(args) >= 3 && args[0] == "finding" && args[1] == "add":
		nn := nnDe(args[2])
		fs := flag.NewFlagSet("qa finding add", flag.ContinueOnError)
		fs.SetOutput(stderr)
		var f qa.Finding
		fs.StringVar(&f.Requirement, "requirement", "", "requisito o línea de la historia")
		fs.StringVar(&f.Commit, "commit", "", "commit probado")
		fs.StringVar(&f.Repro, "repro", "", "comando que reproduce el fallo (debe fallar)")
		fs.StringVar(&f.Observed, "observed", "", "lo observado")
		fs.StringVar(&f.Expected, "expected", "", "lo esperado")
		fs.StringVar(&f.Impact, "impact", "", "impacto para el usuario")
		if fs.Parse(args[3:]) != nil {
			return 1
		}
		f.Dir, _ = p.dirSlice(nn)
		f, err := qa.AddFinding(p.dir, nn, f, qaRunner, time.Now())
		if err != nil {
			fmt.Fprintln(stderr, "orq qa:", err)
			return 1
		}
		if f.Confirmed {
			fmt.Fprintf(stdout, "hallazgo CONFIRMADO (el repro falla con exit %d): bloquea el cierre hasta un QA verde nuevo\n", f.ReproExit)
		} else {
			fmt.Fprintln(stdout, "hallazgo NO confirmado: el repro pasa al re-ejecutarlo; queda registrado y no cuenta")
		}
		return 0
	case len(args) >= 2 && args[0] == "ver":
		nn := nnDe(args[1])
		if rec, err := qa.Recibo(p.dir, nn); err == nil {
			raw, _ := json.MarshalIndent(rec, "", "  ")
			fmt.Fprintln(stdout, string(raw))
		} else {
			fmt.Fprintln(stdout, "sin recibo QA")
		}
		fs := qa.Findings(p.dir, nn)
		fmt.Fprintf(stdout, "hallazgos: %d registrados, %d confirmados\n", len(fs), qa.Confirmados(fs))
		return 0
	}
	fmt.Fprintln(stderr, uso)
	return 1
}

func runEvidence(args []string, stdout, stderr io.Writer) int {
	uso := "uso: orq evidence add S-NN --kind target|before|after --file PNG [--note \"...\"] [--pulir] | readme S-NN [--pulir]"
	p, err := abrirProyecto()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if len(args) < 2 || (args[0] != "add" && args[0] != "readme") {
		fmt.Fprintln(stderr, uso)
		return 1
	}
	nn := nnDe(args[1])
	fs := flag.NewFlagSet("evidence", flag.ContinueOnError)
	fs.SetOutput(stderr)
	kind := fs.String("kind", "", "target|before|after")
	file := fs.String("file", "", "PNG")
	note := fs.String("note", "", "nota en lenguaje de negocio")
	pulir := fs.Bool("pulir", false, "pule el README con el modelo de docs (opcional, con tiempo máximo)")
	if fs.Parse(args[2:]) != nil {
		return 1
	}
	ev := evidencia.Load(p.root, nn)
	if args[0] == "add" {
		if *kind == "" || *file == "" {
			fmt.Fprintln(stderr, uso)
			return 1
		}
		if ev, err = evidencia.Add(p.root, nn, *kind, *file, *note); err != nil {
			fmt.Fprintln(stderr, "orq evidence:", err)
			return 1
		}
	}
	md := evidencia.Readme(ev, p.infoSlice(nn, ev))
	if *pulir {
		if out, ok := evidencia.Pulir(md, pulirDocs); ok {
			md = out
		} else {
			fmt.Fprintln(stderr, "orq evidence: el pulido no llegó o no cumple; queda el README determinista")
		}
	}
	if err := evidencia.Escribir(p.root, nn, md); err != nil {
		fmt.Fprintln(stderr, "orq evidence:", err)
		return 1
	}
	if ev.DiffPct != nil {
		fmt.Fprintf(stdout, "diferencia after vs target: %.2f%%\n", *ev.DiffPct)
	}
	fmt.Fprintf(stdout, "README: docs/slices/%s/README.md\n", ev.Slice)
	return 0
}

// infoSlice junta título, resultado y recibos para el README.
func (p proyecto) infoSlice(nn string, ev evidencia.Evidencia) evidencia.Info {
	in := evidencia.Info{}
	if _, e := p.dirSlice(nn); e != nil {
		in.Titulo = e.Titulo
		if e.Recibo != nil {
			in.Recibos = append(in.Recibos, fmt.Sprintf("verification %s at %.12s: %s (%d run, %d failed)", e.Recibo.Attempt, e.Recibo.Head, e.Recibo.Resultado, e.Recibo.Ejecutados, e.Recibo.Fallidos))
		}
	}
	for _, k := range []string{"after", "target", "before"} {
		if it, ok := ev.Items[k]; ok && it.Note != "" {
			in.Resultado = it.Note
			break
		}
	}
	if rec, err := qa.Recibo(p.dir, nn); err == nil {
		pas, fal := 0, 0
		for _, l := range rec.Layers {
			pas, fal = pas+l.Passed, fal+l.Failed
		}
		in.Aceptacion = fmt.Sprintf("QA %s (%d passed, %d failed, criticality %s)", rec.Result, pas, fal, rec.Criticality)
		in.Recibos = append(in.Recibos, fmt.Sprintf("QA receipt .orq/qa/%s/receipt.json at %.12s", rec.Slice, rec.Head))
	}
	fs := qa.Findings(p.dir, nn)
	if len(fs) > 0 {
		in.Recibos = append(in.Recibos, fmt.Sprintf("findings: %d filed, %d confirmed", len(fs), qa.Confirmados(fs)))
	}
	return in
}
