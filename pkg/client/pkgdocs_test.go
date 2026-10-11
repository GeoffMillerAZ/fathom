package client_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/docindex"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// TestClientSearchPkgDocs pins the pkg/client half of KR-12's scoped docs
// search (the backend capability internal/mcp's search tool reaches): the
// client answers from the doc units indexed for one exact pinned version,
// names a version it never indexed instead of answering from a neighbour,
// refuses a key that is not a pkg: coordinate, and reports a store with no
// doc index by name.
func TestClientSearchPkgDocs(t *testing.T) {
	ctx := context.Background()
	sq, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "pkgdocs.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sq.Close() })

	ref := docindex.PkgRef{Ecosystem: "golang", Name: "example.com/gizmo", Version: "v1.0.0"}
	if err := sq.PutDocUnits(ctx, ref.Key(), "README.md", []store.DocUnit{{
		UnitID: "readme-1", Path: "README.md", LineStart: 1, LineEnd: 3, HeadingPath: "gizmo", Kind: "dep-readme",
		ContentHash: "h1", IndexedAt: time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC).UnixNano(),
		Body: "Version one crafts a widget.",
	}}); err != nil {
		t.Fatalf("PutDocUnits: %v", err)
	}

	c := client.New(connector.NewFauxConnector(), sq)
	res, err := c.SearchPkgDocs(ctx, ref.Key(), "widget", 10)
	if err != nil || res.State != docindex.PkgDocsAnswered || len(res.Hits) == 0 {
		t.Fatalf("SearchPkgDocs(v1) = %+v, %v; want answered with hits", res, err)
	}
	res, err = c.SearchPkgDocs(ctx, "pkg:golang/example.com/gizmo@v2.0.0", "widget", 10)
	if err != nil || res.State != docindex.PkgDocsNotIndexed || len(res.Hits) != 0 ||
		strings.Join(res.IndexedVersions, ",") != "v1.0.0" {
		t.Fatalf("SearchPkgDocs(v2) = %+v, %v; want not_indexed naming v1.0.0", res, err)
	}
	if _, err := c.SearchPkgDocs(ctx, "pkg:bogus/x@1", "widget", 10); !errors.Is(err, docindex.ErrBadPkgKey) {
		t.Fatalf("a bad key: err = %v, want ErrBadPkgKey", err)
	}

	mem := client.New(connector.NewFauxConnector(), store.NewMemoryStore())
	if _, err := mem.SearchPkgDocs(ctx, ref.Key(), "widget", 10); !errors.Is(err, docindex.ErrDocIndexUnavailable) {
		t.Fatalf("a MemoryStore-backed client: err = %v, want ErrDocIndexUnavailable", err)
	}
}
