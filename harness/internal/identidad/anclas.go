package identidad

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Info es lo que orq necesita de un proceso para anclar identidad: padre, arranque y TTY.
// Start es el campo 22 de /proc/<pid>/stat (ticks desde el arranque): pid+Start identifica un
// proceso sin ambigüedad aunque el pid se recicle. TTY es tty_nr (campo 7): 0 sin terminal.
type Info struct {
	PPID  int
	Start uint64
	TTY   int
}

// ConInfo: tablas de procesos que saben dar Info (ProcFS; las pruebas usan tablas falsas).
type ConInfo interface {
	Info(pid int) (Info, error)
}

func (p ProcFS) Info(pid int) (Info, error) {
	raw, err := os.ReadFile(p.root() + "/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return Info{}, err
	}
	s := string(raw)
	i := strings.LastIndexByte(s, ')')
	if i < 0 {
		return Info{}, os.ErrInvalid
	}
	// Tras ')': [0]=state(3) [1]=ppid(4) [2]=pgrp [3]=session [4]=tty_nr(7) ... [19]=starttime(22)
	f := strings.Fields(s[i+1:])
	if len(f) < 20 {
		return Info{}, os.ErrInvalid
	}
	pp, err1 := strconv.Atoi(f[1])
	tty, err2 := strconv.Atoi(f[4])
	st, err3 := strconv.ParseUint(f[19], 10, 64)
	if err := errors.Join(err1, err2, err3); err != nil {
		return Info{}, err
	}
	return Info{PPID: pp, Start: st, TTY: tty}, nil
}

// TerminalDeUsuario: tty_nr de una terminal real de persona: un pseudo-terminal (/dev/pts, mayor
// 136..143) o una consola virtual (/dev/ttyN, mayor 4). Las consolas de sistema (hvc, ttyS) no
// cuentan: en WSL el init las tiene y un huérfano reparentado no debe heredarlas.
func TerminalDeUsuario(ttyNr int) bool {
	if ttyNr == 0 {
		return false
	}
	major := (ttyNr >> 8) & 0xfff
	return major == 4 || (major >= 136 && major <= 143)
}

// Ancla es una sesión privilegiada registrada: un proceso vivo (pid+arranque) con su rol.
type Ancla struct {
	PID   int    `json:"pid"`
	Start uint64 `json:"start"`
	Rol   string `json:"role"`
}

// AnclasDir: las anclas viven junto al token (~/.config/orq/sessions, 0700), fuera del repo y de
// los worktrees: el gate impide a un worker leer o escribir ahí.
func AnclasDir() string { return filepath.Join(Dir(), "sessions") }

// Anclar registra pid como sesión privilegiada (sin comprobaciones: quien llama decide).
func Anclar(t Procesos, pid int, rol string) (Ancla, error) {
	ci, ok := t.(ConInfo)
	if !ok {
		return Ancla{}, errors.New("orq: this platform cannot read process start times; session anchors need Linux /proc")
	}
	in, err := ci.Info(pid)
	if err != nil {
		return Ancla{}, fmt.Errorf("orq: process %d: %w", pid, err)
	}
	if err := os.MkdirAll(AnclasDir(), 0o700); err != nil {
		return Ancla{}, err
	}
	a := Ancla{PID: pid, Start: in.Start, Rol: rol}
	raw, _ := json.Marshal(a)
	nombre := filepath.Join(AnclasDir(), fmt.Sprintf("%d-%d.json", pid, in.Start))
	if err := os.WriteFile(nombre, raw, 0o600); err != nil {
		return Ancla{}, err
	}
	limpiarAnclas(ci)
	return a, nil
}

// Anclas lee las anclas registradas.
func Anclas() []Ancla {
	ents, _ := os.ReadDir(AnclasDir())
	var out []Ancla
	for _, e := range ents {
		raw, err := os.ReadFile(filepath.Join(AnclasDir(), e.Name()))
		var a Ancla
		if err == nil && json.Unmarshal(raw, &a) == nil && a.PID > 0 {
			out = append(out, a)
		}
	}
	return out
}

// limpiarAnclas borra las anclas de procesos que ya no existen (o cuyo pid se recicló).
func limpiarAnclas(ci ConInfo) {
	for _, a := range Anclas() {
		if in, err := ci.Info(a.PID); err != nil || in.Start != a.Start {
			os.Remove(filepath.Join(AnclasDir(), fmt.Sprintf("%d-%d.json", a.PID, a.Start)))
		}
	}
}

// Confiable decide si el proceso pid puede ejercer un rol privilegiado (además del token):
//   - ningún ancestro es un worker (entorno inicial con marca de worker), y
//   - pid o un ancestro coincide con un ancla registrada (pid + arranque), o
//   - pid o un ancestro tiene una terminal de persona (pts o consola virtual).
//
// Un proceso que se separó de un worker (doble fork + setsid) queda reparentado a init o a un
// subreaper sin terminal: no tiene ancla ni TTY en su cadena y se rechaza.
// Sin tabla de procesos (fuera de Linux) devuelve nil: solo queda el token (ver identity.md).
func Confiable(t Procesos, pid int) error {
	if t == nil {
		return nil
	}
	if id, ok := WorkerAncestro(t, pid); ok {
		return fmt.Errorf("process descends from worker %q", id)
	}
	ci, ok := t.(ConInfo)
	if !ok {
		return nil
	}
	anclas := map[[2]uint64]bool{}
	for _, a := range Anclas() {
		anclas[[2]uint64{uint64(a.PID), a.Start}] = true
	}
	vistos := map[int]bool{}
	for p := pid; p > 1 && !vistos[p] && len(vistos) < 256; {
		vistos[p] = true
		in, err := ci.Info(p)
		if err != nil {
			break
		}
		if anclas[[2]uint64{uint64(p), in.Start}] || TerminalDeUsuario(in.TTY) {
			return nil
		}
		p = in.PPID
	}
	return errors.New("no registered privileged session among this process's ancestors and no user terminal: run `orq session register` from the Owner's terminal")
}

// ConfiableAqui: Confiable para este proceso.
func ConfiableAqui() error { return Confiable(Tabla, os.Getpid()) }
