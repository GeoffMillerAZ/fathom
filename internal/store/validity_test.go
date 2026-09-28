package store

// validity_test.go — GL-16 (docs/plans/governance-learning.md, design doc
// docs/design/governance-learning.md §7): validity windows beyond the claim
// that ends. Owner decision: anchored memories are being deleted (SR-13), so
// the row covers claims and doc links only — rules-file rules and
// learned-rule states are claims already, so their window is the claim's.
// What the row activates is the two halves SU-19 left open:
//
//   - a claim's valid_from is stamped at mint from its compile time, so
//     "what did we know on date D" no longer serves a claim that did not
//     exist yet (a zero valid_from is unconstrained, which is only right for
//     a row that predates the column);
//   - a doc link's window, closed never deleted since KR-6, gets its as-of
//     read: DocLinksAsOf.

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
)

func validityClaim(id string, compiledAt time.Time) core.SemanticClaim {
	return core.SemanticClaim{
		ClaimID: id, ServiceID: "svc", Layer: "obligations", LayerVersion: "v1", Kind: "obligation",
		Subject: "docs/g.md#" + id, Rule: "Rule " + id, Scope: []string{"**"},
		Tier: core.TierDocClaimed, TierProfile: "engineering", Status: core.ClaimFresh,
		PromptVersion: "p", ReproKey: id, InputDigest: "d",
		CompiledAtWatermark: "manifest:aaa", CompiledAt: compiledAt,
	}
}

// TestValidityWindowsBeyondClaims is GL-16's gate.
func TestValidityWindowsBeyondClaims(t *testing.T) {
	ctx := context.Background()
	sq, err := NewSQLiteStore(filepath.Join(t.TempDir(), "validity.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sq.Close() })
	backends := map[string]Store{"sqlite": sq, "memory": NewMemoryStore()}

	minted := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	later := minted.Add(72 * time.Hour)

	for name, st := range backends {
		t.Run("claim valid_from at mint/"+name, func(t *testing.T) {
			// A writer that set no ValidFrom (the compiler, a rules-file
			// source, a learned rule) is stamped with its compile time.
			if err := st.PutClaims(ctx, []core.SemanticClaim{validityClaim("c-mint", minted)}, nil); err != nil {
				t.Fatal(err)
			}
			// A writer that set ValidFrom keeps it.
			explicit := validityClaim("c-explicit", minted)
			explicit.ValidFrom = minted.Add(-24 * time.Hour)
			// A claim with no compile time at all stays unconstrained.
			legacy := validityClaim("c-legacy", time.Time{})
			if err := st.PutClaims(ctx, []core.SemanticClaim{explicit, legacy}, nil); err != nil {
				t.Fatal(err)
			}
			got := claimsByID(t, ctx, st)
			if !got["c-mint"].ValidFrom.Equal(minted) {
				t.Errorf("c-mint ValidFrom = %v, want its compile time %v", got["c-mint"].ValidFrom, minted)
			}
			if !got["c-explicit"].ValidFrom.Equal(minted.Add(-24 * time.Hour)) {
				t.Errorf("c-explicit ValidFrom = %v, want the writer's own value", got["c-explicit"].ValidFrom)
			}
			if !got["c-legacy"].ValidFrom.IsZero() {
				t.Errorf("c-legacy ValidFrom = %v, want zero (unconstrained) with no compile time", got["c-legacy"].ValidFrom)
			}
			if got["c-mint"].ValidAt(minted.Add(-time.Second)) {
				t.Error("a claim must not be valid before it was minted")
			}
			if !got["c-mint"].ValidAt(minted) {
				t.Error("a claim must be valid at its mint instant")
			}

			// A recompile of the same claim_id later never moves its history.
			if err := st.PutClaims(ctx, []core.SemanticClaim{validityClaim("c-mint", later)}, nil); err != nil {
				t.Fatal(err)
			}
			if again := claimsByID(t, ctx, st); !again["c-mint"].ValidFrom.Equal(minted) {
				t.Errorf("c-mint ValidFrom after recompile = %v, want the first mint %v", again["c-mint"].ValidFrom, minted)
			}
		})
	}

	t.Run("doc links as of", func(t *testing.T) {
		const svc = "fathom"
		if err := sq.PutDocUnits(ctx, svc, "x.md", []DocUnit{{UnitID: "u1", Path: "x.md", HeadingPath: "Section", Kind: "section", Body: "body"}}); err != nil {
			t.Fatal(err)
		}
		first := []DocLink{
			{TargetKind: "symbol", Target: "Foo", Line: 5, LinkTier: "exact", TargetHashAtLink: "h-foo"},
			{TargetKind: "path", Target: "a/b.go", Line: 6, LinkTier: "exact", TargetHashAtLink: "h-path"},
		}
		if err := sq.SyncDocLinksForUnit(ctx, svc, "u1", first, 1000); err != nil {
			t.Fatal(err)
		}
		// a/b.go is edited out at 2000: closed, never deleted.
		if err := sq.SyncDocLinksForUnit(ctx, svc, "u1", first[:1], 2000); err != nil {
			t.Fatal(err)
		}

		targets := func(at int64) map[string]DocLinkView {
			t.Helper()
			rows, err := sq.DocLinksAsOf(ctx, svc, at)
			if err != nil {
				t.Fatalf("DocLinksAsOf(%d): %v", at, err)
			}
			m := map[string]DocLinkView{}
			for _, r := range rows {
				m[r.Target] = r
			}
			return m
		}
		if got := targets(999); len(got) != 0 {
			t.Errorf("as of 999 (before either link) = %+v, want none", got)
		}
		in := targets(1500)
		if len(in) != 2 {
			t.Fatalf("as of 1500 = %+v, want both links (a/b.go was still open)", in)
		}
		if v := in["a/b.go"]; v.ValidFrom != 1000 || v.ValidTo != 2000 {
			t.Errorf("a/b.go window = [%d, %d), want [1000, 2000)", v.ValidFrom, v.ValidTo)
		}
		if v := in["Foo"]; v.ValidFrom != 1000 || v.ValidTo != 0 {
			t.Errorf("Foo window = [%d, %d), want [1000, open)", v.ValidFrom, v.ValidTo)
		}
		if got := targets(2000); len(got) != 1 || got["Foo"].Target == "" {
			t.Errorf("as of 2000 (the close instant, half-open) = %+v, want only Foo", got)
		}
		if got := targets(9000); len(got) != 1 {
			t.Errorf("as of 9000 = %+v, want only the still-open Foo", got)
		}
		// The closed row is still in the table.
		var n int
		if err := sq.db.QueryRowContext(ctx, `SELECT count(*) FROM doc_links WHERE service_id = ?`, svc).Scan(&n); err != nil || n != 2 {
			t.Errorf("doc_links rows = %d, %v; want 2 (closed, never deleted)", n, err)
		}
	})
}

func claimsByID(t *testing.T, ctx context.Context, st Store) map[string]core.SemanticClaim {
	t.Helper()
	all, err := st.ClaimsForService(ctx, "svc", "")
	if err != nil {
		t.Fatal(err)
	}
	m := make(map[string]core.SemanticClaim, len(all))
	for _, c := range all {
		m[c.ClaimID] = c
	}
	return m
}
