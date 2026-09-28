package store

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

// TestDocUnitsFTS5RoundTrip pins KR-1: docs_fts (FTS5, porter unicode61)
// plus doc_units round-trip through PutDocUnits/QueryDocUnits/
// DocUnitsForPath on the pure-Go modernc.org/sqlite driver — the probe
// docs/design/knowledge-retrieval.md §2 decision 1 cites, now a real test
// instead of a claim. It also pins that a changed path's units replace the
// old ones (never accumulate) and that a deleted path's units disappear
// from both tables in one call.
func TestDocUnitsFTS5RoundTrip(t *testing.T) {
	ctx := context.Background()
	sq, err := NewSQLiteStore(filepath.Join(t.TempDir(), "docindex.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sq.Close() })

	const svc = "fathom"
	units := []DocUnit{
		{
			UnitID:      "adr-049#decisions",
			Path:        "ADR.md",
			LineStart:   10,
			LineEnd:     20,
			HeadingPath: "ADR-049 > Decisions",
			Kind:        "adr-section",
			ContentHash: "h1",
			IndexedAt:   1000,
			Body:        "SQLite FTS5 in the existing store, no embeddings, reciprocal rank fusion",
		},
		{
			UnitID:      "kr-plan#row-kr1",
			Path:        "docs/plans/knowledge-retrieval.md",
			LineStart:   7,
			LineEnd:     7,
			HeadingPath: "Wave 2 > KR-1",
			Kind:        "plan-row",
			ContentHash: "h2",
			IndexedAt:   1000,
			Body:        "docs_fts, doc_units and doc_links, additive to store format v0",
		},
	}
	if err := sq.PutDocUnits(ctx, svc, "ADR.md", units[:1]); err != nil {
		t.Fatalf("PutDocUnits ADR.md: %v", err)
	}
	if err := sq.PutDocUnits(ctx, svc, "docs/plans/knowledge-retrieval.md", units[1:]); err != nil {
		t.Fatalf("PutDocUnits plan: %v", err)
	}

	hits, err := sq.QueryDocUnits(ctx, svc, "\"reciprocal rank fusion\"", 10)
	if err != nil {
		t.Fatalf("QueryDocUnits: %v", err)
	}
	if len(hits) != 1 || hits[0].UnitID != "adr-049#decisions" {
		t.Fatalf("QueryDocUnits fusion = %+v, want exactly the ADR-049 unit", hits)
	}
	if !strings.Contains(hits[0].Snippet, "[") {
		t.Fatalf("QueryDocUnits snippet has no highlight markers: %q", hits[0].Snippet)
	}
	if hits[0].HeadingPath != "ADR-049 > Decisions" || hits[0].Kind != "adr-section" {
		t.Fatalf("QueryDocUnits metadata = %+v", hits[0])
	}

	hits, err = sq.QueryDocUnits(ctx, svc, "docs_fts", 10)
	if err != nil {
		t.Fatalf("QueryDocUnits docs_fts: %v", err)
	}
	if len(hits) != 1 || hits[0].UnitID != "kr-plan#row-kr1" {
		t.Fatalf("QueryDocUnits docs_fts = %+v, want exactly the plan-row unit", hits)
	}

	// Replacing ADR.md's units drops the old one and keeps the plan row.
	replacement := []DocUnit{{
		UnitID:      "adr-049#context",
		Path:        "ADR.md",
		LineStart:   1,
		LineEnd:     9,
		HeadingPath: "ADR-049 > Context",
		Kind:        "adr-section",
		ContentHash: "h1b",
		IndexedAt:   2000,
		Body:        "fathom measured badly on documentation questions",
	}}
	if err := sq.PutDocUnits(ctx, svc, "ADR.md", replacement); err != nil {
		t.Fatalf("PutDocUnits replace: %v", err)
	}
	got, err := sq.DocUnitsForPath(ctx, svc, "ADR.md")
	if err != nil {
		t.Fatalf("DocUnitsForPath: %v", err)
	}
	if len(got) != 1 || got[0].UnitID != "adr-049#context" || got[0].ContentHash != "h1b" {
		t.Fatalf("DocUnitsForPath after replace = %+v, want exactly the new unit", got)
	}
	if hits, err := sq.QueryDocUnits(ctx, svc, "reciprocal", 10); err != nil {
		t.Fatalf("QueryDocUnits stale: %v", err)
	} else if len(hits) != 0 {
		t.Fatalf("QueryDocUnits stale = %+v, want none (old ADR.md body replaced)", hits)
	}

	// Deleting the plan path removes it from both tables.
	if err := sq.DeleteDocUnitsForPath(ctx, svc, "docs/plans/knowledge-retrieval.md"); err != nil {
		t.Fatalf("DeleteDocUnitsForPath: %v", err)
	}
	if got, err := sq.DocUnitsForPath(ctx, svc, "docs/plans/knowledge-retrieval.md"); err != nil {
		t.Fatalf("DocUnitsForPath after delete: %v", err)
	} else if len(got) != 0 {
		t.Fatalf("DocUnitsForPath after delete = %+v, want none", got)
	}
	if hits, err := sq.QueryDocUnits(ctx, svc, "docs_fts", 10); err != nil {
		t.Fatalf("QueryDocUnits after delete: %v", err)
	} else if len(hits) != 0 {
		t.Fatalf("QueryDocUnits after delete = %+v, want none", hits)
	}

	// stale_by_location survives the round trip.
	if err := sq.PutDocUnits(ctx, svc, "docs/plans/archive/old.md", []DocUnit{{
		UnitID: "old#u1", Path: "docs/plans/archive/old.md", HeadingPath: "Old",
		Kind: "plan-row", ContentHash: "h3", StaleByLocation: true, Body: "archived plan text",
	}}); err != nil {
		t.Fatalf("PutDocUnits archive: %v", err)
	}
	if got, err := sq.DocUnitsForPath(ctx, svc, "docs/plans/archive/old.md"); err != nil {
		t.Fatalf("DocUnitsForPath archive: %v", err)
	} else if len(got) != 1 || !got[0].StaleByLocation {
		t.Fatalf("DocUnitsForPath archive = %+v, want StaleByLocation=true", got)
	}
}

// TestPutDocUnitsRejectsEmptyUnitID pins that a unit with no UnitID is
// refused rather than silently written under an empty primary key half —
// doc_units' PRIMARY KEY is (service_id, unit_id), and a blank id would
// collide across every such unit in a path.
func TestPutDocUnitsRejectsEmptyUnitID(t *testing.T) {
	ctx := context.Background()
	sq, err := NewSQLiteStore(filepath.Join(t.TempDir(), "docindex-bad.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sq.Close() })
	err = sq.PutDocUnits(ctx, "fathom", "x.md", []DocUnit{{Path: "x.md", Body: "no id"}})
	if err == nil {
		t.Fatal("PutDocUnits with empty UnitID: want error, got nil")
	}
}
