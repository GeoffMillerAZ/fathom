package docindex

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/store"
)

func newDepStore(t *testing.T) *store.SQLiteStore {
	t.Helper()
	sq, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "dep.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sq.Close() })
	return sq
}

// gizmoCache lays out one version of example.com/gizmo the way a Go module
// cache would hold it, with docs that differ by version so a query can tell
// which version answered.
func gizmoCache(t *testing.T, readme, frobDoc string) string {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, dir, "README.md", readme)
	writeFile(t, dir, "gizmo.go", "// Package gizmo makes things.\npackage gizmo\n\n// "+frobDoc+"\nfunc Frob() {}\n")
	return dir
}

var depNow = time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)

// TestIngestDepDocsHashGated pins KR-11's write half: a package version's
// doc units land in the store under its own `pkg:` scope, a second ingest of
// an unchanged cache re-writes nothing (a version is read once), a changed
// file re-writes only itself, a file gone from the cache loses its units in
// the same call, and a package the cache does not hold writes nothing and is
// refused with the reason (never fetched, I56).
func TestIngestDepDocsHashGated(t *testing.T) {
	ctx := context.Background()
	sq := newDepStore(t)
	ref := PkgRef{Ecosystem: "golang", Name: "example.com/gizmo", Version: "v1.0.0"}
	dir := gizmoCache(t, "# gizmo\n\nOverview text.\n", "Frob frobnicates.")
	loc := cacheLocator(map[string]string{ref.Key(): dir})

	res, err := IngestDepDocs(ctx, sq, loc, ref, depNow)
	if err != nil {
		t.Fatalf("first IngestDepDocs: %v", err)
	}
	if res.Key != ref.Key() || res.FilesScanned != 2 || res.FilesChanged != 2 || res.FilesUnchanged != 0 || res.UnitsWritten == 0 {
		t.Fatalf("first ingest = %+v, want 2 files changed under %s", res, ref.Key())
	}
	paths, err := sq.DocIndexedPaths(ctx, ref.Key())
	if err != nil || strings.Join(paths, ",") != "README.md,gizmo.go" {
		t.Fatalf("indexed paths = %v, %v; want README.md,gizmo.go under the package's own scope", paths, err)
	}
	if others, _ := sq.DocIndexedPaths(ctx, "fathom"); len(others) != 0 {
		t.Fatalf("dependency docs leaked into another service's scope: %v", others)
	}

	res2, err := IngestDepDocs(ctx, sq, loc, ref, depNow.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if res2.FilesChanged != 0 || res2.FilesUnchanged != 2 || res2.UnitsWritten != 0 {
		t.Fatalf("second ingest = %+v, want nothing re-written", res2)
	}

	if err := os.WriteFile(filepath.Join(dir, "gizmo.go"), []byte("package gizmo\n\n// Frob now does more.\nfunc Frob() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res3, err := IngestDepDocs(ctx, sq, loc, ref, depNow.Add(2*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if res3.FilesChanged != 1 || res3.FilesUnchanged != 1 {
		t.Fatalf("third ingest = %+v, want only gizmo.go re-written", res3)
	}
	hits, err := sq.QueryDocUnits(ctx, ref.Key(), "more", 10)
	if err != nil || len(hits) != 1 || hits[0].Path != "gizmo.go" {
		t.Fatalf("edited doc not searchable: %v, %v", hits, err)
	}

	if err := os.Remove(filepath.Join(dir, "gizmo.go")); err != nil {
		t.Fatal(err)
	}
	res4, err := IngestDepDocs(ctx, sq, loc, ref, depNow.Add(3*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if res4.FilesDeleted != 1 {
		t.Fatalf("fourth ingest = %+v, want gizmo.go's units removed", res4)
	}
	if paths, _ := sq.DocIndexedPaths(ctx, ref.Key()); strings.Join(paths, ",") != "README.md" {
		t.Fatalf("paths after removal = %v, want README.md only", paths)
	}

	absent := PkgRef{Ecosystem: "golang", Name: "example.com/absent", Version: "v9.9.9"}
	if _, err := IngestDepDocs(ctx, sq, loc, absent, depNow); !errors.Is(err, ErrNotCached) {
		t.Fatalf("absent package: err = %v, want ErrNotCached", err)
	}
	if ids, _ := sq.DocIndexedServiceIDs(ctx, "pkg:golang/example.com/absent@"); len(ids) != 0 {
		t.Fatalf("a refused package wrote rows: %v", ids)
	}
}
