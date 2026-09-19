package store

// claims_for_serve_test.go — SU-2 (docs/plans/substrate.md §A, ADR-041
// decision 3): ClaimsForServe prefilters claims by layer, served status and
// scope prefix in the backend itself (SQL WHERE for SQLiteStore, a Go loop
// for MemoryStore) instead of loading a service's whole claim set and
// filtering in Go. The scope-prefix filter is a deliberate OVER-approximation
// (never excludes a claim obligationsCore's own match() would have kept —
// see serve.go's ClaimsForServe callers) so callers that need the exact
// match still run it themselves; this test only pins the narrowing the
// backend itself is responsible for.

import (
	"context"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

func claimsForServeFixture() []core.SemanticClaim {
	return []core.SemanticClaim{
		{ServiceID: "svc", ClaimID: "c1", Layer: "obligations", Subject: "s1", Scope: []string{"internal/store/**"}, Status: core.ClaimFresh},
		{ServiceID: "svc", ClaimID: "c2", Layer: "obligations", Subject: "s2", Scope: []string{"internal/mcp/**"}, Status: core.ClaimFresh},
		{ServiceID: "svc", ClaimID: "c3", Layer: "spec-bindings", Subject: "s3", Scope: []string{"docs/**"}, Status: core.ClaimStale},
		{ServiceID: "svc", ClaimID: "c4", Layer: "obligations", Subject: "s4", Scope: []string{"internal/store/**"}, Status: core.ClaimSuperseded},
		{ServiceID: "svc", ClaimID: "c5", Layer: "obligations", Subject: "s5", Scope: []string{"**/*.go"}, Status: core.ClaimFresh},
	}
}

func servedStatusesForTest() []core.ClaimStatus {
	return []core.ClaimStatus{core.ClaimFresh, core.ClaimStale, core.ClaimContradicted}
}

func TestClaimsForServe_FiltersLayerStatusScope(t *testing.T) {
	for _, tc := range []struct {
		name  string
		store func(t *testing.T) Store
	}{
		{"sqlite", func(t *testing.T) Store {
			s, err := NewSQLiteStore("")
			if err != nil {
				t.Fatalf("NewSQLiteStore: %v", err)
			}
			t.Cleanup(func() { _ = s.Close() })
			return s
		}},
		{"memory", func(t *testing.T) Store {
			return NewMemoryStore()
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := tc.store(t)
			ctx := context.Background()
			if err := st.PutClaims(ctx, claimsForServeFixture(), nil); err != nil {
				t.Fatalf("PutClaims: %v", err)
			}

			// Status filter alone: c4 is superseded (never served) and is
			// excluded; the other four are served statuses.
			got, err := st.ClaimsForServe(ctx, "svc", nil, servedStatusesForTest(), nil)
			if err != nil {
				t.Fatalf("ClaimsForServe (status only): %v", err)
			}
			if len(got) != 4 {
				t.Fatalf("status filter: got %d claims, want 4: %+v", len(got), got)
			}

			// Layer filter: only spec-bindings.
			got, err = st.ClaimsForServe(ctx, "svc", []string{"spec-bindings"}, servedStatusesForTest(), nil)
			if err != nil {
				t.Fatalf("ClaimsForServe (layer): %v", err)
			}
			if len(got) != 1 || got[0].ClaimID != "c3" {
				t.Fatalf("layer filter: got %+v, want [c3]", got)
			}

			// Scope-prefix filter: claims scoped under internal/store are
			// kept; a claim scoped under a disjoint directory (internal/mcp)
			// is excluded; a root-reaching glob (c5's "**/*.go") is ALWAYS
			// kept regardless of prefix — the safety net a real gitignore-
			// style match against it could hit anywhere in the tree.
			got, err = st.ClaimsForServe(ctx, "svc", nil, servedStatusesForTest(), []string{"internal/store"})
			if err != nil {
				t.Fatalf("ClaimsForServe (scope prefix): %v", err)
			}
			ids := map[string]bool{}
			for _, c := range got {
				ids[c.ClaimID] = true
			}
			if !ids["c1"] {
				t.Errorf("expected c1 (internal/store/** scope) in scope-prefix('internal/store') result: %+v", got)
			}
			if !ids["c5"] {
				t.Errorf("expected c5 (root-reaching **/*.go scope) always kept: %+v", got)
			}
			if ids["c2"] {
				t.Errorf("did not expect c2 (internal/mcp/** scope) in scope-prefix('internal/store') result: %+v", got)
			}
			if ids["c3"] {
				t.Errorf("did not expect c3 (docs/** scope) in scope-prefix('internal/store') result: %+v", got)
			}
		})
	}
}
