package dlog

import "os"

// RotateIfNeeded renames path to path+".1" (overwriting any prior backup)
// when it exceeds maxBytes, so the NEXT write to path starts a fresh file —
// the one-prior-generation bound cmd/fathom/beats.go's hooks beat log
// already proved out (rotateBeatLogIfNeeded) before this package existed
// (DG-6, diagnostics sweep). Suitable for any log a caller REOPENS BY PATH
// on every write (os.OpenFile(path, os.O_APPEND|os.O_CREATE, ...)) — a
// rename there is invisible to the writer, which simply creates a new file
// at path on its next open.
//
// It is NOT suitable for a file descriptor a caller holds open for the
// life of the process (e.g. a launchd-piped os.Stdout/os.Stderr): renaming
// the path out from under an already-open fd leaves that fd writing into
// the RENAMED file forever, silently orphaning every future write from the
// path anyone else looks at. cmd/fathom's own daemon stdio bound
// (truncateStdioIfNeeded) exists precisely because it needs the opposite
// semantics — truncate the open descriptor in place — for exactly this
// reason.
//
// Best-effort by design, matching every existing beat-log operation: a
// missing file (the overwhelmingly common case — nothing to rotate yet) is
// not an error; any other stat/rename failure is returned so a caller MAY
// log it, but never blocks whatever write prompted the check.
func RotateIfNeeded(path string, maxBytes int64) error {
	fi, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if fi.Size() < maxBytes {
		return nil
	}
	return os.Rename(path, path+".1")
}
