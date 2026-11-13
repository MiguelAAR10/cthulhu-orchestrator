package orca

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"orq/internal/terminal"
)

var _ terminal.Driver = (*Driver)(nil)

// orcaFalsa escribe un `orca` de mentira que anota cada llamada y responde con el sobre JSON de la CLI real.
func orcaFalsa(t *testing.T, guion string) (registro string) {
	t.Helper()
	dir := t.TempDir()
	registro = filepath.Join(dir, "llamadas.log")
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> " + registro + "\n" + guion + "\n"
	bin := filepath.Join(dir, "orca")
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ORQ_ORCA_BIN", bin)
	return registro
}

func llamadas(t *testing.T, registro string) []string {
	t.Helper()
	raw, _ := os.ReadFile(registro)
	return strings.Split(strings.TrimSpace(string(raw)), "\n")
}

const guionSano = `case "$1 $2" in
  "terminal create") echo '{"id":"x","ok":true,"result":{"terminal":{"handle":"term_nuevo","title":"S-01 · p"}}}' ;;
  "terminal split")  echo '{"id":"x","ok":true,"result":{"terminal":{"handle":"term_lado"}}}' ;;
  "terminal list")   echo '{"id":"x","ok":true,"result":{"terminals":[{"handle":"term_a","title":"otro"},{"handle":"term_nuevo","title":"worker-x-s01"}]}}' ;;
  "terminal show")   echo '{"id":"x","ok":true,"result":{"terminal":{"handle":"term_yo"}}}' ;;
  "terminal close")  echo '{"id":"x","ok":false,"error":{"code":"terminal_not_found","message":"no such terminal"}}'; exit 1 ;;
  *) echo '{"id":"x","ok":true,"result":{}}' ;;
esac`

func TestContraUnaOrcaDeMentira(t *testing.T) {
	reg := orcaFalsa(t, guionSano)
	d, ctx := Nuevo(), context.Background()
	if !d.Dentro() || d.PanelActual() != "term_yo" {
		t.Fatalf("panel actual = %q", d.PanelActual())
	}
	p, err := d.CrearPanel(ctx, terminal.PanelNuevo{Etiqueta: "S-01 · p", Cwd: "/repo/w"})
	if err != nil || p.ID != "term_nuevo" {
		t.Fatalf("%+v %v", p, err)
	}
	lado, err := d.CrearPanel(ctx, terminal.PanelNuevo{Junto: p.ID, Cwd: "/repo/w2"})
	if err != nil || lado.ID != "term_lado" || lado.Pestana != p.ID {
		t.Fatalf("%+v %v", lado, err)
	}
	err = d.Arrancar(ctx, terminal.Arranque{Nombre: "worker-x-s01", Kind: "claude", Panel: p.ID, Cwd: "/repo/w",
		Args: []string{"--model", "m", "-n", "con espacio"}, Env: map[string]string{"ORQ_ROL": "worker-x-s01", "PATH": "/s:/usr/bin"}})
	if err != nil {
		t.Fatal(err)
	}
	if v, err := d.Estado(ctx, "worker-x-s01"); v != terminal.Vivo || err != nil {
		t.Fatalf("%v %v", v, err)
	}
	if v, err := d.Estado(ctx, "nadie"); v != terminal.Ausente || err != nil {
		t.Fatalf("%v %v", v, err)
	}
	if err := d.Avisar(ctx, "worker-x-s01", "[orq] 1 mensaje nuevo"); err != nil {
		t.Fatal(err)
	}
	if err := d.Cerrar(ctx, "term_ya_no"); err != nil {
		t.Fatalf("cerrar una terminal que no existe debe ser un no-op: %v", err)
	}
	todo := strings.Join(llamadas(t, reg), "\n")
	for _, want := range []string{
		"terminal create --worktree path:/repo/w --title S-01 · p --json",
		"terminal split --terminal term_nuevo --direction horizontal --json",
		"terminal rename --terminal term_nuevo --title worker-x-s01 --json",
		"terminal send --terminal term_nuevo --text cd /repo/w && env ORQ_ROL=worker-x-s01 PATH=/s:/usr/bin claude --model m -n 'con espacio' --enter --json",
		"terminal wait --terminal term_nuevo --for tui-idle --timeout-ms 90000 --json",
		"terminal send --terminal term_nuevo --text [orq] 1 mensaje nuevo --enter --json",
	} {
		if !strings.Contains(todo, want) {
			t.Errorf("falta la llamada %q en:\n%s", want, todo)
		}
	}
}

func TestLoAmbiguoEsDesconocidoYArrancarSeguroNoRelanza(t *testing.T) {
	// El envío responde con un id de reintento: Orca no sabe si llegó. La terminal no aparece por título.
	reg := orcaFalsa(t, `case "$1 $2" in
  "terminal send") echo '{"id":"x","ok":false,"error":{"code":"transport_failed","message":"reissue with --retry-request req_1"}}'; exit 1 ;;
  "terminal list") echo '{"id":"x","ok":true,"result":{"terminals":[]}}' ;;
  *) echo '{"id":"x","ok":true,"result":{}}' ;;
esac`)
	d := Nuevo()
	_, err := terminal.ArrancarSeguro(context.Background(), d, terminal.Arranque{Nombre: "w", Kind: "codex", Panel: "term_1"})
	if !errors.Is(err, terminal.ErrDesconocido) {
		t.Fatalf("un resultado ambiguo debe ser desconocido: %v", err)
	}
	envios := 0
	for _, l := range llamadas(t, reg) {
		if strings.HasPrefix(l, "terminal send") {
			envios++
		}
	}
	if envios != 1 {
		t.Fatalf("se envió %d veces; lo ambiguo no se reintenta", envios)
	}
}

func TestErroresClarosYRespuestasRaras(t *testing.T) {
	orcaFalsa(t, `case "$1 $2" in
  "terminal create") echo '{"id":"x","ok":true,"result":{"algo":"sin handle"}}' ;;
  "terminal list") echo 'no es json' ;;
  *) echo '{"id":"local","ok":false,"error":{"code":"runtime_unavailable","message":"Start the Orca app first."}}'; exit 1 ;;
esac`)
	d, ctx := Nuevo(), context.Background()
	if _, err := d.CrearPanel(ctx, terminal.PanelNuevo{Etiqueta: "x", Cwd: "/r"}); err == nil || !strings.Contains(err.Error(), "handle") {
		t.Fatalf("aceptó una respuesta sin handle: %v", err)
	}
	if v, err := d.Estado(ctx, "w"); v != terminal.Desconocida || err == nil {
		t.Fatalf("una lista ilegible no es «ausente»: %v %v", v, err)
	}
	err := d.Avisar(ctx, "term_1", "hola")
	if err == nil || errors.Is(err, terminal.ErrDesconocido) || !strings.Contains(err.Error(), "runtime_unavailable") {
		t.Fatalf("un fallo claro se devuelve tal cual: %v", err)
	}
	if d.Dentro() {
		t.Fatal("sin Orca corriendo no se está dentro")
	}
}

func TestOrdenEscapaArgumentos(t *testing.T) {
	got := Orden(terminal.Arranque{Kind: "claude", Args: []string{"-n", "it's", "$(rm -rf x)"}, Env: map[string]string{"A": "b c"}})
	want := `env 'A=b c' claude -n 'it'\''s' '$(rm -rf x)'`
	if got != want {
		t.Fatalf("\n got %s\nwant %s", got, want)
	}
}
