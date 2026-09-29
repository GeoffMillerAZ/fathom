package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func withTempBashDoubtDir(t *testing.T) {
	t.Helper()
	orig := bashDoubtDir
	bashDoubtDir = filepath.Join(t.TempDir(), "fathom-bash-doubt")
	t.Cleanup(func() { bashDoubtDir = orig })
}

// EW-1: the doubt marker is scoped to (session, directory), expires, and
// does not exist for a session with no id (a predictable key with no session
// scope could be planted in a shared temp dir to silence the line).
func TestBashDoubtMarker(t *testing.T) {
	withTempBashDoubtDir(t)
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)

	if bashDoubtServed("s1", "/repo/internal/mcp", now) {
		t.Fatal("a marker that was never written reads as served")
	}
	markBashDoubtServed("s1", "/repo/internal/mcp", now)
	if !bashDoubtServed("s1", "/repo/internal/mcp", now.Add(time.Minute)) {
		t.Error("the marker is not readable right after it was written")
	}
	if bashDoubtServed("s1", "/repo/internal/store", now) {
		t.Error("a marker for one directory must not cover another")
	}
	if bashDoubtServed("s2", "/repo/internal/mcp", now) {
		t.Error("a marker for one session must not cover another")
	}
	if bashDoubtServed("s1", "/repo/internal/mcp", now.Add(bashDoubtTTL+time.Minute)) {
		t.Error("the marker outlived its TTL")
	}

	markBashDoubtServed("", "/repo/x", now)
	if bashDoubtServed("", "/repo/x", now) {
		t.Error("a session with no id must never read as served")
	}
	if entries, err := os.ReadDir(bashDoubtDir); err != nil || len(entries) != 1 {
		t.Errorf("markers on disk = %d (err %v), want 1 (none for the session-less write)", len(entries), err)
	}
}

// A planted symlink at the marker's path is never followed.
func TestBashDoubtMarkerRefusesSymlink(t *testing.T) {
	withTempBashDoubtDir(t)
	now := time.Now()
	if err := os.MkdirAll(bashDoubtDir, 0o700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "planted")
	if err := os.WriteFile(target, []byte(now.UTC().Format(time.RFC3339)), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, bashDoubtPath("s1", "/repo")); err != nil {
		t.Fatal(err)
	}
	if bashDoubtServed("s1", "/repo", now) {
		t.Fatal("a symlinked marker was honored")
	}
}
