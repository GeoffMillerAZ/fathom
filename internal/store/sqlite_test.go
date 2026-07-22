package store

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
)

func TestSQLiteStoreContract_InMemory(t *testing.T) {
	s, err := NewSQLiteStore("") // in-memory SQLite
	if err != nil {
		t.Fatalf("NewSQLiteStore in-memory: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	storeContract(t, s)
}

func TestSQLiteStoreContract_OnDisk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.sqlite")
	s, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatalf("NewSQLiteStore on-disk: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	storeContract(t, s)
}

// TestSQLiteStoreImplementsStore is a compile-time check that *SQLiteStore
// satisfies the Store interface.
func TestSQLiteStoreImplementsStore(t *testing.T) {
	var _ Store = (*SQLiteStore)(nil)
}

// TestSQLite_WALActuallyEngages asserts PRAGMA journal_mode really reports
// "wal" on an opened on-disk store — the DSN pragma wiring is easy to get
// syntactically "working" (no error) while silently landing on SQLite's
// default rollback journal instead, which would quietly forfeit the whole
// point of this backend (concurrent readers beside an open writer). An
// in-memory database is exempt: SQLite does not support WAL for ":memory:"
// (it silently stays "memory"), so this only asserts the on-disk case.
func TestSQLite_WALActuallyEngages(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wal.sqlite")
	s, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	var mode string
	if err := s.db.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatalf("PRAGMA journal_mode: %v", err)
	}
	if mode != "wal" {
		t.Fatalf("journal_mode = %q, want %q — WAL pragma did not engage", mode, "wal")
	}
}

// TestSQLite_ReaderDuringWriter is the ADR-020 cross-process concurrency
// proof (SS-3): the property DuckDB's cross-process single-writer lock never
// gave fathom, and the reason for this backend swap. A true separate-process
// test is heavier than this package needs; the honest approximation here is
// two independent *sql.DB handles (distinct connections/pools, exactly as
// two OS processes would each hold their own) opened against the SAME
// on-disk file. The writer starts an explicit write transaction and leaves
// it open (uncommitted); the reader, on its own separate connection,
// completes a SELECT against the same table while that write transaction is
// still open. Under DuckDB's single-writer model this would deadlock or
// error; under SQLite WAL it must succeed immediately, because WAL readers
// see a consistent snapshot of the last-committed state and never block
// behind an in-progress writer. This proves multi-connection WAL semantics
// on this machine; the daemon/CLI case is separate OS processes against the
// same file, which get identical file-level WAL guarantees from SQLite.
func TestSQLite_ReaderDuringWriter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "concurrent.sqlite")

	// First connection: opens the store (creates schema, seeds one row) and
	// then holds a write transaction open without committing.
	writer, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatalf("NewSQLiteStore writer: %v", err)
	}
	defer writer.Close()

	ctx := context.Background()
	seed := []core.Observation{
		{
			ServiceID:    "svc-concurrent",
			SourceRef:    "pkg/a.go",
			SourceKind:   core.KindGoFunc,
			EvidenceTier: core.TierVerified,
			Symbol:       "Seeded",
			Claim:        "func Seeded()",
			ContentHash:  "seed",
			IngestedAt:   time.Now().UTC(),
		},
	}
	if err := writer.PutObservations(ctx, seed); err != nil {
		t.Fatalf("seed PutObservations: %v", err)
	}

	// Second, independent *sql.DB handle against the same file — the
	// separate-connection stand-in for a second OS process.
	reader, err := sql.Open("sqlite", sqlitePragmaDSN(path))
	if err != nil {
		t.Fatalf("open reader handle: %v", err)
	}
	defer reader.Close()

	// Writer: BEGIN IMMEDIATE (acquires the write lock now, not lazily on
	// first write) and insert a row, but do NOT commit yet.
	tx, err := writer.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("writer BeginTx: %v", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO observations
		(service_id, concern_id, source_ref, source_kind, evidence_tier, symbol, package, claim, content_hash, calls, complexity, line, ingested_at)
		VALUES (?, '', 'pkg/b.go', 'GoFunc', 'verified', 'Uncommitted', '', 'func Uncommitted()', 'uncommitted', '[]', 0, 0, ?)`,
		"svc-concurrent", time.Now().UnixNano()); err != nil {
		_ = tx.Rollback()
		t.Fatalf("writer insert inside open tx: %v", err)
	}
	// Deliberately left uncommitted across the reader's query below.

	done := make(chan error, 1)
	go func() {
		var n int
		qErr := reader.QueryRowContext(ctx, `SELECT count(*) FROM observations WHERE service_id = ?`, "svc-concurrent").Scan(&n)
		if qErr == nil && n < 1 {
			qErr = fmt.Errorf("reader saw %d rows, want at least the committed seed row", n)
		}
		done <- qErr
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("reader query while writer transaction open: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("reader blocked for 5s behind an open (uncommitted) writer transaction — WAL concurrent-read semantics did not hold")
	}

	// Clean up: roll back the writer's uncommitted transaction.
	if err := tx.Rollback(); err != nil {
		t.Fatalf("writer Rollback: %v", err)
	}
}
