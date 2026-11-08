//go:build !windows

package sysinfo

import (
	"io/fs"
	"syscall"
)

// FreeGB devuelve los GB disponibles para el usuario en el sistema de archivos de path.
func FreeGB(path string) (float64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, err
	}
	return float64(st.Bavail) * float64(st.Bsize) / 1e9, nil
}

// enlaceDuro: inodo de un archivo con más de un enlace (para no contarlo dos veces).
func enlaceDuro(info fs.FileInfo) (uint64, bool) {
	if st, ok := info.Sys().(*syscall.Stat_t); ok && st.Nlink > 1 {
		return uint64(st.Ino), true
	}
	return 0, false
}
