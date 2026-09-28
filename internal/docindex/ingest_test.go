package docindex

import (
	"context"
	"os"
	"path/filepath"
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
