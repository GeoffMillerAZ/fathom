// Package readset makes a reader's blindness visible.
//
// The defect it exists to close (ADR-027, docs/design/reader-coverage.md):
// eight readers in fathom's reporting layer did `if err != nil { continue }`,
// which collapses three different facts into one —
//
//   - the file is not there (a real, reportable absence),
//   - the file is there and could not be read (an instrument failure),
//   - the file is there, readable and empty (a real, reportable emptiness).
//
// Measured consequence: `chmod 000 docs/intent/invariants.md` made
// `fathom intent audit` report 0 invariants instead of 37 and 85 unproven
// claims instead of 169, with NOTHING naming the file. The repository's
// most broken state was its healthiest-looking one. On this machine that is
// not hypothetical — a wedged macOS TCC verdict fails open() for a whole
// process tree while stat() keeps working.
//
// The vocabulary deliberately mirrors fusion.ReadFailure (IT-14/IT-15, which
// decided exactly this for `fathom ingest` in v0.5.0). It does NOT import it:
// the ingest path's refusal semantics — never overwrite a good index with a
// plausible empty one — are its own, and the reporting layer wants the
// opposite, to report honestly rather than to refuse.
package readset

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Outcome is what happened to one attempted read.
type Outcome string

const (
	// Read means the bytes are trustworthy.
	Read Outcome = "read"
	// Absent means the file genuinely is not there — a fact worth
	// reporting, and NOT the same as being unable to look.
	Absent Outcome = "absent"
	// Unreadable means the file exists and could not be read. This is an
	// instrument failure, never a finding about the repository.
	Unreadable Outcome = "unreadable"
	// Skipped means the caller chose not to read it, and said why.
	Skipped Outcome = "skipped"
)

// Entry is one recorded read.
type Entry struct {
	Path    string  `json:"path"`
	Outcome Outcome `json:"outcome"`
	Detail  string  `json:"detail,omitempty"` // errno text, or the skip category
	Bytes   int     `json:"bytes,omitempty"`
}

// Set records every read made while building one report. The zero value is
// ready to use and is NOT safe for concurrent use — a Set belongs to one
// invocation, which is the only scope over which "what did I fail to see"
// is a meaningful question.
type Set struct {
	// Root, when set, makes recorded paths repo-relative. A manifest is
	// read by humans in a doctor row and a 160-byte attention line; a
	// 90-character absolute path there is technically honest and
	// practically unreadable.
	Root string

	entries map[string]Entry
}

// rel renders path relative to Root when it is under it, and unchanged
// otherwise — never a "../.." chain, which is harder to read than the
// absolute path it replaces.
func (s *Set) rel(path string) string {
	if s.Root == "" {
		return path
	}
	r, err := filepath.Rel(s.Root, path)
	if err != nil || strings.HasPrefix(r, "..") {
		return path
	}
	return filepath.ToSlash(r)
}

// ReadFile reads path and records the outcome. It returns the bytes and
// whether they are trustworthy — `ok` is false for BOTH absent and
// unreadable, because a caller that wants to tell them apart must ask the
// manifest rather than guess from an empty slice.
func (s *Set) ReadFile(path string) (data []byte, ok bool) {
	b, err := os.ReadFile(path)
	name := s.rel(path)
	switch {
	case err == nil:
		s.record(Entry{Path: name, Outcome: Read, Bytes: len(b)})
		return b, true
	case errors.Is(err, fs.ErrNotExist):
		s.record(Entry{Path: name, Outcome: Absent})
		return nil, false
	default:
		s.record(Entry{Path: name, Outcome: Unreadable, Detail: errnoText(err)})
		return nil, false
	}
}

// Skip records a deliberate non-read with its category, so a manifest
// accounts for every file the caller considered — the coverage discipline
// the kit's D10 requires of an absence claim.
func (s *Set) Skip(path, category string) {
	s.record(Entry{Path: s.rel(path), Outcome: Skipped, Detail: category})
}

func (s *Set) record(e Entry) {
	if s.entries == nil {
		s.entries = map[string]Entry{}
	}
	// First outcome for a path wins: re-reading a file that already failed
	// must not quietly upgrade the manifest to complete.
	if _, seen := s.entries[e.Path]; !seen {
		s.entries[e.Path] = e
	}
}

// Manifest is the accounting: every path the Set considered, sorted.
type Manifest struct {
	Entries []Entry `json:"entries,omitempty"`
}

// Manifest returns the recorded reads, sorted by path.
func (s *Set) Manifest() Manifest {
	out := make([]Entry, 0, len(s.entries))
	for _, e := range s.entries {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return Manifest{Entries: out}
}

// Complete reports whether every considered path was read, genuinely absent,
// or deliberately skipped. False means at least one read FAILED, and every
// count derived from this manifest is a lower bound.
func (m Manifest) Complete() bool { return len(m.UnreadableEntries()) == 0 }

// UnreadableEntries returns only the failures, sorted by path.
func (m Manifest) UnreadableEntries() []Entry {
	var out []Entry
	for _, e := range m.Entries {
		if e.Outcome == Unreadable {
			out = append(out, e)
		}
	}
	return out
}

// Note renders the one-line explanation of what the manifest could not see,
// or "" when it saw everything. This is the string every count derived from
// an incomplete manifest must carry.
func (m Manifest) Note() string {
	bad := m.UnreadableEntries()
	if len(bad) == 0 {
		return ""
	}
	parts := make([]string, 0, len(bad))
	for _, e := range bad {
		parts = append(parts, fmt.Sprintf("%s: %s", e.Path, e.Detail))
	}
	return fmt.Sprintf("%d source(s) unreadable: %s", len(bad), strings.Join(parts, "; "))
}

// RenderCount is how a count derived from this manifest must be written: a
// bare number only when coverage is complete, and a NAMED lower bound
// otherwise. Callers never format a count from an incomplete manifest
// themselves — that is how "0 invariants" got printed for a repo with 37.
func (m Manifest) RenderCount(n int, noun string) string {
	if m.Complete() {
		return fmt.Sprintf("%d %s", n, noun)
	}
	return fmt.Sprintf("≥ %d %s (%s)", n, noun, m.Note())
}

// Merge folds another manifest in, first-outcome-wins, so a report built
// from several readers accounts for all of them.
func (s *Set) Merge(m Manifest) {
	for _, e := range m.Entries {
		s.record(e)
	}
}

// errnoText returns the operating system's own wording for a read failure —
// "operation not permitted" (the TCC wedge), "permission denied", "is a
// directory" — with the os.PathError wrapper stripped, since the path is
// already carried on the Entry.
func errnoText(err error) string {
	var pe *fs.PathError
	if errors.As(err, &pe) && pe.Err != nil {
		return pe.Err.Error()
	}
	return err.Error()
}
