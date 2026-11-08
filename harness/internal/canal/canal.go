// Package canal registra los mensajes entre sesiones (Atena y Orquestador) y
// recuerda qué agente de herdr ocupa cada rol.
package canal

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"syscall"
	"time"

	"orq/internal/orqdir"
)

var (
	Roles = []string{"atena", "orquestador", "planificador", "astra"}
	Tipos = []string{"historia", "pregunta", "respuesta", "estado", "cierre", "plan"}
)

const MaxTexto = 4000

type Msg struct {
	ID      string    `json:"id"`
	Attempt string    `json:"attempt,omitempty"` // slice e intento al que se refiere (S-NN/aK)
	Ref     string    `json:"ref,omitempty"`     // id del mensaje al que responde
	Clave   string    `json:"clave,omitempty"`   // idempotencia: el mismo envío no se guarda dos veces
	Estado  string    `json:"estado,omitempty"`  // pendiente|entregado (lo pliega Load)
	TS      time.Time `json:"ts"`
	De      string    `json:"de"`
	Para    string    `json:"para"`
	Tipo    string    `json:"tipo"`
	Texto   string    `json:"texto"`
}

// esRol: filtro, orquestador o un worker del equipo (los workers se hablan entre ellos).
func esRol(r string) bool { return slices.Contains(Roles, r) || strings.HasPrefix(r, "worker-") }

func (m Msg) Validate() error {
	switch {
	case !esRol(m.De), !esRol(m.Para):
		return fmt.Errorf("de/para deben ser %s o un worker (worker-...)", strings.Join(Roles, "|"))
	case m.De == m.Para:
		return errors.New("un rol no se escribe a sí mismo")
	case !slices.Contains(Tipos, m.Tipo):
		return fmt.Errorf("tipo %q: usa %s", m.Tipo, strings.Join(Tipos, "|"))
	case strings.TrimSpace(m.Texto) == "":
		return errors.New("texto vacío")
	case len([]rune(m.Texto)) > MaxTexto:
		return fmt.Errorf("mensaje de %d caracteres; máximo %d: manda lo que el otro necesita para decidir", len([]rune(m.Texto)), MaxTexto)
	}
	return nil
}

// Format es lo que ve la otra sesión en su entrada.
func (m Msg) Format() string {
	ref := m.ID
	if m.Attempt != "" {
		ref += " · " + m.Attempt
	}
	return fmt.Sprintf("[orq · %s de %s · %s] %s\n(Responde con: orq msg send --para %s --tipo <tipo> \"...\")", m.Tipo, m.De, ref, m.Texto, m.De)
}

// ErrDuplicado: ya existe un mensaje con esa clave; se devuelve el original.
var ErrDuplicado = errors.New("mensaje duplicado")

// evento marca un mensaje ya guardado (append-only, nunca se reescribe el canal).
type evento struct {
	ID     string    `json:"id"`
	Evento string    `json:"evento"`
	TS     time.Time `json:"ts"`
}

// Append guarda el mensaje con un id único bajo candado y lo deja pendiente de entrega.
func Append(path string, m *Msg) error {
	if err := m.Validate(); err != nil {
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
	prev, err := Load(path)
	if err != nil {
		return err
	}
	if m.Clave != "" {
		for _, p := range prev {
			if p.Clave == m.Clave {
				*m = p
				return ErrDuplicado
			}
		}
	}
	m.ID, m.Estado = fmt.Sprintf("c%d", len(prev)+1), "pendiente"
	line, _ := json.Marshal(m)
	_, err = f.Write(append(line, '\n'))
	return err
}

// MarcarEntregado agrega el evento de entrega confirmada.
func MarcarEntregado(path, id string, now time.Time) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	line, _ := json.Marshal(evento{ID: id, Evento: "entregado", TS: now.UTC().Truncate(time.Second)})
	_, err = f.Write(append(line, '\n'))
	return err
}

// delSlice: el intento S-NN/aK (o S-NN) pertenece al slice S-NN; S-010 no es S-01.
func delSlice(attempt, id string) bool {
	return attempt == id || strings.HasPrefix(attempt, id+"/")
}

// DeSlice filtra los mensajes de un slice.
func DeSlice(ms []Msg, id string) []Msg {
	var out []Msg
	for _, m := range ms {
		if delSlice(m.Attempt, id) {
			out = append(out, m)
		}
	}
	return out
}

// Vigentes quita lo archivado al cerrar su slice.
func Vigentes(ms []Msg) []Msg {
	var out []Msg
	for _, m := range ms {
		if m.Estado != "archivado" {
			out = append(out, m)
		}
	}
	return out
}

// Archivar marca como archivados los mensajes ya entregados del slice (y los entregados sin
// slice, que son charla resuelta del proyecto). Nunca toca lo pendiente ni borra el canal:
// el archivo es append-only y otros slices siguen usándolo.
func Archivar(path, id string, now time.Time) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	ms, err := Load(path)
	if err != nil {
		return err
	}
	var b bytes.Buffer
	for _, m := range ms {
		if m.Estado == "entregado" && (m.Attempt == "" || delSlice(m.Attempt, id)) {
			line, _ := json.Marshal(evento{ID: m.ID, Evento: "archivado", TS: now.UTC().Truncate(time.Second)})
			b.Write(append(line, '\n'))
		}
	}
	_, err = f.Write(b.Bytes())
	return err
}

// Pendientes son los mensajes guardados cuya entrega no se confirmó.
func Pendientes(ms []Msg) []Msg {
	var out []Msg
	for _, m := range ms {
		if m.Estado == "pendiente" {
			out = append(out, m)
		}
	}
	return out
}

func Load(path string) ([]Msg, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Msg
	sc := bufio.NewScanner(bytes.NewReader(raw))
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	pos := map[string]int{}
	for sc.Scan() {
		var ev evento
		if json.Unmarshal(sc.Bytes(), &ev) == nil && ev.Evento != "" {
			if i, ok := pos[ev.ID]; ok && (ev.Evento == "entregado" || ev.Evento == "archivado") {
				out[i].Estado = ev.Evento
			}
			continue
		}
		var m Msg
		if json.Unmarshal(sc.Bytes(), &m) == nil {
			if m.ID == "" { // canal de v0.3
				m.ID = fmt.Sprintf("c%d", len(out)+1)
			}
			pos[m.ID] = len(out)
			out = append(out, m)
		}
	}
	return out, sc.Err()
}

// Sesion es el agente de herdr que ocupa un rol.
type Sesion struct {
	Agente string    `json:"agente"`
	Pane   string    `json:"pane"`
	Desde  time.Time `json:"desde"`
	Gen    int       `json:"gen"`
	Tab    string    `json:"tab,omitempty"` // pestaña tipada de herdr donde vive
}

func LoadSesiones(path string) (map[string]Sesion, error) {
	s := map[string]Sesion{}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	return s, json.Unmarshal(raw, &s)
}

func SaveSesiones(path string, s map[string]Sesion) error {
	raw, _ := json.MarshalIndent(s, "", "  ")
	return orqdir.WriteAtomic(path, raw)
}

// RolDePane identifica quién escribe a partir del panel de herdr.
func RolDePane(s map[string]Sesion, pane string) string {
	for rol, ses := range s {
		if pane != "" && ses.Pane == pane {
			return rol
		}
	}
	return ""
}
