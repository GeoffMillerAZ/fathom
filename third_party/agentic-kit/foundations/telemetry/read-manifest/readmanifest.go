// Package readmanifest makes a reader's blindness visible: every path a
// report considered is recorded with what happened to it — read, absent,
// unreadable (with the operating system's own wording), or deliberately
// skipped — so a count derived from an incomplete read can never be
// rendered as a bare total.
//
// The failure class it closes: `if err != nil { continue }` collapses
// three facts into one — the file is not there, the file is there and
// could not be read, the file is there and empty. Measured in the first
// consumer (fathom's internal/readset, its ADR-027): one unreadable file
// made an audit report 0 invariants instead of 37, with nothing naming the
// file. The repository's most broken state was its healthiest-looking one.
// Only "unreadable" makes a manifest incomplete: absence and a stated skip
// are facts a report can carry.
//
// Rules. Set.Record is first-outcome-wins per path, so a retry cannot
// erase a recorded failure (nor can a later failure overwrite a recorded
// success — a manifest is the record of one pass, not a live view; a
// caller that reads a path twice and wants both outcomes records them
// under distinct paths). Manifest output is sorted by path and
// byte-stable. Manifest.LowerBound renders a count as "N" only over a
// complete manifest and as "≥ N (k unreadable: …)" otherwise;
// Manifest.Note carries the full accounting with each failure's errno.
//
// Non-guarantees, stated plainly: a manifest knows only the paths its
// caller told it about — it never enumerates a tree itself, so a path the
// caller never considered is invisible to it too; "absent" trusts the
// operating system's not-exist verdict (a failure that misreports itself
// as not-exist is recorded as absent); nothing is retried, persisted, or
// timed (there is no clock); LowerBound lists at most five paths (Note
// lists all); a Set is scoped to one invocation and is not a coverage tool
// for anything but the reads it was told about.
//
// Consumers: cmd/constitution-lint (walkWithManifest and its read-coverage
// check) and fathom's internal/readset, which this package generalizes.
//
// Concurrency: a *Set is safe for concurrent use via sync.Mutex; a
// Manifest is an immutable value snapshot and needs no synchronization.
package readmanifest

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// maxListed bounds the paths LowerBound names inline; Note lists them all.
const maxListed = 5

// NewSet returns an empty Set ready to record.
func NewSet() *Set {
	return &Set{entries: map[string]Entry{}}
}

// RelativeTo makes subsequently recorded paths root-relative and
// slash-separated when they lie under root, and leaves them unchanged
// otherwise — never a "../.." chain, which is harder to read than the
// absolute path it replaces. A manifest is read by humans in a summary
// line; a 90-character absolute path there is technically honest and
// practically unreadable.
func (s *Set) RelativeTo(root string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.root = root
}

// rel renders path relative to the Set's root. Callers hold s.mu.
func (s *Set) rel(path string) string {
	if s.root == "" {
		return path
	}
	r, err := filepath.Rel(s.root, path)
	if err != nil || r == ".." || strings.HasPrefix(r, "../") || strings.HasPrefix(r, `..\`) {
		return path
	}
	return filepath.ToSlash(r)
}

// Record files the outcome of one attempted read. The first outcome
// recorded for a path wins: a later Record for the same path is dropped,
// so a retry cannot erase a recorded failure.
func (s *Set) Record(path string, o Outcome) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.record(s.rel(path), o)
}

// record stores e under key unless key is already present. Callers hold
// s.mu.
func (s *Set) record(key string, o Outcome) {
	if s.entries == nil {
		s.entries = map[string]Entry{}
	}
	if _, seen := s.entries[key]; !seen {
		s.entries[key] = Entry{Path: key, Outcome: o}
	}
}

// ReadFile reads path and records the outcome. It returns the bytes and
// whether they are trustworthy — ok is false for BOTH absent and
// unreadable, because a caller that wants to tell them apart must ask the
// manifest rather than guess from an empty slice.
func (s *Set) ReadFile(path string) (data []byte, ok bool) {
	b, err := os.ReadFile(path)
	switch {
	case err == nil:
		s.Record(path, Read(len(b)))
		return b, true
	case errors.Is(err, fs.ErrNotExist):
		s.Record(path, Absent())
		return nil, false
	default:
		s.Record(path, Unreadable(err))
		return nil, false
	}
}

// Skip records a deliberate non-read with its category, so the manifest
// accounts for every path the caller considered — the coverage discipline
// an absence claim requires.
func (s *Set) Skip(path, category string) {
	s.Record(path, Skipped(category))
}

// Merge folds another manifest in, first-outcome-wins per path, so a
// report built from several readers accounts for all of them. Merged
// paths are taken as already rendered and are not re-relativized.
func (s *Set) Merge(m Manifest) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range m.Entries {
		s.record(e.Path, e.Outcome)
	}
}

// Manifest returns a snapshot of the recorded reads, sorted by path.
func (s *Set) Manifest() Manifest {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Entry, 0, len(s.entries))
	for _, e := range s.entries {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return Manifest{Entries: out}
}

// Complete reports whether every considered path was read, genuinely
// absent, or deliberately skipped. False means at least one read FAILED,
// and every count derived from this manifest is a lower bound.
func (m Manifest) Complete() bool { return len(m.Unreadable()) == 0 }

// Unreadable returns only the failed reads, sorted by path.
func (m Manifest) Unreadable() []Entry {
	var out []Entry
	for _, e := range m.Entries {
		if e.Outcome.Kind == KindUnreadable {
			out = append(out, e)
		}
	}
	return out
}

// Note renders the one-line explanation of what the manifest could not
// see — every failed path with the operating system's own wording — or ""
// when it saw everything. Unlike LowerBound it is unbounded: it is the
// full accounting, for a log or a finding per path.
func (m Manifest) Note() string {
	bad := m.Unreadable()
	if len(bad) == 0 {
		return ""
	}
	parts := make([]string, 0, len(bad))
	for _, e := range bad {
		parts = append(parts, e.Path+": "+e.Outcome.Detail)
	}
	return fmt.Sprintf("%d unreadable: %s", len(bad), strings.Join(parts, "; "))
}

// LowerBound is how a count derived from this manifest must be written:
// a bare number only when coverage is complete, and a NAMED lower bound
// otherwise — "≥ N (k unreadable: p1, p2, …)", naming at most five paths
// and marking the rest with "…". Callers never format a count from an
// incomplete manifest themselves — that is how "0 invariants" got printed
// for a repository with 37.
func (m Manifest) LowerBound(n int) string {
	bad := m.Unreadable()
	if len(bad) == 0 {
		return strconv.Itoa(n)
	}
	paths := make([]string, 0, maxListed+1)
	for i, e := range bad {
		if i == maxListed {
			paths = append(paths, "…")
			break
		}
		paths = append(paths, e.Path)
	}
	return fmt.Sprintf("≥ %d (%d unreadable: %s)", n, len(bad), strings.Join(paths, ", "))
}
