package wiring

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLedgerWritersLeaveHookAdoptionPending(t *testing.T) {
	for _, version := range []int{0, 1} {
		ledger := Ledger{Version: version}
		ledger.Record("settings.json", OwnedKey{ID: "key", Digest: "value"})
		if ledger.Version != 1 || !ledger.NeedsHookAdoption() {
			t.Errorf("Record consumed adoption: initial=%d version=%d", version, ledger.Version)
		}
		root := t.TempDir()
		if err := SaveLedger(root, Ledger{Version: version}); err != nil {
			t.Fatal(err)
		}
		got, err := LoadLedger(root)
		if err != nil || got.Version != 1 || !got.NeedsHookAdoption() {
			t.Errorf("SaveLedger consumed adoption: initial=%d version=%d err=%v", version, got.Version, err)
		}
	}
	root := t.TempDir()
	ledger, err := LoadLedger(root)
	if err != nil || ledger.Version != 1 || !ledger.NeedsHookAdoption() {
		t.Errorf("absent ledger consumed adoption: version=%d err=%v", ledger.Version, err)
	}
	if ledger := NewLedger(); ledger.Version != 1 || !ledger.NeedsHookAdoption() {
		t.Errorf("NewLedger consumed adoption: version=%d", ledger.Version)
	}
}

func TestApplyHookMigrationWithoutWrites(t *testing.T) {
	root := t.TempDir()
	if err := SaveLedger(root, Ledger{Version: 1}); err != nil {
		t.Fatal(err)
	}
	res, err := ApplyWiring(root, Spec{}, ApplyOptions{Now: fixedClock(), MigrateHooks: true})
	if err != nil {
		t.Fatal(err)
	}
	ledger, err := LoadLedger(root)
	if err != nil || ledger.Version != ledgerVersion || ledger.NeedsHookAdoption() {
		t.Fatalf("migration version=%d, err=%v", ledger.Version, err)
	}
	if len(res.Applied) != 0 || res.Receipt == nil || len(res.Receipt.Files) != 1 || res.Receipt.Files[0].Path != ledgerRelPath {
		t.Fatalf("migration should receipt only the ledger: %+v", res)
	}
	if _, err := Undo(root, res.Receipt.ID, ApplyOptions{Now: fixedClock()}); err != nil {
		t.Fatal(err)
	}
	ledger, err = LoadLedger(root)
	if err != nil || ledger.Version != 1 || !ledger.NeedsHookAdoption() {
		t.Fatalf("undo lost pending migration: version=%d, err=%v", ledger.Version, err)
	}
}

func TestTornApplyKeepsHookMigrationPending(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: a read-only file would still be writable")
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "lefthook.yml"), []byte("pre-push:\n  commands:\n    mine:\n      run: echo hi\n"), 0o444); err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyWiring(root, Spec{Keys: demoSpec().Keys[:1]}, ApplyOptions{Now: fixedClock(), MigrateHooks: true}); err == nil {
		t.Fatal("expected the read-only file to fail the apply")
	}
	if ledger, err := LoadLedger(root); err != nil || !ledger.NeedsHookAdoption() {
		t.Fatalf("a failed apply consumed the migration: version=%d err=%v", ledger.Version, err)
	}
}
