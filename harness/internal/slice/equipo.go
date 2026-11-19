package slice

// A team: several workers on one slice, each in its own clone, branch and zone. orq brings each
// worker's commits in by bundle and merges them into the slice branch inside the main repo.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// puntaDe brings in what is committed on branch rama of a worker clone and fast-forwards that
// branch in the main repo. It never moves a branch backwards or sideways.
func (r Repo) puntaDe(e *Estado, clon, rama, nombre string) (Ingesta, error) {
	in, err := r.ingerir(e, clon, rama, nombre)
	if err != nil {
		return in, err
	}
	actual, code, err := r.gitCodigo("rev-parse", "-q", "--verify", "refs/heads/"+rama+"^{commit}")
	if err != nil {
		return in, err
	}
	if code != 0 { // the branch was already removed from the main repo: its work must be inside the slice
		if ok, err := r.esAncestro(in.Punta, "refs/heads/"+e.Rama); err != nil || !ok {
			return in, errors.Join(err, fmt.Errorf("branch %s no longer exists in the main repo and the clone has commits (%s) that the slice does not", rama, in.Punta[:8]))
		}
		return in, nil
	}
	if in.Punta == actual {
		return in, nil
	}
	if ok, err := r.esAncestro(actual, in.Punta); err != nil {
		return in, err
	} else if !ok {
		if dentro, err := r.esAncestro(in.Punta, actual); err == nil && dentro {
			in.Punta = actual // the clone is behind what was already brought in: nothing new
			return in, nil
		}
		return in, fmt.Errorf("the clone %s no longer continues branch %s (%s): the worker rewrote commits already brought in. It must redo its work on top of %s", clon, rama, actual[:8], actual[:8])
	}
	_, err = r.git("update-ref", "refs/heads/"+rama, in.Punta, actual)
	return in, err
}

// Sumar agrega un worker al equipo del slice, con su rama y su clon propios desde la punta del slice.
func (r Repo) Sumar(nn, nombre, agente, zona string, rutas []string) (*Estado, *Worker, error) {
	soltar, err := r.bloqueo(nn)
	if err != nil {
		return nil, nil, err
	}
	defer soltar()
	e, err := r.Load(nn)
	if err != nil || e == nil {
		return nil, nil, errors.Join(err, errors.New("no hay slice abierto"))
	}
	switch {
	case e.RutaEfectiva() == RutaFix:
		return e, nil, e.errRutaFix("equipo por zonas")
	case e.Fase != Implementando:
		return e, nil, fmt.Errorf("%s está en %s; el equipo trabaja en %s", e.ID(), e.Fase, Implementando)
	case !zonaRe.MatchString(zona):
		return e, nil, errors.New("zona: minúsculas, números o guiones (front, back, test…)")
	case len(rutas) == 0:
		return e, nil, errors.New("di qué rutas son de esta zona (--rutas src/web,src/ui)")
	}
	if err := r.confiable(e); err != nil {
		return e, nil, err
	}
	var propio *Worker
	for i := range e.Equipo {
		w := e.Equipo[i]
		if w.Zona == zona {
			if w.Nombre != nombre {
				return e, nil, fmt.Errorf("la zona %s ya es de %s", zona, w.Nombre)
			}
			// La zona sigue con su worker: la tarea siguiente del plan reusa su rama y su copia.
			propio = &e.Equipo[i]
			continue
		}
		for _, a := range w.Rutas {
			for _, b := range rutas {
				if dentro(a, b) || dentro(b, a) {
					return e, nil, fmt.Errorf("las rutas %s y %s se pisan con la zona %s: dos workers no escriben lo mismo", b, a, w.Zona)
				}
			}
		}
	}
	for _, b := range rutas {
		for _, f := range e.Bundle {
			if dentro(b, f) {
				return e, nil, fmt.Errorf("la ruta %s incluye el examen %s, que nadie edita", b, f)
			}
		}
	}
	if propio != nil && !existe(propio.Worktree) {
		ok, err := r.podable(e, *propio)
		if err != nil {
			return e, nil, err
		}
		if ok {
			// Ya podada: su trabajo está en el slice, así que la zona arranca de nuevo desde ahí.
			e.Equipo = slices.DeleteFunc(e.Equipo, func(w Worker) bool { return w.Zona == zona })
			propio = nil
		}
	}
	if propio != nil {
		return r.reusar(e, propio, agente, rutas)
	}
	if len(e.Equipo) >= MaxEquipo {
		return e, nil, fmt.Errorf("%s ya tiene %d workers (tope %d)", e.ID(), len(e.Equipo), MaxEquipo)
	}
	w := Worker{Nombre: nombre, Agente: agente, Zona: zona, Rutas: rutas, Rama: e.Rama + "-" + zona, Worktree: e.Worktree + "-" + zona}
	if out, err := r.git("branch", "--list", w.Rama); err != nil {
		return e, nil, err
	} else if out != "" || existe(w.Worktree) {
		return e, nil, fmt.Errorf("ya existe %s o %s; revisa con `orq doctor`", w.Rama, w.Worktree)
	}
	// With a team nobody writes in the slice's own clone: what is committed there is brought in first.
	in, err := r.puntaDelSlice(e)
	if err != nil {
		return e, nil, err
	}
	if _, err := r.git("update-ref", "refs/heads/"+w.Rama, in.Punta, ""); err != nil {
		return e, nil, err
	}
	if err := r.crearClon(e, w.Worktree, w.Rama); err != nil {
		_, errRef := r.git("update-ref", "-d", "refs/heads/"+w.Rama, in.Punta)
		return e, nil, errors.Join(err, errRef, os.RemoveAll(w.Worktree))
	}
	e.Equipo = append(e.Equipo, w)
	e.Escritor = "equipo"
	return e, &w, r.save(e)
}

// reusar le da a la zona su tarea siguiente: suma las rutas nuevas y, si su copia está limpia,
// la avanza a la punta del slice para que vea lo que juntaron las otras zonas.
func (r Repo) reusar(e *Estado, w *Worker, agente string, rutas []string) (*Estado, *Worker, error) {
	if !existe(w.Worktree) {
		return e, nil, fmt.Errorf("la zona %s perdió su worktree %s; revisa con `orq doctor`", w.Zona, w.Worktree)
	}
	punta, err := r.Punta(e)
	if err != nil {
		return e, nil, err
	}
	sucio, err := r.avanzarClon(e, w.Worktree, punta) // si no avanza rápido, sigue desde su propia punta
	if err != nil {
		return e, nil, err
	}
	if sucio != "" {
		return e, nil, fmt.Errorf("%s tiene cambios sin commit; júntalos o deshazlos antes de la tarea siguiente", w.Worktree)
	}
	for _, b := range rutas {
		nueva := true
		for _, a := range w.Rutas {
			nueva = nueva && !dentro(a, b)
		}
		if nueva {
			w.Rutas = append(w.Rutas, b)
		}
	}
	w.Agente = agente
	return e, w, r.save(e)
}

// RutaMapa es el mapa de rutas de una app web, versionado en el repo: el worker que suma una ruta
// suma su línea en el mismo commit, así el diff lo muestra. Es de todas las zonas.
const RutaMapa = "docs/mapa.md"

// MapaEn es la ruta del mapa de rutas de root: docs/mapa.md, o el viejo .orq/mapa.md si solo está
// ese; "" si no hay ninguno.
func MapaEn(root string) string {
	for _, p := range []string{filepath.Join(root, RutaMapa), filepath.Join(root, ".orq", "mapa.md")} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// Juntar integra en la rama del slice lo nuevo de cada worker. Rechaza a quien tocó fuera de su zona,
// el examen u otra ruta protegida, y se detiene ante un conflicto sin dejar nada a medias.
func (r Repo) Juntar(nn string) (*Estado, []string, error) {
	soltar, err := r.bloqueo(nn)
	if err != nil {
		return nil, nil, err
	}
	defer soltar()
	e, err := r.Load(nn)
	if err != nil || e == nil {
		return nil, nil, errors.Join(err, errors.New("no hay slice abierto"))
	}
	if len(e.Equipo) == 0 {
		return e, nil, errors.New("este slice no tiene equipo: el worker escribe directo en el worktree del slice")
	}
	if err := r.confiable(e); err != nil {
		return e, nil, err
	}
	var informe []string
	var fallo error
	for i := range e.Equipo {
		w := &e.Equipo[i]
		in, err := r.puntaDe(e, w.Worktree, w.Rama, w.Nombre)
		if err != nil {
			informe = append(informe, fmt.Sprintf("%s: its work could not be brought in: %v", w.Nombre, err))
			fallo = errors.New("a worker's commits could not be brought into the main repo")
			continue
		}
		if in.Sucio != "" {
			informe = append(informe, fmt.Sprintf("%s: tiene cambios sin commit; no se junta", w.Nombre))
			continue
		}
		head := in.Punta
		if head == w.Juntado {
			informe = append(informe, fmt.Sprintf("%s: nada nuevo", w.Nombre))
			continue
		}
		punta, err := r.Punta(e)
		if err != nil {
			return e, informe, err
		}
		base, err := r.git("merge-base", punta, head)
		if err != nil {
			return e, informe, err
		}
		cambios, err := r.tocadas(base, head)
		if err != nil {
			return e, informe, err
		}
		_, protegidas, err := r.protegidasTocadas(e, base, head)
		if err != nil {
			return e, informe, err
		}
		var fuera []string
		for _, f := range cambios {
			ok := false
			for _, ruta := range w.Rutas {
				ok = ok || dentro(ruta, f)
			}
			if (!ok && f != RutaMapa) || slices.Contains(e.Bundle, f) || slices.Contains(protegidas, f) {
				fuera = append(fuera, f)
			}
		}
		if len(fuera) > 0 {
			informe = append(informe, fmt.Sprintf("%s: tocó fuera de su zona (%s); no se junta. Que lo deshaga o lo pida al dueño de esa zona", w.Nombre, strings.Join(fuera, ", ")))
			fallo = errors.New("hay workers fuera de su zona")
			continue
		}
		nuevo, conflicto, err := r.fusionar(punta, head, fmt.Sprintf("%s: junta %s (%s)", e.Attempt(), w.Zona, w.Nombre))
		if err != nil {
			return e, informe, err
		}
		if conflicto {
			informe = append(informe, fmt.Sprintf("%s: conflicto al juntar; nada cambió. Que se coordinen por el canal", w.Nombre))
			fallo = errors.New("conflicto al juntar")
			continue
		}
		if _, err := r.git("update-ref", "refs/heads/"+e.Rama, nuevo, punta); err != nil {
			return e, informe, err
		}
		w.Juntado = head
		informe = append(informe, fmt.Sprintf("%s: juntado %s", w.Nombre, head[:8]))
	}
	return e, informe, errors.Join(r.save(e), fallo)
}

// limpiarEquipo retires the team's clones; a branch goes only when it is already inside the base.
func (r Repo) limpiarEquipo(e *Estado, aceptado bool, now time.Time) []string {
	var q []string
	for _, w := range e.Equipo {
		if x := r.quitarCopia(e, w.Worktree, w.Rama, w.Nombre, now); x != "" {
			q = append(q, x)
		}
		if !aceptado {
			continue
		}
		sha, code, err := r.gitCodigo("rev-parse", "-q", "--verify", "refs/heads/"+w.Rama)
		if err != nil || code != 0 {
			continue // no branch left to decide about
		}
		if ok, err := r.esAncestro(sha, "refs/heads/"+e.Base); err == nil && ok {
			if _, err := r.git("update-ref", "-d", "refs/heads/"+w.Rama, sha); err == nil {
				continue
			}
		}
		if err := appendJSON(r.CuarentenaPath(), Cuarentena{Rama: w.Rama, Slice: e.ID(), Motivo: "trabajo del worker no juntado en el slice", TS: now.UTC().Truncate(time.Second)}); err == nil {
			q = append(q, w.Rama)
		} else {
			q = append(q, w.Rama+" (quarantine not recorded: "+err.Error()+")")
		}
	}
	return q
}

// Alinear no bloquea: detecta desvío para que el Orquestador pregunte al Atena antes de seguir.
// It compares what the main repo already holds of the slice (its branch and each worker's branch).
func (r Repo) Alinear(nn string) (*Estado, *Alineacion, error) {
	e, err := r.Load(nn)
	if err != nil {
		return nil, nil, err
	}
	if e == nil {
		return nil, nil, errors.New("no hay slice abierto")
	}
	// Each branch is read at its clone's tip when there is a clone: the main repo only learns of a
	// worker's commits when they are brought in, and drift must show before that.
	type fuente struct{ rama, clon, nombre string }
	fuentes := []fuente{{rama: e.Rama, nombre: "slice"}}
	if len(e.Equipo) == 0 {
		fuentes[0].clon = e.Worktree
	}
	for _, w := range e.Equipo {
		fuentes = append(fuentes, fuente{w.Rama, w.Worktree, w.Nombre})
	}
	a := &Alineacion{}
	visto := map[string]bool{}
	for _, f := range fuentes {
		punta := "refs/heads/" + f.rama
		if f.clon != "" && esClon(f.clon) {
			in, err := r.ingerir(e, f.clon, f.rama, f.nombre) // only fetches into refs/orq/sNN/<name>
			if err != nil {
				return e, nil, err
			}
			punta = in.Punta
		} else if _, code, err := r.gitCodigo("rev-parse", "-q", "--verify", punta); err != nil || code != 0 {
			continue
		}
		fs, err := r.tocadas(e.SHARojo, punta)
		if err != nil {
			return e, nil, err
		}
		for _, f := range fs {
			if visto[f] {
				continue
			}
			visto[f] = true
			a.Cambiados = append(a.Cambiados, f)
			if e.fueraDeAlcance(f) {
				a.Fuera = append(a.Fuera, f)
			}
		}
	}
	if len(a.Fuera) > 0 {
		a.Avisos = append(a.Avisos, fmt.Sprintf("%d archivo(s) fuera de lo que la historia justifica: %s", len(a.Fuera), strings.Join(cabeza(a.Fuera, 5), ", ")))
	}
	if len(a.Cambiados) > MaxArchivos {
		a.Avisos = append(a.Avisos, fmt.Sprintf("%d archivos cambiados: ya no es un slice delgado", len(a.Cambiados)))
	}
	return e, a, nil
}

// fueraDeAlcance dice si el archivo f cae fuera de lo que el slice justifica: el examen, el bundle, el
// alcance y las zonas de cada worker. Sin alcance ni zonas declarados nada queda fuera.
func (e *Estado) fueraDeAlcance(f string) bool {
	permitido := append([]string{e.Test}, e.Bundle...)
	permitido = append(permitido, e.Alcance...)
	acotado := len(e.Alcance) > 0
	for _, w := range e.Equipo {
		permitido = append(permitido, w.Rutas...)
		acotado = acotado || len(w.Rutas) > 0
	}
	if !acotado || f == RutaMapa {
		return false
	}
	for _, p := range permitido {
		if dentro(p, f) {
			return false
		}
	}
	return true
}

// podable: la rama del worker ya está dentro del slice (o ya no existe). Su trabajo no se pierde.
func (r Repo) podable(e *Estado, w Worker) (bool, error) {
	_, code, err := r.gitCodigo("rev-parse", "-q", "--verify", "refs/heads/"+w.Rama)
	if err != nil {
		return false, err
	}
	if code != 0 {
		return true, nil
	}
	return r.esAncestro("refs/heads/"+w.Rama, "refs/heads/"+e.Rama)
}

// Podar retira workers que terminaron: quita su clon y su rama y los saca del equipo, solo si su
// trabajo ya está juntado en el slice y no dejaron cambios sin commit. Lo demás queda y se informa.
func (r Repo) Podar(nn string, nombres []string) (*Estado, []string, error) {
	soltar, err := r.bloqueo(nn)
	if err != nil {
		return nil, nil, err
	}
	defer soltar()
	e, err := r.Load(nn)
	if err != nil || e == nil {
		return nil, nil, errors.Join(err, errors.New("no hay slice abierto"))
	}
	var informe []string
	for _, nombre := range nombres {
		i := slices.IndexFunc(e.Equipo, func(w Worker) bool { return w.Nombre == nombre })
		if i < 0 {
			informe = append(informe, nombre+": no está en el equipo")
			continue
		}
		w := e.Equipo[i]
		if existe(w.Worktree) {
			// Bring in its last commits first: what is not inside the slice must not be lost.
			in, err := r.puntaDe(e, w.Worktree, w.Rama, w.Nombre)
			if err != nil {
				informe = append(informe, nombre+": its work could not be brought in; not pruned: "+err.Error())
				continue
			}
			if in.Sucio != "" {
				informe = append(informe, nombre+": tiene cambios sin commit; no se poda")
				continue
			}
		}
		ok, err := r.podable(e, w)
		if err != nil {
			return e, informe, err
		}
		if !ok {
			informe = append(informe, nombre+": su rama no está juntada en el slice; no se poda")
			continue
		}
		if existe(w.Worktree) {
			if motivo := r.retirable(e, w.Worktree, w.Rama, w.Nombre); motivo != "" {
				informe = append(informe, nombre+": no se pudo quitar el worktree: "+motivo)
				continue
			}
		}
		if sha, code, err := r.gitCodigo("rev-parse", "-q", "--verify", "refs/heads/"+w.Rama); err != nil {
			return e, informe, err
		} else if code == 0 {
			if _, err := r.git("update-ref", "-d", "refs/heads/"+w.Rama, sha); err != nil {
				return e, informe, err
			}
		}
		if err := r.borrarRef(refWorker(e.NN, w.Nombre)); err != nil {
			return e, informe, err
		}
		e.Equipo = slices.Delete(e.Equipo, i, i+1)
		informe = append(informe, nombre+": podado (worktree y rama "+w.Rama+")")
	}
	return e, informe, r.save(e)
}
