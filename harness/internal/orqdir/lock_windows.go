//go:build windows

package orqdir

import (
	"errors"
	"os"
	"syscall"
	"time"
)

// Windows sin flock: un archivo hermano creado con O_EXCL hace de candado. Si un proceso murió
// con el candado tomado, a los 30 s se considera abandonado. Espera hasta 30 s.
func Bloquear(f *os.File) error {
	marca := f.Name() + ".excl"
	limite := time.Now().Add(30 * time.Second)
	for {
		m, err := os.OpenFile(marca, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			return m.Close()
		}
		if !errors.Is(err, os.ErrExist) {
			return err
		}
		if info, e := os.Stat(marca); e == nil && time.Since(info.ModTime()) > 30*time.Second {
			os.Remove(marca)
			continue
		}
		if time.Now().After(limite) {
			return errors.New("orq: lock busy: " + marca)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func Desbloquear(f *os.File) { os.Remove(f.Name() + ".excl") }

// GrupoPropio: en Windows, un grupo de consola nuevo (no hay grupos POSIX).
func GrupoPropio() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
}

// MatarGrupo: en Windows mata solo el proceso raíz (los nietos pueden sobrevivir).
func MatarGrupo(pid int) error {
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return p.Kill()
}

// PIDVivo: OpenProcess + GetExitCodeProcess == STILL_ACTIVE.
func PIDVivo(pid int) bool {
	h, err := syscall.OpenProcess(syscall.PROCESS_QUERY_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	defer syscall.CloseHandle(h)
	var code uint32
	return syscall.GetExitCodeProcess(h, &code) == nil && code == 259
}

// MatarGrupoTerm: en Windows no hay SIGTERM; mata el proceso raíz.
func MatarGrupoTerm(pid int) { MatarGrupo(pid) }
