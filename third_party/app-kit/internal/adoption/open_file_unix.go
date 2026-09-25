//go:build unix

package adoption

import (
	"os"
	"syscall"
)

// A path may be swapped after Lstat. Nonblocking open prevents a substituted
// FIFO from waiting for a writer; the caller rejects nonregular descriptors
// immediately with f.Stat, before reading any bytes.
func openAdoptionFile(root *os.Root, path string) (*os.File, error) {
	return root.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
}
