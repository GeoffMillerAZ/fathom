package store

// indexes_test.go — SU-1 (docs/plans/substrate.md §A, ADR-041 decision 3):
// EnsureIndexes must be idempotent (safe to call at every Open) and the
// indexes it creates must actually be the ones SQLite's own query planner
// picks for the obligations claim query and the find_callers typed-edge
// query — an index that exists but is never chosen buys nothing.

import (
	"context"
	"strings"
	"testing"
)

func TestEnsureIndexes_Idempotent(t *testing.T) {
	s, err := NewSQLiteStore("")
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer s.Close()
	ctx := context.Background()
	// NewSQLiteStore already calls EnsureIndexes once at construction; a
	// second explicit call must be a no-op, not an error.
	if err := s.EnsureIndexes(ctx); err != nil {
		t.Fatalf("EnsureIndexes (1st explicit call): %v", err)
	}
	if err := s.EnsureIndexes(ctx); err != nil {
		t.Fatalf("EnsureIndexes (2nd explicit call): %v", err)
	}
}

// explainUsesIndex runs EXPLAIN QUERY PLAN for query and reports whether
// indexName appears in any plan step's detail text.
func explainUsesIndex(t *testing.T, s *SQLiteStore, indexName, query string, args ...any) bool {
	t.Helper()
	rows, err := s.db.QueryContext(context.Background(), "EXPLAIN QUERY PLAN "+query, args...)
	if err != nil {
		t.Fatalf("EXPLAIN QUERY PLAN: %v", err)
	}
	defer rows.Close()
	found := false
	for rows.Next() {
		var id, parent, notused int
		var detail string
		if err := rows.Scan(&id, &parent, &notused, &detail); err != nil {
			t.Fatalf("scan query plan row: %v", err)
		}
		if strings.Contains(detail, indexName) {
			found = true
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("query plan rows: %v", err)
	}
	return found
}

func TestEnsureIndexes_ObligationsClaimQueryPlan(t *testing.T) {
	s, err := NewSQLiteStore("")
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer s.Close()

	if !explainUsesIndex(t, s, "idx_semantic_claims_serve",
		`SELECT service_id, claim_id FROM semantic_claims WHERE service_id = ? AND layer = ? AND status = ?`,
		"svc", "obligations", "fresh") {
		t.Fatal("obligations claim query plan does not use idx_semantic_claims_serve")
	}
}

func TestEnsureIndexes_FindCallersTypedEdgeQueryPlan(t *testing.T) {
	s, err := NewSQLiteStore("")
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer s.Close()

	if !explainUsesIndex(t, s, "idx_typed_edges_to_symbol",
		`SELECT from_symbol FROM typed_edges WHERE service_id = ? AND to_symbol = ?`,
		"svc", "pkg.Foo") {
		t.Fatal("find_callers typed-edge query plan does not use idx_typed_edges_to_symbol")
	}

	if !explainUsesIndex(t, s, "idx_typed_edges_from_symbol",
		`SELECT to_symbol FROM typed_edges WHERE service_id = ? AND from_symbol = ?`,
		"svc", "pkg.Bar") {
		t.Fatal("find_callees typed-edge query plan does not use idx_typed_edges_from_symbol")
	}
}

// TestMemoryStore_EnsureIndexesIsNoOp proves the interface parity SU-1 asks
// for: MemoryStore has no SQL planner to steer, so EnsureIndexes is simply
// a safe no-op there.
func TestMemoryStore_EnsureIndexesIsNoOp(t *testing.T) {
	m := NewMemoryStore()
	if err := m.EnsureIndexes(context.Background()); err != nil {
		t.Fatalf("MemoryStore.EnsureIndexes: %v", err)
	}
}
