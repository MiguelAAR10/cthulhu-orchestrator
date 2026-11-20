package sysinfo

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// serialSize is DirSizeBytes as it was before P3: one serial walk, hardlinks counted once.
func serialSize(root string) int64 {
	seen := map[uint64]bool{}
	var total int64
	filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || !d.Type().IsRegular() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if st, ok := info.Sys().(*syscall.Stat_t); ok && st.Nlink > 1 {
			if seen[st.Ino] {
				return nil
			}
			seen[st.Ino] = true
		}
		total += info.Size()
		return nil
	})
	return total
}

func TestDirSizeBytesParaleloIgualAlSerial(t *testing.T) {
	root := t.TempDir()
	escribir := func(rel string, n int) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(strings.Repeat("x", n)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for i, c := range "abcdefghijklmnopqrst" {
		escribir(filepath.Join(string(c), "x", "y", "f"), 100+i)
		escribir(filepath.Join(string(c), "g"), 7)
	}
	escribir("suelto", 1000)
	escribir("a/compartido", 5000)
	// The same inode in two top-level children: counted once.
	if err := os.Link(filepath.Join(root, "a", "compartido"), filepath.Join(root, "b", "enlace")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "suelto"), filepath.Join(root, "c", "sym")); err != nil {
		t.Fatal(err)
	}
	want := serialSize(root)
	for range 20 {
		got, err := DirSizeBytes(root)
		if err != nil || got != want {
			t.Fatalf("DirSizeBytes = %d, %v; serial %d", got, err, want)
		}
	}
	// A file as root and a missing root behave as before.
	f := filepath.Join(root, "suelto")
	if got, _ := DirSizeBytes(f); got != serialSize(f) {
		t.Fatalf("file root: %d vs %d", got, serialSize(f))
	}
}
