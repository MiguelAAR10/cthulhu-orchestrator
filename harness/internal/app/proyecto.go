// Package app son los casos de uso de orq: lo que un comando hace, sin leer banderas ni imprimir.
// Recibe sus puertos (terminal, reloj) en Proyecto; las pruebas le pasan dobles.
package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"orq/internal/canal"
	"orq/internal/config"
	"orq/internal/diario"
	"orq/internal/escriba"
	"orq/internal/identidad"
	"orq/internal/instinto"
	"orq/internal/limits"
	"orq/internal/memoria"
	"orq/internal/orqdir"
	"orq/internal/perfil"
	"orq/internal/plan"
	"orq/internal/slice"
	"orq/internal/terminal"
	"orq/internal/terminal/herdr"
	"orq/internal/terminal/orca"
)

// Proyecto es un repositorio con orq: su raíz, su carpeta .orq, su configuración y sus puertos.
type Proyecto struct {
	Root, Dir string
	Cfg       config.Config
	Term      terminal.Driver
	Ahora     func() time.Time
	Medir     Medir // lecturas de la máquina; vacío = /proc
}

func (p Proyecto) medir() Medir {
	if p.Medir.Maquina != nil && p.Medir.Vivos != nil {
		return p.Medir
	}
	return MedirReal()
}

// NuevoDriver elige el adaptador del terminal configurado. Se reemplaza en pruebas.
var NuevoDriver = func(nombre string) terminal.Driver {
	if nombre == "orca" {
		return orca.Nuevo()
	}
	return herdr.Nuevo()
}

// conNombre makes every line and launch prompt orq2 hands an agent say orq2; orq gets d itself.
func conNombre(d terminal.Driver) terminal.Driver {
	if !identidad.EsOrq2() {
		return d
	}
	return driverNombrado{d}
}

type driverNombrado struct{ terminal.Driver }

func (d driverNombrado) Avisar(ctx context.Context, destino, texto string) error {
	return d.Driver.Avisar(ctx, destino, identidad.Texto(texto))
}

func (d driverNombrado) Arrancar(ctx context.Context, a terminal.Arranque) error {
	args := make([]string, len(a.Args))
	for i, s := range a.Args {
		args[i] = identidad.Texto(s)
	}
	a.Args = args
	return d.Driver.Arrancar(ctx, a)
}

// Abrir ubica el proyecto del directorio actual y carga su configuración.
func Abrir() (Proyecto, error) {
	root, err := orqdir.Root()
	if err != nil {
		return Proyecto{}, errors.New("orq: no estás dentro de un repositorio git")
	}
	dir, err := orqdir.Dir()
	if err != nil {
		return Proyecto{}, err
	}
	// Una configuración mal escrita detiene el comando: no se adivina qué quiso decir.
	cfg, err := config.Cargar(root)
	if err != nil {
		return Proyecto{}, fmt.Errorf("orq: %w. Revísala con `orq config ver`", err)
	}
	return Proyecto{Root: root, Dir: dir, Cfg: cfg, Term: conNombre(NuevoDriver(cfg.Terminal)), Ahora: time.Now}, nil
}

func (p Proyecto) ahora() time.Time {
	if p.Ahora != nil {
		return p.Ahora()
	}
	return time.Now()
}

func (p Proyecto) MemPath() string            { return filepath.Join(p.Dir, "memoria.jsonl") }
func (p Proyecto) CanalPath() string          { return filepath.Join(p.Dir, "canal.jsonl") }
func (p Proyecto) SesPath() string            { return filepath.Join(p.Dir, "sesiones.json") }
func (p Proyecto) RelevoPath(r string) string { return filepath.Join(p.Dir, "relevo-"+r+".md") }
func (p Proyecto) PerfilPath() string         { return filepath.Join(p.Dir, "proyecto.json") }
func (p Proyecto) FichaPath() string          { return filepath.Join(p.Dir, "PROYECTO.md") }
func (p Proyecto) PlanPath(nn string) string  { return plan.Path(p.Dir, nn) }
func (p Proyecto) LockPath() string           { return filepath.Join(p.Dir, "skills.lock.json") }
func (p Proyecto) InstPath() string           { return filepath.Join(p.Dir, "instintos.json") }
func (p Proyecto) DiarioPath() string         { return diario.Ruta(p.Dir) }

// Slices devuelve el repositorio de slices del proyecto con los límites vigentes.
func (p Proyecto) Slices() slice.Repo {
	home, _ := os.UserHomeDir()
	base := os.Getenv("ORQ_WORKTREES")
	if base == "" {
		base = filepath.Join(home, "worktrees")
	}
	r := slice.Repo{Root: p.Root, OrqDir: p.Dir, Worktree: base, Checks: p.Cfg.ChecksEnOrden(), Definiciones: p.Cfg.ArchivosDefinicion()}
	if l, err := limits.Load(limits.DefaultPath()); err == nil {
		r.MaxSlices = l.MaxSlices
	}
	return r
}

// Excluir deja .orq fuera de git sin tocar archivos versionados del proyecto.
func (p Proyecto) Excluir() error {
	path := filepath.Join(p.Root, ".git", "info", "exclude")
	raw, _ := os.ReadFile(path)
	if strings.Contains(string(raw), "/.orq/") {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString("/.orq/\n")
	return err
}

// Escriba es el modelo barato configurado para resumir memoria, relevos y la ficha.
func (p Proyecto) Escriba() escriba.Escriba {
	e := p.Cfg.Rol("escriba")
	return escriba.Nuevo(e.Modelo, e.Entorno)
}

// Perfil es el perfil guardado por `orq init`, o el detectado si no hay.
func (p Proyecto) Perfil() perfil.Perfil {
	var pf perfil.Perfil
	if raw, err := os.ReadFile(p.PerfilPath()); err == nil && json.Unmarshal(raw, &pf) == nil && pf.Modo != "" {
		return pf
	}
	return perfil.Detect(p.Root)
}

// RolActual sale de ORQ_ROL (paneles creados por orq) o del panel registrado del Atena.
func (p Proyecto) RolActual() string {
	if r := os.Getenv("ORQ_ROL"); r != "" {
		return r
	}
	ses, _ := canal.LoadSesiones(p.SesPath())
	return canal.RolDePane(ses, p.Term.PanelActual())
}

// MemVigente es la memoria que vale hoy: sin reemplazados ni vencidos con slices cerrados.
func (p Proyecto) MemVigente() []memoria.Item {
	items, _ := memoria.Load(p.MemPath())
	cerrados := map[string]bool{}
	raw, _ := os.ReadFile(filepath.Join(p.Dir, "slices.jsonl"))
	for _, l := range strings.Split(string(raw), "\n") {
		var r slice.Registro
		if json.Unmarshal([]byte(l), &r) == nil && r.NN != "" {
			cerrados["S-"+r.NN] = true
		}
	}
	return memoria.SinVencer(memoria.Vigentes(items), cerrados)
}

// Fijadas son las skills cuya versión revisada quedó guardada.
func (p Proyecto) Fijadas() map[string]perfil.Fijada {
	m := map[string]perfil.Fijada{}
	if raw, err := os.ReadFile(p.LockPath()); err == nil {
		json.Unmarshal(raw, &m)
	}
	return m
}

// AprenderSolo registra una lección sin frenar el comando que la produjo.
func (p Proyecto) AprenderSolo(texto, fuente, evidencia string) {
	instinto.Aprender(p.InstPath(), Recortar(texto, instinto.MaxTexto), fuente, evidencia, p.ahora())
}

// Lecciones es lo que entra al arrancar: pocas, confiables, del proyecto y globales.
func (p Proyecto) Lecciones() string {
	top := instinto.Top(instinto.Inyectar, p.InstPath(), instinto.Global())
	if len(top) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n## Lecciones aprendidas (aplícalas; si una falla: orq aprender refutar ID)\n")
	for _, x := range top {
		fmt.Fprintf(&b, "- [%s %.2f] %s\n", x.ID, x.Conf, x.Texto)
	}
	return b.String()
}

// HayContexto dice si algún scout dejó criterios para ese slice (S-NN).
func (p Proyecto) HayContexto(id string) bool {
	items, _ := memoria.Load(p.MemPath())
	for _, it := range items {
		if it.Slice == id && it.Fuente == "scout" {
			return true
		}
	}
	return false
}

// anotar deja un evento en el diario. Un diario que no se puede escribir detiene la operación:
// sin registro no hay forma de repetirla con seguridad.
func (p Proyecto) anotar(e diario.Evento) error {
	e.TS = p.ahora()
	if err := diario.Anotar(p.DiarioPath(), e); err != nil {
		return fmt.Errorf("no se pudo escribir el diario (%s): %w", p.DiarioPath(), err)
	}
	return nil
}

// Recortar acorta s a n caracteres con puntos suspensivos.
func Recortar(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// Valor muestra «(nadie)» cuando no hay escritor.
func Valor(s string) string {
	if s == "" {
		return "(nadie)"
	}
	return s
}

func lista(xs []string, vacio string) string {
	if len(xs) == 0 {
		return "- " + vacio
	}
	return "- " + strings.Join(xs, "\n- ")
}

// fondo es el contexto de una llamada sin plazo propio: cada puerto pone el suyo.
func fondo() context.Context { return context.Background() }
