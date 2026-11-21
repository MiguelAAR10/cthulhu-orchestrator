// Package orqdir ubica la carpeta .orq del proyecto: vive en el checkout principal,
// así el orquestador y los workers en worktrees comparten la misma memoria.
package orqdir

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"orq/internal/identidad"
)

// Root devuelve la raíz del checkout principal. ORQ_ROOT la fija para pruebas.
func Root() (string, error) {
	if p := os.Getenv("ORQ_ROOT"); p != "" {
		return p, nil
	}
	out, err := exec.Command("git", "rev-parse", "--path-format=absolute", "--git-common-dir").Output()
	if err != nil {
		return "", err
	}
	root := filepath.Dir(strings.TrimSpace(string(out)))
	// orq2: a worker clone has its own .git, so its project is the main checkout its marker
	// names. orq keeps 1.0.0 and uses the clone itself.
	if !identidad.EsOrq2() {
		return root, nil
	}
	if m, ok, err := marcaArriba(root); ok {
		return m, err
	}
	return root, nil
}

// marcaArriba busca la marca de clon de worker desde dir hacia arriba (también desde un repo
// anidado dentro del clon) y devuelve el checkout principal que nombra. Una marca que no nombra
// un directorio absoluto existente es un error: nunca se cae al .orq propio del clon.
func marcaArriba(dir string) (string, bool, error) {
	for d := filepath.Clean(dir); ; d = filepath.Dir(d) {
		path := filepath.Join(d, ".git", MarcaClon)
		raw, err := os.ReadFile(path)
		if err == nil {
			var m struct {
				Root string `json:"root"`
			}
			if err := json.Unmarshal(raw, &m); err != nil {
				return "", true, fmt.Errorf("worker clone marker %s is unreadable (%v): cannot find the project's .orq", path, err)
			}
			if !filepath.IsAbs(m.Root) {
				return "", true, fmt.Errorf("worker clone marker %s names no absolute project root (%q)", path, m.Root)
			}
			if info, err := os.Stat(m.Root); err != nil || !info.IsDir() {
				return "", true, fmt.Errorf("worker clone marker %s names project %s, which is not a directory", path, m.Root)
			}
			return filepath.Clean(m.Root), true, nil
		}
		// .git may be a file (linked worktree): ENOTDIR means no marker there either.
		if !errors.Is(err, os.ErrNotExist) && !errors.Is(err, syscall.ENOTDIR) {
			return "", true, fmt.Errorf("worker clone marker %s: %w", path, err)
		}
		if filepath.Dir(d) == d {
			return "", false, nil
		}
	}
}

// Dir devuelve <raíz>/.orq y la crea si falta.
func Dir() (string, error) {
	root, err := Root()
	if err != nil {
		return "", err
	}
	d := filepath.Join(root, ".orq")
	return d, os.MkdirAll(d, 0o755)
}

// Slug es el nombre corto del repo, apto para nombres de agentes de herdr.
func Slug(root string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(filepath.Base(root)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	s := strings.Trim(b.String(), "-")
	if s == "" || s[0] < 'a' || s[0] > 'z' {
		s = "p" + s
	}
	if len(s) > 18 {
		s = s[:18]
	}
	return s
}

// WriteAtomic escribe por archivo temporal y rename.
func WriteAtomic(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(f.Name())
		return err
	}
	f.Close()
	os.Chmod(f.Name(), 0o644)
	return os.Rename(f.Name(), path)
}

// Candado toma un flock exclusivo sobre path+".lock"; llama a la función devuelta para soltarlo.
func Candado(path string) (func(), error) {
	f, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, err
	}
	return func() { syscall.Flock(int(f.Fd()), syscall.LOCK_UN); f.Close() }, nil
}

// MarcaClon is the file orq leaves in the .git directory of every worker clone.
// The gate treats any repository that carries it as a worker, whatever its content.
const MarcaClon = "orq-clon"
