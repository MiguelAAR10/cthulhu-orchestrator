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
	"reflect"
	"strings"

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
	Tarjeta    *Tarjeta   `json:"tarjeta,omitempty"`
	// PruebasExamen va en el contrato: un estado reescrito no puede vaciar la lista.
	PruebasExamen []string `json:"pruebas_examen,omitempty"`
}

func contratoDe(e *Estado) Contrato {
	return Contrato{NN: e.NN, Test: e.Test, Bundle: e.Bundle, Cmd: e.Cmd, Sano: e.Sano, Base: e.Base, Rama: e.Rama,
		SHARojo: e.SHARojo, SHAExamen: e.SHAExamen, Huella: e.Huella, Reporte: e.Reporte, Formato: e.Formato,
		Omitir: e.Omitir, SinConteo: e.SinConteo, TimeoutSeg: e.TimeoutSeg, Vigilados: e.Vigilados, Alcance: e.Alcance,
		PruebasExamen: e.PruebasExamen, Tarjeta: e.Tarjeta}
}

func refContrato(nn string) string  { return "refs/orq/contrato/" + nn }
func refRecibo(nn string) string    { return "refs/orq/recibo/" + nn }
func refIntegrado(nn string) string { return "refs/orq/integrado/" + nn }
func refWorker(nn, nombre string) string {
	return "refs/orq/s" + nn + "/" + strings.NewReplacer("/", "-", " ", "-").Replace(nombre)
}

// ErrSinContrato: a state of this version whose contract is not in git. It is never trusted.
var ErrSinContrato = errors.New("slice sin contrato en git")

// ErrOrq1: the slice was opened by orq 1.0.0 (schema 0). This version does not inherit it.
var ErrOrq1 = errors.New("opened by orq 1.0.0")

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
// cambiarlo: abrir, sumar examen, reparar huella, ampliar alcance.
func (r Repo) sellar(e *Estado) error { return r.guardarBlob(refContrato(e.NN), contratoDe(e)) }

// confiable comprueba que el estado dice lo mismo que el contrato guardado en git.
func (r Repo) confiable(e *Estado) error {
	var c Contrato
	existe, err := r.leerBlob(refContrato(e.NN), &c)
	if err != nil {
		return fmt.Errorf("no se pudo leer el contrato de %s: %w", e.ID(), err)
	}
	if !existe {
		return fmt.Errorf("%w: %s has no contract in git (%s), so nothing is accepted with its state; close it as rejected", ErrSinContrato, e.ID(), refContrato(e.NN))
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

// Corto is the short form of a SHA for messages; it never panics on a short or empty value.
func Corto(sha string) string { return sha[:min(8, len(sha))] }
