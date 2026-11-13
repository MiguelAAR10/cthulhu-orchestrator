// Package sandbox encierra en bubblewrap todo lo que ejecuta un worker y todo examen que corre su código.
//
// La raíz de la caja es una lista permitida: nunca `--ro-bind / /`. Se ve el conjunto base de solo
// lectura (/usr /bin /lib /lib64 /etc si existen, más `sandbox.ro` de limits.json) y lo que pide el Spec.
// $HOME, /run y /tmp son tmpfs; /mnt no existe; la red no existe salvo Net. No hay interruptor para
// apagarla: sin bwrap o con la autoprueba fallida, Command devuelve error.
package sandbox

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"orq/internal/limits"
	"orq/internal/state"
)

// Marca es el archivo que solo existe dentro de una caja (un /dev/null montado de solo lectura).
const Marca = "/run/orq/sandbox"

// Spec describe una caja. Todas las rutas son absolutas y se montan en la misma ruta.
type Spec struct {
	RO  []string          // rutas montadas de solo lectura
	RW  []string          // rutas montadas de lectura y escritura
	Cwd string            // directorio de trabajo dentro de la caja; vacío = $HOME (tmpfs)
	Net bool              // false: sin red alguna
	Env map[string]string // las únicas variables del proceso, más PATH y HOME que pone el paquete
}

// Dentro dice si el proceso actual corre dentro de una caja.
func Dentro() bool {
	fi, err := os.Stat(Marca)
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

// Command arma el proceso encerrado. Antes exige Disponible (cacheado por proceso).
// HOME es siempre el $HOME real (vacío, tmpfs); PATH es el PATH del host filtrado a lo visible,
// salvo que s.Env traiga PATH.
func Command(ctx context.Context, s Spec, argv ...string) (*exec.Cmd, error) {
	if len(argv) == 0 {
		return nil, errors.New("sandbox: falta el comando")
	}
	if err := Disponible(); err != nil {
		return nil, err
	}
	return comando(ctx, s, argv)
}

// comando arma el proceso sin pasar por la autoprueba (la autoprueba lo usa).
func comando(ctx context.Context, s Spec, argv []string) (*exec.Cmd, error) {
	bw, err := buscarBwrap()
	if err != nil {
		return nil, err
	}
	h, err := hostFn()
	if err != nil {
		return nil, err
	}
	args, env, err := construir(s, h, argv)
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, bw, args...)
	// bwrap arranca con el entorno final y nada más: así /proc/1/environ dentro no filtra el del lanzador.
	cmd.Env = env
	return cmd, nil
}

// rutasBwrap: primero rutas fijas del sistema, para que un bwrap falso en el PATH no gane.
var rutasBwrap = []string{"/usr/bin/bwrap", "/usr/local/bin/bwrap", "/bin/bwrap"}

func buscarBwrap() (string, error) {
	for _, p := range rutasBwrap {
		if fi, err := os.Stat(p); err == nil && fi.Mode().IsRegular() && fi.Mode()&0o111 != 0 {
			return p, nil
		}
	}
	if len(rutasBwrap) > 0 && rutasBwrap[0] == "/usr/bin/bwrap" {
		if p, err := exec.LookPath("bwrap"); err == nil && filepath.IsAbs(p) {
			return p, nil
		}
	}
	return "", errors.New("sandbox: no está bwrap (bubblewrap); instálalo: sin caja no corre nada de un worker")
}

// host reúne lo que la caja toma de la máquina. construir es pura sobre él, para probarla.
type host struct {
	Home        string
	BaseRO      []string    // directorios reales del sistema (/usr /etc ...), ya existentes
	ExtraRO     []string    // sandbox.ro de limits.json que existen; se validan como las del Spec
	Enlaces     [][2]string // {destino, ruta}: /bin -> usr/bin se recrea como enlace
	Resolv      string      // destino real de /etc/resolv.conf cuando está fuera de /etc y /usr
	PathHost    string
	NuevaSesion bool     // TIOCSTI heredado activo en el kernel: hay que cortar la terminal
	Sensibles   []string // nunca visibles: sockets y /run
	SensiblesRW []string // nunca dentro de un RW: el directorio de estado y el archivo de límites
}

// hostFn es hostReal salvo en las pruebas que simulan una caja rota.
var hostFn = hostReal

func hostReal() (host, error) {
	home, err := os.UserHomeDir()
	if err != nil || !filepath.IsAbs(home) || filepath.Clean(home) == "/" {
		return host{}, fmt.Errorf("sandbox: $HOME inválido (%q)", home)
	}
	home = filepath.Clean(home)
	h := host{Home: home, PathHost: os.Getenv("PATH")}
	for _, p := range []string{"/usr", "/bin", "/lib", "/lib64", "/etc"} {
		fi, err := os.Lstat(p)
		if err != nil {
			continue
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			if dst, err := os.Readlink(p); err == nil {
				h.Enlaces = append(h.Enlaces, [2]string{dst, p})
			}
			continue
		}
		if fi.IsDir() {
			h.BaseRO = append(h.BaseRO, p)
		}
	}
	l, err := limits.Load(limits.DefaultPath())
	switch {
	case err == nil:
		for _, p := range l.Sandbox.RO {
			if !filepath.IsAbs(p) {
				return host{}, fmt.Errorf("sandbox: sandbox.ro en limits.json debe ser absoluta o ~/: %q", p)
			}
			if _, err := os.Stat(p); err == nil {
				h.ExtraRO = append(h.ExtraRO, filepath.Clean(p))
			}
		}
	case errors.Is(err, fs.ErrNotExist):
		// sin limits.json la caja ve menos, nunca más
	default:
		return host{}, err
	}
	if fi, err := os.Lstat("/etc/resolv.conf"); err == nil && fi.Mode()&os.ModeSymlink != 0 {
		if dst, err := filepath.EvalSymlinks("/etc/resolv.conf"); err == nil && !bajo(dst, "/etc") && !bajo(dst, "/usr") {
			h.Resolv = dst
		}
	}
	if raw, err := os.ReadFile("/proc/sys/dev/tty/legacy_tiocsti"); err != nil || strings.TrimSpace(string(raw)) != "0" {
		h.NuevaSesion = true
	}
	h.Sensibles = []string{filepath.Join(home, ".config", "herdr"), "/run", "/var/run", "/proc", "/dev"}
	if dst, err := filepath.EvalSymlinks("/var/run/docker.sock"); err == nil {
		h.Sensibles = append(h.Sensibles, dst)
	}
	h.SensiblesRW = []string{filepath.Clean(state.Dir()), filepath.Clean(limits.DefaultPath())}
	return h, nil
}

// bajo dice si p es base o está dentro de base.
func bajo(p, base string) bool {
	return p == base || base == "/" || strings.HasPrefix(p, base+"/")
}

type montaje struct {
	src, dst string
	rw       bool
}

// validar rechaza rutas que abrirían la caja: relativas, /, $HOME o sus ancestros, sockets, /run.
func validar(p string, rw bool, h host) (montaje, error) {
	if !filepath.IsAbs(p) {
		return montaje{}, fmt.Errorf("sandbox: la ruta debe ser absoluta: %q", p)
	}
	p = filepath.Clean(p)
	if p == "/" || bajo(h.Home, p) {
		return montaje{}, fmt.Errorf("sandbox: %s expone $HOME entero o la raíz", p)
	}
	real, err := filepath.EvalSymlinks(p)
	if err != nil {
		return montaje{}, fmt.Errorf("sandbox: %s no existe: %w", p, err)
	}
	if real == "/" || bajo(h.Home, real) {
		return montaje{}, fmt.Errorf("sandbox: %s (%s) expone $HOME entero o la raíz", p, real)
	}
	for _, q := range []string{p, real} {
		for _, s := range h.Sensibles {
			if bajo(q, s) || bajo(s, q) {
				return montaje{}, fmt.Errorf("sandbox: %s toca %s, que no puede entrar en la caja", p, s)
			}
		}
		// Escribible: no puede contener el estado ni los límites (un subdirectorio, como scratch, sí).
		for _, s := range h.SensiblesRW {
			if rw && bajo(s, q) {
				return montaje{}, fmt.Errorf("sandbox: %s contiene %s, que el worker no puede escribir", p, s)
			}
		}
	}
	return montaje{src: real, dst: p, rw: rw}, nil
}

// construir devuelve los argumentos de bwrap y el entorno con que arranca.
func construir(s Spec, h host, argv []string) ([]string, []string, error) {
	if len(argv) == 0 {
		return nil, nil, errors.New("sandbox: falta el comando")
	}
	var ms []montaje
	for _, p := range h.BaseRO {
		ms = append(ms, montaje{src: p, dst: p})
	}
	for _, p := range append(slices.Clone(h.ExtraRO), s.RO...) {
		m, err := validar(p, false, h)
		if err != nil {
			return nil, nil, err
		}
		ms = append(ms, m)
	}
	for _, p := range s.RW {
		m, err := validar(p, true, h)
		if err != nil {
			return nil, nil, err
		}
		ms = append(ms, m)
	}
	if s.Net && h.Resolv != "" {
		ms = append(ms, montaje{src: h.Resolv, dst: h.Resolv})
	}
	// Una ruta pedida dos veces con permisos distintos es ambigua: mejor fallar.
	vistos := map[string]bool{}
	var unicos []montaje
	for _, m := range ms {
		if rw, ok := vistos[m.dst]; ok {
			if rw != m.rw {
				return nil, nil, fmt.Errorf("sandbox: %s pedida a la vez de solo lectura y de escritura", m.dst)
			}
			continue
		}
		vistos[m.dst] = m.rw
		unicos = append(unicos, m)
	}
	// Padres antes que hijos: un RO dentro de un RW (node_modules en el clon) queda encima.
	sort.SliceStable(unicos, func(i, j int) bool {
		return strings.Count(unicos[i].dst, "/") < strings.Count(unicos[j].dst, "/")
	})

	a := []string{"--unshare-all", "--die-with-parent"}
	if s.Net {
		a = append(a, "--share-net")
	}
	if h.NuevaSesion {
		a = append(a, "--new-session")
	}
	a = append(a, "--proc", "/proc", "--dev", "/dev",
		"--tmpfs", "/tmp", "--tmpfs", "/run", "--dir", "/run/orq", "--ro-bind", "/dev/null", Marca,
		"--tmpfs", h.Home)
	for _, e := range h.Enlaces {
		a = append(a, "--symlink", e[0], e[1])
	}
	for _, m := range unicos {
		if m.rw {
			a = append(a, "--bind", m.src, m.dst)
		} else {
			a = append(a, "--ro-bind", m.src, m.dst)
		}
	}
	cwd := s.Cwd
	if cwd == "" {
		cwd = h.Home
	}
	if !filepath.IsAbs(cwd) {
		return nil, nil, fmt.Errorf("sandbox: Cwd debe ser absoluto: %q", cwd)
	}
	// La raíz de la caja es un tmpfs de bwrap: de solo lectura, nada se escribe fuera de los montajes.
	a = append(a, "--remount-ro", "/", "--chdir", filepath.Clean(cwd), "--")
	a = append(a, argv...)

	env := map[string]string{}
	for k, v := range s.Env {
		if k == "" || strings.ContainsAny(k, "=\x00") || strings.Contains(v, "\x00") {
			return nil, nil, fmt.Errorf("sandbox: variable inválida %q", k)
		}
		env[k] = v
	}
	env["HOME"] = h.Home
	if _, ok := s.Env["PATH"]; !ok {
		env["PATH"] = pathVisible(h, unicos)
	}
	var lista []string
	for k, v := range env {
		lista = append(lista, k+"="+v)
	}
	sort.Strings(lista)
	return a, lista, nil
}

// pathVisible deja del PATH del host solo los directorios que existen dentro de la caja.
func pathVisible(h host, ms []montaje) string {
	visibles := []string{}
	for _, m := range ms {
		visibles = append(visibles, m.dst)
	}
	for _, e := range h.Enlaces {
		visibles = append(visibles, e[1])
	}
	var out []string
	add := func(d string) {
		if !slices.Contains(out, d) {
			out = append(out, d)
		}
	}
	for _, d := range filepath.SplitList(h.PathHost) {
		if !filepath.IsAbs(d) {
			continue
		}
		d = filepath.Clean(d)
		for _, v := range visibles {
			if bajo(d, v) {
				add(d)
				break
			}
		}
	}
	for _, d := range []string{"/usr/local/bin", "/usr/bin", "/bin"} {
		add(d)
	}
	return strings.Join(out, ":")
}
