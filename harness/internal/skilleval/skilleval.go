// Package skilleval puntúa una skill con `claude plugin eval`, guarda una nota por corrida y elige
// las skills de un worker por su nota.
package skilleval

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// Hash identifica el contenido de una skill: sus archivos, por ruta relativa, con sus bytes.
func Hash(dir string) (string, error) {
	var rutas []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && d.Type().IsRegular() {
			rel, _ := filepath.Rel(dir, p)
			rutas = append(rutas, filepath.ToSlash(rel))
		}
		return err
	})
	if err != nil {
		return "", err
	}
	sort.Strings(rutas)
	h := sha256.New()
	for _, r := range rutas {
		raw, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(r)))
		if err != nil {
			return "", err
		}
		h.Write([]byte(r + "\x00"))
		h.Write(raw)
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// Envolver deja en dst un plugin de una sola skill, la forma que `claude plugin eval` acepta:
// el manifiesto, la skill en skills/<nombre>/ y sus casos (evals/ de la skill) en la raíz del plugin.
func Envolver(skillDir, dst string) (nombre string, err error) {
	nombre = filepath.Base(filepath.Clean(skillDir))
	if _, err := os.Stat(filepath.Join(skillDir, "SKILL.md")); err != nil {
		return "", fmt.Errorf("%s no es una skill: falta SKILL.md", skillDir)
	}
	casos := filepath.Join(skillDir, "evals")
	if st, err := os.Stat(casos); err != nil || !st.IsDir() {
		return "", fmt.Errorf("%s sin casos: falta evals/ (un caso por carpeta, evals/<caso>/prompt.md)", skillDir)
	}
	manifiesto, _ := json.Marshal(map[string]string{"name": nombre, "version": "0.0.0", "description": "orq: one-skill wrapper for claude plugin eval"})
	if err := os.MkdirAll(filepath.Join(dst, ".claude-plugin"), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(dst, ".claude-plugin", "plugin.json"), manifiesto, 0o644); err != nil {
		return "", err
	}
	if err := copiar(skillDir, filepath.Join(dst, "skills", nombre), casos); err != nil {
		return "", err
	}
	return nombre, copiar(casos, filepath.Join(dst, "evals"), "")
}

// copiar copia el árbol de origen en destino, sin la carpeta saltar.
func copiar(origen, destino, saltar string) error {
	return filepath.WalkDir(origen, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && p == saltar {
			return filepath.SkipDir
		}
		rel, _ := filepath.Rel(origen, p)
		dst := filepath.Join(destino, rel)
		if d.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		if !d.Type().IsRegular() {
			return errors.New(p + ": solo se copian archivos regulares")
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(dst, raw, 0o644)
	})
}
