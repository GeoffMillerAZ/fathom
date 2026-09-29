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

// TestDocUnitsLinkedToTargets pins the KR-4 read side: a unit with a
// still-holding (valid_to = 0) doc_links row naming one of the caller's
// targets is returned, most distinct targets first; a link whose valid_to
// is already set (superseded, §4's validity-window rule) does not count;
// empty targets short-circuits to nil with no query at all.
func TestDocUnitsLinkedToTargets(t *testing.T) {
	ctx := context.Background()
	sq, err := NewSQLiteStore(filepath.Join(t.TempDir(), "doclinks.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sq.Close() })

	if got, err := sq.DocUnitsLinkedToTargets(ctx, "fathom", nil); err != nil || got != nil {
		t.Fatalf("DocUnitsLinkedToTargets(nil targets) = %+v, %v, want nil, nil", got, err)
	}

	seedLink := func(unitID, target string, validTo int64) {
		t.Helper()
		if _, err := sq.db.ExecContext(ctx, `
INSERT INTO doc_links (service_id, unit_id, target_kind, target, line, link_tier, target_hash_at_link, valid_from, valid_to)
VALUES ('fathom', ?, 'symbol', ?, 1, 'exact', 'h', 0, ?)`, unitID, target, validTo); err != nil {
			t.Fatal(err)
		}
	}
	seedLink("u-two-targets", "PutDocUnits", 0)
	seedLink("u-two-targets", "SplitDocUnits", 0)
	seedLink("u-one-target", "PutDocUnits", 0)
	seedLink("u-superseded", "PutDocUnits", 12345) // valid_to set: must not count

	got, err := sq.DocUnitsLinkedToTargets(ctx, "fathom", []string{"PutDocUnits", "SplitDocUnits"})
	if err != nil {
		t.Fatalf("DocUnitsLinkedToTargets: %v", err)
	}
	want := []DocLinkMatch{{UnitID: "u-two-targets", MatchCount: 2}, {UnitID: "u-one-target", MatchCount: 1}}
	if len(got) != len(want) {
		t.Fatalf("DocUnitsLinkedToTargets = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("row %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// TestSyncDocLinksForUnit pins KR-6's write side (§4's validity-window
// rule, applied per link): an unchanged link is left completely untouched
// — its target_hash_at_link and valid_from never move, which is what lets
// KR-7's docDrift compare a frozen link-time hash against a fresh
// resolution — a removed link is closed (valid_to set, never deleted), and
// a new link is inserted open.
func TestSyncDocLinksForUnit(t *testing.T) {
	ctx := context.Background()
	sq, err := NewSQLiteStore(filepath.Join(t.TempDir(), "synclinks.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sq.Close() })
	const svc = "fathom"

	if err := sq.PutDocUnits(ctx, svc, "x.md", []DocUnit{{
		UnitID: "u1", Path: "x.md", HeadingPath: "Section", Kind: "section", Body: "body",
	}}); err != nil {
		t.Fatal(err)
	}

	first := []DocLink{
		{TargetKind: "symbol", Target: "Foo", Line: 5, LinkTier: "exact", TargetHashAtLink: "hash-foo-v1"},
		{TargetKind: "path", Target: "a/b.go", Line: 6, LinkTier: "exact", TargetHashAtLink: "hash-path-v1"},
	}
	if err := sq.SyncDocLinksForUnit(ctx, svc, "u1", first, 1000); err != nil {
		t.Fatalf("first sync: %v", err)
	}

	open, err := sq.OpenDocLinks(ctx, svc)
	if err != nil {
		t.Fatalf("OpenDocLinks after first sync: %v", err)
	}
	if len(open) != 2 {
		t.Fatalf("OpenDocLinks after first sync = %+v, want 2", open)
	}

	// Second sync: "Foo" reappears UNCHANGED (same target/line — its
	// TargetHashAtLink here is deliberately a DIFFERENT value than what
	// was stored, simulating what a caller would pass if it re-resolved
	// the symbol; the stored row must still keep the ORIGINAL hash,
	// proving the sync never overwrites an already-open link);
	// "a/b.go" is gone (the span was edited out or the path no longer
	// resolves); "Bar" is new.
	second := []DocLink{
		{TargetKind: "symbol", Target: "Foo", Line: 5, LinkTier: "exact", TargetHashAtLink: "hash-foo-v2-SHOULD-NOT-BE-STORED"},
		{TargetKind: "symbol", Target: "Bar", Line: 7, LinkTier: "exact", TargetHashAtLink: "hash-bar-v1"},
	}
	if err := sq.SyncDocLinksForUnit(ctx, svc, "u1", second, 2000); err != nil {
		t.Fatalf("second sync: %v", err)
	}

	open, err = sq.OpenDocLinks(ctx, svc)
	if err != nil {
		t.Fatalf("OpenDocLinks after second sync: %v", err)
	}
	if len(open) != 2 {
		t.Fatalf("OpenDocLinks after second sync = %+v, want 2 (Foo kept, Bar new, a/b.go closed)", open)
	}
	byTarget := make(map[string]DocLinkView, len(open))
	for _, l := range open {
		byTarget[l.Target] = l
	}
	foo, ok := byTarget["Foo"]
	if !ok {
		t.Fatalf("OpenDocLinks missing Foo: %+v", open)
	}
	if foo.TargetHashAtLink != "hash-foo-v1" {
		t.Errorf("Foo TargetHashAtLink = %q, want frozen at %q (unchanged link must not be overwritten)", foo.TargetHashAtLink, "hash-foo-v1")
	}
	if foo.ValidFrom != 1000 {
		t.Errorf("Foo ValidFrom = %d, want 1000 (unchanged link's valid_from must not move)", foo.ValidFrom)
	}
	if _, ok := byTarget["a/b.go"]; ok {
		t.Errorf("a/b.go still open after being dropped from the fresh set: %+v", open)
	}
	bar, ok := byTarget["Bar"]
	if !ok {
		t.Fatalf("OpenDocLinks missing new Bar: %+v", open)
	}
	if bar.TargetHashAtLink != "hash-bar-v1" || bar.ValidFrom != 2000 {
		t.Errorf("Bar = %+v, want hash hash-bar-v1, valid_from 2000", bar)
	}

	// a/b.go's row must still exist, closed, never deleted (§4).
	var validTo int64
	if err := sq.db.QueryRowContext(ctx, `SELECT valid_to FROM doc_links WHERE service_id = ? AND unit_id = ? AND target = ?`, svc, "u1", "a/b.go").Scan(&validTo); err != nil {
		t.Fatalf("a/b.go row missing entirely (must be closed, not deleted): %v", err)
	}
	if validTo != 2000 {
		t.Errorf("a/b.go valid_to = %d, want 2000", validTo)
	}
}

// TestDocIndexedServiceIDsByPrefix pins the store half of KR-12's
// version-exact scope: the service ids that carry doc units under a
// prefix, sorted, with a prefix that ends at a delimiter never matching a
// longer sibling ("pkg:golang/x/y@" is not "pkg:golang/x/yy@..."), and an
// empty prefix refused rather than answered with a whole-index scan.
func TestDocIndexedServiceIDsByPrefix(t *testing.T) {
	ctx := context.Background()
	sq, err := NewSQLiteStore(filepath.Join(t.TempDir(), "docindex.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sq.Close() })

	for _, svc := range []string{"pkg:golang/x/y@v2.0.0", "pkg:golang/x/y@v1.0.0", "pkg:golang/x/yy@v1.0.0", "pkg:npm/x/y@1.0.0", "fathom"} {
		if err := sq.PutDocUnits(ctx, svc, "README.md", []DocUnit{{UnitID: "u", Path: "README.md", HeadingPath: "h", Kind: "dep-readme", ContentHash: "c", Body: "body"}}); err != nil {
			t.Fatal(err)
		}
	}
	got, err := sq.DocIndexedServiceIDs(ctx, "pkg:golang/x/y@")
	if err != nil {
		t.Fatal(err)
	}
	if want := "pkg:golang/x/y@v1.0.0,pkg:golang/x/y@v2.0.0"; strings.Join(got, ",") != want {
		t.Fatalf("DocIndexedServiceIDs = %v, want %s", got, want)
	}
	none, err := sq.DocIndexedServiceIDs(ctx, "pkg:golang/absent@")
	if err != nil || len(none) != 0 {
		t.Fatalf("absent prefix = %v, %v; want none", none, err)
	}
	if _, err := sq.DocIndexedServiceIDs(ctx, ""); err == nil {
		t.Fatal("an empty prefix must be refused, not answered with a scan of every service")
	}
}

// TestQueryDocUnitsPlainTextNeverSyntaxError pins what the production doc
// ingest exposed (KR-8): once the index holds real documents, the queries
// people type carry hyphens (ADR-049, hash-gated), apostrophes, slashes,
// equals signs, parentheses and bare AND/OR, and each of those is an FTS5
// syntax error ("no such column: 049") that reached `search kind: docs` as
// a failure. A query FTS5 rejects is re-run as its literal words, each
// punctuation-joined word an adjacent-token phrase; explicit FTS5 syntax
// that parses (a quoted phrase, a prefix, OR) keeps its meaning.
func TestQueryDocUnitsPlainTextNeverSyntaxError(t *testing.T) {
	ctx := context.Background()
	sq, err := NewSQLiteStore(filepath.Join(t.TempDir(), "docindex.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sq.Close() })
	unit := func(id, heading, body string) DocUnit {
		return DocUnit{UnitID: id, Path: id + ".md", LineStart: 1, LineEnd: 2, HeadingPath: heading, Kind: "section", Body: body}
	}
	for _, u := range []DocUnit{
		unit("a", "Decisions", "ADR-049 chose a hash-gated ingest and fathom's store uses CGO_ENABLED=0 in cmd/fathom/main.go"),
		unit("b", "Other", "nothing relevant here at all"),
	} {
		if err := sq.PutDocUnits(ctx, "svc", u.Path, []DocUnit{u}); err != nil {
			t.Fatal(err)
		}
	}
	for _, q := range []string{
		"ADR-049", "hash-gated", "fathom's", "CGO_ENABLED=0", "cmd/fathom/main.go", "AND ingest", "hash-gated (ingest)", `"hash-gated ingest`,
	} {
		hits, err := sq.QueryDocUnits(ctx, "svc", q, 10)
		if err != nil {
			t.Errorf("QueryDocUnits(%q) = error %v, want the literal words searched", q, err)
			continue
		}
		if len(hits) != 1 || hits[0].UnitID != "a" {
			t.Errorf("QueryDocUnits(%q) = %+v, want exactly unit a", q, hits)
		}
	}
	// A query with no word in it cannot be searched: an error, not a
	// healthy zero (I38).
	if _, err := sq.QueryDocUnits(ctx, "svc", "-- () ''", 10); err == nil {
		t.Error("a query with no searchable word must be an error, not an empty result")
	}
	// Explicit FTS5 syntax that parses keeps its meaning.
	if hits, err := sq.QueryDocUnits(ctx, "svc", "hash OR nothing", 10); err != nil || len(hits) != 2 {
		t.Errorf("QueryDocUnits(hash OR nothing) = %+v, %v; want both units", hits, err)
	}
}
