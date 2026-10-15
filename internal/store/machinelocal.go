package store

// machinelocal.go — IXS-0 (docs/plans/index-snapshots.md): the columns that
// record a fact about the machine that wrote a store rather than about its
// corpus, and the clear an export runs on its copy so the copy carries none
// of them. docs/spec/store-format-v0.md section 3 lists the same four
// fields; TestIndexSnapshotCarriesNoMachineLocalField compares the two
// lists. Clearing changes no column, table or user_version: a path becomes
// '' and a number 0, values a reader already handles (an empty
// service_meta.path reads as "no corpus root recorded", which `fathom remap`
// fixes; a zero modification time and inode only make the next ingest
// re-read the file instead of trusting its stat).

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
)

// machineLocalFields is the one code-side list of machine-local fields: the
// table, the column and the portable value an export writes over it.
var machineLocalFields = []struct {
	table, column string
	cleared       any
}{
	{"file_stats", "ino", 0},
	{"file_stats", "mtime_ns", 0},
	{"ingest_state", "path", ""},
	{"service_meta", "path", ""},
}

// MachineLocalFields returns every machine-local field as "table.column",
// sorted.
func MachineLocalFields() []string {
	out := make([]string, 0, len(machineLocalFields))
	for _, f := range machineLocalFields {
		out = append(out, f.table+"."+f.column)
	}
	sort.Strings(out)
	return out
}

// ClearMachineLocal clears every machine-local field in the store file at
// path, in one transaction: the export's own copy (`fathom module pack`
// runs it between its VACUUM INTO and its conformance mount), never a live
// store. The file must exist (it is opened read-write, never created). A
// table or column the store does not have is skipped: a hand-built
// conforming store need not carry the non-required tables.
func ClearMachineLocal(ctx context.Context, path string) error {
	if path == "" {
		return fmt.Errorf("store: clear machine-local fields: empty path")
	}
	db, err := sql.Open("sqlite", "file:"+path+"?mode=rw&_pragma=busy_timeout(5000)")
	if err != nil {
		return fmt.Errorf("store: clear machine-local fields of %s: %w", path, err)
	}
	defer db.Close()
	if err := clearMachineLocal(ctx, db); err != nil {
		return fmt.Errorf("store: clear machine-local fields of %s: %w", path, err)
	}
	return db.Close()
}

// clearMachineLocal writes each machineLocalFields value over its column in
// db, skipping a table or column db lacks.
func clearMachineLocal(ctx context.Context, db *sql.DB) error {
	names, err := existingTables(db)
	if err != nil {
		return err
	}
	tables := map[string]bool{}
	for _, n := range names {
		tables[n] = true
	}
	var stmts []string
	var values []any
	for _, f := range machineLocalFields {
		if !tables[f.table] {
			continue
		}
		cols, err := existingColumns(db, f.table)
		if err != nil {
			return err
		}
		if !cols[f.column] {
			continue
		}
		// table and column are this file's literals, never input.
		stmts = append(stmts, "UPDATE "+f.table+" SET "+f.column+" = ?")
		values = append(values, f.cleared)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for i, stmt := range stmts {
		if _, err := tx.ExecContext(ctx, stmt, values[i]); err != nil {
			return fmt.Errorf("%s: %w", stmt, err)
		}
	}
	return tx.Commit()
}
