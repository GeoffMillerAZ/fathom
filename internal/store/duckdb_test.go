package store

import (
	"context"
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

// TestOpenDuckDBReadOnly is the F4a regression: migrate-store must open its
// SOURCE read-only — OpenDuckDB runs schema-migration DDL (ALTER TABLE) on
// whatever it opens, so using it for the source would mutate the very file
// the operator is trying to leave untouched. The read-only open must serve
// reads and refuse writes.
func TestOpenDuckDBReadOnly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ro.duckdb")

	// Seed via the normal read-write open, then close it fully.
	rw, err := OpenDuckDB(path)
	if err != nil {
		t.Fatalf("OpenDuckDB seed: %v", err)
	}
	ctx := context.Background()
	if err := rw.SetFingerprint(ctx, "svc-ro", "a.go", "hash-ro"); err != nil {
		t.Fatalf("seed SetFingerprint: %v", err)
	}
	if err := rw.Close(); err != nil {
		t.Fatalf("close seed store: %v", err)
	}

	ro, err := openDuckDBReadOnly(path)
	if err != nil {
		t.Fatalf("openDuckDBReadOnly: %v", err)
	}
	defer ro.Close()

	// Reads work.
	hash, found, err := ro.Fingerprint(ctx, "svc-ro", "a.go")
	if err != nil {
		t.Fatalf("Fingerprint via read-only handle: %v", err)
	}
	if !found || hash != "hash-ro" {
		t.Errorf("Fingerprint = (%q, %v), want (hash-ro, true)", hash, found)
	}

	// Writes must fail — the whole point of the read-only open.
	if err := ro.SetFingerprint(ctx, "svc-ro", "b.go", "nope"); err == nil {
		t.Fatal("SetFingerprint on a read-only handle succeeded; want an error")
	}
}
