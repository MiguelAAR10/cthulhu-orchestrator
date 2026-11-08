//go:build windows

package sysinfo

import (
	"io/fs"
	"syscall"
	"unsafe"
)

var getDiskFreeSpaceEx = syscall.NewLazyDLL("kernel32.dll").NewProc("GetDiskFreeSpaceExW")

// FreeGB devuelve los GB disponibles para el usuario en el volumen de path (GetDiskFreeSpaceExW).
func FreeGB(path string) (float64, error) {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	var libres, total, totalLibres uint64
	r, _, e := getDiskFreeSpaceEx.Call(uintptr(unsafe.Pointer(p)), uintptr(unsafe.Pointer(&libres)), uintptr(unsafe.Pointer(&total)), uintptr(unsafe.Pointer(&totalLibres)))
	if r == 0 {
		return 0, e
	}
	return float64(libres) / 1e9, nil
}

// enlaceDuro: en Windows no se deduplican enlaces duros (se cuentan una vez por ruta).
func enlaceDuro(fs.FileInfo) (uint64, bool) { return 0, false }
