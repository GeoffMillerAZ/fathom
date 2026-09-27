package main

// console_tails.go — the observatory model's reads of the call log and the
// session ledger as tails (RT-113, docs/design/responsible-tenant.md §14).
// The model rebuilt every 30 s by re-reading and re-decoding the whole call
// log (tens of megabytes) and the whole session ledger; the Operate
// projection read the log whole again. Both logs are append-only, so each
// reader here folds, on each read, only the bytes appended since — the
// call log into its fold and 35-day ring, the session ledger into
// console.LedgerFold and the impact model's per-session rows — by the same
// line rules every whole-file reader uses, so a tail never counts
// differently from a whole-file read, and neither keeps a row's bytes.
//
// A file replaced (another inode), truncated, or rewritten in place (the
// last bytes read no longer match) is read again from the start. A file
// that cannot be opened now is an error now — the external volume's wedge
// makes stat succeed and open fail — never the content of an earlier read.

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/geoffmilleraz/fathom/internal/console"
)

// tailCheckBytes is how much of what was read last is compared on the next
// read, to notice a file rewritten in place with the same size.
const tailCheckBytes = 256

// tailFile is one followed file's identity and read position.
type tailFile struct {
	path   string
	info   os.FileInfo
	offset int64
	check  []byte // the last bytes consumed (≤ tailCheckBytes)
}

// open stats and opens path and reports whether the read must start over.
func (f *tailFile) open(path string) (*os.File, os.FileInfo, bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, nil, false, err
	}
	fh, err := os.Open(path)
	if err != nil {
		return nil, nil, false, fmt.Errorf("%w%s", err, tccWedgeHint(path))
	}
	restart := f.path != path || f.info == nil || !os.SameFile(f.info, info) || info.Size() < f.offset
	if !restart && len(f.check) > 0 {
		buf := make([]byte, len(f.check))
		if _, err := fh.ReadAt(buf, f.offset-int64(len(f.check))); err != nil || !bytes.Equal(buf, f.check) {
			restart = true
		}
	}
	return fh, info, restart, nil
}

// consumed records that the bytes b, ending at the new offset, were read.
func (f *tailFile) consumed(path string, info os.FileInfo, b []byte) {
	f.path, f.info = path, info
	f.offset += int64(len(b))
	f.keep(b)
}

// keep appends b to the check window, keeping its last tailCheckBytes.
func (f *tailFile) keep(b []byte) {
	if len(b) >= tailCheckBytes {
		f.check = append(f.check[:0], b[len(b)-tailCheckBytes:]...)
		return
	}
	f.check = append(f.check, b...)
	if n := len(f.check); n > tailCheckBytes {
		f.check = append(f.check[:0], f.check[n-tailCheckBytes:]...)
	}
}

// sessionLedgerSnap is one read of the session ledger, folded: the
// console's rules (console.LedgerFold: the shared sessions count, the
// per-repo and windowed counts, the activity rows, the sessions table)
// and the impact model's (each session's last row that decodes as a
// sessionLedgerEntry, in first-seen order, and the stop count) — never
// the rows' bytes (RT-113). A snapshot is the reader's own: the tail
// folds on without changing it.
type sessionLedgerSnap struct {
	fold   *console.LedgerFold
	latest map[string]sessionLedgerEntry
	order  []string
	// stops is the impact model's LedgerStops: the ledger's lines up to
	// its last non-empty one, at least one.
	stops int
}

// newSessionLedgerSnap returns an empty fold.
func newSessionLedgerSnap() *sessionLedgerSnap {
	return &sessionLedgerSnap{fold: console.NewLedgerFold(), latest: map[string]sessionLedgerEntry{}}
}

// foldSessionLedger folds whole ledger bytes (the CLI verbs' one read).
func foldSessionLedger(data []byte) *sessionLedgerSnap {
	s := newSessionLedgerSnap()
	lines := bytes.Split(data, []byte{'\n'})
	for i, line := range lines {
		s.add(line)
		if len(line) > 0 {
			s.stops = i + 1
		}
	}
	s.stops = max(s.stops, 1)
	return s
}

// add folds one line (without its newline) by every rule.
func (s *sessionLedgerSnap) add(line []byte) {
	s.fold.Add(line)
	if len(line) == 0 {
		return
	}
	var e sessionLedgerEntry
	if json.Unmarshal(line, &e) != nil {
		return
	}
	key := e.SessionID
	if key == "" {
		// A row with no session id cannot be deduped, so it counts once
		// on its own terms rather than being dropped or merged with
		// every other anonymous row.
		key = "\x00anon:" + e.At + ":" + e.Repo
	}
	if _, seen := s.latest[key]; !seen {
		s.order = append(s.order, key)
	}
	s.latest[key] = e
}

// clone is an independent copy (the tail's reader gets one).
func (s *sessionLedgerSnap) clone() *sessionLedgerSnap {
	c := &sessionLedgerSnap{fold: s.fold.Clone(), latest: make(map[string]sessionLedgerEntry, len(s.latest)), order: s.order[:len(s.order):len(s.order)], stops: s.stops}
	for k, v := range s.latest {
		c.latest[k] = v
	}
	return c
}

// lastRow is the session's last ledger row that decodes, nil when none.
func (s *sessionLedgerSnap) lastRow(sessionID string) *sessionLedgerEntry {
	if s == nil || sessionID == "" {
		return nil
	}
	e, ok := s.latest[sessionID]
	if !ok {
		return nil
	}
	return &e
}

// sessionLedgerTail follows the session ledger, folding only the lines
// appended since the last read (RT-113: never a whole-log re-parse, and
// never the whole log held).
type sessionLedgerTail struct {
	mu    sync.Mutex
	f     tailFile
	snap  *sessionLedgerSnap
	lines int // complete lines folded
	// parsed counts the bytes folded over the tail's life (tests).
	parsed int64
}

// read returns the ledger's fold as foldSessionLedger(whole file) would.
func (t *sessionLedgerTail) read(path string) (*sessionLedgerSnap, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	fh, info, restart, err := t.f.open(path)
	if err != nil {
		return nil, err
	}
	defer fh.Close()
	if restart || t.snap == nil {
		t.resetLocked()
	}
	if _, err := fh.Seek(t.f.offset, io.SeekStart); err != nil {
		return nil, fmt.Errorf("read session ledger %s: %w", path, err)
	}
	br := bufio.NewReaderSize(fh, 64*1024)
	var tail []byte
	for {
		line, rerr := br.ReadSlice('\n')
		if rerr == bufio.ErrBufferFull {
			long := append([]byte(nil), line...)
			for rerr == bufio.ErrBufferFull && len(long) <= 16*1024*1024 {
				line, rerr = br.ReadSlice('\n')
				long = append(long, line...)
			}
			if rerr == bufio.ErrBufferFull {
				t.resetLocked()
				return nil, fmt.Errorf("read session ledger %s: a line longer than 16 MiB", path)
			}
			line = long
		}
		if rerr == io.EOF {
			tail = append([]byte(nil), line...)
			break
		}
		if rerr != nil {
			t.resetLocked()
			return nil, fmt.Errorf("read session ledger %s: %w", path, rerr)
		}
		t.parsed += int64(len(line))
		body := line[:len(line)-1]
		t.lines++
		t.snap.add(body)
		if len(body) > 0 {
			t.snap.stops = t.lines
		}
		t.f.offset += int64(len(line))
		t.f.keep(line)
	}
	t.f.path, t.f.info = path, info
	snap := t.snap.clone()
	// An unterminated last line counts as a whole-file read counts it,
	// and is folded again next time (it may still grow).
	if len(tail) > 0 {
		snap.add(tail)
		snap.stops = t.lines + 1
	}
	snap.stops = max(snap.stops, 1)
	return snap, nil
}

func (t *sessionLedgerTail) resetLocked() {
	t.f, t.snap, t.lines = tailFile{}, newSessionLedgerSnap(), 0
}

func (t *sessionLedgerTail) reset() {
	t.mu.Lock()
	t.resetLocked()
	t.parsed = 0
	t.mu.Unlock()
}

// callLogEntryTail follows the call log with callLogReadCounting's
// answer — the counted rows, the malformed lines and the console's own
// reads left out — kept as the log's all-time fold (callLogFold) plus the
// counted rows inside the 35-day ring (RT-112, design §14: "the call-log
// tail keeps aggregates plus a 35-day ring (no row per line forever)").
// Every all-time figure comes from the fold; every windowed one (the
// longest window a page shows is 30 days) from the ring.
type callLogEntryTail struct {
	mu   sync.Mutex
	f    tailFile
	fold *callLogFold
	// ring holds the counted rows inside the window, in file order, and
	// ringAt their times; newest is the greatest row time read. A row
	// whose time does not parse is in the fold only (no window counts it).
	ring                  []adoptionCallEntry
	ringAt                []time.Time
	newest                time.Time
	cutoff                time.Time // the window start the ring was last trimmed to
	skipped, consoleReads int
	intern                map[string]string
	// parsed counts the bytes decoded over the tail's life (tests).
	parsed int64
}

// callLogSnapshot is one read of the call log through the tail: the fold
// is the reader's own copy; ring is shared and must not be modified.
type callLogSnapshot struct {
	fold                  *callLogFold
	ring                  []adoptionCallEntry
	skipped, consoleReads int
}

// callLogTrimEvery is how far the window start moves before the ring is
// trimmed: it may hold up to this much past the window, which no page reads.
const callLogTrimEvery = time.Hour

// read returns the log's snapshot as callLogReadCounting(path) would
// count it, reading only the bytes appended since the last read.
func (t *callLogEntryTail) read(path string) (callLogSnapshot, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	fh, info, restart, err := t.f.open(path)
	if err != nil {
		return callLogSnapshot{}, err
	}
	defer fh.Close()
	if restart || t.fold == nil {
		t.resetLocked()
	}
	if _, err := fh.Seek(t.f.offset, io.SeekStart); err != nil {
		return callLogSnapshot{}, fmt.Errorf("read call log %s: %w", path, err)
	}
	now := callLogNow()
	br := bufio.NewReaderSize(fh, 256*1024)
	var tail []byte
	for {
		line, rerr := br.ReadSlice('\n')
		if rerr == bufio.ErrBufferFull {
			long := append([]byte(nil), line...)
			for rerr == bufio.ErrBufferFull && len(long) <= 4*1024*1024 {
				line, rerr = br.ReadSlice('\n')
				long = append(long, line...)
			}
			if rerr == bufio.ErrBufferFull {
				t.resetLocked() // partly folded: start over next time
				return callLogSnapshot{}, fmt.Errorf("read call log %s: a line longer than 4 MiB", path)
			}
			line = long
		}
		if rerr == io.EOF {
			tail = append([]byte(nil), line...)
			break
		}
		if rerr != nil {
			t.resetLocked()
			return callLogSnapshot{}, fmt.Errorf("read call log %s: %w", path, rerr)
		}
		t.parsed += int64(len(line))
		t.count(line, now)
		t.f.offset += int64(len(line))
		t.f.keep(line)
	}
	t.f.path, t.f.info = path, info
	t.trim(now)
	snap := callLogSnapshot{fold: t.fold.clone(), ring: t.ring, skipped: t.skipped, consoleReads: t.consoleReads}
	// An unterminated last line counts as a whole-file read counts it,
	// and is read again next time (it may still grow).
	if len(tail) > 0 {
		switch e, kind := decodeCallLogLine(tail); kind {
		case callLogLineMalformed:
			snap.skipped++
		case callLogLineConsole:
			snap.consoleReads++
		case callLogLineEntry:
			snap.fold.add(&e)
			if at := callLogEntryAt(e.TS); !at.IsZero() && !at.Before(t.cutoff) {
				snap.ring = append(slices.Clip(snap.ring), e)
			}
		}
	}
	return snap, nil
}

// count folds one complete line and keeps it in the ring when timed and
// not already outside the window (so a first read of a long log never
// holds its old rows, even briefly).
func (t *callLogEntryTail) count(line []byte, now time.Time) {
	switch e, kind := decodeCallLogLine(line); kind {
	case callLogLineMalformed:
		t.skipped++
	case callLogLineConsole:
		t.consoleReads++
	case callLogLineEntry:
		t.fold.add(&e)
		at := callLogEntryAt(e.TS)
		if at.IsZero() {
			return
		}
		if at.After(t.newest) {
			t.newest = at
		}
		if at.Before(t.cutoff) || at.Before(callLogWindowStart(t.newest, now)) {
			return
		}
		e.Tool, e.Service, e.Surface = internString(t.intern, e.Tool), internString(t.intern, e.Service), internString(t.intern, e.Surface)
		e.Origin, e.Group, e.Via, e.MatchedVia = internString(t.intern, e.Origin), internString(t.intern, e.Group), internString(t.intern, e.Via), internString(t.intern, e.MatchedVia)
		// Appending never writes under a snapshot: a snapshot's slice ends
		// before the index an append writes.
		t.ring = append(t.ring, e)
		t.ringAt = append(t.ringAt, at)
	}
}

// trim drops the rows before the window's start — callLogRing before the
// newest row, or before now when that is earlier (callLogWindow's anchor)
// — once the start has moved callLogTrimEvery, into fresh slices so the
// dropped rows' memory is released and a snapshot is never modified.
func (t *callLogEntryTail) trim(now time.Time) {
	cutoff := callLogWindowStart(t.newest, now)
	if !t.cutoff.IsZero() && cutoff.Sub(t.cutoff) < callLogTrimEvery {
		return
	}
	t.cutoff = cutoff
	keep := 0
	for _, at := range t.ringAt {
		if !at.Before(cutoff) {
			keep++
		}
	}
	if keep == len(t.ring) {
		return
	}
	ring, ringAt := make([]adoptionCallEntry, 0, keep), make([]time.Time, 0, keep)
	for i, at := range t.ringAt {
		if !at.Before(cutoff) {
			ring, ringAt = append(ring, t.ring[i]), append(ringAt, at)
		}
	}
	t.ring, t.ringAt = ring, ringAt
}

// callLogWindowStart is the ring's start: callLogRing before the newest
// row, or before now when that is earlier (callLogWindow's anchor).
func callLogWindowStart(newest, now time.Time) time.Time {
	if newest.IsZero() || newest.After(now) {
		newest = now
	}
	return newest.Add(-callLogRing)
}

// callLogEntryAt is a row's time for the ring: its ts, or the day its ts
// starts with (the day-keyed windows count such a row), zero when neither
// parses.
func callLogEntryAt(ts string) time.Time {
	if at, ok := parseCallLogTS(ts); ok {
		return at
	}
	if len(ts) >= 10 {
		if d, err := time.Parse("2006-01-02", ts[:10]); err == nil {
			return d
		}
	}
	return time.Time{}
}

func (t *callLogEntryTail) resetLocked() {
	t.f, t.fold, t.ring, t.ringAt = tailFile{}, newCallLogFold(), nil, nil
	t.newest, t.cutoff, t.skipped, t.consoleReads, t.intern = time.Time{}, time.Time{}, 0, 0, map[string]string{}
}

func (t *callLogEntryTail) reset() {
	t.mu.Lock()
	t.resetLocked()
	t.parsed = 0
	t.mu.Unlock()
}

// The model's and the Operate projection's tails: one per source.
var (
	consoleCallLogTail   callLogEntryTail
	consoleSessionLedger sessionLedgerTail
)

// resetConsoleTails forgets every tail (tests; the shed keeps them, RT-113).
func resetConsoleTails() {
	consoleCallLogTail.reset()
	consoleSessionLedger.reset()
}
