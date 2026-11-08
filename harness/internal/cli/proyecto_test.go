package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"orq/internal/canal"
	"orq/internal/herdr"
	"orq/internal/memoria"
)

func run(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	var out, errb bytes.Buffer
	code := Run(args, strings.NewReader(""), &out, &errb)
	return out.String(), errb.String(), code
}

func TestMensajeSeGuardaYSeEntregaAlRolCorrecto(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ORQ_ROOT", root)
	t.Setenv("ORQ_ROL", "")
	var llamadas [][]string
	orig := herdr.Run
	herdr.Run = func(a ...string) ([]byte, error) { llamadas = append(llamadas, a); return []byte(`{}`), nil }
	defer func() { herdr.Run = orig }()

	os.MkdirAll(filepath.Join(root, ".orq"), 0o755)
	canal.SaveSesiones(filepath.Join(root, ".orq", "sesiones.json"), map[string]canal.Sesion{
		"atena":       {Agente: "atena-x", Pane: "w1:p1"},
		"orquestador": {Agente: "orquestador-x", Pane: "w1:p2"},
	})
	t.Setenv("HERDR_PANE_ID", "w1:p1")

	out, errs, code := run(t, "msg", "send", "--para", "orquestador", "--tipo", "historia", "Como", "Owner", "quiero", "X")
	if code != 0 {
		t.Fatalf("code %d: %s", code, errs)
	}
	if !strings.Contains(out, "orquestador-x") || len(llamadas) != 1 || llamadas[0][2] != "orquestador-x" {
		t.Fatalf("out=%q llamadas=%v", out, llamadas)
	}
	if !strings.Contains(llamadas[0][3], "historia de atena") {
		t.Fatalf("formato: %q", llamadas[0][3])
	}
	out, _, _ = run(t, "msg", "leer")
	if !strings.Contains(out, "atena→orquestador [historia] Como Owner quiero X") {
		t.Fatalf("leer: %q", out)
	}
	if _, _, code := run(t, "msg", "send", "--para", "orquestador", "--tipo", "chisme", "x"); code == 0 {
		t.Fatal("aceptó un tipo fuera de la lista")
	}
}

func TestMemoriaPorLineaDeComandos(t *testing.T) {
	t.Setenv("ORQ_ROOT", t.TempDir())
	t.Setenv("ORQ_ROL", "owner")
	if _, _, code := run(t, "mem", "add", "--imp", "critica", "--cat", "requisito", "--fuente", "owner", "el total incluye envío"); code != 0 {
		t.Fatal("no guardó")
	}
	if _, errs, code := run(t, "mem", "add", "--imp", "muy", "--cat", "requisito", "--fuente", "owner", "x"); code == 0 || !strings.Contains(errs, "critica|alta") {
		t.Fatalf("aceptó etiqueta inválida: %s", errs)
	}
	out, _, _ := run(t, "mem", "ver")
	if !strings.Contains(out, "[critica|requisito] el total incluye envío (owner)") {
		t.Fatalf("ver: %q", out)
	}
}

func TestOrqSkillDaLaGuiaDelRol(t *testing.T) {
	t.Setenv("ORQ_ROOT", t.TempDir())
	out, _, code := run(t, "skill", "worker")
	if code != 0 || !strings.Contains(out, "networkidle") || !strings.Contains(out, "Never load") {
		t.Fatalf("%d %q", code, out)
	}
	if _, _, code := run(t, "--skill", "nadie"); code == 0 {
		t.Fatal("aceptó un rol inexistente")
	}
	// La guía solo nombra comandos de orq que existen.
	for _, rol := range []string{"atena", "orquestador", "planificador", "astra", "scout", "worker", "verificador"} {
		g, _ := guias.ReadFile("guias/" + rol + ".md")
		for _, l := range strings.Split(string(g), "`orq ")[1:] {
			cmd := strings.TrimRight(strings.Fields(l)[0], "`.,;)")
			if !strings.Contains(usageEN+usageES, "orq "+cmd) {
				t.Errorf("%s: la guía usa `orq %s`, que no existe", rol, cmd)
			}
		}
	}
}

// El Orquestador lanza desde otro repo: queda registrado ahí para que el worker le avise directo.
func TestRegistrarLlamador(t *testing.T) {
	ses := map[string]canal.Sesion{"worker-back-s02": {Agente: "worker-back-s02", Pane: "w3D:pX"}}
	registrarLlamador(ses, "orquestador", "w3D:pQ", time.Now())
	if s := ses["orquestador"]; s.Agente != "w3D:pQ" || s.Pane != "w3D:pQ" {
		t.Fatalf("%+v", s)
	}
	propio := map[string]canal.Sesion{"orquestador": {Agente: "orquestador-x-3", Pane: "w3D:pQ", Gen: 3}}
	registrarLlamador(propio, "orquestador", "w3D:pQ", time.Now())
	if propio["orquestador"].Agente != "orquestador-x-3" {
		t.Fatal("en su propio repo no se pisa la sesión")
	}
	registrarLlamador(ses, "worker-front-s04", "w3D:pV", time.Now())
	if _, ok := ses["worker-front-s04"]; ok {
		t.Fatal("un worker no se registra como destino")
	}
}

// H-01: aunque falte el hook, el handler rechaza cambiar el contrato desde un worktree enlazado.
func TestExamenYAlcanceNoDesdeWorktree(t *testing.T) {
	t.Setenv("ORQ_ROOT", t.TempDir())
	wt := t.TempDir()
	os.WriteFile(filepath.Join(wt, ".git"), []byte("gitdir: /x/.git/worktrees/w\n"), 0o644)
	t.Chdir(wt)
	for _, a := range [][]string{{"slice", "examen", "tests/x.sh", "--aprobado", "m1"}, {"slice", "alcance", "src", "--aprobado", "m1"}} {
		if _, errs, code := run(t, a...); code == 0 || !strings.Contains(errs, "worktree") {
			t.Errorf("%v desde un worktree: code %d %q", a, code, errs)
		}
	}
}

// H-01: --aprobado debe nombrar una decisión registrada de Atena u Owner; el texto libre no vale.
func TestAprobadoExigeDecisionRegistrada(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ORQ_ROOT", root)
	t.Setenv("ORQ_ROL", "")
	os.MkdirAll(filepath.Join(root, ".orq"), 0o755)
	memoria.Add(filepath.Join(root, ".orq", "memoria.jsonl"), memoria.Item{Imp: "alta", Cat: "decision", Fuente: "worker", Texto: "yo me apruebo"}, time.Now())
	memoria.Add(filepath.Join(root, ".orq", "memoria.jsonl"), memoria.Item{Imp: "alta", Cat: "decision", Fuente: "owner", Por: "atena", Texto: "sumar examen x"}, time.Now())
	canal.Append(filepath.Join(root, ".orq", "canal.jsonl"), &canal.Msg{De: "orquestador", Para: "atena", Tipo: "pregunta", Texto: "¿apruebo?"})
	canal.Append(filepath.Join(root, ".orq", "canal.jsonl"), &canal.Msg{De: "atena", Para: "orquestador", Tipo: "respuesta", Texto: "sí"})
	p := proyecto{root, filepath.Join(root, ".orq")}
	for _, mal := range []string{"Atena dijo que sí", "req ATN-123", "m1", "c1", "m99", ""} {
		if _, err := p.decisionRegistrada(mal); err == nil {
			t.Errorf("aceptó %q", mal)
		}
	}
	for _, bien := range []string{"m2", "m2: sumar examen x", "c2 de Atena"} {
		if _, err := p.decisionRegistrada(bien); err != nil {
			t.Errorf("rechazó %q: %v", bien, err)
		}
	}
}

// 1B: --aprobado tiene que hablar de este slice y ser posterior a su apertura o al último cambio de contrato.
func TestAprobadoLigadoAlSlice(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, ".orq"), 0o755)
	mem, can := filepath.Join(root, ".orq", "memoria.jsonl"), filepath.Join(root, ".orq", "canal.jsonl")
	desde := time.Date(2026, 11, 8, 10, 0, 0, 0, time.UTC)
	antes, despues := desde.Add(-time.Hour), desde.Add(time.Hour)
	memoria.Add(mem, memoria.Item{Imp: "alta", Cat: "decision", Fuente: "owner", Por: "atena", Texto: "S-07: sumar examen x"}, antes)         // m1 vieja
	memoria.Add(mem, memoria.Item{Imp: "alta", Cat: "decision", Fuente: "owner", Por: "atena", Texto: "sumar examen de S-03"}, despues)       // m2 otro slice
	memoria.Add(mem, memoria.Item{Imp: "alta", Cat: "decision", Fuente: "atena", Por: "atena", Texto: "sumar examen x"}, despues)             // m3 sin slice
	memoria.Add(mem, memoria.Item{Imp: "alta", Cat: "decision", Fuente: "owner", Por: "atena", Texto: "S-07 suma examen x"}, despues)         // m4 bien por texto
	memoria.Add(mem, memoria.Item{Imp: "alta", Cat: "decision", Fuente: "atena", Por: "atena", Slice: "S-07", Texto: "alcance api"}, despues) // m5 bien por campo
	memoria.Add(mem, memoria.Item{Imp: "alta", Cat: "decision", Fuente: "owner", Por: "atena", Texto: "S-070 otra cosa"}, despues)            // m6 id parecido
	canal.Append(can, &canal.Msg{De: "atena", Para: "orquestador", Tipo: "respuesta", Texto: "sí para S-07", TS: antes})        // c1 vieja
	canal.Append(can, &canal.Msg{De: "atena", Para: "orquestador", Tipo: "respuesta", Texto: "sí para S-07", TS: despues})      // c2 bien
	p := proyecto{root, filepath.Join(root, ".orq")}
	for _, mal := range []string{"m1", "m2", "m3", "m6", "c1"} {
		if err := p.decisionDelSlice(mal, "S-07", desde); err == nil {
			t.Errorf("aceptó %q", mal)
		}
	}
	for _, bien := range []string{"m4", "m5 alcance", "c2 de Atena"} {
		if err := p.decisionDelSlice(bien, "S-07", desde); err != nil {
			t.Errorf("rechazó %q: %v", bien, err)
		}
	}
}
