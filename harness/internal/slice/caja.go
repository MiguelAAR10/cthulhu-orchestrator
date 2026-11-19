package slice

// Todo lo que ejecuta código de un worker corre encerrado: el examen, --sano, la revalidación al
// integrar y el git que lee su clon. No hay modo «sin caja».

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"orq/internal/gitx"
	"orq/internal/sandbox"
)

// Caja es el puerto del sandbox: arma un proceso encerrado. La única implementación de producción
// es bubblewrap (internal/sandbox); si no está disponible devuelve error y nada corre.
type Caja interface {
	Command(ctx context.Context, s sandbox.Spec, argv ...string) (*exec.Cmd, error)
}

type cajaReal struct{}

func (cajaReal) Command(ctx context.Context, s sandbox.Spec, argv ...string) (*exec.Cmd, error) {
	return sandbox.Command(ctx, s, argv...)
}

func (r Repo) caja() Caja {
	if r.Caja != nil {
		return r.Caja
	}
	return cajaReal{}
}

// git corre git en el repo principal, que es de confianza. Nunca en el repo de un worker.
func (r Repo) git(args ...string) (string, error) {
	return gitx.Principal(context.Background(), r.Root, args...)
}

// gitCodigo es git con respuesta sí/no por código de salida (merge-base --is-ancestor, merge-tree).
func (r Repo) gitCodigo(args ...string) (string, int, error) {
	return gitx.Codigo(context.Background(), r.Root, args...)
}

// esAncestro dice si a está contenido en b, según el repo principal.
func (r Repo) esAncestro(a, b string) (bool, error) {
	_, code, err := r.gitCodigo("merge-base", "--is-ancestor", a, b)
	if err != nil {
		return false, err
	}
	if code > 1 {
		return false, fmt.Errorf("git merge-base --is-ancestor %s %s: código %d", a, b, code)
	}
	return code == 0, nil
}

// enCaja corre argv encerrado y devuelve salida, código y si salió bien. err solo si la caja no arrancó.
func (r Repo) enCaja(spec sandbox.Spec, limite time.Duration, stdin io.Reader, argv ...string) (string, int, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), limite)
	defer cancel()
	c, err := r.caja().Command(ctx, spec, argv...)
	if err != nil {
		return "", -1, false, fmt.Errorf("sin caja no corre nada de un worker: %w", err)
	}
	c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	c.Cancel = func() error { return syscall.Kill(-c.Process.Pid, syscall.SIGKILL) }
	c.WaitDelay = 2 * time.Second
	c.Stdin = stdin
	var buf bytes.Buffer
	c.Stdout, c.Stderr = &buf, &buf
	errRun := c.Run()
	out := buf.String()
	if ctx.Err() != nil {
		return out + fmt.Sprintf("\n[orq] cortado tras %s\n", limite), -1, false, nil
	}
	var ee *exec.ExitError
	switch {
	case errRun == nil:
		return out, 0, true, nil
	case errors.As(errRun, &ee):
		return out, ee.ExitCode(), false, nil
	}
	return out, -1, false, fmt.Errorf("la caja no arrancó: %w", errRun)
}

// entornoExamen: lo mínimo, sin nada del lanzador. Git sin configuración global ni del sistema.
func entornoExamen() map[string]string {
	return map[string]string{"GIT_CONFIG_GLOBAL": "/dev/null", "GIT_CONFIG_SYSTEM": "/dev/null", "GIT_OPTIONAL_LOCKS": "0",
		"LANG": "C.UTF-8", "CI": "1"}
}

// dependencias del proyecto que se ven de solo lectura dentro de una copia: node_modules se monta
// en la copia; .venv se monta en su ruta real (un venv lleva rutas absolutas) con un enlace en la copia.
func (r Repo) depsEn(copia string) (ro []string, vinculos []sandbox.Vinculo) {
	if fi, err := os.Stat(filepath.Join(r.Root, "node_modules")); err == nil && fi.IsDir() {
		if _, err := os.Lstat(filepath.Join(copia, "node_modules")); errors.Is(err, os.ErrNotExist) {
			vinculos = append(vinculos, sandbox.Vinculo{Origen: filepath.Join(r.Root, "node_modules"), Destino: filepath.Join(copia, "node_modules")})
		}
	}
	venv := filepath.Join(r.Root, ".venv")
	if fi, err := os.Stat(venv); err == nil && fi.IsDir() {
		if _, err := os.Lstat(filepath.Join(copia, ".venv")); errors.Is(err, os.ErrNotExist) {
			if os.Symlink(venv, filepath.Join(copia, ".venv")) == nil {
				ro = append(ro, venv)
			}
		}
	}
	return ro, vinculos
}

// copiaDe extrae el commit sha del repo principal en una carpeta temporal nueva. El tar se desempaqueta
// dentro de la caja: el contenido lo escribió un worker.
func (r Repo) copiaDe(sha string) (string, func(), error) {
	dir, err := os.MkdirTemp("", "orq-exec-")
	if err != nil {
		return "", nil, err
	}
	limpiar := func() { os.RemoveAll(dir) }
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	arch := gitx.Archivo(ctx, r.Root, sha)
	var errArch bytes.Buffer
	arch.Stderr = &errArch
	tubo, err := arch.StdoutPipe()
	if err != nil {
		limpiar()
		return "", nil, err
	}
	if err := arch.Start(); err != nil {
		limpiar()
		return "", nil, fmt.Errorf("git archive %s: %w", sha, err)
	}
	out, _, ok, errCaja := r.enCaja(sandbox.Spec{RW: []string{dir}, Cwd: dir, Env: entornoExamen()}, 5*time.Minute, tubo, "tar", "-x", "-f", "-")
	errWait := arch.Wait()
	switch {
	case errCaja != nil:
		limpiar()
		return "", nil, errCaja
	case errWait != nil:
		limpiar()
		return "", nil, fmt.Errorf("git archive %s: %s", sha, strings.TrimSpace(errArch.String()))
	case !ok:
		limpiar()
		return "", nil, fmt.Errorf("copia limpia de %s: %s", sha, strings.TrimSpace(out))
	}
	return dir, limpiar, nil
}

// correrEn corre cmd con sh dentro de la caja, en la copia dir, sin red.
// La caja no ve $HOME: la toolchain de cada herramienta que nombra cmd (examen, --sano o check)
// se monta de solo lectura, y en un proyecto con checks.e2e, la carpeta de navegadores de Playwright.
func (r Repo) correrEn(dir, cmd string, limite time.Duration) (string, int, bool, error) {
	ro, vinc := r.depsEn(dir)
	ro = append(ro, sandbox.RODe(sandbox.Herramientas(cmd))...)
	env := entornoExamen()
	if _, web := r.e2e(); web {
		if nav := sandbox.Navegadores(); len(nav.RO) > 0 {
			ro = append(ro, nav.RO...)
			env["PLAYWRIGHT_BROWSERS_PATH"] = nav.Ruta
		}
	}
	return r.enCaja(sandbox.Spec{RW: []string{dir}, RO: ro, Vinculos: vinc, Cwd: dir, Env: env}, limite, nil, "sh", "-c", cmd)
}

// MaxReporte acota lo que orq lee de un archivo que escribió el examen.
const MaxReporte = 10 << 20

// leerSeguro lee base/rel sin seguir ningún enlace simbólico en el camino, solo si es un archivo
// regular y pequeño. base es una carpeta de orq; rel lo eligió quien escribió el examen.
func leerSeguro(base, rel string, max int64) ([]byte, error) {
	rel = filepath.Clean(rel)
	if filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, "../") {
		return nil, fmt.Errorf("ruta fuera de la copia: %s", rel)
	}
	fd, err := syscall.Open(base, syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	partes := strings.Split(rel, "/")
	for i, p := range partes {
		flags := syscall.O_RDONLY | syscall.O_NOFOLLOW | syscall.O_CLOEXEC | syscall.O_NONBLOCK
		if i < len(partes)-1 {
			flags |= syscall.O_DIRECTORY
		}
		sig, err := syscall.Openat(fd, p, flags, 0)
		syscall.Close(fd)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", rel, err)
		}
		fd = sig
	}
	f := os.NewFile(uintptr(fd), rel)
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !fi.Mode().IsRegular() {
		return nil, fmt.Errorf("%s no es un archivo regular", rel)
	}
	if fi.Size() > max {
		return nil, fmt.Errorf("%s pesa %d bytes; el máximo es %d", rel, fi.Size(), max)
	}
	return io.ReadAll(io.LimitReader(f, max))
}

// huellaSegura es Huella leyendo cada archivo con leerSeguro: la copia la pudo tocar el examen.
func huellaSegura(dir string, bundle []string, cmd string) (string, error) {
	var datos [][]byte
	for _, f := range bundle {
		raw, err := leerSeguro(dir, f, MaxReporte)
		if err != nil {
			return "", err
		}
		datos = append(datos, raw)
	}
	return huellaDe(bundle, datos, cmd), nil
}
