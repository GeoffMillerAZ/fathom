//go:build !unix

package lang

import "os"

// ownedByOther is false where file ownership is not a user id; the writable
// check alone guards the cache directory there.
func ownedByOther(os.FileInfo) bool { return false }
