package docindex

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/store"
)

var depNow = time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)

func newDepStore(t *testing.T) *store.SQLiteStore {
	t.Helper()
	sq, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "dep.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sq.Close() })
	return sq
}

// putPkgDocs writes one README unit and one API unit under ref's own scope,
// the shape the removed IngestDepDocs wrote (the doc_units rows searchPkgDocs
// reads), so these tests no longer depend on a cache reader.
func putPkgDocs(t *testing.T, sq *store.SQLiteStore, ref PkgRef, readme, frobDoc string) {
	t.Helper()
	ctx := context.Background()
	unit := func(path, heading, kind, body string) store.DocUnit {
		return store.DocUnit{UnitID: ref.Key() + "|" + path + "|" + heading, Path: path, LineStart: 1, LineEnd: 1,
			HeadingPath: heading, Kind: kind, ContentHash: heading + body, IndexedAt: depNow.UnixNano(), Body: body}
	}
	if err := sq.PutDocUnits(ctx, ref.Key(), "README.md", []store.DocUnit{unit("README.md", "gizmo", "dep-readme", readme)}); err != nil {
		t.Fatal(err)
	}
	if err := sq.PutDocUnits(ctx, ref.Key(), "gizmo.go", []store.DocUnit{unit("gizmo.go", "Frob", "dep-doc", frobDoc)}); err != nil {
		t.Fatal(err)
	}
}

// TestSearchPkgDocsVersionExact pins KR-12: a `search kind: docs` scoped to
// a `pkg:` key answers from THAT version's docs and no other's — the same
// package at two versions keeps two doc sets, a version never indexed is
// named (with the versions that are) instead of answered from a neighbour,
// an unpinned scope is refused rather than guessed, and a sibling package
// whose name merely starts the same never leaks in.
func TestSearchPkgDocsVersionExact(t *testing.T) {
	ctx := context.Background()
	sq := newDepStore(t)

	v1 := PkgRef{Ecosystem: "golang", Name: "example.com/gizmo", Version: "v1.0.0"}
	v2 := PkgRef{Ecosystem: "golang", Name: "example.com/gizmo", Version: "v2.0.0"}
	sib := PkgRef{Ecosystem: "golang", Name: "example.com/gizmo2", Version: "v1.0.0"}
	putPkgDocs(t, sq, v1, "Version one crafts a widget.", "Frob returns a widget in version one.")
	putPkgDocs(t, sq, v2, "Version two crafts a gadget.", "Frob returns a gadget in version two.")
	putPkgDocs(t, sq, sib, "The sibling crafts a doohickey.", "Frob returns a doohickey.")

	ask := func(ref PkgRef, q string) PkgDocsResult {
		t.Helper()
		res, err := searchPkgDocs(ctx, sq, ref, q, 10)
		if err != nil {
			t.Fatalf("searchPkgDocs(%s, %q): %v", ref.Key(), q, err)
		}
		return res
	}

	// Each version answers from its own docs.
	r := ask(v1, "widget")
	if r.State != PkgDocsAnswered || len(r.Hits) == 0 || r.Key != v1.Key() {
		t.Fatalf("v1 widget = %+v, want answered with hits", r)
	}
	for _, h := range r.Hits {
		if !strings.Contains(strings.ToLower(h.Snippet), "widget") {
			t.Fatalf("v1 hit does not come from v1's docs: %+v", h)
		}
	}
	if r := ask(v1, "gadget"); r.State != PkgDocsAnswered || len(r.Hits) != 0 {
		t.Fatalf("v1 must not answer from v2's docs: %+v", r)
	}
	if r := ask(v2, "gadget"); r.State != PkgDocsAnswered || len(r.Hits) == 0 {
		t.Fatalf("v2 gadget = %+v, want hits", r)
	}
	if r := ask(v2, "widget"); len(r.Hits) != 0 {
		t.Fatalf("v2 must not answer from v1's docs: %+v", r)
	}
	if r := ask(v1, "doohickey"); len(r.Hits) != 0 {
		t.Fatalf("a sibling package's docs leaked into v1: %+v", r)
	}

	// A version never indexed is named, with the versions that are; no
	// fallback to a neighbour even though "widget" would hit v1.
	v3 := PkgRef{Ecosystem: "golang", Name: "example.com/gizmo", Version: "v3.0.0"}
	r = ask(v3, "widget")
	if r.State != PkgDocsNotIndexed || len(r.Hits) != 0 {
		t.Fatalf("v3 = %+v, want not_indexed with no hits", r)
	}
	if strings.Join(r.IndexedVersions, ",") != "v1.0.0,v2.0.0" {
		t.Fatalf("IndexedVersions = %v, want exactly this package's v1.0.0,v2.0.0 (not the sibling's)", r.IndexedVersions)
	}
	for _, want := range []string{"v3.0.0", "never fetched"} {
		if !strings.Contains(r.Reason, want) {
			t.Fatalf("reason %q should mention %q", r.Reason, want)
		}
	}

	// An unpinned scope is refused, listing what could be pinned.
	unpinned := PkgRef{Ecosystem: "golang", Name: "example.com/gizmo"}
	r = ask(unpinned, "widget")
	if r.State != PkgDocsUnpinned || len(r.Hits) != 0 || strings.Join(r.IndexedVersions, ",") != "v1.0.0,v2.0.0" {
		t.Fatalf("unpinned = %+v, want unpinned listing v1.0.0,v2.0.0", r)
	}

	// The string entry point parses the key the same way.
	res, err := SearchPkgDocs(ctx, sq, "pkg:golang/example.com/gizmo@v2.0.0", "gadget", 10)
	if err != nil || res.State != PkgDocsAnswered || len(res.Hits) == 0 {
		t.Fatalf("SearchPkgDocs = %+v, %v", res, err)
	}
	res, err = SearchPkgDocs(ctx, sq, "golang/example.com/gizmo@v1.0.0", "widget", 10)
	if err != nil || len(res.Hits) == 0 {
		t.Fatalf("SearchPkgDocs without the pkg: prefix = %+v, %v", res, err)
	}
	if _, err := SearchPkgDocs(ctx, sq, "pkg:golang/example.com/gizmo@v1.0.0#Frob", "x", 10); !errors.Is(err, ErrBadPkgKey) {
		t.Fatalf("a #symbol scope: err = %v, want ErrBadPkgKey", err)
	}
	if _, err := SearchPkgDocs(ctx, store.NewMemoryStore(), v1.Key(), "widget", 10); !errors.Is(err, ErrDocIndexUnavailable) {
		t.Fatalf("a non-SQLite store: err = %v, want ErrDocIndexUnavailable", err)
	}
}

// TestSearchPkgDocsNamesWhatIndexingCut pins that a package indexed under a
// byte budget says so on every search of it (KR-9's named elision carried to
// dependency docs, I38): a partial index is never a healthy complete one.
// The report row is what an indexer leaves beside the units.
func TestSearchPkgDocsNamesWhatIndexingCut(t *testing.T) {
	ctx := context.Background()
	sq := newDepStore(t)
	ref := PkgRef{Ecosystem: "golang", Name: "example.com/gizmo", Version: "v1.0.0"}
	putPkgDocs(t, sq, ref, "A readme with words to keep.", "Alpha does the first thing.")
	if err := sq.SetFingerprint(ctx, ref.Key(), depReportRef, `{"elided":2,"skipped":0}`); err != nil {
		t.Fatal(err)
	}
	got, err := searchPkgDocs(ctx, sq, ref, "thing", 10)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != PkgDocsAnswered || !strings.Contains(got.Partial, "elided") {
		t.Fatalf("search of a budget-cut package = %+v, want Partial naming the elision", got)
	}

	// The same package with no cut recorded carries no such note.
	sq2 := newDepStore(t)
	putPkgDocs(t, sq2, ref, "A readme with words to keep.", "Alpha does the first thing.")
	got, err = searchPkgDocs(ctx, sq2, ref, "thing", 10)
	if err != nil || got.Partial != "" {
		t.Fatalf("a complete index must not claim a cut: %+v, %v", got, err)
	}
}
