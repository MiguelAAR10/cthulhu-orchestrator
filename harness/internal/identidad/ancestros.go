package identidad

import (
	"bytes"
	"os"
	"runtime"
	"strconv"
	"strings"
)

// Procesos lee la tabla de procesos. En Linux sale de /proc; las pruebas la reemplazan.
// Environ devuelve el entorno INICIAL de un proceso (/proc/<pid>/environ): el que recibió en exec.
// Un hijo no puede reescribir el de sus ancestros, así que la marca de worker que orq puso al lanzar
// el agente sigue ahí aunque el hijo haga unset/export de ORQ_*.
type Procesos interface {
	PPID(pid int) (int, error)
	Environ(pid int) (map[string]string, error)
}

// ProcFS lee /proc (solo Linux).
type ProcFS struct{ Root string }

func (p ProcFS) root() string {
	if p.Root != "" {
		return p.Root
	}
	return "/proc"
}

func (p ProcFS) PPID(pid int) (int, error) {
	raw, err := os.ReadFile(p.root() + "/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return 0, err
	}
	// pid (comm) state ppid ...; comm puede tener espacios y paréntesis: se corta en el último ')'.
	s := string(raw)
	i := strings.LastIndexByte(s, ')')
	f := strings.Fields(s[i+1:])
	if i < 0 || len(f) < 2 {
		return 0, os.ErrInvalid
	}
	return strconv.Atoi(f[1])
}

func (p ProcFS) Environ(pid int) (map[string]string, error) {
	raw, err := os.ReadFile(p.root() + "/" + strconv.Itoa(pid) + "/environ")
	if err != nil {
		return nil, err
	}
	m := map[string]string{}
	for _, kv := range bytes.Split(raw, []byte{0}) {
		if k, v, ok := strings.Cut(string(kv), "="); ok && strings.HasPrefix(k, "ORQ_") {
			m[k] = v
		}
	}
	return m, nil
}

// Tabla es la fuente de procesos; nil fuera de Linux (sin /proc no hay comprobación de ancestros:
// queda el entorno propio, el shim y el gate; ver docs/v2/specs/identity.md).
var Tabla Procesos = func() Procesos {
	if runtime.GOOS == "linux" {
		return ProcFS{}
	}
	return nil
}()

// rolesNoWorker: paneles de sesión que orq abre y que no son workers.
var rolesNoWorker = map[string]bool{"": true, "owner": true, "atena": true, "orquestador": true, "orchestrator": true,
	"planificador": true, "planner": true, "astra": true}

// MarcaWorker dice si un entorno (inicial) es el de un worker: ORQ_WORKER_ID, el shim o un rol de worker.
func MarcaWorker(env map[string]string) (string, bool) {
	if id := env["ORQ_WORKER_ID"]; id != "" {
		return id, true
	}
	if env["ORQ_SHIM"] != "" {
		return "shim", true
	}
	if r := env["ORQ_ROL"]; !rolesNoWorker[r] {
		return r, true
	}
	return "", false
}

// WorkerAncestro recorre la cadena de padres desde pid (excluido) hasta init y devuelve el id del
// primer ancestro con marca de worker. Un ancestro ilegible (otro usuario, ya muerto) corta la subida.
func WorkerAncestro(t Procesos, pid int) (string, bool) {
	if t == nil {
		return "", false
	}
	vistos := map[int]bool{}
	for i := 0; i < 256; i++ {
		pp, err := t.PPID(pid)
		if err != nil || pp <= 0 || vistos[pp] {
			return "", false
		}
		vistos[pp] = true
		if env, err := t.Environ(pp); err == nil {
			if id, ok := MarcaWorker(env); ok {
				return id, true
			}
		}
		if pp == 1 {
			return "", false
		}
		pid = pp
	}
	return "", false
}

// WorkerHeredado: este proceso desciende de un worker que orq lanzó.
func WorkerHeredado() (string, bool) { return WorkerAncestro(Tabla, os.Getpid()) }
