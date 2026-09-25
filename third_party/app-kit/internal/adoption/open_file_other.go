//go:build !unix

package adoption

import "os"

// Windows has no Unix FIFO filesystem node. Root confinement and the caller's
// pre/post-open regular-file checks still apply; this is not a native-host proof.
func openAdoptionFile(root *os.Root, path string) (*os.File, error) {
	return root.Open(path)
}
