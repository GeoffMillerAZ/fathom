package docindex

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestIngestDocUnitsHashGated pins KR-8: a first ingest writes every
// corpus file; a second ingest with nothing changed re-writes nothing
// (FilesUnchanged, not FilesChanged); editing one file re-splits and
// re-writes only that file; deleting a file removes its units in the same
// call, never leaving a stale row behind.
func TestIngestDocUnitsHashGated(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	writeFile(t, root, "ADR.md", "# ADRs\n\n## ADR-001 — First\n\n**Status.** Accepted.\n")
	writeFile(t, root, "docs/design/x.md", "# X design\n\n## Section\n\nbody text\n")

	sq, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "ingest.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sq.Close() })

	now := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	res, err := IngestDocUnits(ctx, sq, "fathom", root, now)
	if err != nil {
		t.Fatalf("first IngestDocUnits: %v", err)
	}
	if res.FilesScanned != 2 || res.FilesChanged != 2 || res.FilesUnchanged != 0 || res.FilesDeleted != 0 {
		t.Fatalf("first ingest = %+v, want 2 scanned/changed, 0 unchanged/deleted", res)
	}
	if res.UnitsWritten == 0 {
		t.Fatal("first ingest wrote 0 units")
	}
	hits, err := sq.QueryDocUnits(ctx, "fathom", "Accepted", 10)
	if err != nil {
		t.Fatalf("QueryDocUnits: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("QueryDocUnits(Accepted) = %+v, want 1 hit", hits)
	}

	// Second ingest, nothing changed: everything is unchanged, nothing
	// re-written.
	res2, err := IngestDocUnits(ctx, sq, "fathom", root, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("second IngestDocUnits: %v", err)
	}
	if res2.FilesChanged != 0 || res2.FilesUnchanged != 2 || res2.FilesDeleted != 0 {
		t.Fatalf("second ingest = %+v, want 0 changed, 2 unchanged, 0 deleted", res2)
	}

	// Edit one file: only it re-indexes.
	writeFile(t, root, "docs/design/x.md", "# X design\n\n## Section\n\ncompletely different body\n")
	res3, err := IngestDocUnits(ctx, sq, "fathom", root, now.Add(2*time.Hour))
	if err != nil {
		t.Fatalf("third IngestDocUnits: %v", err)
	}
	if res3.FilesChanged != 1 || res3.FilesUnchanged != 1 {
		t.Fatalf("third ingest = %+v, want 1 changed (x.md), 1 unchanged (ADR.md)", res3)
	}
	hits, err = sq.QueryDocUnits(ctx, "fathom", "different", 10)
	if err != nil {
		t.Fatalf("QueryDocUnits after edit: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("QueryDocUnits(different) after edit = %+v, want 1 hit", hits)
	}

	// Delete a file: its units disappear in the same ingest call.
	if err := os.Remove(filepath.Join(root, "docs", "design", "x.md")); err != nil {
		t.Fatal(err)
	}
	res4, err := IngestDocUnits(ctx, sq, "fathom", root, now.Add(3*time.Hour))
	if err != nil {
		t.Fatalf("fourth IngestDocUnits: %v", err)
	}
	if res4.FilesScanned != 1 || res4.FilesDeleted != 1 {
		t.Fatalf("fourth ingest = %+v, want 1 scanned (ADR.md only), 1 deleted (x.md)", res4)
	}
	if got, err := sq.DocUnitsForPath(ctx, "fathom", "docs/design/x.md"); err != nil {
		t.Fatalf("DocUnitsForPath after delete: %v", err)
	} else if len(got) != 0 {
		t.Fatalf("DocUnitsForPath after delete = %+v, want none", got)
	}
	if hits, err := sq.QueryDocUnits(ctx, "fathom", "different", 10); err != nil {
		t.Fatalf("QueryDocUnits after delete: %v", err)
	} else if len(hits) != 0 {
		t.Fatalf("QueryDocUnits(different) after delete = %+v, want none (deleted file)", hits)
	}
	// The still-present ADR.md unit survives untouched.
	if hits, err := sq.QueryDocUnits(ctx, "fathom", "Accepted", 10); err != nil {
		t.Fatalf("QueryDocUnits ADR after delete: %v", err)
	} else if len(hits) != 1 {
		t.Fatalf("QueryDocUnits(Accepted) after unrelated delete = %+v, want 1", hits)
	}
}

// TestIngestDocUnitsEmptyRoot pins that an ingest over a corpus with no
// Markdown files at all is a clean no-op, not an error.
func TestIngestDocUnitsEmptyRoot(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	sq, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "ingest-empty.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sq.Close() })
	res, err := IngestDocUnits(ctx, sq, "fathom", root, time.Now())
	if err != nil {
		t.Fatalf("IngestDocUnits(empty root): %v", err)
	}
	if res.FilesScanned != 0 || res.FilesChanged != 0 || res.UnitsWritten != 0 {
		t.Fatalf("IngestDocUnits(empty root) = %+v, want all zero", res)
	}
}

// openIngestStore opens a throwaway SQLite store for one test.
func openIngestStore(t *testing.T) *store.SQLiteStore {
	t.Helper()
	sq, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "ingest.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sq.Close() })
	return sq
}

// TestIngestDocUnitsDuplicateHeadingPaths pins the real-corpus defect the
// production wiring found (KR-8): two sections with the same heading path
// and kind (CHANGELOG's repeated "### Added" under one release, ADR.md's
// repeated bold labels) share a unit id under path+heading+kind, and the
// second insert failed the whole ingest on doc_units' primary key. The
// second and later occurrences get an ordinal, deterministically, so both
// units are indexed and a re-ingest of the same bytes is a no-op.
func TestIngestDocUnitsDuplicateHeadingPaths(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	writeFile(t, root, "CHANGELOG.md", "# Changelog\n\n## 1.0\n\n### Added\n\nalphafeature one\n\n### Fixed\n\nbug\n\n### Added\n\nbetafeature two\n")
	sq := openIngestStore(t)
	now := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)

	res, err := IngestDocUnits(ctx, sq, "fathom", root, now)
	if err != nil {
		t.Fatalf("IngestDocUnits with a repeated heading path: %v", err)
	}
	if res.UnitsWritten != 3 {
		t.Fatalf("UnitsWritten = %d, want 3 (both Added sections and Fixed)", res.UnitsWritten)
	}
	for _, term := range []string{"alphafeature", "betafeature"} {
		hits, err := sq.QueryDocUnits(ctx, "fathom", term, 10)
		if err != nil || len(hits) != 1 {
			t.Fatalf("QueryDocUnits(%s) = %+v, %v; want exactly the section that says it", term, hits, err)
		}
	}
	again, err := IngestDocUnits(ctx, sq, "fathom", root, now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if again.FilesChanged != 0 || again.FilesUnchanged != 1 {
		t.Fatalf("re-ingest of the same bytes = %+v, want 0 changed, 1 unchanged (stable ids)", again)
	}
}

// TestIngestDocUnitsGateLeavesNoSourceRefs pins that the per-file gate does
// not live in the shared fingerprints table: store.ListSourceRefs unions
// observations and fingerprints, and the pipeline's orphan purge deletes
// every stored ref the connector did not discover, so a "docindex:<path>"
// fingerprint would be purged (and counted as a deleted file) on the next
// code ingest, defeating the gate and dirtying the ingest's change count.
func TestIngestDocUnitsGateLeavesNoSourceRefs(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	writeFile(t, root, "docs/x.md", "# X\n\n## Section\n\nbody text\n")
	sq := openIngestStore(t)
	if _, err := IngestDocUnits(ctx, sq, "fathom", root, time.Now()); err != nil {
		t.Fatal(err)
	}
	refs, err := sq.ListSourceRefs(ctx, "fathom")
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 0 {
		t.Fatalf("ListSourceRefs after a docs ingest = %v, want none (the gate is doc_units itself)", refs)
	}
}

// docUnitBytesOf is the byte cost the budget charges for content's units.
func docUnitBytesOf(kind DocKind, content string) int {
	n := 0
	for _, u := range SplitDocUnits(kind, content) {
		n += len(u.HeadingPath) + len(u.Body)
	}
	return n
}

// TestIngestDocUnitsEnforcesBudget pins KR-9 through the ingest: the
// per-service byte budget is applied to the whole corpus, live documents
// before stale_by_location ones and path order within a group; what does
// not fit is not indexed, is named in the result, and comes back (by a
// later ingest, with no file change) when the budget grows; an unchanged
// file that stops fitting because another grew loses its units.
func TestIngestDocUnitsEnforcesBudget(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	docA := "# A\n\n## S\n\nalphaterm " + strings.Repeat("a", 60) + "\n"
	docB := "# B\n\n## S\n\nbetaterm " + strings.Repeat("b", 60) + "\n"
	docC := "# C\n\n## S\n\ngammaterm " + strings.Repeat("c", 60) + "\n"
	docOld := "# Old\n\n## S\n\narchiveterm " + strings.Repeat("o", 60) + "\n"
	writeFile(t, root, "docs/a.md", docA)
	writeFile(t, root, "docs/b.md", docB)
	writeFile(t, root, "docs/c.md", docC)
	writeFile(t, root, "docs/plans/archive/old.md", docOld)
	sq := openIngestStore(t)
	now := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)

	// Room for a and b, one byte short of c.
	budget := docUnitBytesOf(DocKindOther, docA) + docUnitBytesOf(DocKindOther, docB) + 1
	res, err := IngestDocUnits(ctx, sq, "fathom", root, now, WithBudgetBytes(budget))
	if err != nil {
		t.Fatalf("ingestDocUnits: %v", err)
	}
	if res.UnitsElided != 2 || len(res.Elided) != 2 {
		t.Fatalf("elided = %d (%+v), want c's unit and the archived unit", res.UnitsElided, res.Elided)
	}
	if res.Elided[0].Path != "docs/c.md" || res.Elided[1].Path != "docs/plans/archive/old.md" {
		t.Fatalf("elided order = %+v, want live c.md first, then the stale archive unit (priority order)", res.Elided)
	}
	if res.ElidedBytes != docUnitBytesOf(DocKindOther, docC)+docUnitBytesOf(DocKindOther, docOld) {
		t.Fatalf("ElidedBytes = %d", res.ElidedBytes)
	}
	hitCount := func(term string) int {
		hits, err := sq.QueryDocUnits(ctx, "fathom", term, 10)
		if err != nil {
			t.Fatalf("QueryDocUnits(%s): %v", term, err)
		}
		return len(hits)
	}
	if hitCount("alphaterm") != 1 || hitCount("betaterm") != 1 {
		t.Fatal("the units inside the budget must be searchable")
	}
	if hitCount("gammaterm") != 0 || hitCount("archiveterm") != 0 {
		t.Fatal("an elided unit must not be indexed (it is named in the result instead)")
	}

	// Same corpus, same budget: nothing rewritten, the same units elided.
	again, err := IngestDocUnits(ctx, sq, "fathom", root, now.Add(time.Hour), WithBudgetBytes(budget))
	if err != nil {
		t.Fatal(err)
	}
	if again.FilesChanged != 0 || again.FilesUnchanged != 4 || again.UnitsElided != 2 {
		t.Fatalf("second ingest = %+v, want 0 changed, 4 unchanged, still 2 elided", again)
	}

	// A bigger budget brings the elided units back with no file change.
	grown, err := IngestDocUnits(ctx, sq, "fathom", root, now.Add(2*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if grown.UnitsElided != 0 || grown.FilesChanged != 2 {
		t.Fatalf("grown budget ingest = %+v, want the 2 previously elided files written and nothing elided", grown)
	}
	if hitCount("gammaterm") != 1 || hitCount("archiveterm") != 1 {
		t.Fatal("units elided before must be indexed once the budget fits them")
	}

	// A file that stays byte-identical but no longer fits loses its units.
	shrunk, err := IngestDocUnits(ctx, sq, "fathom", root, now.Add(3*time.Hour), WithBudgetBytes(budget))
	if err != nil {
		t.Fatal(err)
	}
	if shrunk.UnitsElided != 2 || shrunk.FilesChanged != 2 {
		t.Fatalf("shrunk budget ingest = %+v, want the 2 files that stopped fitting rewritten to nothing", shrunk)
	}
	if hitCount("gammaterm") != 0 || hitCount("archiveterm") != 0 {
		t.Fatal("units that stopped fitting must leave the index")
	}
}

// TestIngestDocUnitsNamesUnreadableFile pins I38 for the doc corpus: a file
// this process cannot read is named in the result and skipped; the rest of
// the corpus is still indexed and the unreadable file's stored units (from
// a run when it was readable) are left exactly as they were, never purged
// as if it had been deleted.
func TestIngestDocUnitsNamesUnreadableFile(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads a mode-000 file")
	}
	ctx := context.Background()
	root := t.TempDir()
	writeFile(t, root, "docs/ok.md", "# OK\n\n## S\n\nokterm\n")
	writeFile(t, root, "docs/locked.md", "# Locked\n\n## S\n\nlockedterm\n")
	sq := openIngestStore(t)
	if _, err := IngestDocUnits(ctx, sq, "fathom", root, time.Now()); err != nil {
		t.Fatal(err)
	}
	locked := filepath.Join(root, "docs", "locked.md")
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o644) })
	if err := os.WriteFile(filepath.Join(root, "docs", "ok.md"), []byte("# OK\n\n## S\n\nokterm changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := IngestDocUnits(ctx, sq, "fathom", root, time.Now())
	if err != nil {
		t.Fatalf("an unreadable file must not fail the pass: %v", err)
	}
	if len(res.Failures) != 1 || res.Failures[0].Path != "docs/locked.md" || res.Failures[0].Err == "" {
		t.Fatalf("Failures = %+v, want docs/locked.md named with its error", res.Failures)
	}
	if res.FilesDeleted != 0 {
		t.Fatalf("FilesDeleted = %d, want 0: unreadable is not deleted", res.FilesDeleted)
	}
	if res.FilesChanged != 1 {
		t.Fatalf("FilesChanged = %d, want the readable edited file re-indexed", res.FilesChanged)
	}
	if hits, _ := sq.QueryDocUnits(ctx, "fathom", "lockedterm", 10); len(hits) != 1 {
		t.Fatalf("the unreadable file's stored units = %+v, want them left as they were", hits)
	}
}

// TestIngestDocUnitsCorpusFilter pins the production caller's contract
// (docs/design/knowledge-retrieval.md §3, "excluding generated and vendored
// paths"): the corpus is what the filter accepts. A document the code index
// was told to leave out (.fathomignore, a purge's scoped exclusion) is not
// indexed, and one already indexed leaves the index the moment the filter
// stops accepting it.
func TestIngestDocUnitsCorpusFilter(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	writeFile(t, root, "docs/keep.md", "# Keep\n\n## S\n\nkeepterm\n")
	writeFile(t, root, "private/secret.md", "# Secret\n\n## S\n\nsecretterm\n")
	sq := openIngestStore(t)
	hitCount := func(term string) int {
		hits, err := sq.QueryDocUnits(ctx, "fathom", term, 10)
		if err != nil {
			t.Fatal(err)
		}
		return len(hits)
	}

	if _, err := IngestDocUnits(ctx, sq, "fathom", root, time.Now()); err != nil {
		t.Fatal(err)
	}
	if hitCount("secretterm") != 1 {
		t.Fatal("without a filter every corpus file is indexed")
	}

	notPrivate := WithCorpusFilter(func(p string) bool { return !strings.HasPrefix(p, "private/") })
	res, err := IngestDocUnits(ctx, sq, "fathom", root, time.Now(), notPrivate)
	if err != nil {
		t.Fatal(err)
	}
	if res.FilesScanned != 1 || res.FilesDeleted != 1 {
		t.Fatalf("filtered ingest = %+v, want 1 scanned and the now-excluded document removed", res)
	}
	if hitCount("secretterm") != 0 || hitCount("keepterm") != 1 {
		t.Fatal("an excluded document must leave the index; the rest stays")
	}
}

// TestIngestDocUnitsOverBoundFileNamed pins the per-file bound: a document
// past MaxDocFileBytes is named and never read into the index; units it held
// when it was small are removed with it.
func TestIngestDocUnitsOverBoundFileNamed(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	writeFile(t, root, "docs/grows.md", "# Grows\n\n## S\n\ngrowsterm\n")
	sq := openIngestStore(t)
	if _, err := IngestDocUnits(ctx, sq, "fathom", root, time.Now()); err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, "docs/grows.md", "# Grows\n\n## S\n\ngrowsterm "+strings.Repeat("x", MaxDocFileBytes)+"\n")
	res, err := IngestDocUnits(ctx, sq, "fathom", root, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Failures) != 1 || res.Failures[0].Path != "docs/grows.md" {
		t.Fatalf("Failures = %+v, want docs/grows.md named as over the bound", res.Failures)
	}
	if hits, _ := sq.QueryDocUnits(ctx, "fathom", "growsterm", 10); len(hits) != 0 {
		t.Fatalf("an over-bound document must not stay indexed: %+v", hits)
	}
}
