package skilleval

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Nota es una corrida de evaluación de una versión de una skill para un tipo de tarea.
type Nota struct {
	Skill    string    `json:"skill"`
	Hash     string    `json:"hash"`
	Tipo     string    `json:"tipo"`
	Nota     float64   `json:"nota"`
	Corridas int       `json:"corridas"`
	CostoUSD float64   `json:"costo_usd"`
	Claude   string    `json:"claude"`
	Parcial  bool      `json:"parcial,omitempty"`
	Fecha    time.Time `json:"fecha"`
	Casos    []Caso    `json:"casos,omitempty"`
}

// RutaNotas es el registro de notas dentro del estado de orq.
func RutaNotas(dir string) string { return filepath.Join(dir, "skill-notas.jsonl") }

// Guardar suma una fila al registro.
func Guardar(dir string, n Nota) error {
	raw, err := json.Marshal(n)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(RutaNotas(dir), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, err = f.Write(append(raw, '\n'))
	return errors.Join(err, f.Close())
}

// Notas lee el registro en orden de escritura; sin archivo no hay notas.
func Notas(dir string) ([]Nota, error) {
	f, err := os.Open(RutaNotas(dir))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var ns []Nota
	sc := bufio.NewScanner(f)
	for i := 1; sc.Scan(); i++ {
		var n Nota
		if err := json.Unmarshal(sc.Bytes(), &n); err != nil {
			return nil, fmt.Errorf("%s:%d: %w", RutaNotas(dir), i, err)
		}
		ns = append(ns, n)
	}
	return ns, sc.Err()
}

// Listar ordena por skill; dentro de cada skill, primero el hash con la corrida más nueva, y
// dentro de cada hash, de la más nueva a la más vieja. Con tipo, solo ese tipo.
func Listar(ns []Nota, tipo string) []Nota {
	var out []Nota
	ultima := map[string]time.Time{}
	for _, n := range ns {
		if tipo != "" && n.Tipo != tipo {
			continue
		}
		out = append(out, n)
		if k := n.Skill + "\x00" + n.Hash; n.Fecha.After(ultima[k]) {
			ultima[k] = n.Fecha
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Skill != b.Skill {
			return a.Skill < b.Skill
		}
		if a.Hash != b.Hash {
			return ultima[a.Skill+"\x00"+a.Hash].After(ultima[b.Skill+"\x00"+b.Hash])
		}
		return a.Fecha.After(b.Fecha)
	})
	return out
}
