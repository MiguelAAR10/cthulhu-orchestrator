package perfil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"orq/internal/config"
)

func archivos(t *testing.T, fs map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for f, body := range fs {
		if err := os.WriteFile(filepath.Join(root, f), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestChecksGo(t *testing.T) {
	got := Checks(archivos(t, map[string]string{"go.mod": "module x\n"}))
	want := map[string]config.Check{
		"lint":  {Cmd: "! gofmt -l . | grep ."},
		"types": {Cmd: "go vet ./..."},
		"tests": {Cmd: "go test ./..."},
	}
	if len(got) != 3 || got["lint"] != want["lint"] || got["types"] != want["types"] || got["tests"] != want["tests"] {
		t.Fatalf("got %+v", got)
	}
}

func TestChecksTS(t *testing.T) {
	got := Checks(archivos(t, map[string]string{"package.json": `{"scripts":{"lint":"sh stubs/lint.sh","typecheck":"sh stubs/tipos.sh","test":"sh stubs/test.sh"}}`}))
	if got["lint"].Cmd != "npm run lint" || got["types"].Cmd != "npm run typecheck" || got["tests"].Cmd != "npm test" || len(got) != 3 {
		t.Fatalf("got %+v", got)
	}
}

func TestChecksTSSinScripts(t *testing.T) {
	got := Checks(archivos(t, map[string]string{
		"package.json":  `{"scripts":{"test":"echo \"Error: no test specified\" && exit 1"},"devDependencies":{"typescript":"5.6.0"}}`,
		"tsconfig.json": `{}`,
	}))
	if got["lint"].Cmd != "" || !strings.Contains(got["lint"].Omitir, "lint") {
		t.Errorf("lint = %+v", got["lint"])
	}
	if got["types"].Cmd != "npx --no-install tsc --noEmit" {
		t.Errorf("types = %+v", got["types"])
	}
	if got["tests"].Cmd != "" || !strings.Contains(got["tests"].Omitir, "test") {
		t.Errorf("tests = %+v", got["tests"])
	}
}

func TestChecksSinStack(t *testing.T) {
	got := Checks(archivos(t, map[string]string{"README.md": "hola"}))
	for _, n := range config.ChecksBase {
		if got[n].Cmd != "" || !strings.Contains(got[n].Omitir, ".orq/config.json") {
			t.Errorf("%s = %+v", n, got[n])
		}
	}
}
