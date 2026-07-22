package store

import (
	"path/filepath"
	"testing"
)

func TestDuckStoreContract_InMemory(t *testing.T) {
	s, err := OpenDuckDB("") // in-memory DuckDB
	if err != nil {
		t.Fatalf("OpenDuckDB in-memory: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	storeContract(t, s)
}

func TestDuckStoreContract_OnDisk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.duckdb")
	s, err := OpenDuckDB(path)
	if err != nil {
		t.Fatalf("OpenDuckDB on-disk: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	storeContract(t, s)
}

// TestDuckStoreImplementsStore is a compile-time check that *DuckStore satisfies
// the Store interface.
func TestDuckStoreImplementsStore(t *testing.T) {
	var _ Store = (*DuckStore)(nil)
}
