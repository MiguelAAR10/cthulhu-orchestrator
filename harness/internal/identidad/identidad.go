// Package identidad ancla los roles privilegiados en algo que no sale del entorno de un worker:
// un token del Owner guardado fuera del repo y de los worktrees (~/.config/orq/owner.token, 0600).
// ORQ_ROL dice qué rol se reclama; ORQ_TOKEN prueba que quien lo reclama recibió el token del Owner.
// orq nunca pasa ORQ_TOKEN a un worker. Ver docs/v2/specs/identity.md.
package identidad

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// Dir es la carpeta de configuración de orq del usuario (XDG_CONFIG_HOME/orq o ~/.config/orq).
func Dir() string {
	d, err := os.UserConfigDir()
	if err != nil {
		h, _ := os.UserHomeDir()
		d = filepath.Join(h, ".config")
	}
	return filepath.Join(d, "orq")
}

// Path es el archivo del token del Owner.
func Path() string { return filepath.Join(Dir(), "owner.token") }

// Leer devuelve el token guardado, o "" si no existe o no es privado (0600).
func Leer() string {
	info, err := os.Stat(Path())
	if err != nil || info.Mode().Perm()&0o077 != 0 {
		return ""
	}
	raw, err := os.ReadFile(Path())
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw))
}

// Asegurar crea el token si falta (carpeta 0700, archivo 0600) y lo devuelve.
func Asegurar() (string, error) {
	if t := Leer(); t != "" {
		return t, nil
	}
	if info, err := os.Stat(Path()); err == nil && info.Mode().Perm()&0o077 != 0 {
		return "", errors.New("orq: " + Path() + " is readable by other users; fix it with chmod 600 or delete it")
	}
	if err := os.MkdirAll(Dir(), 0o700); err != nil {
		return "", err
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	t := hex.EncodeToString(b)
	f, err := os.OpenFile(Path(), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) { // otro proceso lo creó a la vez
			if t := Leer(); t != "" {
				return t, nil
			}
		}
		return "", err
	}
	defer f.Close()
	if _, err := f.WriteString(t + "\n"); err != nil {
		return "", err
	}
	return t, nil
}

// Valido: ORQ_TOKEN del entorno coincide con el token guardado.
func Valido() bool { return Coincide(os.Getenv("ORQ_TOKEN")) }

// Coincide compara en tiempo constante con el token guardado.
func Coincide(t string) bool {
	g := Leer()
	return t != "" && g != "" && subtle.ConstantTimeCompare([]byte(strings.TrimSpace(t)), []byte(g)) == 1
}
