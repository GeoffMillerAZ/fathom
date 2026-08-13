package store

// conformance.go — mount-time conformance checking for a module store
// (docs/plans/gke-module-host.md; docs/spec/store-format-v0.md §2/§3;
// docs/spec/expert-module-v0.md). MountReadOnly is the module host's one
// entry point: open a store file genuinely read-only, verify it against the
// spec's format markers and required schema, and refuse loudly — a named
// error, never a partial mount — on any mismatch.
//
// requiredSchemaV0 (below) is the SAME map format_v0_test.go's
// TestStoreFormatV0_RequiredSchema/TestStoreFormatV0_DocMatchesGoSource
// assert against the live DDL and the markdown spec respectively — one
// transcription of docs/spec/store-format-v0.md §3, three consumers (the
// live-schema test, the doc-drift test, and MountReadOnly's own schema
// check), so a change to the DDL, the doc, or the conformance checker can
// never drift from the other two silently.

import (
	"database/sql"
	"fmt"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
)

// requiredSchemaV0 is docs/spec/store-format-v0.md §3's L0 + Writer
// required-schema table, transcribed verbatim (table name -> required
// column names).
var requiredSchemaV0 = map[string][]string{
	// L0 — minimum readable store.
	"service_meta": {"service_id", "path", "indexed_commit", "indexed_at"},
	"observations": {
		"service_id", "concern_id", "source_ref", "source_kind",
		"evidence_tier", "symbol", "package", "claim", "content_hash",
		"calls", "complexity", "line", "end_line", "ingested_at",
		"tool_kind", "generated",
	},
	"semantic_claims": {
		"service_id", "claim_id", "layer", "layer_version", "kind",
		"subject", "rule", "scope", "tier", "tier_profile", "status",
		"model", "prompt_version", "repro_key", "input_digest",
		"compiled_at_watermark", "compiled_at",
	},
	"claim_links": {
		"service_id", "claim_id", "rel", "target_ref", "target_hash", "anchor_text",
	},

	// Writer — additionally required for append-only lineage discipline.
	"claim_events": {"claim_id", "verdict", "author", "note", "event_at"},
	"compile_runs": {
		"run_id", "service_id", "layer", "corpus_watermark", "model",
		"prompt_version", "dry_run", "tokens_in", "tokens_out",
		"cost_usd", "duration_ms", "sections_total", "sections_compiled",
		"sections_failed", "claims_emitted", "claims_rejected", "ran_at",
		"refused", "refusal_reason", "budget_stopped", "escape_note",
	},
}

// levelL0Tables / levelWriterTables partition requiredSchemaV0's keys by the
// conformance level that first requires them — the same L0/Writer split
// docs/spec/store-format-v0.md §3 and §10's summary table document.
var (
	levelL0Tables     = []string{"service_meta", "observations", "semantic_claims", "claim_links"}
	levelWriterTables = []string{"claim_events", "compile_runs"}
)

// ConformanceLevel names a docs/spec/store-format-v0.md §3 schema level
// MountReadOnly can check a store against. Server conformance (§3, §10) is
// a BEHAVIORAL requirement over the SAME tables as L0/Writer — "Server
// conformance adds no new table over L0/Writer" — so it has no distinct
// schema footprint here; a caller enforcing the module host's "L0 + Server"
// floor (docs/plans/gke-module-host.md's design section) checks LevelL0 at
// mount time and is itself responsible for the Server-conformant SERVING
// half (citation+tier+status attached to every payload) — schema
// introspection alone cannot prove behavior.
type ConformanceLevel int

const (
	// LevelL0 requires service_meta, observations, semantic_claims,
	// claim_links (store-format-v0.md §3's L0 table) — the module host's
	// default mount requirement, since that's the exact table set
	// internal/semantic.ObligationsFor and the staleness-report shape need.
	LevelL0 ConformanceLevel = iota
	// LevelWriter additionally requires claim_events, compile_runs
	// (store-format-v0.md §3's Writer table): a stricter mount policy for a
	// module a host wants to trust with its full compile ledger present,
	// not just served claims.
	LevelWriter
)

// String names the level for error messages and logs.
func (l ConformanceLevel) String() string {
	switch l {
	case LevelL0:
		return "L0"
	case LevelWriter:
		return "Writer"
	default:
		return fmt.Sprintf("ConformanceLevel(%d)", int(l))
	}
}

// requiredTablesForLevel returns the sorted table names MountReadOnly must
// find present (with their required columns) for level.
func requiredTablesForLevel(level ConformanceLevel) []string {
	tables := append([]string{}, levelL0Tables...)
	if level >= LevelWriter {
		tables = append(tables, levelWriterTables...)
	}
	sort.Strings(tables)
	return tables
}

// Named, stable reason codes for ConformanceError — a caller (or a test)
// switches on these rather than substring-matching Error()'s prose.
const (
	// ReasonUnsupportedExtension: the path's extension is neither .sqlite
	// nor .db (mirrors BackendForPath's own extension dispatch).
	ReasonUnsupportedExtension = "unsupported-extension"
	// ReasonDuckDBUnsupported: a .duckdb path — refused outright since SS-8
	// deleted the DuckDB backend (ADR-020/ADR-025), same as BackendForPath.
	ReasonDuckDBUnsupported = "duckdb-unsupported"
	// ReasonOpenFailed: the file could not be opened/pinged at all (missing,
	// corrupt, locked, or a raw I/O error) — never reached schema checking.
	ReasonOpenFailed = "open-failed"
	// ReasonBadApplicationID: PRAGMA application_id != storeApplicationID —
	// "this is not a fathom store."
	ReasonBadApplicationID = "bad-application-id"
	// ReasonUnversioned: PRAGMA user_version < 1 — application_id matched,
	// but the store was never touched by a marker-aware fathom writer (or
	// predates the marker). Refused at MOUNT even though store-format-v0.md
	// §2's own general-reader guidance would fall back to schema
	// introspection for this case — a module host serving N third-party
	// stores requires the stronger, explicit signal.
	ReasonUnversioned = "unversioned"
	// ReasonMissingTable / ReasonMissingColumn: an L0-required table or
	// column is absent — the store fails baseline conformance outright, at
	// every level (GKE-MH-9).
	ReasonMissingTable  = "missing-table"
	ReasonMissingColumn = "missing-column"
	// ReasonInsufficientLevel: every L0 table/column is present, but a
	// table/column required ONLY at a level above L0 (e.g. Writer's
	// claim_events/compile_runs) is missing — the store IS conforming, just
	// not at the level THIS mount required (GKE-MH-10).
	ReasonInsufficientLevel = "insufficient-level"
)

// ConformanceError names exactly which check failed for a MountReadOnly
// attempt: the path and a stable Reason code, plus a human Detail — never a
// generic wrapped error a caller has to string-match against
// (docs/plans/gke-module-host.md GKE-MH-1/9/10: "refused loudly ... named
// error: path + which check failed").
type ConformanceError struct {
	Path   string
	Reason string
	Detail string
}

func (e *ConformanceError) Error() string {
	return fmt.Sprintf("store: refusing to mount %s: %s (%s)", e.Path, e.Detail, e.Reason)
}

// sqliteReadOnlyDSN builds a DSN that opens path GENUINELY read-only at the
// SQLite/VFS layer (proven by TestMountReadOnly_SucceedsAndIsGenuinelyReadOnly:
// an INSERT through the returned *SQLiteStore fails with SQLITE_READONLY, not
// merely "discouraged by this package's own code paths").
//
// The "file:" prefix is REQUIRED for modernc.org/sqlite's newConn (conn.go)
// to preserve this query string all the way into the filename it passes to
// sqlite3_open_v2: a dsn NOT prefixed "file:" has its query string stripped
// before that call, and "mode" is honored by SQLite's OWN URI parser
// (enabled via the driver's always-on SQLITE_OPEN_URI flag) — not by
// anything this Go package implements. "mode=ro" is MORE restrictive than
// the driver's hardcoded SQLITE_OPEN_READWRITE|CREATE flags, which SQLite's
// URI rules permit (a mode value may only restrict, never expand, the C
// flags) — this is the same idiom other Go SQLite drivers document for a
// true read-only open, verified against modernc.org/sqlite v1.52.0 directly
// (a missing file fails to open — mode=ro never creates one — and a write
// through the resulting connection returns SQLITE_READONLY).
func sqliteReadOnlyDSN(path string) string {
	q := url.Values{}
	q.Set("mode", "ro")
	q.Add("_pragma", "busy_timeout(5000)")
	return "file:" + path + "?" + q.Encode()
}

// MountReadOnly opens path as a genuinely read-only store and
// conformance-checks it against level before returning it usable. A
// non-conforming or unreadable store is refused loudly: MountReadOnly
// either returns a usable *SQLiteStore or a *ConformanceError naming
// exactly which check failed — it never returns a partially-usable store
// (docs/plans/gke-module-host.md GKE-MH-1, doctrine 3 "never silent").
//
// The marker check is STRICTER than store-format-v0.md §2's own
// general-reader fallback (which tolerates user_version=0 via schema
// introspection): a mount requires application_id == storeApplicationID
// AND user_version >= 1 — see ReasonUnversioned's doc comment for why.
func MountReadOnly(path string, level ConformanceLevel) (*SQLiteStore, error) {
	if path == "" {
		return nil, &ConformanceError{Path: path, Reason: ReasonOpenFailed, Detail: "empty path (in-memory stores cannot be mounted read-only)"}
	}
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".duckdb":
		return nil, &ConformanceError{Path: path, Reason: ReasonDuckDBUnsupported, Detail: "DuckDB stores are no longer supported (removed in SS-8)"}
	case ".sqlite", ".db":
		// proceed
	default:
		return nil, &ConformanceError{Path: path, Reason: ReasonUnsupportedExtension, Detail: fmt.Sprintf("unrecognized store extension %q (expected .sqlite or .db)", ext)}
	}

	db, err := sql.Open("sqlite", sqliteReadOnlyDSN(path))
	if err != nil {
		return nil, &ConformanceError{Path: path, Reason: ReasonOpenFailed, Detail: err.Error()}
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, &ConformanceError{Path: path, Reason: ReasonOpenFailed, Detail: err.Error()}
	}

	if cerr := checkMarkers(db, path); cerr != nil {
		_ = db.Close()
		return nil, cerr
	}
	if cerr := checkRequiredSchema(db, path, level); cerr != nil {
		_ = db.Close()
		return nil, cerr
	}

	return &SQLiteStore{db: db, path: path, gate: newScopeGate()}, nil
}

// checkMarkers verifies the two format-identity pragmas (store-format-v0.md
// §2): application_id must be the fathom magic number, and user_version
// must be >= 1 (see ReasonUnversioned).
func checkMarkers(db *sql.DB, path string) error {
	var appID int
	if err := db.QueryRow(`PRAGMA application_id`).Scan(&appID); err != nil {
		return &ConformanceError{Path: path, Reason: ReasonOpenFailed, Detail: "read application_id: " + err.Error()}
	}
	if appID != storeApplicationID {
		return &ConformanceError{Path: path, Reason: ReasonBadApplicationID, Detail: fmt.Sprintf("application_id = %#x, want %#x (not a fathom store)", appID, storeApplicationID)}
	}
	var version int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return &ConformanceError{Path: path, Reason: ReasonOpenFailed, Detail: "read user_version: " + err.Error()}
	}
	if version < 1 {
		return &ConformanceError{Path: path, Reason: ReasonUnversioned, Detail: fmt.Sprintf("user_version = %d, want >= 1 (unstamped by a marker-aware fathom writer)", version)}
	}
	// version > 1 (a store stamped by a hypothetical future v1+ writer) is
	// deliberately NOT an error and carries no Detail note here — this is
	// store-format-v0.md §2's own reader contract: "a reader MUST treat a
	// user_version higher than the highest version it implements as 'read
	// with caution, expect unknown tables/columns,' never as an error." A
	// mount under v0 rules against such a store still runs the FULL v0
	// required-schema check below (unknown ADDITIONAL tables/columns a v1+
	// writer might have added are simply invisible to this reader, per
	// README.md's "additive within v0" compatibility rule) — so mounting
	// succeeds exactly when the v0-required subset is present, silently
	// tolerant of anything newer, by design rather than by omission.
	return nil
}

// checkRequiredSchema verifies every table+column requiredTablesForLevel(level)
// names is present, distinguishing an L0-baseline gap (ReasonMissingTable/
// Column — GKE-MH-9) from a level-escalation gap (ReasonInsufficientLevel —
// GKE-MH-10: the store IS L0-conforming, just not at the stricter level this
// mount required).
func checkRequiredSchema(db *sql.DB, path string, level ConformanceLevel) error {
	tableNames, err := existingTables(db)
	if err != nil {
		return &ConformanceError{Path: path, Reason: ReasonOpenFailed, Detail: "list tables: " + err.Error()}
	}
	tableSet := map[string]bool{}
	for _, n := range tableNames {
		tableSet[n] = true
	}
	for _, table := range requiredTablesForLevel(level) {
		isL0 := isL0Table(table)
		if !tableSet[table] {
			reason := ReasonMissingTable
			if !isL0 {
				reason = ReasonInsufficientLevel
			}
			return &ConformanceError{Path: path, Reason: reason, Detail: fmt.Sprintf("required table %q missing (level %s)", table, level)}
		}
		cols, err := existingColumns(db, table)
		if err != nil {
			return &ConformanceError{Path: path, Reason: ReasonOpenFailed, Detail: fmt.Sprintf("PRAGMA table_info(%s): %v", table, err)}
		}
		for _, col := range requiredSchemaV0[table] {
			if !cols[col] {
				reason := ReasonMissingColumn
				if !isL0 {
					reason = ReasonInsufficientLevel
				}
				return &ConformanceError{Path: path, Reason: reason, Detail: fmt.Sprintf("table %q missing required column %q (level %s)", table, col, level)}
			}
		}
	}
	return nil
}

func isL0Table(table string) bool {
	for _, t := range levelL0Tables {
		if t == table {
			return true
		}
	}
	return false
}

// existingTables returns every table name in sqlite_master for db.
func existingTables(db *sql.DB) ([]string, error) {
	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type = 'table'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

// existingColumns returns the set of column names on table, via PRAGMA
// table_info. table must be a trusted literal (requiredSchemaV0's own
// keys, or this file's level table lists) — PRAGMA takes no bind
// parameters.
func existingColumns(db *sql.DB, table string) (map[string]bool, error) {
	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols := map[string]bool{}
	for rows.Next() {
		var (
			cid     int
			name    string
			ctype   string
			notNull int
			dflt    sql.NullString
			pk      int
		)
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &dflt, &pk); err != nil {
			return nil, err
		}
		cols[name] = true
	}
	return cols, rows.Err()
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
