package main

// daemonstdio_test.go — DG-6: the daemon's own launchd-piped stdout/stderr
// must be truncated IN PLACE (never renamed out from under the fd launchd
// already holds), the write offset must reset to 0 (never leave a
// sparse-file hole of NUL bytes), and one prior generation must be kept.

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/dlog"
)

// openSameFile opens path for read/write and returns the *os.File — used so
// the test's "f" and "path" are provably os.SameFile, exactly the launchd
// shape truncateStdioIfNeeded exists for.
func openSameFile(t *testing.T, path string) *os.File {
	t.Helper()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

func TestTruncateStdioIfNeeded_UnderCap_NoOp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fathom-http.log")
	f := openSameFile(t, path)
	if _, err := f.WriteString("small"); err != nil {
		t.Fatal(err)
	}
	log := dlog.New(io.Discard, dlog.LevelDebug, time.Now)
	truncateStdioIfNeeded(f, path, 1000, log)

	fi, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if fi.Size() != int64(len("small")) {
		t.Errorf("size = %d after a no-op rotation, want unchanged (%d)", fi.Size(), len("small"))
	}
	if _, err := os.Stat(path + ".1"); err == nil {
		t.Error("expected no backup file for a rotation that never fired")
	}
}

// TestTruncateStdioIfNeeded_OverCap_ResetsOffsetWithoutSparseHole is the
// load-bearing proof: after truncation, a subsequent write through the SAME
// still-open handle must land at byte 0, not at the old (pre-truncate)
// length — a Truncate without a Seek reset would leave the write offset
// where it was, and the next write would create a sparse hole of NUL bytes
// from 0 up to the old offset before the new bytes ever appeared.
func TestTruncateStdioIfNeeded_OverCap_ResetsOffsetWithoutSparseHole(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fathom-http.log")
	f := openSameFile(t, path)

	original := strings.Repeat("x", 200)
	if _, err := f.WriteString(original); err != nil {
		t.Fatal(err)
	}

	log := dlog.New(io.Discard, dlog.LevelDebug, time.Now)
	truncateStdioIfNeeded(f, path, 100, log)

	// The prior generation must be preserved byte-for-byte.
	backup, err := os.ReadFile(path + ".1")
	if err != nil {
		t.Fatalf("reading backup: %v", err)
	}
	if string(backup) != original {
		t.Errorf("backup = %q, want the original %d bytes", backup, len(original))
	}

	// The live file must now be empty...
	fi, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if fi.Size() != 0 {
		t.Fatalf("size after truncate = %d, want 0", fi.Size())
	}

	// ...and the NEXT write through the SAME handle must land at byte 0,
	// not at the old length — this is the sparse-file-hole regression this
	// test exists to catch.
	next := "new-generation"
	if _, err := f.WriteString(next); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != next {
		t.Errorf("file content after the post-truncate write = %q (len %d), want exactly %q with no leading NUL padding",
			got, len(got), next)
	}
}

// TestTruncateStdioIfNeeded_UnrelatedFile_LeavesItAlone pins the
// os.SameFile guard: a handle whose path does not resolve to the SAME file
// (a manual run with stdout redirected elsewhere, or no launchd file at
// that path at all) must never be truncated or backed up — there is
// nothing there to safely restore, and truncating an unrelated file wipe
// out unrelated log content.
func TestTruncateStdioIfNeeded_UnrelatedFile_LeavesItAlone(t *testing.T) {
	dir := t.TempDir()
	actual := filepath.Join(dir, "actual-stdout.log")
	f := openSameFile(t, actual)
	content := strings.Repeat("z", 200)
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}

	// unrelated is a DIFFERENT file that happens to share the "expected"
	// launchd path name but is not the file f is writing to.
	unrelated := filepath.Join(dir, "fathom-http.log")
	if err := os.WriteFile(unrelated, []byte("unrelated content"), 0o644); err != nil {
		t.Fatal(err)
	}

	log := dlog.New(io.Discard, dlog.LevelDebug, time.Now)
	truncateStdioIfNeeded(f, unrelated, 100, log)

	fi, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if fi.Size() != int64(len(content)) {
		t.Errorf("f was mutated (size %d, want %d) despite not being os.SameFile as the rotation target", fi.Size(), len(content))
	}
	if _, err := os.Stat(unrelated + ".1"); err == nil {
		t.Error("an unrelated file must never be backed up as if it were the daemon's own log")
	}
}

func TestRotateDaemonStdio_NoHomeDir_NoOp(t *testing.T) {
	// HOME resolution failing (os.UserHomeDir) must degrade to a silent
	// no-op, never a panic — same discipline as every other best-effort
	// beat-log operation in this codebase.
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "") // windows' os.UserHomeDir fallback
	log := dlog.New(io.Discard, dlog.LevelDebug, time.Now)
	rotateDaemonStdio(nil, nil, 100, log) // must not panic
}
