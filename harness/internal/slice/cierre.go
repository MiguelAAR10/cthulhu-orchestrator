package slice

// Closing: integrate without a working directory, re-run the exam on the exact commit that will be
// published, publish, clean up. Nothing here trusts the state file on its own: the contract, the
// receipt and the integrated commit are read back from git (refs/orq/).

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"orq/internal/diario"
	"orq/internal/orqdir"
)

// quitarCopia retires a worker's working copy. It never forces: when the copy still holds work the
// main repo does not have (commits that could not be brought in, or uncommitted files) it goes to
// quarantine and stays on disk.
func (r Repo) quitarCopia(e *Estado, dir, rama, nombre string, now time.Time) string {
	if !existe(dir) {
		return ""
	}
	if motivo := r.retirable(e, dir, rama, nombre); motivo != "" {
		if err := appendJSON(r.CuarentenaPath(), Cuarentena{Ruta: dir, Slice: e.ID(), Motivo: motivo, TS: now.UTC().Truncate(time.Second)}); err != nil {
			return dir + " (quarantine not recorded: " + err.Error() + ")"
		}
		return dir
	}
	return ""
}

// retirable removes dir when nothing would be lost and returns "" ; otherwise it returns why not
// and leaves dir untouched.
func (r Repo) retirable(e *Estado, dir, rama, nombre string) string {
	if !esClon(dir) {
		return "not a worker clone made by orq"
	}
	in, err := r.puntaDe(e, dir, rama, nombre)
	switch {
	case err != nil:
		return "its work could not be brought into the main repo: " + err.Error()
	case in.Sucio != "":
		return "it has uncommitted files"
	}
	if err := os.RemoveAll(dir); err != nil {
		return err.Error()
	}
	return ""
}

// CerrarOpc son las opciones de un cierre.
type CerrarOpc struct {
	AceptoChecks bool // el Owner vio el diff de los archivos que definen los checks y lo acepta
}

// Cerrar con aceptado integra, revalida, publica la rama base y limpia.
// Cada paso queda guardado: si se interrumpe, volver a correrlo continúa donde quedó.
func (r Repo) Cerrar(nn string, aceptado bool, motivo string, now time.Time, opc ...CerrarOpc) (*Registro, []string, string, error) {
	soltar, err := r.bloqueo(nn)
	if err != nil {
		return nil, nil, "", err
	}
	defer soltar()
	e, err := r.Load(nn)
	if nn != "" && err != nil && strings.Contains(err.Error(), "no está abierto") {
		for _, reg := range leerJSONL[Registro](r.logPath()) {
			if reg.NN == strings.TrimPrefix(nn, "S-") {
				return &reg, nil, "", ErrYaCerrado
			}
		}
	}
	if err != nil || e == nil {
		return nil, nil, "", errors.Join(err, errors.New("no hay slice abierto"))
	}
	reg := &Registro{NN: e.NN, Intento: e.Intento, Titulo: e.Titulo, Abierto: e.Abierto, Cerrado: now.UTC().Truncate(time.Second), Motivo: motivo,
		Ruta: e.RutaEfectiva(), Ascendido: e.Ascendido, RutaMotivo: e.RutaMotivo, Historia: e.Historia}
	var out string
	var cuarentena []string
	if aceptado {
		if !slices.Contains([]string{Verificado, Integrando, Cerrando}, e.Fase) {
			return nil, nil, "", fmt.Errorf("%s está en %s; solo se integra con un recibo de verificación", e.ID(), e.Fase)
		}
		// The state file is not evidence. It must say what orq stored in git when the slice was opened.
		if err := r.confiable(e); err != nil {
			return nil, nil, "", err
		}
		if e.Fase == Verificado {
			rec, err := r.reciboConfiable(e)
			if err != nil {
				return nil, nil, "", err
			}
			in, err := r.puntaDelSlice(e)
			if err != nil {
				return nil, nil, "", err
			}
			if in.Punta != rec.Head || in.Sucio != "" {
				return nil, nil, "", errors.New("el recibo no es de este intento o de este commit: corre `orq slice verificar`")
			}
			if len(rec.DefinicionesChecks) > 0 && !(len(opc) > 0 && opc[0].AceptoChecks) {
				return nil, nil, "", fmt.Errorf("%s cambió lo que corre un check (%s): mira el diff en `orq slice decidir`; si el Owner lo acepta, `orq slice cerrar --aceptado --acepto-checks`",
					e.Attempt(), strings.Join(rec.DefinicionesChecks, ", "))
			}
			if err := r.anotarDecision(e, "aceptado", "", now); err != nil {
				return nil, nil, "", err
			}
			e.Fase = Integrando
			if err := r.save(e); err != nil {
				return nil, nil, "", err
			}
		}
		// Integrar y publicar van en serie: dos cierres nunca integran a la vez.
		soltarInt, err := r.candadoTras("slice-"+e.NN, "integracion")
		if err != nil {
			return nil, nil, "", err
		}
		defer soltarInt()
		// Another process may have advanced this close while we waited for the lock.
		if e, err = r.Load(e.NN); err != nil || e == nil {
			return nil, nil, "", errors.Join(err, errors.New("the slice disappeared while waiting for the integration lock"))
		}
		if err := r.confiable(e); err != nil {
			return nil, nil, "", err
		}
		fallaEn("integrar")
		if e.Fase == Cerrando {
			// A state that claims CERRANDO is only believed when git holds the integrated commit orq revalidated.
			ok, err := r.integradoConfiable(e.NN, e.Integrado)
			if err != nil {
				return nil, nil, "", err
			}
			if !ok {
				e.Fase, e.Integrado = Integrando, ""
				if err := r.save(e); err != nil {
					return nil, nil, "", err
				}
			}
		}
		if e.Fase == Integrando {
			out, err = r.integrar(e)
			if errL := r.anotarIntegracion(e, err, now); err != nil || errL != nil {
				return nil, nil, out, errors.Join(err, errL)
			}
			fallaEn("integrado")
			e.Fase = Cerrando
			if err := r.save(e); err != nil {
				return nil, nil, out, err
			}
		}
		if e.Fase != Cerrando {
			return nil, nil, out, fmt.Errorf("%s está en %s; solo se integra con un recibo de verificación", e.ID(), e.Fase)
		}
		if err := r.publicar(e); err != nil {
			if errors.Is(err, ErrBaseAvanzo) {
				e.Fase = Integrando // la próxima vez integra de nuevo sobre la base nueva
				err = errors.Join(err, r.save(e))
			}
			return nil, nil, out, err
		}
		fallaEn("publicado")
		if err := r.libre(e); err != nil {
			return nil, nil, out, err
		}
		reg.Result, reg.SHA = "aceptado", e.Integrado
	} else {
		if !slices.Contains([]string{Implementando, Verificado, Bloqueado}, e.Fase) {
			return nil, nil, "", fmt.Errorf("%s está en %s: ya se está integrando; termina el cierre aceptado", e.ID(), e.Fase)
		}
		reg.Result = "rechazado"
		if err := r.libre(e); err != nil {
			return nil, nil, "", err
		}
		if err := r.anotarDecision(e, "rechazado", motivo, now); err != nil {
			return nil, nil, "", err
		}
	}
	cuarentena = append(cuarentena, r.limpiarEquipo(e, aceptado, now)...)
	if q := r.quitarCopia(e, e.Worktree, e.Rama, "slice", now); q != "" {
		cuarentena = append(cuarentena, q)
	}
	fallaEn("limpiando")
	var errs []error
	if aceptado {
		if b, err := r.git("branch", "--list", e.Rama); err != nil {
			errs = append(errs, err)
		} else if b != "" {
			// The branch is deleted only when it is already inside the base.
			sha, errSha := r.git("rev-parse", "refs/heads/"+e.Rama)
			dentroBase, errAnc := r.esAncestro("refs/heads/"+e.Rama, "refs/heads/"+e.Base)
			switch {
			case errSha != nil || errAnc != nil:
				errs = append(errs, errSha, errAnc)
			case !dentroBase:
				errs = append(errs, appendJSON(r.CuarentenaPath(), Cuarentena{Rama: e.Rama, Slice: e.ID(), Motivo: "no está integrada en " + e.Base, TS: reg.Cerrado}))
				cuarentena = append(cuarentena, e.Rama)
			default:
				if _, err := r.git("update-ref", "-d", "refs/heads/"+e.Rama, sha); err != nil {
					errs = append(errs, appendJSON(r.CuarentenaPath(), Cuarentena{Rama: e.Rama, Slice: e.ID(), Motivo: err.Error(), TS: reg.Cerrado}))
					cuarentena = append(cuarentena, e.Rama)
				}
			}
		}
	}
	errs = append(errs, r.borrarRefs(e))
	if err := errors.Join(errs...); err != nil {
		return nil, cuarentena, out, err
	}
	if err := appendJSON(r.logPath(), reg); err != nil {
		return nil, cuarentena, out, err
	}
	if err := diario.Anotar(diario.Ruta(r.OrqDir), diario.Evento{Tipo: "fase", Slice: e.NN, Intento: e.Intento, Ruta: e.RutaEfectiva(),
		De: e.Fase, A: "CERRADO", Resultado: reg.Result, Detalle: motivo}); err != nil {
		return nil, cuarentena, out, err
	}
	return reg, cuarentena, out, os.Remove(r.estadoPath(e.NN))
}

// anotarDecision deja en el diario el momento en que se aceptó o rechazó el slice. Un cierre
// aceptado se anota al salir de VERIFICADO; retomarlo no vuelve a decidir.
func (r Repo) anotarDecision(e *Estado, resultado, motivo string, now time.Time) error {
	return diario.Anotar(diario.Ruta(r.OrqDir), diario.Evento{TS: now, Tipo: "decision", Slice: e.NN, Intento: e.Intento, Ruta: e.RutaEfectiva(),
		Resultado: resultado, Detalle: motivo})
}

// anotarIntegracion is the ledger row of one integration into the base: SHA and result.
func (r Repo) anotarIntegracion(e *Estado, fallo error, now time.Time) error {
	ev := diario.Evento{TS: now, Tipo: "integracion", Paso: "base", Slice: e.NN, Intento: e.Intento, Ruta: e.RutaEfectiva(),
		Resultado: "ok", Datos: map[string]any{"sha": e.Integrado, "base": e.Base}}
	if fallo != nil {
		ev.Resultado, ev.Detalle = "fallo", fallo.Error()
	}
	return diario.Anotar(diario.Ruta(r.OrqDir), ev)
}

// libre comprueba que nadie trabaje en las carpetas que se van a quitar.
func (r Repo) libre(e *Estado) error {
	ds := []string{e.Worktree}
	for _, w := range e.Equipo {
		ds = append(ds, w.Worktree)
	}
	for _, d := range ds {
		if u := EnUso(d); len(u) > 0 && existe(d) {
			return fmt.Errorf("%w: %s lo usa %s. Cierra ese panel o sesión y repite el cierre (continúa donde quedó)", ErrEnUso, d, strings.Join(u, ", "))
		}
	}
	return nil
}

// primeraLinea returns the first line of s.
func primeraLinea(s string) string {
	l, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	return strings.TrimSpace(l)
}

// fusionar merges otra into base inside the main repo's object store, with no working directory:
// merge-tree computes the tree, commit-tree records it. conflicto=true means git could not merge.
func (r Repo) fusionar(base, otra, mensaje string) (sha string, conflicto bool, err error) {
	out, code, err := r.gitCodigo("merge-tree", "--write-tree", "--no-messages", base, otra)
	switch {
	case err != nil:
		return "", false, err
	case code == 1:
		return "", true, nil
	case code != 0:
		return "", false, fmt.Errorf("git merge-tree %s %s: %s", base[:8], otra[:8], out)
	}
	sha, err = r.git("-c", "user.name="+r.identidad("user.name", "orq"), "-c", "user.email="+r.identidad("user.email", "orq@local"),
		"commit-tree", primeraLinea(out), "-p", base, "-p", otra, "-m", mensaje)
	return sha, false, err
}

// integrar merges the verified commit into the base (no working directory), re-runs the exam and
// --sano in the box on that exact merge commit, and records it in git as the commit to publish.
func (r Repo) integrar(e *Estado) (string, error) {
	rec, err := r.reciboConfiable(e) // the receipt orq issued, read from git; never the one in the state alone
	if err != nil {
		return "", err
	}
	base, err := r.git("rev-parse", "--verify", "refs/heads/"+e.Base+"^{commit}")
	if err != nil {
		return "", err
	}
	// Protected paths again, on what is really about to be integrated.
	tocoExamen, otras, err := r.protegidasTocadas(e, e.baseExamen(), rec.Head)
	if err != nil {
		return "", err
	}
	if len(tocoExamen)+len(otras) > 0 {
		return "", fmt.Errorf("the verified commit %s changes protected paths (%s): not integrated. Nothing was deleted", Corto(rec.Head), strings.Join(cabeza(append(tocoExamen, otras...), 5), ", "))
	}
	sha, conflicto, err := r.fusionar(base, rec.Head, fmt.Sprintf("slice %s: %s", e.Attempt(), e.Titulo))
	if err != nil {
		return "", err
	}
	if conflicto {
		return "", fmt.Errorf("conflicto al integrar con %s. Nada se borró; el slice sigue en %s", e.Base, Integrando)
	}
	h, err := HuellaEnCommit(r.Root, sha, e.Bundle, e.Cmd)
	if err != nil {
		return "", err
	}
	if h != e.Huella {
		return "", fmt.Errorf("tras integrar, el paquete del examen difiere del rojo (la base cambió). Nada se borró")
	}
	out, _, ok, cnt, intacto, errExamen := r.examen(e, sha)
	if errExamen != nil && cnt == nil {
		return out, errExamen
	}
	ok, nota := veredicto(ok && errExamen == nil && intacto, cnt, e.Omitir)
	if falta := notaFaltan(e.PruebasExamen, cnt); ok && falta != "" {
		ok, nota = false, falta
	}
	if !ok {
		if nota != "" {
			nota = " (" + nota + ")"
		}
		return out, fmt.Errorf("integrado en %s, pero el examen falla ahí%s. Tu rama %s no se tocó; nada se borró", sha[:8], nota, e.Base)
	}
	if e.Sano != "" {
		o2, ok, err := r.sanoEn(sha, e.Sano)
		if err != nil {
			return o2, err
		}
		if !ok {
			return o2, fmt.Errorf("integrado, pero la base dejó de estar sana (--sano falla). Nada se borró")
		}
	}
	if _, err := r.git("update-ref", refIntegrado(e.NN), sha); err != nil {
		return out, err
	}
	e.Integrado = sha
	return out, nil
}

// publicar avanza la rama base al merge revalidado. Si el dueño tiene la rama abierta y limpia,
// solo avanza (fast-forward); si tiene cambios, espera sin tocar nada.
func (r Repo) publicar(e *Estado) error {
	// Only the commit orq revalidated and recorded in git is ever published.
	if ok, err := r.integradoConfiable(e.NN, e.Integrado); err != nil {
		return err
	} else if !ok {
		return fmt.Errorf("%w: %s claims the integrated commit %q, which is not the one orq revalidated (%s). Nothing is published", ErrEstadoForjado, e.ID(), e.Integrado, refIntegrado(e.NN))
	}
	if ya, err := r.esAncestro(e.Integrado, "refs/heads/"+e.Base); err != nil {
		return err
	} else if ya {
		return nil // ya publicado: repetir el cierre no hace nada dos veces
	}
	actual, err := r.git("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return err
	}
	if actual == e.Base {
		st, err := gitDueno(r.Root, "status", "--porcelain", "--untracked-files=no")
		if err != nil {
			return err
		}
		if st != "" {
			return fmt.Errorf("integrado y revalidado (%s), pero tu checkout tiene cambios sin commit en %s. Commitéalos o guárdalos y vuelve a correr el cierre: continúa donde quedó", Corto(e.Integrado), e.Base)
		}
		if _, err := gitDueno(r.Root, "merge", "--ff-only", e.Integrado); err != nil {
			return fmt.Errorf("%w: %s avanzó mientras tanto; vuelve a correr el cierre e integra de nuevo", ErrBaseAvanzo, e.Base)
		}
		return nil
	}
	wl, err := r.git("worktree", "list", "--porcelain")
	if err != nil {
		return err
	}
	if strings.Contains(wl+"\n", "branch refs/heads/"+e.Base+"\n") {
		return fmt.Errorf("integrado y revalidado (%s), pero %s está abierta en otro worktree. Cámbiate a %s en el checkout principal y vuelve a correr el cierre", Corto(e.Integrado), e.Base, e.Base)
	}
	tip, err := r.git("rev-parse", "refs/heads/"+e.Base)
	if err != nil {
		return err
	}
	if ok, err := r.esAncestro(tip, e.Integrado); err != nil {
		return err
	} else if !ok {
		return fmt.Errorf("%w: %s avanzó mientras tanto; vuelve a correr el cierre e integra de nuevo", ErrBaseAvanzo, e.Base)
	}
	_, err = r.git("update-ref", "refs/heads/"+e.Base, e.Integrado, tip)
	return err
}

// Huerfanos lists working copies that belong to no open slice: linked worktrees other than the main
// checkout, and worker clones orq made under the worktrees folder.
func (r Repo) Huerfanos() ([]string, error) {
	out, err := r.git("worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	ab, err := r.Abiertos()
	if err != nil {
		return nil, err
	}
	propio := map[string]bool{r.Root: true}
	for _, e := range ab {
		propio[e.Worktree] = true
		for _, w := range e.Equipo {
			propio[w.Worktree] = true
		}
	}
	var res []string
	for _, l := range strings.Split(out, "\n") {
		p, ok := strings.CutPrefix(l, "worktree ")
		if !ok || propio[p] {
			continue
		}
		res = append(res, p)
	}
	clones, _ := filepath.Glob(filepath.Join(r.Worktree, orqdir.Slug(r.Root)+"-s*"))
	for _, c := range clones {
		if m, ok := LeerMarca(c); ok && m.Root == r.Root && !propio[c] {
			res = append(res, c)
		}
	}
	return res, nil
}
