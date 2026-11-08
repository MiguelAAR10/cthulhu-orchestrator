package cli

import (
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
