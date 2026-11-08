//go:build !windows

package orqdir

import (
	"os"
	"syscall"
)

func Bloquear(f *os.File) error { return syscall.Flock(int(f.Fd()), syscall.LOCK_EX) }
func Desbloquear(f *os.File)    { syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }

// GrupoPropio: el hijo lanza su propio grupo de procesos (MatarGrupo lo mata entero).
func GrupoPropio() *syscall.SysProcAttr { return &syscall.SysProcAttr{Setpgid: true} }

// MatarGrupo manda SIGKILL al grupo de procesos de pid.
func MatarGrupo(pid int) error { return syscall.Kill(-pid, syscall.SIGKILL) }

// PIDVivo: kill(pid, 0) sin enviar señal (EPERM también es vivo).
func PIDVivo(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || err == syscall.EPERM
}

// MatarGrupoTerm manda SIGTERM al grupo de procesos de pid.
func MatarGrupoTerm(pid int) { syscall.Kill(-pid, syscall.SIGTERM) }
