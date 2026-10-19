package main

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/hookcache"
)

// statSkippedLine is the ingest summary's stat-gate line.
var statSkippedLine = regexp.MustCompile(`(?m)^stat-skipped: (\d+) `)

// TestCLIIngestReadsOnlyStatChangedFilesAcrossRuns is RT-180 for the CLI
// (internal/fusion's TestStatGateReadsFileStatsAcrossRestart is the row's
// reachable_from; a second test of that name would make it ambiguous): a
// one-shot `fathom ingest` reads its prior stats from the store's
// file_stats, so a second run over an unchanged tree reads nothing and,
// after one file is touched, reads only that file.
func TestCLIIngestReadsOnlyStatChangedFilesAcrossRuns(t *testing.T) {
	origIdent := hookcache.IdentDir
	hookcache.IdentDir = filepath.Join(t.TempDir(), "ident")
	t.Cleanup(func() { hookcache.IdentDir = origIdent })

	repo := t.TempDir()
	names := []string{"a.py", "b.py", "c.py", "README.md"}
	old := time.Now().Add(-time.Hour) // outside the stat gate's racy window
	for i, n := range names {
		p := filepath.Join(repo, n)
		if err := os.WriteFile(p, []byte("def f"+strconv.Itoa(i)+"():\n    return "+strconv.Itoa(i)+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, old, old); err != nil {
			t.Fatal(err)
		}
	}
	storePath := filepath.Join(t.TempDir(), "fathom.sqlite")
	ingest := func() int {
		t.Helper()
		var out, errBuf bytes.Buffer
		if code := cmdIngest([]string{"--service", "svc", "--path", repo, "--store", storePath}, &out, &errBuf); code != 0 {
			t.Fatalf("ingest exit %d: stdout=%s stderr=%s", code, out.String(), errBuf.String())
		}
		m := statSkippedLine.FindStringSubmatch(out.String())
		if m == nil {
			return 0
		}
		n, _ := strconv.Atoi(m[1])
		return n
	}

	if n := ingest(); n != 0 {
		t.Fatalf("first ingest stat-skipped %d; want every file read", n)
	}
	if n := ingest(); n != len(names) {
		t.Fatalf("second one-shot ingest of an unchanged tree stat-skipped %d; want all %d (nothing read)", n, len(names))
	}
	later := time.Now().Add(-30 * time.Minute)
	if err := os.Chtimes(filepath.Join(repo, "b.py"), later, later); err != nil {
		t.Fatal(err)
	}
	if n := ingest(); n != len(names)-1 {
		t.Fatalf("after touching one file stat-skipped %d; want %d (only that file read)", n, len(names)-1)
	}
}
