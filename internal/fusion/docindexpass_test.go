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

// TestIngestDocIndexSkipNamedForAFixtureService pins the no-root case: a
// service whose path is not a directory on this machine (the faux
// connector's fixture names) has no documents to read, and the report says
// so instead of failing a walk of a path that never existed.
func TestIngestDocIndexSkipNamedForAFixtureService(t *testing.T) {
	sq, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "faux.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sq.Close() })
	p := New(connector.NewFauxConnector(), sq, Config{})
	t.Cleanup(p.Close)
	rep, err := p.Ingest(context.Background(), core.ServiceRef{ID: "payment-service", Path: "payment-service"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rep.DocIndexSkipped, "no local corpus root") {
		t.Fatalf("DocIndexSkipped = %q, want it to name the missing local root", rep.DocIndexSkipped)
	}
}

// TestIngestLinksDocSpans is KR-6's production wiring: an ordinary ingest
// writes doc_links for every unit it (re)indexes — a backticked symbol
// that resolves to exactly one indexed symbol and a backticked repo path,
// each with its target's hash at link time — and nothing for a span that
// resolves to no target. An unchanged re-ingest re-links nothing (the
// link-time hashes KR-7 compares against stay as they were), an edit
// closes the link the doc no longer makes, a deleted doc's links close
// (closed, never deleted: SyncDocLinksForUnit), and a store indexed before
// the link pass is backfilled once.
func TestIngestLinksDocSpans(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, map[string]string{
		"go.mod":            "module example.com/links\n\ngo 1.22\n",
		"main.go":           "package main\n\nfunc main() { _ = Frobnicate() }\n\n// Frobnicate returns one.\nfunc Frobnicate() int { return 1 }\n",
		"docs/design.md":    "# Design\n\n## Frobbing\n\n`Frobnicate` turns the crank; see `main.go`.\nNot linked: `NoSuchSymbol`, `--json`, `docs/missing.md`.\n",
		"docs/unrelated.md": "# Other\n\n## Notes\n\nno spans here\n",
	})
	sq, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "links.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sq.Close() })
	p := New(connector.NewLocalConnector(), sq, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	svc := core.ServiceRef{ID: "links", Path: dir}

	rep, err := p.Ingest(ctx, svc)
	if err != nil {
		t.Fatal(err)
	}
	if rep.DocIndexSkipped != "" {
		t.Fatalf("DocIndexSkipped = %q", rep.DocIndexSkipped)
	}
	if rep.DocIndex.LinksWritten != 2 {
		t.Fatalf("DocIndex.LinksWritten = %d, want 2 (Frobnicate and main.go)", rep.DocIndex.LinksWritten)
	}
	open, err := sq.OpenDocLinks(ctx, "links")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]store.DocLinkView{}
	for _, l := range open {
		got[l.TargetKind+":"+l.Target] = l
	}
	sym, okSym := got["symbol:Frobnicate"]
	path, okPath := got["path:main.go"]
	if len(open) != 2 || !okSym || !okPath {
		t.Fatalf("open doc_links = %+v, want exactly symbol:Frobnicate and path:main.go", open)
	}
	if sym.TargetHashAtLink == "" || path.TargetHashAtLink == "" || sym.Path != "docs/design.md" || sym.HeadingPath != "Design > Frobbing" || sym.Line < 3 {
		t.Errorf("links carry no link-time hash or the wrong span: %+v %+v", sym, path)
	}

	// Unchanged: nothing re-indexed, nothing re-linked, rows untouched.
	rep2, err := p.Ingest(ctx, svc)
	if err != nil {
		t.Fatal(err)
	}
	if rep2.DocIndex.FilesChanged != 0 || rep2.DocIndex.LinksWritten != 0 {
		t.Fatalf("unchanged re-ingest DocIndex = %+v, want nothing re-indexed or re-linked", rep2.DocIndex)
	}
	again, _ := sq.OpenDocLinks(ctx, "links")
	if len(again) != 2 || again[0].ValidFrom != open[0].ValidFrom || again[1].ValidFrom != open[1].ValidFrom {
		t.Errorf("an unchanged re-ingest moved the links: %+v", again)
	}

	// An edit drops the path span: that link closes, the symbol link stays.
	writeCorpus(t, dir, map[string]string{"docs/design.md": "# Design\n\n## Frobbing\n\n`Frobnicate` turns the crank.\n"})
	rep3, err := p.Ingest(ctx, svc)
	if err != nil {
		t.Fatal(err)
	}
	if rep3.DocIndex.FilesChanged != 1 || rep3.DocIndex.LinksWritten != 1 {
		t.Fatalf("edit re-ingest DocIndex = %+v, want one file re-indexed holding one link", rep3.DocIndex)
	}
	if open, _ := sq.OpenDocLinks(ctx, "links"); len(open) != 1 || open[0].Target != "Frobnicate" {
		t.Errorf("after the edit open links = %+v, want only Frobnicate", open)
	}

	// Backfill: a store whose units predate the link pass (content hashes
	// without its version, no doc_links) is re-indexed and linked once.
	units, err := sq.DocUnitsForPath(ctx, "links", "docs/unrelated.md")
	if err != nil || len(units) == 0 {
		t.Fatalf("units: %v %v", units, err)
	}
	for i := range units {
		units[i].ContentHash = "legacy-" + units[i].ContentHash
	}
	if err := sq.PutDocUnits(ctx, "links", "docs/unrelated.md", units); err != nil {
		t.Fatal(err)
	}
	rep4, err := p.Ingest(ctx, svc)
	if err != nil {
		t.Fatal(err)
	}
	if rep4.DocIndex.FilesChanged != 1 {
		t.Fatalf("backfill DocIndex = %+v, want the pre-link-pass file re-indexed once", rep4.DocIndex)
	}
	if rep5, _ := p.Ingest(ctx, svc); rep5.DocIndex.FilesChanged != 0 {
		t.Fatalf("after the backfill DocIndex = %+v, want nothing re-indexed", rep5.DocIndex)
	}

	// A deleted doc's links close.
	if err := os.Remove(filepath.Join(dir, "docs", "design.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Ingest(ctx, svc); err != nil {
		t.Fatal(err)
	}
	if open, _ := sq.OpenDocLinks(ctx, "links"); len(open) != 0 {
		t.Errorf("after deleting the doc open links = %+v, want none", open)
	}
}
