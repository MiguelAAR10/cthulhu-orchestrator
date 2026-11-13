package config

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Los nombres de modelo viven solo en este paquete: el código, las guías y las skills los toman de aquí.
func TestNingunModeloFueraDeConfig(t *testing.T) {
	modelo := regexp.MustCompile(`(?i)\b(haiku|sonnet|opus|gpt-[0-9][a-z0-9.-]*)\b`)
	raiz := filepath.Join("..") // harness/internal
	err := filepath.WalkDir(raiz, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "config" {
				return filepath.SkipDir
			}
			return nil
		}
		ext := filepath.Ext(p)
		if ext != ".go" && ext != ".md" || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		for i, l := range strings.Split(string(raw), "\n") {
			if m := modelo.FindString(l); m != "" {
				t.Errorf("%s:%d nombra el modelo %q; debe salir de la configuración", p, i+1, m)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
