package store

// sqlite_schema.go — SQLiteStore's schema: the fixed createSchema DDL (the
// additive-table convention's hot edit site), the serve-path secondary
// indexes, the format-marker stamp and the PRAGMA table_info helper. Split
// out of sqlite.go (wave g1) so a schema edit no longer shares a file with
// the constructor and every read path.

import (
	"context"
	"database/sql"
	"fmt"
)

// serveIndexes are the secondary indexes SU-1 (docs/plans/substrate.md §A,
// ADR-041 decision 3) adds on top of the fixed schema above: one per column
// a serve path actually filters on — the obligations claim query
// (semantic_claims by service_id/layer/status), the find_callers/
// find_callees typed-edge queries (typed_edges by service_id/to_symbol and
// service_id/from_symbol), claim_links' own service/claim/rel lookup, the
// per-source observations lookup, and external_findings' cursor lookup.
// Every statement is CREATE INDEX IF NOT EXISTS — safe to run on every
// Open, on a fresh store or a years-old one, with no ALTER-TABLE-style
// migration bookkeeping required (unlike the tableColumns migrations
// above, an index either already exists or is created; there is no
// intermediate schema shape to detect).
var serveIndexes = []string{
	`CREATE INDEX IF NOT EXISTS idx_semantic_claims_serve
    ON semantic_claims (service_id, layer, status)`,
	`CREATE INDEX IF NOT EXISTS idx_claim_links_service_claim_rel
    ON claim_links (service_id, claim_id, rel)`,
	`CREATE INDEX IF NOT EXISTS idx_typed_edges_to_symbol
    ON typed_edges (service_id, to_symbol)`,
	`CREATE INDEX IF NOT EXISTS idx_typed_edges_from_symbol
    ON typed_edges (service_id, from_symbol)`,
	`CREATE INDEX IF NOT EXISTS idx_observations_service_source
    ON observations (service_id, source_ref)`,
	// RT-78 (§6): find_symbol / find_callers point lookups by name.
	`CREATE INDEX IF NOT EXISTS idx_observations_service_symbol
    ON observations (service_id, symbol)`,
	`CREATE INDEX IF NOT EXISTS idx_external_findings_service_claim
    ON external_findings (service_id, claim_id)`,
}

// EnsureIndexes creates (idempotently) the secondary indexes serveIndexes
// lists. Called once at construction (NewSQLiteStore) and safe to call
// again any time — every statement is CREATE INDEX IF NOT EXISTS, so a
// repeat call against an already-indexed store is a fast no-op check per
// index, never an error and never a duplicate index.
func (s *SQLiteStore) EnsureIndexes(ctx context.Context) error {
	for _, ddl := range serveIndexes {
		if _, err := s.db.ExecContext(ctx, ddl); err != nil {
			return fmt.Errorf("store.EnsureIndexes: %w", err)
		}
	}
	return nil
}

// createSchema creates the fixed, final-column schema. Unlike the old
// DuckStore backend (deleted in SS-8) there is no ALTER-TABLE migration
// cruft here: every SQLite store was created fresh from this slice onward —
// the now-removed `migrate-store` command ported data from an existing
// DuckDB store rather than upgrading an old SQLite one in place.
func (s *SQLiteStore) createSchema() error {
	const ddl = `
CREATE TABLE IF NOT EXISTS observations (
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
    end_line      INTEGER NOT NULL DEFAULT 0,
    ingested_at   BIGINT  NOT NULL,
    tool_kind     TEXT    NOT NULL DEFAULT '',
    generated     INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS fingerprints (
    service_id   TEXT NOT NULL,
    source_ref   TEXT NOT NULL,
    content_hash TEXT NOT NULL,
    PRIMARY KEY (service_id, source_ref)
);

CREATE TABLE IF NOT EXISTS knowledge_cards (
    service_id TEXT NOT NULL,
    symbol     TEXT NOT NULL,
    file       TEXT NOT NULL DEFAULT '',
    summary    TEXT NOT NULL DEFAULT '',
    tags       TEXT NOT NULL DEFAULT '[]',
    PRIMARY KEY (service_id, symbol)
);

CREATE TABLE IF NOT EXISTS doc_summaries (
    service_id TEXT NOT NULL,
    source_ref TEXT NOT NULL,
    title      TEXT NOT NULL DEFAULT '',
    summary    TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (service_id, source_ref)
);

CREATE TABLE IF NOT EXISTS service_meta (
    service_id      TEXT   NOT NULL PRIMARY KEY,
    path            TEXT   NOT NULL DEFAULT '',
    indexed_commit  TEXT   NOT NULL DEFAULT '',
    indexed_at      BIGINT NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS commits (
    service_id TEXT    NOT NULL,
    hash       TEXT    NOT NULL,
    author     TEXT    NOT NULL DEFAULT '',
    date       BIGINT  NOT NULL DEFAULT 0,
    message    TEXT    NOT NULL DEFAULT '',
    files      TEXT    NOT NULL DEFAULT '[]',
    spec_ids   TEXT    NOT NULL DEFAULT '[]',
    PRIMARY KEY (service_id, hash)
);

CREATE TABLE IF NOT EXISTS semantic_claims (
    service_id             TEXT   NOT NULL,
    claim_id               TEXT   NOT NULL,
    layer                  TEXT   NOT NULL DEFAULT '',
    layer_version          TEXT   NOT NULL DEFAULT '',
    kind                   TEXT   NOT NULL DEFAULT '',
    subject                TEXT   NOT NULL DEFAULT '',
    rule                   TEXT   NOT NULL DEFAULT '',
    scope                  TEXT   NOT NULL DEFAULT '[]',
    tier                   TEXT   NOT NULL DEFAULT '',
    tier_profile           TEXT   NOT NULL DEFAULT '',
    status                 TEXT   NOT NULL DEFAULT '',
    model                  TEXT   NOT NULL DEFAULT '',
    prompt_version         TEXT   NOT NULL DEFAULT '',
    repro_key              TEXT   NOT NULL DEFAULT '',
    input_digest           TEXT   NOT NULL DEFAULT '',
    compiled_at_watermark  TEXT   NOT NULL DEFAULT '',
    compiled_at            BIGINT NOT NULL DEFAULT 0,
    class                  TEXT   NOT NULL DEFAULT '',
    valid_from             TEXT   NOT NULL DEFAULT '',
    valid_to               TEXT   NOT NULL DEFAULT '',
    asserted_by            TEXT   NOT NULL DEFAULT '',
    meta                   TEXT   NOT NULL DEFAULT '',
    lifecycle              TEXT   NOT NULL DEFAULT '',
    PRIMARY KEY (service_id, claim_id)
);

CREATE TABLE IF NOT EXISTS claim_links (
    service_id  TEXT NOT NULL DEFAULT '',
    claim_id    TEXT NOT NULL DEFAULT '',
    rel         TEXT NOT NULL DEFAULT '',
    target_ref  TEXT NOT NULL DEFAULT '',
    target_hash TEXT NOT NULL DEFAULT '',
    anchor_text TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS compile_runs (
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
    budget_stopped     BOOLEAN NOT NULL DEFAULT FALSE,
    escape_note        TEXT    NOT NULL DEFAULT '',
    rejection_reasons_json TEXT NOT NULL DEFAULT '[]',
    resumed_from       TEXT    NOT NULL DEFAULT '',
    uncompiled_scopes_json TEXT NOT NULL DEFAULT '[]',
    sections_prefiltered INTEGER NOT NULL DEFAULT 0,
    uncompiled_reasons_json TEXT NOT NULL DEFAULT '{}'
);

CREATE TABLE IF NOT EXISTS claim_events (
    claim_id          TEXT   NOT NULL DEFAULT '',
    verdict           TEXT   NOT NULL DEFAULT '',
    author            TEXT   NOT NULL DEFAULT '',
    note              TEXT   NOT NULL DEFAULT '',
    event_at          BIGINT NOT NULL DEFAULT 0,
    resolution_kind   TEXT   NOT NULL DEFAULT '',
    resolution_commit TEXT   NOT NULL DEFAULT '',
    asserted_by       TEXT   NOT NULL DEFAULT ''
);

-- Typed call edges (ADR-021 §3, internal/fusion.EnrichTypedEdges). Rows are
-- replaced at PACKAGE granularity (pkg_path), never per-file — the typed
-- pass's cache unit is a whole go/packages load, not a single source file.
CREATE TABLE IF NOT EXISTS typed_edges (
    service_id   TEXT NOT NULL,
    pkg_path     TEXT NOT NULL DEFAULT '',
    from_service TEXT NOT NULL DEFAULT '',
    from_file    TEXT NOT NULL DEFAULT '',
    from_symbol  TEXT NOT NULL DEFAULT '',
    to_symbol    TEXT NOT NULL DEFAULT '',
    kind         TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_typed_edges_service_id
    ON typed_edges (service_id);

-- Per-package content-hash cache: skip reloading a package via go/packages
-- when its file-set hash is unchanged since the last successful load.
CREATE TABLE IF NOT EXISTS typed_edge_fingerprints (
    service_id   TEXT NOT NULL,
    pkg_path     TEXT NOT NULL,
    content_hash TEXT NOT NULL,
    PRIMARY KEY (service_id, pkg_path)
);

-- Every observations query filters on service_id (usually with concern_id
-- or a symbol filter on top); without an index each one is a full table
-- scan over the store's largest table. Cheap to create at birth, a
-- migration later — so it is part of the fresh DDL (F7).
CREATE INDEX IF NOT EXISTS idx_observations_service_id
    ON observations (service_id);
`
	if _, err := s.db.Exec(ddl); err != nil {
		return err
	}
	if _, err := s.db.Exec(externalFindingsDDL); err != nil {
		return err
	}

	// review_runs (FL-15, docs/plans/findings-loop.md) is a separate const,
	// executed right after the main schema — the same additive-table
	// convention a later wave's own table follows; the main ddl const above
	// is never edited for a new table.
	if err := s.createReviewRunsSchema(); err != nil {
		return err
	}

	// Test evidence + declared certifications (vision 33 / ADR-043 stage
	// 0, TC-2/TC-5) — same additive-table convention, own file
	// (sqlite_testevidence.go).
	if err := s.createTestEvidenceSchema(); err != nil {
		return err
	}

	// typed_edge_status (DG-10) follows the same additive-table convention
	// as review_runs above.
	if err := s.createTypedEdgeStatusSchema(); err != nil {
		return err
	}

	// ingest_state (RT-109) — same additive-table convention
	// (sqlite_ingeststate.go).
	if err := s.createIngestStateSchema(); err != nil {
		return err
	}

	// file_stats, service_summary, observation_calls (RT-76, RT-77) — same
	// additive-table convention (derived.go).
	if err := s.createDerivedSchema(); err != nil {
		return err
	}

	// file_imports (RT-108) — same additive-table convention
	// (fileimports.go).
	if err := s.createFileImportsSchema(); err != nil {
		return err
	}

	// file_churn (TQ-1) — same additive-table convention (filechurn.go).
	if err := s.createFileChurnSchema(); err != nil {
		return err
	}

	// symbol_centrality (TQ-2) — same additive-table convention (centrality.go).
	if err := s.createSymbolCentralitySchema(); err != nil {
		return err
	}

	// arch_violation_trend (CX-20) — same additive-table convention
	// (archviolations.go).
	if err := s.createArchViolationTrendSchema(); err != nil {
		return err
	}

	// docs_fts, doc_units, doc_links (KR-1, docs/design/knowledge-retrieval.md
	// §4) — same additive-table convention (docindex.go). SQLite-only
	// (docs_fts is an FTS5 virtual table; no MemoryStore equivalent, same as
	// the physical-purge/VACUUM path — see docindex.go's doc comment).
	if err := s.createDocIndexSchema(); err != nil {
		return err
	}

	// Conditional migrations for SQLite stores created before a column
	// existed. The "no ALTER cruft" stance above holds for the fleet store
	// (freshly migrated), but scratch/workspace SQLite stores created by
	// earlier slices already exist on disk — INCLUDING the live fleet store,
	// which predates escape_note — and SQLite has no ADD COLUMN IF NOT
	// EXISTS, so missing columns are detected via PRAGMA table_info and
	// added individually, per table (ADR-021 §2 / critic F1; A2-1/D3-1
	// bounce finding 1: adding escape_note only to the CREATE TABLE shape
	// was a no-op on every pre-existing store — CompileRuns and
	// RecordCompileRun both errored with "no such column").
	migrations := []struct{ table, column, ddl string }{
		{"observations", "tool_kind", `ALTER TABLE observations ADD COLUMN tool_kind TEXT NOT NULL DEFAULT ''`},
		{"observations", "generated", `ALTER TABLE observations ADD COLUMN generated INTEGER NOT NULL DEFAULT 0`},
		{"observations", "end_line", `ALTER TABLE observations ADD COLUMN end_line INTEGER NOT NULL DEFAULT 0`},
		{"compile_runs", "escape_note", `ALTER TABLE compile_runs ADD COLUMN escape_note TEXT NOT NULL DEFAULT ''`},
		// Q-3 (docs/plans/expert-suite-q.md, O19's fix): a pre-existing
		// store (including the live fleet store) predates rejection-reason
		// capture — RecordCompileRun/CompileRuns would otherwise error with
		// "no such column" exactly like escape_note's own migration bounce
		// finding above.
		{"compile_runs", "rejection_reasons_json", `ALTER TABLE compile_runs ADD COLUMN rejection_reasons_json TEXT NOT NULL DEFAULT '[]'`},
		// O54 (a second pilot reading found the obligations serve-time
		// coverage gap): a pre-existing store (including the live fleet
		// store) predates resumed_from/uncompiled_scopes_json —
		// RecordCompileRun/CompileRuns must not fail with "no such column"
		// on it, exactly like escape_note's own migration bounce finding
		// above; a legacy row simply reads back with ResumedFrom=="" and
		// UncompiledScopes==nil.
		{"compile_runs", "resumed_from", `ALTER TABLE compile_runs ADD COLUMN resumed_from TEXT NOT NULL DEFAULT ''`},
		{"compile_runs", "uncompiled_scopes_json", `ALTER TABLE compile_runs ADD COLUMN uncompiled_scopes_json TEXT NOT NULL DEFAULT '[]'`},
		// DP-6 (ADR-042, vision 32 §2 item 1): a pre-existing store (including
		// the live fleet store) predates the decision-port compile pre-filter —
		// RecordCompileRun/CompileRuns must not fail with "no such column" on
		// it, exactly like every other late compile_runs column above; a
		// legacy row simply reads back with SectionsPrefiltered==0 (honest:
		// no pre-filter ever ran on it).
		{"compile_runs", "sections_prefiltered", `ALTER TABLE compile_runs ADD COLUMN sections_prefiltered INTEGER NOT NULL DEFAULT 0`},
		// EW-35: why each uncompiled section is uncompiled (section subject to
		// failed/prefiltered/refused/budget/cancelled) — an ADDED column, never a
		// table; a legacy row reads back with UncompiledReasons==nil.
		{"compile_runs", "uncompiled_reasons_json", `ALTER TABLE compile_runs ADD COLUMN uncompiled_reasons_json TEXT NOT NULL DEFAULT '{}'`},
		// T5-1/O13: pre-existing claim_links rows (including the live fleet
		// store) predate AnchorText — a re-anchor read/write over such a
		// store must not fail with "no such column", it must simply see ''
		// (Sweep's existing "empty AnchorText → cannot heal, only stale"
		// leniency handles the legacy row honestly from there).
		{"claim_links", "anchor_text", `ALTER TABLE claim_links ADD COLUMN anchor_text TEXT NOT NULL DEFAULT ''`},
		// Findings-as-evidence (docs/design/findings-as-evidence.md §3.2,
		// §6 item 2): a pre-existing store (including the live fleet
		// store) predates the typed finding-class vocabulary —
		// PutClaims/ClaimsForService must not fail with "no such column"
		// on it; a legacy claim simply reads back with Class=="" (additive
		// migration, same "no such column" bounce class escape_note/
		// rejection_reasons_json/anchor_text already hit and fixed).
		{"semantic_claims", "class", `ALTER TABLE semantic_claims ADD COLUMN class TEXT NOT NULL DEFAULT ''`},
		// Findings-as-evidence resolution outcomes (§3.4, §6 item 4): a
		// pre-existing store predates ResolutionKind/ResolutionCommit —
		// AppendClaimEvent/ClaimEvents(Since) must not fail on it; a legacy
		// event simply reads back with both fields "".
		{"claim_events", "resolution_kind", `ALTER TABLE claim_events ADD COLUMN resolution_kind TEXT NOT NULL DEFAULT ''`},
		{"claim_events", "resolution_commit", `ALTER TABLE claim_events ADD COLUMN resolution_commit TEXT NOT NULL DEFAULT ''`},
		// SU-19 (ADR-041 decision 7, docs/vision/31-substrate.md §3 D6): a
		// pre-existing store (including the live fleet store) predates
		// bi-temporal validity — PutClaims/ClaimsForService(Serve) must not
		// fail with "no such column" on it; a legacy claim simply reads back
		// with ValidFrom/ValidTo both zero (core.SemanticClaim.ValidAt's own
		// doc comment: a zero bound is never constraining), exactly like
		// every earlier additive claim column's own migration bounce.
		{"semantic_claims", "valid_from", `ALTER TABLE semantic_claims ADD COLUMN valid_from TEXT NOT NULL DEFAULT ''`},
		{"semantic_claims", "valid_to", `ALTER TABLE semantic_claims ADD COLUMN valid_to TEXT NOT NULL DEFAULT ''`},
		// SU-22 (ADR-041 decision 7): a pre-existing store predates
		// AssertedBy — a legacy claim/event simply reads back with "".
		{"semantic_claims", "asserted_by", `ALTER TABLE semantic_claims ADD COLUMN asserted_by TEXT NOT NULL DEFAULT ''`},
		{"claim_events", "asserted_by", `ALTER TABLE claim_events ADD COLUMN asserted_by TEXT NOT NULL DEFAULT ''`},
		// vision 33 / ADR-043 stage 1 (TC-6): a pre-existing store predates
		// the opaque per-claim Meta provenance blob — PutClaims/
		// ClaimsForService(Serve) must not fail with "no such column" on
		// it; a legacy claim simply reads back with Meta==nil, same
		// additive-migration bounce every earlier claim column hit.
		{"semantic_claims", "meta", `ALTER TABLE semantic_claims ADD COLUMN meta TEXT NOT NULL DEFAULT ''`},
		// CV-2 (docs/plans/refactor-conservation.md, design doc §1.1,
		// O114, vision 34 / ADR-044 stage 1): a pre-existing store
		// (including the live fleet store) predates commit/dirty/race —
		// PutTestRuns/TestCaseRunsForSymbol must not fail with "no such
		// column" on it, same additive-migration bounce every earlier
		// test_runs-adjacent column hit; a legacy run simply reads back
		// unbound (commit_sha=='', dirty=false, race=false), which is
		// the honest answer for a run that never stated them.
		{"test_runs", "commit_sha", `ALTER TABLE test_runs ADD COLUMN commit_sha TEXT NOT NULL DEFAULT ''`},
		{"test_runs", "dirty", `ALTER TABLE test_runs ADD COLUMN dirty INTEGER NOT NULL DEFAULT 0`},
		{"test_runs", "race", `ALTER TABLE test_runs ADD COLUMN race INTEGER NOT NULL DEFAULT 0`},
		// GL-13 (docs/plans/governance-learning.md, design doc §6): a
		// pre-existing store (including the live fleet store) predates the
		// promotion/retirement lifecycle axis — PutClaims/ClaimsForService/
		// ClaimsForServe must not fail with "no such column" on it; a
		// legacy claim simply reads back with Lifecycle=="", which
		// core.ClaimLifecycle.EffectiveLifecycle treats as already active
		// (design doc owner decision: "default active"), same
		// additive-migration bounce every earlier claim column hit.
		{"semantic_claims", "lifecycle", `ALTER TABLE semantic_claims ADD COLUMN lifecycle TEXT NOT NULL DEFAULT ''`},
	}
	existingByTable := map[string]map[string]bool{}
	for _, m := range migrations {
		existing, ok := existingByTable[m.table]
		if !ok {
			var err error
			existing, err = s.tableColumns(m.table)
			if err != nil {
				return err
			}
			existingByTable[m.table] = existing
		}
		if existing[m.column] {
			continue
		}
		if _, err := s.db.Exec(m.ddl); err != nil {
			return fmt.Errorf("migrate %s.%s: %w", m.table, m.column, err)
		}
	}
	return s.stampFormatMarkers()
}

// stampFormatMarkers writes the two SQLite pragmas docs/spec/
// store-format-v0.md §2 documents as a conforming store's self-identifying
// header: PRAGMA application_id (WHOSE file this is — storeApplicationID,
// fathom's stable magic number) and PRAGMA user_version (WHICH spec version
// wrote it — storeFormatVersion). Both writes are additive-safe:
//
//   - application_id is stamped only when it currently reads the SQLite
//     default (0, meaning "never set") — never overwriting a non-zero,
//     foreign application_id some other tool may already have stamped on
//     this file (BackendForPath dispatches on file extension alone, so a
//     ".sqlite" file that is not actually a fathom store is a real,
//     if pathological, possibility; refusing to relabel it is the safe
//     default).
//   - user_version is bumped only when it is strictly lower than
//     storeFormatVersion, never decreased — a store already stamped at or
//     above storeFormatVersion (e.g. by a future v1+ writer) is left
//     untouched, so an older binary opening a newer store cannot regress
//     the marker a newer reader depends on. A fresh or legacy store reads
//     storeFormatVersionUnversioned (0) here and is correctly bumped to 1.
//
// PRAGMA accepts no bind parameters; the interpolated values are the
// package-internal constants above, never caller input.
func (s *SQLiteStore) stampFormatMarkers() error {
	var appID int
	if err := s.db.QueryRow(`PRAGMA application_id`).Scan(&appID); err != nil {
		return fmt.Errorf("read application_id: %w", err)
	}
	if appID == 0 {
		if _, err := s.db.Exec(fmt.Sprintf(`PRAGMA application_id = %d`, storeApplicationID)); err != nil {
			return fmt.Errorf("set application_id: %w", err)
		}
	}

	var version int
	if err := s.db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return fmt.Errorf("read user_version: %w", err)
	}
	if version < storeFormatVersion {
		if _, err := s.db.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, storeFormatVersion)); err != nil {
			return fmt.Errorf("set user_version: %w", err)
		}
	}
	return nil
}

// tableColumns returns the set of column names currently present on the
// named table, via PRAGMA table_info. table must be a trusted literal from
// this package (PRAGMA takes no bind parameters).
func (s *SQLiteStore) tableColumns(table string) (map[string]bool, error) {
	rows, err := s.db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return nil, fmt.Errorf("pragma table_info(%s): %w", table, err)
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
			return nil, fmt.Errorf("pragma table_info scan: %w", err)
		}
		cols[name] = true
	}
	return cols, rows.Err()
}
