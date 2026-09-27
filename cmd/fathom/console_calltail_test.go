package main

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// pinCallLogClock fixes the call-log tail's clock for one test.
func pinCallLogClock(t *testing.T, at time.Time) {
	t.Helper()
	prev := callLogNow
	callLogNow = func() time.Time { return at }
	t.Cleanup(func() { callLogNow = prev })
}

// TestCallLogTailKeepsWindowOnly (RT-112, design §14): the console's
// call-log tail is a 35-day ring — the longest window any page shows — so
// a row older than that is dropped as the clock passes it, a row with no
// parseable time (never counted by any window) is never kept, and the
// kept slice does not pin the dropped rows' memory. Never a row per line
// forever.
func TestCallLogTailKeepsWindowOnly(t *testing.T) {
	resetCallLogTail()
	t.Cleanup(resetCallLogTail)
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	pinCallLogClock(t, now)
	path := filepath.Join(t.TempDir(), "mcp-calls.jsonl")
	at := func(tool string, ts time.Time) string {
		return `{"ts":"` + ts.Format(time.RFC3339) + `","tool":"` + tool + `","service":"pay","latency_ms":1}` + "\n"
	}
	old := now.Add(-36 * 24 * time.Hour)
	var b strings.Builder
	for i := 0; i < 500; i++ {
		b.WriteString(at("old", old))
	}
	b.WriteString(`{"ts":"not a time","tool":"untimed","service":"pay"}` + "\n")
	b.WriteString(at("edge", now.Add(-34*24*time.Hour)))
	b.WriteString(at("recent", now.Add(-time.Hour)))
	mustWrite(t, path, b.String())
	rows, err := callLogRows(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].tool != "edge" || rows[1].tool != "recent" {
		t.Fatalf("rows = %+v; want only the two rows inside the 35-day window", rows)
	}
	if c := cap(callLogTail.rows); c > 64 {
		t.Errorf("the ring still holds capacity for %d rows after dropping 501: the dropped rows stay resident", c)
	}
	// The log moves past the edge row: the next read drops it.
	pinCallLogClock(t, now.Add(2*24*time.Hour))
	fh, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	fh.WriteString(at("new", now.Add(2*24*time.Hour)))
	fh.Close()
	rows, err = callLogRows(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].tool != "recent" || rows[1].tool != "new" {
		t.Fatalf("after the clock moved = %+v; want recent, new", rows)
	}
	if got := callLogWindow(nil, now); got != nil {
		t.Errorf("an empty ring = %v", got)
	}
	// The ring is anchored on the log, never only the wall clock: an old
	// log read years later keeps its last 35 days, and a row stamped in
	// the future does not empty it.
	past := []callLogRow{{at: now.Add(-40 * 24 * time.Hour)}, {at: now.Add(-3 * 24 * time.Hour)}, {at: now}}
	if got := callLogWindow(past, now.AddDate(4, 0, 0)); len(got) != 2 {
		t.Errorf("an old log read later kept %d rows, want its last 35 days (2)", len(got))
	}
	future := append(slices.Clone(past), callLogRow{at: now.AddDate(1, 0, 0)})
	if got := callLogWindow(future, now); len(got) != 3 {
		t.Errorf("a future-stamped row left %d rows, want 3", len(got))
	}
}

// TestCallLogRows_ReadsAppendedBytesOnly: the console's call-log reader
// counts exactly what the whole-file reader counts (console reads, blank
// and malformed lines left out), parses only what was appended since its
// last read, counts an unterminated last line without consuming it, starts
// over on a replaced or truncated log, and returns a read failure as an
// error — a missing log as os.ErrNotExist — never as an empty log.
func TestCallLogRows_ReadsAppendedBytesOnly(t *testing.T) {
	resetCallLogTail()
	t.Cleanup(resetCallLogTail)
	pinCallLogClock(t, time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC))
	path := filepath.Join(t.TempDir(), "mcp-calls.jsonl")
	line := func(tool, svc, origin string) string {
		return `{"ts":"2026-09-24T10:00:00Z","tool":"` + tool + `","service":"` + svc + `","origin":"` + origin + `","latency_ms":3}` + "\n"
	}
	parity := func(label string) []callLogRow {
		t.Helper()
		rows, err := callLogRows(path)
		if err != nil {
			t.Fatalf("%s: %v", label, err)
		}
		whole, _, werr := classifyCallLogRead(path)
		if werr != nil {
			t.Fatalf("%s: whole-file read: %v", label, werr)
		}
		if len(rows) != len(whole) {
			t.Errorf("%s: %d rows incrementally, %d reading the whole file", label, len(rows), len(whole))
		}
		return rows
	}
	mustWrite(t, path, line("find_symbol", "pay", "")+"\n"+"not json\n"+line("list_services", "", "console"))
	if rows := parity("first read"); len(rows) != 1 || rows[0].tool != "find_symbol" || rows[0].at.IsZero() {
		t.Fatalf("first read = %+v", rows)
	}
	offset := callLogTail.offset
	// Unreadable now with nothing appended (the wedge: stat works, open
	// fails): an error now, never the counts of the earlier read.
	if os.Geteuid() != 0 {
		if err := os.Chmod(path, 0o000); err != nil {
			t.Fatal(err)
		}
		rows, err := callLogRows(path)
		_ = os.Chmod(path, 0o644)
		if err == nil || rows != nil {
			t.Errorf("an unchanged log that cannot be opened = %v, %v; want the read error", rows, err)
		}
		if rows, err := callLogRows(path); err != nil || len(rows) != 1 {
			t.Errorf("readable again = %v, %v", rows, err)
		}
	}

	appendTo := func(s string) {
		t.Helper()
		fh, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fh.WriteString(s); err != nil {
			t.Fatal(err)
		}
		fh.Close()
	}
	appendTo(line("obligations", "pay", ""))
	if rows := parity("after an append"); len(rows) != 2 || rows[1].tool != "obligations" {
		t.Fatalf("after an append = %+v", rows)
	}
	if callLogTail.offset <= offset {
		t.Errorf("the offset must move past the appended line: %d → %d", offset, callLogTail.offset)
	}

	// A line still being written: counted once it parses, never consumed.
	partial := strings.TrimSuffix(line("search", "pay", ""), "\n")
	appendTo(partial)
	before := callLogTail.offset
	if rows := parity("an unterminated line"); len(rows) != 3 || rows[2].tool != "search" {
		t.Fatalf("an unterminated line = %+v", rows)
	}
	if callLogTail.offset != before {
		t.Error("an unterminated line must not be consumed")
	}
	appendTo("\n" + line("references", "pay", ""))
	if rows := parity("its newline"); len(rows) != 4 || rows[2].tool != "search" || rows[3].tool != "references" {
		t.Fatalf("after the newline = %+v", rows)
	}

	// Replaced by a shorter log (rotation): read from the start.
	mustWrite(t, path+".new", line("blast_radius", "pay", ""))
	if err := os.Rename(path+".new", path); err != nil {
		t.Fatal(err)
	}
	if rows := parity("a replaced log"); len(rows) != 1 || rows[0].tool != "blast_radius" {
		t.Fatalf("a replaced log = %+v", rows)
	}

	if _, err := callLogRows(path + ".missing"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a missing log = %v, want os.ErrNotExist", err)
	}
	if os.Geteuid() != 0 {
		other := filepath.Join(t.TempDir(), "locked.jsonl")
		mustWrite(t, other, line("find_symbol", "pay", ""))
		if err := os.Chmod(other, 0o000); err != nil {
			t.Fatal(err)
		}
		defer os.Chmod(other, 0o644)
		if rows, err := callLogRows(other); err == nil || errors.Is(err, os.ErrNotExist) || rows != nil {
			t.Errorf("an unreadable log = %v, %v; want the read error", rows, err)
		}
	}
}
