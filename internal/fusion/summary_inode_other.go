//go:build !unix

package fusion

import "os"

// inodeOf has no inode to report on this platform.
func inodeOf(os.FileInfo) uint64 { return 0 }
