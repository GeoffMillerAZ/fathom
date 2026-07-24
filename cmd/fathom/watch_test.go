package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestWatchMissingService verifies that omitting --service returns 1.
func TestWatchMissingService(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := run([]string{"watch"}, &out, &errBuf)
	if code == 0 {
		t.Fatal("expected nonzero exit when --service is missing")
	}
}

// TestWatchMissingPath verifies that omitting --path returns 1.
func TestWatchMissingPath(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := run([]string{"watch", "--service", "svc"}, &out, &errBuf)
	if code == 0 {
		t.Fatal("expected nonzero exit when --path is missing")
	}
}

// TestWatchOnce_NoChange verifies that watchOnce does nothing when no files
// have changed since the previous poll.
func TestWatchOnce_NoChange(t *testing.T) {
	paymentPath := fauxPaymentPathForTest(t)
	store := filepath.Join(t.TempDir(), "watch.sqlite")

	// Record current mtimes so watchOnce sees no delta.
	state := buildMtimeState(t, paymentPath)

	var out, errBuf bytes.Buffer
	reingest, err := watchOnce(watchConfig{
		service:   "payment-service",
		path:      paymentPath,
		storePath: store,
		mode:      "local",
	}, state, &out, &errBuf)
	if err != nil {
		t.Fatalf("watchOnce: unexpected error: %v", err)
	}
	if reingest {
		t.Error("watchOnce reported reingest when no files changed")
	}
	_ = out.String()
}

// TestWatchOnce_Changed verifies that watchOnce triggers an ingest when a
// file's mtime is newer than the recorded state.
func TestWatchOnce_Changed(t *testing.T) {
	paymentPath := fauxPaymentPathForTest(t)
	store := filepath.Join(t.TempDir(), "watch.sqlite")

	// Build state with a zero time so every file appears newer.
	zeroState := mtimeState{}

	var out, errBuf bytes.Buffer
	reingest, err := watchOnce(watchConfig{
		service:   "payment-service",
		path:      paymentPath,
		storePath: store,
		mode:      "faux",
	}, zeroState, &out, &errBuf)
	if err != nil {
		t.Fatalf("watchOnce: unexpected error: %v", err)
	}
	if !reingest {
		t.Error("watchOnce should have detected changes and re-ingested")
	}
	if out.String() == "" {
		t.Error("watchOnce should produce output on reingest")
	}
}

// TestWatchOnce_WrittenFile creates a temp file, records the state, touches
// the file to advance its mtime, then calls watchOnce — expecting a reingest.
func TestWatchOnce_WrittenFile(t *testing.T) {
	// Use a temp dir with a single .go file so we can control mtimes precisely.
	dir := t.TempDir()
	goFile := filepath.Join(dir, "hello.go")
	if err := os.WriteFile(goFile, []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Set the mtime to 2 seconds ago so the upcoming touch is clearly newer.
	past := time.Now().Add(-2 * time.Second)
	if err := os.Chtimes(goFile, past, past); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}

	store := filepath.Join(t.TempDir(), "watch.sqlite")

	// Build state with the current (old) mtime.
	state := buildMtimeState(t, dir)

	// Now advance the mtime to "now".
	now := time.Now()
	if err := os.Chtimes(goFile, now, now); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}

	var out, errBuf bytes.Buffer
	reingest, err := watchOnce(watchConfig{
		service:   "hello-svc",
		path:      dir,
		storePath: store,
		mode:      "local",
	}, state, &out, &errBuf)
	if err != nil {
		t.Fatalf("watchOnce: %v", err)
	}
	if !reingest {
		t.Error("watchOnce should detect the touched file and reingest")
	}
	combined := out.String() + errBuf.String()
	if strings.Contains(combined, "error") {
		// It's fine if the store ingest runs but there's nothing to extract — we
		// just want no fatal error.
		t.Logf("combined output: %s", combined)
	}
}

// buildMtimeState walks dir and records current mtimes — helper for tests.
func buildMtimeState(t *testing.T, dir string) mtimeState {
	t.Helper()
	state, err := scanMtimes(dir)
	if err != nil {
		t.Fatalf("buildMtimeState: %v", err)
	}
	return state
}
