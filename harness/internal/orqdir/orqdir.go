// Package orqdir ubica la carpeta .orq del proyecto: vive en el checkout principal,
// así el orquestador y los workers en worktrees comparten la misma memoria.
package orqdir

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
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
	return filepath.Dir(strings.TrimSpace(string(out))), nil
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
