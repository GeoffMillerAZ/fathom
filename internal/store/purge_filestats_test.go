package store_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestPurgeClearsFileStatsWithoutFingerprint (review finding, §3.2, RT-76):
// file_stats names every discovered file — an oversize or unreadable one
// too, which has no fingerprint — so the purge resolves file_stats by its
// own path column, deletes those rows, reports the location before and
// after, and leaves the unpurged path's row alone.
func TestPurgeClearsFileStatsWithoutFingerprint(t *testing.T) {
	ctx := context.Background()
	s, path := openFileStore(t)
	seedPurgeFixture(t, s)
	meta := store.ServiceMeta{ServiceID: purgeSvc, Path: "/repo", IndexedCommit: "c1", IndexedAt: 1}
	if err := s.WriteServiceSummary(ctx, meta, store.ServiceSummary{ServiceID: purgeSvc, Source: store.SummaryFromIngest}, []store.FileStat{
		{Path: "secrets/huge-credentials.json", Size: 9 << 20, Excluded: "oversize"},
		{Path: secretFile, Size: 10},
		{Path: keptFile, Size: 10},
	}); err != nil {
		t.Fatal(err)
	}
	rep, err := s.PurgePaths(ctx, purgeSvc, secretsMatcher(), store.PurgeOptions{Probes: []string{secretCanary}})
	if err != nil {
		t.Fatal(err)
	}
	if n := locationCount(t, rep.Before, string(store.LocFileStats)); n != 2 {
		t.Fatalf("file_stats before = %d, want 2 (both secrets/ rows, fingerprinted or not)", n)
	}
	if n := locationCount(t, rep.After, string(store.LocFileStats)); n != 0 {
		t.Fatalf("file_stats after = %d", n)
	}
	s.Close()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var left, kept int
	if err := db.QueryRow(`SELECT count(*) FROM file_stats WHERE path LIKE 'secrets/%'`).Scan(&left); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM file_stats WHERE path = ?`, keptFile).Scan(&kept); err != nil {
		t.Fatal(err)
	}
	if left != 0 || kept != 1 {
		t.Fatalf("file_stats after purge: %d secrets/ rows left, kept row present = %d", left, kept)
	}
}
