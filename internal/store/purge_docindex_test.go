package store_test

// purge_docindex_test.go — the doc index is a content location (KR-8 fills
// it in production): a purge, a service removal and a rename must reach
// doc_units, docs_fts and doc_links exactly as they reach every other table
// that holds indexed text, or the text a purge was asked to erase stays
// searchable through `search kind: docs`.

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/store"
)

const (
	secretDoc = "secrets/notes.md"
	keptDoc   = "docs/keep.md"
	// docCanary is one distinctive token: the FTS5 index stores it as a term
	// of its own, which is what a purge that only deleted rows would leave.
	docCanary = "zzpurgecanaryzz"
)

func seedDocIndex(t *testing.T, s *store.SQLiteStore) {
	t.Helper()
	ctx := context.Background()
	unit := func(id, path, body string) store.DocUnit {
		return store.DocUnit{UnitID: id, Path: path, LineStart: 1, LineEnd: 3, HeadingPath: "Notes", Kind: "section", ContentHash: "h-" + id, IndexedAt: 1, Body: body}
	}
	if err := s.PutDocUnits(ctx, purgeSvc, secretDoc, []store.DocUnit{unit("u-secret", secretDoc, "the deploy key is "+docCanary+" do not share")}); err != nil {
		t.Fatal(err)
	}
	if err := s.PutDocUnits(ctx, purgeSvc, keptDoc, []store.DocUnit{unit("u-kept", keptDoc, "harmless prose about deploys")}); err != nil {
		t.Fatal(err)
	}
	link := []store.DocLink{{TargetKind: "path", Target: "internal/keep.go", Line: 2, LinkTier: "exact", TargetHashAtLink: "x"}}
	for _, id := range []string{"u-secret", "u-kept"} {
		if err := s.SyncDocLinksForUnit(ctx, purgeSvc, id, link, 100); err != nil {
			t.Fatal(err)
		}
	}
}

func docHitCount(t *testing.T, s *store.SQLiteStore, svc, term string) int {
	t.Helper()
	hits, err := s.QueryDocUnits(context.Background(), svc, term, 10)
	if err != nil {
		t.Fatalf("QueryDocUnits(%s): %v", term, err)
	}
	return len(hits)
}

// TestPurgePathsClearsDocIndex: purging a Markdown path removes its
// doc_units, its docs_fts rows (text and index terms) and its doc_links,
// reports each location before and after, verifies clean, and leaves the
// unpurged document searchable.
func TestPurgePathsClearsDocIndex(t *testing.T) {
	ctx := context.Background()
	s, _ := openFileStore(t)
	seedDocIndex(t, s)
	if docHitCount(t, s, purgeSvc, docCanary) != 1 {
		t.Fatal("fixture: the canary must be searchable before the purge")
	}

	rep, err := s.PurgePaths(ctx, purgeSvc, secretsMatcher(), store.PurgeOptions{Probes: []string{docCanary}})
	if err != nil {
		t.Fatalf("PurgePaths: %v", err)
	}
	for _, loc := range []store.PurgeLocation{store.LocDocUnits, store.LocDocsFTS, store.LocDocLinks} {
		if n := locationCount(t, rep.Before, string(loc)); n == 0 {
			t.Errorf("%s: purge saw no rows before, want the seeded ones", loc)
		}
		if n := locationCount(t, rep.After, string(loc)); n != 0 {
			t.Errorf("%s still holds %d purged row(s)", loc, n)
		}
	}
	if rep.Verdict != store.PurgeVerified {
		t.Fatalf("purge verdict = %s (%s); survivors %+v; probe hits %+v", rep.Verdict, rep.VerdictNote, rep.Survivors, rep.Physical.Probe.Hits)
	}
	if docHitCount(t, s, purgeSvc, docCanary) != 0 {
		t.Error("the purged document is still searchable")
	}
	if docHitCount(t, s, purgeSvc, "harmless") != 1 {
		t.Error("the unpurged document must stay searchable")
	}
}

// TestRemoveAndRenameServiceCoverTheDocIndex: a removed service leaves no
// searchable text, and a renamed one carries its documents to the new id.
func TestRemoveAndRenameServiceCoverTheDocIndex(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s, err := store.NewSQLiteStore(filepath.Join(dir, "svc.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	seedDocIndex(t, s)
	if err := s.SetServiceMeta(ctx, store.ServiceMeta{ServiceID: purgeSvc, Path: dir}); err != nil { // a service must exist for rename
		t.Fatal(err)
	}

	if err := s.RenameService(ctx, purgeSvc, "renamed"); err != nil {
		t.Fatalf("RenameService: %v", err)
	}
	if docHitCount(t, s, purgeSvc, "harmless") != 0 || docHitCount(t, s, "renamed", "harmless") != 1 {
		t.Error("a rename must move the service's documents to the new id")
	}
	if docHitCount(t, s, "renamed", docCanary) != 1 {
		t.Error("a rename must move every document, not one")
	}
	paths, err := s.DocIndexedPaths(ctx, "renamed")
	if err != nil || len(paths) != 2 {
		t.Fatalf("DocIndexedPaths(renamed) = %v, %v; want both documents", paths, err)
	}

	if _, err := s.RemoveService(ctx, "renamed"); err != nil {
		t.Fatalf("RemoveService: %v", err)
	}
	if docHitCount(t, s, "renamed", "harmless") != 0 || docHitCount(t, s, "renamed", docCanary) != 0 {
		t.Error("a removed service's documents must not stay searchable")
	}
	if paths, _ := s.DocIndexedPaths(ctx, "renamed"); len(paths) != 0 {
		t.Errorf("doc_units rows survived RemoveService: %v", paths)
	}
}
