package store

// format_v0_test.go — TestStoreFormatV0_RequiredSchema is the falsifiability
// instrument docs/spec/store-format-v0.md §3 cites for its conformance
// tables: it opens a scratch SQLite store on an ephemeral, process-local
// path (t.TempDir() — this repo's existing convention for throwaway stores,
// see pkg/client/portability_test.go; NEVER workspace/db/fleet.sqlite or any
// other live store) and asserts, by direct introspection of sqlite_master /
// PRAGMA table_info against the raw file, that every table and column the
// spec's L0/Writer conformance levels claim as REQUIRED actually exists. A
// drift between this test and the DDL in sqlite.go's createSchema is either
// a spec bug (fix the doc) or a store bug (fix the DDL) — never both
// silently agreeing to drift.
//
// This is a SUBSET check, deliberately: required columns must be present,
// but a store may carry additional tables/columns (docs/spec/README.md's
// compatibility rule that a conforming reader MUST tolerate unknown
// tables/columns). Exact-set equality would make this test itself violate
// the spec it enforces.

import (
	"database/sql"
	"path/filepath"
	"sort"
	"testing"
)

// requiredSchemaV0 is docs/spec/store-format-v0.md §3's L0 + Writer
// required-schema table, transcribed verbatim (table name -> required
// column names). Keep this literally in sync with the markdown table; a
// change to one without the other is the exact drift this test exists to
// catch.
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

func TestStoreFormatV0_RequiredSchema(t *testing.T) {
	// Ephemeral scratch path (t.TempDir(): auto-cleaned, process-local,
	// never workspace/db/fleet.sqlite or any live store).
	path := filepath.Join(t.TempDir(), "format-v0-conformance.sqlite")

	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Re-open the same file with a fresh raw *sql.DB for introspection —
	// the Store interface deliberately does not expose its underlying
	// *sql.DB, so a conformance test that wants to inspect sqlite_master /
	// PRAGMA table_info directly opens its own connection to the same file
	// (the "sqlite" driver is registered by sqlite.go's blank import,
	// compiled into this same test binary).
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("raw sql.Open: %v", err)
	}
	defer db.Close()

	tableNames, err := existingTables(db)
	if err != nil {
		t.Fatalf("list tables: %v", err)
	}
	tableSet := map[string]bool{}
	for _, n := range tableNames {
		tableSet[n] = true
	}

	tables := make([]string, 0, len(requiredSchemaV0))
	for table := range requiredSchemaV0 {
		tables = append(tables, table)
	}
	sort.Strings(tables)

	for _, table := range tables {
		wantCols := requiredSchemaV0[table]
		if !tableSet[table] {
			t.Errorf("required table %q missing from a freshly-opened store", table)
			continue
		}
		gotCols, err := existingColumns(db, table)
		if err != nil {
			t.Fatalf("PRAGMA table_info(%s): %v", table, err)
		}
		for _, col := range wantCols {
			if !gotCols[col] {
				t.Errorf("table %q missing required column %q (has: %v)", table, col, sortedKeys(gotCols))
			}
		}
	}

	// docs/spec/store-format-v0.md §2: the format_version marker.
	var userVersion int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&userVersion); err != nil {
		t.Fatalf("PRAGMA user_version: %v", err)
	}
	if userVersion != storeFormatVersion {
		t.Errorf("PRAGMA user_version = %d, want %d (storeFormatVersion)", userVersion, storeFormatVersion)
	}
}

// TestStoreFormatV0_FormatVersionNeverRegresses proves stampFormatVersion's
// additive guarantee (docs/spec/store-format-v0.md §2: a writer MUST NOT
// decrease PRAGMA user_version on open): re-opening an already-stamped store
// leaves the marker unchanged, never reset.
func TestStoreFormatV0_FormatVersionNeverRegresses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "format-v0-reopen.sqlite")

	st1, err := Open(path)
	if err != nil {
		t.Fatalf("Open (first): %v", err)
	}
	if err := st1.Close(); err != nil {
		t.Fatalf("Close (first): %v", err)
	}

	// Simulate a store already stamped at a future format version — a
	// v0 binary reopening a store a hypothetical v1+ writer already
	// touched must never stamp it back down.
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("raw sql.Open: %v", err)
	}
	if _, err := db.Exec(`PRAGMA user_version = 7`); err != nil {
		t.Fatalf("seed user_version=7: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close raw db: %v", err)
	}

	st2, err := Open(path)
	if err != nil {
		t.Fatalf("Open (second): %v", err)
	}
	if err := st2.Close(); err != nil {
		t.Fatalf("Close (second): %v", err)
	}

	db2, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("raw sql.Open (verify): %v", err)
	}
	defer db2.Close()
	var got int
	if err := db2.QueryRow(`PRAGMA user_version`).Scan(&got); err != nil {
		t.Fatalf("PRAGMA user_version: %v", err)
	}
	if got != 7 {
		t.Errorf("PRAGMA user_version = %d after reopen, want 7 (must not regress)", got)
	}
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
// table_info. table must be a trusted literal (this file's own
// requiredSchemaV0 keys) — PRAGMA takes no bind parameters.
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
