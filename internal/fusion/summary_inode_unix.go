//go:build unix

package fusion

import (
	"os"
	"syscall"
)

// inodeOf is the file's inode number (0 when the platform has none).
func inodeOf(fi os.FileInfo) uint64 {
	if st, ok := fi.Sys().(*syscall.Stat_t); ok {
		return uint64(st.Ino) //nolint:unconvert // Ino is uint64 on darwin, uint64 on linux
	}
	return 0
}
