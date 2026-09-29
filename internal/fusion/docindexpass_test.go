package fusion

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// docIndexFixture is a service with code and three Markdown files.
func docIndexFixture(t *testing.T) (dir string, sq *store.SQLiteStore, p *Pipeline) {
	t.Helper()
	dir = t.TempDir()
	writeCorpus(t, dir, map[string]string{
		"go.mod":            "module example.com/docs\n\ngo 1.22\n",
		"main.go":           "package main\n\nfunc main() {}\n",
		"ADR.md":            "# ADRs\n\n## ADR-001 — First\n\n**Status.** Accepted, with a zephyrterm.\n",
		"docs/design/x.md":  "# X design\n\n## Section\n\nbody about quokkaterm\n",
		"docs/guides/y.md":  "# Y guide\n\n## Steps\n\nsteps about walrusterm\n",
		".fathomignore":     "private/\n",
		"private/secret.md": "# Secret\n\n## S\n\nsecretterm\n",
	})
	var err error
	sq, err = store.NewSQLiteStore(filepath.Join(t.TempDir(), "docs.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sq.Close() })
	p = New(connector.NewLocalConnector(), sq, Config{})
	t.Cleanup(p.Close)
	return dir, sq, p
}

func docHits(t *testing.T, sq *store.SQLiteStore, term string) int {
	t.Helper()
	hits, err := sq.QueryDocUnits(context.Background(), "docs", term, 10)
	if err != nil {
		t.Fatalf("QueryDocUnits(%s): %v", term, err)
	}
	return len(hits)
}

// TestIngestFillsDocIndex is KR-8's production wiring: an ordinary ingest
// fills doc_units and docs_fts from the service's Markdown, so
// `search kind: docs` answers from real documents; a re-ingest of the same
// bytes rewrites nothing (the gate is not erased by the orphan purge), an
// edit re-indexes only the edited file, and a deleted file's units leave
// with it.
func TestIngestFillsDocIndex(t *testing.T) {
	dir, sq, p := docIndexFixture(t)
	ctx := context.Background()
	svc := core.ServiceRef{ID: "docs", Path: dir}

	rep, err := p.Ingest(ctx, svc)
	if err != nil {
		t.Fatal(err)
	}
	if rep.DocIndexSkipped != "" {
		t.Fatalf("DocIndexSkipped = %q, want the pass to have run", rep.DocIndexSkipped)
	}
	if rep.DocIndex.FilesChanged != 3 || rep.DocIndex.UnitsWritten == 0 {
		t.Fatalf("DocIndex = %+v, want the 3 Markdown files indexed", rep.DocIndex)
	}
	for _, term := range []string{"zephyrterm", "quokkaterm", "walrusterm"} {
		if docHits(t, sq, term) != 1 {
			t.Errorf("no doc hit for %s after ingest", term)
		}
	}
	if docHits(t, sq, "secretterm") != 0 {
		t.Error(".fathomignore'd document was indexed: the doc index must hold only what Discover returned")
	}

	// Unchanged: nothing rewritten, and no doc-index ref pollutes the
	// service's source refs for the orphan purge to trip over.
	rep2, err := p.Ingest(ctx, svc)
	if err != nil {
		t.Fatal(err)
	}
	if rep2.DocIndex.FilesChanged != 0 || rep2.DocIndex.FilesUnchanged != 3 {
		t.Fatalf("unchanged re-ingest DocIndex = %+v, want 0 changed and 3 unchanged", rep2.DocIndex)
	}
	refs, err := sq.ListSourceRefs(ctx, "docs")
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range refs {
		if strings.HasPrefix(r, "docindex:") {
			t.Fatalf("source ref %q: the doc index must not write into the code ingest's fingerprint namespace", r)
		}
	}

	// One edit: one file re-indexed.
	if err := os.WriteFile(filepath.Join(dir, "docs", "design", "x.md"), []byte("# X design\n\n## Section\n\nnow about pangolinterm\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rep3, err := p.Ingest(ctx, svc)
	if err != nil {
		t.Fatal(err)
	}
	if rep3.DocIndex.FilesChanged != 1 || rep3.DocIndex.FilesUnchanged != 2 {
		t.Fatalf("edit re-ingest DocIndex = %+v, want only the edited file re-indexed", rep3.DocIndex)
	}
	if docHits(t, sq, "pangolinterm") != 1 || docHits(t, sq, "quokkaterm") != 0 {
		t.Error("the edited file's units must replace its old ones")
	}

	// Delete: the units go.
	if err := os.Remove(filepath.Join(dir, "docs", "guides", "y.md")); err != nil {
		t.Fatal(err)
	}
	rep4, err := p.Ingest(ctx, svc)
	if err != nil {
		t.Fatal(err)
	}
	if rep4.DocIndex.FilesDeleted != 1 || docHits(t, sq, "walrusterm") != 0 {
		t.Fatalf("delete re-ingest DocIndex = %+v, walrus hits = %d; want the deleted file's units removed", rep4.DocIndex, docHits(t, sq, "walrusterm"))
	}
}

// TestIngestDocIndexSkipNamedOnMemoryStore pins that a store with no doc
// index (MemoryStore: docs_fts is SQLite-only) is not an error and not a
// silent no-op: the report names why no doc index was built.
func TestIngestDocIndexSkipNamedOnMemoryStore(t *testing.T) {
	dir, _, _ := docIndexFixture(t)
	p := New(connector.NewLocalConnector(), store.NewMemoryStore(), Config{})
	t.Cleanup(p.Close)
	rep, err := p.Ingest(context.Background(), core.ServiceRef{ID: "docs", Path: dir})
	if err != nil {
		t.Fatalf("ingest on a store with no doc index must still succeed: %v", err)
	}
	if !strings.Contains(rep.DocIndexSkipped, "no doc index") {
		t.Fatalf("DocIndexSkipped = %q, want it to say the store has no doc index", rep.DocIndexSkipped)
	}
}

// TestIngestDocIndexFailureNeverFailsCodeIngest pins the additive rule the
// other enrichment passes follow: a doc-index pass that cannot finish is
// named in the report and never fails the code ingest. The directory below
// is one the connector leaves out (.fathomignore) but the doc corpus walk
// cannot read, so the pass fails at its walk.
func TestIngestDocIndexFailureNeverFailsCodeIngest(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads a mode-000 directory")
	}
	dir, _, p := docIndexFixture(t)
	locked := filepath.Join(dir, "private")
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })

	rep, err := p.Ingest(context.Background(), core.ServiceRef{ID: "docs", Path: dir})
	if err != nil {
		t.Fatalf("a doc-index failure must not fail the code ingest: %v", err)
	}
	if rep.Symbols == 0 {
		t.Fatal("the code ingest must still have indexed its symbols")
	}
	if !strings.Contains(rep.DocIndexSkipped, "corpus walk") {
		t.Fatalf("DocIndexSkipped = %q, want the failed walk named", rep.DocIndexSkipped)
	}
}

// TestIngestDocIndexThroughASymlinkedRoot pins that a corpus root reached
// through a symlink indexes its documents (the connector walks the link's
// target, O39; a raw filepath.Walk of the link sees zero files).
func TestIngestDocIndexThroughASymlinkedRoot(t *testing.T) {
	dir, sq, p := docIndexFixture(t)
	link := filepath.Join(t.TempDir(), "root-link")
	if err := os.Symlink(dir, link); err != nil {
		t.Fatal(err)
	}
	rep, err := p.Ingest(context.Background(), core.ServiceRef{ID: "docs", Path: link})
	if err != nil {
		t.Fatal(err)
	}
	if rep.DocIndex.FilesChanged != 3 || docHits(t, sq, "zephyrterm") != 1 {
		t.Fatalf("DocIndex = %+v: a symlinked root must still index its documents", rep.DocIndex)
	}
}
