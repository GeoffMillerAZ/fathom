package store

// sqlite.go — SQLiteStore: the handle, the store-format constants, the DSN
// and the constructor/Close. Its methods live beside it by concern, in
// sqlite_*.go (schema, observations, services, knowledge, claims,
// compileruns, typededges, plus the additive-table files), all in package
// store; the store stays pure Go (invariant I2).

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sync/atomic"

	_ "modernc.org/sqlite"
)

// openStoreCount is the process-lifetime count of SQLiteStore handles
// currently open (O108, cmd/fathom's /metrics fathom_open_stores):
// incremented on every successful NewSQLiteStore, decremented on every
// Close. A daemon that opens exactly one store at startup and never
// re-opens one should hold this at a constant 1 (+1 per mounted
// expert-module store) for its whole lifetime — a climbing value points
// directly at a per-request/per-refresh store leak (O108 candidate c)
// instead of leaving it to be inferred from RSS growth alone.
var openStoreCount int64 // atomic

// OpenStores returns the current count of open SQLiteStore handles in this
// process. See openStoreCount's doc for what a healthy value looks like.
func OpenStores() int64 {
	return atomic.LoadInt64(&openStoreCount)
}

// SQLiteStore is a pure-Go, SQLite-backed (modernc.org/sqlite, WAL mode)
// implementation of Store (ADR-020). Unlike the old DuckStore backend
// (deleted in SS-8/ADR-025) it does not cap the connection pool: SQLite's own single-writer lock serialises writes, and
// WAL mode lets any number of readers proceed concurrently with one writer —
// the property DuckDB's cross-process single-writer lacked and the reason
// for this backend's existence.
type SQLiteStore struct {
	db *sql.DB
	// path is the store file this handle was opened against ("" for an
	// in-memory database). Recorded because the physical half of a purge
	// (docs/design/scope-and-purge.md §3.2's last row) cannot be done
	// through the SQL handle alone: VACUUM needs a file to compact and the
	// raw-bytes probe needs one to read.
	path string
	// gate is the per-call suppression filter (scoped.go). Every
	// constructor initialises it via newScopeGate; the audit test
	// TestEveryStoreConstructorIsScoped fails any constructor that does
	// not.
	gate scopeGate
}

// storeFormatVersion is the format_version marker docs/spec/store-format-v0.md
// §2 requires every conforming store to carry, via SQLite's built-in PRAGMA
// user_version. 1 for this spec (v0's marker value). Bump only alongside a
// new spec version (v1+) that documents what changed — the marker exists so
// a future reader can identify which spec version wrote a store without
// guessing from column presence.
//
// Deliberately NOT 0: PRAGMA user_version defaults to 0 on every SQLite
// database ever created, including one this binary never touched — mapping
// spec v0 to marker 0 would make the marker vacuous (a reader could never
// distinguish a conforming store from an arbitrary SQLite file, and the
// stampFormatMarkers write-if-lower guard below would be structurally
// unreachable, since user_version is never negative in normal use). 0 is
// reserved to mean "pre-spec/unversioned" (see storeFormatVersionUnversioned)
// — a reader that sees 0 MUST fall back to schema introspection rather than
// trusting the marker.
const storeFormatVersion = 1

// storeFormatVersionUnversioned is SQLite's own default PRAGMA user_version.
// Documented as a named constant (rather than a bare 0) so its meaning —
// "no fathom writer has stamped this store yet" — is legible at every call
// site that compares against it.
const storeFormatVersionUnversioned = 0

// storeApplicationID is SQLite's PRAGMA application_id (docs/spec/
// store-format-v0.md §2): the file-TYPE identifier SQLite's own format
// reserves for exactly this purpose, distinct from user_version (which
// identifies WHICH VERSION of that type a given file is). Value: the ASCII
// bytes "FATH" read big-endian (0x46='F', 0x41='A', 0x54='T', 0x48='H') —
// a stable, human-verifiable mnemonic rather than an opaque hash. Fixed
// forever once published: changing it would break every existing reader's
// "is this a fathom store?" check.
const storeApplicationID = 0x46415448

// sqlitePragmaDSN appends the per-connection pragmas this backend requires
// (journal_mode=WAL, busy_timeout, synchronous=NORMAL, journal_size_limit
// 64 MiB) to path as DSN query
// parameters. modernc.org/sqlite applies "_pragma" query parameters to every
// new physical connection it opens (conn.go's newConn → applyQueryParams),
// so these apply uniformly across the whole (uncapped) pool, not just the
// first connection — the property a one-time "PRAGMA journal_mode=WAL"
// exec on a single *sql.DB handle would NOT guarantee once the pool grows.
func sqlitePragmaDSN(path string) string {
	q := url.Values{}
	// busy_timeout first by modernc convention (applied in that order
	// regardless; listed first here for readability).
	q.Add("_pragma", "busy_timeout(5000)")
	q.Add("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", "synchronous(NORMAL)")
	for _, p := range tenancyConnPragmas { // RT-78
		q.Add("_pragma", p)
	}
	return path + "?" + q.Encode()
}

// NewSQLiteStore opens (or creates) a SQLite database at path in WAL mode. Pass
// "" for an in-memory database (no WAL applies to memory databases; SQLite
// ignores the journal_mode pragma there). The schema is created fresh if it
// does not already exist — there is no legacy on-disk migration path for
// this backend (the old DuckStore backend, deleted in SS-8, carried that).
func NewSQLiteStore(path string) (*SQLiteStore, error) {
	dsn := ":memory:"
	if path != "" {
		if dir := filepath.Dir(path); dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return nil, fmt.Errorf("store.NewSQLiteStore mkdir %q: %w", dir, err)
			}
		}
		dsn = sqlitePragmaDSN(path)
	}
	raiseStatementJournalSpill()
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("store.NewSQLiteStore: %w", err)
	}
	// File-backed stores: uncapped read pool (ADR-020) — the
	// SetMaxOpenConns(1) the old DuckStore backend used to serialise
	// writes was DuckDB-specific and must NOT carry over; SQLite's own single-writer
	// lock (busy_timeout-backed) already serialises writes, and capping the
	// pool would throttle concurrent readers for no reason.
	//
	// In-memory stores are the one exception: every physical modernc
	// connection to ":memory:" opens its OWN private database, so a second
	// pool connection sees no tables at all — an uncapped pool made 15 of
	// 16 concurrent readers fail with "no such table" (F1). One connection
	// keeps the single shared database; an ephemeral test/session store
	// needs no reader concurrency.
	if path == "" {
		db.SetMaxOpenConns(1)
	}

	s := &SQLiteStore{db: db, path: path, gate: newScopeGate()}
	if err := applyTenancyPragmas(context.Background(), db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("store.NewSQLiteStore pragmas: %w", err)
	}
	if err := s.createSchema(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("store.NewSQLiteStore schema: %w", err)
	}
	if err := s.EnsureIndexes(context.Background()); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("store.NewSQLiteStore indexes: %w", err)
	}
	atomic.AddInt64(&openStoreCount, 1)
	return s, nil
}

// Close releases the underlying database connection pool.
func (s *SQLiteStore) Close() error {
	atomic.AddInt64(&openStoreCount, -1)
	return s.db.Close()
}
