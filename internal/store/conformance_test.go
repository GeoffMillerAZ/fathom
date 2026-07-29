package store

// conformance_test.go — MountReadOnly's mount-time conformance checking
// (docs/plans/gke-module-host.md GKE-MH-1/9/10): a non-conforming or
// unreadable store is refused loudly with a named ConformanceError, never
// silently degraded or partially mounted; a conforming store is opened
// GENUINELY read-only — a write through the returned handle fails at the
// SQLite layer itself.

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// writeBareStore creates a raw SQLite file at path containing exactly the
// named tables (each with its requiredSchemaV0 columns, all TEXT — column
// TYPE is not part of the conformance check, only column NAME presence is)
// and stamps application_id/user_version to the given values directly
// (bypassing SQLiteStore.stampFormatMarkers' write-only-if-unset guard, so
// a test can simulate a store stamped by something other than this
// package's own writer).
func writeBareStore(t *testing.T, path string, tables []string, appID, userVersion int) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("writeBareStore open: %v", err)
	}
	defer db.Close()
	for _, table := range tables {
		cols := requiredSchemaV0[table]
		if cols == nil {
			t.Fatalf("writeBareStore: unknown table %q", table)
		}
		ddl := "CREATE TABLE " + table + " ("
		for i, c := range cols {
			if i > 0 {
				ddl += ", "
			}
			ddl += c + " TEXT"
		}
		ddl += ")"
		if _, err := db.Exec(ddl); err != nil {
			t.Fatalf("writeBareStore CREATE TABLE %s: %v", table, err)
		}
	}
	if _, err := db.Exec(`PRAGMA application_id = ` + itoa(appID)); err != nil {
		t.Fatalf("writeBareStore set application_id: %v", err)
	}
	if _, err := db.Exec(`PRAGMA user_version = ` + itoa(userVersion)); err != nil {
		t.Fatalf("writeBareStore set user_version: %v", err)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// allL0Tables is levelL0Tables in requiredSchemaV0's own key form (used to
// build a fully-conforming L0 store by hand for the level-escalation test).
var allL0Tables = append([]string{}, levelL0Tables...)

func TestMountReadOnly_SucceedsAndIsGenuinelyReadOnly(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "module.sqlite")

	// Build a normal, fully-conforming store via the writer path, seed one
	// observation so ListServiceIDs has something to report, then close it —
	// MountReadOnly never shares a live writer connection.
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := st.PutObservations(ctx, []core.Observation{{
		ServiceID: "svc", SourceRef: "a.go", SourceKind: core.KindGoFunc,
		EvidenceTier: core.TierVerified, Symbol: "Foo",
	}}); err != nil {
		t.Fatalf("seed PutObservations: %v", err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	mounted, err := MountReadOnly(path, LevelL0)
	if err != nil {
		t.Fatalf("MountReadOnly: %v", err)
	}
	defer mounted.Close()

	ids, err := mounted.ListServiceIDs(ctx)
	if err != nil {
		t.Fatalf("ListServiceIDs via mounted handle: %v", err)
	}
	if len(ids) != 1 || ids[0] != "svc" {
		t.Fatalf("ListServiceIDs = %v, want [svc]", ids)
	}

	// The load-bearing property (docs/vision/12 law 9, GKE-MH-8): a write
	// through the mounted handle fails at the SQLite layer, not merely by
	// this package's own convention.
	err = mounted.PutObservations(ctx, []core.Observation{{
		ServiceID: "svc2", SourceRef: "b.go", SourceKind: core.KindGoFunc,
		EvidenceTier: core.TierVerified, Symbol: "Bar",
	}})
	if err == nil {
		t.Fatal("PutObservations through a MountReadOnly handle succeeded; want a read-only failure")
	}
}

func TestMountReadOnly_RefusesDuckDBExtension(t *testing.T) {
	path := filepath.Join(t.TempDir(), "module.duckdb")
	_, err := MountReadOnly(path, LevelL0)
	assertConformanceReason(t, err, ReasonDuckDBUnsupported)
}

func TestMountReadOnly_RefusesUnsupportedExtension(t *testing.T) {
	path := filepath.Join(t.TempDir(), "module.txt")
	_, err := MountReadOnly(path, LevelL0)
	assertConformanceReason(t, err, ReasonUnsupportedExtension)
}

func TestMountReadOnly_RefusesMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.sqlite")
	_, err := MountReadOnly(path, LevelL0)
	assertConformanceReason(t, err, ReasonOpenFailed)
}

func TestMountReadOnly_RefusesBadApplicationID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "module.sqlite")
	writeBareStore(t, path, allL0Tables, 0xDEADBEEF, 1)
	_, err := MountReadOnly(path, LevelL0)
	assertConformanceReason(t, err, ReasonBadApplicationID)
}

func TestMountReadOnly_RefusesUnversioned(t *testing.T) {
	path := filepath.Join(t.TempDir(), "module.sqlite")
	writeBareStore(t, path, allL0Tables, storeApplicationID, 0)
	_, err := MountReadOnly(path, LevelL0)
	assertConformanceReason(t, err, ReasonUnversioned)
}

func TestMountReadOnly_RefusesMissingTable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "module.sqlite")
	// Omit claim_links entirely — an L0-required table.
	writeBareStore(t, path, []string{"service_meta", "observations", "semantic_claims"}, storeApplicationID, 1)
	_, err := MountReadOnly(path, LevelL0)
	assertConformanceReason(t, err, ReasonMissingTable)
}

func TestMountReadOnly_LevelEscalation_InsufficientLevel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "module.sqlite")
	// A store carrying every L0 table (correctly marked) but NEITHER Writer
	// table (claim_events, compile_runs).
	writeBareStore(t, path, allL0Tables, storeApplicationID, 1)

	// L0 is satisfied: mounts cleanly.
	l0, err := MountReadOnly(path, LevelL0)
	if err != nil {
		t.Fatalf("MountReadOnly at LevelL0 on an L0-conforming store: %v", err)
	}
	l0.Close()

	// Writer is NOT satisfied: refused with the level-escalation reason,
	// distinct from a baseline missing-table refusal (GKE-MH-9 vs GKE-MH-10).
	_, err = MountReadOnly(path, LevelWriter)
	assertConformanceReason(t, err, ReasonInsufficientLevel)
}

func assertConformanceReason(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("MountReadOnly succeeded; want a *ConformanceError with reason %q", want)
	}
	cerr, ok := err.(*ConformanceError)
	if !ok {
		t.Fatalf("MountReadOnly error is %T (%v), want *ConformanceError", err, err)
	}
	if cerr.Reason != want {
		t.Errorf("ConformanceError.Reason = %q, want %q (detail: %s)", cerr.Reason, want, cerr.Detail)
	}
	if cerr.Path == "" {
		t.Error("ConformanceError.Path is empty; the error must name the offending path")
	}
}
