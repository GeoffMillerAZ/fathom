package main

import (
	"bufio"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeGzipLines(t *testing.T, path string, lines string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := gzip.NewWriter(f)
	if _, err := io.WriteString(zw, lines); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func countCallLogRows(t *testing.T, rc io.ReadCloser) int {
	t.Helper()
	defer rc.Close()
	n := 0
	sc := bufio.NewScanner(rc)
	for sc.Scan() {
		if len(sc.Bytes()) > 0 {
			n++
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return n
}

// An archive pass renames the finished gzip into place before it removes
// the pending generation; a kill between the two leaves both. A reader
// must count that generation once, never twice (I38).
func TestCallLogReadersCountArchivedPendingOnce(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp-calls.jsonl")
	rows := `{"ts":"2026-09-27T01:00:00Z","tool":"search"}` + "\n" + `{"ts":"2026-09-27T01:00:01Z","tool":"search"}` + "\n"
	pending := path + ".20260927T020000Z.pending"
	if err := os.WriteFile(pending, []byte(rows), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := gzipFile(pending, path+".20260927T020000Z.jsonl.gz"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"ts":"2026-09-27T03:00:00Z","tool":"obligations"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rc, err := openCallLogGenerations(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := countCallLogRows(t, rc); got != 3 {
		t.Fatalf("rows read = %d, want 3 (the generation is both archived and still pending; count it once)", got)
	}
	entries, _, _, err := callLogReadCounting(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("callLogReadCounting entries = %d, want 3", len(entries))
	}
}

// A windowed reader never opens a generation whose every row predates its
// window: a generation's name is the time it left <log>.1, so all its rows
// are older than that. The old archive here is not even valid gzip — a
// windowed read that opened it would fail.
func TestCallLogWindowedReadSkipsGenerationsBeforeSince(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp-calls.jsonl")
	if err := os.WriteFile(path+".20260901T000000Z.jsonl.gz", []byte("not gzip"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path+".20260902T000000Z-1.pending", []byte(`{"ts":"2026-09-01T12:00:00Z","tool":"old","pointers":["a.go:1"]}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeGzipLines(t, path+".20260927T020000Z.jsonl.gz", `{"ts":"2026-09-27T01:00:00Z","tool":"search","pointers":["b.go:2"]}`+"\n")
	if err := os.WriteFile(path, []byte(`{"ts":"2026-09-27T03:00:00Z","tool":"obligations","pointers":["c.go:3"]}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	since := time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC)
	rc, err := openCallLogGenerationsSince(path, since)
	if err != nil {
		t.Fatalf("a windowed read opened a generation older than its window: %v", err)
	}
	if got := countCallLogRows(t, rc); got != 2 {
		t.Fatalf("windowed rows = %d, want 2", got)
	}
	if got := readCallLogPointerRows(path, "", since); len(got) != 2 {
		t.Fatalf("discoveries rows = %d, want 2 (it must read only the window's generations)", len(got))
	}
	entries, _, _, err := callLogReadCountingSince(path, since)
	if err != nil || len(entries) != 2 {
		t.Fatalf("callLogReadCountingSince = %d entries, %v; want 2, nil", len(entries), err)
	}
	// The zero window is every generation: the corrupt archive is an error,
	// never a silently partial read.
	if _, _, _, err := callLogReadCounting(path); err == nil {
		t.Fatal("an unreadable archive read as whole in an all-history read")
	}
}
