//go:build unix

package connector

import (
	"os"
	"syscall"
)

// inoOf is fi's inode number.
func inoOf(fi os.FileInfo) uint64 {
	if sys, ok := fi.Sys().(*syscall.Stat_t); ok {
		return uint64(sys.Ino)
	}
	return 0
}
