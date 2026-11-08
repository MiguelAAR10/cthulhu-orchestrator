package cli

import (
	"regexp"
	"strings"
	"testing"
)

// marcasES: palabras que delatan una salida en español cuando ORQ_LANG no está puesto.
var marcasES = regexp.MustCompile(`(?i)\b(escribe|exige|el examen|actualizada|reparación|ejecutadas|omitidas|fallidas|IMPLEMENTANDO|BLOQUEADO|VERIFICADO|INTEGRANDO|CERRANDO|paquete|escritor|fallos|conservado|retirado|podado|cuarentena|rechazado|aceptado|nadie|uso|cerrar|cerrado|reintentar|recibo vigente|aceptados|rechazados|sobrantes|historia|pregunta|respuesta)\b|\[estado\]|\bbloqueado:`)

// Evaluador D: los comandos comunes, sin ORQ_LANG, no dejan escapar español.
func TestComandosComunesSinEspanol(t *testing.T) {
	sliceConFake(t, false)
	t.Setenv("ORQ_LANG", "")
	var todo strings.Builder
	paso := func(rol string, args ...string) {
		t.Setenv("ORQ_ROL", rol)
		out, errs, _ := run(t, args...)
		todo.WriteString("$ orq " + strings.Join(args, " ") + "\n" + out + errs)
	}
	paso("", "slice", "take", "w1")
	paso("", "slice", "status")
	paso("", "slice", "verify")
	paso("", "slice", "verify")
	paso("", "slice", "status")
	paso("", "slice", "receipt")
	paso("", "board")
	paso("orquestador", "msg", "send", "--to", "atena", "--type", "status", "half done")
	paso("orquestador", "msg", "send", "--to", "atena", "--type", "question", "which?")
	paso("atena", "msg", "read")
	paso("", "gc", "--dry-run")
	paso("", "metrics")
	paso("", "slice", "close")
	paso("", "--slice", "01", "slice", "close")
	paso("", "slice", "close", "--rejected", "not wanted")
	paso("", "metrics")
	s := todo.String()
	if m := marcasES.FindAllString(s, -1); len(m) > 0 {
		t.Fatalf("español en la salida por defecto: %v\n%s", m, s)
	}
	if strings.Contains(s, "Safety and resources") {
		t.Fatalf("orq --slice 01 slice close imprimió la ayuda:\n%s", s)
	}
}
