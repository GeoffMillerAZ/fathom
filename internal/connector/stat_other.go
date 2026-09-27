//go:build !unix

package connector

import "os"

// inoOf is 0 where the platform exposes no inode: the stat gate then
// compares size and mtime only.
func inoOf(os.FileInfo) uint64 { return 0 }
