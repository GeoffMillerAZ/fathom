package wiring

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveLedgerReplacesAtomically(t *testing.T) {
	root := t.TempDir()
	ledger := NewLedger()
	ledger.Record("settings.json", OwnedKey{ID: "first", Digest: "old"})
	if err := SaveLedger(root, ledger); err != nil {
		t.Fatal(err)
	}
	path := LedgerPath(root)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// A reader holding the old inode must never see a partially rewritten ledger.
	prior := filepath.Join(filepath.Dir(path), "prior.json") // same device as the ledger
	if err := os.Link(path, prior); err != nil {
		t.Fatal(err)
	}
	ledger.Record("settings.json", OwnedKey{ID: "second", Digest: "new"})
	if err := SaveLedger(root, ledger); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(prior); err != nil || string(got) != string(before) {
		t.Fatalf("old ledger inode changed: %s (err=%v)", got, err)
	}
	got, err := LoadLedger(root)
	if err != nil || len(OwnedKeys(got, "settings.json")) != 2 {
		t.Fatalf("replacement ledger = %+v (err=%v)", got, err)
	}
}

func TestSaveLedgerFailedWriteLeavesOldIntact(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: a read-only directory would still be writable")
	}
	root := t.TempDir()
	ledger := NewLedger()
	ledger.Record("settings.json", OwnedKey{ID: "first", Digest: "old"})
	if err := SaveLedger(root, ledger); err != nil {
		t.Fatal(err)
	}
	path := LedgerPath(root)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Dir(path)
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(dir, 0o755); err != nil {
			t.Error(err)
		}
	})
	ledger.Record("settings.json", OwnedKey{ID: "second", Digest: "new"})
	if err := SaveLedger(root, ledger); err == nil {
		t.Error("write succeeded without permission to create a replacement")
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != string(before) {
		t.Fatalf("failed write changed old ledger: %s (err=%v)", got, err)
	}
}
