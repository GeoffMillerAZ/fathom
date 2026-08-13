package scope

// source.go — the per-call state reader (the V5 rework).
//
// The previous build installed the suppression filter at store OPEN time,
// which made the feature inert in the only deployment that exists: the
// daemon opens its store once and holds it for the process lifetime, so a
// suppression written after startup was never seen while the CLI printed
// that it took effect. A Source is the correction: the store's read
// methods call Snapshot on EVERY read, Snapshot stat()s the file, and a
// changed (mtime, size) pair reloads — so a long-lived handle sees a new
// suppression on the NEXT query. The stat costs ~1µs; the reload costs one
// small file parse and only happens when the file actually changed.

import (
	"os"
	"sync"
	"time"
)

// Source is a reloading view of one suppression file. Safe for concurrent
// use; the zero value is not usable — construct via NewSource.
type Source struct {
	path string
	now  func() time.Time

	mu      sync.Mutex
	loaded  bool
	modTime time.Time
	size    int64
	set     *Set
	err     error
}

// NewSource returns a Source over path. An empty path yields a permanently
// empty (disarmed) source. now supplies the read-time clock for expiry
// judgement; nil means time.Now — tests inject a fixed clock.
func NewSource(path string, now func() time.Time) *Source {
	if now == nil {
		now = time.Now
	}
	return &Source{path: path, now: now}
}

// Path returns the file this source reads.
func (s *Source) Path() string {
	if s == nil {
		return ""
	}
	return s.path
}

// Now returns the source's read-time clock value.
func (s *Source) Now() time.Time {
	if s == nil {
		return time.Now()
	}
	return s.now()
}

// Snapshot returns the current suppression Set and the read-time clock,
// reloading the file first when its (mtime, size) changed since the last
// call. A missing file is the empty set. A file that cannot be parsed
// fails OPEN — the set is nil and Err reports the parse error — because a
// malformed suppressions file must never make the whole index unreadable;
// `fathom scope list` is the loud surface for the error.
func (s *Source) Snapshot() (*Set, time.Time) {
	if s == nil || s.path == "" {
		return nil, time.Now()
	}
	now := s.now()

	s.mu.Lock()
	defer s.mu.Unlock()

	var modTime time.Time
	var size int64
	if fi, err := os.Stat(s.path); err == nil {
		modTime = fi.ModTime()
		size = fi.Size()
	}
	if !s.loaded || !modTime.Equal(s.modTime) || size != s.size {
		all, err := LoadSuppressions(s.path)
		if err != nil {
			s.set, s.err = nil, err
		} else if set, serr := NewSet(all); serr != nil {
			s.set, s.err = nil, serr
		} else {
			s.set, s.err = set, nil
		}
		s.loaded = true
		s.modTime = modTime
		s.size = size
	}
	return s.set, now
}

// Err returns the error from the most recent load attempt, or nil. The
// filter fails open on error; this is how a caller (doctor, `scope list`)
// finds out something is wrong.
func (s *Source) Err() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}
