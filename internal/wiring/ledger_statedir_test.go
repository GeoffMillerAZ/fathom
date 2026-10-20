package wiring

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/projectdir"
	"github.com/geoffmilleraz/fathom/internal/statedir"
)

// TestLedgerReadsLegacyWritesStateDir (FD-32): owned.json lives in the state
// directory. A legacy <root>/.fathom/owned.json is read only while the new
// path is absent, every write goes to the new path, and the legacy copy is
// never written or deleted (I63).
func TestLedgerReadsLegacyWritesStateDir(t *testing.T) {
	root := t.TempDir()
	state := filepath.Join(t.TempDir(), "state")
	t.Setenv(statedir.EnvOverride, state)

	legacy := projectdir.Owned.Path(root)
	fresh := filepath.Join(state, string(projectdir.Owned))

	old := NewLedger()
	old.Record("a.json", OwnedKey{ID: "k1", Digest: "d1", Wrote: "2026-01-01T00:00:00Z"})
	if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
		t.Fatal(err)
	}
	// Seed the legacy copy through the state path, then move it by hand, so
	// the test does not depend on a writer that no longer targets it.
	if err := SaveLedger(root, old); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(fresh)
	if err != nil {
		t.Fatalf("SaveLedger did not write the state directory: %v", err)
	}
	if err := os.Remove(fresh); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, data, 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := LoadLedger(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(OwnedKeys(got, "a.json")) != 1 {
		t.Fatalf("legacy copy not read while the new path is absent: %+v", got)
	}

	got.Record("a.json", OwnedKey{ID: "k2", Digest: "d2", Wrote: "2026-01-02T00:00:00Z"})
	if err := SaveLedger(root, got); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Fatalf("write did not land in the state directory: %v", err)
	}
	after, err := os.ReadFile(legacy)
	if err != nil || string(after) != string(data) {
		t.Fatalf("legacy copy was touched or deleted: err=%v", err)
	}

	// With both present, the new path wins.
	again, err := LoadLedger(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(OwnedKeys(again, "a.json")) != 2 {
		t.Fatalf("new path does not win over the legacy copy: %+v", again)
	}
	if !LedgerLegacyStale(root) {
		t.Fatal("LedgerLegacyStale false with both copies present")
	}
}
