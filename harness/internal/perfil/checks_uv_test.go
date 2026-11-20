package perfil

import (
	"reflect"
	"testing"

	"orq/internal/config"
)

const pyTodo = `[project]
name = "x"
dependencies = ["fastapi"]

[dependency-groups]
dev = ["pytest>=8", "mypy>=1.10"]

[tool.ruff]
line-length = 100
`

const uvPre = "PYTHONPATH=src uv run --offline --no-sync "

func TestChecksUvConSrcYTests(t *testing.T) {
	root := archivos(t, map[string]string{"pyproject.toml": pyTodo, "uv.lock": "", "src/x/__init__.py": "", "tests/test_x.py": ""})
	got := ChecksPara(root, true)
	want := map[string]config.Check{
		"lint":  {Cmd: uvPre + "ruff check src tests && " + uvPre + "ruff format --check src tests"},
		"types": {Cmd: uvPre + "mypy src"},
		"tests": {Cmd: uvPre + "pytest -q -p no:cacheprovider"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v\nwant %#v", got, want)
	}
}

func TestChecksUvSinSrc(t *testing.T) {
	root := archivos(t, map[string]string{"pyproject.toml": pyTodo, "uv.lock": "", "app.py": ""})
	got := ChecksPara(root, true)
	const p = "uv run --offline --no-sync "
	want := map[string]config.Check{
		"lint":  {Cmd: p + "ruff check . && " + p + "ruff format --check ."},
		"types": {Cmd: p + "mypy ."},
		"tests": {Cmd: p + "pytest -q -p no:cacheprovider"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v\nwant %#v", got, want)
	}
}

func TestChecksUvSinMypy(t *testing.T) {
	root := archivos(t, map[string]string{"pyproject.toml": "[project]\ndependencies = [\"pytest\"]\n[tool.ruff]\n", "uv.lock": "", "src/a.py": ""})
	got := ChecksPara(root, true)
	if !reflect.DeepEqual(got["types"], config.Check{Omitir: "pyproject.toml no nombra mypy"}) {
		t.Fatalf("types: %#v", got["types"])
	}
	if got["tests"].Cmd == "" || got["lint"].Cmd == "" {
		t.Fatalf("ruff and pytest are named: %#v", got)
	}
}

func TestChecksPythonSinUvLock(t *testing.T) {
	root := archivos(t, map[string]string{"pyproject.toml": pyTodo, "src/a.py": ""})
	sin := config.Check{Omitir: "Python sin uv.lock: escribe el comando en .orq/config.json"}
	if got := ChecksPara(root, true); !reflect.DeepEqual(got, map[string]config.Check{"lint": sin, "types": sin, "tests": sin}) {
		t.Fatalf("orq2 without uv.lock: %#v", got)
	}
}

// orq (not orq2) keeps today's omission on a uv project: its init output does not change.
func TestChecksUvSoloOrq2(t *testing.T) {
	root := archivos(t, map[string]string{"pyproject.toml": pyTodo, "uv.lock": "", "src/a.py": ""})
	hoy := config.Check{Omitir: "stack sin comandos conocidos: escribe el comando en .orq/config.json"}
	want := map[string]config.Check{"lint": hoy, "types": hoy, "tests": hoy}
	for _, got := range []map[string]config.Check{ChecksPara(root, false), Checks(root)} {
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("orq: %#v", got)
		}
	}
}

// F2: pyproject.toml without uv.lock does not hide the Node checks under orq2.
func TestChecksPyprojectSinUvLockNoOcultaNode(t *testing.T) {
	root := archivos(t, map[string]string{"pyproject.toml": pyTodo, "package.json": `{"scripts":{"lint":"eslint .","test":"jest"}}`})
	got, orq := ChecksPara(root, true), ChecksPara(root, false)
	if !reflect.DeepEqual(got, orq) || got["lint"].Cmd != "npm run lint" || got["tests"].Cmd != "npm test" {
		t.Fatalf("orq2 %#v\norq %#v", got, orq)
	}
}

// F3: a flat layout (no src/) lints the whole tree, not only tests/.
func TestChecksUvFlatLintTodo(t *testing.T) {
	root := archivos(t, map[string]string{"pyproject.toml": pyTodo, "uv.lock": "", "mypkg/__init__.py": "", "tests/test_x.py": ""})
	const p = "uv run --offline --no-sync "
	if got := ChecksPara(root, true)["lint"].Cmd; got != p+"ruff check . && "+p+"ruff format --check ." {
		t.Fatalf("lint: %q", got)
	}
}

// F4: single-quoted TOML strings name a tool too.
func TestChecksUvComillaSimple(t *testing.T) {
	py := "[project]\nname = 'x'\n[dependency-groups]\ndev = ['ruff', 'mypy', 'pytest']\n"
	root := archivos(t, map[string]string{"pyproject.toml": py, "uv.lock": "", "src/a.py": ""})
	for k, c := range ChecksPara(root, true) {
		if c.Cmd == "" {
			t.Fatalf("%s omitted: %q", k, c.Omitir)
		}
	}
}
