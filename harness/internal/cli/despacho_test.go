package cli

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// copiaS01 writes, in a temp dir, a copy shaped like Real State's paused S-01: card approved
// 17h46m ago, plan agreed with T1 ready (T2..T5 wait for T1, T6 for all), no worker, the journal's
// last line `aprobada`, and only the orchestration roles in sesiones.json.
func copiaS01(t *testing.T) (root string, aprobada time.Time) {
	t.Helper()
	root = filepath.Join(t.TempDir(), "ai-agent-realstate")
	orq := filepath.Join(root, ".orq")
	for _, d := range []string{"slices", "planes"} {
		if err := os.MkdirAll(filepath.Join(orq, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now().UTC()
	aprobada = now.Add(-(17*time.Hour + 46*time.Minute + 20*time.Second))
	lista := aprobada.Add(-2 * time.Hour)
	abierto := lista.Add(-time.Minute)
	ts := func(x time.Time) string { return x.Format(time.RFC3339Nano) }
	estado := fmt.Sprintf(`{"nn":"01","intento":1,"fase":"IMPLEMENTANDO","titulo":"Outreach controlado","test":"tests/test_s01.py",
 "bundle":["tests/test_s01.py"],"cmd":"pytest tests/test_s01.py","base":"main","sha_rojo":"139fc2f96fd046da5062228727bc4a58b4660592",
 "rama":"slice/01","worktree":%q,"procedencia":"clon","abierto":%q,"esquema":2,
 "tarjeta":{"historia":["h"],"fuera":["f"],"examen":["e"],"zonas":["src"],"preguntas":["p"],"lista":%q,"aprobada":%q}}`,
		filepath.Join(root, "wt-s01"), ts(abierto), ts(lista), ts(aprobada))
	pl := `{"slice":"01","debatido":true,"tareas":[
 {"id":"T1","zona":"back","rutas":["src/models.py"],"dificultad":"baja","estado":"pendiente"},
 {"id":"T2","zona":"back","rutas":["src/kapso.py"],"depende_de":["T1"],"dificultad":"media","estado":"pendiente"},
 {"id":"T3","zona":"back","rutas":["src/outreach.py"],"depende_de":["T1"],"dificultad":"alta","estado":"pendiente"},
 {"id":"T4","zona":"back","rutas":["src/api.py"],"depende_de":["T1"],"dificultad":"media","estado":"pendiente"},
 {"id":"T5","zona":"front","rutas":["dashboard/"],"depende_de":["T1"],"dificultad":"media","estado":"pendiente"},
 {"id":"T6","zona":"doc","rutas":["docs/"],"depende_de":["T2","T3","T4","T5"],"dificultad":"baja","estado":"pendiente"}]}`
	diario := fmt.Sprintf(`{"ts":%q,"tipo":"fase","slice":"01","intento":1,"ruta":"slice","a":"IMPLEMENTANDO"}
{"ts":%q,"tipo":"tarjeta","slice":"01","intento":1,"ruta":"slice","resultado":"lista"}
{"ts":%q,"tipo":"tarjeta","slice":"01","intento":1,"ruta":"slice","resultado":"aprobada"}
`, ts(abierto), ts(lista), ts(aprobada))
	ses := `{"atena":{"agente":"atena-x","pane":"w4E:p1","desde":"2026-11-20T02:28:35Z","gen":1},
 "orquestador":{"agente":"orquestador-x","pane":"w4E:p2","desde":"2026-11-20T02:28:42Z","gen":1},
 "planificador":{"agente":"planificador-x","pane":"w4E:p3","desde":"2026-11-20T02:30:01Z","gen":1}}`
	for f, c := range map[string]string{"slices/01.json": estado, "planes/01.json": pl, "diario.jsonl": diario, "sesiones.json": ses} {
		if err := os.WriteFile(filepath.Join(orq, f), []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root, aprobada
}

func huella(t *testing.T, dir string) string {
	t.Helper()
	h := sha256.New()
	filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, _ := d.Info()
		fmt.Fprintf(h, "%s %v %v\n", path, info.Mode(), info.ModTime().UnixNano())
		if !d.IsDir() {
			raw, _ := os.ReadFile(path)
			h.Write(raw)
		}
		return nil
	})
	return fmt.Sprintf("%x", h.Sum(nil))
}

// Acceptance (G6, H1): on a copy of S-01, `orq2 pendientes` says the approved slice has T1 ready
// and no worker, since when, who owes it and the command; it writes nothing.
func TestPendientesEnCopiaDeS01(t *testing.T) {
	root, _ := copiaS01(t)
	t.Setenv("ORQ_ROOT", root)
	t.Setenv("ORQ_NOMBRE", "orq2")
	t.Setenv("ORQ_ROL", "")
	term := conTerminalFalso(t)
	antes := huella(t, root)
	out, errs, code := run(t, "pendientes")
	if code != 0 {
		t.Fatalf("code %d: %s", code, errs)
	}
	if huella(t, root) != antes || len(term.Llamadas) != 0 {
		t.Fatal("pendientes wrote state or touched the terminal")
	}
	want := "S-01 «Outreach controlado» aprobada hace 17h46m, T1 lista, sin worker · debe: orquestador · orq2 despachar --una-vez"
	if !strings.Contains(out, want) {
		t.Fatalf("missing %q in:\n%s", want, out)
	}
	// A worker may read it too.
	t.Setenv("ORQ_ROL", "worker-back-s01")
	if out2, errs, code := run(t, "pendientes"); code != 0 || out2 != out {
		t.Fatalf("worker read: %d %s\n%s", code, errs, out2)
	}
}

// H30: the status of an approved slice with nothing launched says so, under orq2 only.
func TestStatusAprobadaFaltaLanzar(t *testing.T) {
	root, _ := copiaS01(t)
	t.Setenv("ORQ_ROOT", root)
	t.Setenv("ORQ_ROL", "")
	lim := filepath.Join(t.TempDir(), "limits.json")
	os.WriteFile(lim, []byte(`{"min_free_gb":{"/":1},"max_workers":4,"scratch":{"paths":["`+t.TempDir()+`"],"quota_gb":1,"critical_free_gb":1},"state_max_age_hours":24,"protected":["tests/aceptacion/"]}`), 0o644)
	t.Setenv("ORQ_LIMITS", lim)
	t.Setenv("ORQ_STATE", t.TempDir())
	conTerminalFalso(t)
	t.Setenv("ORQ_NOMBRE", "orq2")
	out, errs, _ := run(t, "status")
	if !strings.Contains(out, "S-01 «Outreach controlado» aprobada, falta lanzar: T1 lista, sin worker · orq2 despachar --una-vez") {
		t.Fatalf("orq2 status:\n%s%s", out, errs)
	}
	out, _, _ = run(t, "slice", "estado")
	if !strings.Contains(out, "  aprobada, falta lanzar: T1 lista, sin worker") {
		t.Fatalf("orq2 slice estado:\n%s", out)
	}
	t.Setenv("ORQ_NOMBRE", "orq")
	out, _, _ = run(t, "status")
	if strings.Contains(out, "falta lanzar") {
		t.Fatalf("orq status changed:\n%s", out)
	}
	out, _, _ = run(t, "slice", "estado")
	if strings.Contains(out, "falta lanzar") {
		t.Fatalf("orq slice estado changed:\n%s", out)
	}
}

// Under identity orq, pendientes and despachar are unknown commands exactly as in 1.0.0, and
// the usage does not offer them.
func TestPendientesYDespacharNoExistenBajoOrqComo100(t *testing.T) {
	root, _ := copiaS01(t)
	t.Setenv("ORQ_ROOT", root)
	t.Setenv("ORQ_NOMBRE", "orq")
	term := conTerminalFalso(t)
	antes := huella(t, root)
	_, errX, codeX := run(t, "nocomando")
	for _, args := range [][]string{{"pendientes"}, {"despachar", "--una-vez"}, {"despachar"}} {
		out, errs, code := run(t, args...)
		if code != codeX || errs != errX || errs != usage || out != "" {
			t.Fatalf("orq %v differs from an unknown command: %d %q", args, code, errs)
		}
	}
	if strings.Contains(usage, "pendientes") || strings.Contains(usage, "despachar") {
		t.Fatal("orq usage offers the new commands")
	}
	if huella(t, root) != antes || len(term.Llamadas) != 0 {
		t.Fatal("orq wrote state")
	}
	t.Setenv("ORQ_NOMBRE", "orq2")
	if u := uso(); !strings.Contains(u, "orq2 pendientes") || !strings.Contains(u, "orq2 despachar --una-vez") {
		t.Fatalf("orq2 usage:\n%s", u)
	}
}

// despachar needs --una-vez (the resident dispatcher is v2.1) and is refused to a worker.
func TestDespacharPideUnaVezYNoEsDeWorkers(t *testing.T) {
	root, _ := copiaS01(t)
	t.Setenv("ORQ_ROOT", root)
	t.Setenv("ORQ_NOMBRE", "orq2")
	t.Setenv("ORQ_ROL", "")
	term := conTerminalFalso(t)
	antes := huella(t, root)
	if _, errs, code := run(t, "despachar"); code != 1 || !strings.Contains(errs, "--una-vez") {
		t.Fatalf("%d %s", code, errs)
	}
	t.Setenv("ORQ_ROL", "worker-back-s01")
	if _, errs, code := run(t, "despachar", "--una-vez"); code != 1 || !strings.Contains(errs, "does not run as a worker") {
		t.Fatalf("%d %s", code, errs)
	}
	if huella(t, root) != antes || len(term.Llamadas) != 0 {
		t.Fatal("refused despachar wrote state")
	}
}

// L1: pendientes is strictly read-only: it never creates .orq.
func TestPendientesNoCreaOrq(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ORQ_ROOT", root)
	t.Setenv("ORQ_NOMBRE", "orq2")
	conTerminalFalso(t)
	out, errs, code := run(t, "pendientes")
	if code != 0 || !strings.Contains(out, "ninguno") {
		t.Fatalf("%d %s%s", code, out, errs)
	}
	if _, err := os.Stat(filepath.Join(root, ".orq")); err == nil {
		t.Fatal("pendientes created .orq")
	}
}

// L5: the --agente help does not offer codex, which config refuses.
func TestDespacharAyudaNoOfreceCodex(t *testing.T) {
	t.Setenv("ORQ_NOMBRE", "orq2")
	_, errs, _ := run(t, "despachar", "-h")
	if strings.Contains(errs, "codex") || !strings.Contains(errs, "claude") {
		t.Fatalf("%s", errs)
	}
}
