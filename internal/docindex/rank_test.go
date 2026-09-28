package docindex

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestRankDocsFusion pins KR-4: with no concern signal, RankDocs preserves
// plain BM25 order; a concern link on the BM25-WORST-ranked candidate lifts
// it, via reciprocal rank fusion (k = 60), above every unlinked candidate —
// the fusion actually reorders, not just annotates.
func TestRankDocsFusion(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "rank.sqlite")
	sq, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sq.Close() })

	const svc = "fathom"
	units := []store.DocUnit{
		{UnitID: "u1", Path: "a.md", HeadingPath: "gizmo gizmo gizmo", Kind: "section", ContentHash: "h1", Body: "gizmo gizmo gizmo widget"},
		{UnitID: "u2", Path: "b.md", HeadingPath: "gizmo", Kind: "section", ContentHash: "h2", Body: "gizmo widget"},
		{UnitID: "u3", Path: "c.md", HeadingPath: "unrelated", Kind: "section", ContentHash: "h3", Body: "gizmo"},
	}
	for _, u := range units {
		if err := sq.PutDocUnits(ctx, svc, u.Path, []store.DocUnit{u}); err != nil {
			t.Fatalf("PutDocUnits %s: %v", u.Path, err)
		}
	}

	baseline, err := RankDocs(ctx, sq, svc, "gizmo", nil, 10)
	if err != nil {
		t.Fatalf("RankDocs baseline: %v", err)
	}
	if len(baseline) != 3 {
		t.Fatalf("RankDocs baseline = %+v, want 3 hits", baseline)
	}
	worst := baseline[len(baseline)-1].UnitID
	// Every hit's Score must actually be the RRF value, never zero/unset.
	for _, r := range baseline {
		if r.Score <= 0 {
			t.Errorf("hit %s Score = %v, want > 0", r.UnitID, r.Score)
		}
	}

	// Link the worst-by-BM25 unit to a concern symbol, directly against the
	// store's own doc_links table (via a second raw connection to the same
	// file — the same way internal/store's own purge_filestats_test.go
	// inspects a store's raw tables) — KR-6's resolver does not exist yet,
	// but KR-4's read side (store.DocUnitsLinkedToTargets) works off the
	// table alone, so this is a legitimate way to exercise it ahead of KR-6.
	raw, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	if _, err := raw.ExecContext(ctx, `
INSERT INTO doc_links (service_id, unit_id, target_kind, target, line, link_tier, target_hash_at_link, valid_from, valid_to)
VALUES (?, ?, 'symbol', 'RankDocs', 1, 'exact', 'h', 0, 0)`, svc, worst); err != nil {
		t.Fatal(err)
	}

	fused, err := RankDocs(ctx, sq, svc, "gizmo", []string{"RankDocs"}, 10)
	if err != nil {
		t.Fatalf("RankDocs fused: %v", err)
	}
	if len(fused) != 3 {
		t.Fatalf("RankDocs fused = %+v, want 3 hits", fused)
	}
	if fused[0].UnitID != worst {
		t.Fatalf("RankDocs fused top = %q, want the concern-linked (BM25-worst) unit %q: %+v", fused[0].UnitID, worst, fused)
	}

	// No query match at all: nil, not an error.
	none, err := RankDocs(ctx, sq, svc, "zzzznomatch", nil, 10)
	if err != nil {
		t.Fatalf("RankDocs no-match: %v", err)
	}
	if len(none) != 0 {
		t.Fatalf("RankDocs no-match = %+v, want none", none)
	}
}

// TestRankDocsNilStore pins that a nil store is refused, never a nil-deref
// panic (I38: never a silent/undefined result for bad input).
func TestRankDocsNilStore(t *testing.T) {
	if _, err := RankDocs(context.Background(), nil, "fathom", "q", nil, 10); err == nil {
		t.Fatal("RankDocs(nil store): want error, got nil")
	}
}
