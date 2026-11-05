//go:build unix

package lang

import (
	"os"
	"syscall"
)

// ownedByOther reports whether another user owns fi. The cache holds machine
// code the process executes, so a directory that user owns is one they can
// rewrite whatever its mode says.
func ownedByOther(fi os.FileInfo) bool {
	st, ok := fi.Sys().(*syscall.Stat_t)
	return ok && int(st.Uid) != os.Getuid()
}
