package cli

import (
	"flag"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"orq/internal/admision"
	"orq/internal/gate"
	"orq/internal/limits"
	"orq/internal/state"
	"orq/internal/sysinfo"
	"orq/internal/worktrees"
)

// contarWorkers cuenta agentes vivos en worktrees; se reemplaza en pruebas.
var contarWorkers = func(names []string) (int, error) {
	procs, err := sysinfo.ListProcs()
	return len(sysinfo.Workers(procs, names, gate.IsLinkedWorktree)), err
}

// orqDeCwd: la carpeta .orq del checkout principal del repo de cwd ("" fuera de git).
func orqDeCwd(cwd string) string {
	out, err := exec.Command("git", "-C", cwd, "rev-parse", "--path-format=absolute", "--git-common-dir").Output()
	if err != nil {
		return ""
	}
	return filepath.Join(filepath.Dir(strings.TrimSpace(string(out))), ".orq")
}

// worktreesDe cuenta los worktrees que orq registró en el proyecto de cwd. Fuera de un repo, 0.
func worktreesDe(cwd string) (int, error) {
	d := orqDeCwd(cwd)
	if d == "" {
		return 0, nil
	}
	return worktrees.Contar(d)
}

// admisionDe arma la misma entrada que usa el gate, para el proyecto p.
func (p proyecto) admision(l limits.Limits) admision.Env {
	return admision.Env{
		DiskFreeGB: sysinfo.FreeGB,
		Workers:    func() (int, error) { return contarWorkers(l.WorkerProcessName) },
		Worktrees:  func() (int, error) { return worktrees.Contar(p.dir) },
		Now:        time.Now,
		StateDir:   state.Dir(),
	}
}

// gc corre el retiro seguro en el proyecto, sin tocar los worktrees de slices abiertos.
func (p proyecto) gc(dry bool) ([]worktrees.Resultado, error) {
	ab, _ := p.slices().Abiertos()
	vivos := map[string]bool{}
	for _, e := range ab {
		vivos[filepath.Clean(e.Worktree)] = true
		vivos[filepath.Clean(p.slices().IntPathDe(e))] = true
		for _, w := range e.Equipo {
			vivos[filepath.Clean(w.Worktree)] = true
		}
	}
	return worktrees.GC(p.dir, worktrees.Opciones{DryRun: dry, Now: time.Now(), Protegido: func(d string) bool { return vivos[d] }})
}

func mostrarGC(w io.Writer, res []worktrees.Resultado) {
	n := map[string]int{}
	for _, r := range res {
		n[r.Accion]++
		linea := fmt.Sprintf("  %-10s %s", r.Accion, r.Path)
		if r.Motivo != "" {
			linea += " · " + r.Motivo
		}
		if r.Bundle != "" {
			linea += " · bundle " + r.Bundle
		}
		if r.Backup != "" {
			linea += " · respaldo " + r.Backup
		}
		fmt.Fprintln(w, linea)
	}
	fmt.Fprintf(w, "gc: %d retirado(s), %d conservado(s), %d en cuarentena, %d podado(s) del registro\n",
		n[worktrees.Retirado]+n[worktrees.Retiraria], n[worktrees.Conservado], n[worktrees.Cuarentena], n[worktrees.Podado])
}

func runGC(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("gc", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dry := fs.Bool("dry-run", false, "solo muestra qué haría")
	if fs.Parse(args) != nil {
		return 1
	}
	p, err := abrirProyecto()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	res, err := p.gc(*dry)
	mostrarGC(stdout, res)
	if err != nil {
		fmt.Fprintln(stderr, "orq gc:", err)
		return 1
	}
	return 0
}
