package slice

// Abrir, verificar y las transiciones que cambian el contrato.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"orq/internal/config"
	"orq/internal/diario"
	"orq/internal/gitx"
	"orq/internal/orqdir"
)

// save escribe el estado. Si cambia la fase o el intento, antes lo anota en el diario:
// así se sabe qué pasó aunque el proceso muera entre un paso y el siguiente.
func (r Repo) save(e *Estado) error {
	if err := os.MkdirAll(r.dirSlices(), 0o755); err != nil {
		return err
	}
	prev, err := leerEstado(r.estadoPath(e.NN))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		prev = nil // un estado ilegible se reemplaza; la transición se anota igual
	}
	if prev == nil || prev.Fase != e.Fase || prev.Intento != e.Intento {
		ev := diario.Evento{Tipo: "fase", Slice: e.NN, Intento: e.Intento, Ruta: e.RutaEfectiva(), A: e.Fase, Actor: e.Escritor, Detalle: e.Motivo}
		if prev != nil {
			ev.De = prev.Fase
		}
		if err := diario.Anotar(diario.Ruta(r.OrqDir), ev); err != nil {
			return fmt.Errorf("no se pudo anotar la transición de %s en el diario: %w", e.ID(), err)
		}
	}
	raw, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return err
	}
	return orqdir.WriteAtomic(r.estadoPath(e.NN), raw)
}

// Orden de los candados, siempre el mismo: abrir → slice-NN → integracion. Los candados de archivo
// (diario, canal, memoria) van después de todos y se sueltan dentro de la función que los toma.
func rangoCandado(nombre string) int {
	switch {
	case nombre == "abrir":
		return 1
	case strings.HasPrefix(nombre, "slice-"):
		return 2
	case nombre == "integracion":
		return 3
	}
	return 9
}

// candado toma un flock exclusivo sobre .orq/<nombre>.lock hasta que se llame a la función devuelta.
func (r Repo) candado(nombre string) (func(), error) {
	f, err := os.OpenFile(filepath.Join(r.OrqDir, nombre+".lock"), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, err
	}
	return func() { syscall.Flock(int(f.Fd()), syscall.LOCK_UN); f.Close() }, nil
}

// candadoTras toma un candado teniendo ya otro, y se niega si eso rompe el orden.
func (r Repo) candadoTras(tenido, nombre string) (func(), error) {
	if rangoCandado(nombre) <= rangoCandado(tenido) {
		return nil, fmt.Errorf("orden de candados: no se toma %q teniendo %q", nombre, tenido)
	}
	return r.candado(nombre)
}

// examen corre el comando del slice sobre una copia limpia del commit sha, dentro de la caja y sin
// red, y cuenta las pruebas si hay reporte. intacto dice si el paquete del examen quedó como en el rojo.
func (r Repo) examen(e *Estado, sha string) (out string, code int, ok bool, cnt *Conteo, intacto bool, err error) {
	dir, limpiar, err := r.copiaDe(sha)
	if err != nil {
		return "", -1, false, nil, false, err
	}
	defer limpiar()
	// El reporte anterior se borra dentro de la caja: la ruta la eligió quien escribió el examen.
	cmd := config.SinOnlyEn(dir, e.Cmd)
	if e.Reporte != "" {
		cmd = "rm -f -- " + comillaSh(e.Reporte) + " 2>/dev/null\n" + cmd
	}
	out, code, ok, err = r.correrEn(dir, r.conServidor(cmd), e.limite())
	if err != nil {
		return out, code, false, nil, false, err
	}
	h, errH := huellaSegura(dir, e.Bundle, e.Cmd)
	intacto = errH == nil && h == e.Huella
	if e.Reporte == "" {
		return out, code, ok, nil, intacto, nil
	}
	raw, errR := leerSeguro(dir, e.Reporte, MaxReporte)
	if errR != nil {
		return out, code, ok, &Conteo{}, intacto, nil // sin reporte legible: 0 pruebas
	}
	c, err := Contar(e.Formato, raw)
	return out, code, ok, &c, intacto, err
}

func comillaSh(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

// sanoEn corre el comando de base sana sobre una copia limpia de sha, en la caja.
func (r Repo) sanoEn(sha, cmd string) (string, bool, error) {
	dir, limpiar, err := r.copiaDe(sha)
	if err != nil {
		return "", false, err
	}
	defer limpiar()
	out, _, ok, err := r.correrEn(dir, r.conServidor(cmd), TimeoutDefecto)
	return out, ok, err
}

// conServidor arranca delante de cmd el servidor de la app de checks.e2e, si lo hay. Va con todo examen
// y toda base sana, no solo los que nombran playwright: `npm run test:e2e` también lo necesita.
func (r Repo) conServidor(cmd string) string {
	if e2e, ok := r.e2e(); ok {
		return e2e.ConServidor(cmd)
	}
	return cmd
}

// Abrir descubre antes de crear, exige base sana y aceptación roja (las dos en la caja, sobre una
// copia limpia del commit), guarda el contrato en git y deja al worker un clon independiente.
func (r Repo) Abrir(nn, titulo, test, cmd string, o Opciones, now time.Time) (*Estado, string, error) {
	soltar, err := r.candado("abrir")
	if err != nil {
		return nil, "", err
	}
	defer soltar()
	ab, err := r.Abiertos()
	if err != nil {
		return nil, "", err
	}
	if len(ab) >= r.max() {
		return nil, "", fmt.Errorf("ya hay %d slice(s) abierto(s) y el tope es %d. Cierra uno antes", len(ab), r.max())
	}
	for _, e := range ab {
		if e.NN == nn {
			return nil, "", fmt.Errorf("%s ya está abierto", e.ID())
		}
	}
	bundle := append([]string{test}, o.Bundle...)
	for _, f := range bundle {
		if !existe(filepath.Join(r.Root, f)) {
			return nil, "", fmt.Errorf("no existe %s", f)
		}
		if _, err := r.git("ls-files", "--error-unmatch", "--", f); err != nil {
			return nil, "", fmt.Errorf("%s no está versionado (¿ignorado por git?): el paquete del examen tiene que estar en el commit", f)
		}
		if out, err := gitDueno(r.Root, "status", "--porcelain", "--", f); err != nil {
			return nil, "", err
		} else if out != "" {
			return nil, "", fmt.Errorf("%s no está commiteado; haz commit del rojo antes de abrir", f)
		}
	}
	rama := "slice/" + nn
	wt := filepath.Join(r.Worktree, orqdir.Slug(r.Root)+"-s"+nn)
	// Paso A: descubrir. Si algo ya ocupa el nombre, no se crea otro encima.
	if out, err := r.git("branch", "--list", rama); err != nil {
		return nil, "", err
	} else if out != "" {
		return nil, "", fmt.Errorf("la rama %s ya existe (¿intento anterior?). Revísala con `orq doctor`; no se crea otra", rama)
	}
	if existe(wt) {
		return nil, "", fmt.Errorf("la carpeta %s ya existe. Revísala con `orq doctor`; no se crea otra", wt)
	}
	base, err := r.git("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil || base == "HEAD" {
		return nil, "", errors.New("el checkout principal no está en una rama")
	}
	if (o.Reporte == "") != (o.Formato == "") {
		return nil, "", errors.New("--reporte y --formato van juntos")
	}
	if o.Formato != "" && !slices.Contains(Formatos, o.Formato) {
		return nil, "", fmt.Errorf("formato %q: usa %s", o.Formato, strings.Join(Formatos, "|"))
	}
	o.SinConteo, o.EnSitio = strings.TrimSpace(o.SinConteo), strings.TrimSpace(o.EnSitio)
	switch {
	case o.EnSitio != "":
		return nil, "", errors.New("--en-sitio ya no existe: el examen corre siempre en la caja, sobre una copia limpia del commit y sin red. Si necesita un archivo que git ignora, versiónalo o que el propio examen lo genere")
	case o.Reporte == "" && o.SinConteo == "":
		return nil, "", errors.New("contar es lo normal: abre con --reporte ARCHIVO --formato tap|junit|gotest-json para saber cuántas pruebas corrieron (verde sin conteo puede ser «no corrió nada»). Si el runner de verdad no puede dejar un reporte, abre con --sin-conteo \"motivo\"")
	case o.Reporte != "" && o.SinConteo != "":
		return nil, "", errors.New("--sin-conteo va sin --reporte: o se cuenta o se explica por qué no")
	case o.Ruta != "" && o.Ruta != RutaFix && o.Ruta != RutaSlice:
		return nil, "", fmt.Errorf("ruta %q: usa %s|%s", o.Ruta, RutaFix, RutaSlice)
	}
	sha, err := r.git("rev-parse", "HEAD")
	if err != nil {
		return nil, "", err
	}
	// Paso B: base sana separada de aceptación roja. Las dos sobre el commit, no sobre la carpeta del dueño.
	if o.Sano != "" {
		out, ok, err := r.sanoEn(sha, o.Sano)
		if err != nil {
			return nil, out, err
		}
		if !ok {
			return nil, out, errors.New("la base no está sana: el comando --sano falla antes de empezar. Arregla la base primero")
		}
	}
	huella, err := HuellaEnCommit(r.Root, sha, bundle, cmd)
	if err != nil {
		return nil, "", err
	}
	prov := &Estado{Cmd: cmd, Reporte: o.Reporte, Formato: o.Formato, TimeoutSeg: o.Timeout, Bundle: bundle, Huella: huella}
	out, codeRojo, ok, cnt, _, err := r.examen(prov, sha)
	if err != nil {
		return nil, out, err
	}
	if codeRojo == 126 || codeRojo == 127 {
		return nil, out, fmt.Errorf("el examen sale con %d: falta una herramienta (¿%s?), no es un rojo. Un rojo válido es una prueba que corre y falla; mira la línea «not found» de la salida y `orq check --caja`", codeRojo, cmd)
	}
	if ok {
		return nil, out, errors.New("el examen ya pasa: no está en rojo, así que no mide nada todavía")
	}
	if cnt != nil && cnt.Fallidos == 0 {
		return nil, out, errors.New("el examen falla, pero no por aserción (0 pruebas fallidas en el reporte: ¿no compila o no arrancó?). Un rojo válido es una prueba que corre y falla")
	}
	host, _ := os.Hostname()
	e := &Estado{Esquema: EsquemaActual, NN: nn, Intento: 1, Fase: Implementando, Titulo: titulo, Test: test, Bundle: bundle, Cmd: cmd,
		Sano: o.Sano, Alcance: o.Alcance, Demo: o.Demo, Base: base, SHARojo: sha, Rama: rama, Worktree: wt, Procedencia: "clon", Host: host,
		Abierto: now.UTC().Truncate(time.Second), Huella: huella, SalidaRojo: cola(out, 20),
		Reporte: o.Reporte, Formato: o.Formato, Omitir: o.PermiteOmitir, TimeoutSeg: o.Timeout, Tipo: o.Tipo,
		SinConteo: o.SinConteo, Vigilados: r.vigiladosEn(sha, cmd, bundle), PruebasExamen: pruebasDelRojo(cnt)}
	if o.Ruta == RutaFix {
		e.Ruta = RutaFix
	}
	// La rama del slice es una referencia del repo principal que solo mueve orq; el worker recibe un clon de ella.
	if _, err := r.git("update-ref", "refs/heads/"+rama, sha, ""); err != nil {
		return nil, out, err
	}
	deshacer := func(causa error) error {
		_, errRef := r.git("update-ref", "-d", "refs/heads/"+rama, sha)
		return errors.Join(causa, errRef, os.RemoveAll(wt), r.borrarRef(refContrato(nn)))
	}
	if err := r.crearClon(e, wt, rama); err != nil {
		return nil, out, deshacer(err)
	}
	if err := r.sellar(e); err != nil {
		return nil, out, deshacer(err)
	}
	if err := r.save(e); err != nil {
		return nil, out, deshacer(err)
	}
	return e, out, nil
}

// gitDueno lee el estado de la carpeta del dueño (status): es la única lectura que mira sus archivos.
func gitDueno(root string, args ...string) (string, error) {
	return gitx.Dueno(context.Background(), root, args...)
}

// Ampliar suma rutas al alcance con una aprobación ya dada (Atena u Owner); el examen nunca entra.
func (r Repo) Ampliar(nn string, rutas []string) (*Estado, error) {
	soltar, err := r.bloqueo(nn)
	if err != nil {
		return nil, err
	}
	defer soltar()
	e, err := r.Load(nn)
	if err != nil || e == nil {
		return nil, errors.Join(err, errors.New("no hay slice abierto"))
	}
	if err := r.confiable(e); err != nil {
		return e, err
	}
	// Sin alcance el slice ya puede tocar todo lo no protegido: sumar rutas lo encogería a solo esas,
	// salvo que la ruta sea protegida, que entonces se autoriza de forma expresa.
	libre := len(e.Alcance) == 0
	for _, b := range rutas {
		for _, f := range e.Bundle {
			if dentro(b, f) {
				return e, fmt.Errorf("la ruta %s incluye el examen %s, que nadie edita", b, f)
			}
		}
		protegida := false
		for _, p := range r.protegidos() {
			protegida = protegida || coincide(b, p) || dentro(b, strings.TrimSuffix(p, "/"))
		}
		if (!libre || protegida) && !slices.Contains(e.Alcance, b) {
			e.Alcance = append(e.Alcance, b)
		}
	}
	if err := r.sellar(e); err != nil {
		return e, err
	}
	return e, r.save(e)
}

// Verificar trae lo commiteado por el worker, comprueba que no tocó nada protegido, corre el examen
// intacto en la caja y deja un recibo (también en git). Falla dos veces o sin progreso → BLOQUEADO.
func (r Repo) Verificar(nn string, now time.Time, opc ...VerificarOpc) (*Estado, string, error) {
	inicio := time.Now()
	e, err := r.Load(nn)
	if err != nil || e == nil {
		return nil, "", errors.Join(err, errors.New("no hay slice abierto"))
	}
	if e.Fase != Implementando && e.Fase != Verificado {
		return e, "", fmt.Errorf("%s está en %s; no se verifica ahora", e.ID(), e.Fase)
	}
	if err = r.confiable(e); err != nil {
		return e, "", err
	}
	in, err := r.puntaDelSlice(e)
	if err != nil {
		return e, "", err
	}
	if in.Sucio != "" {
		return e, in.Sucio, errors.New("la copia del worker tiene cambios sin commit; solo cuenta lo commiteado: el worker debe commitear todo")
	}
	head := in.Punta
	if head == e.SHARojo {
		return e, "", fmt.Errorf("%s: el worker no ha commiteado nada (la punta es el commit rojo %s): no hay trabajo que verificar", e.ID(), head)
	}
	tocoExamen, otras, err := r.protegidasTocadas(e, e.baseExamen(), head)
	if err != nil {
		return e, "", err
	}
	if len(tocoExamen) > 0 {
		return e, strings.Join(tocoExamen, "\n"), r.bloquear(e, "el paquete del examen fue modificado desde el rojo: trampa")
	}
	if len(otras) > 0 {
		return e, strings.Join(otras, "\n"), fmt.Errorf("%s cambió rutas protegidas (%s): no se verifica y no cuenta como reparación. Que el worker lo deshaga; si el cambio es legítimo, apruébalo: `orq slice alcance RUTA --aprobado \"...\"`", e.ID(), strings.Join(cabeza(otras, 5), ", "))
	}
	h, err := HuellaEnCommit(r.Root, head, e.Bundle, e.Cmd)
	if err != nil {
		return e, "", err
	}
	if h != e.Huella {
		return e, "", r.bloquear(e, "la huella del examen cambió desde el rojo: trampa")
	}
	var notas []string
	cambiados, err := r.runnerCambiado(e, head)
	if err != nil {
		return e, "", err
	}
	if len(cambiados) > 0 {
		var acepto string
		if len(opc) > 0 {
			acepto = strings.TrimSpace(opc[0].AceptoRunner)
		}
		if acepto == "" {
			return e, "", &ErrRunnerCambio{ID: e.Attempt(), Archivos: cambiados}
		}
		notas = append(notas, fmt.Sprintf("runner cambiado y aceptado (%s): %s", acepto, strings.Join(cambiados, ", ")))
	}
	out, code, ok, cnt, intacto, err := r.examen(e, head)
	if err != nil {
		return e, out, err
	}
	// Una escritura sobre el examen durante la corrida también cuenta como trampa.
	if !intacto {
		return e, out, r.bloquear(e, "el examen se modificó mientras corría: trampa")
	}
	checks, err := r.checksEn(e, head)
	if err != nil {
		return e, out, err
	}
	rec := &Recibo{Attempt: e.Attempt(), Head: head, Bundle: e.Huella, Runner: Version, Cmd: e.Cmd,
		Exit: code, Salida: cola(out, 20), TS: now.UTC().Truncate(time.Second), Resultado: "paso", Aislado: true, Checks: checks}
	if rec.DefinicionesChecks, err = r.definicionesCambiadas(e.SHARojo, head); err != nil {
		return e, out, err
	}
	if rec.TestsBorrados, err = r.testsBorrados(e.SHARojo, head); err != nil {
		return e, out, err
	}
	if len(rec.DefinicionesChecks) > 0 {
		notas = append(notas, "check definitions changed: "+strings.Join(rec.DefinicionesChecks, ", "))
	}
	if len(rec.TestsBorrados) > 0 {
		notas = append(notas, "tests deleted: "+strings.Join(rec.TestsBorrados, ", "))
	}
	var vnota string
	ok, vnota = veredicto(ok, cnt, e.Omitir)
	if falta := notaFaltan(e.PruebasExamen, cnt); falta != "" {
		ok, vnota = false, strings.TrimPrefix(vnota+"; "+falta, "; ")
	}
	if falla := FalloDeChecks(checks); falla != "" {
		if !ok && vnota == "" {
			vnota = "el examen no pasa"
		}
		ok, vnota = false, strings.TrimPrefix(vnota+"; "+falla, "; ")
	}
	if cnt != nil {
		rec.Contado, rec.Ejecutados, rec.Omitidos, rec.Fallidos = true, cnt.Ejecutados, cnt.Omitidos, cnt.Fallidos
		if vnota != "" {
			notas = append(notas, vnota)
		}
	} else if e.SinConteo != "" {
		notas = append(notas, "sin conteo ("+e.SinConteo+"): no se sabe cuántas pruebas corrieron")
	} else {
		notas = append(notas, "sin contadores: este slice se abrió sin --reporte y --formato")
	}
	rec.Nota = strings.Join(notas, "; ")
	salidas := filepath.Join(r.OrqDir, "salidas")
	if err := os.MkdirAll(salidas, 0o755); err != nil {
		return e, out, err
	}
	rec.Artefacto = filepath.Join(salidas, fmt.Sprintf("%s-a%d-%d.txt", e.ID(), e.Intento, now.UnixNano()))
	if err := os.WriteFile(rec.Artefacto, []byte(out), 0o644); err != nil {
		return e, out, err
	}
	if !ok {
		rec.Resultado = "fallo"
	}
	// La corrida pudo durar minutos: si el slice cambió mientras tanto (relevo, rechazo, reintento), el recibo no vale.
	soltar, err := r.bloqueo(e.NN)
	if err != nil {
		return e, out, err
	}
	defer soltar()
	ahora, err := r.Load(e.NN)
	if err != nil || ahora == nil || ahora.Intento != e.Intento || ahora.Fase != e.Fase {
		return e, out, fmt.Errorf("%s cambió mientras se verificaba (relevo, reintento o cierre); recibo descartado, vuelve a verificar", e.Attempt())
	}
	if err := r.confiable(ahora); err != nil {
		return e, out, err
	}
	e.Escritor = ahora.Escritor
	if err := appendJSON(r.recibosPath(), rec); err != nil {
		return e, out, err
	}
	// terminado: cuándo el worker dejó la punta que se verificó, su último commit.
	terminado, err := r.git("log", "-1", "--format=%cI", head)
	if err != nil {
		return e, out, err
	}
	if err := diario.Anotar(diario.Ruta(r.OrqDir), diario.Evento{TS: now.Add(time.Since(inicio)), Tipo: "verificacion", Slice: e.NN, Intento: e.Intento, Ruta: e.RutaEfectiva(),
		Resultado: rec.Resultado, Datos: map[string]any{"head": head, "ejecutados": rec.Ejecutados, "fallidos": rec.Fallidos, "terminado": terminado}}); err != nil {
		return e, out, err
	}
	if ok {
		e.Fase, e.Recibo = Verificado, rec
		if err := r.sellarRecibo(e.NN, rec); err != nil {
			return e, out, err
		}
		return e, out, r.save(e)
	}
	if err := r.sellarRecibo(e.NN, nil); err != nil {
		return e, out, err
	}
	motivo := "el examen no pasa"
	if vnota != "" {
		motivo = vnota
	}
	// Exit 127: la caja no encontró un comando. Es infraestructura, no el worker: no gasta una
	// reparación ni cuenta para «sin progreso».
	if infra := infraestructura(code, checks); infra != "" {
		e.Recibo, e.Fase = nil, Implementando
		return e, out, errors.Join(r.save(e), fmt.Errorf("%s; infraestructura (%s, exit 127): no cuenta como reparación; `orq check --caja` dice qué no ve la caja", motivo, infra))
	}
	firma := firmaDe(out + FalloDeChecks(checks))
	sinProgreso := e.HeadFallo == head
	if !sinProgreso && e.HeadFallo != "" && firma == e.FirmaFallo {
		_, cod, errDiff := r.gitCodigo("diff", "-w", "--quiet", e.HeadFallo, head)
		sinProgreso = errDiff == nil && cod == 0 // solo cambiaron espacios o commits vacíos
	}
	e.Fallos, e.HeadFallo, e.FirmaFallo, e.Recibo, e.Fase = e.Fallos+1, head, firma, nil, Implementando
	switch {
	case sinProgreso:
		return e, out, r.bloquear(e, motivo+"; falla igual sin cambios reales: sin progreso")
	case e.Fallos >= MaxFallos:
		return e, out, r.bloquear(e, fmt.Sprintf("%s; %d verificaciones fallidas en %s", motivo, e.Fallos, e.Attempt()))
	}
	return e, out, errors.Join(r.save(e), fmt.Errorf("%s (reparación %d de %d)", motivo, e.Fallos, MaxFallos-1))
}

// infraestructura dice qué salió con 127 (comando no encontrado): el examen o un check; "" si nada.
func infraestructura(code int, checks []ResultadoCheck) string {
	var xs []string
	if code == 127 {
		xs = append(xs, "examen")
	}
	for _, c := range checks {
		if c.Resultado == CheckFallo && c.Exit == 127 {
			xs = append(xs, "check "+c.Nombre)
		}
	}
	return strings.Join(xs, ", ")
}

// Causas de un intento nuevo: el worker falló, o el examen estaba mal y se corrigió.
const (
	CausaWorker = "worker"
	CausaExamen = "examen"
)

// Reintentar abre un intento nuevo en la misma copia. Recibos viejos ya no valen. El diario anota
// la causa: un examen corregido no cuenta como fallo del worker.
func (r Repo) Reintentar(nn, cambio, causa string, decisionOwner bool) (*Estado, error) {
	soltar, err := r.bloqueo(nn)
	if err != nil {
		return nil, err
	}
	defer soltar()
	e, err := r.Load(nn)
	if err != nil || e == nil {
		return nil, errors.Join(err, errors.New("no hay slice abierto"))
	}
	if e.Fase != Bloqueado {
		return e, fmt.Errorf("%s está en %s; solo se reintenta desde %s", e.ID(), e.Fase, Bloqueado)
	}
	if strings.TrimSpace(cambio) == "" {
		return e, errors.New("di qué cambia en este intento; repetir lo mismo no es reintentar")
	}
	if causa != CausaWorker && causa != CausaExamen {
		return e, fmt.Errorf("di la causa del intento nuevo: %s|%s", CausaWorker, CausaExamen)
	}
	if e.Intento >= MaxIntentos && !decisionOwner {
		return e, fmt.Errorf("%s ya lleva %d intentos: el siguiente lo decide el Owner (el Atena pregunta y luego --decision-owner), o se parte la historia", e.ID(), e.Intento)
	}
	if err := r.sellarRecibo(e.NN, nil); err != nil {
		return e, err
	}
	e.Intento++
	if err := diario.Anotar(diario.Ruta(r.OrqDir), diario.Evento{Tipo: "intento", Slice: e.NN, Intento: e.Intento, Ruta: e.RutaEfectiva(),
		Detalle: cambio, Datos: map[string]any{"causa": causa}}); err != nil {
		return e, err
	}
	e.FirmaFallo = ""
	e.Fase, e.Fallos, e.HeadFallo, e.Recibo, e.Motivo, e.Escritor = Implementando, 0, "", nil, "", ""
	return e, r.save(e)
}

// Rehuella repara una huella mal tomada al abrir: solo desde BLOQUEADO y solo si ningún commit del
// slice tocó el examen. La huella nueva sale del commit del rojo, así que no sirve para colar un
// examen cambiado.
func (r Repo) Rehuella(nn, motivo string) (*Estado, error) {
	soltar, err := r.bloqueo(nn)
	if err != nil {
		return nil, err
	}
	defer soltar()
	e, err := r.Load(nn)
	if err != nil || e == nil {
		return nil, errors.Join(err, errors.New("no hay slice abierto"))
	}
	if e.Fase != Bloqueado {
		return e, fmt.Errorf("%s está en %s; la huella solo se repara desde %s", e.ID(), e.Fase, Bloqueado)
	}
	if strings.TrimSpace(motivo) == "" {
		return e, errors.New("di por qué la huella estaba mal y quién lo aprobó")
	}
	// El contrato guardado puede traer la huella mala: aquí se compara todo menos ella.
	var c Contrato
	if hay, err := r.leerBlob(refContrato(e.NN), &c); err != nil {
		return e, err
	} else if !hay {
		return e, r.confiable(e)
	} else if dif := diferencias(c, contratoDe(e)); len(dif) > 0 && !slices.Equal(dif, []string{"huella"}) {
		return e, r.confiable(e)
	}
	in, err := r.puntaDelSlice(e)
	if err != nil {
		return e, err
	}
	tocoExamen, _, err := r.protegidasTocadas(e, e.baseExamen(), in.Punta)
	if err != nil {
		return e, err
	}
	if len(tocoExamen) > 0 {
		return e, errors.New("el slice modificó el examen desde el rojo: eso es trampa, no una huella mal tomada")
	}
	enRojo, err := HuellaEnCommit(r.Root, e.baseExamen(), e.Bundle, e.Cmd)
	if err != nil {
		return e, err
	}
	e.Huella, e.Fase, e.Motivo, e.Escritor = enRojo, Implementando, "huella reparada: "+motivo, ""
	if err := r.sellar(e); err != nil {
		return e, err
	}
	return e, r.save(e)
}

// SumarExamen protege un examen nuevo, o corrige uno protegido, dentro de un slice abierto, con aprobación.
// Los archivos nombrados están commiteados en el slice y fuera de toda zona; el resto del examen no cambió.
// La huella y la base del examen pasan a la punta: desde ahí nadie lo edita y la verificación lo corre.
func (r Repo) SumarExamen(nn string, nuevos []string, cmd, aprobado string) (*Estado, error) {
	soltar, err := r.bloqueo(nn)
	if err != nil {
		return nil, err
	}
	defer soltar()
	e, err := r.Load(nn)
	if err != nil || e == nil {
		return nil, errors.Join(err, errors.New("no hay slice abierto"))
	}
	switch {
	case e.Fase != Implementando:
		return e, fmt.Errorf("%s está en %s; un examen se suma mientras se implementa", e.ID(), e.Fase)
	case strings.TrimSpace(aprobado) == "":
		return e, errors.New("di quién aprobó el examen nuevo (req de Atena u Owner)")
	case len(nuevos) == 0:
		return e, errors.New("di qué archivos de examen se suman")
	}
	if err := r.confiable(e); err != nil {
		return e, err
	}
	in, err := r.puntaDelSlice(e)
	if err != nil {
		return e, err
	}
	if in.Sucio != "" {
		return e, errors.New("la copia del slice tiene cambios sin commit; el examen nuevo tiene que estar commiteado")
	}
	head := in.Punta
	cambiados, err := r.tocadas(e.baseExamen(), head)
	if err != nil {
		return e, err
	}
	for _, f := range e.Bundle {
		if !slices.Contains(nuevos, f) && slices.Contains(cambiados, f) {
			return e, errors.New("el slice modificó un examen que no nombraste: no se suma ni se corrige nada")
		}
	}
	bundle := append([]string{}, e.Bundle...)
	for _, f := range nuevos {
		if r.blobEn(head, f) == "" {
			return e, fmt.Errorf("%s no está commiteado en el slice", f)
		}
		for _, w := range e.Equipo {
			for _, ruta := range w.Rutas {
				if dentro(ruta, f) {
					return e, fmt.Errorf("%s está dentro de la zona %s: un examen no es de ningún worker", f, w.Zona)
				}
			}
		}
		if !slices.Contains(bundle, f) {
			bundle = append(bundle, f)
		}
	}
	// Corregir un examen o cambiar su comando puede renombrar sus pruebas: la lista del rojo deja de valer.
	if strings.TrimSpace(cmd) != "" || slices.ContainsFunc(nuevos, func(f string) bool { return slices.Contains(e.Bundle, f) }) {
		e.PruebasExamen = nil
	}
	if strings.TrimSpace(cmd) != "" {
		e.Cmd = cmd
		// El comando nuevo puede apoyarse en otros archivos del runner: se vigilan desde ahora.
		for _, v := range r.vigiladosEn(head, e.Cmd, bundle) {
			if !slices.ContainsFunc(e.Vigilados, func(x Vigilado) bool { return x.Ruta == v.Ruta }) {
				e.Vigilados = append(e.Vigilados, v)
			}
		}
	}
	h, err := HuellaEnCommit(r.Root, head, bundle, e.Cmd)
	if err != nil {
		return e, err
	}
	e.Bundle, e.Huella, e.SHAExamen, e.Motivo = bundle, h, head, "examen sumado o corregido: "+aprobado
	if err := r.sellar(e); err != nil {
		return e, err
	}
	if err := diario.Anotar(diario.Ruta(r.OrqDir), diario.Evento{Tipo: "examen", Slice: e.NN, Intento: e.Intento, Ruta: e.RutaEfectiva(),
		Detalle: aprobado, Datos: map[string]any{"archivos": nuevos, "base": head}}); err != nil {
		return e, err
	}
	// Los clones saben qué examen no se toca: su marca se actualiza con el paquete nuevo.
	var errs []error
	if esClon(e.Worktree) {
		errs = append(errs, r.marcar(e, e.Worktree, e.Rama))
	}
	for _, w := range e.Equipo {
		if esClon(w.Worktree) {
			errs = append(errs, r.marcar(e, w.Worktree, w.Rama))
		}
	}
	return e, errors.Join(append(errs, r.save(e))...)
}

// Reabrir vuelve a construir un slice verificado para una reparación pedida por Atena u Owner:
// mismo intento, misma copia, mismo equipo y sus commits. El recibo deja de valer.
func (r Repo) Reabrir(nn, motivo string) (*Estado, error) {
	soltar, err := r.bloqueo(nn)
	if err != nil {
		return nil, err
	}
	defer soltar()
	e, err := r.Load(nn)
	if err != nil || e == nil {
		return nil, errors.Join(err, errors.New("no hay slice abierto"))
	}
	if e.Fase != Verificado && e.Fase != Integrando {
		return e, fmt.Errorf("%s está en %s; solo se reabre desde %s o %s", e.ID(), e.Fase, Verificado, Integrando)
	}
	if strings.TrimSpace(motivo) == "" {
		return e, errors.New("di qué se repara; reabrir sin motivo no vale")
	}
	if err := errors.Join(r.sellarRecibo(e.NN, nil), r.borrarRef(refIntegrado(e.NN))); err != nil {
		return e, err
	}
	e.Fase, e.Recibo, e.Integrado, e.Motivo = Implementando, nil, "", motivo
	if len(e.Equipo) > 0 {
		e.Escritor = "equipo"
	}
	return e, r.save(e)
}
