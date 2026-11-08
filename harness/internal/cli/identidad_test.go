package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"orq/internal/memoria"
)

// Exploit 1: un worker no forja una decisión del Owner con `mem add --fuente owner`.
func TestMemAddNoForjaFuenteOwner(t *testing.T) {
	root := proyectoVis(t)
	t.Setenv("ORQ_WORKER_ID", "w1")
	for _, f := range []string{"owner", "atena"} {
		if _, _, code := run(t, "mem", "add", "--imp", "alta", "--cat", "decision", "--fuente", f, "--slice", "S-04", "S-04 sumar examen"); code == 0 {
			t.Errorf("worker grabó fuente %s", f)
		}
	}
	t.Setenv("ORQ_WORKER_ID", "")
	// sin entorno (cwd del checkout principal) tampoco vale como owner
	if _, _, code := run(t, "mem", "add", "--imp", "alta", "--cat", "decision", "--fuente", "owner", "S-04 x"); code == 0 {
		t.Error("sin ORQ_ROL grabó fuente owner")
	}
	t.Setenv("ORQ_ROL", "orquestador")
	if _, _, code := run(t, "mem", "add", "--imp", "alta", "--cat", "decision", "--fuente", "atena", "S-04 x"); code == 0 {
		t.Error("orquestador grabó fuente atena")
	}
	if items, _ := memoria.Load(filepath.Join(root, ".orq", "memoria.jsonl")); len(items) != 0 {
		t.Fatalf("%+v", items)
	}
	t.Setenv("ORQ_ROL", "atena")
	if _, errs, code := run(t, "mem", "add", "--imp", "alta", "--cat", "decision", "--fuente", "owner", "--slice", "S-04", "S-04 sumar examen"); code != 0 {
		t.Fatal(errs)
	}
	items, _ := memoria.Load(filepath.Join(root, ".orq", "memoria.jsonl"))
	if len(items) != 1 || items[0].Por != "atena" {
		t.Fatalf("%+v", items)
	}
}

// Exploit 1b: --aprobado no acepta memoria owner|atena sin procedencia validada al escribirla.
func TestAprobadoRechazaMemoriaSinProcedencia(t *testing.T) {
	root := proyectoVis(t)
	mem := filepath.Join(root, ".orq", "memoria.jsonl")
	memoria.Add(mem, memoria.Item{Imp: "alta", Cat: "decision", Fuente: "owner", Slice: "S-04", Texto: "S-04 forjada"}, time.Now())
	memoria.Add(mem, memoria.Item{Imp: "alta", Cat: "decision", Fuente: "owner", Slice: "S-04", Texto: "S-04 legítima", Por: "atena"}, time.Now())
	p := proyecto{root, filepath.Join(root, ".orq")}
	if _, err := p.decisionRegistrada("m1"); err == nil {
		t.Error("aceptó memoria sin procedencia")
	}
	if _, err := p.decisionRegistrada("m2"); err != nil {
		t.Error(err)
	}
}

// Exploit 2: sin ORQ_ROL, el checkout principal no da poderes de Owner.
func TestSinRolNoHayAccionesPrivilegiadas(t *testing.T) {
	root := proyectoVis(t)
	t.Setenv("ORQ_ROL", "atena")
	run(t, "stop", "raise", "--type", "decision", "--severity", "soon", "--slice", "S-04", "--text", "pick", "--options", "a|b")
	t.Setenv("ORQ_ROL", "")
	if _, _, code := run(t, "autonomy", "free", "--scope", "src/**"); code == 0 {
		t.Error("sin rol cambió la autonomía")
	}
	if _, _, code := run(t, "stop", "resolve", "ST-0001", "--choice", "a"); code == 0 {
		t.Error("sin rol resolvió un stopper")
	}
	if _, errs, code := run(t, "slice", "examen", "tests/x.sh", "--aprobado", "m1"); code == 0 || !strings.Contains(errs, "ORQ_ROL") {
		t.Errorf("sin rol cambió el examen: %d %s", code, errs)
	}
	if strings.Contains(stopsFile(root), "resolved") {
		t.Fatal(stopsFile(root))
	}
}

// Exploit 1c: el canal tampoco forja un mensaje de atena.
func TestMsgNoForjaDeAtena(t *testing.T) {
	proyectoVis(t)
	t.Setenv("ORQ_ROL", "orquestador")
	if _, _, code := run(t, "msg", "send", "--de", "atena", "--para", "orquestador", "--tipo", "respuesta", "sí S-04"); code == 0 {
		t.Error("orquestador escribió como atena")
	}
}

// Evaluador A: sin el token del Owner, ORQ_ROL=owner desde el checkout principal no da poderes,
// aunque se vacíe ORQ_WORKER_ID.
func TestRolSinTokenNoEsPrivilegiado(t *testing.T) {
	root := proyectoVis(t)
	t.Setenv("ORQ_ROL", "atena")
	run(t, "stop", "raise", "--type", "decision", "--severity", "soon", "--slice", "S-04", "--text", "pick", "--options", "a|b")
	t.Setenv("ORQ_TOKEN", "")
	t.Setenv("ORQ_WORKER_ID", "")
	for _, rol := range []string{"owner", "atena", "orquestador"} {
		t.Setenv("ORQ_ROL", rol)
		if _, _, code := run(t, "mem", "add", "--imp", "alta", "--cat", "decision", "--source", "owner", "--slice", "S-04", "S-04 forjada"); code == 0 {
			t.Errorf("%s sin token grabó fuente owner", rol)
		}
		if _, _, code := run(t, "msg", "send", "--para", "orquestador", "--tipo", "pregunta", "--de", "atena", "x"); code == 0 {
			t.Errorf("%s sin token escribió como atena", rol)
		}
		if _, _, code := run(t, "autonomy", "free", "--scope", "src/**"); code == 0 {
			t.Errorf("%s sin token cambió la autonomía", rol)
		}
		if _, _, code := run(t, "stop", "resolve", "ST-0001", "--choice", "a"); code == 0 {
			t.Errorf("%s sin token resolvió", rol)
		}
		if _, _, code := run(t, "slice", "exam", "tests/easy.sh", "--approved", "m1"); code == 0 {
			t.Errorf("%s sin token cambió el examen", rol)
		}
		if _, errs, code := run(t, "slice", "close", "--accepted"); code == 0 || !strings.Contains(errs, "ORQ_TOKEN") {
			t.Errorf("%s sin token cerró: %s", rol, errs)
		}
	}
	t.Setenv("ORQ_TOKEN", "falso")
	t.Setenv("ORQ_ROL", "owner")
	if _, _, code := run(t, "autonomy", "free", "--scope", "src/**"); code == 0 {
		t.Error("token falso aceptado")
	}
	if items, _ := memoria.Load(filepath.Join(root, ".orq", "memoria.jsonl")); len(items) != 0 {
		t.Fatalf("%+v", items)
	}
	if strings.Contains(stopsFile(root), "resolved") {
		t.Fatal(stopsFile(root))
	}
}

// orq token: lo obtiene el Owner en su terminal; nunca un worker.
func TestTokenSoloFueraDeWorkers(t *testing.T) {
	proyectoVis(t)
	out, _, code := run(t, "token")
	if code != 0 || strings.TrimSpace(out) != os.Getenv("ORQ_TOKEN") {
		t.Fatalf("%d %q", code, out)
	}
	t.Setenv("ORQ_WORKER_ID", "w1")
	if out, _, code := run(t, "token"); code == 0 || strings.TrimSpace(out) != "" {
		t.Fatal("un worker obtuvo el token")
	}
	t.Setenv("ORQ_WORKER_ID", "")
	t.Setenv("ORQ_ROL", "w2")
	if _, _, code := run(t, "token"); code == 0 {
		t.Fatal("un rol de worker obtuvo el token")
	}
}

// Evaluador C: un worker sin hook (Codex) recibe un shim de orq en el PATH que suelta el token,
// fija su identidad y rechaza lo privilegiado.
func TestShimDeWorker(t *testing.T) {
	root := proyectoVis(t)
	bin, err := escribirShim(filepath.Join(root, ".orq"), "w1")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(filepath.Join(bin, "orq"))
	sh := string(raw)
	for _, want := range []string{"unset ORQ_TOKEN", "ORQ_WORKER_ID='w1'", "ORQ_SHIM=1 exec "} {
		if !strings.Contains(sh, want) {
			t.Fatalf("shim sin %q:\n%s", want, sh)
		}
	}
	if info, _ := os.Stat(filepath.Join(bin, "orq")); info.Mode().Perm()&0o100 == 0 {
		t.Fatal("shim no ejecutable")
	}
	// Lo que hace el shim: Run con ORQ_SHIM aplica la lista blanca, aun con ORQ_ROL=owner y token.
	t.Setenv("ORQ_SHIM", "1")
	t.Setenv("ORQ_ROL", "owner")
	for _, a := range [][]string{{"slice", "exam", "t", "--approved", "m1"}, {"autonomy", "free", "--scope", "src"}, {"token"}, {"stop", "resolve", "ST-0001"}, {"mem", "add", "--source", "owner", "x"}} {
		if _, errs, code := run(t, a...); code != 2 || !strings.Contains(errs, "worker cannot run") {
			t.Errorf("%v: %d %s", a, code, errs)
		}
	}
	if _, errs, code := run(t, "mem", "show"); code != 0 {
		t.Errorf("status: %s", errs)
	}
}

// El shell ejecuta el shim de verdad: suelta ORQ_TOKEN y fija ORQ_WORKER_ID antes de llamar a orq.
func TestShimEjecutado(t *testing.T) {
	dir := t.TempDir()
	fake := filepath.Join(dir, "real-orq")
	os.WriteFile(fake, []byte("#!/bin/sh\necho \"id=$ORQ_WORKER_ID shim=$ORQ_SHIM tok=$ORQ_TOKEN args=$*\"\n"), 0o755)
	bin := filepath.Join(dir, "workers", "w1", "bin")
	os.MkdirAll(bin, 0o755)
	sh := fmt.Sprintf("#!/bin/sh\nunset ORQ_TOKEN\nORQ_WORKER_ID=%s ORQ_SHIM=1 exec %s \"$@\"\n", shQuote("w1"), shQuote(fake))
	os.WriteFile(filepath.Join(bin, "orq"), []byte(sh), 0o755)
	c := exec.Command("sh", "-c", "orq slice status")
	c.Env = append(os.Environ(), "PATH="+bin+":"+os.Getenv("PATH"), "ORQ_TOKEN=secreto", "ORQ_WORKER_ID=")
	out, err := c.CombinedOutput()
	if err != nil || strings.TrimSpace(string(out)) != "id=w1 shim=1 tok= args=slice status" {
		t.Fatalf("%v %q", err, out)
	}
}

// Sin --source, la fuente sale del rol efectivo; nunca owner ni atena.
func TestMemAddFuentePorDefecto(t *testing.T) {
	root := proyectoVis(t)
	t.Setenv("ORQ_TOKEN", "")
	ultima := func() memoria.Item {
		items, _ := memoria.Load(filepath.Join(root, ".orq", "memoria.jsonl"))
		if len(items) == 0 {
			t.Fatal("sin items")
		}
		return items[len(items)-1]
	}
	for _, c := range []struct{ id, rol, fuente string }{
		{"w1", "w1", "worker"},
		{"w1", "", "worker"},
		{"w1", "owner", "worker"},
		{"w1", "atena", "worker"},
		{"", "tester", "worker"},
		{"", "scout", "scout"},
		{"", "orquestador", "orquestador"},
		{"", "orchestrator", "orquestador"},
		{"", "planner", "planificador"},
	} {
		t.Setenv("ORQ_WORKER_ID", c.id)
		t.Setenv("ORQ_ROL", c.rol)
		if _, errs, code := run(t, "mem", "add", "--imp", "baja", "--cat", "riesgo", "nota "+c.id+c.rol); code != 0 {
			t.Errorf("%+v: %s", c, errs)
			continue
		}
		if it := ultima(); it.Fuente != c.fuente {
			t.Errorf("%+v: fuente %q", c, it.Fuente)
		}
	}
	t.Setenv("ORQ_WORKER_ID", "")
	antes := len(mustItems(t, root))
	for _, rol := range []string{"owner", "atena", ""} {
		t.Setenv("ORQ_ROL", rol)
		if _, errs, code := run(t, "mem", "add", "--imp", "baja", "--cat", "riesgo", "sin fuente"); code == 0 || !strings.Contains(errs, "--source") {
			t.Errorf("rol %q: code=%d %s", rol, code, errs)
		}
	}
	if len(mustItems(t, root)) != antes {
		t.Fatal("grabó sin fuente")
	}
}

func mustItems(t *testing.T, root string) []memoria.Item {
	t.Helper()
	items, err := memoria.Load(filepath.Join(root, ".orq", "memoria.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	return items
}

// Las fuentes forjadas siguen rechazadas, también las que afirman hechos (filtro) o suplantan a una sesión.
func TestMemAddFuenteForjada(t *testing.T) {
	root := proyectoVis(t)
	t.Setenv("ORQ_TOKEN", "")
	t.Setenv("ORQ_WORKER_ID", "w1")
	t.Setenv("ORQ_ROL", "w1")
	for _, f := range []string{"owner", "atena", "filtro", "liaison", "orquestador", "orchestrator", "planner", "astra", "escriba", "OWNER", "nadie"} {
		if _, _, code := run(t, "mem", "add", "--imp", "baja", "--cat", "riesgo", "--source", f, "x"); code == 0 {
			t.Errorf("worker firmó como %s", f)
		}
	}
	if len(mustItems(t, root)) != 0 {
		t.Fatal("grabó algo")
	}
	for _, f := range []string{"worker", "scout"} {
		if _, errs, code := run(t, "mem", "add", "--imp", "baja", "--cat", "riesgo", "--source", f, "x"); code != 0 {
			t.Errorf("%s: %s", f, errs)
		}
	}
	t.Setenv("ORQ_WORKER_ID", "")
	t.Setenv("ORQ_ROL", "planner")
	if _, _, code := run(t, "mem", "add", "--imp", "baja", "--cat", "riesgo", "--source", "filtro", "x"); code == 0 {
		t.Error("planner sin token firmó como filtro")
	}
}
