package slice

// El contrato es lo que decide si un slice pasó: qué examen, con qué comando, desde qué commit.
// El estado (.orq/slices/NN.json) es un archivo que cualquiera con acceso al disco puede reescribir;
// por eso el contrato, el recibo vigente y el merge revalidado se guardan además en git, bajo
// refs/orq/, dentro del repo principal: un worker encerrado no puede escribir ahí.
// Cada camino que acepta trabajo compara el estado con esas referencias y se niega si difieren.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"

	"orq/internal/config"
	"orq/internal/gitx"
)

// Contrato son los campos del estado que deciden la aceptación.
type Contrato struct {
	NN         string     `json:"nn"`
	Test       string     `json:"test"`
	Bundle     []string   `json:"bundle"`
	Cmd        string     `json:"cmd"`
	Sano       string     `json:"sano,omitempty"`
	Base       string     `json:"base"`
	Rama       string     `json:"rama"`
	SHARojo    string     `json:"sha_rojo"`
	SHAExamen  string     `json:"sha_examen,omitempty"`
	Huella     string     `json:"huella"`
	Reporte    string     `json:"reporte,omitempty"`
	Formato    string     `json:"formato,omitempty"`
	Omitir     bool       `json:"permite_omitir,omitempty"`
	SinConteo  string     `json:"sin_conteo,omitempty"`
	TimeoutSeg int        `json:"timeout_seg,omitempty"`
	Vigilados  []Vigilado `json:"vigilados,omitempty"`
	Alcance    []string   `json:"alcance,omitempty"`
	Ratifico   string     `json:"ratifico,omitempty"`
}

func contratoDe(e *Estado) Contrato {
	return Contrato{NN: e.NN, Test: e.Test, Bundle: e.Bundle, Cmd: e.Cmd, Sano: e.Sano, Base: e.Base, Rama: e.Rama,
		SHARojo: e.SHARojo, SHAExamen: e.SHAExamen, Huella: e.Huella, Reporte: e.Reporte, Formato: e.Formato,
		Omitir: e.Omitir, SinConteo: e.SinConteo, TimeoutSeg: e.TimeoutSeg, Vigilados: e.Vigilados, Alcance: e.Alcance, Ratifico: e.Ratifico}
}

func refContrato(nn string) string  { return "refs/orq/contrato/" + nn }
func refRecibo(nn string) string    { return "refs/orq/recibo/" + nn }
func refIntegrado(nn string) string { return "refs/orq/integrado/" + nn }
func refWorker(nn, nombre string) string {
	return "refs/orq/s" + nn + "/" + strings.NewReplacer("/", "-", " ", "-").Replace(nombre)
}

// ErrSinContrato: el slice lo abrió una versión que no guardaba el contrato en git (orq 1.0.0).
var ErrSinContrato = errors.New("slice sin contrato en git")

// ErrEstadoForjado: el estado en disco no coincide con lo que orq guardó en git.
var ErrEstadoForjado = errors.New("el estado no coincide con el contrato")

// guardarBlob escribe v como objeto de git y apunta ref a él.
func (r Repo) guardarBlob(ref string, v any) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	sha, err := gitx.ConEntrada(context.Background(), r.Root, raw, "hash-object", "-w", "--stdin")
	if err != nil {
		return err
	}
	_, err = r.git("update-ref", ref, sha)
	return err
}

// leerBlob lee el objeto de ref en v. existe=false si la referencia no está.
func (r Repo) leerBlob(ref string, v any) (existe bool, err error) {
	if _, code, err := r.gitCodigo("rev-parse", "-q", "--verify", ref); err != nil {
		return false, err
	} else if code != 0 {
		return false, nil
	}
	raw, err := r.git("cat-file", "blob", ref)
	if err != nil {
		return true, err
	}
	return true, json.Unmarshal([]byte(raw), v)
}

func (r Repo) borrarRef(ref string) error {
	if _, code, err := r.gitCodigo("rev-parse", "-q", "--verify", ref); err != nil || code != 0 {
		return err
	}
	_, err := r.git("update-ref", "-d", ref)
	return err
}

// sellar guarda en git el contrato del estado. Lo llaman solo los comandos que tienen derecho a
// cambiarlo: abrir, sumar examen, reparar huella, ampliar alcance, convertir.
func (r Repo) sellar(e *Estado) error { return r.guardarBlob(refContrato(e.NN), contratoDe(e)) }

// confiable comprueba que el estado dice lo mismo que el contrato guardado en git.
func (r Repo) confiable(e *Estado) error {
	var c Contrato
	existe, err := r.leerBlob(refContrato(e.NN), &c)
	if err != nil {
		return fmt.Errorf("no se pudo leer el contrato de %s: %w", e.ID(), err)
	}
	if !existe {
		return fmt.Errorf("%w: %s was opened by an older orq. It is not adopted on its own: stop its old processes and convert it: `orq slice convertir %s --ratifico \"<who and why>\"`", ErrSinContrato, e.ID(), e.NN)
	}
	if dif := diferencias(c, contratoDe(e)); len(dif) > 0 {
		return fmt.Errorf("%w: %s dice otra cosa que el contrato que orq guardó en git (%s) en: %s. Alguien reescribió .orq/slices/%s.json; no se acepta nada con ese estado. El contrato vigente: cmd=%q, examen=%s",
			ErrEstadoForjado, e.ID(), refContrato(e.NN), strings.Join(dif, ", "), e.NN, c.Cmd, strings.Join(c.Bundle, ","))
	}
	return nil
}

// diferencias lista los campos del contrato que no coinciden.
func diferencias(a, b Contrato) []string {
	var out []string
	va, vb := reflect.ValueOf(a), reflect.ValueOf(b)
	for i := 0; i < va.NumField(); i++ {
		x, y := va.Field(i).Interface(), vb.Field(i).Interface()
		if reflect.ValueOf(x).Kind() == reflect.Slice && reflect.ValueOf(x).Len() == 0 && reflect.ValueOf(y).Len() == 0 {
			continue // nil y vacío son lo mismo
		}
		if !reflect.DeepEqual(x, y) {
			out = append(out, strings.Split(va.Type().Field(i).Tag.Get("json"), ",")[0])
		}
	}
	return out
}

// ErrReciboNoEmitido: el recibo del estado no es el que orq dejó en git al verificar.
var ErrReciboNoEmitido = errors.New("el recibo no es el que orq emitió")

// sellarRecibo guarda el recibo vigente en git; nil lo borra.
func (r Repo) sellarRecibo(nn string, rec *Recibo) error {
	if rec == nil {
		return r.borrarRef(refRecibo(nn))
	}
	return r.guardarBlob(refRecibo(nn), rec)
}

// reciboConfiable devuelve el recibo que orq emitió para el intento vigente, leído de git.
// El del estado solo vale si es ese mismo.
func (r Repo) reciboConfiable(e *Estado) (*Recibo, error) {
	var rec Recibo
	existe, err := r.leerBlob(refRecibo(e.NN), &rec)
	if err != nil {
		return nil, err
	}
	switch {
	case !existe:
		return nil, fmt.Errorf("%w: %s no tiene un recibo emitido por orq. Corre `orq slice verificar`", ErrReciboNoEmitido, e.ID())
	case rec.Resultado != "paso" || rec.Attempt != e.Attempt() || rec.Bundle != e.Huella:
		return nil, fmt.Errorf("%w: el último recibo de orq para %s es %s (%s); el intento vigente es %s. Corre `orq slice verificar`", ErrReciboNoEmitido, e.ID(), rec.Attempt, rec.Resultado, e.Attempt())
	case e.Recibo == nil || e.Recibo.Head != rec.Head || e.Recibo.Attempt != rec.Attempt || e.Recibo.Resultado != rec.Resultado:
		return nil, fmt.Errorf("%w: el estado de %s trae un recibo distinto del que orq guardó en git (%s). Corre `orq slice verificar`", ErrReciboNoEmitido, e.ID(), refRecibo(e.NN))
	}
	return &rec, nil
}

// integradoConfiable dice si sha es el merge que orq revalidó (lo anotó en git tras pasar el examen).
func (r Repo) integradoConfiable(nn, sha string) (bool, error) {
	out, code, err := r.gitCodigo("rev-parse", "-q", "--verify", refIntegrado(nn))
	if err != nil {
		return false, err
	}
	return code == 0 && sha != "" && out == sha, nil
}

// borrarRefs quita las referencias de orq de un slice cerrado.
func (r Repo) borrarRefs(e *Estado) error {
	errs := []error{r.borrarRef(refContrato(e.NN)), r.borrarRef(refRecibo(e.NN)), r.borrarRef(refIntegrado(e.NN))}
	out, err := r.git("for-each-ref", "--format=%(refname)", "refs/orq/s"+e.NN+"/")
	errs = append(errs, err)
	for _, ref := range strings.Fields(out) {
		errs = append(errs, r.borrarRef(ref))
	}
	return errors.Join(errs...)
}

// ProcedenciaLegado marks a slice opened by orq 1.0.0 and adopted by this version.
const ProcedenciaLegado = "legado"

// ErrProcesosVivos: another process holds the slice lock, so the 1.0.0 processes are still running.
var ErrProcesosVivos = errors.New("the slice lock is held by another process")

// Convertir adopts a slice opened by orq 1.0.0 (schema 0, no contract in git). Only this command
// adopts, never verify, and only with a ratification saying who approved it and why, which goes into
// the contract. The base is the trusted one (baseConfiable), never the state's; the state must name it.
// The red and exam commits must be full SHAs of commits on that branch of the main repo, and the exam hash in the state must be the hash of the exam at
// that commit. It refuses while the old processes may still run: the slice lock held, the writer
// lease held, or a process with its cwd inside the worktree. It shows cmd, bundle and commits on out
// before sealing.
func (r Repo) Convertir(nn, ratifico string, out io.Writer) (*Estado, error) {
	ratifico = strings.TrimSpace(ratifico)
	if ratifico == "" {
		return nil, errors.New("not converted: --ratifico \"<who approved it and why>\" is required")
	}
	e, err := r.Load(nn)
	if err != nil || e == nil {
		return nil, errors.Join(err, errors.New("no hay slice abierto"))
	}
	soltar, err := r.candadoLibre("slice-" + e.NN)
	if err != nil {
		return e, err
	}
	defer soltar()
	if e, err = r.Load(e.NN); err != nil || e == nil {
		return nil, errors.Join(err, errors.New("no hay slice abierto"))
	}
	if hay, err := r.leerBlob(refContrato(e.NN), &Contrato{}); err != nil {
		return e, err
	} else if hay {
		return e, fmt.Errorf("%s already has a contract in git (%s): nothing to convert", e.ID(), refContrato(e.NN))
	}
	if e.Esquema >= EsquemaActual {
		return e, fmt.Errorf("%s was opened by this version and its contract is missing from git: it is not converted, close it as rejected", e.ID())
	}
	if e.Escritor != "" {
		return e, fmt.Errorf("%s is not converted: the writer lease is held by %s, so a worker may still be running. Stop it first", e.ID(), e.Escritor)
	}
	if usan := EnUso(e.Worktree); len(usan) > 0 {
		return e, fmt.Errorf("%s is not converted: processes still work inside %s: %s. Stop them first", e.ID(), e.Worktree, strings.Join(usan, ", "))
	}
	base, err := r.baseConfiable(e)
	if err != nil {
		return e, err
	}
	if e.Base != base {
		return e, fmt.Errorf("%s is not converted: the state says base %q, but the trusted base is %q (project config `base`, else main, else master)", e.ID(), e.Base, base)
	}
	if err := r.enLaBase(e, base, "red", e.SHARojo); err != nil {
		return e, err
	}
	if e.SHAExamen != "" {
		if err := r.enLaBase(e, base, "exam", e.SHAExamen); err != nil {
			return e, err
		}
	}
	h, err := HuellaEnCommit(r.Root, e.baseExamen(), e.Bundle, e.Cmd)
	if err != nil {
		return e, err
	}
	if h != e.Huella {
		return e, fmt.Errorf("%s is not converted: the exam hash in the state does not match the exam at commit %s. Someone changed the state or the exam", e.ID(), e.baseExamen())
	}
	if out != nil {
		fmt.Fprintf(out, "%s: sealing the contract from the state\n  cmd: %s\n  bundle: %s\n  red commit: %s\n", e.Attempt(), e.Cmd, strings.Join(e.Bundle, ", "), e.SHARojo)
		if e.SHAExamen != "" {
			fmt.Fprintf(out, "  exam commit: %s\n", e.SHAExamen)
		}
		fmt.Fprintf(out, "  ratified: %s\n", ratifico)
	}
	e.Procedencia, e.Ratifico = ProcedenciaLegado, ratifico
	if err := r.sellar(e); err != nil {
		return e, err
	}
	return e, errors.Join(r.save(e), r.marcarSiClon(e))
}

// enLaBase checks that sha is a full commit SHA (40 hex, not a ref name or a prefix) and that it is
// on the base branch of the main repo.
func (r Repo) enLaBase(e *Estado, rama, que, sha string) error {
	if !esSHACompleto(sha) {
		return fmt.Errorf("%s is not converted: the %s commit in the state (%q) is not a full SHA", e.ID(), que, sha)
	}
	if got, code, err := r.gitCodigo("rev-parse", "-q", "--verify", sha+"^{commit}"); err != nil {
		return err
	} else if code != 0 || got != sha {
		return fmt.Errorf("%s is not converted: the %s commit %s is not a commit of the main repo", e.ID(), que, sha)
	}
	base, code, err := r.gitCodigo("rev-parse", "-q", "--verify", "refs/heads/"+rama+"^{commit}")
	if err != nil {
		return err
	} else if code != 0 || !esSHACompleto(base) {
		return fmt.Errorf("%s is not converted: the base branch %q does not exist in the main repo", e.ID(), rama)
	}
	if ok, err := r.esAncestro(sha, base); err != nil {
		return err
	} else if !ok {
		return fmt.Errorf("%s is not converted: the %s commit %s is not on the base branch %s of the main repo", e.ID(), que, sha, rama)
	}
	return nil
}

func esSHACompleto(s string) bool {
	if len(s) != 40 {
		return false
	}
	for _, c := range s {
		if !('0' <= c && c <= '9' || 'a' <= c && c <= 'f') {
			return false
		}
	}
	return true
}

// Corto is the short form of a SHA for messages; it never panics on a short or empty value.
func Corto(sha string) string { return sha[:min(8, len(sha))] }

// marcarSiClon is a no-op for a linked worktree: there is no clone to mark.
func (r Repo) marcarSiClon(e *Estado) error {
	if esClon(e.Worktree) {
		return r.marcar(e, e.Worktree, e.Rama)
	}
	return nil
}

// baseConfiable is the base branch a 1.0.0 slice is checked against, never read from the state:
// the project config's `base` if it defines one, else main if it exists, else master. A slice/*
// branch or the slice's own branch is never trusted.
func (r Repo) baseConfiable(e *Estado) (string, error) {
	c, err := config.CargarDe(config.RutaProyecto(r.Root))
	if err != nil {
		return "", err
	}
	base := c.Base
	if base == "" {
		base = "master"
		if _, code, err := r.gitCodigo("rev-parse", "-q", "--verify", "refs/heads/main^{commit}"); err != nil {
			return "", err
		} else if code == 0 {
			base = "main"
		}
	}
	if strings.HasPrefix(base, "slice/") || base == e.Rama {
		return "", fmt.Errorf("%s is not converted: the trusted base %q is a slice branch", e.ID(), base)
	}
	if got, code, err := r.gitCodigo("rev-parse", "-q", "--verify", "refs/heads/"+base+"^{commit}"); err != nil {
		return "", err
	} else if code != 0 || !esSHACompleto(got) {
		return "", fmt.Errorf("%s is not converted: the trusted base branch %q does not exist in the main repo", e.ID(), base)
	}
	return base, nil
}
