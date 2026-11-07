// Package escriba llama a Haiku para compactar la memoria y escribir relevos.
// Toda salida se valida en código antes de usarse.
package escriba

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"orq/internal/memoria"
)

// Modelo se puede cambiar con ORQ_ESCRIBA_MODELO.
func Modelo() string {
	if m := os.Getenv("ORQ_ESCRIBA_MODELO"); m != "" {
		return m
	}
	return "haiku"
}

// Ask se reemplaza en pruebas.
var Ask = func(prompt string) (string, error) {
	cmd := exec.Command("claude", "-p", "--model", Modelo(), "--output-format", "text", "--no-session-persistence")
	cmd.Stdin = strings.NewReader(prompt)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("escriba (%s): %v", Modelo(), err)
	}
	return string(out), nil
}

const promptCompactar = `Eres el escriba de un proyecto de software. Recibes hechos en JSONL con etiquetas.
Devuelve como máximo %d líneas JSONL, solo JSON, sin texto alrededor ni bloques de código.
Une hechos repetidos, borra los que ya no aplican (una decisión posterior gana), conserva cifras, nombres y rutas exactas.
Campos por línea: imp (alta|media|baja; nunca critica), cat (requisito|decision|dominio|tecnico|riesgo|aprendizaje), texto (≤200 caracteres), slice (opcional).
Hechos:
%s`

func Compactar(rest []memoria.Item, max int) ([]memoria.Item, error) {
	var in bytes.Buffer
	for _, it := range rest {
		line, _ := json.Marshal(struct {
			Imp   string `json:"imp"`
			Cat   string `json:"cat"`
			Texto string `json:"texto"`
			Slice string `json:"slice,omitempty"`
		}{it.Imp, it.Cat, it.Texto, it.Slice})
		in.Write(append(line, '\n'))
	}
	out, err := Ask(fmt.Sprintf(promptCompactar, max, in.String()))
	if err != nil {
		return nil, err
	}
	var res []memoria.Item
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.HasPrefix(line, "{") {
			continue
		}
		var it memoria.Item
		if err := json.Unmarshal([]byte(line), &it); err != nil {
			return nil, fmt.Errorf("el escriba devolvió una línea ilegible: %s", line)
		}
		it.Fuente = "escriba"
		res = append(res, it)
	}
	if len(res) == 0 {
		return nil, fmt.Errorf("el escriba no devolvió líneas; memoria sin cambios")
	}
	return res, nil
}

const promptRelevo = `Eres el escriba. Una sesión nueva del rol %q va a continuar el trabajo y solo leerá lo que escribas.
Escribe como máximo %d líneas en español, en viñetas "- ", sin títulos ni introducción:
qué se está haciendo ahora, qué falta, qué decisiones ya se tomaron y qué no se debe repetir.
No repitas los requisitos críticos: ya van aparte. No inventes nada que no esté abajo.

Memoria:
%s
Estado del slice:
%s
Últimos mensajes:
%s`

// Relevo devuelve solo viñetas, recortadas al máximo pedido.
func Relevo(rol, memo, slice, msgs string, max int) (string, error) {
	out, err := Ask(fmt.Sprintf(promptRelevo, rol, max, memo, slice, msgs))
	if err != nil {
		return "", err
	}
	var lines []string
	for _, l := range strings.Split(out, "\n") {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, "- ") && len(lines) < max {
			lines = append(lines, l)
		}
	}
	if len(lines) == 0 {
		return "", fmt.Errorf("el escriba no devolvió viñetas")
	}
	return strings.Join(lines, "\n"), nil
}

const promptDoc = `Eres el escriba. Escribe la ficha viva de un proyecto de software en español, en Markdown, máximo %d líneas.
Secciones fijas, en este orden: "## Qué es", "## Cómo se corre y se prueba", "## Componentes", "## Vocabulario del dominio", "## Decisiones vigentes", "## Slices cerrados".
Solo datos presentes abajo; si una sección no tiene datos escribe "- (sin datos)". Frases cortas. Sin introducción.

Perfil: %s
Archivos versionados (muestra): %s
Memoria vigente:
%s
Slices cerrados:
%s`

// Doc escribe la ficha del proyecto; rechaza salidas sin las secciones o demasiado largas.
func Doc(perfil, archivos, memo, slices string, max int) (string, error) {
	out, err := Ask(fmt.Sprintf(promptDoc, max, perfil, archivos, memo, slices))
	if err != nil {
		return "", err
	}
	out = strings.TrimSpace(strings.TrimPrefix(strings.TrimSuffix(strings.TrimSpace(out), "```"), "```markdown"))
	for _, h := range []string{"## Qué es", "## Decisiones vigentes"} {
		if !strings.Contains(out, h) {
			return "", fmt.Errorf("la ficha no trae la sección %q; se conserva la anterior", h)
		}
	}
	if n := strings.Count(out, "\n") + 1; n > max+5 {
		return "", fmt.Errorf("la ficha tiene %d líneas y el máximo es %d; se conserva la anterior", n, max)
	}
	return out + "\n", nil
}
