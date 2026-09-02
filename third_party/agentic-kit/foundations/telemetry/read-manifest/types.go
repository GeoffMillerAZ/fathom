package readmanifest

import (
	"errors"
	"io/fs"
)

// Kind classifies one attempted read. Only KindUnreadable makes a manifest
// incomplete: absence and a stated skip are facts a report can carry.
type Kind string

const (
	// KindRead means the bytes are trustworthy.
	KindRead Kind = "read"
	// KindAbsent means the path genuinely is not there — a fact worth
	// reporting, and NOT the same as being unable to look.
	KindAbsent Kind = "absent"
	// KindUnreadable means the path exists and could not be read. This is
	// an instrument failure, never a finding about the tree.
	KindUnreadable Kind = "unreadable"
	// KindSkipped means the caller chose not to read it, and said why.
	KindSkipped Kind = "skipped"
)

// Outcome is what happened to one attempted read: the Kind plus the fact
// that qualifies it. Field order is the byte-stability contract for the
// JSON encoding — do not reorder.
type Outcome struct {
	// Kind classifies the read.
	Kind Kind `json:"kind"`
	// Detail is the operating system's own wording for an unreadable path
	// ("permission denied", "operation not permitted") or the caller's
	// category for a skipped one ("binary", "test-file"). Empty otherwise.
	Detail string `json:"detail,omitempty"`
	// Bytes is the length of a successful read; zero otherwise.
	Bytes int `json:"bytes,omitempty"`
}

// Read returns the outcome of a successful read of n bytes.
func Read(n int) Outcome { return Outcome{Kind: KindRead, Bytes: n} }

// Absent returns the outcome for a path that genuinely is not there.
func Absent() Outcome { return Outcome{Kind: KindAbsent} }

// Unreadable returns the outcome for a path that exists and could not be
// read, carrying the operating system's own wording for why — "operation
// not permitted", "permission denied", "is a directory" — with any
// *fs.PathError wrapper stripped, since the path is carried on the Entry.
// A nil err is recorded as "unknown error" rather than dropped: the caller
// said the read failed, and blindness must never be lost to a nil.
func Unreadable(err error) Outcome {
	return Outcome{Kind: KindUnreadable, Detail: errnoText(err)}
}

// Skipped returns the outcome for a path the caller deliberately did not
// read, with the caller's category ("binary", "test-file", "out of scope").
// An empty category is recorded as "unspecified" so the manifest still
// accounts for the path.
func Skipped(category string) Outcome {
	if category == "" {
		category = "unspecified"
	}
	return Outcome{Kind: KindSkipped, Detail: category}
}

// Entry is one recorded path with its outcome.
type Entry struct {
	// Path is the path as the Set rendered it — root-relative and
	// slash-separated when the Set was given a root, otherwise verbatim.
	Path string `json:"path"`
	// Outcome is what happened to the read.
	Outcome Outcome `json:"outcome"`
}

// Manifest is the accounting: every path the Set considered, sorted by
// path. It is an immutable value snapshot.
type Manifest struct {
	// Entries is sorted by Path; a Manifest with no entries is complete by
	// definition (it considered nothing and failed at nothing).
	Entries []Entry `json:"entries,omitempty"`
}

// errnoText returns the operating system's own wording for a read failure
// with the os.PathError wrapper stripped.
func errnoText(err error) string {
	if err == nil {
		return "unknown error"
	}
	var pe *fs.PathError
	if errors.As(err, &pe) && pe.Err != nil {
		return pe.Err.Error()
	}
	return err.Error()
}
