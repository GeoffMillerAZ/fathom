package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestResolveReadStore pins FR-1 (docs/plans/exploration-ledger.md):
// service-aware plan discovery. Explicit --plan flags always win with no
// note; a service with no registered path reachable from here falls back
// to the CURRENT directory loudly — the note names the fallback, so the
// resume_as_code failure (one repo's plans silently verified against
// another repo's index, 2026-08-08) cannot recur unannounced.
func TestResolveReadStore(t *testing.T) {
	// (a) explicit flags win: no discovery, no note.
	files, note, err := discoverPlansForService(planFlags{"x.md"}, "svc", "http://127.0.0.1:1")
	if err != nil {
		t.Fatalf("explicit flags: %v", err)
	}
	if len(files) != 1 || files[0] != "x.md" {
		t.Fatalf("files = %v, want [x.md]", files)
	}
	if note != "" {
		t.Errorf("explicit flags must produce no scan note, got %q", note)
	}

	// (b) unreachable daemon + unknown service → cwd fallback, loudly.
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "docs", "plans"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "docs", "plans", "a.md"), []byte("# plan\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	files, note, err = discoverPlansForService(nil, "nope", "http://127.0.0.1:1")
	if err != nil {
		t.Fatalf("fallback discovery: %v", err)
	}
	if !strings.Contains(note, "scanning the current directory") {
		t.Errorf("cwd fallback must be loud, got note %q", note)
	}
	if len(files) != 1 || filepath.Base(files[0]) != "a.md" {
		t.Errorf("files = %v, want the cwd's docs/plans/a.md", files)
	}
}
