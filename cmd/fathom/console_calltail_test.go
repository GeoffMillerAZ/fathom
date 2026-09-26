package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCallLogRows_ReadsAppendedBytesOnly: the console's call-log reader
// counts exactly what the whole-file reader counts (console reads, blank
// and malformed lines left out), parses only what was appended since its
// last read, counts an unterminated last line without consuming it, starts
// over on a replaced or truncated log, and returns a read failure as an
// error — a missing log as os.ErrNotExist — never as an empty log.
func TestCallLogRows_ReadsAppendedBytesOnly(t *testing.T) {
	resetCallLogTail()
	t.Cleanup(resetCallLogTail)
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
