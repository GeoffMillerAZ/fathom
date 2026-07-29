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
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
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

	// docs/spec/store-format-v0.md §2: the format_version + application_id
	// markers a freshly-created store must carry.
	var userVersion int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&userVersion); err != nil {
		t.Fatalf("PRAGMA user_version: %v", err)
	}
	if userVersion != storeFormatVersion {
		t.Errorf("PRAGMA user_version = %d, want %d (storeFormatVersion)", userVersion, storeFormatVersion)
	}
	var appID int
	if err := db.QueryRow(`PRAGMA application_id`).Scan(&appID); err != nil {
		t.Fatalf("PRAGMA application_id: %v", err)
	}
	if appID != storeApplicationID {
		t.Errorf("PRAGMA application_id = %d, want %d (storeApplicationID)", appID, storeApplicationID)
	}
}

// TestStoreFormatV0_StampsUnversionedStore proves the live write path
// docs/spec/store-format-v0.md §2 depends on: a raw SQLite file that has
// never been touched by fathom (application_id and user_version both at
// SQLite's own default, 0 — storeFormatVersionUnversioned) gets BOTH
// markers stamped the first time store.Open opens it, not merely left as
// "already conforming." This is what makes storeFormatVersion=1 (not 0)
// load-bearing: with the marker mapped to 0, this exact scenario would have
// nothing to assert, since 0 is indistinguishable from "never stamped."
func TestStoreFormatV0_StampsUnversionedStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "format-v0-unversioned.sqlite")

	// Create a bare SQLite file with no fathom schema at all — application_id
	// and user_version both read back as 0 (SQLite's own defaults) until
	// something stamps them.
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("raw sql.Open: %v", err)
	}
	if _, err := raw.Exec(`CREATE TABLE probe (id INTEGER)`); err != nil {
		t.Fatalf("seed a table so the file is materialized on disk: %v", err)
	}
	var preAppID, preVersion int
	if err := raw.QueryRow(`PRAGMA application_id`).Scan(&preAppID); err != nil {
		t.Fatalf("PRAGMA application_id (pre): %v", err)
	}
	if err := raw.QueryRow(`PRAGMA user_version`).Scan(&preVersion); err != nil {
		t.Fatalf("PRAGMA user_version (pre): %v", err)
	}
	if err := raw.Close(); err != nil {
		t.Fatalf("close raw db: %v", err)
	}
	if preAppID != storeFormatVersionUnversioned || preVersion != storeFormatVersionUnversioned {
		t.Fatalf("precondition failed: application_id=%d user_version=%d, want both %d (unversioned)", preAppID, preVersion, storeFormatVersionUnversioned)
	}

	// store.Open runs createSchema (idempotent CREATE TABLE IF NOT EXISTS
	// over the existing file) and, as part of it, stampFormatMarkers.
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	verify, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("raw sql.Open (verify): %v", err)
	}
	defer verify.Close()
	var gotAppID, gotVersion int
	if err := verify.QueryRow(`PRAGMA application_id`).Scan(&gotAppID); err != nil {
		t.Fatalf("PRAGMA application_id (post): %v", err)
	}
	if err := verify.QueryRow(`PRAGMA user_version`).Scan(&gotVersion); err != nil {
		t.Fatalf("PRAGMA user_version (post): %v", err)
	}
	if gotVersion != storeFormatVersion {
		t.Errorf("user_version after Open = %d, want %d (storeFormatVersion) — the live write path never fired", gotVersion, storeFormatVersion)
	}
	if gotAppID != storeApplicationID {
		t.Errorf("application_id after Open = %d, want %d (storeApplicationID)", gotAppID, storeApplicationID)
	}
}

// TestStoreFormatV0_FormatVersionNeverRegresses proves stampFormatMarkers's
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

// docSections names the docs/spec/store-format-v0.md §3 headings whose
// required-schema tables TestStoreFormatV0_DocMatchesGoSource scrapes and
// diffs against requiredSchemaV0. Both this list and requiredSchemaV0 exist
// so editing either the markdown table or the Go map ALONE — without the
// other — fails a test: the doc is the fixture, not just documentation.
var docSections = []string{
	"### L0 — minimum readable store",
	"### Writer — append-only lineage discipline",
}

// tableRowPattern matches one required-schema markdown table ROW (never the
// header row "| Table | Required columns |", which has no backtick, and
// never the separator row "|---|---|"): a line of the shape
// "| `table_name` | `col1`, `col2`, ... |".
var tableRowPattern = regexp.MustCompile("^\\| `([a-z_]+)` \\| (.+) \\|\\s*$")

// backtickIdentPattern extracts every backtick-quoted identifier from a
// table row's second cell (the required-columns list).
var backtickIdentPattern = regexp.MustCompile("`([a-zA-Z0-9_]+)`")

// parseRequiredSchemaFromDoc scrapes docs/spec/store-format-v0.md's §3
// (L0) and Writer required-schema markdown tables into the same shape as
// requiredSchemaV0 (table name -> ordered required column names), scoped to
// the named section headings so §7's evidence-tier table (whose rows ALSO
// start with "| `verified`" et al.) and any other backtick-led table in the
// document can never be mistaken for a required-schema table.
func parseRequiredSchemaFromDoc(t *testing.T, docPath string) map[string][]string {
	t.Helper()
	data, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("read %s: %v", docPath, err)
	}
	doc := string(data)

	got := map[string][]string{}
	for _, heading := range docSections {
		start := strings.Index(doc, heading)
		if start < 0 {
			t.Fatalf("heading %q not found in %s — docSections is out of sync with the doc's actual headings", heading, docPath)
		}
		rest := doc[start+len(heading):]
		// Bound the section at the next heading of any level ("\n#"), or EOF.
		end := len(rest)
		if next := strings.Index(rest, "\n#"); next >= 0 {
			end = next
		}
		section := rest[:end]

		for _, line := range strings.Split(section, "\n") {
			m := tableRowPattern.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			table := m[1]
			cols := backtickIdentPattern.FindAllStringSubmatch(m[2], -1)
			colNames := make([]string, 0, len(cols))
			for _, c := range cols {
				colNames = append(colNames, c[1])
			}
			if len(colNames) == 0 {
				t.Fatalf("table row for %q in section %q parsed zero columns from %q", table, heading, m[2])
			}
			if _, dup := got[table]; dup {
				t.Fatalf("table %q scraped twice from %s — duplicate row or heading scoping bug", table, docPath)
			}
			got[table] = colNames
		}
	}
	return got
}

// TestStoreFormatV0_DocMatchesGoSource is Finding 2's fix: the L0 and
// Writer required-schema tables in docs/spec/store-format-v0.md §3 are
// scraped and diffed against requiredSchemaV0 (the same map
// TestStoreFormatV0_RequiredSchema checks against the live DDL). Editing
// the markdown table without updating requiredSchemaV0 — or vice versa —
// fails here, closing the manual-sync gap §3's own "a drift here is a
// failing test, not a stale doc" claim depended on.
func TestStoreFormatV0_DocMatchesGoSource(t *testing.T) {
	docPath := filepath.Join("..", "..", "docs", "spec", "store-format-v0.md")
	docSchema := parseRequiredSchemaFromDoc(t, docPath)

	docTables := make([]string, 0, len(docSchema))
	for table := range docSchema {
		docTables = append(docTables, table)
	}
	sort.Strings(docTables)
	goTables := make([]string, 0, len(requiredSchemaV0))
	for table := range requiredSchemaV0 {
		goTables = append(goTables, table)
	}
	sort.Strings(goTables)

	if !reflect.DeepEqual(docTables, goTables) {
		t.Fatalf("table set drifted between doc and Go source:\n  doc:    %v\n  go:     %v", docTables, goTables)
	}
	for _, table := range goTables {
		if !reflect.DeepEqual(docSchema[table], requiredSchemaV0[table]) {
			t.Errorf("table %q columns drifted between doc and Go source:\n  doc: %v\n  go:  %v", table, docSchema[table], requiredSchemaV0[table])
		}
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
