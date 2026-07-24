package store

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"sync"
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

// TestSQLiteStore_LegacySchemaGainsToolIdentityColumns is the ADR-021 §2 /
// critic-F1 migration guard: SQLite stores created by earlier slices exist
// on disk WITHOUT tool_kind/generated, and SQLite has no ADD COLUMN IF NOT
// EXISTS — createSchema must detect the missing columns via PRAGMA
// table_info and add them, so reopening a pre-toolident store neither fails
// on INSERT nor silently drops the fields.
func TestSQLiteStore_LegacySchemaGainsToolIdentityColumns(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "legacy.sqlite")

	// Hand-create a store file with the PRE-toolident observations schema
	// (no tool_kind, no generated).
	raw, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatalf("open raw: %v", err)
	}
	if _, err := raw.Exec(`
CREATE TABLE observations (
    service_id    TEXT    NOT NULL,
    concern_id    TEXT    NOT NULL DEFAULT '',
    source_ref    TEXT    NOT NULL,
    source_kind   TEXT    NOT NULL,
    evidence_tier TEXT    NOT NULL,
    symbol        TEXT    NOT NULL DEFAULT '',
    package       TEXT    NOT NULL DEFAULT '',
    claim         TEXT    NOT NULL DEFAULT '',
    content_hash  TEXT    NOT NULL DEFAULT '',
    calls         TEXT    NOT NULL DEFAULT '[]',
    complexity    INTEGER NOT NULL DEFAULT 0,
    line          INTEGER NOT NULL DEFAULT 0,
    ingested_at   BIGINT  NOT NULL
)`); err != nil {
		t.Fatalf("create legacy table: %v", err)
	}
	if _, err := raw.Exec(
		`INSERT INTO observations (service_id, source_ref, source_kind, evidence_tier, symbol, ingested_at)
		 VALUES ('legacy-svc', 'a.go', 'GoFunc', 'verified', 'OldFunc', 1)`,
	); err != nil {
		t.Fatalf("insert legacy row: %v", err)
	}
	if err := raw.Close(); err != nil {
		t.Fatalf("close raw: %v", err)
	}

	// Reopen through the real constructor — migration must fire.
	s, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatalf("NewSQLiteStore on legacy file: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	ctx := context.Background()
	// The pre-existing row reads back with zero-value fields.
	got, err := s.QueryObservations(ctx, "legacy-svc", "")
	if err != nil {
		t.Fatalf("QueryObservations legacy row: %v", err)
	}
	if len(got) != 1 || got[0].ToolKind != "" || got[0].Generated {
		t.Fatalf("legacy row: got %+v, want 1 row with zero-value ToolKind/Generated", got)
	}
	// New writes round-trip both fields.
	if err := s.PutObservations(ctx, []core.Observation{{
		ServiceID: "legacy-svc", SourceRef: "b.yaml", SourceKind: core.KindK8sWorkload,
		EvidenceTier: core.TierVerified, Symbol: "Deployment/x",
		ToolKind: "k8s", Generated: true, IngestedAt: time.Now().UTC(),
	}}); err != nil {
		t.Fatalf("PutObservations after migration: %v", err)
	}
	got, err = s.QueryObservations(ctx, "legacy-svc", "")
	if err != nil {
		t.Fatalf("QueryObservations after write: %v", err)
	}
	var found bool
	for _, o := range got {
		if o.Symbol == "Deployment/x" {
			found = true
			if o.ToolKind != "k8s" || !o.Generated {
				t.Errorf("migrated store round-trip: got (%q, %v), want (\"k8s\", true)", o.ToolKind, o.Generated)
			}
		}
	}
	if !found {
		t.Fatal("new observation not returned after migration")
	}
}

// TestSQLiteStore_LegacySchemaGainsEscapeNoteColumn is the A2-1/D3-1 bounce
// finding-1 regression: escape_note was added only to the CREATE TABLE IF
// NOT EXISTS shape — a no-op on every pre-existing store, including the
// live fleet store, where CompileRuns then errored with "no such column:
// escape_note" (breaking `layers list --store`, silently fail-opening the
// compile idempotency guard, and killing the final RecordCompileRun after
// the LLM spend). The PRAGMA-based migration must add the column to
// legacy-shape stores on open.
func TestSQLiteStore_LegacySchemaGainsEscapeNoteColumn(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "legacy-runs.sqlite")

	// Hand-create a store file with the PRE-escape_note compile_runs schema.
	raw, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatalf("open raw: %v", err)
	}
	if _, err := raw.Exec(`
CREATE TABLE compile_runs (
    run_id             TEXT    NOT NULL DEFAULT '',
    service_id         TEXT    NOT NULL,
    layer              TEXT    NOT NULL DEFAULT '',
    corpus_watermark   TEXT    NOT NULL DEFAULT '',
    model              TEXT    NOT NULL DEFAULT '',
    prompt_version     TEXT    NOT NULL DEFAULT '',
    dry_run            BOOLEAN NOT NULL DEFAULT FALSE,
    tokens_in          INTEGER NOT NULL DEFAULT 0,
    tokens_out         INTEGER NOT NULL DEFAULT 0,
    cost_usd           DOUBLE  NOT NULL DEFAULT 0,
    duration_ms        BIGINT  NOT NULL DEFAULT 0,
    sections_total     INTEGER NOT NULL DEFAULT 0,
    sections_compiled  INTEGER NOT NULL DEFAULT 0,
    sections_failed    INTEGER NOT NULL DEFAULT 0,
    claims_emitted     INTEGER NOT NULL DEFAULT 0,
    claims_rejected    INTEGER NOT NULL DEFAULT 0,
    ran_at             BIGINT  NOT NULL DEFAULT 0,
    refused            BOOLEAN NOT NULL DEFAULT FALSE,
    refusal_reason     TEXT    NOT NULL DEFAULT '',
    budget_stopped     BOOLEAN NOT NULL DEFAULT FALSE
)`); err != nil {
		t.Fatalf("create legacy compile_runs table: %v", err)
	}
	if _, err := raw.Exec(
		`INSERT INTO compile_runs (run_id, service_id, layer, corpus_watermark, prompt_version, ran_at)
		 VALUES ('legacy-run', 'legacy-svc', 'obligations', 'wm-old', 'obligations/old', 1)`,
	); err != nil {
		t.Fatalf("insert legacy run: %v", err)
	}
	if err := raw.Close(); err != nil {
		t.Fatalf("close raw: %v", err)
	}

	// Reopen through the real constructor — the migration must fire.
	s, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatalf("NewSQLiteStore on legacy file: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	ctx := context.Background()
	// The pre-existing row reads back with a zero-value Escape.
	got, err := s.CompileRuns(ctx, "legacy-svc", "")
	if err != nil {
		t.Fatalf("CompileRuns on migrated legacy store: %v", err)
	}
	if len(got) != 1 || got[0].RunID != "legacy-run" || got[0].Escape != "" {
		t.Fatalf("legacy run: got %+v, want 1 row with empty Escape", got)
	}
	// New writes round-trip Escape.
	if err := s.RecordCompileRun(ctx, core.CompileRun{
		RunID: "new-run", ServiceID: "legacy-svc", Layer: "obligations",
		CorpusWatermark: "wm-new", PromptVersion: "obligations/new",
		RanAt:  time.Unix(2, 0).UTC(),
		Escape: "force: idempotent recompile of run legacy-run",
	}); err != nil {
		t.Fatalf("RecordCompileRun after migration: %v", err)
	}
	got, err = s.CompileRuns(ctx, "legacy-svc", "")
	if err != nil {
		t.Fatalf("CompileRuns after write: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 runs after write, got %d", len(got))
	}
	if got[0].RunID != "new-run" || got[0].Escape != "force: idempotent recompile of run legacy-run" {
		t.Errorf("migrated store Escape round-trip: got (%q, %q)", got[0].RunID, got[0].Escape)
	}
}

// TestSQLiteStore_LegacySchemaGainsEndLineColumn is T5-5's migration guard
// (docs/plans/trust-and-tokenomics.md), the same PRAGMA-based-migration
// class as TestSQLiteStore_LegacySchemaGainsToolIdentityColumns above: a
// store created before end_line existed must gain the column on reopen
// (never "no such column: end_line" on the next write), and the pre-existing
// row must read back with EndLine's honest zero value rather than erroring.
func TestSQLiteStore_LegacySchemaGainsEndLineColumn(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "legacy-endline.sqlite")

	// Hand-create a store file with the PRE-end_line observations schema.
	raw, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatalf("open raw: %v", err)
	}
	if _, err := raw.Exec(`
CREATE TABLE observations (
    service_id    TEXT    NOT NULL,
    concern_id    TEXT    NOT NULL DEFAULT '',
    source_ref    TEXT    NOT NULL,
    source_kind   TEXT    NOT NULL,
    evidence_tier TEXT    NOT NULL,
    symbol        TEXT    NOT NULL DEFAULT '',
    package       TEXT    NOT NULL DEFAULT '',
    claim         TEXT    NOT NULL DEFAULT '',
    content_hash  TEXT    NOT NULL DEFAULT '',
    calls         TEXT    NOT NULL DEFAULT '[]',
    complexity    INTEGER NOT NULL DEFAULT 0,
    line          INTEGER NOT NULL DEFAULT 0,
    ingested_at   BIGINT  NOT NULL,
    tool_kind     TEXT    NOT NULL DEFAULT '',
    generated     INTEGER NOT NULL DEFAULT 0
)`); err != nil {
		t.Fatalf("create legacy table: %v", err)
	}
	if _, err := raw.Exec(
		`INSERT INTO observations (service_id, source_ref, source_kind, evidence_tier, symbol, line, ingested_at)
		 VALUES ('legacy-svc', 'README.md', 'DocChunk', 'doc-claimed', 'Old Heading', 5, 1)`,
	); err != nil {
		t.Fatalf("insert legacy row: %v", err)
	}
	if err := raw.Close(); err != nil {
		t.Fatalf("close raw: %v", err)
	}

	// Reopen through the real constructor — migration must fire.
	s, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatalf("NewSQLiteStore on legacy file: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	ctx := context.Background()
	// The pre-existing row reads back with a zero-value EndLine, not an error.
	got, err := s.QueryObservations(ctx, "legacy-svc", "")
	if err != nil {
		t.Fatalf("QueryObservations legacy row: %v", err)
	}
	if len(got) != 1 || got[0].Symbol != "Old Heading" || got[0].Line != 5 || got[0].EndLine != 0 {
		t.Fatalf("legacy row: got %+v, want 1 row (Line=5, EndLine=0)", got)
	}
	// New writes round-trip EndLine.
	if err := s.PutObservations(ctx, []core.Observation{{
		ServiceID: "legacy-svc", SourceRef: "README.md", SourceKind: core.KindDocChunk,
		EvidenceTier: core.TierDocClaimed, Symbol: "New Heading",
		Line: 10, EndLine: 14, IngestedAt: time.Now().UTC(),
	}}); err != nil {
		t.Fatalf("PutObservations after migration: %v", err)
	}
	got, err = s.QueryObservations(ctx, "legacy-svc", "")
	if err != nil {
		t.Fatalf("QueryObservations after write: %v", err)
	}
	var found bool
	for _, o := range got {
		if o.Symbol == "New Heading" {
			found = true
			if o.Line != 10 || o.EndLine != 14 {
				t.Errorf("migrated store round-trip: got (Line=%d, EndLine=%d), want (10, 14)", o.Line, o.EndLine)
			}
		}
	}
	if !found {
		t.Fatal("new observation not returned after migration")
	}
}

// TestSQLiteStore_ParallelReaders is the F1 regression: the Store contract
// guarantees safety for concurrent reads, and modernc's ":memory:" gives
// every physical pool connection its OWN private database — so an uncapped
// pool on the in-memory store made 15 of 16 concurrent readers fail with
// "no such table". The fix serializes the in-memory pool
// (SetMaxOpenConns(1) — an ephemeral test/session store needs no reader
// concurrency) while the file-backed pool stays uncapped; this test runs
// both modes: memory proves serialized correctness, file proves genuinely
// concurrent readers all see the data.
func TestSQLiteStore_ParallelReaders(t *testing.T) {
	for _, tc := range []struct {
		name string
		path func(t *testing.T) string
	}{
		{"memory", func(t *testing.T) string { return "" }},
		{"file", func(t *testing.T) string { return filepath.Join(t.TempDir(), "par.sqlite") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, err := NewSQLiteStore(tc.path(t))
			if err != nil {
				t.Fatalf("NewSQLiteStore: %v", err)
			}
			t.Cleanup(func() { _ = s.Close() })

			ctx := context.Background()
			if err := s.PutObservations(ctx, []core.Observation{
				{
					ServiceID:    "svc-par",
					SourceRef:    "pkg/p.go",
					SourceKind:   core.KindGoFunc,
					EvidenceTier: core.TierVerified,
					Symbol:       "Par",
					Claim:        "func Par()",
					ContentHash:  "h",
					IngestedAt:   time.Now().UTC(),
				},
			}); err != nil {
				t.Fatalf("seed PutObservations: %v", err)
			}

			const readers = 16
			var wg sync.WaitGroup
			errs := make(chan error, readers)
			for i := 0; i < readers; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					obs, err := s.QueryObservations(ctx, "svc-par", "")
					if err != nil {
						errs <- err
						return
					}
					if len(obs) != 1 {
						errs <- fmt.Errorf("reader saw %d observations, want 1", len(obs))
					}
				}()
			}
			wg.Wait()
			close(errs)
			for err := range errs {
				t.Errorf("concurrent reader: %v", err)
			}
		})
	}
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

	// Writer: start a (deferred) transaction and execute an INSERT inside
	// it — the INSERT is what acquires SQLite's write lock — then leave the
	// transaction open, uncommitted, across the reader's query below.
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
