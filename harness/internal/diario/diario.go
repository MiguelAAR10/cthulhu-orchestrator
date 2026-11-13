// Package diario es el registro de lo que orq hizo: cada lanzamiento y cada cambio de fase,
// una línea JSON por evento, solo se agrega.
//
// Sirve para tres cosas: repetir un comando sin repetir su efecto (un intento sin resultado es
// «desconocido» y se resuelve antes de reintentar), saber qué pasó tras una interrupción, y medir.
package diario

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// Evento es una línea del diario.
type Evento struct {
	TS        time.Time      `json:"ts"`
	Op        string         `json:"op,omitempty"`        // une los pasos de una misma operación
	Tipo      string         `json:"tipo"`                // lanzamiento | fase | sesion | verificacion | mensaje
	Paso      string         `json:"paso,omitempty"`      // intento | panel | arranque | contrato | ...
	Slice     string         `json:"slice,omitempty"`     // NN
	Intento   int            `json:"intento,omitempty"`   // intento del slice
	Ruta      string         `json:"ruta,omitempty"`      // fix | slice
	Actor     string         `json:"actor,omitempty"`     // rol o worker al que se refiere
	De        string         `json:"de,omitempty"`        // fase anterior
	A         string         `json:"a,omitempty"`         // fase nueva
	Resultado string         `json:"resultado,omitempty"` // ok | fallo | desconocido | ya_estaba
	Detalle   string         `json:"detalle,omitempty"`
	Datos     map[string]any `json:"datos,omitempty"`
}

// Ruta del diario dentro de .orq.
func Ruta(orqDir string) string { return filepath.Join(orqDir, "diario.jsonl") }

// NuevaOp devuelve un identificador de operación.
func NuevaOp() string {
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		return hex.EncodeToString([]byte(time.Now().Format("150405.000000")))
	}
	return hex.EncodeToString(b[:])
}

// Anotar agrega el evento bajo candado y lo deja en disco antes de volver.
func Anotar(path string, e Evento) error {
	if e.TS.IsZero() {
		e.TS = time.Now()
	}
	e.TS = e.TS.UTC().Truncate(time.Millisecond)
	line, err := json.Marshal(e)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	// Si la última escritura quedó cortada, este evento empieza en su propia línea y no se pierde con ella.
	if fi, err := f.Stat(); err == nil && fi.Size() > 0 {
		var ult [1]byte
		if _, err := f.ReadAt(ult[:], fi.Size()-1); err == nil && ult[0] != '\n' {
			line = append([]byte{'\n'}, line...)
		}
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		return err
	}
	return f.Sync()
}

// Leer devuelve los eventos en orden. Una línea ilegible (escritura cortada) se salta.
func Leer(path string) ([]Evento, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Evento
	sc := bufio.NewScanner(bytes.NewReader(raw))
	sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for sc.Scan() {
		var e Evento
		if json.Unmarshal(sc.Bytes(), &e) == nil && e.Tipo != "" {
			out = append(out, e)
		}
	}
	return out, sc.Err()
}

// Ultimo devuelve el último evento que cumple el filtro.
func Ultimo(evs []Evento, sirve func(Evento) bool) (Evento, bool) {
	for i := len(evs) - 1; i >= 0; i-- {
		if sirve(evs[i]) {
			return evs[i], true
		}
	}
	return Evento{}, false
}
