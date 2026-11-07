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
	if code != 0 || !strings.Contains(out, "networkidle") || !strings.Contains(out, "No cargues") {
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
			if !strings.Contains(usage, "orq "+cmd) {
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
