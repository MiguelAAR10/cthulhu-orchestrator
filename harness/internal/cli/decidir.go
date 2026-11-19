package cli

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"os"
	"strings"
	"time"

	"orq/internal/app"
	"orq/internal/slice"
)

// seccion es un bloque de la página de decisión: qué muestra, de dónde sale y sus líneas.
type seccion struct {
	Titulo, Fuente string
	Lineas         []linea
}

// linea con Alerta pide la atención del Owner: fuera de alcance, examen cambiado, sin conteo.
type linea struct {
	Texto  string
	Alerta bool
}

func normal(f string, a ...any) linea { return linea{Texto: fmt.Sprintf(f, a...)} }
func alerta(f string, a ...any) linea { return linea{Texto: fmt.Sprintf(f, a...), Alerta: true} }

// edad escribe una duración como la lee una persona: 2h05m, 15m, 3d4h.
func edad(d time.Duration) string {
	d = d.Round(time.Minute)
	switch {
	case d >= 48*time.Hour:
		return fmt.Sprintf("%dd%dh", int(d.Hours())/24, int(d.Hours())%24)
	case d >= time.Hour:
		return fmt.Sprintf("%dh%02dm", int(d.Hours()), int(d.Minutes())%60)
	}
	return fmt.Sprintf("%dm", int(d.Minutes()))
}

// maxDiffDefiniciones acota cuántas líneas del diff de definiciones muestra la página.
const maxDiffDefiniciones = 80

func cabeceraDecision(pd *app.PaginaDecision, now time.Time) string {
	e := pd.Estado
	return fmt.Sprintf("%s «%s» · %s hace %s · decide el Owner", e.Attempt(), e.Titulo, e.Fase, edad(now.Sub(e.Recibo.TS)))
}

func seccionesDecision(pd *app.PaginaDecision) []seccion {
	e, rec := pd.Estado, pd.Estado.Recibo
	var out []seccion

	historia := seccion{Titulo: "Historia", Fuente: pd.FuenteHistoria}
	for _, l := range pd.Historia {
		historia.Lineas = append(historia.Lineas, normal("%s", l))
	}
	for _, av := range pd.AvisosHistoria {
		historia.Lineas = append(historia.Lineas, alerta("%s", av))
	}
	out = append(out, historia)

	pruebas := seccion{Titulo: "Pruebas", Fuente: "recibo, sin conteo del runner"}
	if rec.Contado {
		pruebas.Fuente = "recibo, conteo del runner " + e.Formato
		pruebas.Lineas = append(pruebas.Lineas, normal("%d ejecutadas, %d omitidas, %d fallidas · %s", rec.Ejecutados, rec.Omitidos, rec.Fallidos, rec.Resultado))
	} else {
		pruebas.Lineas = append(pruebas.Lineas, alerta("sin conteo: no se sabe cuántas pruebas corrieron · %s", rec.Resultado))
	}
	out = append(out, pruebas)

	checks := seccion{Titulo: "Checks", Fuente: "recibo · checks de .orq/config.json"}
	for _, c := range rec.Checks {
		switch c.Resultado {
		case slice.CheckPaso:
			checks.Lineas = append(checks.Lineas, normal("%s paso · exit 0 · %s", c.Nombre, c.Cmd))
		case slice.CheckOmitido:
			checks.Lineas = append(checks.Lineas, alerta("%s omitido: %s (no cuenta como pasado)", c.Nombre, c.Motivo))
		default:
			primera, _, _ := strings.Cut(c.Salida, "\n")
			if c.Exit == 127 {
				checks.Lineas = append(checks.Lineas, alerta("%s infraestructura · exit 127 · %s · la caja no ve el comando: %s", c.Nombre, c.Cmd, primera))
				continue
			}
			checks.Lineas = append(checks.Lineas, alerta("%s fallo · exit %d · %s · %s", c.Nombre, c.Exit, c.Cmd, primera))
		}
		for _, p := range c.Capturas {
			checks.Lineas = append(checks.Lineas, normal("  captura: %s", p))
		}
	}
	if len(rec.Checks) == 0 {
		checks.Lineas = append(checks.Lineas, alerta("sin checks en el recibo: corre orq init y vuelve a verificar"))
	}
	if len(rec.DefinicionesChecks) > 0 {
		checks.Lineas = append(checks.Lineas, alerta("check definitions changed: %s · el slice cambió lo que corre un check; aceptar pide --acepto-checks", strings.Join(rec.DefinicionesChecks, ", ")))
		lineas := strings.Split(pd.DiffDefiniciones, "\n")
		if len(lineas) > maxDiffDefiniciones {
			lineas = append(lineas[:maxDiffDefiniciones], fmt.Sprintf("… diff recortado: git diff %s %s -- %s", slice.Corto(e.SHARojo), slice.Corto(rec.Head), strings.Join(rec.DefinicionesChecks, " ")))
		}
		for _, l := range lineas {
			checks.Lineas = append(checks.Lineas, alerta("%s", l))
		}
	}
	out = append(out, checks)

	archivos := seccion{Titulo: "Archivos", Fuente: "alcance de orq slice abrir y zonas, contra git diff del rojo a la punta"}
	if plan := pd.Planeado(); len(plan) > 0 {
		archivos.Lineas = append(archivos.Lineas, normal("planeado: %s", strings.Join(plan, ", ")))
	} else {
		archivos.Lineas = append(archivos.Lineas, alerta("sin alcance declarado: nada que marcar fuera"))
	}
	fuera := map[string]bool{}
	for _, f := range pd.Alineacion.Fuera {
		fuera[f] = true
	}
	for _, f := range pd.Alineacion.Cambiados {
		if fuera[f] {
			archivos.Lineas = append(archivos.Lineas, alerta("%s fuera del alcance", f))
		} else {
			archivos.Lineas = append(archivos.Lineas, normal("tocado: %s", f))
		}
	}
	if len(pd.Alineacion.Cambiados) > slice.MaxArchivos {
		archivos.Lineas = append(archivos.Lineas, alerta("%d archivos cambiados: ya no es un slice delgado", len(pd.Alineacion.Cambiados)))
	}
	for _, f := range rec.TestsBorrados {
		archivos.Lineas = append(archivos.Lineas, alerta("test borrado por el slice: %s", f))
	}
	for _, av := range pd.Avisos {
		archivos.Lineas = append(archivos.Lineas, alerta("%s", av))
	}
	out = append(out, archivos)

	examen := seccion{Titulo: "Examen", Fuente: "diario .orq/diario.jsonl y estado del slice"}
	for _, ev := range pd.Examen {
		var fs []string
		if xs, ok := ev.Datos["archivos"].([]any); ok {
			for _, x := range xs {
				fs = append(fs, fmt.Sprint(x))
			}
		}
		examen.Lineas = append(examen.Lineas, alerta("examen cambiado tras abrir (%s): %s · aprobado: %s", ev.TS.Format("2006-01-02 15:04"), strings.Join(fs, ", "), ev.Detalle))
	}
	if len(pd.Examen) == 0 && e.SHAExamen != "" {
		examen.Lineas = append(examen.Lineas, alerta("examen cambiado tras abrir: su base pasó a %s", slice.Corto(e.SHAExamen)))
	}
	if pd.RunnerAceptado() {
		examen.Lineas = append(examen.Lineas, alerta("%s", rec.Nota))
	}
	if len(examen.Lineas) == 0 {
		examen.Lineas = append(examen.Lineas, normal("sin cambios desde que se abrió: %s", strings.Join(e.Bundle, ", ")))
	}
	out = append(out, examen)

	recibo := seccion{Titulo: "Recibo", Fuente: "refs/orq/recibo/" + e.NN, Lineas: []linea{
		normal("cmd: %s · exit %d", rec.Cmd, rec.Exit),
		normal("commit %s · %s", slice.Corto(rec.Head), rec.TS.Format("2006-01-02 15:04")),
	}}
	if rec.Artefacto != "" {
		recibo.Lineas = append(recibo.Lineas, normal("salida completa: %s", rec.Artefacto))
	}
	if rec.Nota != "" && !pd.RunnerAceptado() {
		recibo.Lineas = append(recibo.Lineas, normal("nota: %s", rec.Nota))
	}
	out = append(out, recibo)

	demo := seccion{Titulo: "Cómo verlo funcionar", Fuente: "orq slice abrir --demo"}
	for i, paso := range e.Demo {
		demo.Lineas = append(demo.Lineas, normal("%d. %s", i+1, paso))
	}
	if len(e.Demo) == 0 {
		demo.Lineas = append(demo.Lineas, alerta("sin demo registrada"))
	}
	out = append(out, demo)

	aceptar := normal("aceptar:  orq slice cerrar --slice %s --aceptado", e.NN)
	if len(rec.DefinicionesChecks) > 0 {
		aceptar = alerta("aceptar:  orq slice cerrar --slice %s --aceptado --acepto-checks (tras leer el diff de Checks)", e.NN)
	}
	out = append(out, seccion{Titulo: "Decide", Fuente: "orq slice cerrar", Lineas: []linea{
		aceptar,
		normal(`rechazar: orq slice cerrar --slice %s --rechazado "motivo"`, e.NN),
	}})
	return out
}

func textoDecision(w io.Writer, cabecera string, ss []seccion) {
	fmt.Fprintln(w, cabecera)
	for _, s := range ss {
		fmt.Fprintf(w, "\n== %s (%s)\n", s.Titulo, s.Fuente)
		for _, l := range s.Lineas {
			marca := "  "
			if l.Alerta {
				marca = "!! "
			}
			fmt.Fprintln(w, marca+l.Texto)
		}
	}
}

var plantillaDecision = template.Must(template.New("decision").Parse(`<!doctype html>
<html lang="es"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Cabecera}}</title>
<style>
body{font:15px/1.5 system-ui,sans-serif;max-width:52rem;margin:2rem auto;padding:0 1rem;color:#1b1b1b;background:#fff}
h1{font-size:1.15rem}h2{font-size:1rem;margin:1.4rem 0 .3rem}h2 small{font-weight:normal;color:#666}
ul{list-style:none;padding:0;margin:0}li{font-family:ui-monospace,monospace;padding:.1rem .4rem}
li.alerta{background:#fde8e8;color:#8a1111;font-weight:bold}
@media (prefers-color-scheme:dark){body{color:#e6e6e6;background:#141414}h2 small{color:#999}li.alerta{background:#4a1414;color:#ffd6d6}}
</style></head><body>
<h1>{{.Cabecera}}</h1>
{{range .Secciones}}<h2>{{.Titulo}} <small>({{.Fuente}})</small></h2>
<ul>{{range .Lineas}}<li{{if .Alerta}} class="alerta"{{end}}>{{.Texto}}</li>{{end}}</ul>
{{end}}</body></html>
`))

func htmlDecision(cabecera string, ss []seccion) ([]byte, error) {
	var b bytes.Buffer
	err := plantillaDecision.Execute(&b, struct {
		Cabecera  string
		Secciones []seccion
	}{cabecera, ss})
	return b.Bytes(), err
}

// runDecidir imprime la página de decisión de un slice verificado y, con --html, la escribe en un archivo.
func runDecidir(p proyecto, nn string, args []string, stdout, stderr io.Writer) int {
	html := ""
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--html" && i+1 < len(args):
			html = args[i+1]
			i++
		case nn == "" && !strings.HasPrefix(args[i], "-"):
			nn = strings.TrimPrefix(args[i], "S-")
		default:
			fmt.Fprintln(stderr, "uso: orq slice decidir [NN] [--html RUTA]")
			return 1
		}
	}
	pd, err := p.PaginaDecision(nn)
	if err != nil {
		fmt.Fprintln(stderr, "orq slice:", err)
		return 1
	}
	now := time.Now()
	cab, ss := cabeceraDecision(pd, now), seccionesDecision(pd)
	textoDecision(stdout, cab, ss)
	if html == "" {
		return 0
	}
	raw, err := htmlDecision(cab, ss)
	if err == nil {
		err = os.WriteFile(html, raw, 0o644)
	}
	if err != nil {
		fmt.Fprintln(stderr, "orq slice:", err)
		return 1
	}
	fmt.Fprintf(stdout, "\npágina escrita en %s\n", html)
	return 0
}
