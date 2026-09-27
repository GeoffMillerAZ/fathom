package main

// console_calltail.go — the console's call-log reader (design §1:
// "a per-projection cache is keyed by its input watermarks … nothing
// re-renders when the watermarks are unchanged"). The call log is
// append-only JSONL, and the console's own registry reads append rows to
// it every model build (origin console, never counted), so a cache keyed
// on the file's size and mtime would still re-parse the whole log every
// 30 s. This reader keeps what it parsed and, on the next read, parses
// only the bytes appended since — the same line decoding every usage
// reader shares (decodeCallLogLine), so an incremental read can never
// count differently from a whole-file one.
//
// It is a ring, not a row per line forever (RT-112, design §14): only the
// rows inside callLogRing — the longest window any page shows — are kept,
// so the daemon's resident call-log state is bounded by the last 35 days'
// traffic however long the log itself grows.
//
// A log that cannot be read is an error returned to the caller, never an
// empty read: the external-volume wedge makes stat succeed while open
// fails, and "nothing parsed" must not become "no traffic".

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/geoffmilleraz/fathom/internal/metrics"
)

// callLogRing is the tail's window: the longest any console page reads
// (the traffic pages read 30 days; 35 keeps a margin for a week-aligned
// view). callLogNow is its clock (tests pin it).
const callLogRing = 35 * 24 * time.Hour

var callLogNow = time.Now

// callLogWindow keeps the rows at or after the window's start: callLogRing
// before the newest row's time, or before now when that is earlier (a
// row stamped in the future never empties the ring). Anchoring on the log
// rather than the wall clock keeps the ring a function of the log: every
// page's window ends at its now, which is never before the newest row
// written, so a row the ring drops is outside every page's window. An
// older row, or one whose time does not parse (no window counts it), is
// dropped. The kept rows are copied into a fresh slice only when
// something was dropped, so the dropped rows' memory is released and a
// slice a caller still holds is never modified.
func callLogWindow(rows []callLogRow, now time.Time) []callLogRow {
	var newest time.Time
	for _, r := range rows {
		if r.at.After(newest) {
			newest = r.at
		}
	}
	if newest.IsZero() || newest.After(now) {
		newest = now
	}
	cutoff := newest.Add(-callLogRing)
	drop := 0
	for _, r := range rows {
		if r.at.IsZero() || r.at.Before(cutoff) {
			drop++
		}
	}
	if drop == 0 {
		return rows
	}
	if drop == len(rows) {
		return nil
	}
	out := make([]callLogRow, 0, len(rows)-drop)
	for _, r := range rows {
		if !r.at.IsZero() && !r.at.Before(cutoff) {
			out = append(out, r)
		}
	}
	return out
}

// callLogRow is one counted call, compact: the console keeps the window's
// rows in memory, so strings that repeat (tool, service, surface) are
// interned and the timestamp is kept as a time.
type callLogRow struct {
	at                     time.Time // zero when the row's ts does not parse
	ts                     string
	tool, service, surface string
	latencyMS              float64
	failed                 bool
}

// callEntry is the row in internal/metrics' shape.
func (r callLogRow) callEntry() metrics.CallEntry {
	e := metrics.CallEntry{TS: r.ts, Tool: r.tool, Service: r.service, Surface: r.surface, LatencyMS: r.latencyMS}
	if r.failed {
		e.Error = "error"
	}
	return e
}

var callLogTail struct {
	sync.Mutex
	path   string
	info   os.FileInfo // the file as last read (identity, size, mtime)
	offset int64       // bytes parsed, always at a line boundary
	// tail is set while the log ends in an unterminated line: it is
	// parsed again on every read until its newline arrives.
	tail   bool
	rows   []callLogRow
	intern map[string]string
}

// resetCallLogTail forgets the parsed log (tests).
func resetCallLogTail() {
	callLogTail.Lock()
	callLogTail.path, callLogTail.info, callLogTail.offset, callLogTail.tail, callLogTail.rows, callLogTail.intern = "", nil, 0, false, nil, nil
	callLogTail.Unlock()
	rotatedTail.Lock()
	rotatedTail.path, rotatedTail.info, rotatedTail.rows = "", nil, nil
	rotatedTail.Unlock()
}

// rotatedTail caches the rotated generation (<log>.1, RT-126): it is
// written once, at rotation, so it is parsed once per rotation, and
// returned as its own segment — never copied when a live row arrives.
var rotatedTail struct {
	sync.Mutex
	path string
	info os.FileInfo
	rows []callLogRow
}

// callLogRowSegments returns the counted rows of the log at path inside the
// ring's window (callLogWindow, RT-112) as two
// segments, oldest first: the rotated generation <path>.1 (RT-126, §12.1:
// console tails read it, so a rotation never empties the console's
// traffic) and the live log. Both slices are shared and must not be
// modified. A missing live log is an error satisfying
// errors.Is(err, os.ErrNotExist); any other failure to read is its own
// error — with the external-volume remedy when that is the tell.
func callLogRowSegments(path string) (rotated, live []callLogRow, err error) {
	live, err = liveCallLogRows(path)
	if err != nil {
		return nil, nil, err
	}
	gen := path + ".1"
	info, serr := os.Stat(gen)
	if serr != nil {
		if os.IsNotExist(serr) {
			return nil, live, nil
		}
		return nil, nil, fmt.Errorf("%w%s", serr, tccWedgeHint(gen))
	}
	rt := &rotatedTail
	rt.Lock()
	defer rt.Unlock()
	if rt.path != gen || rt.info == nil || !os.SameFile(rt.info, info) || info.Size() != rt.info.Size() || !info.ModTime().Equal(rt.info.ModTime()) {
		fh, err := os.Open(gen)
		if err != nil {
			return nil, nil, fmt.Errorf("%w%s", err, tccWedgeHint(gen))
		}
		rows, _, tail, err := scanCallLogRows(fh, map[string]string{})
		fh.Close()
		if err != nil {
			return nil, nil, fmt.Errorf("read rotated call log %s: %w", gen, err)
		}
		if row, ok := callLogRowOf(tail, map[string]string{}); ok {
			rows = append(rows, row)
		}
		// The ring (RT-112) holds only the window's rows of either
		// generation.
		rt.path, rt.info, rt.rows = gen, info, slices.Clip(callLogWindow(rows, callLogNow()))
	}
	return rt.rows, live, nil
}

// liveCallLogRows parses the live log incrementally, keeping the window.
func liveCallLogRows(path string) ([]callLogRow, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	// Opened on every read, even when nothing was appended: a log that
	// cannot be read now is an error now — the same answer every other
	// reader of it gives — never the counts of an earlier read.
	fh, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("%w%s", err, tccWedgeHint(path))
	}
	defer fh.Close()
	t := &callLogTail
	t.Lock()
	defer t.Unlock()
	switch {
	case t.path != path || t.info == nil || !os.SameFile(t.info, info) || info.Size() < t.offset:
		// Another log, a replaced file, or a truncated one: start over.
		t.path, t.info, t.offset, t.tail, t.rows, t.intern = path, nil, 0, false, nil, map[string]string{}
	case !t.tail && info.Size() == t.info.Size() && info.ModTime().Equal(t.info.ModTime()):
		return t.rows, nil
	}
	if _, err := fh.Seek(t.offset, io.SeekStart); err != nil {
		return nil, fmt.Errorf("read call log %s: %w", path, err)
	}
	rows, consumed, tail, err := scanCallLogRows(fh, t.intern)
	if err != nil {
		// Partial counts are not a complete observation: keep what was
		// parsed before, and report the failure.
		return nil, fmt.Errorf("read call log %s: %w", path, err)
	}
	t.rows = callLogWindow(append(t.rows, rows...), callLogNow())
	t.offset += consumed
	t.info = info
	// An unterminated last line counts once it parses, as a whole-file
	// read counts it, but is parsed again next time (it may still grow).
	t.tail = len(tail) > 0
	if row, ok := callLogRowOf(tail, t.intern); ok && len(callLogWindow([]callLogRow{row}, callLogNow())) == 1 {
		return append(slices.Clip(t.rows), row), nil
	}
	return t.rows, nil
}

// scanCallLogRows parses complete lines from r and reports how many bytes
// they spanned, and the unterminated last line (a write in progress) that
// it did not consume.
func scanCallLogRows(r io.Reader, intern map[string]string) (rows []callLogRow, consumed int64, tail []byte, err error) {
	br := bufio.NewReaderSize(r, 256*1024)
	for {
		line, err := br.ReadSlice('\n')
		if err == bufio.ErrBufferFull {
			// A line longer than the buffer: gather it whole (the shared
			// reader allows lines up to 4 MiB).
			long := append([]byte(nil), line...)
			for err == bufio.ErrBufferFull && len(long) <= 4*1024*1024 {
				line, err = br.ReadSlice('\n')
				long = append(long, line...)
			}
			if err == bufio.ErrBufferFull {
				return nil, 0, nil, fmt.Errorf("a line longer than 4 MiB")
			}
			line = long
		}
		if err == io.EOF {
			return rows, consumed, append([]byte(nil), line...), nil
		}
		if err != nil {
			return nil, 0, nil, err
		}
		consumed += int64(len(line))
		if row, ok := callLogRowOf(line, intern); ok {
			rows = append(rows, row)
		}
	}
}

// callLogRowOf decodes one line into a counted row (false for a blank,
// malformed or console-origin line).
func callLogRowOf(line []byte, intern map[string]string) (callLogRow, bool) {
	e, kind := decodeCallLogLine(line)
	if kind != callLogLineEntry {
		return callLogRow{}, false
	}
	at, _ := parseCallLogTS(e.TS)
	return callLogRow{
		at: at, ts: e.TS,
		tool: internString(intern, e.Tool), service: internString(intern, e.Service), surface: internString(intern, e.Surface),
		latencyMS: e.LatencyMS, failed: e.Error != "",
	}, true
}

func internString(m map[string]string, s string) string {
	if v, ok := m[s]; ok {
		return v
	}
	m[s] = s
	return s
}

// callEntriesFor is one service's rows in [since, until], in
// internal/metrics' shape — readCallDataFor's window and service filter.
func callEntriesFor(service string, since, until time.Time, segments ...[]callLogRow) []metrics.CallEntry {
	var out []metrics.CallEntry
	for _, rows := range segments {
		for _, r := range rows {
			if r.at.IsZero() || r.at.Before(since) || r.at.After(until) {
				continue
			}
			if service != "" && r.service != service {
				continue
			}
			out = append(out, r.callEntry())
		}
	}
	return out
}
