package main

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// RT-126 (design §12.1): the call log rotates at its size bound; the
// previous generation is rolled up and gzipped into an archive before it
// is removed, so every row written survives in the live log, the rotated
// generation or an archive; console tails read the rotated generation.
func TestCallLogRotationArchivesNeverDeletes(t *testing.T) {
	resetCallLogTail()
	t.Cleanup(resetCallLogTail)
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp-calls.jsonl")
	var logs []string
	var logMu sync.Mutex // the archive pass logs from its own goroutine
	w, err := openRotatingCallLog(path, 4096, func(m string) { logMu.Lock(); logs = append(logs, m); logMu.Unlock() })
	if err != nil {
		t.Fatal(err)
	}
	clock := time.Date(2026, 9, 27, 4, 0, 0, 0, time.UTC)
	w.now = func() time.Time { clock = clock.Add(time.Minute); return clock }
	const n = 200
	for i := 0; i < n; i++ {
		row := fmt.Sprintf(`{"schema_version":3,"ts":"2026-09-27T04:%02d:%02dZ","tool":"search","service":"svc","surface":"agent","outcome":"ok","seq":%d}`+"\n", i/60, i%60, i)
		if _, err := w.Write([]byte(row)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	archives := callLogArchives(path)
	if len(archives) < 2 {
		t.Fatalf("200 rows through a 4 KiB bound rotate more than twice; archives %v (logs %v)", archives, logs)
	}
	// Every row survives exactly once across the archives, .1 and the live log.
	seen := map[int]int{}
	count := func(name string, r *bufio.Scanner) {
		for r.Scan() {
			var e struct{ Seq int }
			if json.Unmarshal(r.Bytes(), &e) != nil {
				t.Fatalf("%s: unparsable line %q", name, r.Text())
			}
			seen[e.Seq]++
		}
	}
	for _, a := range archives {
		f, err := os.Open(a)
		if err != nil {
			t.Fatal(err)
		}
		zr, err := gzip.NewReader(f)
		if err != nil {
			t.Fatalf("%s is not gzip: %v", a, err)
		}
		count(a, bufio.NewScanner(zr))
		f.Close()
		rollup := strings.TrimSuffix(a, ".jsonl.gz") + ".rollup.json"
		b, err := os.ReadFile(rollup)
		if err != nil || !strings.Contains(string(b), `"search"`) {
			t.Fatalf("archive %s has no rollup naming its tools: %v %s", a, err, b)
		}
	}
	for _, p := range []string{path + ".1", path} {
		f, err := os.Open(p)
		if err != nil {
			t.Fatal(err)
		}
		count(p, bufio.NewScanner(f))
		f.Close()
	}
	for i := 0; i < n; i++ {
		if seen[i] != 1 {
			t.Fatalf("row %d appears %d times across the generations; retention must never drop or duplicate a row", i, seen[i])
		}
	}
	if st, _ := os.Stat(path); st.Size() > 4096 {
		t.Fatalf("the live log is %d bytes, over its 4 KiB bound", st.Size())
	}

	// Console tails read the rotated generation first, then the live log.
	rows, err := callLogRows(path)
	if err != nil {
		t.Fatal(err)
	}
	f1, _ := os.ReadFile(path + ".1")
	fl, _ := os.ReadFile(path)
	want := strings.Count(string(f1), "\n") + strings.Count(string(fl), "\n")
	if len(rows) != want || want == 0 {
		t.Fatalf("console tail read %d rows, want the rotated generation plus the live log (%d)", len(rows), want)
	}

	// Past 1 GiB of archives the rotation reports and deletes nothing.
	big := path + ".20260101T000000Z.jsonl.gz"
	bf, _ := os.Create(big)
	_ = bf.Truncate(callLogArchiveWarnBytes + 1)
	bf.Close()
	before := len(callLogArchives(path))
	if _, err := rotateCallLog(path, time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	rot, err := archiveCallLogGenerations(path)
	if err != nil {
		t.Fatal(err)
	}
	if rot.Report == "" || !strings.Contains(rot.Report, "nothing is deleted") {
		t.Fatalf("over 1 GiB of archives must be reported: %+v", rot)
	}
	if after := len(callLogArchives(path)); after != before+1 {
		t.Fatalf("archives went %d → %d; the rotation only adds", before, after)
	}
}
