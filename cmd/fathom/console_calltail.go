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

// callLogRow is one counted call, compact: the console keeps every row of
// the log in memory, so strings that repeat (tool, service, surface) are
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
}

// callLogRows returns every counted row of the log at path. The returned
// slice is shared and must not be modified. A missing log is an error
// satisfying errors.Is(err, os.ErrNotExist); any other failure to read is
// its own error — with the external-volume remedy when that is the tell.
func callLogRows(path string) ([]callLogRow, error) {
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
	t.rows = append(t.rows, rows...)
	t.offset += consumed
	t.info = info
	// An unterminated last line counts once it parses, as a whole-file
	// read counts it, but is parsed again next time (it may still grow).
	t.tail = len(tail) > 0
	if row, ok := callLogRowOf(tail, t.intern); ok {
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
func callEntriesFor(rows []callLogRow, service string, since, until time.Time) []metrics.CallEntry {
	var out []metrics.CallEntry
	for _, r := range rows {
		if r.at.IsZero() || r.at.Before(since) || r.at.After(until) {
			continue
		}
		if service != "" && r.service != service {
			continue
		}
		out = append(out, r.callEntry())
	}
	return out
}
